package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes client-go for platform operations.
type Client struct {
	Clientset *kubernetes.Clientset
	Dynamic   dynamic.Interface
	Config    *rest.Config
}

// NewClient creates a new Kubernetes client, optionally targeting a specific context.
func NewClient(kubeContext string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Dynamic:   dyn,
		Config:    cfg,
	}, nil
}

// --- Kratix Resource Helpers ---

var (
	// VClusterOrchestratorV2GVR is the GroupVersionResource for the vCluster orchestrator.
	VClusterOrchestratorV2GVR = schema.GroupVersionResource{
		Group:    "platform.integratn.tech",
		Version:  "v1alpha1",
		Resource: "vclusterorchestratorv2s",
	}

	// ArgoCDApplicationGVR is the GroupVersionResource for ArgoCD Application CRs.
	ArgoCDApplicationGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	// KratixPromiseGVR is the GroupVersionResource for Kratix Promises.
	KratixPromiseGVR = schema.GroupVersionResource{
		Group:    "platform.kratix.io",
		Version:  "v1alpha1",
		Resource: "promises",
	}

	// WorkGVR is the GroupVersionResource for Kratix Work objects.
	WorkGVR = schema.GroupVersionResource{
		Group:    "platform.kratix.io",
		Version:  "v1alpha1",
		Resource: "works",
	}

	// WorkPlacementGVR is the GroupVersionResource for Kratix WorkPlacement objects.
	WorkPlacementGVR = schema.GroupVersionResource{
		Group:    "platform.kratix.io",
		Version:  "v1alpha1",
		Resource: "workplacements",
	}
)

// ListVClusters returns all VClusterOrchestratorV2 resources.
func (c *Client) ListVClusters(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(VClusterOrchestratorV2GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing vclusters: %w", err)
	}
	return list.Items, nil
}

// GetVCluster returns a specific VClusterOrchestratorV2 resource.
func (c *Client) GetVCluster(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	obj, err := c.Dynamic.Resource(VClusterOrchestratorV2GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting vcluster %s: %w", name, err)
	}
	return obj, nil
}

// ListArgoApps returns all ArgoCD Application resources in the given namespace.
func (c *Client) ListArgoApps(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing argocd applications: %w", err)
	}
	return list.Items, nil
}

// GetArgoApp returns a specific ArgoCD Application.
func (c *Client) GetArgoApp(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	obj, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting argocd application %s: %w", name, err)
	}
	return obj, nil
}

// ListPromises returns all Kratix Promise resources.
func (c *Client) ListPromises(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(KratixPromiseGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing promises: %w", err)
	}
	return list.Items, nil
}

// GetSecretData returns the decoded data from a Secret.
func (c *Client) GetSecretData(ctx context.Context, namespace, name string) (map[string][]byte, error) {
	secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting secret %s/%s: %w", namespace, name, err)
	}
	return secret.Data, nil
}

// ListNodes returns the cluster nodes.
func (c *Client) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var result []NodeInfo
	for _, n := range nodes.Items {
		info := NodeInfo{
			Name: n.Name,
		}
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" {
				info.Ready = cond.Status == "True"
				break
			}
		}
		if v, ok := n.Status.Allocatable["cpu"]; ok {
			info.CPU = v.String()
		}
		if v, ok := n.Status.Allocatable["memory"]; ok {
			info.Memory = v.String()
		}
		for _, addr := range n.Status.Addresses {
			if addr.Type == "InternalIP" {
				info.IP = addr.Address
				break
			}
		}
		info.Roles = extractRoles(n.Labels)
		result = append(result, info)
	}
	return result, nil
}

// NodeInfo is a simplified node representation for display.
type NodeInfo struct {
	Name   string
	Ready  bool
	IP     string
	CPU    string
	Memory string
	Roles  []string
}

func extractRoles(labels map[string]string) []string {
	var roles []string
	for k := range labels {
		if len(k) > 24 && k[:24] == "node-role.kubernetes.io/" {
			roles = append(roles, k[24:])
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "<none>")
	}
	return roles
}

// SetReconcileAnnotation sets the platform.integratn.tech/reconcile-at annotation on a resource.
func (c *Client) SetReconcileAnnotation(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, timestamp string) error {
	obj, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting resource: %w", err)
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["platform.integratn.tech/reconcile-at"] = timestamp
	obj.SetAnnotations(annotations)

	_, err = c.Dynamic.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating annotation: %w", err)
	}
	return nil
}

// ListPods returns pods matching a label selector in a namespace.
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) ([]PodInfo, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var result []PodInfo
	for _, p := range pods.Items {
		info := PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
		}
		ready := 0
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}
		info.ReadyContainers = ready
		info.TotalContainers = len(p.Spec.Containers)
		result = append(result, info)
	}
	return result, nil
}

// PodInfo is a simplified pod representation.
type PodInfo struct {
	Name            string
	Namespace       string
	Phase           string
	ReadyContainers int
	TotalContainers int
}

// WriteKubeconfig writes kubeconfig data to a file.
func WriteKubeconfig(data []byte, name string) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".kube", "hctl")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
