package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// KratixPromiseGVR is the GroupVersionResource for Kratix Promises.
var KratixPromiseGVR = schema.GroupVersionResource{
	Group:    "platform.kratix.io",
	Version:  "v1alpha1",
	Resource: "promises",
}

// WorkGVR is the GroupVersionResource for Kratix Work objects.
var WorkGVR = schema.GroupVersionResource{
	Group:    "platform.kratix.io",
	Version:  "v1alpha1",
	Resource: "works",
}

// WorkPlacementGVR is the GroupVersionResource for Kratix WorkPlacement objects.
var WorkPlacementGVR = schema.GroupVersionResource{
	Group:    "platform.kratix.io",
	Version:  "v1alpha1",
	Resource: "workplacements",
}

// ListPromises returns all Kratix Promise resources.
func (c *Client) ListPromises(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(KratixPromiseGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing promises: %w", err)
	}
	return list.Items, nil
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
