package main

import "fmt"

func buildKubeconfigExternalSecret(config *RegistrationConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig",
	}, baseLabels(config))

	return Resource{
		APIVersion: "external-secrets.io/v1beta1",
		Kind:       "ExternalSecret",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-kubeconfig", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		Spec: ExternalSecretSpec{
			SecretStoreRef: SecretStoreRef{
				Name: "onepassword-store",
				Kind: "ClusterSecretStore",
			},
			Target: ExternalSecretTarget{
				Name: fmt.Sprintf("%s-kubeconfig-external", config.Name),
				Template: &ExternalSecretTemplate{
					EngineVersion: "v2",
					Data: map[string]string{
						config.KubeconfigKey: "{{ .kubeconfig }}\n",
					},
				},
			},
			DataFrom: []ExternalSecretDataFrom{
				{
					Extract: &ExternalSecretExtract{
						Key: config.OnePasswordItem,
					},
				},
			},
			RefreshInterval: "15m",
		},
	}
}

func buildKubeconfigSyncRBAC(config *RegistrationConfig) []Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig-sync",
	}, baseLabels(config))

	onePasswordTokenName := fmt.Sprintf("%s-onepassword-token", config.Name)

	externalSecret := Resource{
		APIVersion: "external-secrets.io/v1beta1",
		Kind:       "ExternalSecret",
		Metadata: resourceMeta(
			onePasswordTokenName,
			config.TargetNamespace,
			labels,
			nil,
		),
		Spec: ExternalSecretSpec{
			SecretStoreRef: SecretStoreRef{
				Name: "onepassword-store",
				Kind: "ClusterSecretStore",
			},
			Target: ExternalSecretTarget{
				Name: onePasswordTokenName,
			},
			Data: []ExternalSecretData{
				{
					SecretKey: "token",
					RemoteRef: RemoteRef{
						Key:      "onepassword-access-token",
						Property: "credential",
					},
				},
				{
					SecretKey: "vault",
					RemoteRef: RemoteRef{
						Key:      "onepassword-access-token",
						Property: "vault",
					},
				},
			},
		},
	}

	baseRBACLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config))

	saName := fmt.Sprintf("%s-kubeconfig-sync", config.Name)

	serviceAccount := Resource{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata:   resourceMeta(saName, config.TargetNamespace, baseRBACLabels, nil),
	}

	role := Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata:   resourceMeta(saName, config.TargetNamespace, baseRBACLabels, nil),
		Rules: []PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{config.KubeconfigSecret, onePasswordTokenName},
				Verbs:         []string{"get"},
			},
		},
	}

	roleBinding := Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata:   resourceMeta(saName, config.TargetNamespace, baseRBACLabels, nil),
		RoleRef: &RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     saName,
		},
		Subjects: []Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: config.TargetNamespace,
			},
		},
	}

	return []Resource{externalSecret, serviceAccount, role, roleBinding}
}

