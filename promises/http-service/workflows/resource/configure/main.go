package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"
)

// Platform-wide defaults — baked into every HTTP service.
const (
	defaultBaseDomain    = "cluster.integratn.tech"
	defaultGatewayName   = "nginx-gateway"
	defaultGatewayNS     = "nginx-gateway"
	defaultSecretStore   = "onepassword-connect"
	defaultSecretStoreKind = "ClusterSecretStore"
	stakaterChartRepo    = "https://stakater.github.io/stakater-charts"
	stakaterChartName    = "application"
	stakaterChartVersion = "6.16.1"
	argoCDProject        = "default"
)

func main() {
	sdk := kratix.New()

	log.Printf("=== HTTP Service Promise Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	config, err := buildConfig(resource)
	if err != nil {
		log.Fatalf("ERROR: Failed to build config: %v", err)
	}

	if sdk.WorkflowAction() == "configure" {
		if err := handleConfigure(sdk, config); err != nil {
			log.Fatalf("ERROR: Configure failed: %v", err)
		}
	} else if sdk.WorkflowAction() == "delete" {
		if err := handleDelete(sdk, config); err != nil {
			log.Fatalf("ERROR: Delete failed: %v", err)
		}
	} else {
		log.Fatalf("ERROR: Unknown workflow action: %s", sdk.WorkflowAction())
	}

	log.Println("=== Pipeline completed successfully ===")
}

// buildConfig extracts all fields from the CR with sensible defaults.
func buildConfig(resource kratix.Resource) (*HTTPServiceConfig, error) {
	config := &HTTPServiceConfig{
		BaseDomain:      defaultBaseDomain,
		GatewayName:     defaultGatewayName,
		GatewayNS:       defaultGatewayNS,
		SecretStoreName:  defaultSecretStore,
		SecretStoreKind:  defaultSecretStoreKind,
	}

	var err error
	config.Name, err = getStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace, _ = getStringValueWithDefault(resource, "spec.namespace", config.Name)
	config.Team, _ = getStringValueWithDefault(resource, "spec.team", "platform")

	// Image
	config.ImageRepository, err = getStringValue(resource, "spec.image.repository")
	if err != nil {
		return nil, fmt.Errorf("spec.image.repository is required: %w", err)
	}
	config.ImageTag, _ = getStringValueWithDefault(resource, "spec.image.tag", "latest")
	config.ImagePullPolicy, _ = getStringValueWithDefault(resource, "spec.image.pullPolicy", "IfNotPresent")
	config.Command = extractStringSlice(resource, "spec.command")
	config.Args = extractStringSlice(resource, "spec.args")

	// Scaling
	config.Replicas, _ = getIntValueWithDefault(resource, "spec.replicas", 1)
	config.CPURequest, _ = getStringValueWithDefault(resource, "spec.resources.requests.cpu", "100m")
	config.MemoryRequest, _ = getStringValueWithDefault(resource, "spec.resources.requests.memory", "128Mi")
	config.CPULimit, _ = getStringValueWithDefault(resource, "spec.resources.limits.cpu", "500m")
	config.MemoryLimit, _ = getStringValueWithDefault(resource, "spec.resources.limits.memory", "256Mi")

	// Networking
	config.Port, _ = getIntValueWithDefault(resource, "spec.port", 8080)
	config.IngressEnabled, _ = getBoolValueWithDefault(resource, "spec.ingress.enabled", true)
	config.IngressHostname, _ = getStringValue(resource, "spec.ingress.hostname")
	if config.IngressHostname == "" {
		config.IngressHostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.IngressPath, _ = getStringValueWithDefault(resource, "spec.ingress.path", "/")

	// Secrets
	config.Secrets = extractSecrets(resource)

	// Environment
	config.Env = extractStringMap(resource, "spec.env")
	config.EnvFromSecrets = extractStringSlice(resource, "spec.envFromSecrets")

	// Health checks
	config.HealthCheckPath, _ = getStringValueWithDefault(resource, "spec.healthCheck.path", "/healthz")
	config.HealthCheckPort, _ = getIntValueWithDefault(resource, "spec.healthCheck.port", config.Port)

	// Monitoring
	config.MonitoringEnabled, _ = getBoolValueWithDefault(resource, "spec.monitoring.enabled", false)
	config.MonitoringPath, _ = getStringValueWithDefault(resource, "spec.monitoring.path", "/metrics")
	config.MonitoringInterval, _ = getStringValueWithDefault(resource, "spec.monitoring.interval", "30s")

	// Storage
	config.PersistenceEnabled, _ = getBoolValueWithDefault(resource, "spec.persistence.enabled", false)
	config.PersistenceSize, _ = getStringValueWithDefault(resource, "spec.persistence.size", "1Gi")
	config.PersistenceClass, _ = getStringValue(resource, "spec.persistence.storageClass")
	config.PersistenceMountPath, _ = getStringValueWithDefault(resource, "spec.persistence.mountPath", "/data")

	// Helm overrides
	if val, err := resource.GetValue("spec.helmOverrides"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.HelmOverrides = m
		}
	}

	return config, nil
}

// handleConfigure generates the Namespace + ArgoCD Application + ExternalSecrets + NetworkPolicies.
func handleConfigure(sdk *kratix.KratixSDK, config *HTTPServiceConfig) error {
	// 0. Create the target Namespace first (low sync-wave so it exists before everything else)
	ns := Resource{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: ObjectMeta{
			Name: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
				"app.kubernetes.io/team":       config.Team,
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "0",
			},
		},
	}
	if err := writeYAML(sdk, "resources/namespace.yaml", ns); err != nil {
		return fmt.Errorf("write Namespace: %w", err)
	}
	log.Printf("✓ Rendered Namespace: %s", config.Namespace)

	// 1. Build Stakater application chart values
	values := buildStakaterValues(config)

	// 2. Deep-merge any helmOverrides on top
	if config.HelmOverrides != nil {
		values = deepMerge(values, config.HelmOverrides)
	}

	// 3. Build ArgoCD Application pointing at the Stakater chart
	app := Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ObjectMeta{
			Name:      config.Name,
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
				"app.kubernetes.io/team":       config.Team,
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "10",
			},
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
		},
		Spec: ApplicationSpec{
			Project: argoCDProject,
			Source: AppSource{
				RepoURL:        stakaterChartRepo,
				Chart:          stakaterChartName,
				TargetRevision: stakaterChartVersion,
				Helm: &HelmSource{
					ReleaseName:  config.Name,
					ValuesObject: values,
				},
			},
			Destination: Destination{
				Server:    "https://kubernetes.default.svc",
				Namespace: config.Namespace,
			},
			SyncPolicy: map[string]interface{}{
				"automated": map[string]interface{}{
					"selfHeal": true,
					"prune":    true,
				},
				"syncOptions": []string{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
			},
		},
	}

	if err := writeYAML(sdk, "resources/argocd-application.yaml", app); err != nil {
		return fmt.Errorf("write ArgoCD Application: %w", err)
	}
	log.Printf("✓ Rendered ArgoCD Application: %s", config.Name)

	// 4. Build ExternalSecrets (these go directly, not through Stakater, 
	//    because we need them created before the Deployment references them)
	if len(config.Secrets) > 0 {
		externalSecrets := buildExternalSecrets(config)
		if err := writeYAMLDocuments(sdk, "resources/external-secrets.yaml", externalSecrets); err != nil {
			return fmt.Errorf("write ExternalSecrets: %w", err)
		}
		log.Printf("✓ Rendered %d ExternalSecret(s)", len(config.Secrets))
	}

	// 5. Build a default deny-all + allow-ingress NetworkPolicy
	netpols := buildNetworkPolicies(config)
	if err := writeYAMLDocuments(sdk, "resources/network-policies.yaml", netpols); err != nil {
		return fmt.Errorf("write NetworkPolicies: %w", err)
	}
	log.Printf("✓ Rendered NetworkPolicies")

	// 6. Write status
	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("HTTP Service %s configured", config.Name))
	status.Set("namespace", config.Namespace)
	if config.IngressEnabled {
		status.Set("url", fmt.Sprintf("https://%s%s", config.IngressHostname, config.IngressPath))
	}

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// handleDelete cleans up the ArgoCD Application.
func handleDelete(sdk *kratix.KratixSDK, config *HTTPServiceConfig) error {
	app := Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ObjectMeta{
			Name:      config.Name,
			Namespace: "argocd",
		},
	}

	if err := writeYAML(sdk, "resources/delete-application.yaml", app); err != nil {
		return fmt.Errorf("write delete application: %w", err)
	}
	log.Printf("✓ Delete scheduled for HTTP Service: %s", config.Name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("HTTP Service %s scheduled for deletion", config.Name))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// buildStakaterValues constructs the Helm values for the Stakater application chart.
