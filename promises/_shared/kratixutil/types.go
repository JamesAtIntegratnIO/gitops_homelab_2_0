// Package kratixutil provides shared types, helpers, and writers for Kratix
// promise pipelines. It eliminates code duplication across promise workflows
// by extracting common Kubernetes resource types, value-extraction helpers,
// YAML output writers, and resource-construction utilities.
package kratixutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================================
// Core Kubernetes Types
// ============================================================================

// Resource is a generic Kubernetes resource suitable for any API object.
type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{} `json:"spec,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Rules      interface{} `json:"rules,omitempty"`
	RoleRef    *RoleRef    `json:"roleRef,omitempty"`
	Subjects   []Subject   `json:"subjects,omitempty"`
}

// RoleRef references a Role or ClusterRole for a RoleBinding.
type RoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// Subject identifies the entity bound by a RoleBinding.
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ObjectMeta is a lightweight Kubernetes metadata block.
type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
}

// ============================================================================
// ArgoCD Kratix ResourceRequest Types
// ============================================================================

// ArgoCDApplicationSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDApplication sub-ResourceRequest. The argocd-application promise
// pipeline reads these fields to construct the actual argoproj.io/v1alpha1
// Application.
type ArgoCDApplicationSpec struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
	Project     string            `json:"project"`
	Source      AppSource         `json:"source"`
	Destination Destination       `json:"destination"`
	SyncPolicy  interface{}       `json:"syncPolicy,omitempty"`
}

// AppSource defines the Helm chart or git source for an ArgoCD Application.
type AppSource struct {
	RepoURL        string      `json:"repoURL"`
	Chart          string      `json:"chart,omitempty"`
	TargetRevision string      `json:"targetRevision"`
	Helm           *HelmSource `json:"helm,omitempty"`
}

// HelmSource holds Helm-specific source config.
type HelmSource struct {
	ReleaseName  string      `json:"releaseName,omitempty"`
	ValuesObject interface{} `json:"valuesObject,omitempty"`
}

// Destination is the ArgoCD deployment target.
type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// SyncPolicy for ArgoCD applications.
type SyncPolicy struct {
	Automated   *AutomatedSync `json:"automated,omitempty"`
	SyncOptions []string       `json:"syncOptions,omitempty"`
}

// AutomatedSync configures automatic syncing for ArgoCD.
type AutomatedSync struct {
	SelfHeal bool `json:"selfHeal"`
	Prune    bool `json:"prune"`
}

// ArgoCDProjectSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDProject sub-ResourceRequest.
type ArgoCDProjectSpec struct {
	Namespace                  string               `json:"namespace"`
	Name                       string               `json:"name"`
	Description                string               `json:"description"`
	Annotations                map[string]string     `json:"annotations,omitempty"`
	Labels                     map[string]string     `json:"labels,omitempty"`
	SourceRepos                []string              `json:"sourceRepos"`
	Destinations               []ProjectDestination  `json:"destinations"`
	ClusterResourceWhitelist   []ResourceFilter      `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []ResourceFilter      `json:"namespaceResourceWhitelist,omitempty"`
}

// ProjectDestination defines an ArgoCD project destination.
type ProjectDestination struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

// ResourceFilter matches a Kubernetes resource by API group and kind.
type ResourceFilter struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

// ArgoCDClusterRegistrationSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDClusterRegistration sub-ResourceRequest.
type ArgoCDClusterRegistrationSpec struct {
	Name                string            `json:"name"`
	TargetNamespace     string            `json:"targetNamespace"`
	KubeconfigSecret    string            `json:"kubeconfigSecret"`
	ExternalServerURL   string            `json:"externalServerURL"`
	Environment         string            `json:"environment,omitempty"`
	BaseDomain          string            `json:"baseDomain,omitempty"`
	BaseDomainSanitized string            `json:"baseDomainSanitized,omitempty"`
	ClusterLabels       map[string]string `json:"clusterLabels,omitempty"`
	ClusterAnnotations  map[string]string `json:"clusterAnnotations,omitempty"`
	SyncJobName         string            `json:"syncJobName,omitempty"`
}

// ============================================================================
// Secret Types (1Password / ExternalSecrets)
// ============================================================================

// SecretRef describes a 1Password-backed ExternalSecret.
type SecretRef struct {
	Name            string      `json:"name"`
	OnePasswordItem string      `json:"onePasswordItem"`
	Keys            []SecretKey `json:"keys"`
}

// SecretKey maps a 1Password property to a Kubernetes Secret key.
type SecretKey struct {
	SecretKey string `json:"secretKey"`
	Property  string `json:"property"`
}

// ============================================================================
// Type Conversion Utilities
// ============================================================================

// ToMap converts a struct to map[string]interface{} via JSON roundtrip.
// Useful at the merge boundary where typed structs meet DeepMerge.
func ToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("toMap marshal: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("toMap unmarshal: %w", err)
	}
	return m, nil
}

// DeleteFromResource creates a minimal delete resource (only apiVersion, kind,
// name, namespace) from a fully-populated Resource.
func DeleteFromResource(r Resource) Resource {
	return Resource{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Metadata: ObjectMeta{
			Name:      r.Metadata.Name,
			Namespace: r.Metadata.Namespace,
		},
	}
}

// DeleteOutputPathForResource computes the output file path for a delete
// resource in the standard "resources/delete-{kind}-{name}.yaml" pattern.
func DeleteOutputPathForResource(prefix string, r Resource) string {
	if prefix == "" {
		prefix = "resources/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sdelete-%s-%s.yaml", prefix, strings.ToLower(r.Kind), r.Metadata.Name)
}
