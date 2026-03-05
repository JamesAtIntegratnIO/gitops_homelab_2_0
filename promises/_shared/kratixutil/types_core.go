// Package kratixutil provides shared types, helpers, and writers for Kratix
// promise pipelines. It eliminates code duplication across promise workflows
// by extracting common Kubernetes resource types, value-extraction helpers,
// YAML output writers, and resource-construction utilities.
package kratixutil

// Resource is a generic Kubernetes resource wrapper used to emit YAML to the
// Kratix state store. The Spec and Data fields are interface{} to support
// different resource kinds (Deployments, ExternalSecrets, ConfigMaps, etc.)
// without requiring a separate struct per kind.
//
// Spec typing convention:
//   - Use typed spec structs (e.g., ArgoCDApplicationSpec, CertificateSpec) for
//     resources whose fields are computed or branched on by pipeline logic.
//   - Use map[string]interface{} for resources that mirror upstream Kubernetes
//     specs (e.g., NetworkPolicy, ConfigMap data) where the pipeline constructs
//     the spec wholesale without inspecting individual fields.
//   - RBAC resources use the dedicated Rules/RoleRef/Subjects fields rather than
//     Spec, matching the Kubernetes role API shape directly.
type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{}  `json:"spec,omitempty"`
	Data       interface{}  `json:"data,omitempty"`
	Rules      []PolicyRule `json:"rules,omitempty"`
	RoleRef    *RoleRef     `json:"roleRef,omitempty"`
	Subjects   []Subject   `json:"subjects,omitempty"`
}

// RoleRef identifies the Role or ClusterRole to bind in a RoleBinding.
type RoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// Subject identifies a user, group, or service account in a role binding.
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ObjectMeta contains common metadata fields shared by all Kubernetes resources.
type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
}
