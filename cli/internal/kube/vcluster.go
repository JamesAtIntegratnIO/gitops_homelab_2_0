package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VClusterOrchestratorV2GVR is the GroupVersionResource for the vCluster orchestrator.
var VClusterOrchestratorV2GVR = schema.GroupVersionResource{
	Group:    "platform.integratn.tech",
	Version:  "v1alpha1",
	Resource: "vclusterorchestratorv2s",
}

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
