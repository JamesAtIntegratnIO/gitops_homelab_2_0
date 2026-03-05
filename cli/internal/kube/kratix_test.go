package kube

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// newFakeClientForKratix extends the scheme with Work and WorkPlacement GVKs
// so the dynamic fake client can handle them.
func newFakeClientForKratix(dynObjs ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	for _, gvk := range []schema.GroupVersionKind{
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "Promise"},
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "PromiseList"},
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "Work"},
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "WorkList"},
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "WorkPlacement"},
		{Group: "platform.kratix.io", Version: "v1alpha1", Kind: "WorkPlacementList"},
	} {
		if gvk.Kind == "PromiseList" || gvk.Kind == "WorkList" || gvk.Kind == "WorkPlacementList" {
			scheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
		} else {
			scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		}
	}
	return &Client{
		Dynamic: dynamicfake.NewSimpleDynamicClient(scheme, dynObjs...),
	}
}

func makeWork(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kratix.io/v1alpha1",
			"kind":       "Work",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func makeWorkPlacement(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kratix.io/v1alpha1",
			"kind":       "WorkPlacement",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func TestListWorks(t *testing.T) {
	w1 := makeWork("work-1", "kratix-platform-system")
	w2 := makeWork("work-2", "kratix-platform-system")

	c := newFakeClientForKratix(w1, w2)
	ctx := context.Background()

	works, err := c.ListWorks(ctx, "kratix-platform-system")
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if len(works) != 2 {
		t.Errorf("expected 2 works, got %d", len(works))
	}
}

func TestListWorks_EmptyNamespace(t *testing.T) {
	c := newFakeClientForKratix()
	ctx := context.Background()

	works, err := c.ListWorks(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if len(works) != 0 {
		t.Errorf("expected 0 works, got %d", len(works))
	}
}

func TestListWorkPlacements(t *testing.T) {
	wp := makeWorkPlacement("wp-1", "kratix-platform-system")

	c := newFakeClientForKratix(wp)
	ctx := context.Background()

	placements, err := c.ListWorkPlacements(ctx, "kratix-platform-system")
	if err != nil {
		t.Fatalf("ListWorkPlacements: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("expected 1 work placement, got %d", len(placements))
	}
	if placements[0].GetName() != "wp-1" {
		t.Errorf("expected name wp-1, got %s", placements[0].GetName())
	}
}

func TestSetReconcileAnnotation(t *testing.T) {
	promise := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kratix.io/v1alpha1",
			"kind":       "Promise",
			"metadata": map[string]interface{}{
				"name":      "my-promise",
				"namespace": "default",
			},
		},
	}

	c := newFakeClientForKratix(promise)
	ctx := context.Background()

	err := c.SetReconcileAnnotation(ctx, KratixPromiseGVR, "default", "my-promise", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("SetReconcileAnnotation: %v", err)
	}

	// Verify annotation was set
	updated, err := c.Dynamic.Resource(KratixPromiseGVR).Namespace("default").Get(ctx, "my-promise", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get after annotation: %v", err)
	}
	annotations := updated.GetAnnotations()
	if annotations["platform.integratn.tech/reconcile-at"] != "2026-01-01T00:00:00Z" {
		t.Errorf("annotation = %q, want %q", annotations["platform.integratn.tech/reconcile-at"], "2026-01-01T00:00:00Z")
	}
}

func TestSetManualReconciliationLabel(t *testing.T) {
	promise := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kratix.io/v1alpha1",
			"kind":       "Promise",
			"metadata": map[string]interface{}{
				"name":      "my-promise",
				"namespace": "default",
			},
		},
	}

	c := newFakeClientForKratix(promise)
	ctx := context.Background()

	err := c.SetManualReconciliationLabel(ctx, KratixPromiseGVR, "default", "my-promise")
	if err != nil {
		t.Fatalf("SetManualReconciliationLabel: %v", err)
	}

	// Verify label was set
	updated, err := c.Dynamic.Resource(KratixPromiseGVR).Namespace("default").Get(ctx, "my-promise", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get after label: %v", err)
	}
	labels := updated.GetLabels()
	if labels["kratix.io/manual-reconciliation"] != "true" {
		t.Errorf("label = %q, want %q", labels["kratix.io/manual-reconciliation"], "true")
	}
}

func TestSetReconcileAnnotation_NotFound(t *testing.T) {
	c := newFakeClientForKratix()
	ctx := context.Background()

	err := c.SetReconcileAnnotation(ctx, KratixPromiseGVR, "default", "nonexistent", "2026-01-01T00:00:00Z")
	if err == nil {
		t.Fatal("expected error for nonexistent resource")
	}
}
