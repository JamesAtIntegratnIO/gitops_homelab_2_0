package score

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Workload represents a parsed Score workload specification (score.dev/v1b1).
type Workload struct {
	APIVersion string            `yaml:"apiVersion"`
	Metadata   WorkloadMetadata  `yaml:"metadata"`
	Containers map[string]Container `yaml:"containers"`
	Service    *Service          `yaml:"service,omitempty"`
	Resources  map[string]Resource `yaml:"resources,omitempty"`
}

// WorkloadMetadata holds workload identity and annotations.
type WorkloadMetadata struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Container represents a Score container spec.
type Container struct {
	Image     string            `yaml:"image"`
	Command   []string          `yaml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
	Files     map[string]File   `yaml:"files,omitempty"`
	Volumes   map[string]Volume `yaml:"volumes,omitempty"`
	Resources *ComputeResources `yaml:"resources,omitempty"`
	LivenessProbe  *Probe       `yaml:"livenessProbe,omitempty"`
	ReadinessProbe *Probe       `yaml:"readinessProbe,omitempty"`
}

// ComputeResources holds resource requests and limits.
type ComputeResources struct {
	Requests map[string]string `yaml:"requests,omitempty"`
	Limits   map[string]string `yaml:"limits,omitempty"`
}

// File represents a Score file mount.
type File struct {
	Mode          string `yaml:"mode,omitempty"`
	Content       string `yaml:"content,omitempty"`
	BinaryContent string `yaml:"binaryContent,omitempty"`
	Source        string `yaml:"source,omitempty"`
	NoExpand      bool   `yaml:"noExpand,omitempty"`
}

// Volume represents a Score volume mount.
type Volume struct {
	Source   string `yaml:"source"`
	Path    string `yaml:"path,omitempty"`
	ReadOnly bool  `yaml:"readOnly,omitempty"`
}

// Service represents a Score service spec.
type Service struct {
	Ports map[string]Port `yaml:"ports"`
}

// Port represents a Score port spec.
type Port struct {
	Port       int    `yaml:"port"`
	Protocol   string `yaml:"protocol,omitempty"`
	TargetPort int    `yaml:"targetPort,omitempty"`
}

// Resource represents a Score resource dependency.
type Resource struct {
	Type     string                 `yaml:"type"`
	Class    string                 `yaml:"class,omitempty"`
	ID       string                 `yaml:"id,omitempty"`
	Metadata map[string]interface{} `yaml:"metadata,omitempty"`
	Params   map[string]interface{} `yaml:"params,omitempty"`
}

// Probe represents a health check probe.
type Probe struct {
	HTTPGet *HTTPGetProbe `yaml:"httpGet,omitempty"`
	Exec    *ExecProbe    `yaml:"exec,omitempty"`
}

// HTTPGetProbe represents an HTTP health check.
type HTTPGetProbe struct {
	Scheme      string       `yaml:"scheme,omitempty"`
	Host        string       `yaml:"host,omitempty"`
	Path        string       `yaml:"path"`
	Port        int          `yaml:"port"`
	HTTPHeaders []HTTPHeader `yaml:"httpHeaders,omitempty"`
}

// ExecProbe represents a command health check.
type ExecProbe struct {
	Command []string `yaml:"command"`
}

// HTTPHeader represents an HTTP header.
type HTTPHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// LoadWorkload reads and parses a score.yaml file.
func LoadWorkload(path string) (*Workload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var w Workload
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if w.APIVersion != "score.dev/v1b1" {
		return nil, fmt.Errorf("unsupported Score API version: %q (expected score.dev/v1b1)", w.APIVersion)
	}

	if w.Metadata.Name == "" {
		return nil, fmt.Errorf("metadata.name is required")
	}

	if len(w.Containers) == 0 {
		return nil, fmt.Errorf("at least one container is required")
	}

	return &w, nil
}

// TargetCluster returns the target vCluster from workload annotations.
func (w *Workload) TargetCluster() string {
	if w.Metadata.Annotations != nil {
		return w.Metadata.Annotations["hctl.integratn.tech/cluster"]
	}
	return ""
}

// ResourcesByType returns all resources matching the given type.
func (w *Workload) ResourcesByType(resourceType string) map[string]Resource {
	result := make(map[string]Resource)
	for name, r := range w.Resources {
		if r.Type == resourceType {
			result[name] = r
		}
	}
	return result
}
