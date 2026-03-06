package kratixutil

// ExternalSecretSpec configures an ExternalSecret resource that syncs secrets
// from an external store (1Password) into a Kubernetes Secret.
type ExternalSecretSpec struct {
	SecretStoreRef  SecretStoreRef           `json:"secretStoreRef"`
	Target          ExternalSecretTarget     `json:"target"`
	Data            []ExternalSecretData     `json:"data,omitempty"`
	DataFrom        []ExternalSecretDataFrom `json:"dataFrom,omitempty"`
	RefreshInterval string                   `json:"refreshInterval,omitempty"`
}

type SecretStoreRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

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

type TemplateMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ExternalSecretData struct {
	SecretKey string    `json:"secretKey"`
	RemoteRef RemoteRef `json:"remoteRef"`
}

type RemoteRef struct {
	Key      string `json:"key"`
	Property string `json:"property,omitempty"`
}

type ExternalSecretDataFrom struct {
	Extract *ExternalSecretExtract `json:"extract"`
}

type ExternalSecretExtract struct {
	Key                string `json:"key"`
	ConversionStrategy string `json:"conversionStrategy,omitempty"`
	DecodingStrategy   string `json:"decodingStrategy,omitempty"`
}
