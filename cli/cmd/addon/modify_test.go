package addon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

func TestAddonModify_RepoPathRequired(t *testing.T) {
	cfg := &config.Config{RepoPath: "", Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	err := addonModify("test-addon", addonModifyOpts{
		Env:   "production",
		Layer: "environment",
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		t.Fatal("mutate should not be called when repo path is empty")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error when repo path is not set")
	}
}

func TestAddonModify_DefaultsEnvToProduction(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), map[string]map[string]interface{}{
		"my-addon": {"enabled": true},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{RepoPath: tmp, Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	var calledPath string
	_ = addonModify("my-addon", addonModifyOpts{
		Env:   "", // empty -> defaults to production
		Layer: "environment",
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		calledPath = addonsPath
		return &addonMutateResult{Action: "test"}, nil
	})

	wantSuffix := filepath.Join("addons", "environments", "production", "addons", "addons.yaml")
	if !containsSuffix(calledPath, wantSuffix) {
		t.Errorf("addonsPath = %q, want suffix %q", calledPath, wantSuffix)
	}
}

func TestAddonModify_DefaultsLayerToEnvironment(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), map[string]map[string]interface{}{
		"my-addon": {"enabled": true},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{RepoPath: tmp, Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	var calledPath string
	_ = addonModify("my-addon", addonModifyOpts{
		Env:   "production",
		Layer: "", // empty -> defaults to environment
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		calledPath = addonsPath
		return &addonMutateResult{Action: "test"}, nil
	})

	wantSuffix := filepath.Join("addons", "environments", "production", "addons", "addons.yaml")
	if !containsSuffix(calledPath, wantSuffix) {
		t.Errorf("addonsPath = %q, want suffix %q", calledPath, wantSuffix)
	}
}

func TestAddonModify_MutateCallbackWritesBack(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(addonsDir, "addons.yaml")
	if err := writeAddonsYAML(yamlPath, map[string]map[string]interface{}{
		"my-addon": {"enabled": true, "namespace": "ns1"},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{RepoPath: tmp, Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	// Mutate callback modifies entries — should be written back
	_ = addonModify("my-addon", addonModifyOpts{
		Env:   "production",
		Layer: "environment",
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		entries["my-addon"]["enabled"] = false
		return &addonMutateResult{Action: "test modify"}, nil
	})

	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatalf("reading back addons.yaml: %v", err)
	}
	if got["my-addon"]["enabled"] != false {
		t.Errorf("expected enabled=false after mutate, got %v", got["my-addon"]["enabled"])
	}
}

func TestAddonModify_AllowCreateWithMissingFile(t *testing.T) {
	tmp := t.TempDir()
	// Don't create the addons.yaml — test AllowCreate

	cfg := &config.Config{RepoPath: tmp, Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	var gotEntries map[string]map[string]interface{}
	_ = addonModify("new-addon", addonModifyOpts{
		Env:         "production",
		Layer:       "environment",
		AllowCreate: true,
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		gotEntries = entries
		entries["new-addon"] = map[string]interface{}{"enabled": true}
		return &addonMutateResult{Action: "create addon"}, nil
	})

	if gotEntries == nil {
		t.Fatal("mutate callback was not called")
	}
}

func TestAddonModify_FailsWithMissingFileAndNoAllowCreate(t *testing.T) {
	tmp := t.TempDir()

	cfg := &config.Config{RepoPath: tmp, Interactive: false}
	config.Set(cfg)
	defer config.Set(config.Default())

	err := addonModify("new-addon", addonModifyOpts{
		Env:         "production",
		Layer:       "environment",
		AllowCreate: false,
	}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
		t.Fatal("mutate should not be called when file is missing and AllowCreate is false")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error when addons.yaml is missing and AllowCreate is false")
	}
}

func containsSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
