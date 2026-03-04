// Package deploy translates Score workloads into platform-native Stakater Application
// chart values and supporting resources (ExternalSecrets, HTTPRoutes, PVCs, Certificates).
package deploy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/pkg/provisioners"
	"gopkg.in/yaml.v3"
)

// TranslateResult holds the output of a Score-to-Stakater translation.
type TranslateResult struct {
	// WorkloadName is the name of the workload from score.yaml metadata.
	WorkloadName string
	// TargetCluster is the vCluster this workload targets.
	TargetCluster string
	// Namespace is the deployment namespace.
	Namespace string
	// StakaterValues is the Stakater Application chart values.yaml content.
	StakaterValues map[string]interface{}
	// AddonsEntry is the entry for workloads/<cluster>/addons.yaml.
	AddonsEntry map[string]interface{}
	// Files maps relative file paths to their content for writing.
	Files map[string][]byte
}

// secretRefRegex matches provisioner output patterns like $(secret-name:key).
var secretRefRegex = regexp.MustCompile(`^\$\(([^:]+):([^)]+)\)$`)

// scoreVarRegex matches Score resource reference patterns like ${resources.db.host}.
var scoreVarRegex = regexp.MustCompile(`\$\{resources\.([^.]+)\.([^}]+)\}`)

// Translate converts a Score workload into platform resources.
// cfg is the active configuration; passing it explicitly avoids a hidden
// dependency on the config.Get() global singleton.
func Translate(workload *score.Workload, cluster string, cfg *config.Config) (*TranslateResult, error) {
	if cfg == nil {
		cfg = config.Get()
	}

	if cluster == "" {
		cluster = workload.TargetCluster()
	}
	if cluster == "" {
		cluster = cfg.DefaultCluster
	}
	if cluster == "" {
		return nil, fmt.Errorf("no target cluster specified — use --cluster, set hctl.integratn.tech/cluster annotation, or configure defaultCluster")
	}

	namespace := cluster // workload namespace defaults to cluster name
	if ns, ok := workload.Metadata.Annotations["hctl.integratn.tech/namespace"]; ok && ns != "" {
		namespace = ns
	}

	// Run provisioners for all resources
	registry := provisioners.NewRegistry()
	allOutputs := make(map[string]map[string]string) // resource-name → key → value
	var extraObjects []map[string]interface{}

	for resName, res := range workload.Resources {
		prov, err := registry.Get(res.Type)
		if err != nil {
			return nil, fmt.Errorf("resource %q: %w", resName, err)
		}

		result, err := prov.Provision(resName, res, workload.Metadata.Name)
		if err != nil {
			return nil, fmt.Errorf("provisioning resource %q: %w", resName, err)
		}

		allOutputs[resName] = result.Outputs

		// Add namespace to manifests
		for _, m := range result.Manifests {
			if meta, ok := m["metadata"].(map[string]interface{}); ok {
				if _, hasNs := meta["namespace"]; !hasNs {
					meta["namespace"] = namespace
				}
			}
			extraObjects = append(extraObjects, m)
		}
	}

	// Build Stakater values
	values := buildStakaterValues(workload, allOutputs, namespace, extraObjects)

	// Build addons.yaml entry
	addonsEntry := map[string]interface{}{
		"enabled":         true,
		"namespace":       namespace,
		"chartRepository": "https://stakater.github.io/stakater-charts",
		"chartName":       "application",
		"defaultVersion":  "6.14.0",
	}

	// Build file map
	result := &TranslateResult{
		WorkloadName:   workload.Metadata.Name,
		TargetCluster:  cluster,
		Namespace:      namespace,
		StakaterValues: values,
		AddonsEntry:    addonsEntry,
		Files:          make(map[string][]byte),
	}

	valuesData, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshaling values: %w", err)
	}

	valuesPath := filepath.Join("workloads", cluster, "addons", workload.Metadata.Name, "values.yaml")
	result.Files[valuesPath] = valuesData

	return result, nil
}

// buildStakaterValues creates the Stakater Application chart values.
func buildStakaterValues(w *score.Workload, allOutputs map[string]map[string]string, namespace string, extraObjects []map[string]interface{}) map[string]interface{} {
	values := map[string]interface{}{
		"applicationName": w.Metadata.Name,
	}

	// --- Deployment section ---
	deployment, additionalContainers := buildDeploymentSection(w, allOutputs)
	if len(additionalContainers) > 0 {
		deployment["additionalContainers"] = additionalContainers
	}
	values["deployment"] = deployment

	// --- Service section ---
	if svc := buildServiceSection(w); svc != nil {
		values["service"] = svc
	}

	// --- Persistence (disabled, managed via extraObjects) ---
	values["persistence"] = map[string]interface{}{
		"enabled": false,
	}

	// --- HTTPRoute and Certificate from route resources ---
	buildRouteAndCertSections(w, values)

	// --- Extra objects (provisioner manifests: ExternalSecrets, PVCs) ---
	if len(extraObjects) > 0 {
		var extras []interface{}
		for _, obj := range extraObjects {
			extras = append(extras, obj)
		}
		values["extraObjects"] = extras
	}

	return values
}

