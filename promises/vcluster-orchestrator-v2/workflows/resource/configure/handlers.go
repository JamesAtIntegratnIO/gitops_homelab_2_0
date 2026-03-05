package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func handleConfigure(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Println("--- Rendering orchestrator resources ---")

	resourceRequests := map[string]u.Resource{
		"resources/argocd-project-request.yaml":              buildArgoCDProjectRequest(config),
		"resources/argocd-application-request.yaml":          buildArgoCDApplicationRequest(config),
		"resources/argocd-cluster-registration-request.yaml": buildArgoCDClusterRegistrationRequest(config),
	}

	for path, obj := range resourceRequests {
		if err := u.WriteYAML(sdk, path, obj); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		log.Printf("✓ Rendered: %s", path)
	}

	if err := u.WriteYAML(sdk, "resources/namespace.yaml", buildNamespace(config)); err != nil {
		return fmt.Errorf("write namespace: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/namespace.yaml")

	if docs := buildEtcdCertificates(config); len(docs) > 0 {
		if err := u.WriteYAMLDocuments(sdk, "resources/etcd-certificates.yaml", docs); err != nil {
			return fmt.Errorf("write etcd certificates: %w", err)
		}
		log.Printf("✓ Rendered: %s", "resources/etcd-certificates.yaml")
	}

	if err := u.WriteYAML(sdk, "resources/coredns-configmap.yaml", buildCorednsConfigMap(config)); err != nil {
		return fmt.Errorf("write coredns configmap: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/coredns-configmap.yaml")

	// Per-vcluster network policies (NFS, extra egress)
	netPolicies := buildNetworkPolicies(config)
	if len(netPolicies) > 0 {
		if err := u.WriteYAMLDocuments(sdk, "resources/network-policies.yaml", netPolicies); err != nil {
			return fmt.Errorf("write network policies: %w", err)
		}
		log.Printf("✓ Rendered: resources/network-policies.yaml (%d policies)", len(netPolicies))
	}

	directResources := 2 // namespace + coredns configmap
	if etcdEnabled(config) {
		directResources++
	}
	if len(netPolicies) > 0 {
		directResources++
	}

	status := kratix.NewStatus()
	status.Set("phase", "Scheduled")
	status.Set("message", "VCluster resources scheduled for creation")
	status.Set("resourceRequestsGenerated", len(resourceRequests))
	status.Set("directResourcesGenerated", directResources)
	status.Set("vclusterName", config.Name)
	status.Set("targetNamespace", config.TargetNamespace)
	status.Set("hostname", config.Hostname)
	status.Set("environment", config.ArgoCDEnvironment)

	// Platform Status Contract — endpoint and credential references
	status.Set("endpoints", map[string]string{
		"api":    config.ExternalServerURL,
		"argocd": fmt.Sprintf("https://argocd.cluster.integratn.tech/applications/vcluster-%s", config.Name),
	})
	status.Set("credentials", map[string]string{
		"kubeconfigSecret": fmt.Sprintf("vcluster-%s-kubeconfig", config.Name),
		"onePasswordItem":  config.OnePasswordItem,
	})

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	log.Println("✓ Status updated")
	return nil
}

// cleanupHostPVs deletes host-level PersistentVolumes that were created by the
// vcluster syncer. These PVs are NOT in the Kratix state store, so they cannot
// be cleaned up via the normal Kratix output mechanism. They must be deleted via
// direct Kubernetes API calls using the pipeline pod's ServiceAccount.
//
// Synced PVs are identified by the label:
//
//	vcluster.loft.sh/managed-by: {vcluster-name}-x-{namespace}
func cleanupHostPVs(config *VClusterConfig) error {
	labelValue := fmt.Sprintf("%s-x-%s", config.Name, config.TargetNamespace)
	labelSelector := fmt.Sprintf("vcluster.loft.sh/managed-by=%s", labelValue)

	log.Printf("Cleaning up host PVs with label selector: %s", labelSelector)

	clientset, err := config.KubeClient.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pvList, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list PVs with selector %s: %w", labelSelector, err)
	}

	if len(pvList.Items) == 0 {
		log.Println("No host PVs found to clean up")
		return nil
	}

	log.Printf("Found %d host PV(s) to delete", len(pvList.Items))

	var errs []error
	for _, pv := range pvList.Items {
		log.Printf("  Deleting PV: %s (status: %s)", pv.Name, pv.Status.Phase)
		if err := clientset.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete PV %s: %w", pv.Name, err))
			log.Printf("  ✗ Failed to delete PV %s: %v", pv.Name, err)
		} else {
			log.Printf("  ✓ Deleted PV: %s", pv.Name)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.Printf("✓ Successfully cleaned up %d host PV(s)", len(pvList.Items))
	return nil
}

// cleanupNamespace deletes the vcluster target namespace, which cascade-deletes
// all namespace-scoped resources (PVCs, pods, services, etc.).
// This ensures no orphaned resources remain after vcluster deletion.
func cleanupNamespace(config *VClusterConfig) error {
	log.Printf("Cleaning up namespace: %s", config.TargetNamespace)

	clientset, err := config.KubeClient.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Check if namespace exists before trying to delete
	_, err = clientset.CoreV1().Namespaces().Get(ctx, config.TargetNamespace, metav1.GetOptions{})
	if err != nil {
		log.Printf("Namespace %s not found or already deleted, skipping", config.TargetNamespace)
		return nil
	}

	if err := clientset.CoreV1().Namespaces().Delete(ctx, config.TargetNamespace, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", config.TargetNamespace, err)
	}

	log.Printf("✓ Namespace %s scheduled for deletion", config.TargetNamespace)
	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Printf("--- Handling delete for vcluster: %s ---", config.Name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", "VCluster resources scheduled for deletion")
	status.Set("vclusterName", config.Name)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	// --- Direct API cleanup for resources NOT in the Kratix state store ---
	// These resources are created by the vcluster syncer directly on the host
	// cluster and must be cleaned up via direct API calls.

	var cleanupErrs []string

	// Clean up host-level PVs created by the vcluster syncer
	if err := cleanupHostPVs(config); err != nil {
		log.Printf("⚠ Warning: PV cleanup encountered errors: %v", err)
		cleanupErrs = append(cleanupErrs, fmt.Sprintf("PV cleanup: %v", err))
	}

	// Clean up the vcluster namespace (cascade-deletes PVCs, pods, etc.)
	if err := cleanupNamespace(config); err != nil {
		log.Printf("⚠ Warning: Namespace cleanup encountered errors: %v", err)
		cleanupErrs = append(cleanupErrs, fmt.Sprintf("namespace cleanup: %v", err))
	}

	// --- Kratix state store cleanup (removes manifests → ArgoCD deletes from cluster) ---

	outputs := map[string]u.Resource{}

	// Delete all created resources
	allResources := []u.Resource{
		buildArgoCDProjectRequest(config),
		buildArgoCDApplicationRequest(config),
		buildArgoCDClusterRegistrationRequest(config),
		buildCorednsConfigMap(config),
	}

	for _, obj := range allResources {
		deleteObj := u.DeleteFromResource(obj)
		path := u.DeleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	// Delete per-vcluster network policies
	for _, obj := range buildNetworkPolicies(config) {
		deleteObj := u.DeleteFromResource(obj)
		path := u.DeleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	if etcdEnabled(config) {
		for _, obj := range buildEtcdCertificates(config) {
			deleteObj := u.DeleteFromResource(obj)
			path := u.DeleteOutputPathForResource("resources", obj)
			outputs[path] = deleteObj
		}
	}

	outputs["resources/delete-vcluster-clusterrole.yaml"] = u.DeleteResource(
		"rbac.authorization.k8s.io/v1",
		"ClusterRole",
		fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
		"",
	)
	outputs["resources/delete-vcluster-clusterrolebinding.yaml"] = u.DeleteResource(
		"rbac.authorization.k8s.io/v1",
		"ClusterRoleBinding",
		fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
		"",
	)

	if etcdEnabled(config) {
		outputs["resources/delete-etcd-ca-secret.yaml"] = u.DeleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-server-secret.yaml"] = u.DeleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-peer-secret.yaml"] = u.DeleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-merged-secret.yaml"] = u.DeleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-certs", config.Name),
			config.TargetNamespace,
		)
	}

	for path, obj := range outputs {
		if err := u.WriteYAML(sdk, path, obj); err != nil {
			return fmt.Errorf("write delete output %s: %w", path, err)
		}
	}

	if len(cleanupErrs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(cleanupErrs, "; "))
	}

	return nil
}
