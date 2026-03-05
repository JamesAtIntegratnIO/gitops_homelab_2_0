package kratixutil

// ============================================================================
// Resource Construction Helpers
// ============================================================================

func ResourceMeta(name, namespace string, labels, annotations map[string]string) ObjectMeta {
	return ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

func BaseLabels(promiseName, resourceName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       promiseName,
		"kratix.io/resource-name":      resourceName,
	}
}

// DeleteResource creates a minimal resource with only identity fields,
// suitable for emitting as a Kratix delete output.
func DeleteResource(apiVersion, kind, name, namespace string) Resource {
	return Resource{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ============================================================================
// RBAC Resource Builders
// ============================================================================

// BuildServiceAccount creates a namespace-scoped ServiceAccount for use in
// pipeline jobs or operator workloads. The resulting resource can be written
// directly to the Kratix state store.
func BuildServiceAccount(name, namespace string, labels map[string]string) Resource {
	return Resource{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
	}
}

// BuildRole creates a namespace-scoped Role. Use BuildClusterRole for
// cluster-wide permissions.
func BuildRole(name, namespace string, labels map[string]string, rules []PolicyRule) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Rules:      rules,
	}
}

// BuildClusterRole creates a cluster-wide ClusterRole. Use BuildRole when
// permissions should be scoped to a single namespace.
func BuildClusterRole(name string, labels map[string]string, rules []PolicyRule) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata:   ResourceMeta(name, "", labels, nil),
		Rules:      rules,
	}
}

// BuildRoleBinding binds a Role to the given subjects within a namespace.
// Use BuildClusterRoleBinding for cluster-wide bindings.
func BuildRoleBinding(name, namespace string, labels map[string]string, roleRef RoleRef, subjects []Subject) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		RoleRef:    &roleRef,
		Subjects:   subjects,
	}
}

// BuildClusterRoleBinding binds a ClusterRole to the given subjects across
// all namespaces.
func BuildClusterRoleBinding(name string, labels map[string]string, roleRef RoleRef, subjects []Subject) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata:   ResourceMeta(name, "", labels, nil),
		RoleRef:    &roleRef,
		Subjects:   subjects,
	}
}