func buildKubeconfigSyncJob(config *RegistrationConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config))

	saName := fmt.Sprintf("%s-kubeconfig-sync", config.Name)
	onePasswordTokenName := fmt.Sprintf("%s-onepassword-token", config.Name)

	initCommand := fmt.Sprintf(`echo "Waiting for kubeconfig secret to be mounted..."
until [ -f /kubeconfig/%s ]; do
  echo "Kubeconfig not found yet, sleeping..."
  sleep 5
done
echo "Kubeconfig found!"`, config.KubeconfigKey)

	syncCommand := `set -e

apk add --no-cache curl jq >/dev/null 2>&1

echo "=== Kubeconfig Sync to 1Password ==="
echo "Cluster: $CLUSTER_NAME"
echo "1Password Item: $OP_ITEM_NAME"
echo "Vault: $OP_VAULT"

OP_CONNECT_HOST_CLEAN=$(printf '%s' "$OP_CONNECT_HOST" | tr -d '\r\n')
OP_CONNECT_TOKEN_CLEAN=$(printf '%s' "$OP_CONNECT_TOKEN" | tr -d '\r\n')
API_BASE="${OP_CONNECT_HOST_CLEAN%/}/v1"
AUTH_HEADER="Authorization: Bearer ${OP_CONNECT_TOKEN_CLEAN}"

VAULT_ID="${OP_VAULT:-}"
VAULT_ID=$(printf '%s' "$VAULT_ID" | tr -d '\r\n')
if [ -z "$VAULT_ID" ]; then
	VAULT_ID=$(curl -fsS -H "$AUTH_HEADER" "$API_BASE/vaults" | jq -r --arg name "homelab" '.[] | select(.name==$name) | .id' | head -n1)
fi
VAULT_ID=$(printf '%s' "$VAULT_ID" | tr -d '\r\n')
if [ -z "$VAULT_ID" ]; then
	echo "Vault not found: homelab"
	exit 1
fi

# Read kubeconfig from secret
KUBECONFIG_CONTENT=$(cat /kubeconfig/$KUBECONFIG_KEY)

# Extract TLS data from kubeconfig for ArgoCD cluster config
CA_DATA=$(grep 'certificate-authority-data:' /kubeconfig/$KUBECONFIG_KEY | awk '{print $2}' | tr -d '\r\n' | head -n1)
CLIENT_CERT=$(grep 'client-certificate-data:' /kubeconfig/$KUBECONFIG_KEY | awk '{print $2}' | tr -d '\r\n' | head -n1)
CLIENT_KEY=$(grep 'client-key-data:' /kubeconfig/$KUBECONFIG_KEY | awk '{print $2}' | tr -d '\r\n' | head -n1)

# Build ArgoCD cluster config with TLS client certificates
if [ -n "$CA_DATA" ] && [ -n "$CLIENT_CERT" ] && [ -n "$CLIENT_KEY" ]; then
  ARGOCD_CONFIG=$(printf '{"tlsClientConfig":{"insecure":false,"caData":"%s","certData":"%s","keyData":"%s"}}' "$CA_DATA" "$CLIENT_CERT" "$CLIENT_KEY")
  echo "ArgoCD config built with TLS certs (caData=${#CA_DATA} chars, certData=${#CLIENT_CERT} chars, keyData=${#CLIENT_KEY} chars)"
else
  echo "WARNING: Could not extract TLS data from kubeconfig, falling back to insecure"
  ARGOCD_CONFIG='{"tlsClientConfig":{"insecure":true}}'
fi

# Check if item exists
echo "Checking if item exists..."
ITEM_ID=$(curl -fsS -H "$AUTH_HEADER" "$API_BASE/vaults/$VAULT_ID/items" | jq -r --arg title "$OP_ITEM_NAME" '.[] | select(.title==$title) | .id' | head -n1)

if [ -z "$ITEM_ID" ]; then
  echo "Creating new 1Password item..."
  RESPONSE=$(curl -sS -w "\n%{http_code}" -X POST "$API_BASE/vaults/$VAULT_ID/items" \
    -H "$AUTH_HEADER" \
    -H "Content-Type: application/json" \
    -d "{
      \"title\": \"$OP_ITEM_NAME\",
      \"vault\": {\"id\": \"$VAULT_ID\"},
      \"category\": \"SERVER\",
      \"tags\": [\"cluster\", \"kubeconfig\", \"$ARGOCD_ENVIRONMENT\"],
      \"fields\": [
        {
          \"id\": \"kubeconfig\",
          \"type\": \"CONCEALED\",
          \"label\": \"kubeconfig\",
          \"value\": $(printf '%s' "$KUBECONFIG_CONTENT" | jq -Rs .)
        },
        {
          \"id\": \"argocd-name\",
          \"type\": \"STRING\",
          \"label\": \"argocd-name\",
          \"value\": \"$CLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
        },
        {
          \"id\": \"argocd-server\",
          \"type\": \"STRING\",
          \"label\": \"argocd-server\",
          \"value\": \"$EXTERNAL_SERVER_URL\"
        },
        {
          \"id\": \"argocd-config\",
          \"type\": \"CONCEALED\",
          \"label\": \"argocd-config\",
          \"value\": $(printf '%s' "$ARGOCD_CONFIG" | jq -Rc .)
        }
      ]
    }")
  HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  if [ "$HTTP_STATUS" -ge 400 ]; then
    echo "Failed to create 1Password item (HTTP $HTTP_STATUS): $BODY"
    exit 1
  fi
  ITEM_ID=$(echo "$BODY" | jq -r '.id')
  if [ -z "$ITEM_ID" ] || [ "$ITEM_ID" = "null" ]; then
    echo "Failed to extract item ID from response"
    exit 1
  fi
  echo "Created item with ID: $ITEM_ID"
else
  echo "Updating existing item ID: $ITEM_ID"
  RESPONSE=$(curl -sS -w "\n%{http_code}" -X PUT "$API_BASE/vaults/$VAULT_ID/items/$ITEM_ID" \
    -H "$AUTH_HEADER" \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"$ITEM_ID\",
      \"title\": \"$OP_ITEM_NAME\",
      \"vault\": {\"id\": \"$VAULT_ID\"},
      \"category\": \"SERVER\",
      \"tags\": [\"cluster\", \"kubeconfig\", \"$ARGOCD_ENVIRONMENT\"],
      \"fields\": [
        {
          \"id\": \"kubeconfig\",
          \"type\": \"CONCEALED\",
          \"label\": \"kubeconfig\",
          \"value\": $(printf '%s' "$KUBECONFIG_CONTENT" | jq -Rs .)
        },
        {
          \"id\": \"argocd-name\",
          \"type\": \"STRING\",
          \"label\": \"argocd-name\",
          \"value\": \"$CLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
        },
        {
          \"id\": \"argocd-server\",
          \"type\": \"STRING\",
          \"label\": \"argocd-server\",
          \"value\": \"$EXTERNAL_SERVER_URL\"
        },
        {
          \"id\": \"argocd-config\",
          \"type\": \"CONCEALED\",
          \"label\": \"argocd-config\",
          \"value\": $(printf '%s' "$ARGOCD_CONFIG" | jq -Rc .)
        }
      ]
    }")
  HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  if [ "$HTTP_STATUS" -ge 400 ]; then
    echo "Failed to update 1Password item (HTTP $HTTP_STATUS): $BODY"
    exit 1
  fi
  echo "Updated item successfully"
fi

echo "✓ Kubeconfig synced to 1Password successfully"`

	return Resource{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata:   resourceMeta(config.SyncJobName, config.TargetNamespace, labels, nil),
		Spec: JobSpec{
			BackoffLimit:            3,
			TTLSecondsAfterFinished: 600,
			Template: PodTemplateSpec{
				Metadata: &ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "kubeconfig-sync",
						"app.kubernetes.io/instance": config.Name,
					},
				},
				Spec: PodSpec{
					ServiceAccountName: saName,
					RestartPolicy:      "OnFailure",
					InitContainers: []Container{
						{
							Name:    "wait-for-kubeconfig",
							Image:   "busybox:1.36",
							Command: []string{"sh", "-c", initCommand},
							VolumeMounts: []VolumeMount{
								{Name: "kubeconfig", MountPath: "/kubeconfig"},
							},
						},
					},
					Containers: []Container{
						{
							Name:  "sync-to-onepassword",
							Image: "alpine:3.20",
							Env: []EnvVar{
								{Name: "OP_CONNECT_HOST", Value: config.OnePasswordConnectHost},
								{
									Name: "OP_CONNECT_TOKEN",
									ValueFrom: &EnvVarSource{
										SecretKeyRef: &SecretKeySelector{
											Name: onePasswordTokenName,
											Key:  "token",
										},
									},
								},
								{
									Name: "OP_VAULT",
									ValueFrom: &EnvVarSource{
										SecretKeyRef: &SecretKeySelector{
											Name: onePasswordTokenName,
											Key:  "vault",
										},
									},
								},
								{Name: "CLUSTER_NAME", Value: config.Name},
								{Name: "KUBECONFIG_KEY", Value: config.KubeconfigKey},
								{Name: "OP_ITEM_NAME", Value: config.OnePasswordItem},
								{Name: "BASE_DOMAIN", Value: config.BaseDomain},
								{Name: "BASE_DOMAIN_SANITIZED", Value: config.BaseDomainSanitized},
								{Name: "EXTERNAL_SERVER_URL", Value: config.ExternalServerURL},
								{Name: "ARGOCD_ENVIRONMENT", Value: config.Environment},
							},
							Command: []string{"sh", "-c", syncCommand},
							VolumeMounts: []VolumeMount{
								{Name: "kubeconfig", MountPath: "/kubeconfig", ReadOnly: true},
							},
						},
					},
					Volumes: []Volume{
						{
							Name: "kubeconfig",
							Secret: &SecretVolume{
								SecretName: config.KubeconfigSecret,
								Optional:   false,
							},
						},
					},
				},
			},
		},
	}
}

