package platform

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseStatusContract_FullStatus(t *testing.T) {
	vc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase":          "Ready",
				"message":        "All systems go",
				"lastReconciled": "2026-03-01T12:00:00Z",
				"endpoints": map[string]interface{}{
					"api":    "https://media.cluster.integratn.tech",
					"argocd": "https://argocd.media.cluster.integratn.tech",
				},
				"credentials": map[string]interface{}{
					"kubeconfigSecret": "media-kubeconfig",
					"onePasswordItem":  "media-vcluster",
				},
				"health": map[string]interface{}{
					"argocd": map[string]interface{}{
						"syncStatus":   "Synced",
						"healthStatus": "Healthy",
					},
					"workloads": map[string]interface{}{
						"ready": int64(5),
						"total": int64(5),
					},
					"subApps": map[string]interface{}{
						"healthy":   int64(3),
						"total":     int64(4),
						"unhealthy": []interface{}{"sonarr"},
					},
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "AllHealthy",
						"message":            "All checks passed",
						"lastTransitionTime": "2026-03-01T12:00:00Z",
					},
				},
			},
		},
	}

	sc, err := parseStatusContract(vc)
	if err != nil {
		t.Fatalf("parseStatusContract: %v", err)
	}

	// Top-level fields
	if sc.Phase != "Ready" {
		t.Errorf("Phase = %q, want Ready", sc.Phase)
	}
	if sc.Message != "All systems go" {
		t.Errorf("Message = %q, want 'All systems go'", sc.Message)
	}
	if sc.LastReconciled != "2026-03-01T12:00:00Z" {
		t.Errorf("LastReconciled = %q", sc.LastReconciled)
	}

	// Endpoints
	if sc.Endpoints.API != "https://media.cluster.integratn.tech" {
		t.Errorf("Endpoints.API = %q", sc.Endpoints.API)
	}
	if sc.Endpoints.ArgoCD != "https://argocd.media.cluster.integratn.tech" {
		t.Errorf("Endpoints.ArgoCD = %q", sc.Endpoints.ArgoCD)
	}

	// Credentials
	if sc.Credentials.KubeconfigSecret != "media-kubeconfig" {
		t.Errorf("Credentials.KubeconfigSecret = %q", sc.Credentials.KubeconfigSecret)
	}
	if sc.Credentials.OnePasswordItem != "media-vcluster" {
		t.Errorf("Credentials.OnePasswordItem = %q", sc.Credentials.OnePasswordItem)
	}

	// Health
	if sc.Health.ArgoCDSync != "Synced" {
		t.Errorf("Health.ArgoCDSync = %q", sc.Health.ArgoCDSync)
	}
	if sc.Health.ArgoCDHealth != "Healthy" {
		t.Errorf("Health.ArgoCDHealth = %q", sc.Health.ArgoCDHealth)
	}
	if sc.Health.PodsReady != 5 {
		t.Errorf("Health.PodsReady = %d, want 5", sc.Health.PodsReady)
	}
	if sc.Health.PodsTotal != 5 {
		t.Errorf("Health.PodsTotal = %d, want 5", sc.Health.PodsTotal)
	}
	if sc.Health.SubAppsHealthy != 3 {
		t.Errorf("Health.SubAppsHealthy = %d, want 3", sc.Health.SubAppsHealthy)
	}
	if sc.Health.SubAppsTotal != 4 {
		t.Errorf("Health.SubAppsTotal = %d, want 4", sc.Health.SubAppsTotal)
	}
	if len(sc.Health.SubAppsUnhealthy) != 1 || sc.Health.SubAppsUnhealthy[0] != "sonarr" {
		t.Errorf("Health.SubAppsUnhealthy = %v, want [sonarr]", sc.Health.SubAppsUnhealthy)
	}

	// Conditions
	if len(sc.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(sc.Conditions))
	}
	cond := sc.Conditions[0]
	if cond.Type != "Ready" {
		t.Errorf("Condition.Type = %q, want Ready", cond.Type)
	}
	if cond.Status != "True" {
		t.Errorf("Condition.Status = %q, want True", cond.Status)
	}
	if cond.Reason != "AllHealthy" {
		t.Errorf("Condition.Reason = %q, want AllHealthy", cond.Reason)
	}
	if cond.Message != "All checks passed" {
		t.Errorf("Condition.Message = %q", cond.Message)
	}
}

func TestParseStatusContract_EmptyStatus(t *testing.T) {
	vc := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	sc, err := parseStatusContract(vc)
	if err != nil {
		t.Fatalf("parseStatusContract: %v", err)
	}

	if sc.Phase != "" {
		t.Errorf("Phase = %q, want empty", sc.Phase)
	}
	if sc.Message != "" {
		t.Errorf("Message = %q, want empty", sc.Message)
	}
	if sc.Endpoints.API != "" {
		t.Errorf("Endpoints.API = %q, want empty", sc.Endpoints.API)
	}
	if sc.Health.PodsReady != 0 {
		t.Errorf("Health.PodsReady = %d, want 0", sc.Health.PodsReady)
	}
	if len(sc.Conditions) != 0 {
		t.Errorf("expected 0 conditions, got %d", len(sc.Conditions))
	}
}

func TestParseStatusContract_PartialStatus(t *testing.T) {
	vc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase":   "Provisioning",
				"message": "Waiting for pods",
			},
		},
	}

	sc, err := parseStatusContract(vc)
	if err != nil {
		t.Fatalf("parseStatusContract: %v", err)
	}

	if sc.Phase != "Provisioning" {
		t.Errorf("Phase = %q, want Provisioning", sc.Phase)
	}
	if sc.Endpoints.API != "" {
		t.Errorf("Endpoints.API = %q, want empty for partial status", sc.Endpoints.API)
	}
	if sc.Health.ArgoCDSync != "" {
		t.Errorf("Health.ArgoCDSync = %q, want empty for partial status", sc.Health.ArgoCDSync)
	}
	if sc.Health.SubAppsUnhealthy != nil {
		t.Errorf("Health.SubAppsUnhealthy = %v, want nil for partial status", sc.Health.SubAppsUnhealthy)
	}
}
