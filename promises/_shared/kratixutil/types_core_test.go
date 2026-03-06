package kratixutil

import (
	"fmt"
	"testing"
)

type validSpec struct{}

func (v *validSpec) Validate() error { return nil }

type invalidSpec struct{}

func (i *invalidSpec) Validate() error { return fmt.Errorf("spec validation failed") }

func TestResource_Validate_ValidatableSpecValid(t *testing.T) {
	r := &Resource{
		APIVersion: "v1",
		Kind:       "Test",
		Spec:       &validSpec{},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestResource_Validate_ValidatableSpecInvalid(t *testing.T) {
	r := &Resource{
		APIVersion: "v1",
		Kind:       "Test",
		Spec:       &invalidSpec{},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error from invalid spec")
	}
	if err.Error() != "spec validation failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResource_Validate_UntypedMapSpecSkipped(t *testing.T) {
	r := &Resource{
		APIVersion: "v1",
		Kind:       "Test",
		Spec:       map[string]interface{}{"key": "value"},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected nil error for untyped spec, got: %v", err)
	}
}

func TestResource_Validate_NilSpec(t *testing.T) {
	r := &Resource{
		APIVersion: "v1",
		Kind:       "Test",
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected nil error for nil spec, got: %v", err)
	}
}

func TestResource_Validate_ArgoCDAppSpec(t *testing.T) {
	spec := &ArgoCDApplicationSpec{
		Name:        "test-app",
		Source:      AppSource{RepoURL: "https://example.com"},
		Destination: Destination{Server: "https://kubernetes.default.svc"},
	}
	r := &Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Spec:       spec,
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestResource_Validate_ArgoCDAppSpec_Invalid(t *testing.T) {
	spec := &ArgoCDApplicationSpec{} // missing required fields
	r := &Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Spec:       spec,
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for invalid ArgoCDApplicationSpec")
	}
}
