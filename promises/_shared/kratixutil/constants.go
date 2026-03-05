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
)
