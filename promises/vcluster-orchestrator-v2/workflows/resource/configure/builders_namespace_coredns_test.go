package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestConfigMapCorefile_ContainsClusterDomain(t *testing.T) {
	tests := []struct {
		name          string
		clusterDomain string
	}{
		{"default", "cluster.local"},
		{"custom", "my-cluster.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corefile := configMapCorefile(tt.clusterDomain)
			if !strings.Contains(corefile, tt.clusterDomain) {
				t.Errorf("expected cluster domain %q in corefile", tt.clusterDomain)
			}
		})
	}
}

func TestConfigMapCorefile_ContainsRequiredSections(t *testing.T) {
	corefile := configMapCorefile("cluster.local")

	required := []string{
		"errors",
		"health",
		"ready",
		"kubernetes",
		"hosts /etc/coredns/NodeHosts",
		"prometheus :9153",
		"forward . /etc/resolv.conf",
		"cache 30",
		"loop",
		"reload",
		"loadbalance",
		"import /etc/coredns/custom/*.server",
	}
	for _, section := range required {
		if !strings.Contains(corefile, section) {
			t.Errorf("expected %q in configMap corefile", section)
		}
	}
}

func TestConfigMapCorefile_ListenPort(t *testing.T) {
	corefile := configMapCorefile("cluster.local")
	if !strings.HasPrefix(corefile, ".:1053") {
		t.Error("expected corefile to listen on .:1053")
	}
}

func TestHelmCorefileOverwrite_ContainsClusterDomain(t *testing.T) {
	tests := []struct {
		name          string
		clusterDomain string
	}{
		{"default", "cluster.local"},
		{"custom", "vcluster.internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corefile := helmCorefileOverwrite(tt.clusterDomain)
			if !strings.Contains(corefile, tt.clusterDomain) {
				t.Errorf("expected cluster domain %q in helm corefile", tt.clusterDomain)
			}
		})
	}
}

func TestHelmCorefileOverwrite_SlimmerThanConfigMap(t *testing.T) {
	helm := helmCorefileOverwrite("cluster.local")
	cm := configMapCorefile("cluster.local")

	// Helm variant should NOT have NodeHosts or custom server imports
	if strings.Contains(helm, "NodeHosts") {
		t.Error("helm corefile should not contain NodeHosts")
	}
	if strings.Contains(helm, "import /etc/coredns/custom") {
		t.Error("helm corefile should not contain custom server imports")
	}
	// ConfigMap variant SHOULD have them
	if !strings.Contains(cm, "NodeHosts") {
		t.Error("configMap corefile should contain NodeHosts")
	}
}

func TestHelmCorefileOverwrite_RequiredSections(t *testing.T) {
	corefile := helmCorefileOverwrite("cluster.local")

	required := []string{
		"errors",
		"health",
		"ready",
		"kubernetes",
		"prometheus",
		"forward . /etc/resolv.conf",
		"cache 30",
		"loop",
		"reload",
		"loadbalance",
	}
	for _, section := range required {
		if !strings.Contains(corefile, section) {
			t.Errorf("expected %q in helm corefile", section)
		}
	}
}

func TestBuildNamespace_APIVersion(t *testing.T) {
	config := minimalConfig()
	ns := buildNamespace(config)

	if ns.APIVersion != "v1" {
		t.Errorf("expected apiVersion 'v1', got %q", ns.APIVersion)
	}
}

func TestBuildNamespace_EmptyNamespaceField(t *testing.T) {
	config := minimalConfig()
	ns := buildNamespace(config)

	// Namespace resources should not have a namespace in metadata
	if ns.Metadata.Namespace != "" {
		t.Errorf("expected empty metadata.namespace for Namespace resource, got %q", ns.Metadata.Namespace)
	}
}

func TestBuildNamespace_SyncWaveAnnotation(t *testing.T) {
	config := minimalConfig()
	ns := buildNamespace(config)

	if ns.Metadata.Annotations["argocd.argoproj.io/sync-wave"] != "-2" {
		t.Errorf("expected sync-wave '-2', got %q", ns.Metadata.Annotations["argocd.argoproj.io/sync-wave"])
	}
}

func TestBuildNamespace_AllLabels(t *testing.T) {
	config := minimalConfig()
	ns := buildNamespace(config)

	wantLabels := map[string]string{
		"app.kubernetes.io/name":        "vcluster-namespace",
		"vcluster.loft.sh/namespace":    "true",
		"platform.integratn.tech/type":  "vcluster",
		"app.kubernetes.io/managed-by":  "kratix",
		"kratix.io/promise-name":        config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":       config.Name,
	}
	for k, want := range wantLabels {
		got := ns.Metadata.Labels[k]
		if got != want {
			t.Errorf("label %q: got %q, want %q", k, got, want)
		}
	}
}

func TestBuildCorednsConfigMap_APIVersion(t *testing.T) {
	config := minimalConfig()
	cm := buildCorednsConfigMap(config)

	if cm.APIVersion != "v1" {
		t.Errorf("expected apiVersion 'v1', got %q", cm.APIVersion)
	}
}

func TestBuildCorednsConfigMap_Labels(t *testing.T) {
	config := minimalConfig()
	cm := buildCorednsConfigMap(config)

	wantLabels := map[string]string{
		"app.kubernetes.io/name":       "coredns",
		"app.kubernetes.io/instance":   fmt.Sprintf("vc-%s", config.Name),
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":      config.Name,
	}
	for k, want := range wantLabels {
		got := cm.Metadata.Labels[k]
		if got != want {
			t.Errorf("label %q: got %q, want %q", k, got, want)
		}
	}
}

func TestBuildCorednsConfigMap_Namespace(t *testing.T) {
	config := minimalConfig()
	cm := buildCorednsConfigMap(config)

	if cm.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("expected namespace %q, got %q", config.TargetNamespace, cm.Metadata.Namespace)
	}
}

func TestBuildCorednsConfigMap_DataKeys(t *testing.T) {
	config := minimalConfig()
	cm := buildCorednsConfigMap(config)

	dataMap, ok := cm.Data.(map[string]string)
	if !ok {
		t.Fatal("expected Data to be map[string]string")
	}
	if _, exists := dataMap["Corefile"]; !exists {
		t.Error("expected 'Corefile' key in data")
	}
	if _, exists := dataMap["NodeHosts"]; !exists {
		t.Error("expected 'NodeHosts' key in data")
	}
	if dataMap["NodeHosts"] != "" {
		t.Errorf("expected empty NodeHosts, got %q", dataMap["NodeHosts"])
	}
}

func TestBuildCorednsConfigMap_UsesCorefileFunc(t *testing.T) {
	config := minimalConfig()
	config.ClusterDomain = "custom.domain"
	cm := buildCorednsConfigMap(config)

	dataMap := cm.Data.(map[string]string)
	expectedCorefile := configMapCorefile("custom.domain")
	if dataMap["Corefile"] != expectedCorefile {
		t.Error("expected corefile data to match configMapCorefile output")
	}
}

func TestBuildNamespace_DifferentTargetNamespaces(t *testing.T) {
	tests := []struct {
		name            string
		targetNamespace string
	}{
		{"default", "vcluster-test-vc"},
		{"custom", "my-custom-ns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := minimalConfig()
			config.TargetNamespace = tt.targetNamespace
			ns := buildNamespace(config)
			if ns.Metadata.Name != tt.targetNamespace {
				t.Errorf("expected name %q, got %q", tt.targetNamespace, ns.Metadata.Name)
			}
		})
	}
}
