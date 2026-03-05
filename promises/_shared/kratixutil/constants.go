package kratixutil

// Platform-wide default constants shared across all promise pipelines.
const (
	// DefaultSecretStoreName is the name of the platform's default
	// ClusterSecretStore (backed by 1Password Connect).
	DefaultSecretStoreName = "onepassword-store"

	// DefaultSecretStoreKind is the ExternalSecrets store kind used by the
	// platform. All promises should reference a ClusterSecretStore unless
	// there is a namespace-scoped override.
	DefaultSecretStoreKind = "ClusterSecretStore"

	// DefaultGatewayName is the name of the platform's default
	// nginx-gateway-fabric Gateway resource.
	DefaultGatewayName = "nginx-gateway"

	// DefaultGatewayNamespace is the namespace where the default Gateway
	// resource is deployed.
	DefaultGatewayNamespace = "nginx-gateway"

	// DefaultArgoCDNamespace is the namespace where ArgoCD is deployed.
	DefaultArgoCDNamespace = "argocd"

	// DefaultPlatformRequestsNamespace is the namespace used for Kratix
	// sub-ResourceRequests between promises.
	DefaultPlatformRequestsNamespace = "platform-requests"

	// PlatformRepoURL is the canonical HTTPS URL of the platform GitOps
	// repository, used as the default ArgoCD source for workloads.
	PlatformRepoURL = "https://github.com/jamesatintegratnio/gitops_homelab_2_0"

	// PlatformRepoGitURL is the .git variant used for ArgoCD annotations.
	PlatformRepoGitURL = PlatformRepoURL + ".git"

	// OnePasswordConnectCIDR is the IP of the 1Password Connect server
	// used in network policy egress rules.
	OnePasswordConnectCIDR = "10.0.1.139/32"

	// TalosNodeLocalDNSCIDR is the Talos node-local DNS link-local address.
	// Cilium classifies this as 'world', not 'host'.
	TalosNodeLocalDNSCIDR = "169.254.116.108/32"

	// DefaultMetalLBPoolOffset is the IP offset within a subnet for calculating
	// default MetalLB VIP addresses (e.g., 10.0.4.0/24 + 200 = 10.0.4.200).
	DefaultMetalLBPoolOffset = 200

	// DefaultEtcdReplicas is the number of etcd replicas used for DNS name
	// generation in external-etcd backed vclusters.
	DefaultEtcdReplicas = 3

	// DefaultKubectlImage is the pinned bitnami/kubectl image used for
	// pipeline Jobs that need kubectl access.
	DefaultKubectlImage = "bitnami/kubectl:1.31"
)
