package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
	kratix "github.com/syntasso/kratix-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ============================================================================
// Mock Resource
// ============================================================================

type mockResource struct {
	data map[string]interface{}
	name string
	ns   string
}

var _ kratix.Resource = (*mockResource)(nil)

func (m *mockResource) GetValue(path string) (interface{}, error) {
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current interface{} = m.data
	for _, key := range keys {
		if cm, ok := current.(map[string]interface{}); ok {
			val, found := cm[key]
			if !found {
				return nil, fmt.Errorf("path %s not found", path)
			}
			current = val
		} else {
			return nil, fmt.Errorf("path %s not found", path)
		}
	}
	return current, nil
}

func (m *mockResource) GetStatus() (kratix.Status, error) { return nil, nil }
func (m *mockResource) GetName() string                    { return m.name }
func (m *mockResource) GetNamespace() string               { return m.ns }
func (m *mockResource) GetGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{}
}
func (m *mockResource) GetLabels() map[string]string      { return nil }
func (m *mockResource) GetAnnotations() map[string]string { return nil }
func (m *mockResource) ToUnstructured() unstructured.Unstructured {
	return unstructured.Unstructured{}
}

// mockSDK wraps a real SDK for testing, providing helpers.
type testSDKHelper struct {
	sdk *kratix.KratixSDK
	dir string
}

func newTestSDK(t *testing.T) *testSDKHelper {
	t.Helper()
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir), kratix.WithMetadataDir(dir))
	return &testSDKHelper{sdk: sdk, dir: dir}
}

