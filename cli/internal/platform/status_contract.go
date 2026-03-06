package platform

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StatusContract represents the status contract from a VClusterOrchestratorV2 resource.
type StatusContract struct {
	Phase          string
	Message        string
	LastReconciled string

	Endpoints   StatusEndpoints
	Credentials StatusCredentials
	Health      StatusHealth
	Conditions  []StatusCondition
}

// StatusEndpoints holds discoverable URLs.
type StatusEndpoints struct {
	API    string
	ArgoCD string
}

// StatusCredentials holds credential references.
type StatusCredentials struct {
	KubeconfigSecret string
	OnePasswordItem  string
}

// StatusHealth aggregates health data.
type StatusHealth struct {
	ArgoCDSync   string
	ArgoCDHealth string
	PodsReady    int64
	PodsTotal    int64
	SubAppsHealthy int64
	SubAppsTotal   int64
	SubAppsUnhealthy []string
}

// StatusCondition represents a Kubernetes-style condition.
type StatusCondition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime string
}

// GetStatusContract reads the .status contract from a VClusterOrchestratorV2 resource.
func GetStatusContract(ctx context.Context, client KubeClient, namespace, name string) (*StatusContract, error) {
	vc, err := client.GetVCluster(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	return parseStatusContract(vc)
}

func parseStatusContract(vc *unstructured.Unstructured) (*StatusContract, error) {
	sc := &StatusContract{}

	// Top-level fields
	sc.Phase, _, _ = unstructured.NestedString(vc.Object, "status", "phase")
	sc.Message, _, _ = unstructured.NestedString(vc.Object, "status", "message")
	sc.LastReconciled, _, _ = unstructured.NestedString(vc.Object, "status", "lastReconciled")

	// Endpoints
	if endpoints, found, _ := unstructured.NestedStringMap(vc.Object, "status", "endpoints"); found {
		sc.Endpoints.API = endpoints["api"]
		sc.Endpoints.ArgoCD = endpoints["argocd"]
	}

	// Credentials
	if creds, found, _ := unstructured.NestedStringMap(vc.Object, "status", "credentials"); found {
		sc.Credentials.KubeconfigSecret = creds["kubeconfigSecret"]
		sc.Credentials.OnePasswordItem = creds["onePasswordItem"]
	}

	// Health
	sc.Health.ArgoCDSync, _, _ = unstructured.NestedString(vc.Object, "status", "health", "argocd", "syncStatus")
	sc.Health.ArgoCDHealth, _, _ = unstructured.NestedString(vc.Object, "status", "health", "argocd", "healthStatus")
	sc.Health.PodsReady, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "ready")
	sc.Health.PodsTotal, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "total")
	sc.Health.SubAppsHealthy, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "subApps", "healthy")
	sc.Health.SubAppsTotal, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "subApps", "total")

	if unhealthy, found, _ := unstructured.NestedStringSlice(vc.Object, "status", "health", "subApps", "unhealthy"); found {
		sc.Health.SubAppsUnhealthy = unhealthy
	}

	// Conditions
	if condSlice, found, _ := unstructured.NestedSlice(vc.Object, "status", "conditions"); found {
		for _, c := range condSlice {
			if condMap, ok := c.(map[string]interface{}); ok {
				cond := StatusCondition{}
				if v, ok := condMap["type"].(string); ok {
					cond.Type = v
				}
				if v, ok := condMap["status"].(string); ok {
					cond.Status = v
				}
				if v, ok := condMap["reason"].(string); ok {
					cond.Reason = v
				}
				if v, ok := condMap["message"].(string); ok {
					cond.Message = v
				}
				if v, ok := condMap["lastTransitionTime"].(string); ok {
					cond.LastTransitionTime = v
				}
				sc.Conditions = append(sc.Conditions, cond)
			}
		}
	}

	return sc, nil
}
