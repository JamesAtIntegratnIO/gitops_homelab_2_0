package kargo

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func obj(o map[string]any) unstructured.Unstructured {
	return unstructured.Unstructured{Object: o}
}

// Kargo leaves status.phase unset between creating a Promotion and reconciling
// it. An empty column reads as a bug rather than a transient, so it defaults.
func TestPhaseOfDefaultsToPending(t *testing.T) {
	if got := phaseOf(obj(map[string]any{})); got != "Pending" {
		t.Fatalf("want Pending for a promotion with no phase, got %q", got)
	}
	p := obj(map[string]any{"status": map[string]any{"phase": "Errored"}})
	if got := phaseOf(p); got != "Errored" {
		t.Fatalf("want Errored, got %q", got)
	}
}

func TestConditionOfReadsStatusAndReason(t *testing.T) {
	s := obj(map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Healthy", "status": "True", "reason": "Healthy"},
		map[string]any{"type": "Verified", "status": "False", "reason": "VerificationError"},
	}}})

	if st, rs := conditionOf(s, "Verified"); st != "False" || rs != "VerificationError" {
		t.Fatalf("want False/VerificationError, got %q/%q", st, rs)
	}
	if st, _ := conditionOf(s, "Healthy"); st != "True" {
		t.Fatalf("want True for Healthy, got %q", st)
	}
	// A condition Kargo has not written yet must read as empty, not as False --
	// "not reported" and "reported as bad" are different states.
	if st, rs := conditionOf(s, "Ready"); st != "" || rs != "" {
		t.Fatalf("want empty for an absent condition, got %q/%q", st, rs)
	}
}

// Failed and Errored are different problems with different fixes, but both
// need a human, so both count as needing attention.
func TestIsFailureCoversEveryBadPhase(t *testing.T) {
	for _, p := range []string{"Failed", "Errored", "Aborted"} {
		if !isFailure(p) {
			t.Errorf("%s should count as needing attention", p)
		}
	}
	for _, p := range []string{"Succeeded", "Running", "Pending"} {
		if isFailure(p) {
			t.Errorf("%s should not count as needing attention", p)
		}
	}
}

// Kargo keeps a stack of verifications, newest first, nested two levels down.
// Getting the index wrong silently reports a stale result as current.
func TestLatestVerificationPhaseReadsNewestFirst(t *testing.T) {
	s := obj(map[string]any{"status": map[string]any{
		"freightHistory": []any{
			map[string]any{"verificationHistory": []any{
				map[string]any{"phase": "Failed"},
				map[string]any{"phase": "Successful"},
			}},
			map[string]any{"verificationHistory": []any{
				map[string]any{"phase": "Successful"},
			}},
		},
	}})
	if got := latestVerificationPhase(s); got != "Failed" {
		t.Fatalf("want the newest verification (Failed), got %q", got)
	}
}

func TestLatestVerificationPhaseHandlesAbsentHistory(t *testing.T) {
	for name, o := range map[string]unstructured.Unstructured{
		"no status":         obj(map[string]any{}),
		"no freightHistory": obj(map[string]any{"status": map[string]any{}}),
		"empty history":     obj(map[string]any{"status": map[string]any{"freightHistory": []any{}}}),
		"no verifications": obj(map[string]any{"status": map[string]any{
			"freightHistory": []any{map[string]any{}}}}),
	} {
		if got := latestVerificationPhase(o); got != "" {
			t.Errorf("%s: want empty, got %q", name, got)
		}
	}
}

func TestBoolishMakesConditionsScannable(t *testing.T) {
	cases := map[string]string{"True": "yes", "False": "NO", "": "-", "Unknown": "Unknown"}
	for in, want := range cases {
		if got := boolish(in); got != want {
			t.Errorf("boolish(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortDurationPicksAUsefulUnit(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{3 * time.Hour, "3h"},
		{50 * time.Hour, "2d"},
	}
	for _, c := range cases {
		if got := shortDuration(c.d); got != c.want {
			t.Errorf("shortDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTruncateKeepsTheDistinguishingPrefix(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("want unchanged, got %q", got)
	}
	got := truncate("cert-manager.01m0kqtsqew7fqrzcsz2173p0h.34d9473", 20)
	if len([]rune(got)) != 20 {
		t.Fatalf("want 20 runes, got %d (%q)", len([]rune(got)), got)
	}
	if got[:12] != "cert-manager" {
		t.Fatalf("truncation must keep the front, got %q", got)
	}
}
