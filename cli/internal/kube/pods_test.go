package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestExtractRoles_ControlPlane(t *testing.T) {
	labels := map[string]string{
		"node-role.kubernetes.io/control-plane": "",
		"kubernetes.io/os":                      "linux",
	}
	roles := extractRoles(labels)
	if len(roles) != 1 || roles[0] != "control-plane" {
		t.Errorf("extractRoles = %v, want [control-plane]", roles)
	}
}

func TestExtractRoles_MultipleRoles(t *testing.T) {
	labels := map[string]string{
		"node-role.kubernetes.io/control-plane": "",
		"node-role.kubernetes.io/worker":        "",
	}
	roles := extractRoles(labels)
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d: %v", len(roles), roles)
	}
}

func TestExtractRoles_NoRoles(t *testing.T) {
	labels := map[string]string{"kubernetes.io/os": "linux"}
	roles := extractRoles(labels)
	if len(roles) != 1 || roles[0] != "<none>" {
		t.Errorf("extractRoles = %v, want [<none>]", roles)
	}
}

func TestExtractRoles_EmptyLabels(t *testing.T) {
	roles := extractRoles(nil)
	if len(roles) != 1 || roles[0] != "<none>" {
		t.Errorf("extractRoles(nil) = %v, want [<none>]", roles)
	}
}

func TestListPods_MultipleContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-container",
			Namespace: "default",
			Labels:    map[string]string{"app": "multi"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1"},
				{Name: "sidecar", Image: "envoy:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "main", Ready: true},
				{Name: "sidecar", Ready: false},
			},
		},
	}

	c := newFakeClient([]runtime.Object{pod})
	ctx := context.Background()

	pods, err := c.ListPods(ctx, "default", "app=multi")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].ReadyContainers != 1 {
		t.Errorf("ReadyContainers = %d, want 1", pods[0].ReadyContainers)
	}
	if pods[0].TotalContainers != 2 {
		t.Errorf("TotalContainers = %d, want 2", pods[0].TotalContainers)
	}
}

func TestGetPodResourceInfo_MultipleRestarts(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crash-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "crash"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1"},
				{Name: "sidecar", Image: "envoy:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "main", RestartCount: 3},
				{Name: "sidecar", RestartCount: 1},
			},
		},
	}

	c := newFakeClient([]runtime.Object{pod})
	ctx := context.Background()

	infos, err := c.GetPodResourceInfo(ctx, "default", "app=crash")
	if err != nil {
		t.Fatalf("GetPodResourceInfo: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].Restarts != 4 {
		t.Errorf("Restarts = %d, want 4", infos[0].Restarts)
	}
}

func TestListPods_EmptyNamespace(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	pods, err := c.ListPods(ctx, "empty", "")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(pods))
	}
}

func TestListNodes_AllocatableResources(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.5"},
			},
			Allocatable: corev1.ResourceList{
				"cpu":    mustParseQuantity("4"),
				"memory": mustParseQuantity("8Gi"),
			},
		},
	}

	c := newFakeClient([]runtime.Object{node})
	ctx := context.Background()

	nodes, err := c.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Ready {
		t.Error("expected node to NOT be ready")
	}
	if nodes[0].IP != "10.0.0.5" {
		t.Errorf("IP = %q, want 10.0.0.5", nodes[0].IP)
	}
	if nodes[0].CPU != "4" {
		t.Errorf("CPU = %q, want 4", nodes[0].CPU)
	}
	if nodes[0].Memory != "8Gi" {
		t.Errorf("Memory = %q, want 8Gi", nodes[0].Memory)
	}
}
