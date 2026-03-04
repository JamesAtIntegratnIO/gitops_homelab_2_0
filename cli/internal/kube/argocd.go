package kube

import (
	"context"
	"fmt"

	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ArgoCDApplicationGVR is the GroupVersionResource for ArgoCD Application CRs.
var ArgoCDApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

// ArgoAppInfo holds parsed ArgoCD application status for display.
type ArgoAppInfo struct {
	Name         string
	SyncStatus   string
	HealthStatus string
	OpPhase      string
	RetryCount   int64
	OpStartedAt  string
	Message      string
	HasSelfHeal  bool
	DestName     string
}

// ListArgoApps returns all ArgoCD Application resources in the given namespace.
func (c *Client) ListArgoApps(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing argocd applications: %w", err)
	}
	return list.Items, nil
}

// ListArgoAppsWithSelector returns ArgoCD Application resources matching a label selector.
func (c *Client) ListArgoAppsWithSelector(ctx context.Context, namespace, labelSelector string) ([]unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing argocd applications: %w", err)
	}
	return list.Items, nil
}

// GetArgoApp returns a specific ArgoCD Application.
func (c *Client) GetArgoApp(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	obj, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting argocd application %s: %w", name, err)
	}
	return obj, nil
}

// ListArgoAppsForCluster returns all ArgoCD applications targeting a specific cluster destination.
func (c *Client) ListArgoAppsForCluster(ctx context.Context, namespace, clusterName string) ([]ArgoAppInfo, error) {
	apps, err := c.ListArgoApps(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var result []ArgoAppInfo
	for _, app := range apps {
		destName := unstr.MustString(app.Object, "spec", "destination", "name")
		if destName != clusterName {
			continue
		}

		info := ArgoAppInfo{
			Name:     app.GetName(),
			DestName: destName,
		}

		info.SyncStatus = unstr.MustString(app.Object, "status", "sync", "status")
		info.HealthStatus = unstr.MustString(app.Object, "status", "health", "status")

		// Check operation state
		info.OpPhase = unstr.MustString(app.Object, "status", "operationState", "phase")
		info.OpStartedAt = unstr.MustString(app.Object, "status", "operationState", "startedAt")
		info.Message = unstr.MustString(app.Object, "status", "operationState", "message")

		// Check retry count
		retryVal, found, _ := unstr.NestedInt64(app.Object, "status", "operationState", "retryCount")
		if found {
			info.RetryCount = retryVal
		}

		// Check selfHeal policy
		selfHeal, found, _ := unstr.NestedBool(app.Object, "spec", "syncPolicy", "automated", "selfHeal")
		info.HasSelfHeal = found && selfHeal

		result = append(result, info)
	}
	return result, nil
}

// ClearArgoAppOperationState removes the operationState from an ArgoCD application.
func (c *Client) ClearArgoAppOperationState(ctx context.Context, namespace, name string) error {
	patch := []byte(`[{"op": "remove", "path": "/status/operationState"}]`)
	_, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Patch(
		ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{}, "status",
	)
	if err != nil {
		return fmt.Errorf("clearing operation state on %s: %w", name, err)
	}
	return nil
}

// TriggerArgoAppSync triggers a sync operation on an ArgoCD application.
func (c *Client) TriggerArgoAppSync(ctx context.Context, namespace, name string) error {
	// Get current app to read its target revision
	app, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting app %s: %w", name, err)
	}

	revision := unstr.MustString(app.Object, "spec", "source", "targetRevision")

	patch := fmt.Sprintf(`{"operation":{"initiatedBy":{"username":"hctl"},"sync":{"revision":"%s"}}}`, revision)
	_, err = c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("triggering sync on %s: %w", name, err)
	}
	return nil
}

// DisableArgoAutoSync removes the syncPolicy from an ArgoCD application.
func (c *Client) DisableArgoAutoSync(ctx context.Context, argoNamespace, appName string) error {
	patch := []byte(`{"spec":{"syncPolicy":null}}`)
	_, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(argoNamespace).Patch(
		ctx, appName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("disabling auto-sync for %s: %w", appName, err)
	}
	return nil
}

// EnableArgoAutoSync restores the automated sync policy on an ArgoCD application.
func (c *Client) EnableArgoAutoSync(ctx context.Context, argoNamespace, appName string) error {
	patch := []byte(`{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}`)
	_, err := c.Dynamic.Resource(ArgoCDApplicationGVR).Namespace(argoNamespace).Patch(
		ctx, appName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("enabling auto-sync for %s: %w", appName, err)
	}
	return nil
}
