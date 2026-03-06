package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func handleConfigure(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Println("--- Rendering orchestrator resources ---")

	resourceRequests := map[string]ku.Resource{
		"resources/argocd-project-request.yaml":              buildArgoCDProjectRequest(config),
		"resources/argocd-application-request.yaml":          buildArgoCDApplicationRequest(config),
		"resources/argocd-cluster-registration-request.yaml": buildArgoCDClusterRegistrationRequest(config),
	}

	if err := ku.WriteOrderedResources(sdk, resourceRequests); err != nil {
		return fmt.Errorf("write resource requests: %w", err)
	}

	if err := ku.WriteYAML(sdk, "resources/namespace.yaml", buildNamespace(config)); err != nil {
		return fmt.Errorf("write namespace: %w", err)
	}

	if docs := buildEtcdCertificates(config); len(docs) > 0 {
		if err := ku.WriteYAMLDocuments(sdk, "resources/etcd-certificates.yaml", docs); err != nil {
			return fmt.Errorf("write etcd certificates: %w", err)
		}
	}

	if err := ku.WriteYAML(sdk, "resources/coredns-configmap.yaml", buildCorednsConfigMap(config)); err != nil {
		return fmt.Errorf("write coredns configmap: %w", err)
	}

	// Per-vcluster network policies (NFS, extra egress)
	netPolicies := buildNetworkPolicies(config)
	if len(netPolicies) > 0 {
		if err := ku.WriteYAMLDocuments(sdk, "resources/network-policies.yaml", netPolicies); err != nil {
			return fmt.Errorf("write network policies: %w", err)
		}
	}

	directResources := 2 // namespace + coredns configmap
	if etcdEnabled(config) {
		directResources++
	}
	if len(netPolicies) > 0 {
		directResources++
	}

	if err := ku.WritePromiseStatus(sdk, ku.PhaseScheduled, "VCluster resources scheduled for creation",
		map[string]interface{}{
			"resourceRequestsGenerated": len(resourceRequests),
			"directResourcesGenerated":  directResources,
			"vclusterName":              config.Name,
			"targetNamespace":           config.TargetNamespace,
			"hostname":                  config.Hostname,
			"environment":               config.ArgoCDEnvironment,
			// Platform Status Contract — endpoint and credential references
			"endpoints": map[string]string{
				"api":    config.ExternalServerURL,
				"argocd": fmt.Sprintf("https://argocd.cluster.integratn.tech/applications/vcluster-%s", config.Name),
			},
			"credentials": map[string]string{
				"kubeconfigSecret": fmt.Sprintf("vcluster-%s-kubeconfig", config.Name),
				"onePasswordItem":  config.OnePasswordItem,
			},
		}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// newKubeClientWithTimeout creates a Kubernetes client from the config's
// KubeClientFactory and a context with the given timeout. Callers must
// defer cancel().
func newKubeClientWithTimeout(config *VClusterConfig, timeout time.Duration) (kubernetes.Interface, context.Context, context.CancelFunc, error) {
	clientset, err := config.KubeClient.NewClient()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return clientset, ctx, cancel, nil
}

// vclusterRBACName returns the RBAC resource name that the vcluster Helm chart
// creates for a given vcluster. The convention is "vc-<name>-v-<namespace>"
// and is dictated by the upstream Helm chart.
func vclusterRBACName(name, namespace string) string {
	return fmt.Sprintf("vc-%s-v-%s", name, namespace)
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

	clientset, ctx, cancel, err := newKubeClientWithTimeout(config, 2*time.Minute)
	if err != nil {
		return err
	}
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
			log.Printf("  failed to delete PV %s: %v", pv.Name, err)
		} else {
			log.Printf("  deleted PV: %s", pv.Name)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.Printf("successfully cleaned up %d host PV(s)", len(pvList.Items))
	return nil
}

// cleanupNamespace deletes the vcluster target namespace, which cascade-deletes
// all namespace-scoped resources (PVCs, pods, services, etc.).
// This ensures no orphaned resources remain after vcluster deletion.
func cleanupNamespace(config *VClusterConfig) error {
	log.Printf("Cleaning up namespace: %s", config.TargetNamespace)

	clientset, ctx, cancel, err := newKubeClientWithTimeout(config, 2*time.Minute)
	if err != nil {
		return err
	}
	defer cancel()

	// Check if namespace exists before trying to delete
	_, err = clientset.CoreV1().Namespaces().Get(ctx, config.TargetNamespace, metav1.GetOptions{})
	if err != nil {
		// Only skip on not-found; propagate other errors
		if apierrors.IsNotFound(err) {
			log.Printf("Namespace %s not found or already deleted, skipping", config.TargetNamespace)
			return nil
		}
		return fmt.Errorf("failed to get namespace %s: %w", config.TargetNamespace, err)
	}

	if err := clientset.CoreV1().Namespaces().Delete(ctx, config.TargetNamespace, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", config.TargetNamespace, err)
	}

	log.Printf("namespace %s scheduled for deletion", config.TargetNamespace)
	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Printf("--- Handling delete for vcluster: %s ---", config.Name)

	if err := ku.WritePromiseStatus(sdk, ku.PhaseDeleting, "VCluster resources scheduled for deletion",
		map[string]interface{}{"vclusterName": config.Name}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	cleanupErr := directCleanup(config)

	if err := stateStoreCleanup(sdk, config); err != nil {
		return err
	}

	if cleanupErr != nil {
		return fmt.Errorf("cleanup errors: %w", cleanupErr)
	}

	return nil
}

// directCleanup performs direct Kubernetes API cleanup for resources NOT in the
// Kratix state store (host PVs, namespace). Returns a combined error or nil.
func directCleanup(config *VClusterConfig) error {
	var errs []error

	if err := cleanupHostPVs(config); err != nil {
		log.Printf("warning: PV cleanup encountered errors: %v", err)
		errs = append(errs, fmt.Errorf("PV cleanup: %w", err))
	}

	if err := cleanupNamespace(config); err != nil {
		log.Printf("warning: namespace cleanup encountered errors: %v", err)
		errs = append(errs, fmt.Errorf("namespace cleanup: %w", err))
	}

	return errors.Join(errs...)
}

// stateStoreCleanup emits Kratix delete-output manifests that cause ArgoCD
// to remove the managed resources from the cluster.
func stateStoreCleanup(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	outputs := map[string]ku.Resource{}

	allResources := []ku.Resource{
		buildArgoCDProjectRequest(config),
		buildArgoCDApplicationRequest(config),
		buildArgoCDClusterRegistrationRequest(config),
		buildCorednsConfigMap(config),
	}

	for _, obj := range allResources {
		deleteObj := ku.DeleteFromResource(obj)
		path := ku.DeleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	for _, obj := range buildNetworkPolicies(config) {
		deleteObj := ku.DeleteFromResource(obj)
		path := ku.DeleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	if etcdEnabled(config) {
		for _, obj := range buildEtcdCertificates(config) {
			deleteObj := ku.DeleteFromResource(obj)
			path := ku.DeleteOutputPathForResource("resources", obj)
			outputs[path] = deleteObj
		}
	}

	crName := vclusterRBACName(config.Name, config.TargetNamespace)
	outputs["resources/delete-vcluster-clusterrole.yaml"] = ku.DeleteFromResource(ku.Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata:   ku.ObjectMeta{Name: crName},
	})
	outputs["resources/delete-vcluster-clusterrolebinding.yaml"] = ku.DeleteFromResource(ku.Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata:   ku.ObjectMeta{Name: crName},
	})

	if etcdEnabled(config) {
		for _, suffix := range []string{"ca", "server", "peer"} {
			outputs[fmt.Sprintf("resources/delete-etcd-%s-secret.yaml", suffix)] = ku.DeleteFromResource(ku.Resource{
				APIVersion: "v1",
				Kind:       "Secret",
				Metadata:   ku.ObjectMeta{Name: fmt.Sprintf("%s-etcd-%s", config.Name, suffix), Namespace: config.TargetNamespace},
			})
		}
		outputs["resources/delete-etcd-merged-secret.yaml"] = ku.DeleteFromResource(ku.Resource{
			APIVersion: "v1",
			Kind:       "Secret",
			Metadata:   ku.ObjectMeta{Name: fmt.Sprintf("%s-etcd-certs", config.Name), Namespace: config.TargetNamespace},
		})
	}

	if err := ku.WriteOrderedResources(sdk, outputs); err != nil {
		return fmt.Errorf("write delete outputs: %w", err)
	}

	return nil
}
