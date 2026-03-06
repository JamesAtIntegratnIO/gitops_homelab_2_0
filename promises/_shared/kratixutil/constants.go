package kratixutil

// Status phases used in promise status reporting.
const (
	PhaseConfigured        = "Configured"
	PhaseDeleting          = "Deleting"
	PhaseScheduled         = "Scheduled"
	PhaseConfigurationError = "ConfigurationError"
)

// Platform-wide default constants shared across all promise pipelines.
const (
	// Used by all ExternalSecret resources to reference the 1Password ClusterSecretStore.
	DefaultSecretStoreName = "onepassword-store"

	// All promises reference a ClusterSecretStore unless there is a
	// namespace-scoped override.
	DefaultSecretStoreKind = "ClusterSecretStore"

	// Must match the deployed nginx-gateway-fabric Gateway resource name.
	// GatewayRoute and HTTP service promises reference this for parent routing.
	DefaultGatewayName = "nginx-gateway"

	// Co-located with the nginx-gateway-fabric controller.
	DefaultGatewayNamespace = "nginx-gateway"

	DefaultArgoCDNamespace           = "argocd"
	DefaultPlatformRequestsNamespace = "platform-requests"

	// Well-known namespace names used in network policies, RBAC, and selectors.
	MonitoringNamespace = "monitoring"
	KubeSystemNamespace = "kube-system"

	// DefaultBaseDomain is the platform top-level base domain.
	// Used by vcluster and cluster-registration promises for hostnames and
	// DNS annotations.
	DefaultBaseDomain = "integratn.tech"

	// DefaultClusterBaseDomain is the cluster-specific sub-domain used by
	// workload-level promises (http-service, gateway-route) for ingress hostnames.
	DefaultClusterBaseDomain = "cluster.integratn.tech"

	// Default cert-manager issuer label selector applied when no explicit
	// selector is provided in the resource request.
	DefaultCertManagerIssuerLabel = "integratn.tech/cluster-issuer"
	DefaultCertManagerIssuer      = "letsencrypt-prod"

	// Default external-secrets store label selector applied when no explicit
	// selector is provided in the resource request.
	DefaultExternalSecretsStoreLabel = "integratn.tech/cluster-secret-store"
	DefaultExternalSecretsStore      = "onepassword-store"

	// Canonical HTTPS URL of the platform GitOps repository, used as the
	// default ArgoCD source for workloads.
	PlatformRepoURL = "https://github.com/jamesatintegratnio/gitops_homelab_2_0"

	// .git variant required by ArgoCD annotations.
	PlatformRepoGitURL = PlatformRepoURL + ".git"

	// Static IP of the 1Password Connect server. Referenced in network
	// policy egress rules so vcluster namespaces can reach it.
	OnePasswordConnectCIDR = "10.0.1.139/32"

	// Talos node-local DNS link-local address. Cilium classifies this as
	// 'world', not 'host', so it needs an explicit CiliumNetworkPolicy.
	TalosNodeLocalDNSCIDR = "169.254.116.108/32"

	// Offset within a subnet CIDR to derive the first MetalLB VIP address.
	// Example: 10.0.4.0/24 + 200 → 10.0.4.200.
	DefaultMetalLBPoolOffset = 200

	// Controls etcd DNS name generation for external-etcd backed vclusters.
	DefaultEtcdReplicas = 3

	// Pinned to 1.31 for consistency across all promise pipeline jobs.
	// Update when upgrading the host cluster Kubernetes version.
	DefaultKubectlImage = "bitnami/kubectl:1.31"

	// Common well-known ports used in network policies.
	DNSPort   = 53
	HTTPPort  = 80
	HTTPSPort = 443
	NFSPort   = 2049

	// RFC 1918 private network CIDRs.
	RFC1918Class10  = "10.0.0.0/8"
	RFC1918Class172 = "172.16.0.0/12"
	RFC1918Class192 = "192.168.0.0/16"

	// AllIPv4 matches all IPv4 addresses.
	AllIPv4 = "0.0.0.0/0"
)
