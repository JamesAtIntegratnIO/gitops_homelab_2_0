package kube

import (
	"testing"
)

func TestParseArgoAppStatus_PartialStatus(t *testing.T) {
	// Only sync status, no health
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"sync": map[string]interface{}{"status": "OutOfSync"},
		},
	}
	sync, health := ParseArgoAppStatus(obj)
	if sync != "OutOfSync" {
		t.Errorf("sync = %q, want 'OutOfSync'", sync)
	}
	if health != "" {
		t.Errorf("health = %q, want empty", health)
	}
}

func TestParseArgoAppStatus_OnlyHealth(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"health": map[string]interface{}{"status": "Degraded"},
		},
	}
	sync, health := ParseArgoAppStatus(obj)
	if sync != "" {
		t.Errorf("sync = %q, want empty", sync)
	}
	if health != "Degraded" {
		t.Errorf("health = %q, want 'Degraded'", health)
	}
}

func TestParseArgoAppStatus_NilMap(t *testing.T) {
	var obj map[string]interface{}
	sync, health := ParseArgoAppStatus(obj)
	if sync != "" || health != "" {
		t.Errorf("expected empty strings for nil map, got sync=%q, health=%q", sync, health)
	}
}

func TestParseArgoAppStatus_AllStatuses(t *testing.T) {
	tests := []struct {
		syncStatus   string
		healthStatus string
	}{
		{"Synced", "Healthy"},
		{"OutOfSync", "Degraded"},
		{"Unknown", "Missing"},
		{"Synced", "Progressing"},
	}
	for _, tt := range tests {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": tt.syncStatus},
				"health": map[string]interface{}{"status": tt.healthStatus},
			},
		}
		sync, health := ParseArgoAppStatus(obj)
		if sync != tt.syncStatus {
			t.Errorf("sync = %q, want %q", sync, tt.syncStatus)
		}
		if health != tt.healthStatus {
			t.Errorf("health = %q, want %q", health, tt.healthStatus)
		}
	}
}

func TestParsePromiseStatus_EmptyConditionsList(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{},
		},
	}
	got := ParsePromiseStatus(obj)
	if got != "Unknown" {
		t.Errorf("ParsePromiseStatus = %q, want 'Unknown'", got)
	}
}

func TestParsePromiseStatus_MultipleConditions(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
				map[string]interface{}{"type": "Available", "status": "True"},
				map[string]interface{}{"type": "Progressing", "status": "False"},
			},
		},
	}
	got := ParsePromiseStatus(obj)
	if got != "Available" {
		t.Errorf("ParsePromiseStatus = %q, want 'Available'", got)
	}
}

func TestParsePromiseStatus_InvalidConditionType(t *testing.T) {
	// Condition entries that are not maps should be skipped
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				"not-a-map",
				42,
				map[string]interface{}{"type": "Available", "status": "False"},
			},
		},
	}
	got := ParsePromiseStatus(obj)
	if got != "Unavailable" {
		t.Errorf("ParsePromiseStatus = %q, want 'Unavailable'", got)
	}
}
