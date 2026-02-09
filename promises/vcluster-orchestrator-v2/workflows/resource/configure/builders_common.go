package main

import (
	"fmt"
	"strings"
)

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

func resourceMeta(name, namespace string, labels, annotations map[string]string) map[string]interface{} {
	meta := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	if len(labels) > 0 {
		meta["labels"] = labels
	}
	if len(annotations) > 0 {
		meta["annotations"] = annotations
	}
	return meta
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

func deleteResource(apiVersion, kind, name, namespace string) map[string]interface{} {
	meta := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	return map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   meta,
	}
}

func deleteFromObject(obj map[string]interface{}) (map[string]interface{}, error) {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	meta, _ := obj["metadata"].(map[string]interface{})
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)

	if apiVersion == "" || kind == "" || name == "" {
		return nil, fmt.Errorf("missing apiVersion/kind/metadata.name for delete")
	}

	return deleteResource(apiVersion, kind, name, namespace), nil
}

func deleteOutputPath(prefix string, obj map[string]interface{}) (string, error) {
	kind, _ := obj["kind"].(string)
	meta, _ := obj["metadata"].(map[string]interface{})
	name, _ := meta["name"].(string)
	if kind == "" || name == "" {
		return "", fmt.Errorf("missing kind/metadata.name for delete path")
	}
	if prefix == "" {
		prefix = "resources/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sdelete-%s-%s.yaml", prefix, strings.ToLower(kind), name), nil
}
