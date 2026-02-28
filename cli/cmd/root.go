package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jamesatintegratnio/hctl/cmd/addon"
	"github.com/jamesatintegratnio/hctl/cmd/ai"
	"github.com/jamesatintegratnio/hctl/cmd/deploy"
	"github.com/jamesatintegratnio/hctl/cmd/scale"
	"github.com/jamesatintegratnio/hctl/cmd/secret"
	"github.com/jamesatintegratnio/hctl/cmd/vcluster"
	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"
	// Commit is set at build time via ldflags.
	Commit = "none"

	cfgFile      string
	nonInteract  bool
	outputFormat string
	verboseFlag  bool
	quietFlag    bool
	watchFlag    bool
	watchInterval time.Duration
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
		os.Exit(hcerrors.ExitCode(err))
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $XDG_CONFIG_HOME/hctl/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&nonInteract, "non-interactive", false, "disable interactive prompts")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "output format: text, json, yaml")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable verbose/debug output")
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "suppress informational output")

	// Register sub-command groups
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "continuously refresh status (structured output only)")
	statusCmd.Flags().DurationVar(&watchInterval, "interval", 10*time.Second, "refresh interval for --watch")
	rootCmd.AddCommand(diagnoseCmd)
	rootCmd.AddCommand(reconcileCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(alertsCmd)
	rootCmd.AddCommand(completionCmd)

	rootCmd.AddCommand(vcluster.NewCmd())
	rootCmd.AddCommand(deploy.NewCmd())
	rootCmd.AddCommand(addon.NewCmd())
	rootCmd.AddCommand(scale.NewCmd())
	rootCmd.AddCommand(secret.NewCmd())
	rootCmd.AddCommand(ai.NewCmd())

	// Convenience commands
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(logsCmd)
}

func initConfig() {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.Default()
		// Auto-create config on first run if no custom path was specified
		if cfgFile == "" {
			if saveErr := config.Save(cfg); saveErr == nil {
				fmt.Fprintf(os.Stderr, "Created default config at %s\n", config.ConfigPath())
			}
		}
	}
	if nonInteract {
		cfg.Interactive = false
	}
	// CLI flags override config file values
	if outputFormat != "" {
		cfg.OutputFormat = outputFormat
	}
	if verboseFlag {
		cfg.Verbose = true
	}
	if quietFlag {
		cfg.Quiet = true
	}
	config.Set(cfg)

	// Wire output format into TUI layer
	if cfg.OutputFormat != "" {
		tui.SetOutputFormat(cfg.OutputFormat)
	}
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
	Long:  "Shows node health, ArgoCD application status, Kratix promises, active vClusters, workloads, and addons.",
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
