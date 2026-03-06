package cmd

import (
	"errors"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/testutil"
)

func TestRunStatusOnceWithClient_CollectError(t *testing.T) {
	fake := &testutil.FakeKubeClient{
		// CollectPlatformStatus accumulates ArgoCD errors as warnings
		// and returns partial results — no hard failure.
		ArgoAppsErr: errors.New("connection refused"),
	}

	cfg := &config.Config{
		Platform: config.PlatformConfig{
			PlatformNamespace: "kratix-platform-system",
		},
	}

	// After graceful-degradation refactor, collect returns partial data + warnings
	// instead of a hard error — so runStatusOnceWithClient should succeed.
	err := runStatusOnceWithClient(fake, cfg)
	if err != nil {
		t.Fatalf("expected graceful degradation (nil error), got: %v", err)
	}
}

func TestRunStatusOnceWithClient_Success(t *testing.T) {
	fake := &testutil.FakeKubeClient{
		// All lists return empty — CollectPlatformStatus should succeed
	}

	cfg := &config.Config{
		Platform: config.PlatformConfig{
			PlatformNamespace: "kratix-platform-system",
		},
	}

	// With default (text) output format, RenderOutput prints empty text — no error.
	err := runStatusOnceWithClient(fake, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
