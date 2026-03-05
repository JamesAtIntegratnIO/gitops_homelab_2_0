package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"gopkg.in/yaml.v3"
)

func TestResolveVariableValue_Literal(t *testing.T) {
	result := resolveVariableValue("hello-world", nil)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["value"] != "hello-world" {
		t.Errorf("value = %v, want 'hello-world'", m["value"])
	}
}

func TestResolveVariableValue_DirectSecretRef(t *testing.T) {
	result := resolveVariableValue("$(my-secret:password)", nil)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	vf, ok := m["valueFrom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected valueFrom map")
	}
	ref, ok := vf["secretKeyRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected secretKeyRef map")
	}
	if ref["name"] != "my-secret" {
		t.Errorf("secretKeyRef name = %v, want 'my-secret'", ref["name"])
	}
	if ref["key"] != "password" {
		t.Errorf("secretKeyRef key = %v, want 'password'", ref["key"])
	}
}

func TestResolveVariableValue_ScoreResourceRef_SecretOutput(t *testing.T) {
	outputs := map[string]map[string]string{
		"db": {
			"host":     "$(db-secret:host)",
			"password": "$(db-secret:password)",
		},
	}

	result := resolveVariableValue("${resources.db.host}", outputs)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	vf, ok := m["valueFrom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected valueFrom")
	}
	ref := vf["secretKeyRef"].(map[string]interface{})
	if ref["name"] != "db-secret" {
		t.Errorf("name = %v, want 'db-secret'", ref["name"])
	}
	if ref["key"] != "host" {
		t.Errorf("key = %v, want 'host'", ref["key"])
	}
}

func TestResolveVariableValue_ScoreResourceRef_LiteralOutput(t *testing.T) {
	outputs := map[string]map[string]string{
		"cache": {
			"host": "redis.default.svc.cluster.local",
		},
	}

	result := resolveVariableValue("${resources.cache.host}", outputs)
	m := result.(map[string]interface{})
	if m["value"] != "redis.default.svc.cluster.local" {
		t.Errorf("value = %v, want literal host", m["value"])
	}
}

func TestResolveVariableValue_UnresolvedReference(t *testing.T) {
	result := resolveVariableValue("${resources.missing.key}", nil)
	m := result.(map[string]interface{})
	if m["value"] != "${resources.missing.key}" {
		t.Errorf("expected unresolved placeholder, got %v", m["value"])
	}
}

func TestBuildContainerSpec_Basic(t *testing.T) {
	c := score.Container{
		Image:   "nginx:1.25",
		Command: []string{"nginx", "-g", "daemon off;"},
		Args:    []string{"-c", "/etc/nginx.conf"},
	}

	spec := buildContainerSpec("web", c, nil)
	if spec["name"] != "web" {
		t.Errorf("name = %v, want 'web'", spec["name"])
	}
	if spec["image"] != "nginx:1.25" {
		t.Errorf("image = %v, want 'nginx:1.25'", spec["image"])
	}
	cmd, ok := spec["command"].([]string)
	if !ok || len(cmd) != 3 {
		t.Errorf("expected 3 command entries, got %v", spec["command"])
	}
	args, ok := spec["args"].([]string)
	if !ok || len(args) != 2 {
		t.Errorf("expected 2 args entries, got %v", spec["args"])
	}
}

func TestBuildContainerSpec_WithVariables(t *testing.T) {
	c := score.Container{
		Image: "app:latest",
		Variables: map[string]string{
			"DB_HOST": "$(db-secret:host)",
			"PORT":    "8080",
		},
	}

	spec := buildContainerSpec("sidecar", c, nil)
	envList, ok := spec["env"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected env list, got %T", spec["env"])
	}
	if len(envList) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(envList))
	}

	// Variables are sorted by name
	if envList[0]["name"] != "DB_HOST" {
		t.Errorf("first env name = %v, want 'DB_HOST'", envList[0]["name"])
	}
	if envList[1]["name"] != "PORT" {
		t.Errorf("second env name = %v, want 'PORT'", envList[1]["name"])
	}
}