func buildArgoCDClusterExternalSecret(config *RegistrationConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":         "external-secret",
		"app.kubernetes.io/component":    "argocd-cluster",
		"argocd.argoproj.io/secret-type": "cluster",
	}, baseLabels(config))
	if config.ClusterLabels != nil {
		labels = mergeStringMap(labels, config.ClusterLabels)
	}

	metadataAnnotations := map[string]string{}
	if len(config.ClusterAnnotations) > 0 {
		metadataAnnotations = mergeStringMap(metadataAnnotations, config.ClusterAnnotations)
	}

	targetLabels := mergeStringMap(map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"integratn.tech/cluster-name":    config.Name,
		"integratn.tech/environment":     config.Environment,
	}, config.ClusterLabels)

	targetAnnotations := map[string]string{}
	if len(config.ClusterAnnotations) > 0 {
		targetAnnotations = mergeStringMap(targetAnnotations, config.ClusterAnnotations)
	}

	tmplMeta := &TemplateMetadata{
		Labels: targetLabels,
	}
	if len(targetAnnotations) > 0 {
		tmplMeta.Annotations = targetAnnotations
	}

	esName := fmt.Sprintf("%s-argocd-cluster", config.Name)
	if len(metadataAnnotations) == 0 {
		metadataAnnotations = nil
	}

	return Resource{
		APIVersion: "external-secrets.io/v1beta1",
		Kind:       "ExternalSecret",
		Metadata:   resourceMeta(esName, "argocd", labels, metadataAnnotations),
		Spec: ExternalSecretSpec{
			SecretStoreRef: SecretStoreRef{
				Name: "onepassword-store",
				Kind: "ClusterSecretStore",
			},
			Target: ExternalSecretTarget{
				Name: fmt.Sprintf("cluster-%s", config.Name),
				Template: &ExternalSecretTemplate{
					EngineVersion: "v2",
					Type:          "Opaque",
					Metadata:      tmplMeta,
					Data: map[string]string{
						"name":   "{{ index . \"argocd-name\" }}",
						"server": "{{ index . \"argocd-server\" }}",
						"config": "{{ index . \"argocd-config\" }}",
					},
				},
			},
			DataFrom: []ExternalSecretDataFrom{
				{
					Extract: &ExternalSecretExtract{
						Key:                config.OnePasswordItem,
						ConversionStrategy: "Default",
						DecodingStrategy:   "None",
					},
				},
			},
			RefreshInterval: "15m",
		},
	}
}
