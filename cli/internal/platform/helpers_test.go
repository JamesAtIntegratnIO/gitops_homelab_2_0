package platform

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

// ---------------------------------------------------------------------------
// fakeKubeClient — shared configurable mock for all platform tests
// ---------------------------------------------------------------------------

// fakeKubeClient is a configurable mock that satisfies KubeClient.
// For GetVCluster, it supports two modes:
//   - Simple: set vcluster/vclusterErr for a single return value.
//   - Sequential: set vcReturns/vcErrs for call-by-call return values.
//     If vcReturns is non-nil the sequential mode is used.
type fakeKubeClient struct {
	// Simple single-return fields
	vcluster    *unstructured.Unstructured
	vclusterErr error

	// Sequential return fields (GetVCluster returns different values per call)
	vcOnCall  int
	vcReturns []*unstructured.Unstructured
	vcErrs    []error

	vclusters       []unstructured.Unstructured
	vclustersErr    error
	argoApps        []unstructured.Unstructured
	argoAppsErr     error
	argoApp         *unstructured.Unstructured
	argoAppErr      error
	argoAppsCluster []kube.ArgoAppInfo
	argoAppsClErr   error
	argoAppsSel     []unstructured.Unstructured
	argoAppsSelErr  error
	pods            []kube.PodInfo
	podsErr         error
	podResources    []kube.PodResourceInfo
	podResourcesErr error
	jobs            []batchv1.Job
	jobsErr         error
	works           []unstructured.Unstructured
	worksErr        error
	workPlacements  []unstructured.Unstructured
	workPlErr       error
}

func (f *fakeKubeClient) GetVCluster(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	if f.vcReturns != nil {
		idx := f.vcOnCall
		f.vcOnCall++
		if idx < len(f.vcReturns) {
			return f.vcReturns[idx], f.vcErrs[idx]
		}
		last := len(f.vcReturns) - 1
		if last >= 0 {
			return f.vcReturns[last], f.vcErrs[last]
		}
		return nil, fmt.Errorf("not found")
	}
	return f.vcluster, f.vclusterErr
}
func (f *fakeKubeClient) ListVClusters(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.vclusters, f.vclustersErr
}
func (f *fakeKubeClient) ListArgoApps(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.argoApps, f.argoAppsErr
}
func (f *fakeKubeClient) GetArgoApp(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	return f.argoApp, f.argoAppErr
}
func (f *fakeKubeClient) ListArgoAppsForCluster(_ context.Context, _, _ string) ([]kube.ArgoAppInfo, error) {
	return f.argoAppsCluster, f.argoAppsClErr
}
func (f *fakeKubeClient) ListArgoAppsWithSelector(_ context.Context, _, _ string) ([]unstructured.Unstructured, error) {
	return f.argoAppsSel, f.argoAppsSelErr
}
func (f *fakeKubeClient) ListPods(_ context.Context, _, _ string) ([]kube.PodInfo, error) {
	return f.pods, f.podsErr
}
func (f *fakeKubeClient) GetPodResourceInfo(_ context.Context, _, _ string) ([]kube.PodResourceInfo, error) {
	return f.podResources, f.podResourcesErr
}
func (f *fakeKubeClient) ListJobs(_ context.Context, _, _ string) ([]batchv1.Job, error) {
	return f.jobs, f.jobsErr
}
func (f *fakeKubeClient) ListWorks(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.works, f.worksErr
}
func (f *fakeKubeClient) ListWorkPlacements(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.workPlacements, f.workPlErr
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// contains checks whether sub is a substring of s.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