// buildDeploymentSection constructs the deployment values and additional containers from the Score workload.
func buildDeploymentSection(w *score.Workload, allOutputs map[string]map[string]string) (map[string]interface{}, []map[string]interface{}) {
	deployment := map[string]interface{}{}

	// Use the first (or only) container for the primary deployment
	var primaryContainer score.Container
	var containerName string
	var additionalContainers []map[string]interface{}

	// Sort container names for deterministic output
	containerNames := make([]string, 0, len(w.Containers))
	for name := range w.Containers {
		containerNames = append(containerNames, name)
	}
	sort.Strings(containerNames)

	for i, name := range containerNames {
		c := w.Containers[name]
		if i == 0 {
			primaryContainer = c
			containerName = name
			_ = containerName
		} else {
			additionalContainers = append(additionalContainers, buildContainerSpec(name, c, allOutputs))
		}
	}

	// Image
	if primaryContainer.Image != "" && primaryContainer.Image != "." {
		parts := strings.SplitN(primaryContainer.Image, ":", 2)
		deployment["image"] = map[string]interface{}{
			"repository": parts[0],
		}
		if len(parts) == 2 {
			deployment["image"].(map[string]interface{})["tag"] = parts[1]
		} else {
			deployment["image"].(map[string]interface{})["tag"] = "latest"
		}
	}

	// Ports from service
	if w.Service != nil && len(w.Service.Ports) > 0 {
		var ports []map[string]interface{}
		portNames := make([]string, 0, len(w.Service.Ports))
		for name := range w.Service.Ports {
			portNames = append(portNames, name)
		}
		sort.Strings(portNames)

		for _, name := range portNames {
			p := w.Service.Ports[name]
			port := map[string]interface{}{
				"name":          name,
				"containerPort": p.Port,
				"protocol":      "TCP",
			}
			if p.Protocol != "" {
				port["protocol"] = p.Protocol
			}
			ports = append(ports, port)
		}
		deployment["ports"] = ports
	}

	// Environment variables — resolve Score resource references
	if len(primaryContainer.Variables) > 0 {
		env := map[string]interface{}{}
		varNames := make([]string, 0, len(primaryContainer.Variables))
		for name := range primaryContainer.Variables {
			varNames = append(varNames, name)
		}
		sort.Strings(varNames)

		for _, name := range varNames {
			val := primaryContainer.Variables[name]
			resolved := resolveVariableValue(val, allOutputs)
			env[name] = resolved
		}
		deployment["env"] = env
	}

	// Resources
	if primaryContainer.Resources != nil {
		resources := map[string]interface{}{}
		if primaryContainer.Resources.Requests != nil {
			resources["requests"] = primaryContainer.Resources.Requests
		}
		if primaryContainer.Resources.Limits != nil {
			resources["limits"] = primaryContainer.Resources.Limits
		}
		deployment["resources"] = resources
	}

	// Volume mounts
	if len(primaryContainer.Volumes) > 0 {
		volumes := map[string]interface{}{}
		volumeMounts := map[string]interface{}{}

		volNames := make([]string, 0, len(primaryContainer.Volumes))
		for name := range primaryContainer.Volumes {
			volNames = append(volNames, name)
		}
		sort.Strings(volNames)

		for _, name := range volNames {
			vol := primaryContainer.Volumes[name]
			// source refers to a Score resource, resolve to PVC name
			pvcName := vol.Source
			if outputs, ok := allOutputs[vol.Source]; ok {
				if src, ok := outputs["source"]; ok {
					pvcName = src
				}
			}
			volumes[name] = map[string]interface{}{
				"persistentVolumeClaim": map[string]interface{}{
					"claimName": pvcName,
				},
			}
			mount := map[string]interface{}{
				"mountPath": vol.Path,
			}
			if vol.ReadOnly {
				mount["readOnly"] = true
			}
			volumeMounts[name] = mount
		}
		deployment["volumes"] = volumes
		deployment["volumeMounts"] = volumeMounts
	}

	return deployment, additionalContainers
}