func TestUpdateAddonsYAML_NewFile(t *testing.T) {
	tmp := t.TempDir()
	addonsPath := filepath.Join(tmp, "addons.yaml")

	entry := map[string]interface{}{
		"enabled":   true,
		"namespace": "myapp",
	}

	err := updateAddonsYAML(addonsPath, "myapp", entry, "test-cluster")
	if err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(addonsPath)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Should have globalSelectors and the workload entry
	gs, ok := result["globalSelectors"].(map[string]interface{})
	if !ok {
		t.Fatal("expected globalSelectors")
	}
	if gs["cluster_name"] != "test-cluster" {
		t.Errorf("cluster_name = %v, want 'test-cluster'", gs["cluster_name"])
	}

	app, ok := result["myapp"].(map[string]interface{})
	if !ok {
		t.Fatal("expected myapp entry")
	}
	if app["enabled"] != true {
		t.Errorf("myapp enabled = %v, want true", app["enabled"])
	}
}

func TestUpdateAddonsYAML_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	addonsPath := filepath.Join(tmp, "addons.yaml")

	// Write initial file
	initial := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": "my-cluster"},
		"useAddonNameForValues": true,
		"existing-app": map[string]interface{}{
			"enabled":   true,
			"namespace": "existing",
		},
	}
	data, _ := yaml.Marshal(initial)
	if err := os.WriteFile(addonsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Add new workload
	entry := map[string]interface{}{
		"enabled":   true,
		"namespace": "new-app",
	}
	if err := updateAddonsYAML(addonsPath, "new-app", entry, "my-cluster"); err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	// Read back
	data, _ = os.ReadFile(addonsPath)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	// Both should exist
	if _, ok := result["existing-app"]; !ok {
		t.Error("existing-app entry should still exist")
	}
	if _, ok := result["new-app"]; !ok {
		t.Error("new-app entry should exist")
	}
}

func TestRemoveWorkload(t *testing.T) {
	tmp := t.TempDir()

	// Set up addons.yaml
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"globalSelectors": map[string]interface{}{"cluster_name": cluster},
		"myapp": map[string]interface{}{
			"enabled":   true,
			"namespace": "myapp",
		},
		"other-app": map[string]interface{}{
			"enabled":   true,
			"namespace": "other",
		},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	// Create values directory
	valuesDir := filepath.Join(addonsDir, "addons", "myapp")
	os.MkdirAll(valuesDir, 0o755)
	os.WriteFile(filepath.Join(valuesDir, "values.yaml"), []byte("test: true"), 0o644)

	// Remove
	removed, err := RemoveWorkload(tmp, cluster, "myapp")
	if err != nil {
		t.Fatalf("RemoveWorkload: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed paths, got %d", len(removed))
	}

	// Verify addons.yaml no longer has myapp
	data, _ = os.ReadFile(filepath.Join(addonsDir, "addons.yaml"))
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)
	if _, ok := result["myapp"]; ok {
		t.Error("myapp should have been removed from addons.yaml")
	}
	if _, ok := result["other-app"]; !ok {
		t.Error("other-app should still exist")
	}

	// Verify values directory is gone
	if _, err := os.Stat(valuesDir); !os.IsNotExist(err) {
		t.Error("values directory should have been removed")
	}
}

func TestRemoveWorkload_NotFound(t *testing.T) {
	tmp := t.TempDir()
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"existing": map[string]interface{}{"enabled": true},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	_, err := RemoveWorkload(tmp, cluster, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workload")
	}
}

func TestListWorkloads(t *testing.T) {
	tmp := t.TempDir()
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": cluster},
		"useAddonNameForValues": true,
		"app-a": map[string]interface{}{
			"enabled":   true,
			"namespace": "a",
		},
		"app-b": map[string]interface{}{
			"enabled":   false,
			"namespace": "b",
		},
		"app-c": map[string]interface{}{
			"enabled":   true,
			"namespace": "c",
		},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(tmp, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}

	// Should only include enabled workloads, sorted
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d: %v", len(workloads), workloads)
	}
	if workloads[0] != "app-a" || workloads[1] != "app-c" {
		t.Errorf("workloads = %v, want [app-a, app-c]", workloads)
	}
}

