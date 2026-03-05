package main

import (
	"fmt"
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestVclusterRBACName(t *testing.T) {
	tests := []struct {
		name      string
		vcName    string
		namespace string
		want      string
	}{
		{
			name:      "simple",
			vcName:    "media",
			namespace: "vcluster-media",
			want:      "vc-media-v-vcluster-media",
		},
		{
			name:      "with hyphens",
			vcName:    "my-cluster",
			namespace: "vcluster-my-cluster",
			want:      "vc-my-cluster-v-vcluster-my-cluster",
		},
		{
			name:      "short names",
			vcName:    "a",
			namespace: "b",
			want:      "vc-a-v-b",
		},
		{
			name:      "test-vc from minimalConfig",
			vcName:    "test-vc",
			namespace: "vcluster-test-vc",
			want:      "vc-test-vc-v-vcluster-test-vc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vclusterRBACName(tt.vcName, tt.namespace)
			if got != tt.want {
				t.Errorf("vclusterRBACName(%q, %q) = %q, want %q", tt.vcName, tt.namespace, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleConfigure — structured output assertions
// ---------------------------------------------------------------------------

func TestHandleConfigure_OutputResourceContent(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleConfigure(sdk, config); err != nil {
		t.Fatalf("handleConfigure: %v", err)
	}

	// Verify ArgoCD project request content
	projResources := ku.ReadOutputAsResources(t, dir, "resources/argocd-project-request.yaml")
	proj := ku.FindResource(projResources, "ArgoCDProject", config.ProjectName)
	if proj == nil {
		t.Fatalf("expected ArgoCDProject %q in output", config.ProjectName)
	}
	if proj.Metadata.Namespace != config.Namespace {
		t.Errorf("ArgoCDProject namespace: got %q, want %q", proj.Metadata.Namespace, config.Namespace)
	}

	// Verify ArgoCD application request content
	appResources := ku.ReadOutputAsResources(t, dir, "resources/argocd-application-request.yaml")
	expectedAppName := fmt.Sprintf("vcluster-%s", config.Name)
	app := ku.FindResource(appResources, "ArgoCDApplication", expectedAppName)
	if app == nil {
		t.Fatalf("expected ArgoCDApplication %q in output", expectedAppName)
	}

	// Verify cluster registration request
	regResources := ku.ReadOutputAsResources(t, dir, "resources/argocd-cluster-registration-request.yaml")
	expectedRegName := fmt.Sprintf("%s-cluster-registration", config.Name)
	reg := ku.FindResource(regResources, "ArgoCDClusterRegistration", expectedRegName)
	if reg == nil {
		t.Fatalf("expected ArgoCDClusterRegistration %q in output", expectedRegName)
	}

	// Verify namespace output
	nsResources := ku.ReadOutputAsResources(t, dir, "resources/namespace.yaml")
	ns := ku.FindResource(nsResources, "Namespace", config.TargetNamespace)
	if ns == nil {
		t.Fatalf("expected Namespace %q in output", config.TargetNamespace)
	}

	// Verify coredns configmap
	cmResources := ku.ReadOutputAsResources(t, dir, "resources/coredns-configmap.yaml")
	expectedCMName := fmt.Sprintf("vc-%s-coredns", config.Name)
	cm := ku.FindResource(cmResources, "ConfigMap", expectedCMName)
	if cm == nil {
		t.Fatalf("expected ConfigMap %q in output", expectedCMName)
	}
}

func TestHandleConfigure_NetworkPolicyContent(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	config.EnableNFS = true
	config.ExtraEgress = []ExtraEgressRule{
		{Name: "postgres", CIDR: "10.0.1.50/32", Port: 5432, Protocol: "TCP"},
	}
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleConfigure(sdk, config); err != nil {
		t.Fatalf("handleConfigure: %v", err)
	}

	// Network policies file should exist and contain all expected policies
	npContent := ku.ReadOutput(t, dir, "resources/network-policies.yaml")
	if !strings.Contains(npContent, "allow-dns") {
		t.Error("expected allow-dns network policy in output")
	}
	if !strings.Contains(npContent, "allow-nfs-egress") {
		t.Error("expected allow-nfs-egress network policy when NFS enabled")
	}
	if !strings.Contains(npContent, "allow-postgres-egress") {
		t.Error("expected allow-postgres-egress network policy for extra egress rule")
	}
}

func TestHandleConfigure_StatusOutput(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleConfigure(sdk, config); err != nil {
		t.Fatalf("handleConfigure: %v", err)
	}

	// Status file is written to status.yaml in the metadata dir (same as output dir in tests)
	statusContent := ku.ReadOutput(t, dir, "status.yaml")
	if !strings.Contains(statusContent, "Scheduled") {
		t.Error("expected status phase 'Scheduled'")
	}
	if !strings.Contains(statusContent, config.Name) {
		t.Errorf("expected vcluster name %q in status", config.Name)
	}
	if !strings.Contains(statusContent, config.TargetNamespace) {
		t.Errorf("expected target namespace %q in status", config.TargetNamespace)
	}
	if !strings.Contains(statusContent, config.ExternalServerURL) {
		t.Errorf("expected external server URL %q in status", config.ExternalServerURL)
	}
}

func TestHandleConfigure_EtcdCertsContent(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{"enabled": true},
		},
	}
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleConfigure(sdk, config); err != nil {
		t.Fatalf("handleConfigure: %v", err)
	}

	// Verify etcd certificates file contains expected resources
	etcdContent := ku.ReadOutput(t, dir, "resources/etcd-certificates.yaml")
	if !strings.Contains(etcdContent, "Certificate") {
		t.Error("expected Certificate kinds in etcd-certificates.yaml")
	}
	if !strings.Contains(etcdContent, "Issuer") {
		t.Error("expected Issuer kinds in etcd-certificates.yaml")
	}
	if !strings.Contains(etcdContent, "Job") {
		t.Error("expected Job kind in etcd-certificates.yaml")
	}
	if !strings.Contains(etcdContent, fmt.Sprintf("%s-etcd-ca", config.Name)) {
		t.Error("expected etcd CA certificate name in output")
	}
}

func TestHandleConfigure_NoNetPoliciesWhenEmpty(t *testing.T) {
	// This tests that network policies ARE always generated (baseline exists).
	// Even with no NFS/extra egress, baseline policies are still written.
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleConfigure(sdk, config); err != nil {
		t.Fatalf("handleConfigure: %v", err)
	}

	if !ku.FileExists(dir, "resources/network-policies.yaml") {
		t.Error("expected network-policies.yaml even with baseline-only policies")
	}
}

// ---------------------------------------------------------------------------
// handleDelete — structured output and state store cleanup assertions
// ---------------------------------------------------------------------------

func TestHandleDelete_StateStoreCleanupContent(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()

	fakeClient := fakeclientset.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: config.TargetNamespace},
		},
	)
	config.KubeClient = MockKubeClientFactory{Clientset: fakeClient}

	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleDelete(sdk, config); err != nil {
		t.Fatalf("handleDelete: %v", err)
	}

	// Verify delete outputs for each resource request type
	deleteFiles := map[string]string{
		"resources/delete-argocdproject-" + config.ProjectName + ".yaml":                                     "ArgoCDProject",
		"resources/delete-argocdapplication-vcluster-" + config.Name + ".yaml":                               "ArgoCDApplication",
		"resources/delete-argocdclusterregistration-" + config.Name + "-cluster-registration.yaml":           "ArgoCDClusterRegistration",
		"resources/delete-configmap-vc-" + config.Name + "-coredns.yaml":                                     "ConfigMap",
		"resources/delete-vcluster-clusterrole.yaml":                                                         "ClusterRole",
		"resources/delete-vcluster-clusterrolebinding.yaml":                                                  "ClusterRoleBinding",
	}
	for path, expectedKind := range deleteFiles {
		if !ku.FileExists(dir, path) {
			t.Errorf("expected delete output file: %s", path)
			continue
		}
		resources := ku.ReadOutputAsResources(t, dir, path)
		if len(resources) == 0 {
			t.Errorf("empty delete output: %s", path)
			continue
		}
		if resources[0].Kind != expectedKind {
			t.Errorf("delete output %s: got kind %q, want %q", path, resources[0].Kind, expectedKind)
		}
	}

	// Verify RBAC delete resources have correct names
	crResources := ku.ReadOutputAsResources(t, dir, "resources/delete-vcluster-clusterrole.yaml")
	expectedRBACName := vclusterRBACName(config.Name, config.TargetNamespace)
	if crResources[0].Metadata.Name != expectedRBACName {
		t.Errorf("ClusterRole delete name: got %q, want %q", crResources[0].Metadata.Name, expectedRBACName)
	}
}

func TestHandleDelete_NetworkPolicyCleanup(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	config.EnableNFS = true

	fakeClient := fakeclientset.NewSimpleClientset()
	config.KubeClient = MockKubeClientFactory{Clientset: fakeClient}

	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleDelete(sdk, config); err != nil {
		t.Fatalf("handleDelete: %v", err)
	}

	// Network policy delete outputs should exist for all baseline + NFS policies
	expectedPolicies := buildNetworkPolicies(config)
	for _, p := range expectedPolicies {
		deletePath := ku.DeleteOutputPathForResource("resources", p)
		if !ku.FileExists(dir, deletePath) {
			t.Errorf("expected network policy delete file: %s (for policy %q)", deletePath, p.Metadata.Name)
		}
	}
}

func TestHandleDelete_EtcdStateStoreCleanup(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{"enabled": true},
		},
	}

	fakeClient := fakeclientset.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: config.TargetNamespace},
		},
	)
	config.KubeClient = MockKubeClientFactory{Clientset: fakeClient}

	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleDelete(sdk, config); err != nil {
		t.Fatalf("handleDelete: %v", err)
	}

	// Verify etcd certificate delete outputs
	etcdCerts := buildEtcdCertificates(config)
	for _, cert := range etcdCerts {
		deletePath := ku.DeleteOutputPathForResource("resources", cert)
		if !ku.FileExists(dir, deletePath) {
			t.Errorf("expected etcd cert delete file: %s (%s %q)", deletePath, cert.Kind, cert.Metadata.Name)
		}
	}

	// Verify etcd secret delete outputs
	etcdSecretFiles := []string{
		"resources/delete-etcd-ca-secret.yaml",
		"resources/delete-etcd-server-secret.yaml",
		"resources/delete-etcd-peer-secret.yaml",
		"resources/delete-etcd-merged-secret.yaml",
	}
	for _, f := range etcdSecretFiles {
		if !ku.FileExists(dir, f) {
			t.Errorf("expected etcd secret delete file: %s", f)
		}
	}
}

