package vcluster

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

var (
	kubeconfigOutput string
)

func newKubeconfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig [name]",
		Short: "Get vCluster kubeconfig",
		Long: `Extract the kubeconfig for a vCluster.

The kubeconfig is retrieved from the vc-<name> secret in the vCluster's namespace.
By default it writes to ~/.kube/hctl/<name>.yaml.

Examples:
  hctl vcluster kubeconfig my-cluster
  hctl vcluster kubeconfig my-cluster -o /tmp/kubeconfig.yaml
  export KUBECONFIG=$(hctl vcluster kubeconfig my-cluster)`,
		Args: cobra.ExactArgs(1),
		RunE: runKubeconfig,
	}

	cmd.Flags().StringVarP(&kubeconfigOutput, "output", "o", "", "output file path (default: ~/.kube/hctl/<name>.yaml)")
	// TODO: implement --merge to merge into existing KUBECONFIG
	// cmd.Flags().BoolVar(&kubeconfigMerge, "merge", false, "merge into existing KUBECONFIG")

	return cmd
}

// resolveVClusterKubeconfig looks up the kubeconfig for a vCluster by trying
// common secret name patterns and key names. Returns the raw kubeconfig bytes.
func resolveVClusterKubeconfig(ctx context.Context, client *kube.Client, name string) ([]byte, error) {
	secretNames := []string{
		"vc-" + name,                // vCluster default
		name + "-kubeconfig",        // ExternalSecret pattern
		"vc-" + name + "-kubeconfig", // alternate pattern
	}

	for _, secretName := range secretNames {
		data, err := client.GetSecretData(ctx, name, secretName)
		if err != nil {
			continue
		}
		// Look for common kubeconfig keys
		for _, key := range []string{"config", "value", "kubeconfig"} {
			if v, ok := data[key]; ok {
				return v, nil
			}
		}
		// If no known key, try base64 decode of first value
		for _, v := range data {
			decoded, err := base64.StdEncoding.DecodeString(string(v))
			if err == nil && len(decoded) > 0 {
				return decoded, nil
			}
			return v, nil
		}
	}

	return nil, fmt.Errorf("kubeconfig secret not found for vCluster %q — tried: %v", name, secretNames)
}

func runKubeconfig(cmd *cobra.Command, args []string) error {
	name := args[0]

	client, err := kube.Shared()
	if err != nil {
		return hcerrors.NewPlatformError("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	kubeconfigData, err := resolveVClusterKubeconfig(ctx, client, name)
	if err != nil {
		return hcerrors.NewPlatformError("resolving kubeconfig for %s: %w", name, err)
	}

	// Write output
	if kubeconfigOutput != "" {
		if err := writeFile(kubeconfigOutput, kubeconfigData); err != nil {
			return hcerrors.NewPlatformError("writing kubeconfig to %s: %w", kubeconfigOutput, err)
		}
		fmt.Println(kubeconfigOutput)
	} else {
		path, err := kube.WriteKubeconfig(kubeconfigData, name)
		if err != nil {
			return hcerrors.NewPlatformError("writing kubeconfig: %w", err)
		}
		fmt.Printf("%s Kubeconfig written to %s\n", tui.SuccessStyle.Render(tui.IconCheck), path)
		fmt.Printf("\n  %s\n", tui.DimStyle.Render(fmt.Sprintf("export KUBECONFIG=%s", path)))
	}

	return nil
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [name]",
		Short: "Connect to a vCluster (get kubeconfig + set context)",
		Long:  "Retrieves the kubeconfig and sets the current kubectl context to the vCluster.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			client, err := kube.Shared()
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			kubeconfigData, err := resolveVClusterKubeconfig(ctx, client, name)
			if err != nil {
				return hcerrors.NewPlatformError("resolving kubeconfig for %s: %w", name, err)
			}

			path, err := kube.WriteKubeconfig(kubeconfigData, name)
			if err != nil {
				return hcerrors.NewPlatformError("writing kubeconfig: %w", err)
			}

			fmt.Printf("%s Connected to vCluster %s\n", tui.SuccessStyle.Render(tui.IconCheck), name)
			fmt.Printf("\n  Run: %s\n", tui.DimStyle.Render(fmt.Sprintf("export KUBECONFIG=%s", path)))

			return nil
		},
	}
}
