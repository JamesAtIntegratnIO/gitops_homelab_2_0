package cmd

import (
	"errors"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/testutil"
)

func TestRunStatusOnceWithClient_CollectError(t *testing.T) {
	fake := &testutil.FakeKubeClient{
		// CollectPlatformStatus calls ListArgoApps — make it fail
		ArgoAppsErr: errors.New("connection refused"),
	}

	cfg := &config.Config{
		Platform: config.PlatformConfig{
			PlatformNamespace: "kratix-platform-system",
		},
	}

	err := runStatusOnceWithClient(fake, cfg)
	if err == nil {
		t.Fatal("expected error when collect fails, got nil")
	}

	var hErr *hcerrors.HctlError
	if !errors.As(err, &hErr) {
		t.Fatalf("expected HctlError, got %T: %v", err, err)
	}
	if hErr.Code != hcerrors.ExitPlatformError {
		t.Errorf("exit code = %d, want %d (ExitPlatformError)", hErr.Code, hcerrors.ExitPlatformError)
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
