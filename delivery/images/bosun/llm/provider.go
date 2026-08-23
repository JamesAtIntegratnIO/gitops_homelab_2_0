// Package llm is the model-provider seam.
//
// The agent never lets a model touch a file. A model is asked one question and
// returns one structured answer; the agent decides what, if anything, to do
// with it. That is what keeps this portable -- any provider able to return
// schema-constrained JSON is enough -- and it is also the safety model, because
// "never edit the gate" is then a property of the code rather than a sentence
// in a prompt.
package llm

import (
	"context"
	"fmt"
)

// Verdict is what the model is asked to produce. The schema is deliberately
// small: a classification, a short explanation a human will read on the pull
// request, and -- only for the mechanical case -- a list of precise edits.
type Verdict struct {
	// Classification is one of: mechanical, escalate, no_action.
	Classification string `json:"classification"`

	// Summary is one sentence, shown as the first line of the PR comment.
	Summary string `json:"summary"`

	// Reasoning is the explanation. This is the part that earns the agent its
	// place even when it proposes no edit at all.
	Reasoning string `json:"reasoning"`

	// Edits are only meaningful when Classification is "mechanical". Every one
	// is checked against the allowlist and against its own `From` value before
	// anything is written.
	Edits []Edit `json:"edits,omitempty"`

	// EscalationReason says why a human is needed, when they are.
	EscalationReason string `json:"escalationReason,omitempty"`
}

// Edit is one scalar change: set Key in Path from From to To.
//
// `From` is not decoration. It is checked against what the file actually
// contains, so a model working from a stale or hallucinated view of the
// repository has its edit rejected rather than applied to the wrong line.
type Edit struct {
	Path      string `json:"path"`
	Key       string `json:"key"`
	From      string `json:"from"`
	To        string `json:"to"`
	Rationale string `json:"rationale"`
}

const (
	ClassMechanical = "mechanical"
	ClassEscalate   = "escalate"
	ClassNoAction   = "no_action"
)

// Valid reports whether a verdict is well-formed enough to act on. Schema
// validation guarantees shape, not sense.
func (v *Verdict) Valid() error {
	switch v.Classification {
	case ClassMechanical:
		if len(v.Edits) == 0 {
			return fmt.Errorf("classification %q with no edits: nothing to apply", v.Classification)
		}
	case ClassEscalate:
		if v.EscalationReason == "" {
			// Recoverable: the reasoning says why. Failing the whole triage
			// over a soft field would throw away a correct verdict.
			v.EscalationReason = v.Reasoning
		}
		if v.EscalationReason == "" {
			return fmt.Errorf("classification %q with neither escalationReason nor reasoning", v.Classification)
		}
	case ClassNoAction:
	default:
		return fmt.Errorf("unknown classification %q", v.Classification)
	}
	if v.Summary == "" {
		return fmt.Errorf("verdict has no summary")
	}
	return nil
}

// Provider is one model backend.
//
// Two implementations cover the field: Anthropic Messages and OpenAI chat
// completions. Between them they reach hosted Anthropic, Bedrock, Vertex,
// OpenAI, Azure, vLLM, Ollama, LM Studio, and anything behind a LiteLLM-style
// proxy -- because `baseURL` is a value, not a constant.
type Provider interface {
	// Classify sends the prompt and returns a validated Verdict. An
	// implementation must constrain the model to the Verdict schema where the
	// backend supports it, and must not retry indefinitely.
	Classify(ctx context.Context, systemPrompt, userPrompt string) (*Verdict, error)

	// Name identifies the provider and model in logs and PR comments, so a
	// reader can tell what produced a verdict.
	Name() string
}

// VerdictSchema is the JSON Schema handed to providers that support
// constrained decoding. With it, a malformed answer is impossible -- the model
// can be wrong, but it cannot return something the agent fails to parse.
func VerdictSchema() map[string]any {
	edit := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"path", "key", "from", "to", "rationale"},
		"properties": map[string]any{
			"path":      map[string]any{"type": "string", "description": "Repository-relative file path."},
			"key":       map[string]any{"type": "string", "description": "Dotted path to the scalar, e.g. argocd.defaultVersion."},
			"from":      map[string]any{"type": "string", "description": "The value currently in the file. Checked before anything is written."},
			"to":        map[string]any{"type": "string", "description": "The value to write."},
			"rationale": map[string]any{"type": "string", "description": "Why this specific change, in one sentence."},
		},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		// Every property is required. Strict constrained decoding expects it,
		// and a model will simply omit an optional field -- the first live
		// test returned classification "escalate" with an empty
		// escalationReason for exactly that reason.
		"required": []string{"classification", "summary", "reasoning", "edits", "escalationReason"},
		"properties": map[string]any{
			"classification": map[string]any{
				"type": "string",
				"enum": []string{ClassMechanical, ClassEscalate, ClassNoAction},
			},
			"summary":   map[string]any{"type": "string"},
			"reasoning": map[string]any{"type": "string"},
			"edits": map[string]any{
				"type":        "array",
				"items":       edit,
				"description": "Empty unless classification is mechanical.",
			},
			"escalationReason": map[string]any{
				"type":        "string",
				"description": "Required when classification is escalate.",
			},
		},
	}
}
