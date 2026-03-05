package kratixutil

import (
	"testing"
)

// ============================================================================
// ToMap
// ============================================================================

func TestToMap_Struct(t *testing.T) {
	r := Resource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	m, err := ToMap(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["apiVersion"] != "v1" {
		t.Errorf("expected apiVersion 'v1', got %v", m["apiVersion"])
	}
	if m["kind"] != "ConfigMap" {
		t.Errorf("expected kind 'ConfigMap', got %v", m["kind"])
	}
	meta, ok := m["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata to be a map")
	}
	if meta["name"] != "test" {
		t.Errorf("expected name 'test', got %v", meta["name"])
	}
}

func TestToMap_OmitsEmptyFields(t *testing.T) {
	r := Resource{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   ObjectMeta{Name: "test"},
	}
	m, err := ToMap(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// spec, data, rules should be omitted (omitempty)
	if _, exists := m["spec"]; exists {
		t.Error("expected spec to be omitted")
	}
	if _, exists := m["data"]; exists {
		t.Error("expected data to be omitted")
	}
}

func TestToMap_WithData(t *testing.T) {
	r := Resource{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   ObjectMeta{Name: "mysecret", Namespace: "ns"},
		Data:       map[string]string{"key": "value"},
	}
	m, err := ToMap(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := m["data"]; !exists {
		t.Error("expected data to be present")
	}
}

// ============================================================================
// DeleteFromResource
// ============================================================================

func TestDeleteFromResource_StripsExtras(t *testing.T) {
	full := Resource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: ObjectMeta{
			Name:        "myapp",
			Namespace:   "production",
			Labels:      map[string]string{"app": "myapp"},
			Annotations: map[string]string{"note": "test"},
			Finalizers:  []string{"cleanup"},
		},
		Spec: map[string]interface{}{"replicas": 3},
	}
	del := DeleteFromResource(full)

	if del.APIVersion != "apps/v1" {
		t.Errorf("expected apiVersion 'apps/v1', got %q", del.APIVersion)
	}
	if del.Kind != "Deployment" {
		t.Errorf("expected kind 'Deployment', got %q", del.Kind)
	}
	if del.Metadata.Name != "myapp" {
		t.Errorf("expected name 'myapp', got %q", del.Metadata.Name)
	}
	if del.Metadata.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", del.Metadata.Namespace)
	}
	if del.Metadata.Labels != nil {
		t.Error("expected labels to be nil in delete resource")
	}
	if del.Metadata.Annotations != nil {
		t.Error("expected annotations to be nil in delete resource")
	}
	if del.Metadata.Finalizers != nil {
		t.Error("expected finalizers to be nil in delete resource")
	}
	if del.Spec != nil {
		t.Error("expected spec to be nil in delete resource")
	}
}

// ============================================================================
// DeleteOutputPathForResource
// ============================================================================

