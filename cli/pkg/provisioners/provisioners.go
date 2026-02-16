package provisioners

import (
	"fmt"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/score"
	"gopkg.in/yaml.v3"
)

// ProvisionResult holds the generated platform resources from a provisioner.
type ProvisionResult struct {
	// Outputs are key-value pairs available for placeholder substitution.
	Outputs map[string]string
	// Manifests are additional Kubernetes manifests to deploy alongside the workload.
	Manifests []map[string]interface{}
}

// Provisioner translates a Score resource into platform-native resources.
type Provisioner interface {
	// Type returns the Score resource type this provisioner handles.
	Type() string
	// Provision generates platform resources for the given Score resource.
	Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error)
}

// Registry holds all available provisioners.
type Registry struct {
	provisioners map[string]Provisioner
}

// NewRegistry creates a registry with all platform provisioners registered.
func NewRegistry() *Registry {
	r := &Registry{
		provisioners: make(map[string]Provisioner),
	}
	r.Register(&PostgresProvisioner{})
	r.Register(&RedisProvisioner{})
	r.Register(&RouteProvisioner{})
	r.Register(&VolumeProvisioner{})
	r.Register(&DNSProvisioner{})
	return r
}

// Register adds a provisioner to the registry.
func (r *Registry) Register(p Provisioner) {
	r.provisioners[p.Type()] = p
}

// Get returns the provisioner for the given resource type.
func (r *Registry) Get(resourceType string) (Provisioner, error) {
	p, ok := r.provisioners[resourceType]
	if !ok {
		return nil, fmt.Errorf("no provisioner for resource type %q (available: %s)",
			resourceType, strings.Join(r.Types(), ", "))
	}
	return p, nil
}

// Types returns all registered provisioner types.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.provisioners))
	for t := range r.provisioners {
		types = append(types, t)
	}
	return types
}

// --- Postgres Provisioner ---

// PostgresProvisioner generates ExternalSecret resources for PostgreSQL credentials.
type PostgresProvisioner struct{}

func (p *PostgresProvisioner) Type() string { return "postgres" }

func (p *PostgresProvisioner) Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error) {
	secretName := fmt.Sprintf("%s-%s-credentials", workloadName, name)
	opItem := fmt.Sprintf("%s-%s-db", workloadName, name)

	// Generate ExternalSecret
	externalSecret := map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": map[string]interface{}{
			"name": secretName,
		},
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-connect",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": secretName,
			},
			"data": []interface{}{
				map[string]interface{}{
					"secretKey": "host",
					"remoteRef": map[string]interface{}{
						"key":      opItem,
						"property": "host",
					},
				},
				map[string]interface{}{
					"secretKey": "port",
					"remoteRef": map[string]interface{}{
						"key":      opItem,
						"property": "port",
					},
				},
				map[string]interface{}{
					"secretKey": "database",
					"remoteRef": map[string]interface{}{
						"key":      opItem,
						"property": "database",
					},
				},
				map[string]interface{}{
					"secretKey": "username",
					"remoteRef": map[string]interface{}{
						"key":      opItem,
						"property": "username",
					},
				},
				map[string]interface{}{
					"secretKey": "password",
					"remoteRef": map[string]interface{}{
						"key":      opItem,
						"property": "password",
					},
				},
			},
		},
	}

	return &ProvisionResult{
		Outputs: map[string]string{
			"host":     fmt.Sprintf("$(%s:host)", secretName),
			"port":     fmt.Sprintf("$(%s:port)", secretName),
			"name":     fmt.Sprintf("$(%s:database)", secretName),
			"database": fmt.Sprintf("$(%s:database)", secretName),
			"username": fmt.Sprintf("$(%s:username)", secretName),
			"password": fmt.Sprintf("$(%s:password)", secretName),
		},
		Manifests: []map[string]interface{}{externalSecret},
	}, nil
}

// --- Redis Provisioner ---

// RedisProvisioner generates ExternalSecret resources for Redis credentials.
type RedisProvisioner struct{}

func (p *RedisProvisioner) Type() string { return "redis" }

