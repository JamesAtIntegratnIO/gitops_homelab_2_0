package main

import (
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// newValuesTestConfig returns a fully-populated config for value builder tests.
func newValuesTestConfig() *HTTPServiceConfig {
	return &HTTPServiceConfig{
		Name:      "valapp",
		Namespace: "valns",
		Team:      "teamA",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "myrepo/valapp",
			ImageTag:        "v2.0",
			ImagePullPolicy: "Always",
			Command:         []string{"/app"},
			Args:            []string{"--port=8080"},
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      2,
			CPURequest:    "200m",
			MemoryRequest: "256Mi",
			CPULimit:      "1",
			MemoryLimit:   "512Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{Port: 9090},
		HealthCheckPath:   "/healthz",
		HealthCheckPort:   9090,
	}
}

// ---------------------------------------------------------------------------
// buildDeploymentValues
// ---------------------------------------------------------------------------

func TestBuildDeploymentValues_Image(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	img := d["image"].(map[string]interface{})
	if img["repository"] != "myrepo/valapp" {
		t.Errorf("expected repo 'myrepo/valapp', got %v", img["repository"])
	}
	if img["tag"] != "v2.0" {
		t.Errorf("expected tag 'v2.0', got %v", img["tag"])
	}
	if img["pullPolicy"] != "Always" {
		t.Errorf("expected pullPolicy 'Always', got %v", img["pullPolicy"])
	}
}

func TestBuildDeploymentValues_CommandAndArgs(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	cmd := d["command"].([]string)
	if len(cmd) != 1 || cmd[0] != "/app" {
		t.Errorf("unexpected command: %v", cmd)
	}
	args := d["args"].([]string)
	if len(args) != 1 || args[0] != "--port=8080" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestBuildDeploymentValues_Ports(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	ports := d["ports"].([]map[string]interface{})
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(ports))
	}
	if ports[0]["containerPort"] != 9090 {
		t.Errorf("expected containerPort 9090, got %v", ports[0]["containerPort"])
	}
	if ports[0]["name"] != "http" {
		t.Errorf("expected port name 'http', got %v", ports[0]["name"])
	}
	if ports[0]["protocol"] != "TCP" {
		t.Errorf("expected protocol 'TCP', got %v", ports[0]["protocol"])
	}
}

func TestBuildDeploymentValues_Resources(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	resources := d["resources"].(map[string]interface{})
	requests := resources["requests"].(map[string]string)
	if requests["cpu"] != "200m" {
		t.Errorf("expected cpu request '200m', got %q", requests["cpu"])
	}
	if requests["memory"] != "256Mi" {
		t.Errorf("expected memory request '256Mi', got %q", requests["memory"])
	}
	limits := resources["limits"].(map[string]string)
	if limits["cpu"] != "1" {
		t.Errorf("expected cpu limit '1', got %q", limits["cpu"])
	}
	if limits["memory"] != "512Mi" {
		t.Errorf("expected memory limit '512Mi', got %q", limits["memory"])
	}
}

func TestBuildDeploymentValues_ReadinessProbe(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	probe := d["readinessProbe"].(map[string]interface{})
	if probe["enabled"] != true {
		t.Error("expected readinessProbe enabled")
	}
	httpGet := probe["httpGet"].(map[string]interface{})
	if httpGet["path"] != "/healthz" {
		t.Errorf("expected path '/healthz', got %v", httpGet["path"])
	}
	if httpGet["port"] != 9090 {
		t.Errorf("expected port 9090, got %v", httpGet["port"])
	}
	if httpGet["scheme"] != "HTTP" {
		t.Errorf("expected scheme 'HTTP', got %v", httpGet["scheme"])
	}
}

func TestBuildDeploymentValues_LivenessProbe(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	probe := d["livenessProbe"].(map[string]interface{})
	if probe["enabled"] != true {
		t.Error("expected livenessProbe enabled")
	}
	if probe["failureThreshold"] != 3 {
		t.Errorf("expected failureThreshold 3, got %v", probe["failureThreshold"])
	}
	if probe["periodSeconds"] != 10 {
		t.Errorf("expected periodSeconds 10, got %v", probe["periodSeconds"])
	}
}