func readOutput(t *testing.T, dir, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func fileExists(dir, path string) bool {
	_, err := os.Stat(filepath.Join(dir, path))
	return err == nil
}

// minimalConfig returns a VClusterConfig with all required fields for testing.
func minimalConfig() *VClusterConfig {
	return &VClusterConfig{
		Name:            "test-vc",
		Namespace:       "platform-requests",
		TargetNamespace: "vcluster-test-vc",
		ProjectName:     "vcluster-test-vc",
		K8sVersion:      "v1.34.3",
		Preset:          "dev",
		Replicas:        1,
		CPURequest:      "200m",
		MemoryRequest:   "768Mi",
		CPULimit:        "1000m",
		MemoryLimit:     "1536Mi",
		CorednsReplicas: 1,
		ClusterDomain:   "cluster.local",
		Hostname:        "test-vc.integratn.tech",
		APIPort:         443,
		ExternalServerURL: "https://test-vc.integratn.tech:443",
		CertManagerIssuerLabels:    map[string]string{"integratn.tech/cluster-issuer": "letsencrypt-prod"},
		ExternalSecretsStoreLabels: map[string]string{"integratn.tech/cluster-secret-store": "onepassword-store"},
		ArgoCDEnvironment:          "development",
		ArgoCDClusterLabels:        map[string]string{"cluster_name": "test-vc"},
		ArgoCDClusterAnnotations:   map[string]string{"cluster_name": "test-vc"},
		ArgoCDRepoURL:              "https://charts.loft.sh",
		ArgoCDChart:                "vcluster",
		ArgoCDTargetRevision:       "0.30.4",
		ArgoCDDestServer:           "https://kubernetes.default.svc",
		ArgoCDSyncPolicy: map[string]interface{}{
			"automated": map[string]interface{}{"selfHeal": true, "prune": true},
		},
		OnePasswordItem:       "vcluster-test-vc-kubeconfig",
		KubeconfigSyncJobName: "vcluster-test-vc-kubeconfig-sync",
		BaseDomain:            "integratn.tech",
		BaseDomainSanitized:   "integratn-tech",
		WorkloadRepoURL:       "https://github.com/jamesatintegratnio/gitops_homelab_2_0",
		WorkloadRepoPath:      "workloads",
		WorkloadRepoRevision:  "main",
		WorkflowContext: WorkflowContext{
			PromiseName: "vcluster-orchestrator-v2",
		},
	}
}

// ============================================================================
// IP Utility Functions
// ============================================================================

func TestDefaultVIPFromCIDR(t *testing.T) {
	vip, err := defaultVIPFromCIDR("10.0.4.0/24", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vip != "10.0.4.200" {
		t.Errorf("expected 10.0.4.200, got %s", vip)
	}
}

func TestDefaultVIPFromCIDR_InvalidCIDR(t *testing.T) {
	_, err := defaultVIPFromCIDR("not-a-cidr", 200)
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

func TestIpInCIDR_True(t *testing.T) {
	if !ipInCIDR("10.0.4.200", "10.0.4.0/24") {
		t.Error("expected 10.0.4.200 in 10.0.4.0/24")
	}
}

func TestIpInCIDR_False(t *testing.T) {
	if ipInCIDR("10.0.5.200", "10.0.4.0/24") {
		t.Error("expected 10.0.5.200 not in 10.0.4.0/24")
	}
}

func TestIpInCIDR_InvalidIP(t *testing.T) {
	if ipInCIDR("not-an-ip", "10.0.4.0/24") {
		t.Error("expected false for invalid IP")
	}
}

func TestIpToIntAndBack(t *testing.T) {
	// Direct test: intToIP + ipToInt roundtrip
	ip := intToIP(167773384)
	if ip.String() != "10.0.4.200" {
		t.Errorf("expected 10.0.4.200, got %s", ip.String())
	}
	back := ipToInt(ip)
	if back != 167773384 {
		t.Errorf("expected 167773384, got %d", back)
	}
}

// ============================================================================
// etcdEnabled
// ============================================================================

func TestEtcdEnabled_Nil(t *testing.T) {
	config := &VClusterConfig{}
	if etcdEnabled(config) {
		t.Error("expected false when BackingStore is nil")
	}
}

func TestEtcdEnabled_NoEtcd(t *testing.T) {
	config := &VClusterConfig{
		BackingStore: map[string]interface{}{"something": "else"},
	}
	if etcdEnabled(config) {
		t.Error("expected false when no etcd key")
	}
}

func TestEtcdEnabled_True(t *testing.T) {
	config := &VClusterConfig{
		BackingStore: map[string]interface{}{
			"etcd": map[string]interface{}{
				"deploy": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}
	if !etcdEnabled(config) {
		t.Error("expected true when etcd.deploy.enabled is true")
	}
}

func TestEtcdEnabled_False(t *testing.T) {
	config := &VClusterConfig{
		BackingStore: map[string]interface{}{
			"etcd": map[string]interface{}{
				"deploy": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}
	if etcdEnabled(config) {
		t.Error("expected false when etcd.deploy.enabled is false")
	}
}

// ============================================================================
// applyPresetDefaults
// ============================================================================

func TestApplyPresetDefaults_Dev(t *testing.T) {
	config := &VClusterConfig{Preset: "dev"}
	resource := &mockResource{data: map[string]interface{}{"spec": map[string]interface{}{}}}
	applyPresetDefaults(config, resource)

	if config.Replicas != 1 {
		t.Errorf("expected dev replicas 1, got %d", config.Replicas)
	}
	if config.CPURequest != "200m" {
		t.Errorf("expected dev CPURequest '200m', got %q", config.CPURequest)
	}
	if config.PersistenceEnabled {
		t.Error("expected dev persistence disabled")
	}
	if config.CorednsReplicas != 1 {
		t.Errorf("expected dev coredns replicas 1, got %d", config.CorednsReplicas)
	}
}

func TestApplyPresetDefaults_Prod(t *testing.T) {
	config := &VClusterConfig{Preset: "prod"}
	resource := &mockResource{data: map[string]interface{}{"spec": map[string]interface{}{}}}
	applyPresetDefaults(config, resource)

	if config.Replicas != 3 {
		t.Errorf("expected prod replicas 3, got %d", config.Replicas)
	}
	if config.CPURequest != "500m" {
		t.Errorf("expected prod CPURequest '500m', got %q", config.CPURequest)
	}
	if !config.PersistenceEnabled {
		t.Error("expected prod persistence enabled")
	}
	if config.PersistenceSize != "10Gi" {
		t.Errorf("expected prod persistence size '10Gi', got %q", config.PersistenceSize)
	}
	if config.CorednsReplicas != 2 {
		t.Errorf("expected prod coredns replicas 2, got %d", config.CorednsReplicas)
	}
}

func TestApplyPresetDefaults_Override(t *testing.T) {
	config := &VClusterConfig{Preset: "dev"}
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"vcluster": map[string]interface{}{
					"replicas": 5,
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu": "999m",
						},
					},
					"persistence": map[string]interface{}{
						"enabled": true,
						"size":    "50Gi",
					},
				},
			},
		},
	}
	applyPresetDefaults(config, resource)

	if config.Replicas != 5 {
		t.Errorf("expected overridden replicas 5, got %d", config.Replicas)
	}
	if config.CPURequest != "999m" {
		t.Errorf("expected overridden CPURequest '999m', got %q", config.CPURequest)
	}
	if !config.PersistenceEnabled {
		t.Error("expected overridden persistence enabled")
	}
	if config.PersistenceSize != "50Gi" {
		t.Errorf("expected overridden persistence size '50Gi', got %q", config.PersistenceSize)
	}
}

func TestApplyPresetDefaults_UnknownPreset(t *testing.T) {
	config := &VClusterConfig{Preset: "unknown"}
	resource := &mockResource{data: map[string]interface{}{"spec": map[string]interface{}{}}}
	applyPresetDefaults(config, resource)

	// Falls back to dev defaults
	if config.Replicas != 1 {
		t.Errorf("expected fallback to dev replicas 1, got %d", config.Replicas)
	}
}

// ============================================================================
// extractExtraEgress
// ============================================================================

func TestExtractExtraEgress_Valid(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"networkPolicies": map[string]interface{}{
					"extraEgress": []interface{}{
						map[string]interface{}{
							"name":     "postgres",
							"cidr":     "10.0.1.50/32",
							"port":     float64(5432),
							"protocol": "TCP",
						},
					},
				},
			},
		},
	}

	rules := extractExtraEgress(resource)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "postgres" {
		t.Errorf("expected name 'postgres', got %q", rules[0].Name)
	}
	if rules[0].CIDR != "10.0.1.50/32" {
		t.Errorf("expected cidr '10.0.1.50/32', got %q", rules[0].CIDR)
	}
	if rules[0].Port != 5432 {
		t.Errorf("expected port 5432, got %d", rules[0].Port)
	}
}

