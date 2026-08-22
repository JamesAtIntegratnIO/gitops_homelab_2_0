package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// cmdClusters regenerates the inventory from the live ArgoCD cluster Secrets.
//
// The inventory has to be checked in, because CI cannot reach the cluster. That
// makes it a snapshot, and snapshots go stale silently. Running this somewhere
// with cluster access and diffing the result is the only way that drift ever
// surfaces -- so `export` is built to be run in a check, not just by hand.
func cmdClusters(args []string) error {
	if len(args) == 0 || args[0] != "export" {
		return fmt.Errorf("usage: gitops-gate clusters export [-out FILE] [-context CTX] [-namespace NS]")
	}
	fs := flag.NewFlagSet("clusters export", flag.ExitOnError)
	out := fs.String("out", "", "write the inventory here (default: stdout)")
	kubeContext := fs.String("context", "", "kubectl context to read from")
	namespace := fs.String("namespace", "argocd", "namespace holding the ArgoCD cluster Secrets")
	check := fs.Bool("check", false, "compare against an existing inventory and exit non-zero if it has drifted")
	configPath := fs.String("config", ".gitops-gate.yaml", "config to read clustersExport.ignoreKeys from")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	// Pick up site-specific ignore keys if a config is reachable. Export is
	// deliberately usable without one, so a missing config is not an error.
	if cfg, err := LoadConfig(*configPath); err == nil {
		noisyKeys = append(append([]string{}, defaultNoisyKeys...), cfg.ClustersExport.IgnoreKeys...)
		root := filepath.Dir(*configPath)
		if used, err := annotationsUsedBy(root); err == nil && len(used) > 0 {
			keepAnnotations = used
		}
	}

	inv, err := exportClusters(*kubeContext, *namespace)
	if err != nil {
		return err
	}

	rendered, err := yaml.Marshal(inv)
	if err != nil {
		return err
	}

	if *check {
		if *out == "" {
			return fmt.Errorf("-check needs -out to name the inventory to compare against")
		}
		existing, err := os.ReadFile(*out)
		if err != nil {
			return fmt.Errorf("reading %s to compare: %w", *out, err)
		}
		if normalizeInventory(existing) != normalizeInventory(rendered) {
			fmt.Fprintf(os.Stderr, "cluster inventory has drifted from the live cluster.\n"+
				"The gate's targeting check is only as good as this file.\n"+
				"Refresh it with: gitops-gate clusters export -out %s\n", *out)
			return fmt.Errorf("inventory is stale")
		}
		fmt.Fprintln(os.Stderr, "cluster inventory matches the live cluster")
		return nil
	}

	if *out == "" {
		_, err = os.Stdout.Write(rendered)
		return err
	}
	return os.WriteFile(*out, rendered, 0o644)
}

// normalizeInventory drops the generatedAt stamp so a re-export does not report
// drift purely because time passed.
func normalizeInventory(raw []byte) string {
	var inv Inventory
	if err := yaml.Unmarshal(raw, &inv); err != nil {
		return string(raw)
	}
	inv.GeneratedAt = ""
	out, err := yaml.Marshal(inv)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func exportClusters(kubeContext, namespace string) (*Inventory, error) {
	args := []string{"get", "secrets", "-n", namespace,
		"-l", "argocd.argoproj.io/secret-type=cluster", "-o", "json"}
	if kubeContext != "" {
		args = append(args, "--context="+kubeContext)
	}

	cmd := exec.Command("kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl get secrets: %w\n%s", err, stderr.String())
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name        string            `json:"name"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		return nil, fmt.Errorf("parsing kubectl output: %w", err)
	}

	inv := &Inventory{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}

	for _, item := range list.Items {
		labels := stripNoise(item.Metadata.Labels)
		c := Cluster{
			Labels: labels,
			// Annotations are trimmed to what the bootstraps actually
			// template with. Labels are NOT: they are selector inputs, and
			// which ones a future selector will match on is unknowable, so
			// dropping any would reintroduce the stale-fixture failure this
			// export exists to prevent.
			Annotations: keepOnly(stripNoise(item.Metadata.Annotations), keepAnnotations),
		}
		c.Name = decode(item.Data["name"])
		if c.Name == "" {
			c.Name = item.Metadata.Name
		}
		c.Server = decode(item.Data["server"])
		inv.Clusters = append(inv.Clusters, c)
	}
	if len(inv.Clusters) == 0 {
		return nil, fmt.Errorf("no cluster Secrets found in namespace %q", namespace)
	}

	return inv, nil
}

// keepAnnotations is the set of annotation keys the bootstraps reference,
// discovered from their own templates. Empty means keep everything.
var keepAnnotations map[string]bool

func keepOnly(m map[string]string, keep map[string]bool) map[string]string {
	if len(keep) == 0 {
		return m
	}
	out := map[string]string{}
	for k, v := range m {
		if keep[k] {
			out[k] = v
		}
	}
	return out
}

// annotationsUsedBy finds every cluster annotation the repository templates
// with, so the exported inventory carries the handful that are read rather
// than the twenty that happen to exist on the Secret.
//
// Derived rather than configured: a list an operator maintains by hand is a
// list that goes wrong, and the answer is already in the repository.
//
// It scans the WHOLE repository, not just the bootstraps. Scanning only the
// bootstraps was the obvious first guess and it was wrong -- the inner
// ApplicationSets reference `cert_manager_namespace` and
// `external_dns_namespace` too, and trimming those broke their templates.
// Under-collecting here silently drops Applications from the render, so the
// scan errs wide: an annotation kept unnecessarily costs one line.
func annotationsUsedBy(repoRoot string) (map[string]bool, error) {
	re := regexp.MustCompile(`metadata\.annotations(?:\.([A-Za-z0-9_.\-]+)|\[["']([^"']+)["']\])`)
	out := map[string]bool{}

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable paths are not worth failing an export over
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".yaml", ".yml", ".tpl", ".json":
		default:
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			for _, g := range m[1:] {
				if g != "" {
					out[g] = true
				}
			}
		}
		return nil
	})
	return out, err
}

func decode(s string) string {
	if s == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// stripNoise removes keys that churn without changing what any selector or
// template sees -- a resync timestamp, a content hash. Otherwise every export
// reports drift, and a check that always fails gets switched off, which is
// worse than not having it.
//
// The defaults are the ones common to any ArgoCD install. Anything
// site-specific belongs in `clustersExport.ignoreKeys` in .gitops-gate.yaml --
// hardcoding a particular platform's annotation here would be exactly the kind
// of host coupling this package is built to avoid.
var defaultNoisyKeys = []string{
	"kubectl.kubernetes.io/last-applied-configuration",
	"reconcile.external-secrets.io/data-hash",
	"reconcile.external-secrets.io/created-by",
}

// noisyKeys is the effective ignore list, defaults plus configuration.
var noisyKeys = defaultNoisyKeys

func stripNoise(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		skip := false
		for _, n := range noisyKeys {
			if k == n || (strings.HasSuffix(n, "*") && strings.HasPrefix(k, strings.TrimSuffix(n, "*"))) {
				skip = true
				break
			}
		}
		if !skip {
			out[k] = v
		}
	}
	return out
}