func TestBuildDeploymentValues_StaticFields(t *testing.T) {
	config := newValuesTestConfig()
	d := buildDeploymentValues(config)

	if d["enabled"] != true {
		t.Error("expected enabled true")
	}
	if d["replicas"] != 2 {
		t.Errorf("expected replicas 2, got %v", d["replicas"])
	}
	if d["revisionHistoryLimit"] != 3 {
		t.Errorf("expected revisionHistoryLimit 3, got %v", d["revisionHistoryLimit"])
	}
	if d["reloadOnChange"] != true {
		t.Error("expected reloadOnChange true")
	}
}

func TestBuildDeploymentValues_ContainerSecurityContext(t *testing.T) {
	truev := true
	uid := int64(1000)
	config := newValuesTestConfig()
	config.RunAsNonRoot = &truev
	config.RunAsUser = &uid

	d := buildDeploymentValues(config)
	ctx := d["containerSecurityContext"].(map[string]interface{})
	if ctx["runAsNonRoot"] != true {
		t.Error("expected runAsNonRoot true")
	}
	if ctx["runAsUser"] != int64(1000) {
		t.Errorf("expected runAsUser 1000, got %v", ctx["runAsUser"])
	}
}

// ---------------------------------------------------------------------------
// buildServiceValues
// ---------------------------------------------------------------------------

func TestBuildServiceValues_Structure(t *testing.T) {
	config := newValuesTestConfig()
	svc := buildServiceValues(config)

	if svc["enabled"] != true {
		t.Error("expected enabled true")
	}
	if svc["type"] != "ClusterIP" {
		t.Errorf("expected type 'ClusterIP', got %v", svc["type"])
	}

	ports := svc["ports"].([]map[string]interface{})
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(ports))
	}
	if ports[0]["port"] != 9090 {
		t.Errorf("expected port 9090, got %v", ports[0]["port"])
	}
	if ports[0]["targetPort"] != 9090 {
		t.Errorf("expected targetPort 9090, got %v", ports[0]["targetPort"])
	}
	if ports[0]["name"] != "http" {
		t.Errorf("expected name 'http', got %v", ports[0]["name"])
	}
	if ports[0]["protocol"] != "TCP" {
		t.Errorf("expected protocol 'TCP', got %v", ports[0]["protocol"])
	}
}

// ---------------------------------------------------------------------------
// applyEnvToDeployment
// ---------------------------------------------------------------------------

func TestApplyEnvToDeployment_WithEnvVars(t *testing.T) {
	config := &HTTPServiceConfig{
		Env: map[string]string{"PORT": "3000", "NODE_ENV": "production"},
	}
	deployment := map[string]interface{}{}
	applyEnvToDeployment(config, deployment)

	env := deployment["env"].(map[string]interface{})
	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(env))
	}
	portEntry := env["PORT"].(map[string]interface{})
	if portEntry["value"] != "3000" {
		t.Errorf("expected PORT=3000, got %v", portEntry["value"])
	}
}

func TestApplyEnvToDeployment_NoEnv(t *testing.T) {
	config := &HTTPServiceConfig{}
	deployment := map[string]interface{}{}
	applyEnvToDeployment(config, deployment)

	if _, ok := deployment["env"]; ok {
		t.Error("expected no env key when Env is empty")
	}
	if _, ok := deployment["envFrom"]; ok {
		t.Error("expected no envFrom key when no secrets")
	}
}

func TestApplyEnvToDeployment_EnvFromSecrets(t *testing.T) {
	config := &HTTPServiceConfig{
		EnvFromSecrets: []string{"db-creds", "api-token"},
	}
	deployment := map[string]interface{}{}
	applyEnvToDeployment(config, deployment)

	envFrom := deployment["envFrom"].(map[string]interface{})
	if len(envFrom) != 2 {
		t.Fatalf("expected 2 envFrom entries, got %d", len(envFrom))
	}
	dbEntry := envFrom["db-creds"].(map[string]interface{})
	if dbEntry["type"] != "secret" {
		t.Error("expected type 'secret'")
	}
	if dbEntry["nameSuffix"] != "db-creds" {
		t.Errorf("expected nameSuffix 'db-creds', got %v", dbEntry["nameSuffix"])
	}
}