func buildStakaterValues(config *HTTPServiceConfig) map[string]interface{} {
	values := map[string]interface{}{
		"applicationName": config.Name,

		"additionalLabels": map[string]string{
			"app.kubernetes.io/managed-by": "kratix",
			"kratix.io/promise-name":       "http-service",
			"app.kubernetes.io/part-of":    config.Name,
			"app.kubernetes.io/team":       config.Team,
		},

		// ── Deployment ──────────────────────────────────
		"deployment": map[string]interface{}{
			"enabled":  true,
			"replicas": config.Replicas,
			"image": map[string]interface{}{
				"repository": config.ImageRepository,
				"tag":        config.ImageTag,
				"pullPolicy": config.ImagePullPolicy,
			},
			"command": config.Command,
			"args":    config.Args,
			"ports": []map[string]interface{}{
				{
					"containerPort": config.Port,
					"name":          "http",
					"protocol":      "TCP",
				},
			},
			"resources": map[string]interface{}{
				"requests": map[string]string{
					"cpu":    config.CPURequest,
					"memory": config.MemoryRequest,
				},
				"limits": map[string]string{
					"cpu":    config.CPULimit,
					"memory": config.MemoryLimit,
				},
			},
			"readinessProbe": map[string]interface{}{
				"enabled":          true,
				"failureThreshold": 3,
				"periodSeconds":    10,
				"successThreshold": 1,
				"timeoutSeconds":   3,
				"httpGet": map[string]interface{}{
					"path":   config.HealthCheckPath,
					"port":   config.HealthCheckPort,
					"scheme": "HTTP",
				},
			},
			"livenessProbe": map[string]interface{}{
				"enabled":          true,
				"failureThreshold": 3,
				"periodSeconds":    10,
				"successThreshold": 1,
				"timeoutSeconds":   3,
				"httpGet": map[string]interface{}{
					"path":   config.HealthCheckPath,
					"port":   config.HealthCheckPort,
					"scheme": "HTTP",
				},
			},
			"containerSecurityContext": map[string]interface{}{
				"readOnlyRootFilesystem": true,
				"runAsNonRoot":           true,
			},
			"revisionHistoryLimit": 3,
			"reloadOnChange":       true,
		},

		// ── Service ─────────────────────────────────────
		"service": map[string]interface{}{
			"enabled": true,
			"type":    "ClusterIP",
			"ports": []map[string]interface{}{
				{
					"port":       config.Port,
					"name":       "http",
					"protocol":   "TCP",
					"targetPort": config.Port,
				},
			},
		},

		// ── RBAC ────────────────────────────────────────
		"rbac": map[string]interface{}{
			"enabled": true,
			"serviceAccount": map[string]interface{}{
				"enabled": true,
				"name":    config.Name,
			},
		},
	}

	// ── Env vars ────────────────────────────────────
	if len(config.Env) > 0 {
		envMap := map[string]interface{}{}
		for k, v := range config.Env {
			envMap[k] = map[string]interface{}{
				"value": v,
			}
		}
		deployment := values["deployment"].(map[string]interface{})
		deployment["env"] = envMap
	}

	// ── EnvFrom (mount secrets) ─────────────────────
	if len(config.EnvFromSecrets) > 0 || len(config.Secrets) > 0 {
		envFrom := map[string]interface{}{}
		// Wire up any explicitly requested envFrom secrets
		for _, s := range config.EnvFromSecrets {
			envFrom[s] = map[string]interface{}{
				"type":       "secret",
				"nameSuffix": s,
			}
		}
		// Also wire up ExternalSecrets so the deployment gets them
		for _, s := range config.Secrets {
			secretName := s.Name
			if secretName == "" {
				secretName = fmt.Sprintf("%s-%s", config.Name, s.OnePasswordItem)
			}
			envFrom[secretName] = map[string]interface{}{
				"type":       "secret",
				"nameSuffix": secretName,
			}
		}
		if len(envFrom) > 0 {
			deployment := values["deployment"].(map[string]interface{})
			deployment["envFrom"] = envFrom
		}
	}

	// ── HTTPRoute (Gateway API) ─────────────────────
	if config.IngressEnabled {
		values["httpRoute"] = map[string]interface{}{
			"enabled": true,
			"parentRefs": []map[string]interface{}{
				{
					"name":      config.GatewayName,
					"namespace": config.GatewayNS,
				},
			},
			"hostnames": []string{config.IngressHostname},
			"rules": []map[string]interface{}{
				{
					"matches": []map[string]interface{}{
						{
							"path": map[string]interface{}{
								"type":  "PathPrefix",
								"value": config.IngressPath,
							},
						},
					},
					"backendRefs": []map[string]interface{}{
						{
							"name": config.Name,
							"port": config.Port,
						},
					},
				},
			},
		}
	}

	// ── ServiceMonitor ──────────────────────────────
	if config.MonitoringEnabled {
		values["serviceMonitor"] = map[string]interface{}{
			"enabled": true,
			"additionalLabels": map[string]string{
				"release": "kube-prometheus-stack",
			},
			"endpoints": []map[string]interface{}{
				{
					"port":     "http",
					"path":     config.MonitoringPath,
					"interval": config.MonitoringInterval,
				},
			},
		}
	}

	// ── Persistence ─────────────────────────────────
	if config.PersistenceEnabled {
		persistenceValues := map[string]interface{}{
			"enabled":     true,
			"mountPVC":    true,
			"mountPath":   config.PersistenceMountPath,
			"accessMode":  "ReadWriteOnce",
			"storageSize": config.PersistenceSize,
		}
		if config.PersistenceClass != "" {
			persistenceValues["storageClass"] = config.PersistenceClass
		}
		values["persistence"] = persistenceValues
	}

	// Disable everything we don't use
	values["ingress"] = map[string]interface{}{"enabled": false}
	values["route"] = map[string]interface{}{"enabled": false}
	values["forecastle"] = map[string]interface{}{"enabled": false}
	values["cronJob"] = map[string]interface{}{"enabled": false}
	values["job"] = map[string]interface{}{"enabled": false}
	values["configMap"] = map[string]interface{}{"enabled": false}
	values["sealedSecret"] = map[string]interface{}{"enabled": false}
	values["secret"] = map[string]interface{}{"enabled": false}
	values["certificate"] = map[string]interface{}{"enabled": false}
	values["secretProviderClass"] = map[string]interface{}{"enabled": false}
	values["alertmanagerConfig"] = map[string]interface{}{"enabled": false}
	values["prometheusRule"] = map[string]interface{}{"enabled": false}
	values["externalSecret"] = map[string]interface{}{"enabled": false}
	values["autoscaling"] = map[string]interface{}{"enabled": false}
	values["vpa"] = map[string]interface{}{"enabled": false}
	values["endpointMonitor"] = map[string]interface{}{"enabled": false}
	values["pdb"] = map[string]interface{}{"enabled": false}
	values["grafanaDashboard"] = map[string]interface{}{"enabled": false}
	values["backup"] = map[string]interface{}{"enabled": false}
	values["networkPolicy"] = map[string]interface{}{"enabled": false} // We render our own

	return values
}

