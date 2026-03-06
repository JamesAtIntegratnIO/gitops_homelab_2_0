package main

import (
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// newBuilderTestConfig creates a fully-populated RegistrationConfig for builder tests.
// This intentionally differs from newTestConfig in main_test.go to avoid coupling.
func newBuilderTestConfig() *RegistrationConfig {
	return &RegistrationConfig{
		Name:                   "builder-test",
		TargetNamespace:        "vcluster-builder",
		KubeconfigSecret:       "builder-kubeconfig",
		KubeconfigKey:          "admin.conf",
		ExternalServerURL:      "https://builder.example.com:6443",
		OnePasswordItem:        "builder-test-kubeconfig",
		OnePasswordConnectHost: "https://connect.example.com",
		Environment:            "staging",
		BaseDomain:             "example.com",
		BaseDomainSanitized:    "example-com",
		SyncJobName:            "builder-test-sync",
		PromiseName:            "argocd-cluster-registration",
	}
}

// ---------------------------------------------------------------------------
// buildKubeconfigExternalSecret – deep spec verification
// ---------------------------------------------------------------------------

func TestBuildKubeconfigExternalSecret_SecretStoreRef(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildKubeconfigExternalSecret(config)

	spec, ok := es.Spec.(ku.ExternalSecretSpec)
	if !ok {
		t.Fatal("Spec is not ExternalSecretSpec")
	}
	if spec.SecretStoreRef.Name != ku.DefaultSecretStoreName {
		t.Errorf("expected store name %q, got %q", ku.DefaultSecretStoreName, spec.SecretStoreRef.Name)
	}
	if spec.SecretStoreRef.Kind != ku.DefaultSecretStoreKind {
		t.Errorf("expected store kind %q, got %q", ku.DefaultSecretStoreKind, spec.SecretStoreRef.Kind)
	}
}

func TestBuildKubeconfigExternalSecret_TargetTemplate(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildKubeconfigExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if spec.Target.Name != "builder-test-kubeconfig-external" {
		t.Errorf("expected target name 'builder-test-kubeconfig-external', got %q", spec.Target.Name)
	}
	if spec.Target.Template == nil {
		t.Fatal("expected target template, got nil")
	}
	if spec.Target.Template.EngineVersion != "v2" {
		t.Errorf("expected engine 'v2', got %q", spec.Target.Template.EngineVersion)
	}
	tmplData, ok := spec.Target.Template.Data[config.KubeconfigKey]
	if !ok {
		t.Fatalf("template data missing key %q", config.KubeconfigKey)
	}
	if tmplData != "{{ .kubeconfig }}\n" {
		t.Errorf("unexpected template data: %q", tmplData)
	}
}

func TestBuildKubeconfigExternalSecret_DataFrom(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildKubeconfigExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if len(spec.DataFrom) != 1 {
		t.Fatalf("expected 1 dataFrom, got %d", len(spec.DataFrom))
	}
	if spec.DataFrom[0].Extract == nil {
		t.Fatal("expected extract in dataFrom")
	}
	if spec.DataFrom[0].Extract.Key != config.OnePasswordItem {
		t.Errorf("expected key %q, got %q", config.OnePasswordItem, spec.DataFrom[0].Extract.Key)
	}
}

func TestBuildKubeconfigExternalSecret_RefreshInterval(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildKubeconfigExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if spec.RefreshInterval != "15m" {
		t.Errorf("expected refresh '15m', got %q", spec.RefreshInterval)
	}
}

// ---------------------------------------------------------------------------
// buildKubeconfigSyncRBAC – deeper RBAC resource verification
// ---------------------------------------------------------------------------

func TestBuildKubeconfigSyncRBAC_ExternalSecretSpec(t *testing.T) {
	config := newBuilderTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	es := resources[0]
	spec, ok := es.Spec.(ku.ExternalSecretSpec)
	if !ok {
		t.Fatal("first resource Spec is not ExternalSecretSpec")
	}
	if len(spec.Data) != 2 {
		t.Fatalf("expected 2 data entries, got %d", len(spec.Data))
	}

	tests := []struct {
		idx       int
		secretKey string
		remoteKey string
		property  string
	}{
		{0, "token", "onepassword-access-token", "credential"},
		{1, "vault", "onepassword-access-token", "vault"},
	}
	for _, tt := range tests {
		t.Run(tt.secretKey, func(t *testing.T) {
			d := spec.Data[tt.idx]
			if d.SecretKey != tt.secretKey {
				t.Errorf("expected secretKey %q, got %q", tt.secretKey, d.SecretKey)
			}
			if d.RemoteRef.Key != tt.remoteKey {
				t.Errorf("expected remoteRef.key %q, got %q", tt.remoteKey, d.RemoteRef.Key)
			}
			if d.RemoteRef.Property != tt.property {
				t.Errorf("expected property %q, got %q", tt.property, d.RemoteRef.Property)
			}
		})
	}
}

func TestBuildKubeconfigSyncRBAC_ExternalSecretTargetName(t *testing.T) {
	config := newBuilderTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	es := resources[0]
	spec := es.Spec.(ku.ExternalSecretSpec)
	want := "builder-test-onepassword-token"
	if spec.Target.Name != want {
		t.Errorf("expected target name %q, got %q", want, spec.Target.Name)
	}
}

func TestBuildKubeconfigSyncRBAC_RolePolicyRules(t *testing.T) {
	config := newBuilderTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	role := resources[2]
	if len(role.Rules) != 1 {
		t.Fatalf("expected 1 policy rule, got %d", len(role.Rules))
	}
	rule := role.Rules[0]
	if len(rule.APIGroups) != 1 || rule.APIGroups[0] != "" {
		t.Errorf("expected core API group, got %v", rule.APIGroups)
	}
	if len(rule.Resources) != 1 || rule.Resources[0] != "secrets" {
		t.Errorf("expected secrets resource, got %v", rule.Resources)
	}
	if len(rule.Verbs) != 1 || rule.Verbs[0] != "get" {
		t.Errorf("expected [get] verbs, got %v", rule.Verbs)
	}
	// ResourceNames should include kubeconfig secret and 1password token
	expectedNames := map[string]bool{
		config.KubeconfigSecret:                   true,
		"builder-test-onepassword-token": true,
	}
	for _, rn := range rule.ResourceNames {
		if !expectedNames[rn] {
			t.Errorf("unexpected resourceName %q", rn)
		}
		delete(expectedNames, rn)
	}
	if len(expectedNames) > 0 {
		t.Errorf("missing resourceNames: %v", expectedNames)
	}
}

func TestBuildKubeconfigSyncRBAC_AllNamespacesMatch(t *testing.T) {
	config := newBuilderTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	for i, r := range resources {
		if r.Metadata.Namespace != config.TargetNamespace {
			t.Errorf("resource[%d] (%s) namespace %q != %q", i, r.Kind, r.Metadata.Namespace, config.TargetNamespace)
		}
	}
}

func TestBuildKubeconfigSyncRBAC_ConsistentSAName(t *testing.T) {
	config := newBuilderTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	wantSA := "builder-test-kubeconfig-sync"

	// SA
	if resources[1].Metadata.Name != wantSA {
		t.Errorf("SA name: got %q, want %q", resources[1].Metadata.Name, wantSA)
	}
	// Role
	if resources[2].Metadata.Name != wantSA {
		t.Errorf("Role name: got %q, want %q", resources[2].Metadata.Name, wantSA)
	}
	// RoleBinding
	if resources[3].Metadata.Name != wantSA {
		t.Errorf("RoleBinding name: got %q, want %q", resources[3].Metadata.Name, wantSA)
	}
	// RoleRef target
	if resources[3].RoleRef.Name != wantSA {
		t.Errorf("RoleBinding roleRef.name: got %q, want %q", resources[3].RoleRef.Name, wantSA)
	}
	// Subject name
	if resources[3].Subjects[0].Name != wantSA {
		t.Errorf("Subject name: got %q, want %q", resources[3].Subjects[0].Name, wantSA)
	}
	if resources[3].Subjects[0].Namespace != config.TargetNamespace {
		t.Errorf("Subject namespace: got %q, want %q", resources[3].Subjects[0].Namespace, config.TargetNamespace)
	}
}

// ---------------------------------------------------------------------------
// buildKubeconfigSyncJob — deep container / volume verification
// ---------------------------------------------------------------------------

func TestBuildKubeconfigSyncJob_ServiceAccount(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	want := "builder-test-kubeconfig-sync"
	if spec.Template.Spec.ServiceAccountName != want {
		t.Errorf("expected SA %q, got %q", want, spec.Template.Spec.ServiceAccountName)
	}
}

func TestBuildKubeconfigSyncJob_InitContainerWaitsForKubeconfig(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	init := spec.Template.Spec.InitContainers[0]
	if init.Name != "wait-for-kubeconfig" {
		t.Errorf("expected init name 'wait-for-kubeconfig', got %q", init.Name)
	}
	if init.Image != "busybox:1.36" {
		t.Errorf("expected busybox:1.36, got %q", init.Image)
	}
	if len(init.VolumeMounts) != 1 || init.VolumeMounts[0].MountPath != "/kubeconfig" {
		t.Error("expected volume mount at /kubeconfig")
	}
}

func TestBuildKubeconfigSyncJob_SyncContainerEnvVars(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	container := spec.Template.Spec.Containers[0]

	// Collect all env vars by name
	envByName := map[string]ku.EnvVar{}
	for _, e := range container.Env {
		envByName[e.Name] = e
	}

	// Plain value env vars
	plainExpect := map[string]string{
		"OP_CONNECT_HOST":      config.OnePasswordConnectHost,
		"CLUSTER_NAME":         config.Name,
		"KUBECONFIG_KEY":       config.KubeconfigKey,
		"OP_ITEM_NAME":         config.OnePasswordItem,
		"BASE_DOMAIN":          config.BaseDomain,
		"BASE_DOMAIN_SANITIZED": config.BaseDomainSanitized,
		"EXTERNAL_SERVER_URL":  config.ExternalServerURL,
		"ARGOCD_ENVIRONMENT":   config.Environment,
	}
	for name, want := range plainExpect {
		t.Run("env_"+name, func(t *testing.T) {
			ev, ok := envByName[name]
			if !ok {
				t.Fatalf("missing env var %s", name)
			}
			if ev.Value != want {
				t.Errorf("got %q, want %q", ev.Value, want)
			}
		})
	}

	// SecretKeyRef env vars
	secretExpect := map[string]struct{ secret, key string }{
		"OP_CONNECT_TOKEN": {"builder-test-onepassword-token", "token"},
		"OP_VAULT":         {"builder-test-onepassword-token", "vault"},
	}
	for name, want := range secretExpect {
		t.Run("secretEnv_"+name, func(t *testing.T) {
			ev, ok := envByName[name]
			if !ok {
				t.Fatalf("missing env var %s", name)
			}
			if ev.ValueFrom == nil || ev.ValueFrom.SecretKeyRef == nil {
				t.Fatal("expected secretKeyRef")
			}
			if ev.ValueFrom.SecretKeyRef.Name != want.secret {
				t.Errorf("secret name: got %q, want %q", ev.ValueFrom.SecretKeyRef.Name, want.secret)
			}
			if ev.ValueFrom.SecretKeyRef.Key != want.key {
				t.Errorf("key: got %q, want %q", ev.ValueFrom.SecretKeyRef.Key, want.key)
			}
		})
	}
}

func TestBuildKubeconfigSyncJob_VolumeReferencesKubeconfigSecret(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	vol := spec.Template.Spec.Volumes[0]
	if vol.Name != "kubeconfig" {
		t.Errorf("expected volume name 'kubeconfig', got %q", vol.Name)
	}
	if vol.Secret == nil {
		t.Fatal("expected secret volume source")
	}
	if vol.Secret.SecretName != config.KubeconfigSecret {
		t.Errorf("expected secretName %q, got %q", config.KubeconfigSecret, vol.Secret.SecretName)
	}
	if vol.Secret.Optional {
		t.Error("expected optional=false")
	}
}

func TestBuildKubeconfigSyncJob_ContainerReadOnlyMount(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	vm := spec.Template.Spec.Containers[0].VolumeMounts[0]
	if !vm.ReadOnly {
		t.Error("expected readOnly mount on sync container")
	}
}

func TestBuildKubeconfigSyncJob_PodLabels(t *testing.T) {
	config := newBuilderTestConfig()
	job := buildKubeconfigSyncJob(config)

	spec := job.Spec.(ku.JobSpec)
	labels := spec.Template.Metadata.Labels
	if labels["app.kubernetes.io/name"] != "kubeconfig-sync" {
		t.Errorf("expected pod label app name 'kubeconfig-sync', got %q", labels["app.kubernetes.io/name"])
	}
	if labels["app.kubernetes.io/instance"] != config.Name {
		t.Errorf("expected pod label instance %q, got %q", config.Name, labels["app.kubernetes.io/instance"])
	}
}

// ---------------------------------------------------------------------------
// buildArgoCDClusterExternalSecret — deep target/template verification
// ---------------------------------------------------------------------------

func TestBuildArgoCDClusterExternalSecret_TargetName(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if spec.Target.Name != "cluster-builder-test" {
		t.Errorf("expected target 'cluster-builder-test', got %q", spec.Target.Name)
	}
}

func TestBuildArgoCDClusterExternalSecret_TemplateData(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if spec.Target.Template == nil {
		t.Fatal("expected template")
	}
	tmpl := spec.Target.Template
	if tmpl.EngineVersion != "v2" {
		t.Errorf("expected engine v2, got %q", tmpl.EngineVersion)
	}
	if tmpl.Type != "Opaque" {
		t.Errorf("expected type Opaque, got %q", tmpl.Type)
	}
	for _, key := range []string{"name", "server", "config"} {
		if _, ok := tmpl.Data[key]; !ok {
			t.Errorf("missing template data key %q", key)
		}
	}
}

func TestBuildArgoCDClusterExternalSecret_TemplateMetadataLabels(t *testing.T) {
	config := newBuilderTestConfig()
	config.ClusterLabels = map[string]string{"custom-label": "yes"}
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	meta := spec.Target.Template.Metadata
	if meta == nil {
		t.Fatal("expected template metadata")
	}
	if meta.Labels["argocd.argoproj.io/secret-type"] != "cluster" {
		t.Error("missing argocd secret-type in template labels")
	}
	if meta.Labels["integratn.tech/cluster-name"] != config.Name {
		t.Error("missing cluster-name in template labels")
	}
	if meta.Labels["integratn.tech/environment"] != config.Environment {
		t.Error("missing environment in template labels")
	}
	if meta.Labels["custom-label"] != "yes" {
		t.Error("cluster labels not merged into template labels")
	}
}

func TestBuildArgoCDClusterExternalSecret_TemplateMetadataAnnotations(t *testing.T) {
	config := newBuilderTestConfig()
	config.ClusterAnnotations = map[string]string{"team": "platform"}
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	meta := spec.Target.Template.Metadata
	if meta.Annotations == nil || meta.Annotations["team"] != "platform" {
		t.Error("expected template metadata annotations")
	}
}

func TestBuildArgoCDClusterExternalSecret_NoAnnotationsWhenEmpty(t *testing.T) {
	config := newBuilderTestConfig()
	config.ClusterAnnotations = nil
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	meta := spec.Target.Template.Metadata
	if meta.Annotations != nil {
		t.Error("expected no template annotations when ClusterAnnotations is nil")
	}
	if es.Metadata.Annotations != nil {
		t.Error("expected no metadata annotations when ClusterAnnotations is nil")
	}
}

func TestBuildArgoCDClusterExternalSecret_DataFromExtract(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if len(spec.DataFrom) != 1 {
		t.Fatalf("expected 1 dataFrom, got %d", len(spec.DataFrom))
	}
	extract := spec.DataFrom[0].Extract
	if extract == nil {
		t.Fatal("expected extract")
	}
	if extract.Key != config.OnePasswordItem {
		t.Errorf("expected key %q, got %q", config.OnePasswordItem, extract.Key)
	}
	if extract.ConversionStrategy != "Default" {
		t.Errorf("expected conversion strategy 'Default', got %q", extract.ConversionStrategy)
	}
	if extract.DecodingStrategy != "None" {
		t.Errorf("expected decoding strategy 'None', got %q", extract.DecodingStrategy)
	}
}

func TestBuildArgoCDClusterExternalSecret_ShortRefreshInterval(t *testing.T) {
	config := newBuilderTestConfig()
	es := buildArgoCDClusterExternalSecret(config)

	spec := es.Spec.(ku.ExternalSecretSpec)
	if spec.RefreshInterval != "1m" {
		t.Errorf("expected refresh '1m', got %q", spec.RefreshInterval)
	}
}
