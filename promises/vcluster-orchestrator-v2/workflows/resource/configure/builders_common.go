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
