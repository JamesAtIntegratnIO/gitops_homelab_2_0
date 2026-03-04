package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// WriteResult writes the translation result to the gitops repo.
func WriteResult(result *TranslateResult, repoPath string) ([]string, error) {
	var writtenPaths []string

	for relPath, data := range result.Files {
		absPath := filepath.Join(repoPath, relPath)
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", relPath, err)
		}
		writtenPaths = append(writtenPaths, relPath)
	}

	// Update addons.yaml
	addonsPath := filepath.Join(repoPath, "workloads", result.TargetCluster, "addons.yaml")
	if err := updateAddonsYAML(addonsPath, result.WorkloadName, result.AddonsEntry, result.TargetCluster); err != nil {
		return nil, fmt.Errorf("updating addons.yaml: %w", err)
	}
	addonsRelPath := filepath.Join("workloads", result.TargetCluster, "addons.yaml")
	writtenPaths = append(writtenPaths, addonsRelPath)

	return writtenPaths, nil
}

// updateAddonsYAML reads or creates the addons.yaml and adds/updates the workload entry.
func updateAddonsYAML(path, workloadName string, entry map[string]interface{}, clusterName string) error {
	var existing map[string]interface{}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing existing addons.yaml: %w", err)
		}
	}

	if existing == nil {
		existing = map[string]interface{}{
			"globalSelectors": map[string]interface{}{
				"cluster_name": clusterName,
			},
			"useAddonNameForValues": true,
		}
	}

	// Add or update the workload entry
	existing[workloadName] = entry

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshaling addons.yaml: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	return os.WriteFile(path, out, 0o644)
}

// RemoveWorkload removes a workload from the addons.yaml and deletes its values directory.
func RemoveWorkload(repoPath, cluster, workloadName string) ([]string, error) {
	var removedPaths []string

	// Remove from addons.yaml
	addonsPath := filepath.Join(repoPath, "workloads", cluster, "addons.yaml")
	data, err := os.ReadFile(addonsPath)
	if err != nil {
		return nil, fmt.Errorf("reading addons.yaml: %w", err)
	}

	var existing map[string]interface{}
	if err := yaml.Unmarshal(data, &existing); err != nil {
		return nil, fmt.Errorf("parsing addons.yaml: %w", err)
	}

	if _, ok := existing[workloadName]; !ok {
		return nil, fmt.Errorf("workload %q not found in addons.yaml", workloadName)
	}

	delete(existing, workloadName)

	out, err := yaml.Marshal(existing)
	if err != nil {
		return nil, fmt.Errorf("marshaling addons.yaml: %w", err)
	}

	if err := os.WriteFile(addonsPath, out, 0o644); err != nil {
		return nil, fmt.Errorf("writing addons.yaml: %w", err)
	}
	removedPaths = append(removedPaths, filepath.Join("workloads", cluster, "addons.yaml"))

	// Remove values directory
	valuesDir := filepath.Join(repoPath, "workloads", cluster, "addons", workloadName)
	if _, err := os.Stat(valuesDir); err == nil {
		if err := os.RemoveAll(valuesDir); err != nil {
			return nil, fmt.Errorf("removing values directory: %w", err)
		}
		removedPaths = append(removedPaths, filepath.Join("workloads", cluster, "addons", workloadName))
	}

	return removedPaths, nil
}

// ListWorkloads reads a cluster's addons.yaml and returns all enabled workload names.
func ListWorkloads(repoPath, cluster string) ([]string, error) {
	addonsPath := filepath.Join(repoPath, "workloads", cluster, "addons.yaml")
	data, err := os.ReadFile(addonsPath)
	if err != nil {
		return nil, err
	}

	var existing map[string]interface{}
	if err := yaml.Unmarshal(data, &existing); err != nil {
		return nil, err
	}

	// Skip non-addon keys
	skipKeys := map[string]bool{
		"globalSelectors":       true,
		"useAddonNameForValues": true,
		"appsetPrefix":          true,
	}

	var workloads []string
	for name, val := range existing {
		if skipKeys[name] {
			continue
		}
		if entry, ok := val.(map[string]interface{}); ok {
			if enabled, ok := entry["enabled"].(bool); ok && enabled {
				workloads = append(workloads, name)
			}
		}
	}
	sort.Strings(workloads)
	return workloads, nil
}
