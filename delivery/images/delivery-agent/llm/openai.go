package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI speaks the chat-completions API.
//
// This is the broadest single reach available: OpenAI, Azure OpenAI, vLLM,
// Ollama, LM Studio, llama.cpp's server, and anything behind LiteLLM all speak
// it. `BaseURL` is what makes a self-hosted model a first-class path rather
// than a workaround.
type OpenAI struct {
	BaseURL string
	APIKey  string
	Model   string
	// ReasoningEffort maps to the parameter reasoning-capable models expose.
	// Empty omits it, which is what non-reasoning models need.
	ReasoningEffort string
	Timeout         time.Duration
	HTTP            *http.Client
}

func (o *OpenAI) Name() string { return fmt.Sprintf("openai/%s", o.Model) }

func (o *OpenAI) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	t := o.Timeout
	if t == 0 {
		// Deliberately generous. A local model doing real reasoning over a
		// 20k-token diff can take minutes, and the caller is already
		// asynchronous -- the promotion is never waiting on this.
		t = 10 * time.Minute
	}
	return &http.Client{Timeout: t}
}

func (o *OpenAI) Classify(ctx context.Context, systemPrompt, userPrompt string) (*Verdict, error) {
	body := map[string]any{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		// Constrained decoding. Where the backend honours it, a malformed
		// answer becomes impossible rather than merely unlikely.
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "verdict",
				"strict": true,
				"schema": VerdictSchema(),
			},
		},
		"temperature": 0,
	}
	if o.ReasoningEffort != "" {
		body["reasoning_effort"] = o.ReasoningEffort
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(o.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	resp, err := o.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", url, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, truncate(string(payload), 400))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				// A reasoning model's thinking channel. LM Studio (and other
				// llama.cpp-derived servers) put the schema-constrained answer
				// HERE and leave `content` empty -- verified live against
				// qwen3.6-35b-a3b. Ignoring it makes every reasoning model
				// look like it returned nothing, silently.
				ReasoningContent string `json:"reasoning_content,omitempty"`
				Reasoning        string `json:"reasoning,omitempty"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	msg := out.Choices[0].Message
	for _, candidate := range []string{msg.Content, msg.ReasoningContent, msg.Reasoning} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if v, err := parseVerdict(candidate); err == nil {
			return v, nil
		}
	}
	return nil, fmt.Errorf("no parseable verdict in the response (content %d bytes, reasoning_content %d bytes)",
		len(msg.Content), len(msg.ReasoningContent))
}

// parseVerdict tolerates the two things that survive constrained decoding on
// looser backends: a fenced code block, and a reasoning model that emits its
// thinking before the JSON.
func parseVerdict(content string) (*Verdict, error) {
	s := strings.TrimSpace(content)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			s = s[j+1:]
		}
		if k := strings.LastIndex(s, "```"); k >= 0 {
			s = s[:k]
		}
		s = strings.TrimSpace(s)
	}
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = s[i:]
	}
	var v Verdict
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("model did not return a parseable verdict: %w (got %q)", err, truncate(s, 300))
	}
	if err := v.Valid(); err != nil {
		return nil, fmt.Errorf("model returned an unusable verdict: %w", err)
	}
	return &v, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
