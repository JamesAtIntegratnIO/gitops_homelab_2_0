// Package kratixutil provides shared types, helpers, and writers for Kratix
// promise pipelines. It eliminates code duplication across promise workflows
// by extracting common Kubernetes resource types, value-extraction helpers,
// YAML output writers, and resource-construction utilities.
//
// Type definitions are organized into domain-specific files:
//   - types_core.go: Resource, ObjectMeta, RoleRef, Subject
//   - types_argocd.go: ArgoCD application, project, cluster registration specs
//   - types_externalsecret.go: ExternalSecret CRD types
//   - types_workload.go: SecretRef, SecretKey, Job, Pod, Container types
//   - types_specs.go: Platform-level sub-ResourceRequest specs
package kratixutil
