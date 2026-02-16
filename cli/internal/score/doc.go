// Package score provides Score (score.dev/v1b1) workload specification parsing
// and translation to platform-native resources.
//
// This is a custom Score implementation for the integratn.tech platform.
// It uses score-go for parsing and validation, and custom provisioners
// to translate Score resource types into platform resources:
//
//   - postgres → ExternalSecret (credentials from 1Password)
//   - redis → ExternalSecret
//   - route → HTTPRoute (Gateway API via nginx-gateway-fabric)
//   - volume → PVC with NFS StorageClass
//   - dns → DNS record configuration
//
// Output is a Stakater Application Helm chart values.yaml plus supporting
// resources (ExternalSecrets, HTTPRoutes, NetworkPolicies).
package score
