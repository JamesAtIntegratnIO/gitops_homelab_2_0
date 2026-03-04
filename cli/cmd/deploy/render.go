package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDeployRenderCmd() *cobra.Command {
	var (
		cluster   string
		scoreFile string
	)
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render Score workload to Kubernetes manifests",
		Long: `Translates score.yaml and prints the generated platform resources to stdout.
No files are written and no git operations are performed.

Useful for reviewing what will be generated before running 'hctl deploy run'.
Supports --output json/yaml for machine-readable output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workload, err := score.LoadWorkload(scoreFile)
			if err != nil {
				return fmt.Errorf("loading score workload: %w", err)
			}

			result, err := deploylib.Translate(workload, cluster)
			if err != nil {
				return fmt.Errorf("translating workload: %w", err)
			}

			// Structured output: emit the full translation result
			if tui.IsStructured() {
				renderData := map[string]interface{}{
					"workload":       result.WorkloadName,
					"cluster":        result.TargetCluster,
					"namespace":      result.Namespace,
					"stakaterValues": result.StakaterValues,
					"addonsEntry":    result.AddonsEntry,
					"files":          map[string]string{},
				}
				filesMap := renderData["files"].(map[string]string)
				for path, data := range result.Files {
					filesMap[path] = string(data)
				}
				return tui.RenderOutput(renderData, "")
			}

			// Text output: print each file with a header
			for path, data := range result.Files {
				fmt.Printf("%s\n", tui.TitleStyle.Render("# "+path))
				fmt.Println(string(data))
			}

			// Show addons.yaml entry
			fmt.Printf("%s\n", tui.TitleStyle.Render("# addons.yaml entry"))
			entry, _ := yaml.Marshal(map[string]interface{}{result.WorkloadName: result.AddonsEntry})
			fmt.Println(string(entry))

			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	return cmd
}

func newDeployDiffCmd() *cobra.Command {
	var (
		cluster   string
		scoreFile string
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between rendered and deployed manifests",
		Long: `Translates score.yaml and compares the output against what is currently
on disk in the gitops repo. Shows a unified diff for each changed file.

Exit codes: 0 = no changes, 1 = error, 2 = changes detected.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			workload, err := score.LoadWorkload(scoreFile)
			if err != nil {
				return fmt.Errorf("loading score workload: %w", err)
			}

			result, err := deploylib.Translate(workload, cluster)
			if err != nil {
				return fmt.Errorf("translating workload: %w", err)
			}

			hasChanges := false

			// Compare each rendered file against what's on disk
			for relPath, newData := range result.Files {
				absPath := filepath.Join(cfg.RepoPath, relPath)
				existing, readErr := os.ReadFile(absPath)
				if readErr != nil {
					// File doesn't exist yet — show as new
					fmt.Printf("%s %s\n", tui.SuccessStyle.Render("+ new file:"), relPath)
					fmt.Println(string(newData))
					hasChanges = true
					continue
				}

				if string(existing) == string(newData) {
					fmt.Printf("%s %s\n", tui.DimStyle.Render("  unchanged:"), relPath)
					continue
				}

				hasChanges = true
				fmt.Printf("%s %s\n", tui.WarningStyle.Render("~ modified:"), relPath)
				printUnifiedDiff(relPath, string(existing), string(newData))
			}

			// Check addons.yaml for changes
			addonsPath := filepath.Join(cfg.RepoPath, "workloads", result.TargetCluster, "addons.yaml")
			if existingAddons, readErr := os.ReadFile(addonsPath); readErr == nil {
				var existingMap map[string]interface{}
				if yaml.Unmarshal(existingAddons, &existingMap) == nil {
					if existing, ok := existingMap[result.WorkloadName]; ok {
						existingYAML, _ := yaml.Marshal(existing)
						newYAML, _ := yaml.Marshal(result.AddonsEntry)
						if string(existingYAML) != string(newYAML) {
							hasChanges = true
							relPath := filepath.Join("workloads", result.TargetCluster, "addons.yaml")
							fmt.Printf("%s %s (entry: %s)\n", tui.WarningStyle.Render("~ modified:"), relPath, result.WorkloadName)
							printUnifiedDiff(relPath, string(existingYAML), string(newYAML))
						}
					} else {
						hasChanges = true
						fmt.Printf("%s addons.yaml (new entry: %s)\n", tui.SuccessStyle.Render("+ new:"), result.WorkloadName)
					}
				}
			} else {
				hasChanges = true
				fmt.Printf("%s addons.yaml (new file)\n", tui.SuccessStyle.Render("+ new:"))
			}

			if !hasChanges {
				fmt.Println(tui.DimStyle.Render("No changes detected"))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster (overrides score.yaml annotation)")
	cmd.Flags().StringVarP(&scoreFile, "file", "f", "score.yaml", "path to score.yaml")
	return cmd
}

// printUnifiedDiff prints a simple line-by-line diff between two strings.
func printUnifiedDiff(path, old, new string) {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Simple line-by-line comparison (not a true unified diff, but useful)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if i < len(oldLines) {
				fmt.Printf("  %s\n", tui.ErrorStyle.Render("- "+oldLine))
			}
			if i < len(newLines) {
				fmt.Printf("  %s\n", tui.SuccessStyle.Render("+ "+newLine))
			}
		}
	}
	fmt.Println()
}
