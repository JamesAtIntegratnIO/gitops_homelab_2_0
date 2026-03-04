package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"GitMode", d.GitMode, "prompt"},
		{"ArgocdURL", d.ArgocdURL, "https://argocd.cluster.integratn.tech"},
		{"Domain", d.Platform.Domain, "cluster.integratn.tech"},
		{"ClusterSubnet", d.Platform.ClusterSubnet, "10.0.4.0/24"},
		{"MetalLBPool", d.Platform.MetalLBPool, "10.0.4.200-253"},
		{"PlatformNamespace", d.Platform.PlatformNamespace, "platform-requests"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Default().%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if !d.Interactive {
		t.Error("Default().Interactive = false, want true")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	content := `repoPath: /tmp/repo
gitMode: auto
argocdURL: https://argocd.example.com
interactive: false
platform:
  domain: example.com
  clusterSubnet: 10.1.0.0/16
  metalLBPool: 10.1.0.100-200
  platformNamespace: my-ns
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath = %q", cfg.RepoPath)
	}
	if cfg.GitMode != "auto" {
		t.Errorf("GitMode = %q", cfg.GitMode)
	}
	if cfg.ArgocdURL != "https://argocd.example.com" {
		t.Errorf("ArgocdURL = %q", cfg.ArgocdURL)
	}
	if cfg.Interactive {
		t.Error("Interactive = true, want false")
	}
	if cfg.Platform.Domain != "example.com" {
		t.Errorf("Domain = %q", cfg.Platform.Domain)
	}
	if cfg.Platform.PlatformNamespace != "my-ns" {
		t.Errorf("PlatformNamespace = %q", cfg.Platform.PlatformNamespace)
	}
}

func TestLoad_MergesWithDefaults(t *testing.T) {
	content := `repoPath: /tmp/repo
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath = %q", cfg.RepoPath)
	}
	// Defaults should be preserved for unset fields.
	if cfg.GitMode != "prompt" {
		t.Errorf("GitMode = %q, want default %q", cfg.GitMode, "prompt")
	}
	if cfg.Platform.Domain != "cluster.integratn.tech" {
		t.Errorf("Domain = %q, want default", cfg.Platform.Domain)
	}
	if !cfg.Interactive {
		t.Error("Interactive should default to true")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("gitMode: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSetAndGet(t *testing.T) {
	cfg := &Config{
		RepoPath: "/custom/path",
		GitMode:  "auto",
		Platform: PlatformConfig{Domain: "test.example"},
	}
	Set(cfg)
	t.Cleanup(func() { Set(nil) })

	got := Get()
	if got.RepoPath != "/custom/path" {
		t.Errorf("RepoPath = %q", got.RepoPath)
	}
	if got.GitMode != "auto" {
		t.Errorf("GitMode = %q", got.GitMode)
	}
	if got.Platform.Domain != "test.example" {
		t.Errorf("Domain = %q", got.Platform.Domain)
	}
}

func TestGet_NilReturnsDefault(t *testing.T) {
	Set(nil)
	t.Cleanup(func() { Set(nil) })

	got := Get()
	d := Default()
	if got.GitMode != d.GitMode {
		t.Errorf("GitMode = %q, want %q", got.GitMode, d.GitMode)
	}
	if got.Platform.Domain != d.Platform.Domain {
		t.Errorf("Domain = %q, want %q", got.Platform.Domain, d.Platform.Domain)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	// Use a real directory for repoPath so Stat succeeds.
	cfg := &Config{
		RepoPath: t.TempDir(),
		GitMode:  "auto",
		Platform: PlatformConfig{
			Domain:            "example.com",
			PlatformNamespace: "platform",
		},
	}
	errs := Validate(cfg)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantField string
		wantMsg   string
	}{
		{
			name: "InvalidGitMode",
			cfg: Config{
				GitMode: "bogus",
				Platform: PlatformConfig{
					Domain:            "d",
					PlatformNamespace: "ns",
				},
			},
			wantField: "gitMode",
			wantMsg:   "invalid value",
		},
		{
			name: "EmptyGitMode",
			cfg: Config{
				GitMode: "",
				Platform: PlatformConfig{
					Domain:            "d",
					PlatformNamespace: "ns",
				},
			},
			wantField: "gitMode",
			wantMsg:   "not set",
		},
		{
			name: "InvalidOutputFormat",
			cfg: Config{
				GitMode:      "auto",
				OutputFormat: "xml",
				Platform: PlatformConfig{
					Domain:            "d",
					PlatformNamespace: "ns",
				},
			},
			wantField: "outputFormat",
			wantMsg:   "invalid value",
		},
		{
			name: "MissingDomain",
			cfg: Config{
				GitMode: "auto",
				Platform: PlatformConfig{
					Domain:            "",
					PlatformNamespace: "ns",
				},
			},
			wantField: "platform.domain",
			wantMsg:   "not set",
		},
		{
			name: "MissingPlatformNamespace",
			cfg: Config{
				GitMode: "auto",
				Platform: PlatformConfig{
					Domain:            "d",
					PlatformNamespace: "",
				},
			},
			wantField: "platform.platformNamespace",
			wantMsg:   "not set",
		},
		{
			name: "BadRepoPath",
			cfg: Config{
				RepoPath: "/nonexistent/path/should/not/exist",
				GitMode:  "auto",
				Platform: PlatformConfig{
					Domain:            "d",
					PlatformNamespace: "ns",
				},
			},
			wantField: "repoPath",
			wantMsg:   "path does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.cfg)
			found := false
			for _, e := range errs {
				if e.Field == tt.wantField && strings.Contains(e.Message, tt.wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field %q containing %q, got %v", tt.wantField, tt.wantMsg, errs)
			}
		})
	}
}
