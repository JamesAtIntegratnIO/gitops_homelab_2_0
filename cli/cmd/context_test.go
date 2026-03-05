package cmd

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

func TestRunContext_ReturnsNil(t *testing.T) {
	// runContext always returns nil — it's a display-only command.
	// Set a known config to exercise the code path.
	cfg := &config.Config{
		RepoPath:  "/tmp/test-repo",
		GitMode:   "auto",
		ArgocdURL: "https://argocd.example.com",
		Platform: config.PlatformConfig{
			Domain:            "example.com",
			PlatformNamespace: "platform",
		},
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	err := runContext(nil, nil)
	if err != nil {
		t.Errorf("runContext should return nil, got %v", err)
	}
}

func TestRunContext_WithEmptyConfig(t *testing.T) {
	cfg := &config.Config{}
	config.Set(cfg)
	defer config.Set(config.Default())

	// Should not panic or error even with empty config
	err := runContext(nil, nil)
	if err != nil {
		t.Errorf("runContext with empty config should return nil, got %v", err)
	}
}
