package main

import (
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestBuildValuesObject_ServiceMonitorLabels(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := values["controlPlane"].(map[string]interface{})
	if !ok {
		t.Fatal("expected controlPlane map")
	}
	sm, ok := cp["serviceMonitor"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serviceMonitor in controlPlane")
	}
	if sm["enabled"] != true {
		t.Error("expected serviceMonitor enabled")
	}
	labels, ok := sm["labels"].(map[string]interface{})
	if !ok {
		t.Fatal("expected labels in serviceMonitor")
	}
	wantLabels := map[string]string{
		"vcluster_name":      config.Name,
		"vcluster_namespace": config.TargetNamespace,
		"environment":        config.ArgoCDEnvironment,
		"cluster_role":       "vcluster",
	}
	for k, want := range wantLabels {
		got, _ := labels[k].(string)
		if got != want {
			t.Errorf("serviceMonitor label %q = %q, want %q", k, got, want)
		}
	}
}

func TestBuildValuesObject_Integrations(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	integrations, ok := values["integrations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected integrations map")
	}

	es, ok := integrations["externalSecrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected externalSecrets in integrations")
	}
	if es["enabled"] != true {
		t.Error("expected externalSecrets enabled")
	}

	cm, ok := integrations["certManager"].(map[string]interface{})
	if !ok {
		t.Fatal("expected certManager in integrations")
	}
	if cm["enabled"] != true {
		t.Error("expected certManager enabled")
	}

	ms, ok := integrations["metricsServer"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metricsServer in integrations")
	}
	if ms["enabled"] != true {
		t.Error("expected metricsServer enabled")
	}
}

func TestBuildValuesObject_SyncConfig(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sync, ok := values["sync"].(map[string]interface{})
	if !ok {
		t.Fatal("expected sync map")
	}

	toHost, ok := sync["toHost"].(map[string]interface{})
	if !ok {
		t.Fatal("expected toHost in sync")
	}

	pods, ok := toHost["pods"].(map[string]interface{})
	if !ok {
		t.Fatal("expected pods in toHost")
	}
	if pods["enabled"] != true {
		t.Error("expected pods sync enabled")
	}

	fromHost, ok := sync["fromHost"].(map[string]interface{})
	if !ok {
		t.Fatal("expected fromHost in sync")
	}

	sc, ok := fromHost["storageClasses"].(map[string]interface{})
	if !ok {
		t.Fatal("expected storageClasses in fromHost")
	}
	if sc["enabled"] != true {
		t.Error("expected storageClasses sync enabled")
	}
}

func TestBuildValuesObject_RBAC(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rbac, ok := values["rbac"].(map[string]interface{})
	if !ok {
		t.Fatal("expected rbac map")
	}

	cr, ok := rbac["clusterRole"].(map[string]interface{})
	if !ok {
		t.Fatal("expected clusterRole in rbac")
	}
	if cr["enabled"] != true {
		t.Error("expected clusterRole enabled")
	}

	rules, ok := cr["extraRules"].([]interface{})
	if !ok || len(rules) == 0 {
		t.Fatal("expected at least 1 extra RBAC rule")
	}
}

func TestBuildValuesObject_TelemetryDisabled(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	telemetry, ok := values["telemetry"].(map[string]interface{})
	if !ok {
		t.Fatal("expected telemetry map")
	}
	if telemetry["enabled"] != false {
		t.Errorf("expected telemetry disabled, got %v", telemetry["enabled"])
	}
}

func TestBuildValuesObject_LoggingJSON(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logging, ok := values["logging"].(map[string]interface{})
	if !ok {
		t.Fatal("expected logging map")
	}
	if logging["encoding"] != "json" {
		t.Errorf("expected logging encoding 'json', got %v", logging["encoding"])
	}
}

func TestBuildValuesObject_NonStandardAPIPort(t *testing.T) {
	config := minimalConfig()
	config.APIPort = 6443

	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := values["controlPlane"].(map[string]interface{})
	service := cp["service"].(map[string]interface{})
	spec := service["spec"].(map[string]interface{})

	ports, ok := spec["ports"].([]interface{})
	if !ok {
		t.Fatal("expected ports slice")
	}

	// Should have 2 ports: main (6443) + internal (443)
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports when APIPort != 443, got %d", len(ports))
	}

	// Check first port is the main one
	p0, ok := ports[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected port map")
	}
	// After ToMap, port might be float64
	portVal := toInt(p0["port"])
	if portVal != 6443 {
		t.Errorf("first port = %v, want 6443", p0["port"])
	}

	// Check second port is the internal 443
	p1, ok := ports[1].(map[string]interface{})
	if !ok {
		t.Fatal("expected port map")
	}
	portVal1 := toInt(p1["port"])
	if portVal1 != 443 {
		t.Errorf("second port = %v, want 443", p1["port"])
	}
}

// toInt converts numeric types (int, float64) to int for comparison.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	default:
		return -1
	}
}

