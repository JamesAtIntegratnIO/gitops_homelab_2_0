package cmd

import (
	"context"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/spf13/cobra"
)

// completeVClusterNames provides dynamic completion for vCluster resource names.
func completeVClusterNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := config.Get()
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vclusters, err := client.ListVClusters(ctx, cfg.Platform.PlatformNamespace)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, vc := range vclusters {
		names = append(names, vc.GetName())
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeWorkloadNames provides dynamic completion for workload names.
func completeWorkloadNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := config.Get()
	if cfg.RepoPath == "" || cfg.DefaultCluster == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	workloads, err := deploy.ListWorkloads(cfg.RepoPath, cfg.DefaultCluster)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return workloads, cobra.ShellCompDirectiveNoFileComp
}

// registerCompletions wires up dynamic argument completions for commands.
func registerCompletions() {
	diagnoseCmd.ValidArgsFunction = completeVClusterNames
	reconcileCmd.ValidArgsFunction = completeVClusterNames
	traceCmd.ValidArgsFunction = completeVClusterNames
	upCmd.ValidArgsFunction = completeWorkloadNames
	downCmd.ValidArgsFunction = completeWorkloadNames
	logsCmd.ValidArgsFunction = completeWorkloadNames
	openCmd.ValidArgsFunction = completeWorkloadNames
}
