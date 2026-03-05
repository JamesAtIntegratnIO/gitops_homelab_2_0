package kratixutil

import (
	"testing"
)

func TestBuildServiceAccount(t *testing.T) {
	labels := map[string]string{"team": "platform"}
	sa := BuildServiceAccount("my-sa", "my-ns", labels)

	if sa.APIVersion != "v1" {
		t.Errorf("expected apiVersion 'v1', got %q", sa.APIVersion)
	}
	if sa.Kind != "ServiceAccount" {
		t.Errorf("expected kind 'ServiceAccount', got %q", sa.Kind)
	}
	if sa.Metadata.Name != "my-sa" {
		t.Errorf("expected name 'my-sa', got %q", sa.Metadata.Name)
	}
	if sa.Metadata.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", sa.Metadata.Namespace)
	}
	if sa.Metadata.Labels["team"] != "platform" {
		t.Errorf("expected label team=platform, got %v", sa.Metadata.Labels)
	}
}

func TestBuildRole(t *testing.T) {
	rules := []PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
	}
	labels := map[string]string{"app": "test"}
	role := BuildRole("my-role", "my-ns", labels, rules)

	if role.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("expected apiVersion 'rbac.authorization.k8s.io/v1', got %q", role.APIVersion)
	}
	if role.Kind != "Role" {
		t.Errorf("expected kind 'Role', got %q", role.Kind)
	}
	if role.Metadata.Name != "my-role" {
		t.Errorf("expected name 'my-role', got %q", role.Metadata.Name)
	}
	if role.Metadata.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", role.Metadata.Namespace)
	}
	if len(role.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(role.Rules))
	}
	if role.Rules[0].Resources[0] != "pods" {
		t.Errorf("expected resource 'pods', got %q", role.Rules[0].Resources[0])
	}
	if role.Rules[0].Verbs[0] != "get" {
		t.Errorf("expected verb 'get', got %q", role.Rules[0].Verbs[0])
	}
}

func TestBuildClusterRole(t *testing.T) {
	rules := []PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"*"},
		},
	}
	labels := map[string]string{"managed-by": "kratix"}
	cr := BuildClusterRole("cluster-admin-role", labels, rules)

	if cr.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("expected apiVersion 'rbac.authorization.k8s.io/v1', got %q", cr.APIVersion)
	}
	if cr.Kind != "ClusterRole" {
		t.Errorf("expected kind 'ClusterRole', got %q", cr.Kind)
	}
	if cr.Metadata.Name != "cluster-admin-role" {
		t.Errorf("expected name 'cluster-admin-role', got %q", cr.Metadata.Name)
	}
	if cr.Metadata.Namespace != "" {
		t.Errorf("expected empty namespace for ClusterRole, got %q", cr.Metadata.Namespace)
	}
	if len(cr.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cr.Rules))
	}
	if cr.Rules[0].APIGroups[0] != "apps" {
		t.Errorf("expected apiGroup 'apps', got %q", cr.Rules[0].APIGroups[0])
	}
	if cr.Rules[1].Resources[0] != "services" {
		t.Errorf("expected resource 'services', got %q", cr.Rules[1].Resources[0])
	}
}

func TestBuildRoleBinding(t *testing.T) {
	labels := map[string]string{"app": "test"}
	roleRef := RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     "my-role",
	}
	subjects := []Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "my-sa",
			Namespace: "my-ns",
		},
	}
	rb := BuildRoleBinding("my-rb", "my-ns", labels, roleRef, subjects)

	if rb.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("expected apiVersion 'rbac.authorization.k8s.io/v1', got %q", rb.APIVersion)
	}
	if rb.Kind != "RoleBinding" {
		t.Errorf("expected kind 'RoleBinding', got %q", rb.Kind)
	}
	if rb.Metadata.Name != "my-rb" {
		t.Errorf("expected name 'my-rb', got %q", rb.Metadata.Name)
	}
	if rb.Metadata.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", rb.Metadata.Namespace)
	}
	if rb.RoleRef == nil {
		t.Fatal("expected roleRef to be set")
	}
	if rb.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("expected roleRef apiGroup 'rbac.authorization.k8s.io', got %q", rb.RoleRef.APIGroup)
	}
	if rb.RoleRef.Kind != "Role" {
		t.Errorf("expected roleRef kind 'Role', got %q", rb.RoleRef.Kind)
	}
	if rb.RoleRef.Name != "my-role" {
		t.Errorf("expected roleRef name 'my-role', got %q", rb.RoleRef.Name)
	}
	if len(rb.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(rb.Subjects))
	}
	if rb.Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("expected subject kind 'ServiceAccount', got %q", rb.Subjects[0].Kind)
	}
	if rb.Subjects[0].Name != "my-sa" {
		t.Errorf("expected subject name 'my-sa', got %q", rb.Subjects[0].Name)
	}
	if rb.Subjects[0].Namespace != "my-ns" {
		t.Errorf("expected subject namespace 'my-ns', got %q", rb.Subjects[0].Namespace)
	}
}