// buildServiceSection constructs the service values from the Score workload. Returns nil if no service.
func buildServiceSection(w *score.Workload) map[string]interface{} {
	if w.Service == nil || len(w.Service.Ports) == 0 {
		return nil
	}

	var servicePorts []map[string]interface{}
	portNames := make([]string, 0, len(w.Service.Ports))
	for name := range w.Service.Ports {
		portNames = append(portNames, name)
	}
	sort.Strings(portNames)

	for _, name := range portNames {
		p := w.Service.Ports[name]
		sp := map[string]interface{}{
			"name":       name,
			"port":       p.Port,
			"targetPort": p.Port,
			"protocol":   "TCP",
		}
		if p.TargetPort > 0 {
			sp["targetPort"] = p.TargetPort
		}
		if p.Protocol != "" {
			sp["protocol"] = p.Protocol
		}
		servicePorts = append(servicePorts, sp)
	}
	return map[string]interface{}{
		"ports": servicePorts,
	}
}

// buildRouteAndCertSections adds httpRoute and certificate entries to values if a route resource is present.
func buildRouteAndCertSections(w *score.Workload, values map[string]interface{}) {
	for _, res := range w.Resources {
		if res.Type != "route" {
			continue
		}
		host, _ := res.Params["host"].(string)
		port := 8080
		if p, ok := res.Params["port"]; ok {
			if pi, ok := p.(int); ok {
				port = pi
			}
			if pf, ok := p.(float64); ok {
				port = int(pf)
			}
		}
		path := "/"
		if p, ok := res.Params["path"].(string); ok {
			path = p
		}

		if host != "" {
			values["httpRoute"] = map[string]interface{}{
				"enabled": true,
				"parentRefs": []map[string]interface{}{
					{
						"name":        "nginx-gateway",
						"namespace":   "nginx-gateway",
						"sectionName": "https-public",
					},
				},
				"hostnames": []string{host},
				"rules": []map[string]interface{}{
					{
						"backendRefs": []map[string]interface{}{
							{
								"name": w.Metadata.Name,
								"port": port,
							},
						},
						"matches": []map[string]interface{}{
							{
								"path": map[string]interface{}{
									"type":  "PathPrefix",
									"value": path,
								},
							},
						},
					},
				},
			}

			// Auto-generate certificate
			values["certificate"] = map[string]interface{}{
				"enabled":    true,
				"secretName": w.Metadata.Name + "-tls",
				"dnsNames":   []string{host},
				"commonName": host,
				"usages":     []string{"digital signature", "key encipherment", "server auth"},
				"issuerRef": map[string]interface{}{
					"name": "letsencrypt-prod",
					"kind": "ClusterIssuer",
				},
			}
		}
		break // only use the first route resource
	}
}

// buildContainerSpec converts a Score container to a Stakater additional container spec.
func buildContainerSpec(name string, c score.Container, allOutputs map[string]map[string]string) map[string]interface{} {
	spec := map[string]interface{}{
		"name":  name,
		"image": c.Image,
	}
	if len(c.Command) > 0 {
		spec["command"] = c.Command
	}
	if len(c.Args) > 0 {
		spec["args"] = c.Args
	}
	if len(c.Variables) > 0 {
		var envList []map[string]interface{}
		varNames := make([]string, 0, len(c.Variables))
		for name := range c.Variables {
			varNames = append(varNames, name)
		}
		sort.Strings(varNames)
		for _, name := range varNames {
			val := c.Variables[name]
			envEntry := map[string]interface{}{"name": name}
			resolved := resolveVariableValue(val, allOutputs)
			if valMap, ok := resolved.(map[string]interface{}); ok {
				if vf, ok := valMap["valueFrom"]; ok {
					envEntry["valueFrom"] = vf
				} else if v, ok := valMap["value"]; ok {
					envEntry["value"] = v
				}
			}
			envList = append(envList, envEntry)
		}
		spec["env"] = envList
	}
	return spec
}

// resolveVariableValue translates Score variable references to Stakater env format.
// Handles:
//   - ${resources.db.host} → secretKeyRef if the resource output is $(secret:key)
//   - $(secret-name:key) → secretKeyRef
//   - literal values → { value: "..." }
func resolveVariableValue(val string, allOutputs map[string]map[string]string) interface{} {
	// Check for Score resource reference: ${resources.<name>.<key>}
	if matches := scoreVarRegex.FindStringSubmatch(val); len(matches) == 3 {
		resName := matches[1]
		resKey := matches[2]

		if outputs, ok := allOutputs[resName]; ok {
			if output, ok := outputs[resKey]; ok {
				// Check if the output is a secret reference
				if ref := secretRefRegex.FindStringSubmatch(output); len(ref) == 3 {
					return map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": ref[1],
								"key":  ref[2],
							},
						},
					}
				}
				// Literal provisioner output
				return map[string]interface{}{"value": output}
			}
		}
		// Unresolved reference — leave as placeholder
		return map[string]interface{}{"value": val}
	}

	// Direct secret reference: $(secret-name:key)
	if ref := secretRefRegex.FindStringSubmatch(val); len(ref) == 3 {
		return map[string]interface{}{
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"name": ref[1],
					"key":  ref[2],
				},
			},
		}
	}

	// Literal value
	return map[string]interface{}{"value": val}
}
