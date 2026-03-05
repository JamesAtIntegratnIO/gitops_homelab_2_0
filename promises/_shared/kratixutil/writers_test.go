package kratixutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	kratix "github.com/syntasso/kratix-go"
)

// ============================================================================
// WriteYAML
// ============================================================================

func TestWriteYAML_WritesValidYAML(t *testing.T) {
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir))

	obj := Resource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   ObjectMeta{Name: "test", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}

	err := WriteYAML(sdk, "test.yaml", obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "test.yaml"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "apiVersion: v1") {
		t.Errorf("expected 'apiVersion: v1' in output, got:\n%s", content)
	}
	if !strings.Contains(content, "kind: ConfigMap") {
		t.Errorf("expected 'kind: ConfigMap' in output, got:\n%s", content)
	}
	if !strings.Contains(content, "name: test") {
		t.Errorf("expected 'name: test' in output, got:\n%s", content)
	}
}

func TestWriteYAML_SubDirectory(t *testing.T) {
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir))

	obj := Resource{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   ObjectMeta{Name: "web"},
	}

	err := WriteYAML(sdk, "nested/dir/pod.yaml", obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "nested", "dir", "pod.yaml"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "kind: Pod") {
		t.Errorf("expected 'kind: Pod' in output")
	}
}

// ============================================================================
// WriteYAMLDocuments
// ============================================================================

func TestWriteYAMLDocuments_MultiDoc(t *testing.T) {
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir))

	docs := []Resource{
		{
			APIVersion: "v1",
			Kind:       "Service",
			Metadata:   ObjectMeta{Name: "svc-1"},
		},
		{
			APIVersion: "v1",
			Kind:       "Service",
			Metadata:   ObjectMeta{Name: "svc-2"},
		},
	}

	err := WriteYAMLDocuments(sdk, "services.yaml", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "services.yaml"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "name: svc-1") {
		t.Errorf("expected 'name: svc-1' in output")
	}
	if !strings.Contains(content, "name: svc-2") {
		t.Errorf("expected 'name: svc-2' in output")
	}
	if !strings.Contains(content, "---") {
		t.Errorf("expected document separator '---' in output")
	}
}

func TestWriteYAMLDocuments_EmptySlice(t *testing.T) {
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir))

	err := WriteYAMLDocuments(sdk, "empty.yaml", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should not be created for empty docs
	_, err = os.Stat(filepath.Join(dir, "empty.yaml"))
	if err == nil {
		t.Error("expected no file for empty docs")
	}
}

func TestWriteYAMLDocuments_SingleDoc(t *testing.T) {
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir))

	docs := []Resource{
		{
			APIVersion: "v1",
			Kind:       "Secret",
			Metadata:   ObjectMeta{Name: "single"},
		},
	}

	err := WriteYAMLDocuments(sdk, "single.yaml", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "single.yaml"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "---") {
		t.Error("single doc should not contain separator")
	}
	if !strings.Contains(content, "name: single") {
		t.Errorf("expected 'name: single' in output")
	}
}
