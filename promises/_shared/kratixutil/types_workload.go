package kratixutil

// SecretRef defines a secret to provision via ExternalSecrets. Each ref maps
// a 1Password item to one or more Kubernetes Secret keys.
type SecretRef struct {
	Name            string      `json:"name"`
	OnePasswordItem string      `json:"onePasswordItem"`
	Keys            []SecretKey `json:"keys"`
}

// SecretKey maps a single key inside a Kubernetes Secret to a property
// inside a 1Password item.
type SecretKey struct {
	SecretKey string `json:"secretKey"`
	Property  string `json:"property"`
}

// PolicyRule defines a single RBAC permission (API groups × resources × verbs).
type PolicyRule struct {
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

// JobSpec configures a Kubernetes Job template.
type JobSpec struct {
	BackoffLimit            int             `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished int             `json:"ttlSecondsAfterFinished,omitempty"`
	Template                PodTemplateSpec `json:"template"`
}

// PodTemplateSpec wraps a PodSpec with optional metadata.
type PodTemplateSpec struct {
	Metadata *ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec     `json:"spec"`
}

// PodSpec configures a pod's containers, volumes, and scheduling.
type PodSpec struct {
	RestartPolicy      string      `json:"restartPolicy,omitempty"`
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	InitContainers     []Container `json:"initContainers,omitempty"`
	Containers         []Container `json:"containers"`
	Volumes            []Volume    `json:"volumes,omitempty"`
}

// Container configures a single container within a Pod.
type Container struct {
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	Command      []string      `json:"command,omitempty"`
	Env          []EnvVar      `json:"env,omitempty"`
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
}

// EnvVar is an environment variable entry within a container.
type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource references a secret key for populating an environment variable.
type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// SecretKeySelector identifies a key within a Kubernetes Secret.
type SecretKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// VolumeMount mounts a Volume into a container at the given path.
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// Volume defines a named volume that can be mounted by containers.
type Volume struct {
	Name   string        `json:"name"`
	Secret *SecretVolume `json:"secret,omitempty"`
}

// SecretVolume mounts a Kubernetes Secret as a volume.
type SecretVolume struct {
	SecretName string `json:"secretName"`
	Optional   bool   `json:"optional,omitempty"`
}