func TestDeleteOutputPathForResource_DefaultPrefix(t *testing.T) {
	r := Resource{
		Kind:     "Deployment",
		Metadata: ObjectMeta{Name: "my-app"},
	}
	path := DeleteOutputPathForResource("", r)
	expected := "resources/delete-deployment-my-app.yaml"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDeleteOutputPathForResource_CustomPrefix(t *testing.T) {
	r := Resource{
		Kind:     "Service",
		Metadata: ObjectMeta{Name: "web"},
	}
	path := DeleteOutputPathForResource("output", r)
	expected := "output/delete-service-web.yaml"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDeleteOutputPathForResource_PrefixWithSlash(t *testing.T) {
	r := Resource{
		Kind:     "ConfigMap",
		Metadata: ObjectMeta{Name: "cfg"},
	}
	path := DeleteOutputPathForResource("output/", r)
	expected := "output/delete-configmap-cfg.yaml"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDeleteOutputPathForResource_LowercasesKind(t *testing.T) {
	r := Resource{
		Kind:     "ClusterRoleBinding",
		Metadata: ObjectMeta{Name: "admin"},
	}
	path := DeleteOutputPathForResource("", r)
	expected := "resources/delete-clusterrolebinding-admin.yaml"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// ============================================================================
// ResourceMeta (builders.go)
// ============================================================================

func TestResourceMeta_AllFields(t *testing.T) {
	labels := map[string]string{"app": "test"}
	annotations := map[string]string{"note": "val"}
	meta := ResourceMeta("name", "ns", labels, annotations)
	if meta.Name != "name" {
		t.Errorf("expected 'name', got %q", meta.Name)
	}
	if meta.Namespace != "ns" {
		t.Errorf("expected 'ns', got %q", meta.Namespace)
	}
	if meta.Labels["app"] != "test" {
		t.Errorf("expected label 'app'='test', got %v", meta.Labels)
	}
	if meta.Annotations["note"] != "val" {
		t.Errorf("expected annotation 'note'='val', got %v", meta.Annotations)
	}
}

func TestResourceMeta_NilMaps(t *testing.T) {
	meta := ResourceMeta("name", "", nil, nil)
	if meta.Labels != nil {
		t.Error("expected nil labels")
	}
	if meta.Annotations != nil {
		t.Error("expected nil annotations")
	}
}

// ============================================================================
// BaseLabels (builders.go)
// ============================================================================

func TestBaseLabels(t *testing.T) {
	labels := BaseLabels("my-promise", "my-resource")
	expected := map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       "my-promise",
		"kratix.io/resource-name":      "my-resource",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("expected %s=%q, got %q", k, v, labels[k])
		}
	}
	if len(labels) != 3 {
		t.Errorf("expected exactly 3 labels, got %d", len(labels))
	}
}

// ============================================================================
// DeleteResource (builders.go)
// ============================================================================

func TestDeleteResource(t *testing.T) {
	r := DeleteResource("v1", "Service", "my-svc", "my-ns")
	if r.APIVersion != "v1" {
		t.Errorf("expected 'v1', got %q", r.APIVersion)
	}
	if r.Kind != "Service" {
		t.Errorf("expected 'Service', got %q", r.Kind)
	}
	if r.Metadata.Name != "my-svc" {
		t.Errorf("expected 'my-svc', got %q", r.Metadata.Name)
	}
	if r.Metadata.Namespace != "my-ns" {
		t.Errorf("expected 'my-ns', got %q", r.Metadata.Namespace)
	}
	if r.Spec != nil || r.Data != nil {
		t.Error("expected nil spec and data in delete resource")
	}
}

// ============================================================================
// ObjectMeta JSON serialization
// ============================================================================

func TestObjectMeta_OmitsEmptyOptionals(t *testing.T) {
	meta := ObjectMeta{Name: "test"}
	m, err := ToMap(Resource{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata:   meta,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metaMap, ok := m["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata to be a map")
	}
	if _, exists := metaMap["namespace"]; exists {
		t.Error("expected namespace to be omitted when empty")
	}
	if _, exists := metaMap["labels"]; exists {
		t.Error("expected labels to be omitted when nil")
	}
	if _, exists := metaMap["annotations"]; exists {
		t.Error("expected annotations to be omitted when nil")
	}
	if _, exists := metaMap["finalizers"]; exists {
		t.Error("expected finalizers to be omitted when nil")
	}
}

// ============================================================================
// RoleRef / Subject JSON roundtrip
// ============================================================================

func TestRoleRef_InResource(t *testing.T) {
	r := Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata:   ObjectMeta{Name: "admin-binding"},
		RoleRef: &RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
		Subjects: []Subject{
			{Kind: "ServiceAccount", Name: "default", Namespace: "kube-system"},
		},
	}
	m, err := ToMap(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["roleRef"] == nil {
		t.Error("expected roleRef in map")
	}
	subjects, ok := m["subjects"].([]interface{})
	if !ok || len(subjects) != 1 {
		t.Error("expected 1 subject")
	}
}

// ============================================================================
// SecretRef / SecretKey serialization
// ============================================================================

func TestSecretRef_Serialization(t *testing.T) {
	ref := SecretRef{
		Name:            "db-creds",
		OnePasswordItem: "vault-item",
		Keys: []SecretKey{
			{SecretKey: "password", Property: "password"},
		},
	}
	m, err := ToMap(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["name"] != "db-creds" {
		t.Errorf("expected 'db-creds', got %v", m["name"])
	}
	if m["onePasswordItem"] != "vault-item" {
		t.Errorf("expected 'vault-item', got %v", m["onePasswordItem"])
	}
}
