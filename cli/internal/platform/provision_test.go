package platform

import (
	"strings"
	"testing"
)

func TestFormatProvisionSummary_Healthy(t *testing.T) {
	result := &ProvisionResult{
		Name:    "media",
		Phase:   "Ready",
		Healthy: true,
		Endpoints: ProvisionEndpoints{
			API:    "https://media.cluster.integratn.tech",
			ArgoCD: "https://argocd-media.cluster.integratn.tech",
		},
		Health: ProvisionHealth{
			ComponentsReady: 3,
			ComponentsTotal: 3,
			SubAppsHealthy:  5,
			SubAppsTotal:    5,
		},
	}

	output := FormatProvisionSummary(result, "")
	if !strings.Contains(output, "media") {
		t.Error("output should contain cluster name")
	}
	if !strings.Contains(output, "ready") {
		t.Error("output should indicate readiness")
	}
	if !strings.Contains(output, "3/3") {
		t.Error("output should show component count 3/3")
	}
	if !strings.Contains(output, "5/5") {
		t.Error("output should show sub-app count 5/5")
	}
	if !strings.Contains(output, "hctl vcluster connect media") {
		t.Error("output should include connect next step")
	}
	if !strings.Contains(output, "hctl vcluster status media") {
		t.Error("output should include status next step")
	}
}

func TestFormatProvisionSummary_Unhealthy(t *testing.T) {
	result := &ProvisionResult{
		Name:    "dev",
		Phase:   "Progressing",
		Healthy: false,
		Health: ProvisionHealth{
			ComponentsReady: 1,
			ComponentsTotal: 3,
			SubAppsHealthy:  2,
			SubAppsTotal:    4,
			Unhealthy:       []string{"cert-manager", "external-secrets"},
		},
	}

	output := FormatProvisionSummary(result, "dev.example.com")
	if !strings.Contains(output, "provisioning") {
		t.Error("unhealthy output should mention provisioning")
	}
	if !strings.Contains(output, "1/3") {
		t.Error("output should show component count 1/3")
	}
	if !strings.Contains(output, "2/4") {
		t.Error("output should show sub-app count 2/4")
	}
	if !strings.Contains(output, "cert-manager") {
		t.Error("output should list unhealthy items")
	}
	if !strings.Contains(output, "external-secrets") {
		t.Error("output should list all unhealthy items")
	}
	// hostname fallback
	if !strings.Contains(output, "dev.example.com") {
		t.Error("output should include hostname when API endpoint is empty")
	}
}

func TestFormatProvisionSummary_NoEndpoints(t *testing.T) {
	result := &ProvisionResult{
		Name:    "test",
		Healthy: true,
	}

	output := FormatProvisionSummary(result, "")
	// Should not contain "Access" section when no endpoints
	if !strings.Contains(output, "Next Steps") {
		t.Error("output should always contain Next Steps")
	}
	if !strings.Contains(output, "hctl vcluster connect test") {
		t.Error("output should include connect command for 'test'")
	}
}

func TestFormatProvisionSummary_ZeroComponents(t *testing.T) {
	result := &ProvisionResult{
		Name:    "fresh",
		Healthy: false,
		Health: ProvisionHealth{
			ComponentsReady: 0,
			ComponentsTotal: 0,
		},
	}

	output := FormatProvisionSummary(result, "")
	// With 0 total components, health section should be omitted
	if strings.Contains(output, "0/0") {
		t.Error("output should not show 0/0 component count")
	}
}

func TestFormatProvisionSummary_EndpointFallback(t *testing.T) {
	result := &ProvisionResult{
		Name:    "my-vc",
		Healthy: true,
		Endpoints: ProvisionEndpoints{
			API: "https://my-vc.cluster.integratn.tech",
		},
	}

	output := FormatProvisionSummary(result, "fallback.example.com")
	// When API endpoint exists, it should be preferred over hostname
	if !strings.Contains(output, "https://my-vc.cluster.integratn.tech") {
		t.Error("output should show API endpoint")
	}
}

func TestProvisionResultTypes(t *testing.T) {
	// Ensure the struct can be instantiated with all fields
	r := ProvisionResult{
		Name:  "test",
		Phase: "Ready",
		Health: ProvisionHealth{
			Unhealthy: []string{"a", "b"},
		},
	}
	if r.Name != "test" {
		t.Error("Name should be 'test'")
	}
	if len(r.Health.Unhealthy) != 2 {
		t.Error("Unhealthy should have 2 items")
	}
}