func TestListWorkloads_NoFile(t *testing.T) {
	_, err := ListWorkloads("/nonexistent", "cluster")
	if err == nil {
		t.Error("expected error for missing addons.yaml")
	}
}

func TestSecretRefRegex(t *testing.T) {
	tests := []struct {
		input string
		match bool
		name  string
		key   string
	}{
		{"$(my-secret:password)", true, "my-secret", "password"},
		{"$(db-creds:host)", true, "db-creds", "host"},
		{"literal-value", false, "", ""},
		{"${resources.db.host}", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := secretRefRegex.FindStringSubmatch(tt.input)
			if tt.match {
				if len(matches) != 3 {
					t.Fatalf("expected match, got %v", matches)
				}
				if matches[1] != tt.name {
					t.Errorf("name = %q, want %q", matches[1], tt.name)
				}
				if matches[2] != tt.key {
					t.Errorf("key = %q, want %q", matches[2], tt.key)
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("expected no match, got %v", matches)
				}
			}
		})
	}
}

func TestScoreVarRegex(t *testing.T) {
	tests := []struct {
		input   string
		match   bool
		resName string
		resKey  string
	}{
		{"${resources.db.host}", true, "db", "host"},
		{"${resources.cache.port}", true, "cache", "port"},
		{"literal", false, "", ""},
		{"$(secret:key)", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := scoreVarRegex.FindStringSubmatch(tt.input)
			if tt.match {
				if len(matches) != 3 {
					t.Fatalf("expected match, got %v", matches)
				}
				if matches[1] != tt.resName {
					t.Errorf("resName = %q, want %q", matches[1], tt.resName)
				}
				if matches[2] != tt.resKey {
					t.Errorf("resKey = %q, want %q", matches[2], tt.resKey)
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("expected no match, got %v", matches)
				}
			}
		})
	}
}

func TestWriteResult(t *testing.T) {
	tmp := t.TempDir()

	result := &TranslateResult{
		WorkloadName:  "myapp",
		TargetCluster: "test-cluster",
		Namespace:     "myapp",
		StakaterValues: map[string]interface{}{
			"applicationName": "myapp",
		},
		AddonsEntry: map[string]interface{}{
			"enabled":   true,
			"namespace": "myapp",
		},
		Files: map[string][]byte{
			filepath.Join("workloads", "test-cluster", "addons", "myapp", "values.yaml"): []byte("applicationName: myapp\n"),
		},
	}

	paths, err := WriteResult(result, tmp)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	if len(paths) < 2 {
		t.Errorf("expected at least 2 written paths, got %d", len(paths))
	}

	// Verify values file was written
	valuesPath := filepath.Join(tmp, "workloads", "test-cluster", "addons", "myapp", "values.yaml")
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		t.Error("values.yaml should have been created")
	}

	// Verify addons.yaml was created
	addonsPath := filepath.Join(tmp, "workloads", "test-cluster", "addons.yaml")
	if _, err := os.Stat(addonsPath); os.IsNotExist(err) {
		t.Error("addons.yaml should have been created")
	}
}

// ---------------------------------------------------------------------------
// buildDeploymentSection tests
// ---------------------------------------------------------------------------

