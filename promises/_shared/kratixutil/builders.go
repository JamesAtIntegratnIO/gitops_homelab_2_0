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
