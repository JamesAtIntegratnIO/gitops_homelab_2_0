package platform

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestArgoAppToResourceStatus_FullyPopulated(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      "media-sonarr",
				"namespace": "argocd",
				"labels": map[string]interface{}{
					"addon":       "true",
					"addonName":   "sonarr",
					"clusterName": "media",
				},
			},
			"spec": map[string]interface{}{
				"project": "media",
				"destination": map[string]interface{}{
					"namespace": "media",
				},
			},
			"status": map[string]interface{}{
				"sync": map[string]interface{}{
					"status": "Synced",
				},
				"health": map[string]interface{}{
					"status": "Healthy",
				},
			},
		},
	}

	rs := argoAppToResourceStatus(app)
	if rs.Name != "sonarr" {
		t.Errorf("Name = %q, want 'sonarr'", rs.Name)
	}
	if rs.Namespace != "media" {
		t.Errorf("Namespace = %q, want 'media'", rs.Namespace)
	}
	if rs.ArgoCD.AppName != "media-sonarr" {
		t.Errorf("ArgoCD.AppName = %q, want 'media-sonarr'", rs.ArgoCD.AppName)
	}
	if rs.ArgoCD.SyncStatus != "Synced" {
		t.Errorf("SyncStatus = %q, want 'Synced'", rs.ArgoCD.SyncStatus)
	}
	if rs.ArgoCD.HealthStatus != "Healthy" {
		t.Errorf("HealthStatus = %q, want 'Healthy'", rs.ArgoCD.HealthStatus)
	}
	if rs.ArgoCD.Project != "media" {
		t.Errorf("Project = %q, want 'media'", rs.ArgoCD.Project)
	}
	if rs.Phase != "Ready" {
		t.Errorf("Phase = %q, want 'Ready'", rs.Phase)
	}
}

func TestArgoAppToResourceStatus_MissingStatus(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      "test-app",
				"namespace": "argocd",
				"labels": map[string]interface{}{
					"addon":     "true",
					"addonName": "test",
				},
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{},
			},
		},
	}

	rs := argoAppToResourceStatus(app)
	if rs.ArgoCD.SyncStatus != "Unknown" {
		t.Errorf("SyncStatus = %q, want 'Unknown' when missing", rs.ArgoCD.SyncStatus)
	}
	if rs.ArgoCD.HealthStatus != "Unknown" {
		t.Errorf("HealthStatus = %q, want 'Unknown' when missing", rs.ArgoCD.HealthStatus)
	}
	if rs.Phase != "Unknown" {
		t.Errorf("Phase = %q, want 'Unknown' for missing statuses", rs.Phase)
	}
}

func TestArgoAppToResourceStatus_Degraded(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":   "degraded-app",
				"labels": map[string]interface{}{"addonName": "broken"},
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{"namespace": "test"},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "Synced"},
				"health": map[string]interface{}{"status": "Degraded"},
			},
		},
	}

	rs := argoAppToResourceStatus(app)
	if rs.Phase != "Degraded" {
		t.Errorf("Phase = %q, want 'Degraded'", rs.Phase)
	}
}

func TestArgoAppToResourceStatus_OutOfSync(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":   "oos-app",
				"labels": map[string]interface{}{"addonName": "drifted"},
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{"namespace": "dev"},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "OutOfSync"},
				"health": map[string]interface{}{"status": "Healthy"},
			},
		},
	}

	rs := argoAppToResourceStatus(app)
	if rs.Phase != "Progressing" {
		t.Errorf("Phase = %q, want 'Progressing' for OutOfSync", rs.Phase)
	}
}

func TestArgoAppToResourceStatus_NoLabels(t *testing.T) {
	app := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "no-labels",
			},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "Synced"},
				"health": map[string]interface{}{"status": "Healthy"},
			},
		},
	}

	rs := argoAppToResourceStatus(app)
	// Name comes from addonName label which is empty
	if rs.Name != "" {
		t.Errorf("Name = %q, want empty when no addonName label", rs.Name)
	}
	if rs.Phase != "Ready" {
		t.Errorf("Phase = %q, want 'Ready'", rs.Phase)
	}
}