func TestBuildValuesObject_PersistenceClass(t *testing.T) {
	config := minimalConfig()
	config.PersistenceClass = "longhorn"
	config.PersistenceEnabled = true

	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := values["controlPlane"].(map[string]interface{})
	ss := cp["statefulSet"].(map[string]interface{})
	persist := ss["persistence"].(map[string]interface{})
	vc := persist["volumeClaim"].(map[string]interface{})

	if vc["storageClass"] != "longhorn" {
		t.Errorf("storageClass = %v, want longhorn", vc["storageClass"])
	}
}

func TestBuildValuesObject_ExportKubeConfig(t *testing.T) {
	config := minimalConfig()
	config.ExportKubeConfig = map[string]interface{}{
		"context": map[string]interface{}{
			"name": "my-context",
		},
	}

	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ekc, ok := values["exportKubeConfig"]
	if !ok || ekc == nil {
		t.Fatal("expected exportKubeConfig in values")
	}
	ekcMap, ok := ekc.(map[string]interface{})
	if !ok {
		t.Fatal("expected exportKubeConfig to be a map")
	}
	ctx, ok := ekcMap["context"].(map[string]interface{})
	if !ok {
		t.Fatal("expected context in exportKubeConfig")
	}
	if ctx["name"] != "my-context" {
		t.Errorf("context name = %v, want my-context", ctx["name"])
	}
}

func TestBuildValuesObject_Networking(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	net, ok := values["networking"].(map[string]interface{})
	if !ok {
		t.Fatal("expected networking map")
	}

	adv, ok := net["advanced"].(map[string]interface{})
	if !ok {
		t.Fatal("expected advanced in networking")
	}
	if adv["clusterDomain"] != config.ClusterDomain {
		t.Errorf("clusterDomain = %v, want %q", adv["clusterDomain"], config.ClusterDomain)
	}

	rs, ok := net["replicateServices"].(map[string]interface{})
	if !ok {
		t.Fatal("expected replicateServices in networking")
	}
	fromHost, ok := rs["fromHost"].([]interface{})
	if !ok || len(fromHost) == 0 {
		t.Fatal("expected fromHost service mappings")
	}
}

func TestBuildValuesObject_StatefulSetResources(t *testing.T) {
	config := minimalConfig()
	config.CPURequest = "500m"
	config.MemoryRequest = "1Gi"
	config.CPULimit = "2"
	config.MemoryLimit = "4Gi"

	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := values["controlPlane"].(map[string]interface{})
	ss := cp["statefulSet"].(map[string]interface{})
	res := ss["resources"].(map[string]interface{})

	requests := res["requests"].(map[string]interface{})
	if requests["cpu"] != "500m" {
		t.Errorf("cpu request = %v, want 500m", requests["cpu"])
	}
	if requests["memory"] != "1Gi" {
		t.Errorf("memory request = %v, want 1Gi", requests["memory"])
	}

	limits := res["limits"].(map[string]interface{})
	if limits["cpu"] != "2" {
		t.Errorf("cpu limit = %v, want 2", limits["cpu"])
	}
	if limits["memory"] != "4Gi" {
		t.Errorf("memory limit = %v, want 4Gi", limits["memory"])
	}
}

func TestApplyPresetDefaults_CorednsReplicasOverride(t *testing.T) {
	config := &VClusterConfig{Preset: "dev"}
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"vcluster": map[string]interface{}{
					"coredns": map[string]interface{}{
						"replicas": float64(3),
					},
				},
			},
		},
	}
	if err := applyPresetDefaults(config, resource); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.CorednsReplicas != 3 {
		t.Errorf("expected coredns replicas 3, got %d", config.CorednsReplicas)
	}
}

func TestApplyPresetDefaults_MemoryLimitOverride(t *testing.T) {
	config := &VClusterConfig{Preset: "prod"}
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"vcluster": map[string]interface{}{
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"memory": "8Gi",
						},
					},
				},
			},
		},
	}
	if err := applyPresetDefaults(config, resource); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.MemoryLimit != "8Gi" {
		t.Errorf("expected memory limit '8Gi', got %q", config.MemoryLimit)
	}
	// Non-overridden fields should get prod defaults
	if config.CPURequest != "500m" {
		t.Errorf("expected prod CPURequest '500m', got %q", config.CPURequest)
	}
}

func TestBuildValuesObject_DeployMetalLBEnabled(t *testing.T) {
	config := minimalConfig()
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy, ok := values["deploy"].(map[string]interface{})
	if !ok {
		t.Fatal("expected deploy map")
	}
	metallb, ok := deploy["metallb"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metallb in deploy")
	}
	if metallb["enabled"] != true {
		t.Error("expected metallb enabled")
	}
}

func TestBuildValuesObject_ServiceAnnotations(t *testing.T) {
	config := minimalConfig()
	config.Hostname = "custom.integratn.tech"
	values, err := buildValuesObject(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := values["controlPlane"].(map[string]interface{})
	svc := cp["service"].(map[string]interface{})
	annotations, ok := svc["annotations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected annotations in service")
	}
	want := "custom.integratn.tech"
	got, _ := annotations["external-dns.alpha.kubernetes.io/hostname"].(string)
	if got != want {
		t.Errorf("external-dns hostname = %q, want %q", got, want)
	}
}
