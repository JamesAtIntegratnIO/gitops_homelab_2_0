package main

// RegistrationConfig holds all configuration extracted from the ResourceRequest.
type RegistrationConfig struct {
	Name                   string
	TargetNamespace        string
	KubeconfigSecret       string
	KubeconfigKey          string
	ExternalServerURL      string
	OnePasswordItem        string
	OnePasswordConnectHost string
	Environment            string
	BaseDomain             string
	BaseDomainSanitized    string
	ClusterLabels          map[string]string
	ClusterAnnotations     map[string]string
	SyncJobName            string
	PromiseName            string
}
