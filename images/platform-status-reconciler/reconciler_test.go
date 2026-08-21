package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// --- write-loop regression tests -------------------------------------------
//
// These cover the defect found on 2026-08-21: the reconciler rewrote .status on
// every 60s pass even in a steady state, which woke Kratix's watch and fed a
// two-controller write loop running at ~9.5 writes/second.

func TestCarryTransitionTimeKeepsTimestampWhenStatusUnchanged(t *testing.T) {
	existing := []Condition{
		{Type: "Ready", Status: "True", LastTransitionTime: "2026-08-01T00:00:00Z"},
	}
	fresh := []Condition{
		{Type: "Ready", Status: "True", LastTransitionTime: "2026-08-21T16:34:18Z"},
	}
	got := CarryTransitionTime(fresh, existing)
	if got[0].LastTransitionTime != "2026-08-01T00:00:00Z" {
		t.Errorf("steady-state condition re-stamped: got %q, want the original timestamp",
			got[0].LastTransitionTime)
	}
}

func TestCarryTransitionTimeStampsOnRealTransition(t *testing.T) {
	existing := []Condition{
		{Type: "Ready", Status: "True", LastTransitionTime: "2026-08-01T00:00:00Z"},
	}
	fresh := []Condition{
		{Type: "Ready", Status: "False", LastTransitionTime: "2026-08-21T16:34:18Z"},
	}
	got := CarryTransitionTime(fresh, existing)
	if got[0].LastTransitionTime != "2026-08-21T16:34:18Z" {
		t.Errorf("a real True->False transition must re-stamp: got %q", got[0].LastTransitionTime)
	}
}

func TestCarryTransitionTimeHandlesNewCondition(t *testing.T) {
	fresh := []Condition{{Type: "PodsReady", Status: "True", LastTransitionTime: "2026-08-21T16:34:18Z"}}
	got := CarryTransitionTime(fresh, nil)
	if len(got) != 1 || got[0].LastTransitionTime != "2026-08-21T16:34:18Z" {
		t.Errorf("a condition with no predecessor should keep its fresh timestamp: %+v", got)
	}
}

func TestMergeForeignConditionsPreservesOtherControllers(t *testing.T) {
	// Kratix owns WorksSucceeded on the same resource. A merge patch replaces the
	// conditions list wholesale, so without this the reconciler deleted it on
	// every write and Kratix put it straight back.
	ours := []Condition{
		{Type: "Ready", Status: "True"},
		{Type: "PodsReady", Status: "True"},
	}
	existing := []Condition{
		{Type: "Ready", Status: "True"},
		{Type: "WorksSucceeded", Status: "True", Reason: "WorksSucceeded"},
	}
	got := MergeForeignConditions(ours, existing)
	var found bool
	for _, c := range got {
		if c.Type == "WorksSucceeded" {
			found = true
		}
	}
	if !found {
		t.Fatalf("WorksSucceeded was dropped; got %+v", got)
	}
	if len(got) != 3 {
		t.Errorf("expected our 2 conditions plus 1 foreign, got %d: %+v", len(got), got)
	}
}

func TestMergeForeignConditionsDoesNotDuplicateOurOwn(t *testing.T) {
	ours := []Condition{{Type: "Ready", Status: "False", Reason: "New"}}
	existing := []Condition{{Type: "Ready", Status: "True", Reason: "Old"}}
	got := MergeForeignConditions(ours, existing)
	if len(got) != 1 {
		t.Fatalf("our condition must win, not duplicate: %+v", got)
	}
	if got[0].Reason != "New" {
		t.Errorf("expected our value to win, got %q", got[0].Reason)
	}
}

func TestStatusUnchangedIgnoresLastReconciled(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"phase":          "Ready",
			"lastReconciled": "2026-08-21T16:33:17Z",
		},
	}}
	next := map[string]interface{}{
		"phase":          "Ready",
		"lastReconciled": "2026-08-21T16:34:17Z",
	}
	if !statusUnchanged(vcr, next) {
		t.Error("a pass that only advances lastReconciled must not trigger a write")
	}
}

