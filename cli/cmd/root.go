package cmd

import (
	"fmt"
	"os"

	"github.com/jamesatintegratnio/hctl/cmd/addon"
	"github.com/jamesatintegratnio/hctl/cmd/deploy"
	"github.com/jamesatintegratnio/hctl/cmd/scale"
	"github.com/jamesatintegratnio/hctl/cmd/secret"
	"github.com/jamesatintegratnio/hctl/cmd/vcluster"
	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"
	// Commit is set at build time via ldflags.
	Commit = "none"

	cfgFile     string
	nonInteract bool
)

var rootCmd = &cobra.Command{
	Use:   "hctl",
	Short: "Homelab platform CLI",
	Long: `hctl is a CLI for the integratn.tech homelab platform.

It provides self-service operations for vClusters, workload deployment (via Score),
addon management, platform diagnostics, and day-to-day operational tasks.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $XDG_CONFIG_HOME/hctl/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&nonInteract, "non-interactive", false, "disable interactive prompts")

	// Register sub-command groups
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(diagnoseCmd)
	rootCmd.AddCommand(reconcileCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(completionCmd)

	rootCmd.AddCommand(vcluster.NewCmd())
	rootCmd.AddCommand(deploy.NewCmd())
	rootCmd.AddCommand(addon.NewCmd())
	rootCmd.AddCommand(scale.NewCmd())
	rootCmd.AddCommand(secret.NewCmd())
}

func initConfig() {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		// Config is optional on first run; init command will create it
		cfg = config.Default()
	}
	if nonInteract {
		cfg.Interactive = false
	}
	config.Set(cfg)
}

// --- Inline simple commands ---

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print hctl version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hctl %s (commit: %s)\n", Version, Commit)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hctl configuration",
	Long:  "Detects the gitops repo, validates cluster access, and writes ~/.config/hctl/config.yaml.",
	RunE:  runInit,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Platform health dashboard",
	Long:  "Shows node health, ArgoCD application status, Kratix promises, and active vClusters.",
	RunE:  runStatus,
}

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose [resource]",
	Short: "Automated troubleshooting",
	Long:  "Walks the resource lifecycle chain (CR → Pipeline → Work → ArgoCD) and reports issues.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDiagnose,
}

var reconcileCmd = &cobra.Command{
	Use:   "reconcile [resource]",
	Short: "Force re-reconciliation of a resource",
	Long:  "Sets the reconcile-at annotation to trigger Kratix pipeline re-execution.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReconcile,
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show current platform context",
	RunE:  runContext,
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for hctl.

Examples:
  # Bash
  source <(hctl completion bash)

  # Zsh
  hctl completion zsh > "${fpath[1]}/_hctl"

  # Fish
  hctl completion fish | source`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}