func TestExtractExtraEgress_DefaultProtocol(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"networkPolicies": map[string]interface{}{
					"extraEgress": []interface{}{
						map[string]interface{}{
							"name": "redis",
							"cidr": "10.0.1.60/32",
							"port": float64(6379),
						},
					},
				},
			},
		},
	}

	rules := extractExtraEgress(resource)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Protocol != "TCP" {
		t.Errorf("expected default protocol TCP, got %q", rules[0].Protocol)
	}
}

func TestExtractExtraEgress_IncompleteSkipped(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"networkPolicies": map[string]interface{}{
					"extraEgress": []interface{}{
						map[string]interface{}{
							"name": "incomplete",
							// Missing cidr and port
						},
					},
				},
			},
		},
	}

	rules := extractExtraEgress(resource)
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for incomplete entry, got %d", len(rules))
	}
}

func TestExtractExtraEgress_NoField(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{"spec": map[string]interface{}{}},
	}
	rules := extractExtraEgress(resource)
	if rules != nil {
		t.Errorf("expected nil, got %v", rules)
	}
}

// ============================================================================
// Builder: ArgoCD Project Request
// ============================================================================

func TestBuildArgoCDProjectRequest(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDProjectRequest(config)

	if res.Kind != "ArgoCDProject" {
		t.Errorf("expected kind ArgoCDProject, got %q", res.Kind)
	}
	if res.Metadata.Name != config.ProjectName {
		t.Errorf("expected name %q, got %q", config.ProjectName, res.Metadata.Name)
	}
	if res.Metadata.Namespace != config.Namespace {
		t.Errorf("expected namespace %q, got %q", config.Namespace, res.Metadata.Namespace)
	}

	spec, ok := res.Spec.(u.ArgoCDProjectSpec)
	if !ok {
		t.Fatal("expected ArgoCDProjectSpec type")
	}
	if spec.Name != config.ProjectName {
		t.Errorf("expected project name %q, got %q", config.ProjectName, spec.Name)
	}
}

// ============================================================================
// Builder: ArgoCD Application Request
// ============================================================================

