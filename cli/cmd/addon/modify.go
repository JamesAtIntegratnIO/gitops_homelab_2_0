package addon

import (
	"fmt"
	"os"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/tui"
)

// addonModifyOpts holds the layer-resolution flags shared by enable/disable.
type addonModifyOpts struct {
	Env         string
	Layer       string
	ClusterRole string
	Cluster     string
	// AllowCreate when true treats a missing addons.yaml as an empty map
	// instead of returning an error.
	AllowCreate bool
}

// addonMutateResult is returned by the mutation callback to communicate
// additional paths changed and the git-commit action verb.
type addonMutateResult struct {
	ExtraPaths []string // paths changed besides addons.yaml
	Action     string   // e.g. "enable addon", "disable addon"
}

// addonModify is the shared scaffold for enable/disable commands.  It handles
// config validation, path resolution, YAML read-modify-write, and git commit.
// The caller supplies a mutate callback that performs the actual modification.
func addonModify(
	addonName string,
	opts addonModifyOpts,
	mutate func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error),
) error {
	cfg := config.Get()
	if cfg.RepoPath == "" {
		return hcerrors.NewUserError("repo path not set — run 'hctl init'")
	}

	if opts.Env == "" {
		opts.Env = "production"
	}
	if opts.Layer == "" {
		opts.Layer = "environment"
	}

	addonsPath, valuesDir, err := resolveLayerPaths(cfg.RepoPath, opts.Layer, opts.Env, opts.ClusterRole, opts.Cluster, addonName)
	if err != nil {
		return hcerrors.NewUserError("resolving addon paths: %w", err)
	}

	entries, err := readAddonsYAML(addonsPath)
	if err != nil {
		if os.IsNotExist(err) && opts.AllowCreate {
			entries = make(map[string]map[string]interface{})
		} else {
			return hcerrors.NewPlatformError("reading addons config: %w", err)
		}
	}

	mr, err := mutate(entries, addonsPath, valuesDir)
	if err != nil {
		return err
	}

	if err := writeAddonsYAML(addonsPath, entries); err != nil {
		return hcerrors.NewPlatformError("writing addons config: %w", err)
	}

	changedPaths := append([]string{addonsPath}, mr.ExtraPaths...)

	// Git operations
	repo, err := git.DetectRepo(cfg.RepoPath)
	if err != nil {
		fmt.Println(tui.DimStyle.Render("⚠ Git detection failed: " + err.Error() + " — changes not committed"))
		return nil
	}

	var relPaths []string
	for _, p := range changedPaths {
		rp, relErr := repo.RelPath(p)
		if relErr == nil {
			relPaths = append(relPaths, rp)
		}
	}

	if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
		RepoPath:    cfg.RepoPath,
		Paths:       relPaths,
		Action:      mr.Action,
		Resource:    addonName,
		Details:     opts.Layer + "/" + opts.Env,
		GitMode:     cfg.GitMode,
		Interactive: cfg.Interactive,
		UI:          tui.GitUIAdapter{},
	}); err != nil {
		return hcerrors.NewPlatformError("committing addon changes: %w", err)
	}

	return nil
}
