package addon

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// resolveLayerPaths returns the addons.yaml path and values directory for a given layer.
func resolveLayerPaths(repoPath, layer, env, clusterRole, cluster, addonName string) (string, string, error) {
	var base string
	switch layer {
	case "environment":
		base = filepath.Join(repoPath, "addons", "environments", env, "addons")
	case "cluster-role":
		if clusterRole == "" {
			return "", "", fmt.Errorf("--cluster-role is required for layer 'cluster-role'")
		}
		base = filepath.Join(repoPath, "addons", "cluster-roles", clusterRole, "addons")
	case "cluster":
		if cluster == "" {
			return "", "", fmt.Errorf("--cluster is required for layer 'cluster'")
		}
		base = filepath.Join(repoPath, "addons", "clusters", cluster, "addons")
	default:
		return "", "", fmt.Errorf("invalid layer %q (must be environment, cluster-role, or cluster)", layer)
	}

	addonsFile := filepath.Join(base, "addons.yaml")
	valuesDir := filepath.Join(base, addonName)
	return addonsFile, valuesDir, nil
}

// readAddonsYAML reads and parses an addons.yaml file into a map of addon entries.
func readAddonsYAML(path string) (map[string]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing addons.yaml: %w", err)
	}

	entries := make(map[string]map[string]interface{})
	for name, val := range raw {
		if m, ok := val.(map[string]interface{}); ok {
			entries[name] = m
		}
	}
	return entries, nil
}

// writeAddonsYAML writes addon entries back to addons.yaml.
func writeAddonsYAML(path string, entries map[string]map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating addons directory: %w", err)
	}

	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshaling addons.yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing addons.yaml: %w", err)
	}
	return nil
}
