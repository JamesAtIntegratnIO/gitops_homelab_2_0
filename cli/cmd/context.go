package cmd

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func runContext(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	fmt.Println()
	fmt.Println(tui.TitleStyle.Render("  Platform Context"))
	fmt.Println()
	fmt.Println(tui.KeyValue("Repo", tui.ValueOrMuted(cfg.RepoPath, "(not set)")))
	fmt.Println(tui.KeyValue("Git Mode", cfg.GitMode))
	fmt.Println(tui.KeyValue("ArgoCD", cfg.ArgocdURL))
	fmt.Println(tui.KeyValue("Domain", cfg.Platform.Domain))
	fmt.Println(tui.KeyValue("Namespace", cfg.Platform.PlatformNamespace))
	fmt.Println(tui.KeyValue("Config", tui.MutedStyle.Render(config.ConfigPath())))
	fmt.Println()

	return nil
}
