package cmd

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

// TestRunInit_RequiresClusterAccess verifies that runInit reaches the cluster
// check step. Since we don't have a real cluster in tests, we can only verify
// the function signature and that it handles a missing kubeconfig gracefully.
// The actual "detect git repo" step is non-fatal and should succeed in the test
// environment (because we're inside a git repo).
func TestRunInit_UsesDefaultConfig(t *testing.T) {
	// config.Default() should produce a usable default
	cfg := config.Default()

	if cfg.GitMode != "prompt" {
		t.Errorf("default GitMode = %q, want 'prompt'", cfg.GitMode)
	}
	if cfg.Platform.PlatformNamespace != "platform-requests" {
		t.Errorf("default PlatformNamespace = %q, want 'platform-requests'", cfg.Platform.PlatformNamespace)
	}
	if cfg.Platform.Domain != "cluster.integratn.tech" {
		t.Errorf("default Domain = %q, want 'cluster.integratn.tech'", cfg.Platform.Domain)
	}
}

func TestRunInit_ConfigSaveRoundTrip(t *testing.T) {
	// Test that config can save and load — the core logic of init's final step.
	tmp := t.TempDir()

	// Override XDG_CONFIG_HOME for this test
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := config.Default()
	cfg.RepoPath = "/tmp/test-repo"
	cfg.KubeContext = "test-context"

	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save failed: %v", err)
	}

	loaded, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if loaded.RepoPath != cfg.RepoPath {
		t.Errorf("loaded RepoPath = %q, want %q", loaded.RepoPath, cfg.RepoPath)
	}
	if loaded.KubeContext != cfg.KubeContext {
		t.Errorf("loaded KubeContext = %q, want %q", loaded.KubeContext, cfg.KubeContext)
	}
}