func TestApplyEnvToDeployment_SecretsGenerateName(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Secrets: []ku.SecretRef{
			{OnePasswordItem: "vault-item"}, // no Name -> generated
		},
	}
	deployment := map[string]interface{}{}
	applyEnvToDeployment(config, deployment)

	envFrom := deployment["envFrom"].(map[string]interface{})
	generatedName := "web-vault-item"
	entry, ok := envFrom[generatedName].(map[string]interface{})
	if !ok {
		t.Fatalf("expected envFrom key %q", generatedName)
	}
	if entry["nameSuffix"] != generatedName {
		t.Errorf("expected nameSuffix %q, got %v", generatedName, entry["nameSuffix"])
	}
}

func TestApplyEnvToDeployment_SecretsWithExplicitName(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Secrets: []ku.SecretRef{
			{Name: "my-secret", OnePasswordItem: "vault-item"},
		},
	}
	deployment := map[string]interface{}{}
	applyEnvToDeployment(config, deployment)

	envFrom := deployment["envFrom"].(map[string]interface{})
	entry, ok := envFrom["my-secret"].(map[string]interface{})
	if !ok {
		t.Fatal("expected envFrom key 'my-secret'")
	}
	if entry["nameSuffix"] != "my-secret" {
		t.Errorf("expected nameSuffix 'my-secret', got %v", entry["nameSuffix"])
	}
}

// ---------------------------------------------------------------------------
// buildSecurityContext — partial values (complement main_test.go)
// ---------------------------------------------------------------------------

func TestBuildSecurityContext_PartialValues(t *testing.T) {
	falsev := false
	gid := int64(500)
	config := &HTTPServiceConfig{
		HTTPSecurityConfig: HTTPSecurityConfig{
			RunAsNonRoot:           &falsev,
			ReadOnlyRootFilesystem: nil,
			RunAsGroup:             &gid,
		},
	}
	ctx := buildSecurityContext(config)

	if ctx["runAsNonRoot"] != false {
		t.Error("expected runAsNonRoot false when explicitly set")
	}
	if ctx["readOnlyRootFilesystem"] != false {
		t.Error("expected readOnlyRootFilesystem default false when nil")
	}
	if _, ok := ctx["runAsUser"]; ok {
		t.Error("runAsUser should not be set when nil")
	}
	if ctx["runAsGroup"] != int64(500) {
		t.Errorf("expected runAsGroup 500, got %v", ctx["runAsGroup"])
	}
}

// ---------------------------------------------------------------------------
// buildStakaterValues — additional paths not covered by main_test.go
// ---------------------------------------------------------------------------

func TestBuildStakaterValues_AdditionalLabels(t *testing.T) {
	config := newValuesTestConfig()
	values := buildStakaterValues(config)

	labels := values["additionalLabels"].(map[string]string)
	if labels["app.kubernetes.io/managed-by"] != "kratix" {
		t.Error("missing managed-by label")
	}
	if labels["kratix.io/promise-name"] != "http-service" {
		t.Error("missing promise-name label")
	}
	if labels["app.kubernetes.io/part-of"] != "valapp" {
		t.Error("missing part-of label")
	}
	if labels["app.kubernetes.io/team"] != "teamA" {
		t.Error("missing team label")
	}
}

func TestBuildStakaterValues_RBAC(t *testing.T) {
	config := newValuesTestConfig()
	values := buildStakaterValues(config)

	rbac := values["rbac"].(map[string]interface{})
	if rbac["enabled"] != true {
		t.Error("expected rbac enabled")
	}
	sa := rbac["serviceAccount"].(map[string]interface{})
	if sa["enabled"] != true {
		t.Error("expected serviceAccount enabled")
	}
	if sa["name"] != "valapp" {
		t.Errorf("expected SA name 'valapp', got %v", sa["name"])
	}
}