func TestStatusUnchangedDetectsRealChange(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{"phase": "Ready"},
	}}
	next := map[string]interface{}{"phase": "Degraded"}
	if statusUnchanged(vcr, next) {
		t.Error("a phase change must be written")
	}
}

func TestStatusUnchangedIgnoresFieldsWeDoNotSend(t *testing.T) {
	// The pipeline owns endpoints/credentials. A merge patch only asserts the
	// keys it carries, so their presence must not read as drift.
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"phase":     "Ready",
			"endpoints": map[string]interface{}{"api": "https://media.integratn.tech:443"},
		},
	}}
	next := map[string]interface{}{"phase": "Ready"}
	if !statusUnchanged(vcr, next) {
		t.Error("pipeline-owned fields we never send must not count as a change")
	}
}

// The defect found on 2026-08-21: with the write-loop fix deployed the
// reconciler still patched every 60s, and the only stored key that ever changed
// was lastReconciled. The patch carried "unhealthy": null for an empty list; a
// merge patch treats null as delete, so the stored object never had the key,
// the patch always did, and a byte comparison of the two could never be equal.
func TestStatusUnchangedLiveSteadyStateDoesNotWrite(t *testing.T) {
	// .status exactly as the API server stored it for vcluster-media, restricted
	// to the keys this reconciler sends. Numbers are float64, as they come back
	// from JSON; subApps has no "unhealthy" key.
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"phase":          "Ready",
			"message":        "VCluster vcluster-media is fully operational",
			"lastReconciled": "2026-08-21T18:10:50Z",
			"endpoints": map[string]interface{}{
				"api":    "https://media.integratn.tech:443",
				"argocd": "https://argocd.cluster.integratn.tech/applications/vcluster-vcluster-media",
			},
			"credentials": map[string]interface{}{
				"kubeconfigSecret": "vcluster-vcluster-media-kubeconfig",
				"onePasswordItem":  "vcluster-vcluster-media-kubeconfig",
			},
			"health": map[string]interface{}{
				"argocd":    map[string]interface{}{"healthStatus": "Healthy", "syncStatus": "Synced"},
				"subApps":   map[string]interface{}{"healthy": float64(0), "total": float64(0)},
				"workloads": map[string]interface{}{"ready": float64(37), "total": float64(37)},
			},
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True", "reason": "AllHealthy", "message": "All checks passed", "lastTransitionTime": "2026-08-21T17:01:18Z"},
				map[string]interface{}{"type": "ArgoSynced", "status": "True", "reason": "Synced", "message": "Application is synced", "lastTransitionTime": "2026-08-21T16:53:18Z"},
				map[string]interface{}{"type": "PodsReady", "status": "True", "reason": "AllPodsRunning", "message": "37/37 pods ready", "lastTransitionTime": "2026-08-21T17:01:18Z"},
				map[string]interface{}{"type": "KubeconfigAvailable", "status": "True", "reason": "SecretExists", "message": "Kubeconfig secret exists", "lastTransitionTime": "2026-08-21T16:53:18Z"},
				map[string]interface{}{"type": "WorksSucceeded", "status": "True", "reason": "WorksSucceeded", "message": "All works succeeded", "lastTransitionTime": "2026-08-21T16:53:18Z"},
			},
		},
	}}

	// What the next 60s pass computes for the same, unchanged cluster.
	var ours []Condition
	for _, c := range readConditions(vcr) {
		if c.Type != "WorksSucceeded" {
			ours = append(ours, c)
		}
	}
	result := &StatusResult{
		Phase:          "Ready",
		Message:        "VCluster vcluster-media is fully operational",
		LastReconciled: "2026-08-21T18:11:50Z",
		Endpoints:      Endpoints{API: "https://media.integratn.tech:443", ArgoCD: "https://argocd.cluster.integratn.tech/applications/vcluster-vcluster-media"},
		Credentials:    Credentials{KubeconfigSecret: "vcluster-vcluster-media-kubeconfig", OnePasswordItem: "vcluster-vcluster-media-kubeconfig"},
		Health: Health{
			ArgoCD:    ArgoCDHealth{SyncStatus: "Synced", HealthStatus: "Healthy"},
			Workloads: WorkloadHealth{Ready: 37, Total: 37},
			SubApps:   SubAppHealth{Healthy: 0, Total: 0, Unhealthy: nil},
		},
		Conditions: ours,
	}
	next := buildStatusMap(vcr, result)

	// Premise: the patch really does carry the null. (A nil []string is a typed
	// nil inside an interface, so compare the bytes, not the value.)
	if b, _ := json.Marshal(next["health"]); !strings.Contains(string(b), `"unhealthy":null`) {
		t.Fatalf("test premise: the patch must carry unhealthy: null for an empty list, got %s", b)
	}
	if !statusUnchanged(vcr, next) {
		t.Fatal("a steady-state pass must not write: the patch would leave the stored status identical")
	}
}

