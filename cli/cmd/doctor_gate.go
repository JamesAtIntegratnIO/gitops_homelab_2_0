package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"sigs.k8s.io/yaml"
)

// checkGateInventory compares the checked-in cluster inventory against the
// live ArgoCD cluster Secrets.
//
// This is the only place the staleness can actually be caught. The gate runs
// in CI, which has no cluster access, so its inventory is a snapshot -- and a
// snapshot that has drifted produces a confidently wrong answer rather than an
// error: selectors stop matching, the render shrinks, and two diminished sets
// compare equal. Measured once at 62 Applications down to 7, silently.
//
// So the check belongs here, on a workstation that can see both.
func checkGateInventory(cfg *config.Config) (string, error) {
	root := cfg.RepoPath
	if root == "" {
		root = "."
	}
	invPath, err := gateInventoryPath(root)
	if err != nil {
		// No gate configured is not a failure; most repos do not have one.
		return "no .gitops-gate.yaml (skipped)", nil
	}

	raw, err := os.ReadFile(invPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", invPath, err)
	}
	var stored struct {
		Clusters []struct {
			Name   string            `json:"name"`
			Server string            `json:"server"`
			Labels map[string]string `json:"labels"`
		} `json:"clusters"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		return "", fmt.Errorf("parsing %s: %w", invPath, err)
	}

	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return "", fmt.Errorf("connecting to cluster: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	live, err := client.ListArgoClusterSecrets(ctx, "argocd")
	if err != nil {
		return "", err
	}

	ignore := ignoredLabels(root)
	strip := func(in map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range in {
			if !ignore[k] {
				out[k] = v
			}
		}
		return out
	}

	storedByName := map[string]map[string]string{}
	for _, c := range stored.Clusters {
		storedByName[c.Name] = strip(c.Labels)
	}
	liveByName := map[string]map[string]string{}
	for name, labels := range live {
		liveByName[name] = strip(labels)
	}

	var problems []string
	for name := range liveByName {
		if _, ok := storedByName[name]; !ok {
			problems = append(problems, "cluster "+name+" is registered but missing from the inventory")
		}
	}
	for name := range storedByName {
		if _, ok := liveByName[name]; !ok {
			problems = append(problems, "cluster "+name+" is in the inventory but no longer registered")
		}
	}
	// Label drift is the dangerous kind: selectors match on labels, so a key
	// that appeared or vanished changes which Applications the gate believes
	// exist.
	for name, liveLabels := range liveByName {
		storedLabels, ok := storedByName[name]
		if !ok {
			continue
		}
		for k, v := range liveLabels {
			if sv, ok := storedLabels[k]; !ok {
				problems = append(problems, fmt.Sprintf("%s: label %s=%s is not in the inventory", name, k, v))
			} else if sv != v {
				problems = append(problems, fmt.Sprintf("%s: label %s is %q live but %q in the inventory", name, k, v, sv))
			}
		}
		for k := range storedLabels {
			if _, ok := liveLabels[k]; !ok {
				problems = append(problems, fmt.Sprintf("%s: label %s is in the inventory but gone from the cluster", name, k))
			}
		}
	}

	if len(problems) == 0 {
		age := ""
		if stored.GeneratedAt != "" {
			if t, err := time.Parse(time.RFC3339, stored.GeneratedAt); err == nil {
				age = fmt.Sprintf(", exported %s ago", time.Since(t).Round(time.Hour))
			}
		}
		return fmt.Sprintf("matches %d live cluster(s)%s", len(liveByName), age), nil
	}

	sort.Strings(problems)
	if len(problems) > 4 {
		problems = append(problems[:4], fmt.Sprintf("(and %d more)", len(problems)-4))
	}
	return "", fmt.Errorf("cluster inventory has drifted; the gate's answers cannot be trusted until it is refreshed.\n"+
		"      Run: gitops-gate clusters export -out %s\n      %s",
		mustRel(root, invPath), strings.Join(problems, "\n      "))
}

// gateInventoryPath reads .gitops-gate.yaml to find where the inventory lives.
func gateInventoryPath(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".gitops-gate.yaml"))
	if err != nil {
		return "", err
	}
	var cfg gateConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return "", err
	}
	if cfg.Clusters == "" {
		return "", fmt.Errorf("no `clusters` in .gitops-gate.yaml")
	}
	return filepath.Join(root, cfg.Clusters), nil
}

type gateConfig struct {
	Clusters       string `json:"clusters"`
	ClustersExport struct {
		IgnoreKeys []string `json:"ignoreKeys"`
	} `json:"clustersExport"`
}

// ignoredLabels are keys the exporter strips because they churn without
// affecting any selector or template. Comparing them here would make this
// check fail constantly, and a check that always fails gets switched off --
// which is worse than not having one.
func ignoredLabels(root string) map[string]bool {
	out := map[string]bool{
		"kubectl.kubernetes.io/last-applied-configuration": true,
		"reconcile.external-secrets.io/data-hash":          true,
		"reconcile.external-secrets.io/created-by":         true,
	}
	raw, err := os.ReadFile(filepath.Join(root, ".gitops-gate.yaml"))
	if err != nil {
		return out
	}
	var cfg gateConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return out
	}
	for _, k := range cfg.ClustersExport.IgnoreKeys {
		out[k] = true
	}
	return out
}

func mustRel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}
