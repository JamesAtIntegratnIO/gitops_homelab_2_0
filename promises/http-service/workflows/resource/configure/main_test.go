package main

import (
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestBuildConfig_MinimalValid(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
				"image": map[string]interface{}{
					"repository": "nginx",
				},
			},
		},
	}

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", config.Name)
	}
	if config.Namespace != "my-app" {
		t.Errorf("expected namespace defaulted to name 'my-app', got %q", config.Namespace)
	}
	if config.ImageRepository != "nginx" {
		t.Errorf("expected image 'nginx', got %q", config.ImageRepository)
	}
	if config.ImageTag != "latest" {
		t.Errorf("expected default tag 'latest', got %q", config.ImageTag)
	}
	if config.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %d", config.Replicas)
	}
	if config.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", config.Port)
	}
	if !config.IngressEnabled {
		t.Error("expected ingress enabled by default")
	}
	if config.IngressHostname != "my-app.cluster.integratn.tech" {
		t.Errorf("expected generated hostname, got %q", config.IngressHostname)
	}
	if config.Team != "platform" {
		t.Errorf("expected default team 'platform', got %q", config.Team)
	}
	if config.BaseDomain != defaultBaseDomain {
		t.Errorf("expected default base domain, got %q", config.BaseDomain)
	}
	if config.SecretStoreName != ku.DefaultSecretStoreName {
		t.Errorf("expected default secret store, got %q", config.SecretStoreName)
	}
}

func TestBuildConfig_WithAllFields(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "api-server",
				"namespace": "production",
				"team":      "backend",
				"image": map[string]interface{}{
					"repository": "myrepo/api",
					"tag":        "v1.2.3",
					"pullPolicy": "Always",
				},
				"replicas": 3,
				"port":     3000,
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"cpu":    "250m",
						"memory": "512Mi",
					},
					"limits": map[string]interface{}{
						"cpu":    "1",
						"memory": "1Gi",
					},
				},
				"ingress": map[string]interface{}{
					"enabled":  true,
					"hostname": "api.example.com",
					"path":     "/api",
				},
				"healthCheck": map[string]interface{}{
					"path": "/healthz",
					"port": 3001,
				},
				"monitoring": map[string]interface{}{
					"enabled":  true,
					"path":     "/prom",
					"interval": "15s",
				},
				"persistence": map[string]interface{}{
					"enabled":      true,
					"size":         "10Gi",
					"storageClass": "nfs",
					"mountPath":    "/var/data",
				},
				"env": map[string]interface{}{
					"PORT":     "3000",
					"NODE_ENV": "production",
				},
				"envFromSecrets": []interface{}{"db-creds"},
				"secrets": []interface{}{
					map[string]interface{}{
						"onePasswordItem": "api-secrets",
						"keys": []interface{}{
							map[string]interface{}{
								"secretKey": "api-key",
								"property":  "apiKey",
							},
						},
					},
				},
				"securityContext": map[string]interface{}{
					"runAsNonRoot":           true,
					"readOnlyRootFilesystem": true,
					"runAsUser":              1000,
					"runAsGroup":             1000,
				},
			},
		},
	}

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", config.Namespace)
	}
	if config.Team != "backend" {
		t.Errorf("expected team 'backend', got %q", config.Team)
	}
	if config.ImageTag != "v1.2.3" {
		t.Errorf("expected tag 'v1.2.3', got %q", config.ImageTag)
	}
	if config.Replicas != 3 {
		t.Errorf("expected replicas 3, got %d", config.Replicas)
	}
	if config.Port != 3000 {
		t.Errorf("expected port 3000, got %d", config.Port)
	}
	if config.CPURequest != "250m" {
		t.Errorf("expected cpu request '250m', got %q", config.CPURequest)
	}
	if config.MemoryLimit != "1Gi" {
		t.Errorf("expected memory limit '1Gi', got %q", config.MemoryLimit)
	}
	if config.IngressHostname != "api.example.com" {
		t.Errorf("expected hostname 'api.example.com', got %q", config.IngressHostname)
	}
	if config.IngressPath != "/api" {
		t.Errorf("expected path '/api', got %q", config.IngressPath)
	}
	if config.HealthCheckPath != "/healthz" {
		t.Errorf("expected health path '/healthz', got %q", config.HealthCheckPath)
	}
	if !config.MonitoringEnabled {
		t.Error("expected monitoring enabled")
	}
	if !config.PersistenceEnabled {
		t.Error("expected persistence enabled")
	}
	if config.PersistenceSize != "10Gi" {
		t.Errorf("expected size '10Gi', got %q", config.PersistenceSize)
	}
	if len(config.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(config.Env))
	}
	if len(config.Secrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(config.Secrets))
	}
	if config.RunAsNonRoot == nil || !*config.RunAsNonRoot {
		t.Error("expected runAsNonRoot true")
	}
	if config.RunAsUser == nil || *config.RunAsUser != 1000 {
		t.Error("expected runAsUser 1000")
	}
}

