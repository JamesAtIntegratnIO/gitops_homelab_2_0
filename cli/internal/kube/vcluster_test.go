package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newFakeClientForVCluster(dynObjs ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "platform.integratn.tech", Version: "v1alpha1", Kind: "VClusterOrchestratorV2",
	}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "platform.integratn.tech", Version: "v1alpha1", Kind: "VClusterOrchestratorV2List",
	}, &unstructured.UnstructuredList{})

	return &Client{
		Dynamic: dynamicfake.NewSimpleDynamicClient(scheme, dynObjs...),
	}
}

func makeVClusterObj(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.integratn.tech/v1alpha1",
			"kind":       "VClusterOrchestratorV2",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"status": map[string]interface{}{
				"phase":   "Ready",
				"message": "All systems operational",
			},
		},
	}
}

func TestListVClusters(t *testing.T) {
	vc1 := makeVClusterObj("media", "platform-requests")
	vc2 := makeVClusterObj("staging", "platform-requests")

	c := newFakeClientForVCluster(vc1, vc2)
	ctx := context.Background()

	vclusters, err := c.ListVClusters(ctx, "platform-requests")
	if err != nil {
		t.Fatalf("ListVClusters: %v", err)
	}
	if len(vclusters) != 2 {
		t.Errorf("expected 2 vclusters, got %d", len(vclusters))
	}
}

func TestListVClusters_EmptyNamespace(t *testing.T) {
	c := newFakeClientForVCluster()
	ctx := context.Background()

	vclusters, err := c.ListVClusters(ctx, "empty")
	if err != nil {
		t.Fatalf("ListVClusters: %v", err)
	}
	if len(vclusters) != 0 {
		t.Errorf("expected 0 vclusters, got %d", len(vclusters))
	}
}

func TestGetVCluster(t *testing.T) {
	vc := makeVClusterObj("media", "platform-requests")

	c := newFakeClientForVCluster(vc)
	ctx := context.Background()

	result, err := c.GetVCluster(ctx, "platform-requests", "media")
	if err != nil {
		t.Fatalf("GetVCluster: %v", err)
	}
	if result.GetName() != "media" {
		t.Errorf("name = %q, want media", result.GetName())
	}

	// Verify status fields are accessible
	phase, _, _ := unstructured.NestedString(result.Object, "status", "phase")
	if phase != "Ready" {
		t.Errorf("phase = %q, want Ready", phase)
	}
}

func TestGetVCluster_NotFound(t *testing.T) {
	c := newFakeClientForVCluster()
	ctx := context.Background()

	_, err := c.GetVCluster(ctx, "platform-requests", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent vcluster")
	}
}
