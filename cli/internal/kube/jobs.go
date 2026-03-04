package kube

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListJobs returns jobs matching the given label selector in a namespace.
func (c *Client) ListJobs(ctx context.Context, namespace, labelSelector string) ([]batchv1.Job, error) {
	jobs, err := c.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	return jobs.Items, nil
}
