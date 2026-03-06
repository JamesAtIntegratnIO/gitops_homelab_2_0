package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newFakeClient creates a Client backed by fake k8s interfaces for testing.
func newFakeClient(objs []runtime.Object, dynObjs ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "ApplicationList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "platform.kratix.io",
		Version: "v1alpha1",
		Kind:    "Promise",
	}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "platform.kratix.io",
		Version: "v1alpha1",
		Kind:    "PromiseList",
	}, &unstructured.UnstructuredList{})

	return &Client{
		Clientset: kubefake.NewSimpleClientset(objs...),
		Dynamic:   dynamicfake.NewSimpleDynamicClient(scheme, dynObjs...),
	}
}

func makeArgoApp(name, namespace, syncStatus, healthStatus string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{
					"name": "in-cluster",
				},
				"source": map[string]interface{}{
					"targetRevision": "HEAD",
				},
			},
			"status": map[string]interface{}{
				"sync": map[string]interface{}{
					"status": syncStatus,
				},
				"health": map[string]interface{}{
					"status": healthStatus,
				},
			},
		},
	}
}

func TestListArgoApps(t *testing.T) {
	app1 := makeArgoApp("app1", "argocd", "Synced", "Healthy")
	app2 := makeArgoApp("app2", "argocd", "OutOfSync", "Degraded")

	c := newFakeClient(nil, app1, app2)
	ctx := context.Background()

	apps, err := c.ListArgoApps(ctx, "argocd")
	if err != nil {
		t.Fatalf("ListArgoApps: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestListArgoApps_EmptyNamespace(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	apps, err := c.ListArgoApps(ctx, "argocd")
	if err != nil {
		t.Fatalf("ListArgoApps: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestGetArgoApp(t *testing.T) {
	app := makeArgoApp("my-app", "argocd", "Synced", "Healthy")
	c := newFakeClient(nil, app)
	ctx := context.Background()

	got, err := c.GetArgoApp(ctx, "argocd", "my-app")
	if err != nil {
		t.Fatalf("GetArgoApp: %v", err)
	}
	if got.GetName() != "my-app" {
		t.Errorf("expected name my-app, got %s", got.GetName())
	}
}

func TestGetArgoApp_NotFound(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	_, err := c.GetArgoApp(ctx, "argocd", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestListArgoAppsForCluster(t *testing.T) {
	app1 := makeArgoApp("app1", "argocd", "Synced", "Healthy")
	// app2 has different destination
	app2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      "app2",
				"namespace": "argocd",
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{
					"name": "other-cluster",
				},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "Synced"},
				"health": map[string]interface{}{"status": "Healthy"},
			},
		},
	}

	c := newFakeClient(nil, app1, app2)
	ctx := context.Background()

	result, err := c.ListArgoAppsForCluster(ctx, "argocd", "in-cluster")
	if err != nil {
		t.Fatalf("ListArgoAppsForCluster: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 app for in-cluster, got %d", len(result))
	}
	if result[0].Name != "app1" {
		t.Errorf("expected app1, got %s", result[0].Name)
	}
	if result[0].SyncStatus != "Synced" {
		t.Errorf("expected Synced, got %s", result[0].SyncStatus)
	}
}

func TestListDeployments(t *testing.T) {
	replicas := int32(3)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Annotations: map[string]string{
				"argocd.argoproj.io/tracking-id": "my-app:apps/Deployment:default/nginx",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "nginx"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "nginx"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
			},
		},
	}

	c := newFakeClient([]runtime.Object{deploy})
	ctx := context.Background()

	result, err := c.ListDeployments(ctx, "default")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(result))
	}
	if result[0].Name != "nginx" {
		t.Errorf("expected nginx, got %s", result[0].Name)
	}
	if result[0].Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", result[0].Replicas)
	}
	if result[0].ArgoApp != "my-app" {
		t.Errorf("expected ArgoApp my-app, got %s", result[0].ArgoApp)
	}
}

func TestGetSecretData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("s3cret"),
			"token":    []byte("tok123"),
		},
	}

	c := newFakeClient([]runtime.Object{secret})
	ctx := context.Background()

	data, err := c.GetSecretData(ctx, "default", "my-secret")
	if err != nil {
		t.Fatalf("GetSecretData: %v", err)
	}
	if string(data["password"]) != "s3cret" {
		t.Errorf("expected password=s3cret, got %s", string(data["password"]))
	}
	if string(data["token"]) != "tok123" {
		t.Errorf("expected token=tok123, got %s", string(data["token"]))
	}
}

func TestGetSecretData_NotFound(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	_, err := c.GetSecretData(ctx, "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
}

func TestListNodes(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
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
	if nodes[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", nodes[0].Name)
	}
	if !nodes[0].Ready {
		t.Error("expected node to be ready")
	}
	if nodes[0].IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", nodes[0].IP)
	}
}