func TestBuildDeploymentSection_ImageSplitting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		image   string
		wantRepo string
		wantTag  string
	}{
		{"with tag", "nginx:1.25", "nginx", "1.25"},
		{"no tag", "nginx", "nginx", "latest"},
		{"registry with tag", "ghcr.io/org/app:v2.1.0", "ghcr.io/org/app", "v2.1.0"},
		{"sha digest", "nginx:sha256@abcdef", "nginx", "sha256@abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &score.Workload{
				Containers: map[string]score.Container{
					"main": {Image: tt.image},
				},
			}
			deployment, _ := buildDeploymentSection(w, nil)
			img, ok := deployment["image"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected image map, got %T", deployment["image"])
			}
			if img["repository"] != tt.wantRepo {
				t.Errorf("repository = %v, want %q", img["repository"], tt.wantRepo)
			}
			if img["tag"] != tt.wantTag {
				t.Errorf("tag = %v, want %q", img["tag"], tt.wantTag)
			}
		})
	}
}

func TestBuildDeploymentSection_DotImage(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {Image: "."},
		},
	}
	deployment, _ := buildDeploymentSection(w, nil)
	if _, ok := deployment["image"]; ok {
		t.Error("image should not be set when input is '.'")
	}
}

func TestBuildDeploymentSection_ServicePorts(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
		Service: &score.Service{
			Ports: map[string]score.Port{
				"http":  {Port: 8080, Protocol: "TCP"},
				"grpc":  {Port: 9090},
				"admin": {Port: 9000, Protocol: "UDP"},
			},
		},
	}
	deployment, _ := buildDeploymentSection(w, nil)
	ports, ok := deployment["ports"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected ports slice, got %T", deployment["ports"])
	}
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
	// Ports should be sorted by name: admin, grpc, http
	if ports[0]["name"] != "admin" {
		t.Errorf("first port name = %v, want 'admin'", ports[0]["name"])
	}
	if ports[0]["protocol"] != "UDP" {
		t.Errorf("admin protocol = %v, want 'UDP'", ports[0]["protocol"])
	}
	if ports[1]["name"] != "grpc" {
		t.Errorf("second port name = %v, want 'grpc'", ports[1]["name"])
	}
	if ports[1]["protocol"] != "TCP" {
		t.Errorf("grpc protocol should default to TCP, got %v", ports[1]["protocol"])
	}
}

func TestBuildDeploymentSection_EnvVariables(t *testing.T) {
	t.Parallel()
	outputs := map[string]map[string]string{
		"db": {"host": "$(db-secret:host)", "port": "5432"},
	}
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {
				Image: "app:latest",
				Variables: map[string]string{
					"DB_HOST": "${resources.db.host}",
					"DB_PORT": "${resources.db.port}",
					"FIXED":   "static-value",
				},
			},
		},
	}
	deployment, _ := buildDeploymentSection(w, outputs)
	env, ok := deployment["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected env map, got %T", deployment["env"])
	}
	// DB_HOST should be a secretKeyRef
	dbHost, ok := env["DB_HOST"].(map[string]interface{})
	if !ok {
		t.Fatalf("DB_HOST: expected map, got %T", env["DB_HOST"])
	}
	if _, hasValueFrom := dbHost["valueFrom"]; !hasValueFrom {
		t.Error("DB_HOST should have valueFrom (secretKeyRef)")
	}
	// DB_PORT should be a literal from provisioner output
	dbPort, ok := env["DB_PORT"].(map[string]interface{})
	if !ok {
		t.Fatalf("DB_PORT: expected map, got %T", env["DB_PORT"])
	}
	if dbPort["value"] != "5432" {
		t.Errorf("DB_PORT value = %v, want '5432'", dbPort["value"])
	}
	// FIXED should be a literal
	fixed, ok := env["FIXED"].(map[string]interface{})
	if !ok {
		t.Fatalf("FIXED: expected map, got %T", env["FIXED"])
	}
	if fixed["value"] != "static-value" {
		t.Errorf("FIXED value = %v, want 'static-value'", fixed["value"])
	}
}

