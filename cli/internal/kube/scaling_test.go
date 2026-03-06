package kube

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func makeDeployment(name, namespace string, replicas int32, annotations map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: name + ":latest"}}},
			},
		},
	}
}

func TestListDeployments_MultipleDeployments(t *testing.T) {
	d1 := makeDeployment("api", "default", 3, map[string]string{
		"argocd.argoproj.io/tracking-id": "app1:apps/Deployment:default/api",
	})
	d2 := makeDeployment("worker", "default", 2, nil)

	c := newFakeClient([]runtime.Object{d1, d2})
	ctx := context.Background()

	result, err := c.ListDeployments(ctx, "default")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(result))
	}

	// Verify ArgoApp parsed for first, empty for second
	var found map[string]DeploymentInfo = make(map[string]DeploymentInfo)
	for _, d := range result {
		found[d.Name] = d
	}
	if found["api"].ArgoApp != "app1" {
		t.Errorf("api ArgoApp = %q, want app1", found["api"].ArgoApp)
	}
	if found["worker"].ArgoApp != "" {
		t.Errorf("worker ArgoApp = %q, want empty", found["worker"].ArgoApp)
	}
}

func TestListDeployments_NoArgoAnnotation(t *testing.T) {
	deploy := makeDeployment("simple", "default", 2, nil)

	c := newFakeClient([]runtime.Object{deploy})
	ctx := context.Background()

	result, err := c.ListDeployments(ctx, "default")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(result))
	}
	if result[0].ArgoApp != "" {
		t.Errorf("ArgoApp = %q, want empty for no annotation", result[0].ArgoApp)
	}
	if result[0].Replicas != 2 {
		t.Errorf("Replicas = %d, want 2", result[0].Replicas)
	}
}

func TestListDeployments_WithArgoTracking(t *testing.T) {
	deploy := makeDeployment("api", "production", 3, map[string]string{
		"argocd.argoproj.io/tracking-id": "my-app:apps/Deployment:production/api",
	})

	c := newFakeClient([]runtime.Object{deploy})
	ctx := context.Background()

	result, err := c.ListDeployments(ctx, "production")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(result))
	}
	if result[0].ArgoApp != "my-app" {
		t.Errorf("ArgoApp = %q, want my-app", result[0].ArgoApp)
	}
}

func TestListDeployments_EmptyNamespace(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	result, err := c.ListDeployments(ctx, "empty-ns")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 deployments, got %d", len(result))
	}
}