func (p *RedisProvisioner) Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error) {
	secretName := fmt.Sprintf("%s-%s-credentials", workloadName, name)
	opItem := fmt.Sprintf("%s-%s-redis", workloadName, name)

	externalSecret := map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": map[string]interface{}{
			"name": secretName,
		},
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-connect",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": secretName,
			},
			"data": []interface{}{
				map[string]interface{}{
					"secretKey": "host",
					"remoteRef": map[string]interface{}{"key": opItem, "property": "host"},
				},
				map[string]interface{}{
					"secretKey": "port",
					"remoteRef": map[string]interface{}{"key": opItem, "property": "port"},
				},
				map[string]interface{}{
					"secretKey": "password",
					"remoteRef": map[string]interface{}{"key": opItem, "property": "password"},
				},
			},
		},
	}

	return &ProvisionResult{
		Outputs: map[string]string{
			"host":     fmt.Sprintf("$(%s:host)", secretName),
			"port":     fmt.Sprintf("$(%s:port)", secretName),
			"password": fmt.Sprintf("$(%s:password)", secretName),
		},
		Manifests: []map[string]interface{}{externalSecret},
	}, nil
}

// --- Route Provisioner ---

// RouteProvisioner generates HTTPRoute resources for Gateway API.
type RouteProvisioner struct{}

func (p *RouteProvisioner) Type() string { return "route" }

func (p *RouteProvisioner) Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error) {
	host, _ := resource.Params["host"].(string)
	path, _ := resource.Params["path"].(string)
	port := 8080
	if p, ok := resource.Params["port"]; ok {
		if pi, ok := p.(int); ok {
			port = pi
		}
	}

	if host == "" {
		return nil, fmt.Errorf("route resource %q requires params.host", name)
	}
	if path == "" {
		path = "/"
	}

	httpRoute := map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name": fmt.Sprintf("%s-%s", workloadName, name),
		},
		"spec": map[string]interface{}{
			"parentRefs": []interface{}{
				map[string]interface{}{
					"name":      "nginx",
					"namespace": "nginx-gateway",
				},
			},
			"hostnames": []string{host},
			"rules": []interface{}{
				map[string]interface{}{
					"matches": []interface{}{
						map[string]interface{}{
							"path": map[string]interface{}{
								"type":  "PathPrefix",
								"value": path,
							},
						},
					},
					"backendRefs": []interface{}{
						map[string]interface{}{
							"name": workloadName,
							"port": port,
						},
					},
				},
			},
		},
	}

	return &ProvisionResult{
		Outputs:   map[string]string{},
		Manifests: []map[string]interface{}{httpRoute},
	}, nil
}

// --- Volume Provisioner ---

// VolumeProvisioner generates PVC resources with NFS StorageClass.
type VolumeProvisioner struct{}

func (p *VolumeProvisioner) Type() string { return "volume" }

func (p *VolumeProvisioner) Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error) {
	pvcName := fmt.Sprintf("%s-%s", workloadName, name)
	size := "1Gi"
	if s, ok := resource.Params["size"].(string); ok {
		size = s
	}

	pvc := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name": pvcName,
		},
		"spec": map[string]interface{}{
			"accessModes": []string{"ReadWriteMany"},
			"storageClassName": "democratic-csi-nfs",
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"storage": size,
				},
			},
		},
	}

	return &ProvisionResult{
		Outputs: map[string]string{
			"source": pvcName,
		},
		Manifests: []map[string]interface{}{pvc},
	}, nil
}

// --- DNS Provisioner ---

// DNSProvisioner handles DNS record resources.
type DNSProvisioner struct{}

func (p *DNSProvisioner) Type() string { return "dns" }

func (p *DNSProvisioner) Provision(name string, resource score.Resource, workloadName string) (*ProvisionResult, error) {
	host, _ := resource.Params["host"].(string)
	if host == "" {
		host = fmt.Sprintf("%s.cluster.integratn.tech", workloadName)
	}

	return &ProvisionResult{
		Outputs: map[string]string{
			"host": host,
		},
		Manifests: nil, // DNS is handled by external-dns watching HTTPRoutes/Services
	}, nil
}

// MarshalManifests converts provisioner manifests to YAML strings.
func MarshalManifests(manifests []map[string]interface{}) ([]byte, error) {
	var sb strings.Builder
	for i, m := range manifests {
		if i > 0 {
			sb.WriteString("---\n")
		}
		data, err := yaml.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("marshaling manifest: %w", err)
		}
		sb.Write(data)
	}
	return []byte(sb.String()), nil
}
