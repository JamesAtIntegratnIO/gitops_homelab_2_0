package evals

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JamesAtIntegratnIO/delivery-kit/delivery-agent/llm"
)

// TestEval measures the prompt against a real endpoint. Skipped unless
// DELIVERY_AGENT_LIVE is set, so `go test ./...` stays hermetic and offline.
//
//	DELIVERY_AGENT_LIVE=http://localhost:1234/v1 \
//	DELIVERY_AGENT_MODELS=qwen/qwen3.5-9b,qwen/qwen3.6-35b-a3b \
//	DELIVERY_AGENT_PROMPT="$(cat prompt.txt)" \
//	go test ./evals -run Eval -v -timeout 60m
func TestEval(t *testing.T) {
	base := os.Getenv("DELIVERY_AGENT_LIVE")
	if base == "" {
		t.Skip("set DELIVERY_AGENT_LIVE to measure against a real endpoint")
	}
	models := strings.Split(os.Getenv("DELIVERY_AGENT_MODELS"), ",")
	if len(models) == 0 || models[0] == "" {
		t.Fatal("DELIVERY_AGENT_MODELS is required")
	}
	system := os.Getenv("DELIVERY_AGENT_PROMPT")
	if system == "" {
		t.Fatal("DELIVERY_AGENT_PROMPT is required")
	}
	withInventory := os.Getenv("DELIVERY_AGENT_NO_INVENTORY") == ""

	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		p := &llm.OpenAI{
			BaseURL: base, Model: model,
			APIKey:          os.Getenv("DELIVERY_AGENT_KEY"),
			ReasoningEffort: os.Getenv("DELIVERY_AGENT_EFFORT"),
			Timeout:         20 * time.Minute,
		}
		var results []Result
		for _, c := range Cases {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			results = append(results, Run(ctx, p, system, c, withInventory))
			cancel()
		}
		t.Log(Summarise(model, results).String())
	}
}