func TestBuildClusterRoleBinding(t *testing.T) {
	labels := map[string]string{"app": "test"}
	roleRef := RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "cluster-admin-role",
	}
	subjects := []Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "cluster-sa",
			Namespace: "kube-system",
		},
	}
	crb := BuildClusterRoleBinding("my-crb", labels, roleRef, subjects)

	if crb.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("expected apiVersion 'rbac.authorization.k8s.io/v1', got %q", crb.APIVersion)
	}
	if crb.Kind != "ClusterRoleBinding" {
		t.Errorf("expected kind 'ClusterRoleBinding', got %q", crb.Kind)
	}
	if crb.Metadata.Name != "my-crb" {
		t.Errorf("expected name 'my-crb', got %q", crb.Metadata.Name)
	}
	if crb.Metadata.Namespace != "" {
		t.Errorf("expected empty namespace for ClusterRoleBinding, got %q", crb.Metadata.Namespace)
	}
	if crb.RoleRef == nil {
		t.Fatal("expected roleRef to be set")
	}
	if crb.RoleRef.Kind != "ClusterRole" {
		t.Errorf("expected roleRef kind 'ClusterRole', got %q", crb.RoleRef.Kind)
	}
	if crb.RoleRef.Name != "cluster-admin-role" {
		t.Errorf("expected roleRef name 'cluster-admin-role', got %q", crb.RoleRef.Name)
	}
	if len(crb.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(crb.Subjects))
	}
	if crb.Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("expected subject kind 'ServiceAccount', got %q", crb.Subjects[0].Kind)
	}
	if crb.Subjects[0].Namespace != "kube-system" {
		t.Errorf("expected subject namespace 'kube-system', got %q", crb.Subjects[0].Namespace)
	}
}

func TestBuildRoleBinding_MultipleSubjects(t *testing.T) {
	roleRef := RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     "viewer",
	}
	subjects := []Subject{
		{Kind: "ServiceAccount", Name: "sa-1", Namespace: "ns-a"},
		{Kind: "ServiceAccount", Name: "sa-2", Namespace: "ns-b"},
		{Kind: "Group", Name: "developers"},
	}
	rb := BuildRoleBinding("multi-rb", "default", nil, roleRef, subjects)

	if len(rb.Subjects) != 3 {
		t.Fatalf("expected 3 subjects, got %d", len(rb.Subjects))
	}
	if rb.Subjects[2].Kind != "Group" {
		t.Errorf("expected third subject kind 'Group', got %q", rb.Subjects[2].Kind)
	}
	if rb.Subjects[2].Name != "developers" {
		t.Errorf("expected third subject name 'developers', got %q", rb.Subjects[2].Name)
	}
}

func TestBuildRole_MultipleRules(t *testing.T) {
	rules := []PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
		{APIGroups: []string{""}, Resources: []string{"services"}, Verbs: []string{"list"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "update"}},
	}
	role := BuildRole("multi-rule", "ns", nil, rules)

	if len(role.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(role.Rules))
	}
	if len(role.Rules[2].Verbs) != 2 {
		t.Errorf("expected 2 verbs in third rule, got %d", len(role.Rules[2].Verbs))
	}
}

func TestBuildServiceAccount_NilLabels(t *testing.T) {
	sa := BuildServiceAccount("no-labels", "default", nil)

	if sa.Metadata.Name != "no-labels" {
		t.Errorf("expected name 'no-labels', got %q", sa.Metadata.Name)
	}
	if sa.Metadata.Labels != nil {
		t.Errorf("expected nil labels, got %v", sa.Metadata.Labels)
	}
}