func TestBuildArgoCDApplicationRequest(t *testing.T) {
	config := minimalConfig()
	config.ValuesObject = map[string]interface{}{"key": "value"}
	res := buildArgoCDApplicationRequest(config)

	if res.Kind != "ArgoCDApplication" {
		t.Errorf("expected kind ArgoCDApplication, got %q", res.Kind)
	}
	expectedName := fmt.Sprintf("vcluster-%s", config.Name)
	if res.Metadata.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, res.Metadata.Name)
	}

	spec, ok := res.Spec.(u.ArgoCDApplicationSpec)
	if !ok {
		t.Fatal("expected ArgoCDApplicationSpec type")
	}
	if spec.Project != config.ProjectName {
		t.Errorf("expected project %q, got %q", config.ProjectName, spec.Project)
	}
	if spec.Source.Chart != "vcluster" {
		t.Errorf("expected chart 'vcluster', got %q", spec.Source.Chart)
	}
}

// ============================================================================
// Builder: ArgoCD Cluster Registration Request
// ============================================================================

func TestBuildArgoCDClusterRegistrationRequest(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDClusterRegistrationRequest(config)

	if res.Kind != "ArgoCDClusterRegistration" {
		t.Errorf("expected kind ArgoCDClusterRegistration, got %q", res.Kind)
	}
	expectedName := fmt.Sprintf("%s-cluster-registration", config.Name)
	if res.Metadata.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, res.Metadata.Name)
	}

	spec, ok := res.Spec.(u.ArgoCDClusterRegistrationSpec)
	if !ok {
		t.Fatal("expected ArgoCDClusterRegistrationSpec type")
	}
	if spec.Name != config.Name {
		t.Errorf("expected spec name %q, got %q", config.Name, spec.Name)
	}
	if spec.ExternalServerURL != config.ExternalServerURL {
		t.Errorf("expected server URL %q, got %q", config.ExternalServerURL, spec.ExternalServerURL)
	}
}

// ============================================================================
// Builder: Namespace
// ============================================================================

func TestBuildNamespace(t *testing.T) {
	config := minimalConfig()
	ns := buildNamespace(config)

	if ns.Kind != "Namespace" {
		t.Errorf("expected kind Namespace, got %q", ns.Kind)
	}
	if ns.Metadata.Name != config.TargetNamespace {
		t.Errorf("expected name %q, got %q", config.TargetNamespace, ns.Metadata.Name)
	}
	if ns.Metadata.Labels["vcluster.loft.sh/namespace"] != "true" {
		t.Error("expected vcluster namespace label")
	}
}

// ============================================================================
// Builder: CoreDNS ConfigMap
// ============================================================================

func TestBuildCorednsConfigMap(t *testing.T) {
	config := minimalConfig()
	cm := buildCorednsConfigMap(config)

	if cm.Kind != "ConfigMap" {
		t.Errorf("expected kind ConfigMap, got %q", cm.Kind)
	}
	expectedName := fmt.Sprintf("vc-%s-coredns", config.Name)
	if cm.Metadata.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, cm.Metadata.Name)
	}
	dataMap, ok := cm.Data.(map[string]string)
	if !ok {
		t.Fatal("expected Data to be map[string]string")
	}
	corefile, ok := dataMap["Corefile"]
	if !ok || corefile == "" {
		t.Error("expected Corefile data")
	}
	if !strings.Contains(corefile, config.ClusterDomain) {
		t.Error("expected cluster domain in Corefile")
	}
}

// ============================================================================
// Builder: etcd Certificates
// ============================================================================

func TestBuildEtcdCertificates_NotEnabled(t *testing.T) {
	config := minimalConfig()
	certs := buildEtcdCertificates(config)
	if certs != nil {
		t.Error("expected nil when etcd not enabled")
	}
}

func TestBuildEtcdCertificates_Enabled(t *testing.T) {
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	certs := buildEtcdCertificates(config)
	if len(certs) != 9 {
		t.Fatalf("expected 9 resources (SA, Role, RoleBinding, CA cert, SelfSigned issuer, CA issuer, merge job, server cert, peer cert), got %d", len(certs))
	}

	// Check types
	kinds := map[string]int{}
	for _, c := range certs {
		kinds[c.Kind]++
	}
	if kinds["ServiceAccount"] != 1 {
		t.Error("expected 1 ServiceAccount")
	}
	if kinds["Role"] != 1 {
		t.Error("expected 1 Role")
	}
	if kinds["RoleBinding"] != 1 {
		t.Error("expected 1 RoleBinding")
	}
	if kinds["Certificate"] != 3 {
		t.Errorf("expected 3 Certificates, got %d", kinds["Certificate"])
	}
	if kinds["Issuer"] != 2 {
		t.Errorf("expected 2 Issuers, got %d", kinds["Issuer"])
	}
	if kinds["Job"] != 1 {
		t.Error("expected 1 Job")
	}
}

