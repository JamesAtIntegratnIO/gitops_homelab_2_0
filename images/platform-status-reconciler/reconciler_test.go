package main

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeVCR(phase string, createdAgo time.Duration) *unstructured.Unstructured {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":              "test-vc",
			"namespace":         "platform-requests",
			"creationTimestamp": metav1.NewTime(time.Now().Add(-createdAgo)).Format(time.RFC3339),
		},
	}}
	if phase != "" {
		vcr.Object["status"] = map[string]interface{}{
			"phase": phase,
		}
	}
	return vcr
}

func TestComputePhaseReady(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Healthy"},
			Workloads: WorkloadHealth{Ready: 5, Total: 5},
			SubApps:   SubAppHealth{Healthy: 3, Total: 3},
		},
	}
	vcr := makeVCR("", 30*time.Minute)
	phase := computePhase(result, vcr, true)
	if phase != "Ready" {
		t.Errorf("expected Ready, got %s", phase)
	}
}

func TestComputePhaseScheduled(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Unknown", HealthStatus: "Missing"},
			Workloads: WorkloadHealth{Ready: 0, Total: 0},
		},
	}
	vcr := makeVCR("", 2*time.Minute)
	phase := computePhase(result, vcr, false)
	if phase != "Scheduled" {
		t.Errorf("expected Scheduled, got %s", phase)
	}
}

func TestComputePhaseDeleting(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD: ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Healthy"},
		},
	}
	vcr := makeVCR("Deleting", 5*time.Minute)
	phase := computePhase(result, vcr, true)
	if phase != "Deleting" {
		t.Errorf("expected Deleting, got %s", phase)
	}
}

func TestComputePhaseDegraded(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Degraded"},
			Workloads: WorkloadHealth{Ready: 3, Total: 5},
		},
	}
	vcr := makeVCR("", 5*time.Minute) // younger than 15min
	phase := computePhase(result, vcr, true)
	if phase != "Degraded" {
		t.Errorf("expected Degraded, got %s", phase)
	}
}

func TestComputePhaseFailed(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Degraded"},
			Workloads: WorkloadHealth{Ready: 1, Total: 5},
		},
	}
	vcr := makeVCR("", 20*time.Minute) // older than 15min
	phase := computePhase(result, vcr, true)
	if phase != "Failed" {
		t.Errorf("expected Failed, got %s", phase)
	}
}

func TestComputePhaseProgressing(t *testing.T) {
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "OutOfSync", HealthStatus: "Healthy"},
			Workloads: WorkloadHealth{Ready: 4, Total: 5},
		},
	}
	vcr := makeVCR("", 3*time.Minute)
	phase := computePhase(result, vcr, false)
	if phase != "Progressing" {
		t.Errorf("expected Progressing, got %s", phase)
	}
}

func TestComputePhaseNoSubApps(t *testing.T) {
	// When there are no sub-apps, subApps should not block Ready
	result := &StatusResult{
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Healthy"},
			Workloads: WorkloadHealth{Ready: 3, Total: 3},
			SubApps:   SubAppHealth{Healthy: 0, Total: 0},
		},
	}
	vcr := makeVCR("", 10*time.Minute)
	phase := computePhase(result, vcr, true)
	if phase != "Ready" {
		t.Errorf("expected Ready with no sub-apps, got %s", phase)
	}
}

func TestBuildConditions(t *testing.T) {
	result := &StatusResult{
		Phase:   "Ready",
		Message: "All good",
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Healthy"},
			Workloads: WorkloadHealth{Ready: 3, Total: 3},
		},
	}
	conds := buildConditions(result, true)

	if len(conds) != 4 {
		t.Fatalf("expected 4 conditions, got %d", len(conds))
	}

	// Check Ready condition
	if conds[0].Type != "Ready" || conds[0].Status != "True" {
		t.Errorf("expected Ready=True, got %s=%s", conds[0].Type, conds[0].Status)
	}
	// Check ArgoSynced
	if conds[1].Type != "ArgoSynced" || conds[1].Status != "True" {
		t.Errorf("expected ArgoSynced=True, got %s=%s", conds[1].Type, conds[1].Status)
	}
	// Check PodsReady
	if conds[2].Type != "PodsReady" || conds[2].Status != "True" {
		t.Errorf("expected PodsReady=True, got %s=%s", conds[2].Type, conds[2].Status)
	}
	// Check KubeconfigAvailable
	if conds[3].Type != "KubeconfigAvailable" || conds[3].Status != "True" {
		t.Errorf("expected KubeconfigAvailable=True, got %s=%s", conds[3].Type, conds[3].Status)
	}
}