func TestBuildConfig_MissingName(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"image": map[string]interface{}{"repository": "nginx"},
			},
		},
	}
	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "spec.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingImageRepository(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}
	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing image.repository")
	}
	if !strings.Contains(err.Error(), "spec.image.repository is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WrongTypeEnvReturnsError(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
				"image": map[string]interface{}{
					"repository": "nginx",
				},
				"env": "not-a-map",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for wrong-type env")
	}
	if !strings.Contains(err.Error(), "env") {
		t.Errorf("error should mention 'env', got: %s", err.Error())
	}
}

func TestBuildConfig_WrongTypeHelmOverridesReturnsError(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
				"image": map[string]interface{}{
					"repository": "nginx",
				},
				"helmOverrides": "not-a-map",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for wrong-type helmOverrides")
	}
	if !strings.Contains(err.Error(), "helmOverrides") {
		t.Errorf("error should mention 'helmOverrides', got: %s", err.Error())
	}
}

func TestBuildSecurityContext_Defaults(t *testing.T) {
	config := &HTTPServiceConfig{}
	ctx := buildSecurityContext(config)

	if ctx["runAsNonRoot"] != false {
		t.Error("expected runAsNonRoot false by default")
	}
	if ctx["readOnlyRootFilesystem"] != false {
		t.Error("expected readOnlyRootFilesystem false by default")
	}
	if _, ok := ctx["runAsUser"]; ok {
		t.Error("runAsUser should not be set when nil")
	}
}

func TestBuildSecurityContext_AllSet(t *testing.T) {
	truev := true
	uid := int64(1000)
	gid := int64(2000)
	config := &HTTPServiceConfig{
		HTTPSecurityConfig: HTTPSecurityConfig{
			RunAsNonRoot:           &truev,
			ReadOnlyRootFilesystem: &truev,
			RunAsUser:              &uid,
			RunAsGroup:             &gid,
		},
	}
	ctx := buildSecurityContext(config)

	if ctx["runAsNonRoot"] != true {
		t.Error("expected runAsNonRoot true")
	}
	if ctx["readOnlyRootFilesystem"] != true {
		t.Error("expected readOnlyRootFilesystem true")
	}
	if ctx["runAsUser"] != int64(1000) {
		t.Errorf("expected runAsUser 1000, got %v", ctx["runAsUser"])
	}
	if ctx["runAsGroup"] != int64(2000) {
		t.Errorf("expected runAsGroup 2000, got %v", ctx["runAsGroup"])
	}
}

func TestBuildStakaterValues_MinimalConfig(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Team: "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{Port: 8080},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
	}

	values := buildStakaterValues(config)

	if values["applicationName"] != "web" {
		t.Errorf("expected applicationName 'web', got %v", values["applicationName"])
	}

	deployment, ok := values["deployment"].(map[string]interface{})
	if !ok {
		t.Fatal("expected deployment map")
	}
	if deployment["replicas"] != 1 {
		t.Errorf("expected replicas 1, got %v", deployment["replicas"])
	}

	// httpRoute disabled
	httpRoute, ok := values["httpRoute"].(map[string]interface{})
	if !ok || httpRoute["enabled"] != false {
		t.Error("expected httpRoute disabled")
	}

	// Service enabled
	svc, ok := values["service"].(map[string]interface{})
	if !ok || svc["enabled"] != true {
		t.Error("expected service enabled")
	}

	// Disabled features
	for _, key := range []string{"ingress", "route", "forecastle", "cronJob", "job"} {
		section, ok := values[key].(map[string]interface{})
		if !ok || section["enabled"] != false {
			t.Errorf("expected %s disabled", key)
		}
	}
}

func TestBuildStakaterValues_WithEnvAndSecrets(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Team: "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{Port: 8080},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
		Env:            map[string]string{"PORT": "8080"},
		EnvFromSecrets: []string{"db-creds"},
		Secrets: []ku.SecretRef{
			{OnePasswordItem: "api-key", Name: "api-secret"},
		},
	}

	values := buildStakaterValues(config)

	deployment := values["deployment"].(map[string]interface{})
	env, ok := deployment["env"].(map[string]interface{})
	if !ok {
		t.Fatal("expected env map")
	}
	portEnv, ok := env["PORT"].(map[string]interface{})
	if !ok || portEnv["value"] != "8080" {
		t.Error("expected PORT env var")
	}

	envFrom, ok := deployment["envFrom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected envFrom map")
	}
	if _, ok := envFrom["db-creds"]; !ok {
		t.Error("expected db-creds in envFrom")
	}
	if _, ok := envFrom["api-secret"]; !ok {
		t.Error("expected api-secret in envFrom")
	}
}