func TestBuildEtcdDNSNames(t *testing.T) {
	config := minimalConfig()
	dns := buildEtcdDNSNames(config)

	if len(dns) == 0 {
		t.Fatal("expected DNS names")
	}
	// Should contain base DNS + 3 replicas * 4 variants + localhost
	expectedMin := 8 + 12 + 1 // 21
	if len(dns) < expectedMin {
		t.Errorf("expected at least %d DNS names, got %d", expectedMin, len(dns))
	}

	// Check some expected names
	found := map[string]bool{}
	for _, name := range dns {
		found[name] = true
	}
	if !found[fmt.Sprintf("%s-etcd", config.Name)] {
		t.Error("expected base etcd DNS name")
	}
	if !found["localhost"] {
		t.Error("expected localhost")
	}
}

// ============================================================================
// Builder: Network Policies
// ============================================================================

func TestBuildNetworkPolicies_Baseline(t *testing.T) {
	config := minimalConfig()
	policies := buildNetworkPolicies(config)

	// Baseline: default-deny, DNS, kube-api, coredns-host-dns, intra-namespace, external, lb-snat = 7
	if len(policies) != 7 {
		t.Fatalf("expected 7 baseline policies, got %d", len(policies))
	}

	names := map[string]bool{}
	for _, p := range policies {
		names[p.Metadata.Name] = true
	}
	expected := []string{
		"default-deny-all",
		"allow-dns",
		"allow-kube-api",
		"allow-coredns-to-host-dns",
		"allow-intra-namespace",
		"allow-vcluster-external",
		"allow-vcluster-lb-snat",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected policy %q", name)
		}
	}
}

func TestBuildNetworkPolicies_WithNFS(t *testing.T) {
	config := minimalConfig()
	config.EnableNFS = true
	policies := buildNetworkPolicies(config)

	if len(policies) != 8 {
		t.Fatalf("expected 8 policies (baseline + NFS), got %d", len(policies))
	}
}

func TestBuildNetworkPolicies_WithExtraEgress(t *testing.T) {
	config := minimalConfig()
	config.ExtraEgress = []ExtraEgressRule{
		{Name: "postgres", CIDR: "10.0.1.50/32", Port: 5432, Protocol: "TCP"},
		{Name: "redis", CIDR: "10.0.1.60/32", Port: 6379, Protocol: "TCP"},
	}
	policies := buildNetworkPolicies(config)

	if len(policies) != 9 {
		t.Fatalf("expected 9 policies (baseline + 2 extra), got %d", len(policies))
	}
}

func TestBuildExtraEgressPolicy(t *testing.T) {
	config := minimalConfig()
	rule := ExtraEgressRule{
		Name:     "postgres",
		CIDR:     "10.0.1.50/32",
		Port:     5432,
		Protocol: "TCP",
	}

	policy := buildExtraEgressPolicy(config, rule)
	if policy.Metadata.Name != "allow-postgres-egress" {
		t.Errorf("expected name 'allow-postgres-egress', got %q", policy.Metadata.Name)
	}
	if policy.Kind != "NetworkPolicy" {
		t.Errorf("expected kind NetworkPolicy, got %q", policy.Kind)
	}
}

// ============================================================================
// buildValuesObject
// ============================================================================

func TestBuildValuesObject_Minimal(t *testing.T) {
	config := minimalConfig()
	var err error
	config.ValuesObject, err = buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.ValuesObject == nil {
		t.Fatal("expected non-nil values object")
	}

	cp, ok := config.ValuesObject["controlPlane"].(map[string]interface{})
	if !ok {
		t.Fatal("expected controlPlane in values")
	}
	service, ok := cp["service"].(map[string]interface{})
	if !ok {
		t.Fatal("expected service in controlPlane")
	}
	if service["enabled"] != true {
		t.Error("expected service enabled")
	}
}

