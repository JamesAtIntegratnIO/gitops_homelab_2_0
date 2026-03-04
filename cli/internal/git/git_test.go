package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initTestRepo creates a temporary git repository with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "main"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}
	return dir
}

func TestDetectRepo_Valid(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)

	repo, err := DetectRepo(dir)
	if err != nil {
		t.Fatalf("DetectRepo: %v", err)
	}
	if repo.Root == "" {
		t.Error("repo.Root should not be empty")
	}
}

func TestDetectRepo_Subdirectory(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	subdir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := DetectRepo(subdir)
	if err != nil {
		t.Fatalf("DetectRepo from subdir: %v", err)
	}
	// The root should be the top-level repo dir
	if !strings.HasPrefix(dir, repo.Root) && !strings.HasPrefix(repo.Root, dir) {
		t.Errorf("repo.Root = %q, should contain %q", repo.Root, dir)
	}
}

func TestDetectRepo_NotARepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := DetectRepo(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestDetectRepo_EmptyDir_UsesCwd(t *testing.T) {
	// Cannot use t.Parallel because it changes the cwd conceptually,
	// but DetectRepo("") uses os.Getwd internally. We just verify it
	// doesn't panic and returns either a repo or an error.
	_, _ = DetectRepo("")
}

func TestRepo_IsClean(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	clean, err := repo.IsClean()
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !clean {
		t.Error("fresh repo should be clean")
	}

	// Create a dirty file
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("change"), 0o644); err != nil {
		t.Fatal(err)
	}

	clean, err = repo.IsClean()
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if clean {
		t.Error("repo with untracked file should not be clean")
	}
}

func TestRepo_Status(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	out, err := repo.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty status for clean repo, got %q", out)
	}

	// Add a file
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hi"), 0o644)
	out, err = repo.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out, "new.txt") {
		t.Errorf("status should mention new.txt, got %q", out)
	}
}

func TestRepo_CurrentBranch(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want 'main'", branch)
	}
}

func TestRepo_Add(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	filePath := filepath.Join(dir, "staged.txt")
	os.WriteFile(filePath, []byte("stage me"), 0o644)

	if err := repo.Add("staged.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify file is staged
	out, _ := repo.Status()
	if !strings.Contains(out, "staged.txt") {
		t.Errorf("staged.txt should appear in status, got %q", out)
	}
}

func TestRepo_Commit(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	// Stage and commit a file
	os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("data"), 0o644)
	repo.Add("committed.txt")

	if err := repo.Commit("test commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	clean, _ := repo.IsClean()
	if !clean {
		t.Error("repo should be clean after commit")
	}
}

func TestRepo_Commit_NothingStaged_Fails(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	repo := &Repo{Root: dir}

	err := repo.Commit("empty commit")
	if err == nil {
		t.Error("expected error when committing with nothing staged")
	}
}

func TestRepo_RelPath(t *testing.T) {
	t.Parallel()
	repo := &Repo{Root: "/home/user/project"}

	tests := []struct {
		abs  string
		want string
	}{
		{"/home/user/project/src/main.go", "src/main.go"},
		{"/home/user/project/file.txt", "file.txt"},
		{"/home/user/project", "."},
	}

	for _, tt := range tests {
		got, err := repo.RelPath(tt.abs)
		if err != nil {
			t.Errorf("RelPath(%q): %v", tt.abs, err)
			continue
		}
		if got != tt.want {
			t.Errorf("RelPath(%q) = %q, want %q", tt.abs, got, tt.want)
		}
	}
}

func TestFormatCommitMessage_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		action, resource, details string
		want                      string
	}{
		{"create vcluster", "media", "prod, 3 replicas", "hctl: create vcluster media (prod, 3 replicas)"},
		{"delete vcluster", "dev", "", "hctl: delete vcluster dev"},
		{"deploy", "sonarr", "vcluster-media", "hctl: deploy sonarr (vcluster-media)"},
		{"enable addon", "kube-prometheus-stack", "", "hctl: enable addon kube-prometheus-stack"},
		{"remove", "app", "cluster=prod", "hctl: remove app (cluster=prod)"},
	}

	for _, tt := range tests {
		t.Run(tt.action+"_"+tt.resource, func(t *testing.T) {
			t.Parallel()
			got := FormatCommitMessage(tt.action, tt.resource, tt.details)
			if got != tt.want {
				t.Errorf("FormatCommitMessage(%q, %q, %q) = %q, want %q",
					tt.action, tt.resource, tt.details, got, tt.want)
			}
		})
	}
}

func TestReconcileAnnotation_Format(t *testing.T) {
	t.Parallel()
	ann := ReconcileAnnotation()
	if ann == "" {
		t.Fatal("ReconcileAnnotation returned empty string")
	}
	// Should be parseable as RFC3339
	parsed, err := time.Parse(time.RFC3339, ann)
	if err != nil {
		t.Errorf("ReconcileAnnotation() = %q, not valid RFC3339: %v", ann, err)
	}
	// Should be recent (within last minute)
	if time.Since(parsed) > time.Minute {
		t.Errorf("ReconcileAnnotation timestamp too old: %v", parsed)
	}
}

func TestReconcileAnnotation_IsUTC(t *testing.T) {
	t.Parallel()
	ann := ReconcileAnnotation()
	if !strings.HasSuffix(ann, "Z") {
		t.Errorf("ReconcileAnnotation should be UTC (end with Z), got %q", ann)
	}
}