func TestBuildDeploymentSection_Resources(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {
				Image: "app:latest",
				Resources: &score.ComputeResources{
					Requests: map[string]string{"memory": "256Mi", "cpu": "100m"},
					Limits:   map[string]string{"memory": "512Mi"},
				},
			},
		},
	}
	deployment, _ := buildDeploymentSection(w, nil)
	res, ok := deployment["resources"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected resources map, got %T", deployment["resources"])
	}
	req := res["requests"].(map[string]string)
	if req["memory"] != "256Mi" {
		t.Errorf("memory request = %v, want '256Mi'", req["memory"])
	}
	lim := res["limits"].(map[string]string)
	if lim["memory"] != "512Mi" {
		t.Errorf("memory limit = %v, want '512Mi'", lim["memory"])
	}
}

func TestBuildDeploymentSection_NoResources(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
	}
	deployment, _ := buildDeploymentSection(w, nil)
	if _, ok := deployment["resources"]; ok {
		t.Error("resources should not be set when container has no resource spec")
	}
}

func TestBuildDeploymentSection_Volumes(t *testing.T) {
	t.Parallel()
	outputs := map[string]map[string]string{
		"data": {"source": "my-pvc"},
	}
	w := &score.Workload{
		Containers: map[string]score.Container{
			"main": {
				Image: "app:latest",
				Volumes: map[string]score.Volume{
					"data-vol": {Source: "data", Path: "/var/data", ReadOnly: true},
				},
			},
		},
	}
	deployment, _ := buildDeploymentSection(w, outputs)
	volumes, ok := deployment["volumes"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected volumes map, got %T", deployment["volumes"])
	}
	vol, ok := volumes["data-vol"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data-vol entry")
	}
	pvc := vol["persistentVolumeClaim"].(map[string]interface{})
	if pvc["claimName"] != "my-pvc" {
		t.Errorf("claimName = %v, want 'my-pvc'", pvc["claimName"])
	}
	mounts := deployment["volumeMounts"].(map[string]interface{})
	mount := mounts["data-vol"].(map[string]interface{})
	if mount["mountPath"] != "/var/data" {
		t.Errorf("mountPath = %v, want '/var/data'", mount["mountPath"])
	}
	if mount["readOnly"] != true {
		t.Error("readOnly should be true")
	}
}

func TestBuildDeploymentSection_AdditionalContainers(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Containers: map[string]score.Container{
			"app":     {Image: "app:latest"},
			"sidecar": {Image: "envoy:1.28", Command: []string{"envoy"}},
		},
	}
	_, additional := buildDeploymentSection(w, nil)
	if len(additional) != 1 {
		t.Fatalf("expected 1 additional container, got %d", len(additional))
	}
	// sorted by name: "app" is primary (index 0), "sidecar" is additional
	if additional[0]["name"] != "sidecar" {
		t.Errorf("additional container name = %v, want 'sidecar'", additional[0]["name"])
	}
}

// ---------------------------------------------------------------------------
// buildServiceSection tests
// ---------------------------------------------------------------------------

func TestBuildServiceSection_NilService(t *testing.T) {
	t.Parallel()
	w := &score.Workload{Service: nil}
	if got := buildServiceSection(w); got != nil {
		t.Errorf("expected nil for nil service, got %v", got)
	}
}

func TestBuildServiceSection_EmptyPorts(t *testing.T) {
	t.Parallel()
	w := &score.Workload{Service: &score.Service{Ports: map[string]score.Port{}}}
	if got := buildServiceSection(w); got != nil {
		t.Errorf("expected nil for empty ports, got %v", got)
	}
}

func TestBuildServiceSection_WithPorts(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Service: &score.Service{
			Ports: map[string]score.Port{
				"http": {Port: 80, TargetPort: 8080, Protocol: "TCP"},
				"grpc": {Port: 9090},
			},
		},
	}
	svc := buildServiceSection(w)
	if svc == nil {
		t.Fatal("expected non-nil service section")
	}
	ports, ok := svc["ports"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected ports slice, got %T", svc["ports"])
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	// grpc sorts before http
	if ports[0]["name"] != "grpc" {
		t.Errorf("first port = %v, want 'grpc'", ports[0]["name"])
	}
	if ports[0]["targetPort"] != 9090 {
		t.Errorf("grpc targetPort = %v, want 9090 (same as port when 0)", ports[0]["targetPort"])
	}
	if ports[1]["name"] != "http" {
		t.Errorf("second port = %v, want 'http'", ports[1]["name"])
	}
	if ports[1]["targetPort"] != 8080 {
		t.Errorf("http targetPort = %v, want 8080", ports[1]["targetPort"])
	}
}

