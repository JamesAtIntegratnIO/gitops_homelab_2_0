package platform

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

// KubeClient covers read-only query methods used by platform status collection.
// Mutation operations and specialized operations (secrets, CRDs) are accessed
// via kube.Client directly where needed. The interface is intentionally scoped
// for testability — not every kube.Client method belongs here.
// Concrete implementations include *kube.Client; tests can provide fakes.
type KubeClient interface {
	// VCluster CR operations
	GetVCluster(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error)
	ListVClusters(ctx context.Context, namespace string) ([]unstructured.Unstructured, error)

	// ArgoCD Application operations
	ListArgoApps(ctx context.Context, namespace string) ([]unstructured.Unstructured, error)
	GetArgoApp(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error)
	ListArgoAppsForCluster(ctx context.Context, namespace, clusterName string) ([]kube.ArgoAppInfo, error)
	ListArgoAppsWithSelector(ctx context.Context, namespace, labelSelector string) ([]unstructured.Unstructured, error)

	// Pod operations
	ListPods(ctx context.Context, namespace, labelSelector string) ([]kube.PodInfo, error)
	GetPodResourceInfo(ctx context.Context, namespace, labelSelector string) ([]kube.PodResourceInfo, error)

	// Node operations
	ListNodes(ctx context.Context) ([]kube.NodeInfo, error)

	// Kratix / batch operations
	ListPromises(ctx context.Context) ([]unstructured.Unstructured, error)
	ListJobs(ctx context.Context, namespace, labelSelector string) ([]batchv1.Job, error)
	ListWorks(ctx context.Context, namespace string) ([]unstructured.Unstructured, error)
	ListWorkPlacements(ctx context.Context, namespace string) ([]unstructured.Unstructured, error)
}
