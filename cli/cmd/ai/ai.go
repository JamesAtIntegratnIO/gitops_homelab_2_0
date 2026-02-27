package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	aiNamespace     = "ai"
	cronJobName     = "git-indexer"
	jobPrefixManual = "git-indexer-manual-"
)

// NewCmd returns the ai command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI platform operations",
		Long:  "Commands for managing the AI/RAG platform (reindex, status, etc.).",
	}

	cmd.AddCommand(newReindexCmd())

	return cmd
}

func newReindexCmd() *cobra.Command {
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Trigger the git-indexer job",
		Long:  "Creates a one-off Job from the git-indexer CronJob to re-index the repository into the RAG vector store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout+10*time.Second)
			defer cancel()

			// Get the CronJob to extract the job template
			cronJob, err := client.Clientset.BatchV1().CronJobs(aiNamespace).Get(ctx, cronJobName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting cronjob %s/%s: %w", aiNamespace, cronJobName, err)
			}

			// Build a Job from the CronJob template
			jobName := fmt.Sprintf("%s%d", jobPrefixManual, time.Now().Unix())
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: aiNamespace,
					Labels: map[string]string{
						"app":                    "git-indexer",
						"triggered-by":           "hctl",
						"batch.kubernetes.io/job": jobName,
					},
					Annotations: map[string]string{
						"cronjob.kubernetes.io/instantiate": "manual",
					},
				},
				Spec: cronJob.Spec.JobTemplate.Spec,
			}

			created, err := client.Clientset.BatchV1().Jobs(aiNamespace).Create(ctx, job, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating job: %w", err)
			}

			fmt.Printf("  %s Created job %s/%s\n", tui.SuccessStyle.Render(tui.IconCheck), aiNamespace, created.Name)

			if !wait {
				fmt.Println("  Use --wait to follow until completion, or:")
				fmt.Printf("  %s\n", tui.CodeStyle.Render(fmt.Sprintf("kubectl logs -n %s job/%s -f", aiNamespace, created.Name)))
				return nil
			}

			// Poll for job completion
			fmt.Printf("  %s Waiting for completion (timeout: %s)...\n", tui.MutedStyle.Render(tui.IconPending), timeout)
			deadline := time.Now().Add(timeout)
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timed out waiting for job to complete")
				case <-ticker.C:
					j, err := client.Clientset.BatchV1().Jobs(aiNamespace).Get(ctx, created.Name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("checking job status: %w", err)
					}

					for _, cond := range j.Status.Conditions {
						if cond.Type == batchv1.JobComplete && cond.Status == "True" {
							fmt.Printf("  %s Job completed successfully\n", tui.SuccessStyle.Render(tui.IconCheck))
							return nil
						}
						if cond.Type == batchv1.JobFailed && cond.Status == "True" {
							return fmt.Errorf("job failed: %s", cond.Message)
						}
					}

					if time.Now().After(deadline) {
						return fmt.Errorf("timed out after %s — job still running", timeout)
					}
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for the job to complete")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 10*time.Minute, "Timeout when waiting for job completion")

	return cmd
}