func TestBuildConditionsUnhealthy(t *testing.T) {
	result := &StatusResult{
		Phase:   "Degraded",
		Message: "Pods not ready",
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "OutOfSync", HealthStatus: "Degraded"},
			Workloads: WorkloadHealth{Ready: 1, Total: 5},
		},
	}
	conds := buildConditions(result, false)

	if conds[0].Status != "False" {
		t.Errorf("expected Ready=False, got %s", conds[0].Status)
	}
	if conds[1].Status != "False" {
		t.Errorf("expected ArgoSynced=False, got %s", conds[1].Status)
	}
	if conds[2].Status != "False" {
		t.Errorf("expected PodsReady=False, got %s", conds[2].Status)
	}
	if conds[3].Status != "False" {
		t.Errorf("expected KubeconfigAvailable=False, got %s", conds[3].Status)
	}
}

func TestPhaseMessage(t *testing.T) {
	tests := []struct {
		phase, name string
		wantPrefix  string
	}{
		{"Ready", "media", "VCluster media is fully operational"},
		{"Scheduled", "dev", "VCluster dev resources have been scheduled"},
		{"Progressing", "test", "VCluster test is being provisioned"},
		{"Degraded", "test", "VCluster test is running but"},
		{"Failed", "test", "VCluster test has failed"},
		{"Deleting", "test", "VCluster test is being deleted"},
		{"Unknown", "test", "VCluster test is in an unknown state"},
	}
	for _, tt := range tests {
		msg := phaseMessage(tt.phase, tt.name)
		if !containsHelper(msg, tt.wantPrefix[:10]) {
			t.Errorf("phaseMessage(%q, %q) = %q, want contains %q", tt.phase, tt.name, msg, tt.wantPrefix)
		}
	}
}

func TestAggregateAppHealth(t *testing.T) {
	apps := []unstructured.Unstructured{
		{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "app1"},
			"status": map[string]interface{}{
				"health": map[string]interface{}{"status": "Healthy"},
			},
		}},
		{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "app2"},
			"status": map[string]interface{}{
				"health": map[string]interface{}{"status": "Degraded"},
			},
		}},
		{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "app3"},
			"status": map[string]interface{}{
				"health": map[string]interface{}{"status": "Healthy"},
			},
		}},
	}

	result := aggregateAppHealth(apps)
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.Healthy != 2 {
		t.Errorf("Healthy = %d, want 2", result.Healthy)
	}
	if len(result.Unhealthy) != 1 || result.Unhealthy[0] != "app2" {
		t.Errorf("Unhealthy = %v, want [app2]", result.Unhealthy)
	}
}

func TestPhaseFromArgoCD(t *testing.T) {
	tests := []struct {
		sync, health, want string
	}{
		{"Synced", "Healthy", "Ready"},
		{"Synced", "Degraded", "Degraded"},
		{"Unknown", "Missing", "Unknown"},
		{"OutOfSync", "Healthy", "Progressing"},
		{"Synced", "Progressing", "Progressing"},
		{"Synced", "Suspended", "Suspended"},
	}
	for _, tt := range tests {
		got := phaseFromArgoCD(tt.sync, tt.health)
		if got != tt.want {
			t.Errorf("phaseFromArgoCD(%q, %q) = %q, want %q", tt.sync, tt.health, got, tt.want)
		}
	}
}

func TestExtractAppStatus(t *testing.T) {
	app := unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "sonarr-vcluster-media",
			"labels": map[string]interface{}{
				"addon":       "true",
				"addonName":   "sonarr",
				"clusterName": "vcluster-media",
				"environment": "production",
			},
		},
		"spec": map[string]interface{}{
			"destination": map[string]interface{}{"namespace": "media"},
		},
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "Synced"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}}

	status := extractAppStatus(app)
	if status.Name != "sonarr-vcluster-media" {
		t.Errorf("Name = %q", status.Name)
	}
	if status.AddonName != "sonarr" {
		t.Errorf("AddonName = %q", status.AddonName)
	}
	if status.ClusterName != "vcluster-media" {
		t.Errorf("ClusterName = %q", status.ClusterName)
	}
	if status.Phase != "Ready" {
		t.Errorf("Phase = %q", status.Phase)
	}
	if status.Namespace != "media" {
		t.Errorf("Namespace = %q", status.Namespace)
	}
}

func TestNewCondition(t *testing.T) {
	cond := NewCondition("Ready", "True", "AllHealthy", "test msg")
	if cond.Type != "Ready" || cond.Status != "True" || cond.Reason != "AllHealthy" {
		t.Errorf("unexpected condition: %+v", cond)
	}
	if cond.LastTransitionTime == "" {
		t.Error("expected LastTransitionTime to be set")
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello", "hello", true},
		{"", "x", false},
		{"abc", "abcd", false},
		{"x.y.z", "y.z", true},
	}
	for _, tt := range tests {
		got := containsHelper(tt.s, tt.sub)
		if got != tt.want {
			t.Errorf("containsHelper(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}