func TestBuildStakaterValues_PersistenceWithoutStorageClass(t *testing.T) {
	config := newValuesTestConfig()
	config.PersistenceEnabled = true
	config.PersistenceSize = "1Gi"
	config.PersistenceMountPath = "/data"
	config.PersistenceClass = "" // empty

	values := buildStakaterValues(config)
	p := values["persistence"].(map[string]interface{})
	if p["enabled"] != true {
		t.Error("expected persistence enabled")
	}
	if _, ok := p["storageClass"]; ok {
		t.Error("storageClass should not be set when empty")
	}
	if p["storageSize"] != "1Gi" {
		t.Errorf("expected storageSize '1Gi', got %v", p["storageSize"])
	}
	if p["mountPath"] != "/data" {
		t.Errorf("expected mountPath '/data', got %v", p["mountPath"])
	}
	if p["accessMode"] != "ReadWriteOnce" {
		t.Errorf("expected accessMode 'ReadWriteOnce', got %v", p["accessMode"])
	}
}

func TestBuildStakaterValues_MonitoringDisabled(t *testing.T) {
	config := newValuesTestConfig()
	config.MonitoringEnabled = false

	values := buildStakaterValues(config)
	if _, ok := values["serviceMonitor"]; ok {
		// If it exists, it should be in the disabled block
		sm := values["serviceMonitor"].(map[string]interface{})
		if sm["enabled"] != false {
			t.Error("expected serviceMonitor disabled when MonitoringEnabled=false")
		}
	}
}

func TestBuildStakaterValues_DisabledFeatures(t *testing.T) {
	config := newValuesTestConfig()
	values := buildStakaterValues(config)

	disabled := []string{
		"ingress", "route", "forecastle", "cronJob", "job", "configMap",
		"sealedSecret", "secret", "certificate", "secretProviderClass",
		"alertmanagerConfig", "prometheusRule", "externalSecret",
		"autoscaling", "vpa", "endpointMonitor", "pdb",
		"grafanaDashboard", "backup", "networkPolicy",
	}
	for _, key := range disabled {
		section, ok := values[key].(map[string]interface{})
		if !ok {
			t.Errorf("expected %s section", key)
			continue
		}
		if section["enabled"] != false {
			t.Errorf("expected %s disabled", key)
		}
	}
}

func TestBuildStakaterValues_HTTPRouteDisabled(t *testing.T) {
	config := newValuesTestConfig()
	values := buildStakaterValues(config)

	hr := values["httpRoute"].(map[string]interface{})
	if hr["enabled"] != false {
		t.Error("expected httpRoute disabled")
	}
}

func TestBuildStakaterValues_ServiceMonitorEndpoints(t *testing.T) {
	config := newValuesTestConfig()
	config.MonitoringEnabled = true
	config.MonitoringPath = "/metrics"
	config.MonitoringInterval = "30s"

	values := buildStakaterValues(config)
	sm := values["serviceMonitor"].(map[string]interface{})
	endpoints := sm["endpoints"].([]map[string]interface{})
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0]["port"] != "http" {
		t.Errorf("expected port 'http', got %v", endpoints[0]["port"])
	}
	if endpoints[0]["path"] != "/metrics" {
		t.Errorf("expected path '/metrics', got %v", endpoints[0]["path"])
	}
	if endpoints[0]["interval"] != "30s" {
		t.Errorf("expected interval '30s', got %v", endpoints[0]["interval"])
	}
}

func TestBuildStakaterValues_ServiceMonitorLabels(t *testing.T) {
	config := newValuesTestConfig()
	config.MonitoringEnabled = true
	config.MonitoringPath = "/metrics"
	config.MonitoringInterval = "30s"

	values := buildStakaterValues(config)
	sm := values["serviceMonitor"].(map[string]interface{})
	labels := sm["additionalLabels"].(map[string]string)
	if labels["release"] != "kube-prometheus-stack" {
		t.Errorf("expected release label 'kube-prometheus-stack', got %v", labels["release"])
	}
}