func TestBuildStakaterValues_WithMonitoring(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Team: "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig:    HTTPNetworkConfig{Port: 8080},
		HTTPMonitoringConfig: HTTPMonitoringConfig{
			MonitoringEnabled:  true,
			MonitoringPath:     "/metrics",
			MonitoringInterval: "30s",
		},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
	}

	values := buildStakaterValues(config)

	sm, ok := values["serviceMonitor"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serviceMonitor")
	}
	if sm["enabled"] != true {
		t.Error("expected serviceMonitor enabled")
	}
}

func TestBuildStakaterValues_WithPersistence(t *testing.T) {
	config := &HTTPServiceConfig{
		Name: "web",
		Team: "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{Port: 8080},
		HTTPStorageConfig: HTTPStorageConfig{
			PersistenceEnabled:   true,
			PersistenceSize:      "5Gi",
			PersistenceClass:     "nfs",
			PersistenceMountPath: "/data",
		},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
	}

	values := buildStakaterValues(config)

	p, ok := values["persistence"].(map[string]interface{})
	if !ok {
		t.Fatal("expected persistence")
	}
	if p["enabled"] != true {
		t.Error("expected persistence enabled")
	}
	if p["storageSize"] != "5Gi" {
		t.Errorf("expected storageSize '5Gi', got %v", p["storageSize"])
	}
	if p["storageClass"] != "nfs" {
		t.Errorf("expected storageClass 'nfs', got %v", p["storageClass"])
	}
}

func TestBuildNetworkPolicies_Minimal(t *testing.T) {
	config := &HTTPServiceConfig{
		Name:              "web",
		Namespace:         "production",
		HTTPNetworkConfig: HTTPNetworkConfig{Port: 8080},
		GatewayNS:         "nginx-gateway",
	}

	policies := buildNetworkPolicies(config)
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies (gateway + dns), got %d", len(policies))
	}

	gatewayPolicy := policies[0]
	if gatewayPolicy.Metadata.Name != "web-allow-gateway" {
		t.Errorf("expected gateway policy name, got %q", gatewayPolicy.Metadata.Name)
	}
	if gatewayPolicy.Metadata.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", gatewayPolicy.Metadata.Namespace)
	}

	dnsPolicy := policies[1]
	if dnsPolicy.Metadata.Name != "web-allow-dns" {
		t.Errorf("expected dns policy name, got %q", dnsPolicy.Metadata.Name)
	}
}

func TestBuildNetworkPolicies_WithMonitoring(t *testing.T) {
	config := &HTTPServiceConfig{
		Name:                 "web",
		Namespace:            "production",
		HTTPNetworkConfig:    HTTPNetworkConfig{Port: 8080},
		HTTPMonitoringConfig: HTTPMonitoringConfig{MonitoringEnabled: true},
		GatewayNS:            "nginx-gateway",
	}

	policies := buildNetworkPolicies(config)
	if len(policies) != 3 {
		t.Fatalf("expected 3 policies (gateway + monitoring + dns), got %d", len(policies))
	}

	monitoringPolicy := policies[1]
	if monitoringPolicy.Metadata.Name != "web-allow-monitoring" {
		t.Errorf("expected monitoring policy name, got %q", monitoringPolicy.Metadata.Name)
	}
}

func TestBuildExternalSecretRequest(t *testing.T) {
	config := &HTTPServiceConfig{
		Name:            "my-app",
		Namespace:       "production",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []ku.SecretRef{
			{
				OnePasswordItem: "my-vault-item",
				Name:            "app-secret",
				Keys: []ku.SecretKey{
					{SecretKey: "password", Property: "password"},
				},
			},
		},
	}

	esReq := buildExternalSecretRequest(config)
	if esReq.Kind != "PlatformExternalSecret" {
		t.Errorf("expected kind PlatformExternalSecret, got %q", esReq.Kind)
	}
	if esReq.Metadata.Name != "my-app-secrets" {
		t.Errorf("expected name 'my-app-secrets', got %q", esReq.Metadata.Name)
	}
	if esReq.Metadata.Namespace != "platform-requests" {
		t.Errorf("expected namespace 'platform-requests', got %q", esReq.Metadata.Namespace)
	}

	spec, ok := esReq.Spec.(ku.PlatformExternalSecretSpec)
	if !ok {
		t.Fatal("expected spec to be PlatformExternalSecretSpec")
	}
	if spec.Namespace != "production" {
		t.Errorf("expected namespace 'production' in spec, got %v", spec.Namespace)
	}
	if len(spec.Secrets) != 1 {
		t.Fatalf("expected 1 secret in spec, got %v", spec.Secrets)
	}
	if spec.Secrets[0].OnePasswordItem != "my-vault-item" {
		t.Errorf("expected onePasswordItem 'my-vault-item', got %v", spec.Secrets[0].OnePasswordItem)
	}
}

