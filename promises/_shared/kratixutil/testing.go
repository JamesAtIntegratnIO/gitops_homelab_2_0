package kratixutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kratix "github.com/syntasso/kratix-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// MockResource implements kratix.Resource for testing. Exported fields allow
// construction from external test packages.
type MockResource struct {
	Data map[string]interface{}
	Name string
	Ns   string
}

var _ kratix.Resource = (*MockResource)(nil)

func (m *MockResource) GetValue(path string) (interface{}, error) {
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current interface{} = m.Data
	for _, key := range keys {
		if cm, ok := current.(map[string]interface{}); ok {
			val, found := cm[key]
			if !found {
				return nil, fmt.Errorf("path %s not found", path)
			}
			current = val
		} else {
			return nil, fmt.Errorf("path %s not found", path)
		}
	}
	return current, nil
}

func (m *MockResource) GetStatus() (kratix.Status, error)              { return nil, nil }
func (m *MockResource) GetName() string                                { return m.Name }
func (m *MockResource) GetNamespace() string                           { return m.Ns }
func (m *MockResource) GetGroupVersionKind() schema.GroupVersionKind   { return schema.GroupVersionKind{} }
func (m *MockResource) GetLabels() map[string]string                   { return nil }
func (m *MockResource) GetAnnotations() map[string]string              { return nil }
func (m *MockResource) ToUnstructured() unstructured.Unstructured      { return unstructured.Unstructured{} }

// NewTestSDK creates a configured kratix.KratixSDK for testing with a temp
// output directory. Returns the SDK and the output directory path.
func NewTestSDK(t *testing.T) (*kratix.KratixSDK, string) {
	t.Helper()
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir), kratix.WithMetadataDir(dir))
	return sdk, dir
}

// ReadOutput reads a file relative to the output directory and returns its
// content as a string. Fails the test on error.
func ReadOutput(t *testing.T, dir, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

// FileExists reports whether a file exists at the combined dir/path location.
func FileExists(dir, path string) bool {
	_, err := os.Stat(filepath.Join(dir, path))
	return err == nil
}

// ReadOutputAsResources parses multi-document YAML output files into a slice of Resource objects.
// This enables structured field assertions instead of fragile string matching.
func ReadOutputAsResources(t *testing.T, dir, filename string) []Resource {
	t.Helper()
	content := ReadOutput(t, dir, filename)
	if content == "" {
		t.Fatalf("empty output file: %s/%s", dir, filename)
	}

	var resources []Resource
	docs := strings.Split(content, "---")
	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			continue
		}
		var r Resource
		if err := yaml.Unmarshal([]byte(trimmed), &r); err != nil {
			t.Fatalf("failed to unmarshal YAML document in %s: %v", filename, err)
		}
		resources = append(resources, r)
	}
	return resources
}

// FindResource finds the first resource matching the given kind and name.
func FindResource(resources []Resource, kind, name string) *Resource {
	for _, r := range resources {
		if r.Kind == kind && r.Metadata.Name == name {
			return &r
		}
	}
	return nil
}
