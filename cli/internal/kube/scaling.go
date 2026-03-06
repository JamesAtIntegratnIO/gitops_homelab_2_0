package kube

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentInfo holds basic information about a deployment.
type DeploymentInfo struct {
	Name     string
	Replicas int32
	ArgoApp  string // ArgoCD app name from tracking annotation, if any
}

// ListDeployments returns deployment info for all deployments in a namespace.
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]DeploymentInfo, error) {
	deploys, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var result []DeploymentInfo
	for _, d := range deploys.Items {
		info := DeploymentInfo{
			Name:     d.Name,
			Replicas: *d.Spec.Replicas,
		}
		if tracking, ok := d.Annotations["argocd.argoproj.io/tracking-id"]; ok {
			before, _, _ := strings.Cut(tracking, ":")
			info.ArgoApp = before
		}
		result = append(result, info)
	}
	return result, nil
}

// ScaleDeployment sets the replica count for a deployment.
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	scale, err := c.Clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale: %w", err)
	}
	scale.Spec.Replicas = replicas
	_, err = c.Clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling deployment %s: %w", name, err)
	}
	return nil
}
