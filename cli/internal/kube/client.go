package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// SetManualReconciliationLabel sets the kratix.io/manual-reconciliation=true label to trigger pipeline re-execution.
func (c *Client) SetManualReconciliationLabel(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	obj, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting resource: %w", err)
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["kratix.io/manual-reconciliation"] = "true"
	obj.SetLabels(labels)

	_, err = c.Dynamic.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating label: %w", err)
	}
	return nil
}

// ArgoAppInfo holds parsed ArgoCD application status for display.
type ArgoAppInfo struct {
	Name         string
	SyncStatus   string
	HealthStatus string
	OpPhase      string
	RetryCount   int64
	OpStartedAt  string
	Message      string
	HasSelfHeal  bool
	DestName     string
}

// ListArgoAppsForCluster returns all ArgoCD applications targeting a specific cluster destination.
func (c *Client) ListArgoAppsForCluster(ctx context.Context, namespace, clusterName string) ([]ArgoAppInfo, error) {
	apps, err := c.ListArgoApps(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var result []ArgoAppInfo
	for _, app := range apps {
		destName, _, _ := unstructuredNestedString(app.Object, "spec", "destination", "name")
		if destName != clusterName {
			continue
		}

		info := ArgoAppInfo{
			Name:     app.GetName(),
			DestName: destName,
		}

		info.SyncStatus, _, _ = unstructuredNestedString(app.Object, "status", "sync", "status")
		info.HealthStatus, _, _ = unstructuredNestedString(app.Object, "status", "health", "status")

		// Check operation state
		info.OpPhase, _, _ = unstructuredNestedString(app.Object, "status", "operationState", "phase")
		info.OpStartedAt, _, _ = unstructuredNestedString(app.Object, "status", "operationState", "startedAt")
		info.Message, _, _ = unstructuredNestedString(app.Object, "status", "operationState", "message")

		// Check retry count
		retryVal, found, _ := nestedFieldInt64(app.Object, "status", "operationState", "retryCount")
		if found {
			info.RetryCount = retryVal
		}

		// Check selfHeal policy
		selfHeal, found, _ := nestedFieldBool(app.Object, "spec", "syncPolicy", "automated", "selfHeal")
		info.HasSelfHeal = found && selfHeal

		result = append(result, info)
	}
	return result, nil
}

// ClearArgoAppOperationState removes the operationState from an ArgoCD application.
func (c *Client) ClearArgoAppOperationState(ctx context.Context, namespace, name string) error {
	patch := []byte(`[{"op": "remove", "path": "/status/operationState"}]`)
	_, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Patch(
		ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{}, "status",
	)
	if err != nil {
		return fmt.Errorf("clearing operation state on %s: %w", name, err)
	}
	return nil
}

// TriggerArgoAppSync triggers a sync operation on an ArgoCD application.
func (c *Client) TriggerArgoAppSync(ctx context.Context, namespace, name string) error {
	// Get current app to read its target revision
	app, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting app %s: %w", name, err)
	}

	revision, _, _ := unstructuredNestedString(app.Object, "spec", "source", "targetRevision")

	patch := fmt.Sprintf(`{"operation":{"initiatedBy":{"username":"hctl"},"sync":{"revision":"%s"}}}`, revision)
	_, err = c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("triggering sync on %s: %w", name, err)
	}
	return nil
}

// GetPodResourceInfo returns resource allocation info for pods matching a selector.
func (c *Client) GetPodResourceInfo(ctx context.Context, namespace, labelSelector string) ([]PodResourceInfo, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var result []PodResourceInfo
	for _, p := range pods.Items {
		info := PodResourceInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
		}

		// Get resource requests/limits from spec
		for _, container := range p.Spec.Containers {
			if req, ok := container.Resources.Requests["memory"]; ok {
				info.MemoryRequest = req.String()
			}
			if lim, ok := container.Resources.Limits["memory"]; ok {
				info.MemoryLimit = lim.String()
			}
			if req, ok := container.Resources.Requests["cpu"]; ok {
				info.CPURequest = req.String()
			}
			if lim, ok := container.Resources.Limits["cpu"]; ok {
				info.CPULimit = lim.String()
			}
		}

		// Restart count
		for _, cs := range p.Status.ContainerStatuses {
			info.Restarts += int(cs.RestartCount)
		}

		result = append(result, info)
	}
	return result, nil
}

// PodResourceInfo holds pod resource allocation info.
type PodResourceInfo struct {
	Name          string
	Namespace     string
	Phase         string
	MemoryRequest string
	MemoryLimit   string
	CPURequest    string
	CPULimit      string
	Restarts      int
}

// helper functions for nested field access on unstructured objects
func unstructuredNestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	val, found, err := nestedFieldGeneric(obj, fields...)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := val.(string)
	return s, ok, nil
}

func nestedFieldInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := nestedFieldGeneric(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}
	switch v := val.(type) {
	case int64:
		return v, true, nil
	case float64:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	default:
		return 0, false, nil
	}
}

func nestedFieldBool(obj map[string]interface{}, fields ...string) (bool, bool, error) {
	val, found, err := nestedFieldGeneric(obj, fields...)
	if !found || err != nil {
		return false, found, err
	}
	b, ok := val.(bool)
	return b, ok, nil
}

func nestedFieldGeneric(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var current interface{} = obj
	for _, f := range fields {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
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
