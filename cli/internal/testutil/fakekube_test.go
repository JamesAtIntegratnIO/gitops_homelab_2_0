package testutil

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

func TestFakeKubeClient_GetVCluster_Static(t *testing.T) {
	vc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "media"},
		},
	}
	f := &FakeKubeClient{VCluster: vc}

	got, err := f.GetVCluster(context.Background(), "ns", "media")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetName() != "media" {
		t.Errorf("name = %q, want media", got.GetName())
	}
}

func TestFakeKubeClient_GetVCluster_StaticError(t *testing.T) {
	f := &FakeKubeClient{VClusterErr: fmt.Errorf("not found")}

	_, err := f.GetVCluster(context.Background(), "ns", "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "not found" {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestFakeKubeClient_GetVCluster_Sequential(t *testing.T) {
	vc1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "first"},
			"status":   map[string]interface{}{"phase": "Provisioning"},
		},
	}
	vc2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "second"},
			"status":   map[string]interface{}{"phase": "Ready"},
		},
	}

	f := &FakeKubeClient{
		VCReturns: []*unstructured.Unstructured{vc1, vc2},
		VCErrs:    []error{nil, nil},
	}

	ctx := context.Background()

	// First call returns vc1
	got1, err := f.GetVCluster(ctx, "ns", "test")
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if got1.GetName() != "first" {
		t.Errorf("call 1: name = %q, want first", got1.GetName())
	}

	// Second call returns vc2
	got2, err := f.GetVCluster(ctx, "ns", "test")
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if got2.GetName() != "second" {
		t.Errorf("call 2: name = %q, want second", got2.GetName())
	}

	// Third call repeats last (vc2)
	got3, err := f.GetVCluster(ctx, "ns", "test")
	if err != nil {
		t.Fatalf("call 3: %v", err)
	}
	if got3.GetName() != "second" {
		t.Errorf("call 3: name = %q, want second (last repeat)", got3.GetName())
	}
}

func TestFakeKubeClient_ListArgoApps_Sequential(t *testing.T) {
	app1 := unstructured.Unstructured{
		Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "app1"}},
	}
	app2 := unstructured.Unstructured{
		Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "app2"}},
	}

	f := &FakeKubeClient{
		ArgoAppsResults: []ListArgoAppsResult{
			{Items: []unstructured.Unstructured{app1}, Err: nil},
			{Items: []unstructured.Unstructured{app1, app2}, Err: nil},
		},
	}

	ctx := context.Background()

	// First call returns 1 app
	apps1, err := f.ListArgoApps(ctx, "argocd")
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if len(apps1) != 1 {
		t.Errorf("call 1: got %d apps, want 1", len(apps1))
	}

	// Second call returns 2 apps
	apps2, err := f.ListArgoApps(ctx, "argocd")
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if len(apps2) != 2 {
		t.Errorf("call 2: got %d apps, want 2", len(apps2))
	}
}

func TestFakeKubeClient_ListArgoApps_Static(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "app1"}},
	}
	f := &FakeKubeClient{ArgoApps: []unstructured.Unstructured{app}}

	apps, err := f.ListArgoApps(context.Background(), "argocd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("got %d apps, want 1", len(apps))
	}
}

func TestFakeKubeClient_ListPods_Sequential(t *testing.T) {
	f := &FakeKubeClient{
		PodsResults: []ListPodsResult{
			{Items: nil, Err: fmt.Errorf("connection refused")},
			{Items: []kube.PodInfo{{Name: "pod-1", Phase: "Running"}}, Err: nil},
		},
	}

	ctx := context.Background()

	// First call fails
	_, err := f.ListPods(ctx, "ns", "")
	if err == nil {
		t.Fatal("call 1: expected error")
	}

	// Second call succeeds
	pods, err := f.ListPods(ctx, "ns", "")
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if len(pods) != 1 || pods[0].Name != "pod-1" {
		t.Errorf("call 2: got %v, want [pod-1]", pods)
	}
}

func TestFakeKubeClient_DefaultNilReturns(t *testing.T) {
	f := &FakeKubeClient{}
	ctx := context.Background()

	// All methods return nil/empty by default without error
	vc, err := f.GetVCluster(ctx, "", "")
	if err != nil {
		t.Errorf("GetVCluster err: %v", err)
	}
	if vc != nil {
		t.Errorf("GetVCluster: expected nil, got %v", vc)
	}

	nodes, err := f.ListNodes(ctx)
	if err != nil {
		t.Errorf("ListNodes err: %v", err)
	}
	if nodes != nil {
		t.Errorf("ListNodes: expected nil, got %v", nodes)
	}

	promises, err := f.ListPromises(ctx)
	if err != nil {
		t.Errorf("ListPromises err: %v", err)
	}
	if promises != nil {
		t.Errorf("ListPromises: expected nil, got %v", promises)
	}

	jobs, err := f.ListJobs(ctx, "", "")
	if err != nil {
		t.Errorf("ListJobs err: %v", err)
	}
	if jobs != nil {
		t.Errorf("ListJobs: expected nil, got %v", jobs)
	}
}
