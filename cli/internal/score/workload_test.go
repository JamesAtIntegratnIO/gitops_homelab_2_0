package score

import (
	"os"
	"path/filepath"
	"testing"
)

const validScoreYAML = `apiVersion: score.dev/v1b1
metadata:
  name: test-app
  annotations:
    hctl.integratn.tech/cluster: my-cluster
containers:
  main:
    image: nginx:latest
    variables:
      PORT: "8080"
service:
  ports:
    http:
      port: 80
      targetPort: 8080
resources:
  db:
    type: postgres
    class: default
  cache:
    type: redis
`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadWorkload_Valid(t *testing.T) {
	path := writeTemp(t, "score.yaml", validScoreYAML)

	w, err := LoadWorkload(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.APIVersion != "score.dev/v1b1" {
		t.Errorf("APIVersion = %q, want %q", w.APIVersion, "score.dev/v1b1")
	}
	if w.Metadata.Name != "test-app" {
		t.Errorf("Metadata.Name = %q, want %q", w.Metadata.Name, "test-app")
	}
	if w.Metadata.Annotations["hctl.integratn.tech/cluster"] != "my-cluster" {
		t.Errorf("cluster annotation = %q, want %q", w.Metadata.Annotations["hctl.integratn.tech/cluster"], "my-cluster")
	}
	c, ok := w.Containers["main"]
	if !ok {
		t.Fatal("container 'main' not found")
	}
	if c.Image != "nginx:latest" {
		t.Errorf("Image = %q, want %q", c.Image, "nginx:latest")
	}
	if c.Variables["PORT"] != "8080" {
		t.Errorf("Variables[PORT] = %q, want %q", c.Variables["PORT"], "8080")
	}
	if w.Service == nil {
		t.Fatal("Service is nil")
	}
	hp, ok := w.Service.Ports["http"]
	if !ok {
		t.Fatal("port 'http' not found")
	}
	if hp.Port != 80 || hp.TargetPort != 8080 {
		t.Errorf("Port = %d/%d, want 80/8080", hp.Port, hp.TargetPort)
	}
	if len(w.Resources) != 2 {
		t.Fatalf("len(Resources) = %d, want 2", len(w.Resources))
	}
	if w.Resources["db"].Type != "postgres" {
		t.Errorf("db.Type = %q, want %q", w.Resources["db"].Type, "postgres")
	}
	if w.Resources["cache"].Type != "redis" {
		t.Errorf("cache.Type = %q, want %q", w.Resources["cache"].Type, "redis")
	}
}

func TestLoadWorkload_Errors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "WrongAPIVersion",
			yaml: `apiVersion: score.dev/v999
metadata:
  name: app
containers:
  main:
    image: nginx
`,
			wantErr: "unsupported Score API version",
		},
		{
			name: "MissingName",
			yaml: `apiVersion: score.dev/v1b1
metadata:
  name: ""
containers:
  main:
    image: nginx
`,
			wantErr: "metadata.name is required",
		},
		{
			name: "NoContainers",
			yaml: `apiVersion: score.dev/v1b1
metadata:
  name: app
containers: {}
`,
			wantErr: "at least one container is required",
		},
		{
			name: "InvalidYAML",
			yaml: `apiVersion: score.dev/v1b1
  bad indent: [
`,
			wantErr: "parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "score.yaml", tt.yaml)
			_, err := LoadWorkload(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestLoadWorkload_FileNotFound(t *testing.T) {
	_, err := LoadWorkload(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestTargetCluster(t *testing.T) {
	w := &Workload{
		Metadata: WorkloadMetadata{
			Annotations: map[string]string{
				"hctl.integratn.tech/cluster": "media-cluster",
			},
		},
	}
	if got := w.TargetCluster(); got != "media-cluster" {
		t.Errorf("TargetCluster() = %q, want %q", got, "media-cluster")
	}
}

func TestTargetCluster_NoAnnotations(t *testing.T) {
	w := &Workload{Metadata: WorkloadMetadata{}}
	if got := w.TargetCluster(); got != "" {
		t.Errorf("TargetCluster() = %q, want %q", got, "")
	}
}

func TestResourcesByType(t *testing.T) {
	w := &Workload{
		Resources: map[string]Resource{
			"db":    {Type: "postgres"},
			"cache": {Type: "redis"},
			"db2":   {Type: "postgres"},
		},
	}

	pg := w.ResourcesByType("postgres")
	if len(pg) != 2 {
		t.Fatalf("len = %d, want 2", len(pg))
	}
	for name, r := range pg {
		if r.Type != "postgres" {
			t.Errorf("resource %q has type %q, want %q", name, r.Type, "postgres")
		}
	}

	rd := w.ResourcesByType("redis")
	if len(rd) != 1 {
		t.Errorf("len = %d, want 1", len(rd))
	}
}

func TestResourcesByType_None(t *testing.T) {
	w := &Workload{
		Resources: map[string]Resource{
			"db": {Type: "postgres"},
		},
	}
	got := w.ResourcesByType("s3")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
