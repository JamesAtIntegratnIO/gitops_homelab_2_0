package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSecretData returns the decoded data from a Secret.
func (c *Client) GetSecretData(ctx context.Context, namespace, name string) (map[string][]byte, error) {
	secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting secret %s/%s: %w", namespace, name, err)
	}
	return secret.Data, nil
}

// WriteKubeconfig writes kubeconfig data to a file.
func WriteKubeconfig(data []byte, name string) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".kube", "hctl")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating kubeconfig directory: %w", err)
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing kubeconfig %s: %w", path, err)
	}
	return path, nil
}