func TestHandleDelete_StatusIsDeleting(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := minimalConfig()

	fakeClient := fakeclientset.NewSimpleClientset()
	config.KubeClient = MockKubeClientFactory{Clientset: fakeClient}

	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleDelete(sdk, config); err != nil {
		t.Fatalf("handleDelete: %v", err)
	}

	statusContent := ku.ReadOutput(t, dir, "status.yaml")
	if !strings.Contains(statusContent, "Deleting") {
		t.Error("expected status phase 'Deleting' on delete")
	}
	if !strings.Contains(statusContent, config.Name) {
		t.Errorf("expected vcluster name %q in delete status", config.Name)
	}
}

func TestHandleDelete_MultiplePVsCleanedUp(t *testing.T) {
	sdk, _ := ku.NewTestSDK(t)
	config := minimalConfig()

	labelValue := fmt.Sprintf("%s-x-%s", config.Name, config.TargetNamespace)
	fakeClient := fakeclientset.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pv-synced-1",
				Labels: map[string]string{"vcluster.loft.sh/managed-by": labelValue},
			},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pv-synced-2",
				Labels: map[string]string{"vcluster.loft.sh/managed-by": labelValue},
			},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pv-other",
				Labels: map[string]string{"vcluster.loft.sh/managed-by": "other-cluster"},
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: config.TargetNamespace},
		},
	)
	config.KubeClient = MockKubeClientFactory{Clientset: fakeClient}

	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject: %v", err)
	}
	config.ValuesObject = vals

	if err := handleDelete(sdk, config); err != nil {
		t.Fatalf("handleDelete: %v", err)
	}

	// Verify only matching PVs were deleted (the "other" PV should remain)
	ctx := t
	_ = ctx // PVs are deleted via the fake client; verify by listing remaining PVs
	pvList, err := fakeClient.CoreV1().PersistentVolumes().List(
		t.Context(), metav1.ListOptions{},
	)
	if err != nil {
		t.Fatalf("listing PVs: %v", err)
	}
	if len(pvList.Items) != 1 {
		t.Errorf("expected 1 remaining PV (pv-other), got %d", len(pvList.Items))
	}
	if len(pvList.Items) > 0 && pvList.Items[0].Name != "pv-other" {
		t.Errorf("expected remaining PV 'pv-other', got %q", pvList.Items[0].Name)
	}
}
