package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/edits"
	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/gitprovider"
	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/llm"
)

const valuesPath = "addons/values.yaml"

const valuesBefore = `# MetalLB, L2 only.
metallb:
  enabled: true
  defaultVersion: 0.16.0
`

// The gate's report, in the shape triage looks for: the marker is how the
// comment is found among every other comment on the pull request, and the
// version it names is the only corroboration an edit's new value can have.
const gateReport = `<!-- gitops-gate -->
### addons-gate — FAILED

metallb 0.16.0 -> 0.16.1: the chart's rendered speaker DaemonSet no longer
matches what this cluster runs.
`

type harness struct {
	triage *Triage
	git    *gitprovider.Fake
	model  *llm.Fake
	root   string
}

// newHarness wires the workflow to a real directory on disk, so a permitted
// edit genuinely rewrites a file and a refused one genuinely leaves it alone.
func newHarness(t *testing.T) *harness {
	t.Helper()

	root := t.TempDir()
	full := filepath.Join(root, valuesPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(valuesBefore), 0o644); err != nil {
		t.Fatal(err)
	}

	git := &gitprovider.Fake{
		PR: &gitprovider.PullRequest{
			Number:  42,
			Title:   "chore(deps): metallb 0.16.0 -> 0.16.1",
			Branch:  "kargo/metallb",
			HeadSHA: "c0ffee",
		},
		Comments: []gitprovider.Comment{{Author: "gitops-gate", Body: gateReport}},
		Check:    gitprovider.CheckFailure,
	}
	model := &llm.Fake{}

	return &harness{
		git:   git,
		model: model,
		root:  root,
		triage: &Triage{
			Git:         git,
			LLM:         model,
			Policy:      edits.Policy{Allow: []string{"addons/**"}},
			CheckName:   "addons-gate",
			MaxAttempts: 2,
			GatePoll:    time.Millisecond,
			Log:         t.Logf,
			Checkout: func(context.Context, *gitprovider.PullRequest) (string, func(), error) {
				return root, func() {}, nil
			},
		},
	}
}

func (h *harness) values(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.root, valuesPath))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func promotion() Promotion {
	return Promotion{
		Project: "addons", Stage: "metallb", Artifact: "metallb",
		From: "0.16.0", To: "0.16.1",
		PRNumber: 42, Branch: "kargo/metallb",
		Files: []string{valuesPath},
	}
}