func TestDisableArgoAutoSync(t *testing.T) {
	app := makeArgoApp("my-app", "argocd", "Synced", "Healthy")
	c := newFakeClient(nil, app)
	ctx := context.Background()

	err := c.DisableArgoAutoSync(ctx, "argocd", "my-app")
	if err != nil {
		t.Fatalf("DisableArgoAutoSync: %v", err)
	}

	// Verify the patch was applied
	updated, err := c.GetArgoApp(ctx, "argocd", "my-app")
	if err != nil {
		t.Fatalf("GetArgoApp after patch: %v", err)
	}
	// After merge patch with null syncPolicy, it should be removed
	spec := updated.Object["spec"].(map[string]interface{})
	if sp, exists := spec["syncPolicy"]; exists && sp != nil {
		t.Errorf("expected syncPolicy to be nil, got %v", sp)
	}
}

func TestTriggerArgoAppSync(t *testing.T) {
	app := makeArgoApp("my-app", "argocd", "Synced", "Healthy")
	c := newFakeClient(nil, app)
	ctx := context.Background()

	err := c.TriggerArgoAppSync(ctx, "argocd", "my-app")
	if err != nil {
		t.Fatalf("TriggerArgoAppSync: %v", err)
	}

	// Verify operation was set
	updated, err := c.GetArgoApp(ctx, "argocd", "my-app")
	if err != nil {
		t.Fatalf("GetArgoApp after sync: %v", err)
	}
	op, exists := updated.Object["operation"]
	if !exists {
		t.Fatal("expected operation field after sync trigger")
	}
	opMap := op.(map[string]interface{})
	if _, ok := opMap["sync"]; !ok {
		t.Error("expected sync in operation")
	}
}

func TestListPods(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-abc123",
			Namespace: "default",
			Labels:    map[string]string{"app": "web"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "web", Image: "nginx:1.25"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "web", Ready: true},
			},
		},
	}

	c := newFakeClient([]runtime.Object{pod})
	ctx := context.Background()

	pods, err := c.ListPods(ctx, "default", "app=web")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Name != "web-abc123" {
		t.Errorf("expected pod name web-abc123, got %s", pods[0].Name)
	}
	if pods[0].Phase != "Running" {
		t.Errorf("expected Running phase, got %s", pods[0].Phase)
	}
	if pods[0].ReadyContainers != 1 {
		t.Errorf("expected 1 ready container, got %d", pods[0].ReadyContainers)
	}
}

func TestGetPodResourceInfo(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "jobs",
			Labels:    map[string]string{"app": "worker"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "worker",
					Image: "worker:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu":    mustParseQuantity("100m"),
							"memory": mustParseQuantity("128Mi"),
						},
						Limits: corev1.ResourceList{
							"cpu":    mustParseQuantity("500m"),
							"memory": mustParseQuantity("256Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	c := newFakeClient([]runtime.Object{pod})
	ctx := context.Background()

	infos, err := c.GetPodResourceInfo(ctx, "jobs", "app=worker")
	if err != nil {
		t.Fatalf("GetPodResourceInfo: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 pod info, got %d", len(infos))
	}
	if infos[0].CPURequest != "100m" {
		t.Errorf("CPURequest = %q, want '100m'", infos[0].CPURequest)
	}
	if infos[0].MemoryLimit != "256Mi" {
		t.Errorf("MemoryLimit = %q, want '256Mi'", infos[0].MemoryLimit)
	}
}

func TestListPromises(t *testing.T) {
	promise := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kratix.io/v1alpha1",
			"kind":       "Promise",
			"metadata": map[string]interface{}{
				"name": "postgres-instance",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Available",
						"status": "True",
					},
				},
			},
		},
	}

	c := newFakeClient(nil, promise)
	ctx := context.Background()

	promises, err := c.ListPromises(ctx)
	if err != nil {
		t.Fatalf("ListPromises: %v", err)
	}
	if len(promises) != 1 {
		t.Fatalf("expected 1 promise, got %d", len(promises))
	}
	if promises[0].GetName() != "postgres-instance" {
		t.Errorf("expected name postgres-instance, got %s", promises[0].GetName())
	}
}

func TestParseArgoAppStatus(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "Synced"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}
	sync, health := ParseArgoAppStatus(obj)
	if sync != "Synced" {
		t.Errorf("sync = %q, want 'Synced'", sync)
	}
	if health != "Healthy" {
		t.Errorf("health = %q, want 'Healthy'", health)
	}

	// Missing status fields
	sync2, health2 := ParseArgoAppStatus(map[string]interface{}{})
	if sync2 != "" || health2 != "" {
		t.Errorf("expected empty strings for missing status, got sync=%q, health=%q", sync2, health2)
	}
}

func TestParsePromiseStatus(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]interface{}
		want string
	}{
		{
			name: "available",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Available", "status": "True"},
					},
				},
			},
			want: "Available",
		},
		{
			name: "unavailable",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Available", "status": "False"},
					},
				},
			},
			want: "Unavailable",
		},
		{
			name: "no conditions",
			obj:  map[string]interface{}{},
			want: "Unknown",
		},
		{
			name: "no available condition",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Ready", "status": "True"},
					},
				},
			},
			want: "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePromiseStatus(tt.obj)
			if got != tt.want {
				t.Errorf("ParsePromiseStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}