// buildExternalSecrets creates ExternalSecret resources backed by 1Password.
func buildExternalSecrets(config *HTTPServiceConfig) []Resource {
	var resources []Resource

	for _, s := range config.Secrets {
		secretName := s.Name
		if secretName == "" {
			secretName = fmt.Sprintf("%s-%s", config.Name, s.OnePasswordItem)
		}

		data := []map[string]interface{}{}
		for _, k := range s.Keys {
			data = append(data, map[string]interface{}{
				"secretKey": k.SecretKey,
				"remoteRef": map[string]interface{}{
					"key":      s.OnePasswordItem,
					"property": k.Property,
				},
			})
		}

		es := Resource{
			APIVersion: "external-secrets.io/v1beta1",
			Kind:       "ExternalSecret",
			Metadata: ObjectMeta{
				Name:      secretName,
				Namespace: config.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kratix",
					"kratix.io/promise-name":       "http-service",
					"app.kubernetes.io/part-of":    config.Name,
				},
			},
			Spec: map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": config.SecretStoreName,
					"kind": config.SecretStoreKind,
				},
				"target": map[string]interface{}{
					"name": secretName,
				},
				"data": data,
			},
		}

		resources = append(resources, es)
	}

	return resources
}

// buildNetworkPolicies creates a default-deny + allow-ingress-from-gateway policy set.
func buildNetworkPolicies(config *HTTPServiceConfig) []Resource {
	var policies []Resource

	// 1. Default deny all ingress
	policies = append(policies, Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ObjectMeta{
			Name:      fmt.Sprintf("%s-default-deny", config.Name),
			Namespace: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "5",
			},
		},
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app.kubernetes.io/name": config.Name,
				},
			},
			"policyTypes": []string{"Ingress"},
		},
	})

	// 2. Allow ingress from the gateway namespace
	policies = append(policies, Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-gateway", config.Name),
			Namespace: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "5",
			},
		},
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app.kubernetes.io/name": config.Name,
				},
			},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": config.GatewayNS,
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{
							"protocol": "TCP",
							"port":     config.Port,
						},
					},
				},
			},
		},
	})

	// 3. Allow monitoring scrape if enabled
	if config.MonitoringEnabled {
		policies = append(policies, Resource{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
			Metadata: ObjectMeta{
				Name:      fmt.Sprintf("%s-allow-monitoring", config.Name),
				Namespace: config.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kratix",
					"kratix.io/promise-name":       "http-service",
					"app.kubernetes.io/part-of":    config.Name,
				},
				Annotations: map[string]string{
					"argocd.argoproj.io/sync-wave": "5",
				},
			},
			Spec: map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchLabels": map[string]string{
						"app.kubernetes.io/name": config.Name,
					},
				},
				"policyTypes": []string{"Ingress"},
				"ingress": []map[string]interface{}{
					{
						"from": []map[string]interface{}{
							{
								"namespaceSelector": map[string]interface{}{
									"matchLabels": map[string]string{
										"kubernetes.io/metadata.name": "monitoring",
									},
								},
							},
						},
						"ports": []map[string]interface{}{
							{
								"protocol": "TCP",
								"port":     config.Port,
							},
						},
					},
				},
			},
		})
	}

	// 4. Allow DNS egress (all pods need this)
	policies = append(policies, Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-dns", config.Name),
			Namespace: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "5",
			},
		},
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app.kubernetes.io/name": config.Name,
				},
			},
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"ports": []map[string]interface{}{
						{"protocol": "UDP", "port": 53},
						{"protocol": "TCP", "port": 53},
					},
				},
			},
		},
	})

	return policies
}
