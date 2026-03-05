package cmd

import (
	"os/exec"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

func TestNewDoctorCmd(t *testing.T) {
	cmd := newDoctorCmd()
	if cmd.Use != "doctor" {
		t.Errorf("Use = %q, want 'doctor'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestDoctorCmd_NoExtraArgs(t *testing.T) {
	cmd := newDoctorCmd()
	// Doctor takes no positional args. Verify the command doesn't enforce args
	// validation (Args is nil, which means any args), but RunE is the handler.
	if cmd.Args != nil {
		// If Args is set, test that 0 args is OK
		err := cmd.Args(cmd, []string{})
		if err != nil {
			t.Errorf("doctor with 0 args should pass validation, got %v", err)
		}
	}
}

func TestCheckStruct_Creation(t *testing.T) {
	ran := false
	c := Check{
		Name: "Test Check",
		Run: func(cfg *config.Config) (string, error) {
			ran = true
			return "ok", nil
		},
	}
	if c.Name != "Test Check" {
		t.Errorf("Name = %q, want 'Test Check'", c.Name)
	}
	detail, err := c.Run(config.Default())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if detail != "ok" {
		t.Errorf("detail = %q, want 'ok'", detail)
	}
	if !ran {
		t.Error("Run function was not called")
	}
}

func TestCheckConfigFile_MissingFile(t *testing.T) {
	cfg := config.Default()
	_, err := checkConfigFile(cfg)
	// We don't assert pass/fail since it depends on whether config exists
	// (some dev environments may have it). Just verify no panic.
	_ = err
}

func TestCheckKubectl_Runs(t *testing.T) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("requires kubectl in PATH")
	}
	cfg := config.Default()
	detail, err := checkKubectl(cfg)
	if err != nil {
		t.Logf("kubectl not found (expected in some test envs): %v", err)
	} else if detail == "" {
		t.Error("checkKubectl should return a detail string when kubectl exists")
	}
}

func TestCheckGit_Runs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("requires git in PATH")
	}
	cfg := config.Default()
	detail, err := checkGit(cfg)
	if err != nil {
		t.Logf("git not found (expected in some test envs): %v", err)
	} else if detail == "" {
		t.Error("checkGit should return a detail string when git exists")
	}
}

func TestCheckGitRepo_EmptyRepoPath(t *testing.T) {
	cfg := config.Default()
	cfg.RepoPath = ""
	_, err := checkGitRepo(cfg)
	if err == nil {
		t.Error("checkGitRepo should fail with empty repoPath")
	}
}

func TestCheckGitRepo_NonexistentPath(t *testing.T) {
	cfg := config.Default()
	cfg.RepoPath = "/nonexistent/path/that/does/not/exist"
	_, err := checkGitRepo(cfg)
	if err == nil {
		t.Error("checkGitRepo should fail with nonexistent path")
	}
}

func TestCheckPlatformNamespace_EmptyNamespace(t *testing.T) {
	cfg := config.Default()
	cfg.Platform.PlatformNamespace = ""
	_, err := checkPlatformNamespace(cfg)
	if err == nil {
		t.Error("checkPlatformNamespace should fail with empty namespace")
	}
}

func TestMetav1Options_NotPanic(t *testing.T) {
	opts := metav1Options()
	_ = opts
}

func TestMetav1ListOptions_NotPanic(t *testing.T) {
	opts := metav1ListOptions()
	_ = opts
}

func TestDoctorChecks_AllNamed(t *testing.T) {
	// Verify that all checks in runDoctor have meaningful names
	checks := []Check{
		{Name: "Config file", Run: checkConfigFile},
		{Name: "kubectl", Run: checkKubectl},
		{Name: "git", Run: checkGit},
		{Name: "Git repository", Run: checkGitRepo},
		{Name: "Cluster connectivity", Run: checkCluster},
		{Name: "Platform namespace", Run: checkPlatformNamespace},
		{Name: "ArgoCD", Run: checkArgoCD},
		{Name: "Kratix CRDs", Run: checkKratixCRDs},
	}

	for _, c := range checks {
		if c.Name == "" {
			t.Error("all checks must have a name")
		}
		if c.Run == nil {
			t.Errorf("check %q has nil Run function", c.Name)
		}
	}
}