// ---------------------------------------------------------------------------
// buildRouteAndCertSections tests
// ---------------------------------------------------------------------------

func TestBuildRouteAndCertSections_NoRouteResource(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "myapp"},
		Resources: map[string]score.Resource{
			"db": {Type: "postgres"},
		},
	}
	values := map[string]interface{}{}
	buildRouteAndCertSections(w, values)
	if _, ok := values["httpRoute"]; ok {
		t.Error("httpRoute should not be set without a route resource")
	}
	if _, ok := values["certificate"]; ok {
		t.Error("certificate should not be set without a route resource")
	}
}

func TestBuildRouteAndCertSections_WithRoute(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "webapp"},
		Resources: map[string]score.Resource{
			"web-route": {
				Type: "route",
				Params: map[string]interface{}{
					"host": "webapp.example.com",
					"port": 3000,
					"path": "/api",
				},
			},
		},
	}
	values := map[string]interface{}{}
	buildRouteAndCertSections(w, values)

	httpRoute, ok := values["httpRoute"].(map[string]interface{})
	if !ok {
		t.Fatal("expected httpRoute map")
	}
	if httpRoute["enabled"] != true {
		t.Error("httpRoute should be enabled")
	}
	hostnames, ok := httpRoute["hostnames"].([]string)
	if !ok || len(hostnames) != 1 || hostnames[0] != "webapp.example.com" {
		t.Errorf("hostnames = %v, want [webapp.example.com]", hostnames)
	}
	rules := httpRoute["rules"].([]map[string]interface{})
	backendRefs := rules[0]["backendRefs"].([]map[string]interface{})
	if backendRefs[0]["port"] != 3000 {
		t.Errorf("backendRef port = %v, want 3000", backendRefs[0]["port"])
	}
	matches := rules[0]["matches"].([]map[string]interface{})
	pathMatch := matches[0]["path"].(map[string]interface{})
	if pathMatch["value"] != "/api" {
		t.Errorf("path value = %v, want '/api'", pathMatch["value"])
	}

	cert, ok := values["certificate"].(map[string]interface{})
	if !ok {
		t.Fatal("expected certificate map")
	}
	if cert["enabled"] != true {
		t.Error("certificate should be enabled")
	}
	if cert["secretName"] != "webapp-tls" {
		t.Errorf("secretName = %v, want 'webapp-tls'", cert["secretName"])
	}
	dnsNames, ok := cert["dnsNames"].([]string)
	if !ok || len(dnsNames) != 1 || dnsNames[0] != "webapp.example.com" {
		t.Errorf("dnsNames = %v, want [webapp.example.com]", dnsNames)
	}
}

func TestBuildRouteAndCertSections_DefaultPortAndPath(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "svc"},
		Resources: map[string]score.Resource{
			"route": {
				Type: "route",
				Params: map[string]interface{}{
					"host": "svc.example.com",
				},
			},
		},
	}
	values := map[string]interface{}{}
	buildRouteAndCertSections(w, values)

	httpRoute := values["httpRoute"].(map[string]interface{})
	rules := httpRoute["rules"].([]map[string]interface{})
	backendRefs := rules[0]["backendRefs"].([]map[string]interface{})
	if backendRefs[0]["port"] != 8080 {
		t.Errorf("default port = %v, want 8080", backendRefs[0]["port"])
	}
	matches := rules[0]["matches"].([]map[string]interface{})
	pathMatch := matches[0]["path"].(map[string]interface{})
	if pathMatch["value"] != "/" {
		t.Errorf("default path = %v, want '/'", pathMatch["value"])
	}
}