func TestBuildGatewayRouteRequest(t *testing.T) {
	config := &HTTPServiceConfig{
		Name:      "my-app",
		Namespace: "production",
		HTTPNetworkConfig: HTTPNetworkConfig{
			IngressHostname: "my-app.example.com",
			IngressPath:     "/",
			Port:            8080,
		},
		GatewayName: "nginx-gateway",
		GatewayNS:   "nginx-gateway",
	}

	gwReq := buildGatewayRouteRequest(config)
	if gwReq.Kind != "GatewayRoute" {
		t.Errorf("expected kind GatewayRoute, got %q", gwReq.Kind)
	}
	if gwReq.Metadata.Name != "my-app-route" {
		t.Errorf("expected name 'my-app-route', got %q", gwReq.Metadata.Name)
	}

	spec, ok := gwReq.Spec.(ku.GatewayRouteSpec)
	if !ok {
		t.Fatal("expected spec to be GatewayRouteSpec")
	}
	if spec.Hostname != "my-app.example.com" {
		t.Errorf("expected hostname 'my-app.example.com', got %v", spec.Hostname)
	}
	if spec.BackendRef.Port != 8080 {
		t.Errorf("expected port 8080, got %v", spec.BackendRef.Port)
	}
}

func TestHandleConfigure_MinimalConfig(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:      "web",
		Namespace: "web",
		Team:      "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{
			Port:            8080,
			IngressEnabled:  true,
			IngressHostname: "web.cluster.integratn.tech",
			IngressPath:     "/",
		},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
		GatewayName:     "nginx-gateway",
		GatewayNS:       "nginx-gateway",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		BaseDomain:      "cluster.integratn.tech",
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Namespace
	ns := ku.ReadOutput(t, dir, "resources/namespace.yaml")
	if !strings.Contains(ns, "kind: Namespace") {
		t.Error("expected Namespace kind")
	}
	if !strings.Contains(ns, "name: web") {
		t.Error("expected namespace name 'web'")
	}

	// ArgoCD application request
	app := ku.ReadOutput(t, dir, "resources/argocd-application-request.yaml")
	if !strings.Contains(app, "kind: ArgoCDApplication") {
		t.Error("expected ArgoCDApplication kind")
	}

	// Network policies
	if !ku.FileExists(dir, "resources/network-policies.yaml") {
		t.Error("expected network-policies.yaml")
	}

	// Gateway route request
	if !ku.FileExists(dir, "resources/gateway-route-request.yaml") {
		t.Error("expected gateway-route-request.yaml")
	}

	// No external-secret request (no secrets)
	if ku.FileExists(dir, "resources/external-secret-request.yaml") {
		t.Error("should not create external-secret request without secrets")
	}
}

func TestHandleConfigure_WithSecrets(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:      "web",
		Namespace: "web",
		Team:      "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "nginx",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{
			Port:            8080,
			IngressEnabled:  true,
			IngressHostname: "web.cluster.integratn.tech",
			IngressPath:     "/",
		},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
		GatewayName:     "nginx-gateway",
		GatewayNS:       "nginx-gateway",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		BaseDomain:      "cluster.integratn.tech",
		Secrets: []ku.SecretRef{
			{
				OnePasswordItem: "my-item",
				Keys: []ku.SecretKey{
					{SecretKey: "pass", Property: "password"},
				},
			},
		},
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ku.FileExists(dir, "resources/external-secret-request.yaml") {
		t.Error("expected external-secret-request.yaml")
	}
}

func TestHandleConfigure_IngressDisabled(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:      "worker",
		Namespace: "worker",
		Team:      "platform",
		HTTPImageConfig: HTTPImageConfig{
			ImageRepository: "busybox",
			ImageTag:        "latest",
			ImagePullPolicy: "IfNotPresent",
		},
		HTTPResourceConfig: HTTPResourceConfig{
			Replicas:      1,
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
		},
		HTTPNetworkConfig: HTTPNetworkConfig{
			Port:           8080,
			IngressEnabled: false,
		},
		HealthCheckPath: "/",
		HealthCheckPort: 8080,
		GatewayName:     "nginx-gateway",
		GatewayNS:       "nginx-gateway",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		BaseDomain:      "cluster.integratn.tech",
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ku.FileExists(dir, "resources/gateway-route-request.yaml") {
		t.Error("should not create gateway-route when ingress disabled")
	}
}

