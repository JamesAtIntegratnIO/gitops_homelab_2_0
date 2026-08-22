package kargo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const defaultTimeout = 15 * time.Second

func client() (*kube.Client, error) {
	cfg := config.Get()
	c, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return nil, fmt.Errorf("connecting to cluster: %w", err)
	}
	return c, nil
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}

func str(obj map[string]any, fields ...string) string {
	v, _, _ := platform.UnstructuredNestedString(obj, fields...)
	return v
}

// phaseOf reports a Promotion's phase, defaulting to Pending. Kargo leaves the
// field unset briefly between creating a Promotion and reconciling it, and an
// empty column reads as a bug rather than a transient.
func phaseOf(p unstructured.Unstructured) string {
	if ph := str(p.Object, "status", "phase"); ph != "" {
		return ph
	}
	return "Pending"
}

// conditionOf returns the status and reason of a named condition.
func conditionOf(obj unstructured.Unstructured, want string) (status, reason string) {
	conds, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return "", ""
	}
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == want {
			s, _ := m["status"].(string)
			r, _ := m["reason"].(string)
			return s, r
		}
	}
	return "", ""
}

// age renders a creation timestamp as a compact duration.
func age(obj unstructured.Unstructured) string {
	ts := obj.GetCreationTimestamp().Time
	if ts.IsZero() {
		return "-"
	}
	return shortDuration(time.Since(ts))
}

func shortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// stageOf reports which Stage a Promotion belongs to.
func stageOf(p unstructured.Unstructured) string {
	return str(p.Object, "spec", "stage")
}

// truncate keeps a table readable. Kargo's generated names are long and the
// distinguishing part is at the front.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// sortByNamespaceName gives every listing a stable order, so two runs are
// diffable.
func sortByNamespaceName(items []unstructured.Unstructured) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].GetNamespace() != items[j].GetNamespace() {
			return items[i].GetNamespace() < items[j].GetNamespace()
		}
		return items[i].GetName() < items[j].GetName()
	})
}

// isFailure reports whether a phase means someone needs to look.
//
// Failed and Errored are different problems: Failed means the pipeline ran and
// did not get what it wanted (a PR closed unmerged, a merge that timed out
// against a red check); Errored means a step could not run at all. Both need
// attention, which is why they group together here even though they route to
// different fixes.
func isFailure(phase string) bool {
	return phase == "Failed" || phase == "Errored" || phase == "Aborted"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "-"
}
