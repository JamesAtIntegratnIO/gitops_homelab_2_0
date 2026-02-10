package main

func etcdEnabled(config *VClusterConfig) bool {
	if config.BackingStore == nil {
		return false
	}
	etcd, ok := config.BackingStore["etcd"].(map[string]interface{})
	if !ok {
		return false
	}
	deploy, ok := etcd["deploy"].(map[string]interface{})
	if !ok {
		return false
	}
	enabled, ok := deploy["enabled"].(bool)
	return ok && enabled
}

func resourceMeta(name, namespace string, labels, annotations map[string]string) ObjectMeta {
	return ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

func mergeStringMap(dst, src map[string]string) map[string]string {
	if dst == nil {
		dst = map[string]string{}
	}
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func baseLabels(config *VClusterConfig, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":      name,
	}
}

func deleteResource(apiVersion, kind, name, namespace string) Resource {
	return Resource{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
