package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// HTTPServiceConfig holds the fully resolved config from the CR.
type HTTPServiceConfig struct {
	Name      string
	Namespace string
	Team      string

	HTTPImageConfig      // ImageRepository, ImageTag, ImagePullPolicy, Command, Args
	HTTPResourceConfig   // Replicas, CPURequest, MemoryRequest, CPULimit, MemoryLimit
	HTTPNetworkConfig    // Port, IngressEnabled, IngressHostname, IngressPath
	HTTPStorageConfig    // PersistenceEnabled, PersistenceSize, PersistenceClass, PersistenceMountPath
	HTTPSecurityConfig   // RunAsNonRoot, ReadOnlyRootFilesystem, RunAsUser, RunAsGroup
	HTTPMonitoringConfig // MonitoringEnabled, MonitoringPath, MonitoringInterval

	// Secrets
	Secrets []ku.SecretRef

	// Environment
	Env            map[string]string
	EnvFromSecrets []string

	// Health checks
	HealthCheckPath string
	HealthCheckPort int

	// Escape hatch
	HelmOverrides map[string]interface{}

	// Platform defaults
	BaseDomain      string
	GatewayName     string
	GatewayNS       string
	SecretStoreName string
	SecretStoreKind string
}

// HTTPImageConfig groups container image settings.
type HTTPImageConfig struct {
	ImageRepository string
	ImageTag        string
	ImagePullPolicy string
	Command         []string
	Args            []string
}

// HTTPResourceConfig groups compute resource settings.
type HTTPResourceConfig struct {
	Replicas      int
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string
}

// HTTPNetworkConfig groups networking and ingress settings.
type HTTPNetworkConfig struct {
	Port            int
	IngressEnabled  bool
	IngressHostname string
	IngressPath     string
}

// HTTPStorageConfig groups persistence settings.
type HTTPStorageConfig struct {
	PersistenceEnabled   bool
	PersistenceSize      string
	PersistenceClass     string
	PersistenceMountPath string
}

// HTTPSecurityConfig groups security context settings.
type HTTPSecurityConfig struct {
	RunAsNonRoot           *bool
	ReadOnlyRootFilesystem *bool
	RunAsUser              *int64
	RunAsGroup             *int64
}

// HTTPMonitoringConfig groups monitoring/metrics settings.
type HTTPMonitoringConfig struct {
	MonitoringEnabled  bool
	MonitoringPath     string
	MonitoringInterval string
}
