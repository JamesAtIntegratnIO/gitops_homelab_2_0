package vcluster

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/platform"
)

func TestApplyFlagOverrides_K8sVersion(t *testing.T) {
	spec := &platform.VClusterSpec{}
	opts := &CreateOptions{Core: CoreOpts{K8sVersion: "1.30.0"}}
	cmd := newCreateCmd()

	applyFlagOverrides(cmd, opts, spec)
	if spec.VCluster.K8sVersion != "1.30.0" {
		t.Errorf("K8sVersion = %q, want '1.30.0'", spec.VCluster.K8sVersion)
	}
}

func TestApplyFlagOverrides_Replicas(t *testing.T) {
	spec := &platform.VClusterSpec{}
	opts := &CreateOptions{Core: CoreOpts{Replicas: 3}}
	cmd := newCreateCmd()

	applyFlagOverrides(cmd, opts, spec)
	if spec.VCluster.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3", spec.VCluster.Replicas)
	}
}

func TestApplyFlagOverrides_IsolationMode(t *testing.T) {
	spec := &platform.VClusterSpec{}
	opts := &CreateOptions{Core: CoreOpts{IsolationMode: "strict"}}
	cmd := newCreateCmd()

	applyFlagOverrides(cmd, opts, spec)
	if spec.VCluster.IsolationMode != "strict" {
		t.Errorf("IsolationMode = %q, want 'strict'", spec.VCluster.IsolationMode)
	}
}

func TestApplyFlagOverrides_NoOp_WhenEmpty(t *testing.T) {
	spec := &platform.VClusterSpec{
		VCluster: platform.VClusterConfig{
			K8sVersion:    "1.29.0",
			Replicas:      1,
			IsolationMode: "standard",
		},
	}
	opts := &CreateOptions{} // all zero values
	cmd := newCreateCmd()

	applyFlagOverrides(cmd, opts, spec)
	// Should not overwrite existing values with zero values
	if spec.VCluster.K8sVersion != "1.29.0" {
		t.Errorf("K8sVersion changed to %q, should remain '1.29.0'", spec.VCluster.K8sVersion)
	}
	// Replicas 0 means "don't override"
	if spec.VCluster.Replicas != 1 {
		t.Errorf("Replicas changed to %d, should remain 1", spec.VCluster.Replicas)
	}
}

func TestApplyProdPresetExtras(t *testing.T) {
	spec := &platform.VClusterSpec{
		VCluster: platform.VClusterConfig{
			Replicas: 3,
		},
	}

	applyProdPresetExtras(spec, "production")

	if spec.VCluster.HelmOverrides == nil {
		t.Fatal("HelmOverrides should not be nil after prod preset")
	}

	cp, ok := spec.VCluster.HelmOverrides["controlPlane"].(map[string]interface{})
	if !ok {
		t.Fatal("expected controlPlane in HelmOverrides")
	}

	// Check etcd-certs volume
	ss, ok := cp["statefulSet"].(map[string]interface{})
	if !ok {
		t.Fatal("expected statefulSet in controlPlane")
	}
	persistence, ok := ss["persistence"].(map[string]interface{})
	if !ok {
		t.Fatal("expected persistence in statefulSet")
	}
	volumes, ok := persistence["addVolumes"].([]interface{})
	if !ok || len(volumes) == 0 {
		t.Fatal("expected addVolumes in persistence")
	}
	vol := volumes[0].(map[string]interface{})
	if vol["name"] != "etcd-certs" {
		t.Errorf("volume name = %v, want 'etcd-certs'", vol["name"])
	}
	secretMap := vol["secret"].(map[string]interface{})
	if secretMap["secretName"] != "production-etcd-certs" {
		t.Errorf("secretName = %v, want 'production-etcd-certs'", secretMap["secretName"])
	}

	// Check backing store
	bs, ok := cp["backingStore"].(map[string]interface{})
	if !ok {
		t.Fatal("expected backingStore in controlPlane")
	}
	etcd, ok := bs["etcd"].(map[string]interface{})
	if !ok {
		t.Fatal("expected etcd in backingStore")
	}
	deploy, ok := etcd["deploy"].(map[string]interface{})
	if !ok {
		t.Fatal("expected deploy in etcd")
	}
	if enabled, ok := deploy["enabled"].(bool); !ok || !enabled {
		t.Error("etcd deploy should be enabled")
	}

	// Check ingress disabled
	ingress, ok := cp["ingress"].(map[string]interface{})
	if !ok {
		t.Fatal("expected ingress in controlPlane")
	}
	if ingress["enabled"] != false {
		t.Error("ingress should be disabled for prod")
	}
}

func TestApplyProdPresetExtras_MetricsServerDisabled(t *testing.T) {
	spec := &platform.VClusterSpec{
		VCluster: platform.VClusterConfig{Replicas: 3},
	}

	applyProdPresetExtras(spec, "test")

	integrations, ok := spec.VCluster.HelmOverrides["integrations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected integrations in HelmOverrides")
	}
	ms, ok := integrations["metricsServer"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metricsServer in integrations")
	}
	if ms["enabled"] != false {
		t.Error("metricsServer should be disabled for prod preset")
	}
}