func TestTriageRun(t *testing.T) {
	mechanical := func(e llm.Edit) *llm.Verdict {
		return &llm.Verdict{
			Classification: llm.ClassMechanical,
			Summary:        "Move the metallb pin with the chart.",
			Reasoning:      "The rendered diff proves the default changed.",
			Edits:          []llm.Edit{e},
		}
	}
	permitted := llm.Edit{
		Path: valuesPath, Key: "metallb.defaultVersion",
		From: "0.16.0", To: "0.16.1", Rationale: "The gate names this version.",
	}

	tests := []struct {
		name     string
		check    gitprovider.CheckState
		labels   []string
		verdict  *llm.Verdict
		modelErr error

		wantModelCalled bool
		wantComments    int
		wantSaying      []string
		wantLabels      []string
		wantPush        bool
		wantVersion     string
	}{
		{
			name:        "a green gate is left alone",
			check:       gitprovider.CheckSuccess,
			wantVersion: "0.16.0",
		},
		{
			name:        "a pull request with no gate check is left alone",
			check:       gitprovider.CheckMissing,
			wantVersion: "0.16.0",
		},
		{
			name:        "a gate still running is left for the next run",
			check:       gitprovider.CheckPending,
			wantVersion: "0.16.0",
		},
		{
			name:            "an escalate verdict asks for a human and pushes nothing",
			check:           gitprovider.CheckFailure,
			verdict:         &llm.Verdict{Classification: llm.ClassEscalate, Summary: "This is a migration.", Reasoning: "The CRD schema changed.", EscalationReason: "The upgrade needs a CRD migration."},
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"Needs a human.", "The upgrade needs a CRD migration."},
			wantLabels:      []string{labelNeedsHuman},
			wantVersion:     "0.16.0",
		},
		{
			name:            "a permitted mechanical edit is applied, pushed and counted",
			check:           gitprovider.CheckFailure,
			verdict:         mechanical(permitted),
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"Applied", "metallb.defaultVersion", "0.16.1", "attempt 1 of 2"},
			wantLabels:      []string{labelAttempt + "1"},
			wantPush:        true,
			wantVersion:     "0.16.1",
		},
		{
			name:  "a mechanical edit on a denied path escalates instead of landing",
			check: gitprovider.CheckFailure,
			verdict: mechanical(llm.Edit{
				Path: ".github/workflows/gate.yaml", Key: "jobs.gate.if",
				From: "true", To: "false", Rationale: "Skip the gate.",
			}),
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"Needs a human.", "rejected before anything was written", "path is denied"},
			wantLabels:      []string{labelNeedsHuman},
			wantVersion:     "0.16.0",
		},
		{
			name:  "a mechanical edit whose from value is stale escalates instead of landing",
			check: gitprovider.CheckFailure,
			verdict: mechanical(llm.Edit{
				Path: valuesPath, Key: "metallb.defaultVersion",
				From: "0.15.9", To: "0.16.1", Rationale: "Bump the pin.",
			}),
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"Needs a human.", "refusing to overwrite"},
			wantLabels:      []string{labelNeedsHuman},
			wantVersion:     "0.16.0",
		},
		{
			name:        "a pull request already marked needs-human is left alone",
			check:       gitprovider.CheckFailure,
			labels:      []string{labelNeedsHuman},
			wantVersion: "0.16.0",
		},
		{
			name:         "a pull request out of attempts escalates without asking the model",
			check:        gitprovider.CheckFailure,
			labels:       []string{labelAttempt + "1", labelAttempt + "2"},
			verdict:      mechanical(permitted),
			wantComments: 1,
			wantSaying:   []string{"Needs a human", "limit of 2 automatic fix attempts"},
			wantLabels:   []string{labelNeedsHuman},
			wantVersion:  "0.16.0",
		},
		{
			name:            "a model that cannot be reached is reported rather than ignored",
			check:           gitprovider.CheckFailure,
			modelErr:        errors.New("connection refused"),
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"Needs a human", "Could not reach the model", "connection refused"},
			wantLabels:      []string{labelNeedsHuman},
			wantVersion:     "0.16.0",
		},
		{
			name:            "a no_action verdict is explained and nothing else",
			check:           gitprovider.CheckFailure,
			verdict:         &llm.Verdict{Classification: llm.ClassNoAction, Summary: "The gate is red for an unrelated reason.", Reasoning: "A flaky registry pull."},
			wantModelCalled: true,
			wantComments:    1,
			wantSaying:      []string{"No change proposed.", "unrelated reason"},
			wantVersion:     "0.16.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.git.Check = tc.check
			h.git.PR.Labels = tc.labels
			h.model.Verdict = tc.verdict
			h.model.Err = tc.modelErr

			if err := h.triage.Run(context.Background(), promotion()); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if called := h.model.Calls > 0; called != tc.wantModelCalled {
				t.Errorf("model called = %v, want %v", called, tc.wantModelCalled)
			}
			if len(h.git.Posted) != tc.wantComments {
				t.Errorf("posted %d comment(s), want %d: %q", len(h.git.Posted), tc.wantComments, h.git.Posted)
			}
			body := strings.Join(h.git.Posted, "\n")
			for _, want := range tc.wantSaying {
				if !strings.Contains(body, want) {
					t.Errorf("comment does not mention %q:\n%s", want, body)
				}
			}
			if !equal(h.git.Labelled, tc.wantLabels) {
				t.Errorf("labelled %v, want %v", h.git.Labelled, tc.wantLabels)
			}

			if got := len(h.git.Pushes) > 0; got != tc.wantPush {
				t.Fatalf("pushed = %v, want %v", got, tc.wantPush)
			}
			if !strings.Contains(h.values(t), "defaultVersion: "+tc.wantVersion) {
				t.Errorf("file does not hold defaultVersion %s:\n%s", tc.wantVersion, h.values(t))
			}
			if !tc.wantPush {
				return
			}
			push := h.git.Pushes[0]
			if push.Branch != "kargo/metallb" {
				t.Errorf("pushed to %q, want the pull request's own branch", push.Branch)
			}
			if !strings.Contains(push.Tree[valuesPath], "defaultVersion: "+tc.wantVersion) {
				t.Errorf("the pushed tree does not hold the new value:\n%s", push.Tree[valuesPath])
			}
		})
	}
}

// The evidence check only works if the model's own prompt is what the applier
// corroborates against, so the gate report has to reach both.
func TestTheModelIsShownTheGateReport(t *testing.T) {
	h := newHarness(t)
	h.model.Verdict = &llm.Verdict{Classification: llm.ClassNoAction, Summary: "Nothing to do.", Reasoning: "n/a"}

	if err := h.triage.Run(context.Background(), promotion()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{gateReportMarker, "0.16.1", "metallb.defaultVersion"} {
		if !strings.Contains(h.model.User, want) {
			t.Errorf("prompt does not contain %q:\n%s", want, h.model.User)
		}
	}
}

// A version the model invented renders perfectly and breaks at runtime, so the
// applier refuses it -- and a mechanical verdict that applies nothing escalates.
func TestAnInventedVersionIsRefusedAndEscalated(t *testing.T) {
	h := newHarness(t)
	h.model.Verdict = &llm.Verdict{
		Classification: llm.ClassMechanical,
		Summary:        "Bump the pin.",
		Reasoning:      "The gate says 0.16 is required.",
		Edits: []llm.Edit{{
			Path: valuesPath, Key: "metallb.defaultVersion",
			From: "0.16.0", To: "0.16.4", Rationale: "The gate wants a newer 0.16.",
		}},
	}

	if err := h.triage.Run(context.Background(), promotion()); err != nil {
		t.Fatal(err)
	}
	if len(h.git.Pushes) != 0 {
		t.Fatalf("pushed an invented version: %+v", h.git.Pushes)
	}
	if !equal(h.git.Labelled, []string{labelNeedsHuman}) {
		t.Errorf("labelled %v, want %v", h.git.Labelled, []string{labelNeedsHuman})
	}
	if !strings.Contains(h.values(t), "defaultVersion: 0.16.0") {
		t.Errorf("the file was changed:\n%s", h.values(t))
	}
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
