package main

import "fmt"

// ============================================================================
// Stakater Values Builder
// ============================================================================

// buildSecurityContext creates the container security context from config.
func buildSecurityContext(config *HTTPServiceConfig) map[string]interface{} {
	ctx := map[string]interface{}{}

	if config.RunAsNonRoot != nil {
		ctx["runAsNonRoot"] = *config.RunAsNonRoot
	} else {
		ctx["runAsNonRoot"] = false
	}
	if config.ReadOnlyRootFilesystem != nil {
		ctx["readOnlyRootFilesystem"] = *config.ReadOnlyRootFilesystem
	} else {
		ctx["readOnlyRootFilesystem"] = false
	}
	if config.RunAsUser != nil {
		ctx["runAsUser"] = *config.RunAsUser
	}
	if config.RunAsGroup != nil {
		ctx["runAsGroup"] = *config.RunAsGroup
	}

	return ctx
}

// buildDeploymentValues constructs the deployment section of the Stakater Helm values.
func buildDeploymentValues(config *HTTPServiceConfig) map[string]interface{} {
	return map[string]interface{}{
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
		"containerSecurityContext": buildSecurityContext(config),
		"revisionHistoryLimit":    3,
		"reloadOnChange":          true,
	}
}

// buildServiceValues constructs the service section of the Stakater Helm values.
func buildServiceValues(config *HTTPServiceConfig) map[string]interface{} {
	return map[string]interface{}{
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
	}
}

// addEnvValues mutates the deployment map to add env vars and envFrom entries.
func addEnvValues(config *HTTPServiceConfig, deployment map[string]interface{}) {
	if len(config.Env) > 0 {
		envMap := map[string]interface{}{}
		for k, v := range config.Env {
			envMap[k] = map[string]interface{}{
				"value": v,
			}
		}
		deployment["env"] = envMap
	}

	if len(config.EnvFromSecrets) > 0 || len(config.Secrets) > 0 {
		envFrom := map[string]interface{}{}
		for _, s := range config.EnvFromSecrets {
			envFrom[s] = map[string]interface{}{
				"type":       "secret",
				"nameSuffix": s,
			}
		}
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
			deployment["envFrom"] = envFrom
		}
	}
}

// buildStakaterValues constructs the Helm values for the Stakater application chart.
// HTTPRoute is DISABLED here — the gateway-route sub-promise owns routing.
func buildStakaterValues(config *HTTPServiceConfig) map[string]interface{} {
	deployment := buildDeploymentValues(config)
	addEnvValues(config, deployment)

	values := map[string]interface{}{
		"applicationName": config.Name,

		"additionalLabels": map[string]string{
			"app.kubernetes.io/managed-by": "kratix",
			"kratix.io/promise-name":       "http-service",
			"app.kubernetes.io/part-of":    config.Name,
			"app.kubernetes.io/team":       config.Team,
		},

		"deployment": deployment,
		"service":    buildServiceValues(config),

		"rbac": map[string]interface{}{
			"enabled": true,
			"serviceAccount": map[string]interface{}{
				"enabled": true,
				"name":    config.Name,
			},
		},
	}

	// ── HTTPRoute DISABLED — owned by gateway-route sub-promise ──
	values["httpRoute"] = map[string]interface{}{"enabled": false}

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
	for _, feature := range []string{
		"ingress", "route", "forecastle", "cronJob", "job", "configMap",
		"sealedSecret", "secret", "certificate", "secretProviderClass",
		"alertmanagerConfig", "prometheusRule", "externalSecret",
		"autoscaling", "vpa", "endpointMonitor", "pdb",
		"grafanaDashboard", "backup", "networkPolicy",
	} {
		values[feature] = map[string]interface{}{"enabled": false}
	}

	return values
}