func TestStatusUnchangedNullOnAbsentKeyIsNoop(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"health": map[string]interface{}{"subApps": map[string]interface{}{"healthy": float64(0), "total": float64(0)}},
		},
	}}
	next := map[string]interface{}{
		"health": map[string]interface{}{"subApps": map[string]interface{}{"healthy": 0, "total": 0, "unhealthy": nil}},
	}
	if !statusUnchanged(vcr, next) {
		t.Error("null for a key the object does not have deletes nothing and must not trigger a write")
	}
}

func TestStatusUnchangedNullOnPresentKeyIsAChange(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"health": map[string]interface{}{"subApps": map[string]interface{}{"healthy": float64(1), "total": float64(2), "unhealthy": []interface{}{"app-a"}}},
		},
	}}
	next := map[string]interface{}{
		"health": map[string]interface{}{"subApps": map[string]interface{}{"healthy": 1, "total": 2, "unhealthy": nil}},
	}
	if statusUnchanged(vcr, next) {
		t.Error("the list has emptied; null deletes a stored key and that is a real change")
	}
}

func TestStatusUnchangedKeepsUnsentNestedKeys(t *testing.T) {
	// A merge patch only replaces what it mentions. Nested keys owned by someone
	// else (or by an older build of us) survive, so their presence is not drift.
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"health": map[string]interface{}{
				"argocd":   map[string]interface{}{"healthStatus": "Healthy", "syncStatus": "Synced"},
				"somebody": map[string]interface{}{"else": "owns this"},
			},
		},
	}}
	next := map[string]interface{}{
		"health": map[string]interface{}{"argocd": map[string]interface{}{"healthStatus": "Healthy", "syncStatus": "Synced"}},
	}
	if !statusUnchanged(vcr, next) {
		t.Error("nested keys the patch does not mention are kept by the server and must not count as a change")
	}
}

func TestStatusUnchangedNumberTypesAreNotAChange(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{"health": map[string]interface{}{"workloads": map[string]interface{}{"ready": float64(37), "total": int64(37)}}},
	}}
	next := map[string]interface{}{"health": map[string]interface{}{"workloads": map[string]interface{}{"ready": 37, "total": 37}}}
	if !statusUnchanged(vcr, next) {
		t.Error("int from our structs vs float64/int64 from the API server is the same number")
	}
}

func TestStatusUnchangedDetectsARealChange(t *testing.T) {
	vcr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{"health": map[string]interface{}{"workloads": map[string]interface{}{"ready": float64(36), "total": float64(37)}}},
	}}
	next := map[string]interface{}{"health": map[string]interface{}{"workloads": map[string]interface{}{"ready": 37, "total": 37}}}
	if statusUnchanged(vcr, next) {
		t.Error("36/37 -> 37/37 is a change and must be written")
	}
}

func TestApplyMergePatchDoesNotMutateInputs(t *testing.T) {
	target := normalizeJSON(map[string]interface{}{"a": map[string]interface{}{"keep": 1, "drop": 2}})
	patch := normalizeJSON(map[string]interface{}{"a": map[string]interface{}{"drop": nil, "add": 3}})
	out := applyMergePatch(target, patch)
	if _, still := target.(map[string]interface{})["a"].(map[string]interface{})["drop"]; !still {
		t.Error("target was mutated")
	}
	a := out.(map[string]interface{})["a"].(map[string]interface{})
	if _, gone := a["drop"]; gone || a["add"] != float64(3) || a["keep"] != float64(1) {
		t.Errorf("merge result wrong: %#v", a)
	}
}
