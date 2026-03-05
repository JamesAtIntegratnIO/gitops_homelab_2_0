package kratixutil

// ============================================================================
// ExternalSecret Types
// ============================================================================

// ExternalSecretSpec configures an ExternalSecret resource that syncs secrets
// from an external store (1Password) into a Kubernetes Secret.
type ExternalSecretSpec struct {
	SecretStoreRef  SecretStoreRef           `json:"secretStoreRef"`
	Target          ExternalSecretTarget     `json:"target"`
	Data            []ExternalSecretData     `json:"data,omitempty"`
	DataFrom        []ExternalSecretDataFrom `json:"dataFrom,omitempty"`
	RefreshInterval string                   `json:"refreshInterval,omitempty"`
}

// SecretStoreRef points to a ClusterSecretStore or SecretStore to use
// when fetching secret data.
type SecretStoreRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// ExternalSecretTarget configures the Kubernetes Secret that the ExternalSecret
// operator creates or updates.
type ExternalSecretTarget struct {
	Name     string                  `json:"name,omitempty"`
	Template *ExternalSecretTemplate `json:"template,omitempty"`
}

// ExternalSecretTemplate allows templating the target Secret's metadata
// and data fields using Go templates over the fetched values.
type ExternalSecretTemplate struct {
	EngineVersion string            `json:"engineVersion,omitempty"`
	Type          string            `json:"type,omitempty"`
	Metadata      *TemplateMetadata `json:"metadata,omitempty"`
	Data          map[string]string `json:"data,omitempty"`
}

// TemplateMetadata sets labels and annotations on the templated Secret.
type TemplateMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ExternalSecretData maps a single Kubernetes Secret key to a field in an
// external secret provider.
type ExternalSecretData struct {
	SecretKey string    `json:"secretKey"`
	RemoteRef RemoteRef `json:"remoteRef"`
}

// RemoteRef identifies a key (and optional property) in the external secret
// provider.
type RemoteRef struct {
	Key      string `json:"key"`
	Property string `json:"property,omitempty"`
}

// ExternalSecretDataFrom extracts all fields from a single external secret
// item.
type ExternalSecretDataFrom struct {
	Extract *ExternalSecretExtract `json:"extract"`
}

// ExternalSecretExtract configures how to extract data from an external
// secret item, including encoding and decoding strategies.
type ExternalSecretExtract struct {
	Key                string `json:"key"`
	ConversionStrategy string `json:"conversionStrategy,omitempty"`
	DecodingStrategy   string `json:"decodingStrategy,omitempty"`
}
