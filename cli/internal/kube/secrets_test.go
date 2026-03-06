package kube

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestGetSecretData_EmptyData(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-secret",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{},
	}

	client := &Client{Clientset: kubefake.NewSimpleClientset(secret)}

	data, err := client.GetSecretData(context.Background(), "kube-system", "empty-secret")
	if err != nil {
		t.Fatalf("GetSecretData returned error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data map, got %d entries", len(data))
	}
}

func TestGetSecretData_WrongNamespace(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "ns-a",
		},
		Data: map[string][]byte{"key": []byte("val")},
	}

	client := &Client{Clientset: kubefake.NewSimpleClientset(secret)}

	_, err := client.GetSecretData(context.Background(), "ns-b", "my-secret")
	if err == nil {
		t.Fatal("expected error when secret is in different namespace")
	}
}

func TestWriteKubeconfig_Success(t *testing.T) {
	// Not parallel: t.Setenv is incompatible with t.Parallel.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	data := []byte("apiVersion: v1\nkind: Config\n")
	path, err := WriteKubeconfig(data, "test-cluster")
	if err != nil {
		t.Fatalf("WriteKubeconfig returned error: %v", err)
	}

	expected := filepath.Join(tmpHome, ".kube", "hctl", "test-cluster.yaml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("file content = %q, want %q", content, data)
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestWriteKubeconfig_OverwritesExisting(t *testing.T) {
	// Not parallel: t.Setenv is incompatible with t.Parallel.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write initial
	_, err := WriteKubeconfig([]byte("old"), "overwrite-test")
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Overwrite
	path, err := WriteKubeconfig([]byte("new"), "overwrite-test")
	if err != nil {
		t.Fatalf("second write: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "new" {
		t.Errorf("content = %q, want %q", content, "new")
	}
}
