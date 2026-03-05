package kube

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes client-go for platform operations.
//
// The cmd/ layer imports kube/ directly for lightweight operations (scaling,
// log streaming, ArgoCD toggling). The platform/ package provides a KubeClient
// interface for testable status-collection workflows.
type Client struct {
	Clientset kubernetes.Interface
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
