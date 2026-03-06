package kube

import (
	"bufio"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeInfo is a simplified node representation for display.
type NodeInfo struct {
	Name   string
	Ready  bool
	IP     string
	CPU    string
	Memory string
	Roles  []string
}

// ListNodes returns the cluster nodes.
func (c *Client) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var result []NodeInfo
	for _, n := range nodes.Items {
		info := NodeInfo{
			Name: n.Name,
		}
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" {
				info.Ready = cond.Status == "True"
				break
			}
		}
		if v, ok := n.Status.Allocatable["cpu"]; ok {
			info.CPU = v.String()
		}
		if v, ok := n.Status.Allocatable["memory"]; ok {
			info.Memory = v.String()
		}
		for _, addr := range n.Status.Addresses {
			if addr.Type == "InternalIP" {
				info.IP = addr.Address
				break
			}
		}
		info.Roles = extractRoles(n.Labels)
		result = append(result, info)
	}
	return result, nil
}

func extractRoles(labels map[string]string) []string {
	var roles []string
	for k := range labels {
		if len(k) > 24 && k[:24] == "node-role.kubernetes.io/" {
			roles = append(roles, k[24:])
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "<none>")
	}
	return roles
}

// PodResourceInfo holds pod resource allocation info.
type PodResourceInfo struct {
	Name          string
	Namespace     string
	Phase         string
	MemoryRequest string
	MemoryLimit   string
	CPURequest    string
	CPULimit      string
	Restarts      int
}

// GetPodResourceInfo returns resource allocation info for pods matching a selector.
func (c *Client) GetPodResourceInfo(ctx context.Context, namespace, labelSelector string) ([]PodResourceInfo, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var result []PodResourceInfo
	for _, p := range pods.Items {
		info := PodResourceInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
		}

		// Get resource requests/limits from spec
		for _, container := range p.Spec.Containers {
			if req, ok := container.Resources.Requests["memory"]; ok {
				info.MemoryRequest = req.String()
			}
			if lim, ok := container.Resources.Limits["memory"]; ok {
				info.MemoryLimit = lim.String()
			}
			if req, ok := container.Resources.Requests["cpu"]; ok {
				info.CPURequest = req.String()
			}
			if lim, ok := container.Resources.Limits["cpu"]; ok {
				info.CPULimit = lim.String()
			}
		}

		// Restart count
		for _, cs := range p.Status.ContainerStatuses {
			info.Restarts += int(cs.RestartCount)
		}

		result = append(result, info)
	}
	return result, nil
}

// PodInfo is a simplified pod representation.
type PodInfo struct {
	Name            string
	Namespace       string
	Phase           string
	ReadyContainers int
	TotalContainers int
}

// ListPods returns pods matching a label selector in a namespace.
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) ([]PodInfo, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var result []PodInfo
	for _, p := range pods.Items {
		info := PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
		}
		ready := 0
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}
		info.ReadyContainers = ready
		info.TotalContainers = len(p.Spec.Containers)
		result = append(result, info)
	}
	return result, nil
}

// StreamPodLogs streams logs from a pod to the given writer. If follow is true,
// it streams continuously. It returns when the context is cancelled or the stream ends.
func (c *Client) StreamPodLogs(ctx context.Context, namespace, podName, container string, follow bool, tailLines int64, w io.Writer) error {
	opts := &corev1.PodLogOptions{
		Follow: follow,
	}
	if container != "" {
		opts.Container = container
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	stream, err := c.Clientset.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return fmt.Errorf("opening log stream for %s: %w", podName, err)
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		if _, err := fmt.Fprintln(w, scanner.Text()); err != nil {
			return nil // writer closed
		}
	}
	return scanner.Err()
}