func TestBuildValuesObject_WithEtcdBackingStore(t *testing.T) {
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := values["controlPlane"].(map[string]interface{})
	if !ok {
		t.Fatal("expected controlPlane in values")
	}
	if cp["backingStore"] == nil {
		t.Error("expected backingStore in controlPlane")
	}
}

func TestBuildValuesObject_WithVIPAndProxy(t *testing.T) {
	config := minimalConfig()
	config.VIP = "10.0.4.200"
	config.ProxyExtraSANs = []string{"test-vc.integratn.tech", "10.0.4.200"}
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := values["controlPlane"].(map[string]interface{})
	if cp["proxy"] == nil {
		t.Error("expected proxy config with extra SANs")
	}
	service := cp["service"].(map[string]interface{})
	spec, ok := service["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec in service")
	}
	if spec["loadBalancerIP"] != "10.0.4.200" {
		t.Errorf("expected loadBalancerIP '10.0.4.200', got %v", spec["loadBalancerIP"])
	}
}

func TestBuildValuesObject_WithHelmOverrides(t *testing.T) {
	config := minimalConfig()
	config.HelmOverrides = map[string]interface{}{
		"telemetry": map[string]interface{}{
			"enabled": true,
		},
	}
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject failed: %v", err)
	}

	telemetry, ok := values["telemetry"].(map[string]interface{})
	if !ok {
		t.Fatal("expected telemetry in values")
	}
	// HelmOverrides should override the default (false → true)
	if telemetry["enabled"] != true {
		t.Error("expected telemetry enabled via HelmOverrides")
	}
}

// ============================================================================
// handleConfigure
// ============================================================================

func TestHandleConfigure_Basic(t *testing.T) {
	h := newTestSDK(t)
	config := minimalConfig()
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject failed: %v", err)
	}
	config.ValuesObject = vals

	err = handleConfigure(h.sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check ArgoCD requests
	if !fileExists(h.dir, "resources/argocd-project-request.yaml") {
		t.Error("expected argocd-project-request.yaml")
	}
	if !fileExists(h.dir, "resources/argocd-application-request.yaml") {
		t.Error("expected argocd-application-request.yaml")
	}
	if !fileExists(h.dir, "resources/argocd-cluster-registration-request.yaml") {
		t.Error("expected argocd-cluster-registration-request.yaml")
	}

	// Namespace
	if !fileExists(h.dir, "resources/namespace.yaml") {
		t.Error("expected namespace.yaml")
	}

	// CoreDNS
	if !fileExists(h.dir, "resources/coredns-configmap.yaml") {
		t.Error("expected coredns-configmap.yaml")
	}

	// No etcd certs (not enabled)
	if fileExists(h.dir, "resources/etcd-certificates.yaml") {
		t.Error("should not create etcd-certificates when not enabled")
	}

	// Network policies
	if !fileExists(h.dir, "resources/network-policies.yaml") {
		t.Error("expected network-policies.yaml")
	}
}

func TestHandleConfigure_WithEtcd(t *testing.T) {
	h := newTestSDK(t)
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	vals, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("buildValuesObject failed: %v", err)
	}
	config.ValuesObject = vals

	err = handleConfigure(h.sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fileExists(h.dir, "resources/etcd-certificates.yaml") {
		t.Error("expected etcd-certificates.yaml")
	}
}

// ============================================================================
// handleDelete (cannot fully test — uses in-cluster k8s client)
// We test that it doesn't panic with properly configured SDK.
// The cleanupHostPVs and cleanupNamespace will fail (no in-cluster config)
// but the function should still write delete output files.
// ============================================================================

// Note: handleDelete calls rest.InClusterConfig() which will fail in test,
// but the function is designed to log warnings and continue.
// We test the delete output generation indirectly through the builder functions.

