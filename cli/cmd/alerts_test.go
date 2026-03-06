package cmd

import (
	"sort"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

func TestNoiseAlerts(t *testing.T) {
	// Verify the noiseAlerts map contains expected entries
	expectedNoise := []string{"Watchdog", "InfoInhibitor"}
	for _, name := range expectedNoise {
		if !noiseAlerts[name] {
			t.Errorf("expected %q to be in noiseAlerts", name)
		}
	}
}

func TestNoiseAlerts_DoesNotContainReal(t *testing.T) {
	realAlerts := []string{"KubePodCrashLooping", "NodeNotReady", "TargetDown"}
	for _, name := range realAlerts {
		if noiseAlerts[name] {
			t.Errorf("%q should not be a noise alert", name)
		}
	}
}

func TestNewAlertsCmd(t *testing.T) {
	cmd := newAlertsCmd()
	if cmd.Use != "alerts" {
		t.Errorf("expected Use 'alerts', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check --all flag exists
	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("missing --all flag")
	}
}

func TestAlertSorting(t *testing.T) {
	// Test the severity sorting logic used in runAlerts
	alerts := []kube.PrometheusAlert{
		{AlertName: "ZZZ", Severity: "info"},
		{AlertName: "AAA", Severity: "critical"},
		{AlertName: "BBB", Severity: "warning"},
		{AlertName: "CCC", Severity: "critical"},
		{AlertName: "DDD", Severity: "info"},
	}

	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2, "none": 3, "": 4}
	sort.Slice(alerts, func(i, j int) bool {
		si := severityOrder[alerts[i].Severity]
		sj := severityOrder[alerts[j].Severity]
		if si != sj {
			return si < sj
		}
		return alerts[i].AlertName < alerts[j].AlertName
	})

	// Critical should come first
	if alerts[0].Severity != "critical" {
		t.Errorf("expected first alert to be critical, got %q", alerts[0].Severity)
	}
	if alerts[0].AlertName != "AAA" {
		t.Errorf("expected first critical to be AAA, got %q", alerts[0].AlertName)
	}
	if alerts[1].AlertName != "CCC" {
		t.Errorf("expected second critical to be CCC, got %q", alerts[1].AlertName)
	}

	// Then warning
	if alerts[2].Severity != "warning" {
		t.Errorf("expected third alert to be warning, got %q", alerts[2].Severity)
	}

	// Then info, sorted alphabetically
	if alerts[3].AlertName != "DDD" {
		t.Errorf("expected fourth to be DDD, got %q", alerts[3].AlertName)
	}
	if alerts[4].AlertName != "ZZZ" {
		t.Errorf("expected fifth to be ZZZ, got %q", alerts[4].AlertName)
	}
}

func TestAlertFiltering_HidesNoise(t *testing.T) {
	alerts := []kube.PrometheusAlert{
		{AlertName: "Watchdog", Severity: "none"},
		{AlertName: "InfoInhibitor", Severity: "info"},
		{AlertName: "RealAlert", Severity: "critical"},
	}

	var filtered []kube.PrometheusAlert
	noiseCount := 0
	showAll := false
	for _, a := range alerts {
		if !showAll && noiseAlerts[a.AlertName] {
			noiseCount++
			continue
		}
		filtered = append(filtered, a)
	}

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered alert, got %d", len(filtered))
	}
	if noiseCount != 2 {
		t.Errorf("expected 2 noise alerts, got %d", noiseCount)
	}
	if filtered[0].AlertName != "RealAlert" {
		t.Errorf("expected RealAlert, got %q", filtered[0].AlertName)
	}
}

func TestAlertFiltering_ShowAll(t *testing.T) {
	alerts := []kube.PrometheusAlert{
		{AlertName: "Watchdog", Severity: "none"},
		{AlertName: "InfoInhibitor", Severity: "info"},
		{AlertName: "RealAlert", Severity: "critical"},
	}

	var filtered []kube.PrometheusAlert
	noiseCount := 0
	showAll := true
	for _, a := range alerts {
		if !showAll && noiseAlerts[a.AlertName] {
			noiseCount++
			continue
		}
		filtered = append(filtered, a)
	}

	if len(filtered) != 3 {
		t.Errorf("expected 3 alerts with --all, got %d", len(filtered))
	}
	if noiseCount != 0 {
		t.Errorf("expected 0 noise with --all, got %d", noiseCount)
	}
}

func TestAlertSeverityCounting(t *testing.T) {
	alerts := []kube.PrometheusAlert{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "warning"},
		{Severity: "info"},
		{Severity: "info"},
		{Severity: "info"},
		{Severity: ""},
	}

	critical, warning, info := 0, 0, 0
	for _, a := range alerts {
		switch a.Severity {
		case "critical":
			critical++
		case "warning":
			warning++
		default:
			info++
		}
	}

	if critical != 2 {
		t.Errorf("critical = %d, want 2", critical)
	}
	if warning != 1 {
		t.Errorf("warning = %d, want 1", warning)
	}
	if info != 4 {
		t.Errorf("info = %d, want 4 (includes empty severity)", info)
	}
}
