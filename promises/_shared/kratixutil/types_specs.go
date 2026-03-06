package kratixutil

// GatewayRouteSpec is the spec for a platform.integratn.tech/v1alpha1
// GatewayRoute sub-ResourceRequest. The gateway-route promise pipeline reads
// these fields to construct the actual Gateway API HTTPRoute.
type GatewayRouteSpec struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Hostname     string            `json:"hostname"`
	Path         string            `json:"path"`
	BackendRef   GatewayBackendRef `json:"backendRef"`
	Gateway      GatewayRef        `json:"gateway"`
	HTTPRedirect bool              `json:"httpRedirect"`
	OwnerPromise string            `json:"ownerPromise"`
}

// GatewayBackendRef identifies the backend Service for a GatewayRoute.
type GatewayBackendRef struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// GatewayRef identifies the Gateway resource a route attaches to.
type GatewayRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// PlatformExternalSecretSpec is the spec for a platform.integratn.tech/v1alpha1
// PlatformExternalSecret sub-ResourceRequest. The external-secret promise
// pipeline reads these fields to construct ExternalSecret resources.
type PlatformExternalSecretSpec struct {
	Namespace       string                       `json:"namespace"`
	AppName         string                       `json:"appName"`
	SecretStoreName string                       `json:"secretStoreName"`
	SecretStoreKind string                       `json:"secretStoreKind"`
	OwnerPromise    string                       `json:"ownerPromise"`
	Secrets         []PlatformExternalSecretItem `json:"secrets"`
}

// PlatformExternalSecretItem is an alias for SecretRef, used within
// PlatformExternalSecret sub-requests. The types are identical.
type PlatformExternalSecretItem = SecretRef
