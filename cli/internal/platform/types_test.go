package platform

import "testing"

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
		{"Synced", "Unknown", "Progressing"},
		{"", "", "Progressing"},
	}
	for _, tt := range tests {
		got := PhaseFromArgoCD(tt.sync, tt.health)
		if got != tt.want {
			t.Errorf("PhaseFromArgoCD(%q, %q) = %q, want %q", tt.sync, tt.health, got, tt.want)
		}
	}
}

func TestResourceKindConstants(t *testing.T) {
	if KindVCluster != "vcluster" {
		t.Errorf("KindVCluster = %q", KindVCluster)
	}
	if KindWorkload != "workload" {
		t.Errorf("KindWorkload = %q", KindWorkload)
	}
	if KindAddon != "addon" {
		t.Errorf("KindAddon = %q", KindAddon)
	}
}

func TestPlatformStatusStructure(t *testing.T) {
	ps := &PlatformStatus{
		VClusters: []ResourceStatus{{Kind: KindVCluster, Name: "media", Phase: "Ready"}},
		Workloads: []ResourceStatus{{Kind: KindWorkload, Name: "sonarr", Phase: "Ready"}},
		Addons:    []ResourceStatus{{Kind: KindAddon, Name: "cert-manager", Phase: "Ready"}},
	}
	if len(ps.VClusters) != 1 {
		t.Errorf("expected 1 vcluster, got %d", len(ps.VClusters))
	}
	if len(ps.Workloads) != 1 {
		t.Errorf("expected 1 workload, got %d", len(ps.Workloads))
	}
	if len(ps.Addons) != 1 {
		t.Errorf("expected 1 addon, got %d", len(ps.Addons))
	}
}
