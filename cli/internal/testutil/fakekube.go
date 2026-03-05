// Package testutil provides shared test doubles for the hctl CLI.
package testutil

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

// FakeKubeClient is a configurable mock that satisfies platform.KubeClient.
// For GetVCluster, it supports two modes:
//   - Simple: set VCluster/VClusterErr for a single return value.
//   - Sequential: set VCReturns/VCErrs for call-by-call return values.
//     If VCReturns is non-nil the sequential mode is used.
type FakeKubeClient struct {
	// Simple single-return fields
	VCluster    *unstructured.Unstructured
	VClusterErr error

	// Sequential return fields (GetVCluster returns different values per call)
	VCOnCall  int
	VCReturns []*unstructured.Unstructured
	VCErrs    []error

	VClusters       []unstructured.Unstructured
	VClustersErr    error
	ArgoApps        []unstructured.Unstructured
	ArgoAppsErr     error
	ArgoApp         *unstructured.Unstructured
	ArgoAppErr      error
	ArgoAppsCluster []kube.ArgoAppInfo
	ArgoAppsClErr   error
	ArgoAppsSel     []unstructured.Unstructured
	ArgoAppsSelErr  error
	Pods            []kube.PodInfo
	PodsErr         error
	PodResources    []kube.PodResourceInfo
	PodResourcesErr error
	Jobs            []batchv1.Job
	JobsErr         error
	Works           []unstructured.Unstructured
	WorksErr        error
	WorkPlacements  []unstructured.Unstructured
	WorkPlErr       error
	Nodes           []kube.NodeInfo
	NodesErr        error
	Promises        []unstructured.Unstructured
	PromisesErr     error
}

func (f *FakeKubeClient) GetVCluster(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	if f.VCReturns != nil {
		idx := f.VCOnCall
		f.VCOnCall++
		if idx < len(f.VCReturns) {
			return f.VCReturns[idx], f.VCErrs[idx]
		}
		last := len(f.VCReturns) - 1
		if last >= 0 {
			return f.VCReturns[last], f.VCErrs[last]
		}
		return nil, fmt.Errorf("not found")
	}
	return f.VCluster, f.VClusterErr
}
func (f *FakeKubeClient) ListVClusters(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.VClusters, f.VClustersErr
}
func (f *FakeKubeClient) ListArgoApps(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.ArgoApps, f.ArgoAppsErr
}
func (f *FakeKubeClient) GetArgoApp(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	return f.ArgoApp, f.ArgoAppErr
}
func (f *FakeKubeClient) ListArgoAppsForCluster(_ context.Context, _, _ string) ([]kube.ArgoAppInfo, error) {
	return f.ArgoAppsCluster, f.ArgoAppsClErr
}
func (f *FakeKubeClient) ListArgoAppsWithSelector(_ context.Context, _, _ string) ([]unstructured.Unstructured, error) {
	return f.ArgoAppsSel, f.ArgoAppsSelErr
}
func (f *FakeKubeClient) ListPods(_ context.Context, _, _ string) ([]kube.PodInfo, error) {
	return f.Pods, f.PodsErr
}
func (f *FakeKubeClient) GetPodResourceInfo(_ context.Context, _, _ string) ([]kube.PodResourceInfo, error) {
	return f.PodResources, f.PodResourcesErr
}
func (f *FakeKubeClient) ListJobs(_ context.Context, _, _ string) ([]batchv1.Job, error) {
	return f.Jobs, f.JobsErr
}
func (f *FakeKubeClient) ListWorks(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.Works, f.WorksErr
}
func (f *FakeKubeClient) ListWorkPlacements(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.WorkPlacements, f.WorkPlErr
}
func (f *FakeKubeClient) ListNodes(_ context.Context) ([]kube.NodeInfo, error) {
	return f.Nodes, f.NodesErr
}
func (f *FakeKubeClient) ListPromises(_ context.Context) ([]unstructured.Unstructured, error) {
	return f.Promises, f.PromisesErr
}