func TestBuildRouteAndCertSections_FloatPort(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "svc"},
		Resources: map[string]score.Resource{
			"route": {
				Type: "route",
				Params: map[string]interface{}{
					"host": "svc.example.com",
					"port": float64(443),
				},
			},
		},
	}
	values := map[string]interface{}{}
	buildRouteAndCertSections(w, values)
	rules := values["httpRoute"].(map[string]interface{})["rules"].([]map[string]interface{})
	backendRefs := rules[0]["backendRefs"].([]map[string]interface{})
	if backendRefs[0]["port"] != 443 {
		t.Errorf("float port = %v, want 443", backendRefs[0]["port"])
	}
}

// ---------------------------------------------------------------------------
// buildStakaterValues integration tests
// ---------------------------------------------------------------------------

func TestBuildStakaterValues_MinimalWorkload(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "simple"},
		Containers: map[string]score.Container{
			"main": {Image: "nginx:latest"},
		},
	}
	values := buildStakaterValues(w, nil, "default", nil)

	if values["applicationName"] != "simple" {
		t.Errorf("applicationName = %v, want 'simple'", values["applicationName"])
	}
	// Should have deployment section
	if _, ok := values["deployment"]; !ok {
		t.Error("expected deployment section")
	}
	// Persistence should be disabled
	p, ok := values["persistence"].(map[string]interface{})
	if !ok || p["enabled"] != false {
		t.Error("persistence should be disabled")
	}
	// No service, so no service section
	if _, ok := values["service"]; ok {
		t.Error("service should not be set for workload without service")
	}
	// No route, so no httpRoute or certificate
	if _, ok := values["httpRoute"]; ok {
		t.Error("httpRoute should not be set without route resource")
	}
}

func TestBuildStakaterValues_WithExtraObjects(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "app"},
		Containers: map[string]score.Container{
			"main": {Image: "app:v1"},
		},
	}
	extras := []map[string]interface{}{
		{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "cm1"}},
		{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "cm2"}},
	}
	values := buildStakaterValues(w, nil, "default", extras)

	objs, ok := values["extraObjects"].([]interface{})
	if !ok {
		t.Fatalf("expected extraObjects slice, got %T", values["extraObjects"])
	}
	if len(objs) != 2 {
		t.Errorf("expected 2 extraObjects, got %d", len(objs))
	}
}

func TestBuildStakaterValues_NoExtraObjects(t *testing.T) {
	t.Parallel()
	w := &score.Workload{
		Metadata: score.WorkloadMetadata{Name: "app"},
		Containers: map[string]score.Container{
			"main": {Image: "app:v1"},
		},
	}
	values := buildStakaterValues(w, nil, "default", nil)
	if _, ok := values["extraObjects"]; ok {
		t.Error("extraObjects should not be set when empty")
	}
}

// ---------------------------------------------------------------------------
// Translate integration tests
// ---------------------------------------------------------------------------

func TestTranslate_BasicWorkload(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{DefaultCluster: "test-cluster"}

	w := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata: score.WorkloadMetadata{
			Name: "myapp",
		},
		Containers: map[string]score.Container{
			"main": {Image: "myapp:v1.0"},
		},
		Service: &score.Service{
			Ports: map[string]score.Port{
				"http": {Port: 8080},
			},
		},
	}

	result, err := Translate(context.Background(), w, "test-cluster", cfg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.WorkloadName != "myapp" {
		t.Errorf("WorkloadName = %q, want 'myapp'", result.WorkloadName)
	}
	if result.TargetCluster != "test-cluster" {
		t.Errorf("TargetCluster = %q, want 'test-cluster'", result.TargetCluster)
	}
	if result.Namespace != "test-cluster" {
		t.Errorf("Namespace = %q, want 'test-cluster' (defaults to cluster)", result.Namespace)
	}
	if len(result.Files) == 0 {
		t.Error("expected at least one file in result")
	}
	// Verify values.yaml path
	expectedPath := filepath.Join("workloads", "test-cluster", "addons", "myapp", "values.yaml")
	if _, ok := result.Files[expectedPath]; !ok {
		t.Errorf("missing expected file path %q", expectedPath)
	}
}

