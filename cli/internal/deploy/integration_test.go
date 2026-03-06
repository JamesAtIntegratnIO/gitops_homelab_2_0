package deploy_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"gopkg.in/yaml.v3"
)

// TestTranslateAndWrite_Integration exercises the full translate → write pipeline:
//
//  1. Build a score.Workload in-memory
//  2. Call deploy.Translate to get platform values
//  3. Call deploy.WriteResult to write to a temp directory
//  4. Verify output files exist and contain expected content
func TestTranslateAndWrite_Integration(t *testing.T) {
	workload := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata: score.WorkloadMetadata{
			Name: "integration-app",
		},
		Containers: map[string]score.Container{
			"main": {
				Image: "nginx:1.25",
				Variables: map[string]string{
					"PORT": "8080",
				},
			},
		},
		Service: &score.Service{
			Ports: map[string]score.Port{
				"http": {Port: 80, TargetPort: 8080},
			},
		},
		Resources: map[string]score.Resource{
			"db": {
				Type: "postgres",
			},
		},
	}

	cfg := &config.Config{
		DefaultCluster: "test-cluster",
		RepoPath:       t.TempDir(),
	}
	ctx := config.NewContext(context.Background(), cfg)

	// Phase 1: Translate
	result, err := deploy.Translate(ctx, workload, "test-cluster", cfg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if result.WorkloadName != "integration-app" {
		t.Errorf("WorkloadName = %q, want %q", result.WorkloadName, "integration-app")
	}
	if result.TargetCluster != "test-cluster" {
		t.Errorf("TargetCluster = %q, want %q", result.TargetCluster, "test-cluster")
	}
	if result.Namespace != "test-cluster" {
		t.Errorf("Namespace = %q, want %q", result.Namespace, "test-cluster")
	}
	if len(result.Files) == 0 {
		t.Fatal("expected at least one output file")
	}

	// Verify values contain applicationName
	if result.StakaterValues["applicationName"] != "integration-app" {
		t.Errorf("StakaterValues[applicationName] = %v, want %q",
			result.StakaterValues["applicationName"], "integration-app")
	}

	// Verify addons entry
	if result.AddonsEntry["enabled"] != true {
		t.Error("AddonsEntry should have enabled=true")
	}

	// Phase 2: Write
	repoDir := t.TempDir()
	writtenPaths, err := deploy.WriteResult(result, repoDir)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	if len(writtenPaths) == 0 {
		t.Fatal("expected at least one written path")
	}

	// Verify values.yaml was written
	valuesPath := filepath.Join("workloads", "test-cluster", "addons", "integration-app", "values.yaml")
	valuesAbsPath := filepath.Join(repoDir, valuesPath)
	data, err := os.ReadFile(valuesAbsPath)
	if err != nil {
		t.Fatalf("reading values.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Error("values.yaml is empty")
	}

	// Parse and verify values content
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("parsing values.yaml: %v", err)
	}
	if values["applicationName"] != "integration-app" {
		t.Errorf("values.yaml applicationName = %v, want %q", values["applicationName"], "integration-app")
	}

	// Verify addons.yaml was written and contains the workload entry
	addonsAbsPath := filepath.Join(repoDir, "workloads", "test-cluster", "addons.yaml")
	addonsData, err := os.ReadFile(addonsAbsPath)
	if err != nil {
		t.Fatalf("reading addons.yaml: %v", err)
	}
	if !strings.Contains(string(addonsData), "integration-app") {
		t.Error("addons.yaml should contain integration-app entry")
	}

	// Verify the postgres ExternalSecret is referenced in values
	valuesStr := string(data)
	if !strings.Contains(valuesStr, "integration-app-db-credentials") {
		t.Error("values.yaml should reference the postgres ExternalSecret name")
	}
}

// TestTranslateAndWrite_WithRoute_Integration verifies the route provisioner
// integrates correctly through the full pipeline.
func TestTranslateAndWrite_WithRoute_Integration(t *testing.T) {
	workload := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata: score.WorkloadMetadata{
			Name: "web-app",
		},
		Containers: map[string]score.Container{
			"web": {
				Image: "myapp:latest",
			},
		},
		Service: &score.Service{
			Ports: map[string]score.Port{
				"http": {Port: 80, TargetPort: 8080},
			},
		},
		Resources: map[string]score.Resource{
			"ingress": {
				Type: "route",
				Params: map[string]interface{}{
					"host": "web-app.cluster.integratn.tech",
					"path": "/",
					"port": 8080,
				},
			},
		},
	}

	cfg := &config.Config{DefaultCluster: "dev"}
	ctx := config.NewContext(context.Background(), cfg)

	result, err := deploy.Translate(ctx, workload, "dev", cfg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	// Write and verify
	repoDir := t.TempDir()
	_, err = deploy.WriteResult(result, repoDir)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	// Verify values file exists
	valuesPath := filepath.Join(repoDir, "workloads", "dev", "addons", "web-app", "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("reading values.yaml: %v", err)
	}

	// Verify HTTPRoute host appears in the output
	if !strings.Contains(string(data), "web-app.cluster.integratn.tech") {
		t.Error("values.yaml should contain the route hostname")
	}
}
