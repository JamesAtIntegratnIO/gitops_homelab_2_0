package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// HTTPServiceConfig holds the fully resolved config from the CR.
type HTTPServiceConfig struct {
	Name      string
	Namespace string
	Team      string

	// Image
	ImageRepository string
	ImageTag        string
	ImagePullPolicy string
	Command         []string
	Args            []string

	// Scaling
	Replicas      int
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string

	// Networking
	Port            int
	IngressEnabled  bool
	IngressHostname string
	IngressPath     string

	// Secrets
	Secrets []ku.SecretRef

	// Environment
	Env            map[string]string
	EnvFromSecrets []string

	// Health checks
	HealthCheckPath string
	HealthCheckPort int

	// Monitoring
	MonitoringEnabled  bool
	MonitoringPath     string
	MonitoringInterval string

	// Storage
	PersistenceEnabled   bool
	PersistenceSize      string
	PersistenceClass     string
	PersistenceMountPath string

	// Security
	RunAsNonRoot           *bool
	ReadOnlyRootFilesystem *bool
	RunAsUser              *int64
	RunAsGroup             *int64

	// Escape hatch
	HelmOverrides map[string]interface{}

	// Platform defaults
	BaseDomain      string
	GatewayName     string
	GatewayNS       string
	SecretStoreName string
	SecretStoreKind string
}
