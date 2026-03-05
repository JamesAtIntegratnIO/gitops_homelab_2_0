package platform

import (
	"testing"
)

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
