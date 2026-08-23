package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveOpenAI exercises a real endpoint. Skipped unless DELIVERY_AGENT_LIVE
// names one, so `go test ./...` stays hermetic.
//
//	DELIVERY_AGENT_LIVE=http://localhost:1234/v1 \
//	DELIVERY_AGENT_MODEL=qwen/qwen3.6-35b-a3b go test ./llm -run Live -v
func TestLiveOpenAI(t *testing.T) {
	base := os.Getenv("DELIVERY_AGENT_LIVE")
	if base == "" {
		t.Skip("set DELIVERY_AGENT_LIVE to run against a real endpoint")
	}
	model := os.Getenv("DELIVERY_AGENT_MODEL")
	if model == "" {
		t.Fatal("DELIVERY_AGENT_MODEL is required alongside DELIVERY_AGENT_LIVE")
	}

	p := &OpenAI{BaseURL: base, Model: model, APIKey: os.Getenv("DELIVERY_AGENT_KEY"), Timeout: 15 * time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	system := os.Getenv("DELIVERY_AGENT_PROMPT")
	if system == "" {
		t.Fatal("DELIVERY_AGENT_PROMPT is required: the point of this test is the real prompt")
	}

	user := `PR: bump metallb chart 0.15.2 -> 0.16.0. The pre-merge gate is RED.

Rendered diff shows the speaker's FRR sidecars are gone and a new frr-k8s
DaemonSet, four CRDs and a webhook appear, because chart 0.16.0 flips
speaker.frr.enabled false and frrk8s.enabled true by default. This cluster is
L2-only and does not use FRR.

File addons/environments/production/addons/addons.yaml currently contains:

  metallb:
    defaultVersion: 0.16.0
    valuesObject:
      speaker:
        frr:
          enabled: true
      frrk8s:
        enabled: true

Propose the fix.`

	start := time.Now()
	v, err := p.Classify(ctx, system, user)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	t.Logf("provider=%s elapsed=%s", p.Name(), time.Since(start).Round(time.Second))
	t.Logf("classification=%s", v.Classification)
	t.Logf("summary=%s", v.Summary)
	for _, e := range v.Edits {
		t.Logf("edit: %s :: %s  %q -> %q  (%s)", e.Path, e.Key, e.From, e.To, e.Rationale)
	}
	if v.EscalationReason != "" {
		t.Logf("escalation=%s", v.EscalationReason)
	}
	if err := v.Valid(); err != nil {
		t.Fatalf("verdict invalid: %v", err)
	}
}
