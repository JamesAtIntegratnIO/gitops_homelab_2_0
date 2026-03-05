package kratixutil

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

// Deprecated: Use DeleteFromResource instead.
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

func BuildServiceAccount(name, namespace string, labels map[string]string) Resource {
	return Resource{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
	}
}

func BuildRole(name, namespace string, labels map[string]string, rules []PolicyRule) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Rules:      rules,
	}
}

func BuildClusterRole(name string, labels map[string]string, rules []PolicyRule) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata:   ResourceMeta(name, "", labels, nil),
		Rules:      rules,
	}
}

func BuildRoleBinding(name, namespace string, labels map[string]string, roleRef RoleRef, subjects []Subject) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		RoleRef:    &roleRef,
		Subjects:   subjects,
	}
}

func BuildClusterRoleBinding(name string, labels map[string]string, roleRef RoleRef, subjects []Subject) Resource {
	return Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata:   ResourceMeta(name, "", labels, nil),
		RoleRef:    &roleRef,
		Subjects:   subjects,
	}
}
