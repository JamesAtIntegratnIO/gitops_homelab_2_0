package kube

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestListJobs_ReturnsMatchingJobs(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-run-abc",
			Namespace: "platform-requests",
			Labels: map[string]string{
				"kratix.io/promise-name": "vcluster",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "pipeline", Image: "pipeline:latest"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
		},
	}

	c := newFakeClient([]runtime.Object{job})
	ctx := context.Background()

	jobs, err := c.ListJobs(ctx, "platform-requests", "kratix.io/promise-name=vcluster")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "pipeline-run-abc" {
		t.Errorf("job name = %q, want 'pipeline-run-abc'", jobs[0].Name)
	}
	if jobs[0].Status.Succeeded != 1 {
		t.Errorf("job succeeded = %d, want 1", jobs[0].Status.Succeeded)
	}
}

func TestListJobs_EmptyNamespace(t *testing.T) {
	c := newFakeClient(nil)
	ctx := context.Background()

	jobs, err := c.ListJobs(ctx, "default", "app=test")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobs_MultipleJobs(t *testing.T) {
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-1",
			Namespace: "ns",
			Labels:    map[string]string{"app": "worker"},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	job2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-2",
			Namespace: "ns",
			Labels:    map[string]string{"app": "worker"},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	c := newFakeClient([]runtime.Object{job1, job2})
	ctx := context.Background()

	jobs, err := c.ListJobs(ctx, "ns", "app=worker")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}
