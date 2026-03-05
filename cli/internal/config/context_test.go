package config

import (
	"context"
	"testing"
)

func TestNewContext_FromContext_RoundTrip(t *testing.T) {
	cfg := &Config{
		RepoPath: "/my/repo",
		GitMode:  "auto",
		Platform: PlatformConfig{Domain: "test.example.com"},
	}

	ctx := NewContext(context.Background(), cfg)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned false")
	}
	if got.RepoPath != "/my/repo" {
		t.Errorf("RepoPath = %q, want '/my/repo'", got.RepoPath)
	}
	if got.GitMode != "auto" {
		t.Errorf("GitMode = %q, want 'auto'", got.GitMode)
	}
	if got.Platform.Domain != "test.example.com" {
		t.Errorf("Domain = %q, want 'test.example.com'", got.Platform.Domain)
	}
}

func TestFromContext_EmptyContext(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("FromContext on empty context should return false")
	}
}

func TestFromContext_NilConfig(t *testing.T) {
	ctx := context.WithValue(context.Background(), cfgKey{}, (*Config)(nil))

	_, ok := FromContext(ctx)
	if ok {
		t.Error("FromContext with nil Config should return false")
	}
}

func TestNewContext_OverwritesPrevious(t *testing.T) {
	cfg1 := &Config{RepoPath: "/first"}
	cfg2 := &Config{RepoPath: "/second"}

	ctx := NewContext(context.Background(), cfg1)
	ctx = NewContext(ctx, cfg2)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned false")
	}
	if got.RepoPath != "/second" {
		t.Errorf("RepoPath = %q, want '/second' (latest context)", got.RepoPath)
	}
}