func TestTranslate_ClusterFromAnnotation(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{DefaultCluster: "fallback"}

	w := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata: score.WorkloadMetadata{
			Name:        "myapp",
			Annotations: map[string]string{"hctl.integratn.tech/cluster": "annotated-cluster"},
		},
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
	}

	result, err := Translate(context.Background(), w, "", cfg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.TargetCluster != "annotated-cluster" {
		t.Errorf("TargetCluster = %q, want 'annotated-cluster'", result.TargetCluster)
	}
}

func TestTranslate_NamespaceOverride(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{DefaultCluster: "my-cluster"}

	w := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata: score.WorkloadMetadata{
			Name:        "myapp",
			Annotations: map[string]string{"hctl.integratn.tech/namespace": "custom-ns"},
		},
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
	}

	result, err := Translate(context.Background(), w, "my-cluster", cfg)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.Namespace != "custom-ns" {
		t.Errorf("Namespace = %q, want 'custom-ns'", result.Namespace)
	}
}

func TestTranslate_NoCluster_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{DefaultCluster: ""}

	w := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata:   score.WorkloadMetadata{Name: "myapp"},
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
	}

	_, err := Translate(context.Background(), w, "", cfg)
	if err == nil {
		t.Fatal("expected error when no cluster specified")
	}
}

func TestTranslate_UnknownResourceType_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{DefaultCluster: "test"}

	w := &score.Workload{
		APIVersion: "score.dev/v1b1",
		Metadata:   score.WorkloadMetadata{Name: "myapp"},
		Containers: map[string]score.Container{
			"main": {Image: "app:latest"},
		},
		Resources: map[string]score.Resource{
			"unknown": {Type: "nonexistent-resource-type"},
		},
	}

	_, err := Translate(context.Background(), w, "test", cfg)
	if err == nil {
		t.Fatal("expected error for unknown resource type")
	}
}

// ---------------------------------------------------------------------------
// WriteResult / updateAddonsYAML edge cases
// ---------------------------------------------------------------------------

func TestWriteResult_MultipleFiles(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	result := &TranslateResult{
		WorkloadName:  "multi",
		TargetCluster: "c1",
		Namespace:     "c1",
		StakaterValues: map[string]interface{}{
			"applicationName": "multi",
		},
		AddonsEntry: map[string]interface{}{"enabled": true, "namespace": "c1"},
		Files: map[string][]byte{
			filepath.Join("workloads", "c1", "addons", "multi", "values.yaml"): []byte("app: multi\n"),
			filepath.Join("workloads", "c1", "addons", "multi", "extra.yaml"):  []byte("extra: data\n"),
		},
	}

	paths, err := WriteResult(result, tmp)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}
	// 2 data files + 1 addons.yaml
	if len(paths) < 3 {
		t.Errorf("expected at least 3 written paths, got %d: %v", len(paths), paths)
	}
}

func TestListWorkloads_SkipsMetadataKeys(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cluster := "c1"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": cluster},
		"useAddonNameForValues": true,
		"appsetPrefix":          "prefix",
		"real-app": map[string]interface{}{
			"enabled":   true,
			"namespace": "ns",
		},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(tmp, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if len(workloads) != 1 || workloads[0] != "real-app" {
		t.Errorf("workloads = %v, want [real-app]", workloads)
	}
}

func TestRemoveWorkload_NoValuesDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cluster := "c1"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"myapp": map[string]interface{}{"enabled": true, "namespace": "ns"},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	// No values directory to remove — should still succeed
	removed, err := RemoveWorkload(tmp, cluster, "myapp")
	if err != nil {
		t.Fatalf("RemoveWorkload: %v", err)
	}
	// Should have removed addons.yaml entry only
	if len(removed) != 1 {
		t.Errorf("expected 1 removed path, got %d: %v", len(removed), removed)
	}
}