func TestHandleDelete_MinimalConfig(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:              "web",
		Namespace:         "web",
		HTTPNetworkConfig: HTTPNetworkConfig{IngressEnabled: true},
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ArgoCD deletion
	if !ku.FileExists(dir, "resources/delete-argocdapplication-web.yaml") {
		t.Error("expected ArgoCD delete file")
	}

	// Gateway route deletion
	if !ku.FileExists(dir, "resources/delete-gatewayroute-web.yaml") {
		t.Error("expected gateway route delete file")
	}

	// No external-secret deletion (no secrets)
	if ku.FileExists(dir, "resources/delete-externalsecret-web.yaml") {
		t.Error("should not create external-secret delete without secrets")
	}
}

func TestHandleDelete_WithSecrets(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:              "web",
		Namespace:         "web",
		HTTPNetworkConfig: HTTPNetworkConfig{IngressEnabled: true},
		Secrets: []ku.SecretRef{
			{OnePasswordItem: "item"},
		},
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ku.FileExists(dir, "resources/delete-externalsecret-web.yaml") {
		t.Error("expected external-secret delete file")
	}
}

func TestHandleDelete_IngressDisabled(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &HTTPServiceConfig{
		Name:              "worker",
		Namespace:         "worker",
		HTTPNetworkConfig: HTTPNetworkConfig{IngressEnabled: false},
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ku.FileExists(dir, "resources/delete-gatewayroute-worker.yaml") {
		t.Error("should not create gateway-route delete when ingress disabled")
	}
}

// setNestedValue sets a value at a dot-separated path inside a nested map,
// creating intermediate maps as needed.
func setNestedValue(m map[string]interface{}, path string, value interface{}) {
	keys := strings.Split(path, ".")
	cur := m
	for _, k := range keys[:len(keys)-1] {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[k] = next
		}
		cur = next
	}
	cur[keys[len(keys)-1]] = value
}

// validBaseData returns the minimal valid resource data for http-service tests.
func validBaseData() map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"name": "my-app",
			"image": map[string]interface{}{
				"repository": "nginx",
			},
		},
	}
}

func TestBuildConfig_WrongTypeOptionalFields(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		value     interface{}
		wantSubstr string
	}{
		{
			name:       "command is not a slice",
			path:       "spec.command",
			value:      42,
			wantSubstr: "command",
		},
		{
			name:       "args is not a slice",
			path:       "spec.args",
			value:      true,
			wantSubstr: "args",
		},
		{
			name:       "ingress.hostname is not a string",
			path:       "spec.ingress.hostname",
			value:      123,
			wantSubstr: "hostname",
		},
		{
			name:       "secrets is not a slice",
			path:       "spec.secrets",
			value:      "not-a-slice",
			wantSubstr: "secrets",
		},
		{
			name:       "envFromSecrets is not a slice",
			path:       "spec.envFromSecrets",
			value:      99,
			wantSubstr: "envFromSecrets",
		},
		{
			name:       "persistence.storageClass is not a string",
			path:       "spec.persistence.storageClass",
			value:      []interface{}{"nfs"},
			wantSubstr: "storageClass",
		},
		{
			name:       "securityContext.runAsNonRoot is not a bool",
			path:       "spec.securityContext.runAsNonRoot",
			value:      "yes",
			wantSubstr: "runAsNonRoot",
		},
		{
			name:       "securityContext.readOnlyRootFilesystem is not a bool",
			path:       "spec.securityContext.readOnlyRootFilesystem",
			value:      "true",
			wantSubstr: "readOnlyRootFilesystem",
		},
		{
			name:       "securityContext.runAsUser is not numeric",
			path:       "spec.securityContext.runAsUser",
			value:      "nobody",
			wantSubstr: "runAsUser",
		},
		{
			name:       "securityContext.runAsGroup is not numeric",
			path:       "spec.securityContext.runAsGroup",
			value:      false,
			wantSubstr: "runAsGroup",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := validBaseData()
			setNestedValue(data, tc.path, tc.value)

			resource := &ku.MockResource{Data: data}
			_, err := buildConfig(nil, resource)
			if err == nil {
				t.Fatalf("expected error for %s with wrong type", tc.path)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("expected error to contain %q, got: %s", tc.wantSubstr, err.Error())
			}
		})
	}
}