func TestDeleteOutputGeneration(t *testing.T) {
	config := minimalConfig()

	// Test that we can build all the delete resources without error
	allResources := []u.Resource{
		buildArgoCDProjectRequest(config),
		buildArgoCDApplicationRequest(config),
		buildArgoCDClusterRegistrationRequest(config),
		buildCorednsConfigMap(config),
	}

	for _, obj := range allResources {
		deleteObj := u.DeleteFromResource(obj)
		path := u.DeleteOutputPathForResource("resources", obj)
		if deleteObj.Kind != obj.Kind {
			t.Errorf("expected kind %q in delete, got %q", obj.Kind, deleteObj.Kind)
		}
		if path == "" {
			t.Error("expected non-empty delete path")
		}
	}

	// Test network policy deletes
	netPolicies := buildNetworkPolicies(config)
	for _, obj := range netPolicies {
		deleteObj := u.DeleteFromResource(obj)
		if deleteObj.Kind != obj.Kind {
			t.Errorf("expected kind %q in delete, got %q", obj.Kind, deleteObj.Kind)
		}
	}
}

func TestDeleteOutputGeneration_WithEtcd(t *testing.T) {
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{"enabled": true},
		},
	}

	etcdCerts := buildEtcdCertificates(config)
	for _, obj := range etcdCerts {
		deleteObj := u.DeleteFromResource(obj)
		if deleteObj.Kind != obj.Kind {
			t.Errorf("expected kind %q in delete, got %q", obj.Kind, deleteObj.Kind)
		}
	}
}

// ============================================================================
// buildConfig (integration via mock resource)
// ============================================================================

func TestBuildConfig_MinimalValid(t *testing.T) {
	h := newTestSDK(t)
	resource := &mockResource{
		name: "test-vc",
		ns:   "default",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "test-vc",
			},
			"metadata": map[string]interface{}{},
		},
	}

	config, err := buildConfig(h.sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "test-vc" {
		t.Errorf("expected name 'test-vc', got %q", config.Name)
	}
	if config.TargetNamespace != "default" {
		t.Errorf("expected targetNamespace 'default', got %q", config.TargetNamespace)
	}
	if config.ProjectName != "vcluster-test-vc" {
		t.Errorf("expected projectName 'vcluster-test-vc', got %q", config.ProjectName)
	}
	if config.Preset != "dev" {
		t.Errorf("expected preset 'dev', got %q", config.Preset)
	}
	if config.Hostname != "test-vc.integratn.tech" {
		t.Errorf("expected hostname 'test-vc.integratn.tech', got %q", config.Hostname)
	}
	if config.ValuesObject == nil {
		t.Error("expected non-nil ValuesObject")
	}
}

func TestBuildConfig_MissingName(t *testing.T) {
	h := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}

	_, err := buildConfig(h.sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing spec.name")
	}
	if !strings.Contains(err.Error(), "spec.name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WithSubnet(t *testing.T) {
	h := newTestSDK(t)
	resource := &mockResource{
		name: "test",
		ns:   "default",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "test",
				"exposure": map[string]interface{}{
					"subnet": "10.0.4.0/24",
				},
			},
			"metadata": map[string]interface{}{},
		},
	}

	config, err := buildConfig(h.sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.VIP != "10.0.4.200" {
		t.Errorf("expected VIP '10.0.4.200', got %q", config.VIP)
	}
}

func TestBuildConfig_VIPNotInSubnet(t *testing.T) {
	h := newTestSDK(t)
	resource := &mockResource{
		name: "test",
		ns:   "default",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "test",
				"exposure": map[string]interface{}{
					"subnet": "10.0.4.0/24",
					"vip":    "10.0.5.200",
				},
			},
			"metadata": map[string]interface{}{},
		},
	}

	_, err := buildConfig(h.sdk, resource)
	if err == nil {
		t.Fatal("expected error for VIP not in subnet")
	}
	if !strings.Contains(err.Error(), "not within subnet") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WithProdPreset(t *testing.T) {
	h := newTestSDK(t)
	resource := &mockResource{
		name: "prod-vc",
		ns:   "default",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "prod-vc",
				"vcluster": map[string]interface{}{
					"preset": "prod",
				},
			},
			"metadata": map[string]interface{}{},
		},
	}

	config, err := buildConfig(h.sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Replicas != 3 {
		t.Errorf("expected prod replicas 3, got %d", config.Replicas)
	}
	if config.ArgoCDEnvironment != "production" {
		t.Errorf("expected production environment, got %q", config.ArgoCDEnvironment)
	}
}
