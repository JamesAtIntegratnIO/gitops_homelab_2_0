package main

import "fmt"

func buildKubeconfigExternalSecret(config *VClusterConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig",
	}, baseLabels(config, config.Name))

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
				Name: fmt.Sprintf("vcluster-%s-kubeconfig-external", config.Name),
				Template: &ExternalSecretTemplate{
					EngineVersion: "v2",
					Data: map[string]string{
						"config": "{{ .kubeconfig }}\n",
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

func buildKubeconfigSyncRBAC(config *VClusterConfig) []Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	externalSecret := Resource{
		APIVersion: "external-secrets.io/v1beta1",
		Kind:       "ExternalSecret",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-onepassword-token", config.Name),
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
				Name: fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
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
	}, baseLabels(config, config.Name))

	serviceAccount := Resource{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
	}

	role := Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		Rules: []PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{fmt.Sprintf("vc-%s", config.Name), fmt.Sprintf("vcluster-%s-onepassword-token", config.Name)},
				Verbs:         []string{"get"},
			},
		},
	}

	roleBinding := Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		RoleRef: &RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("%s-kubeconfig-sync", config.Name),
		},
		Subjects: []Subject{
			{
				Kind:      "ServiceAccount",
				Name:      fmt.Sprintf("%s-kubeconfig-sync", config.Name),
				Namespace: config.TargetNamespace,
			},
		},
	}

	return []Resource{externalSecret, serviceAccount, role, roleBinding}
}

func buildKubeconfigSyncJob(config *VClusterConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	initCommand := fmt.Sprintf(`echo "Waiting for secret vc-%s to exist..."
until [ -f /kubeconfig/config ]; do
  echo "Kubeconfig not found yet, sleeping..."
  sleep 5
done
echo "Kubeconfig found!"`, config.Name)

	syncCommand := `set -e

apk add --no-cache curl jq >/dev/null 2>&1

echo "=== VCluster Kubeconfig Sync to 1Password ==="
echo "VCluster: $VCLUSTER_NAME"
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
KUBECONFIG_CONTENT=$(cat /kubeconfig/config)

# Extract TLS data from kubeconfig for ArgoCD cluster config
CA_DATA=$(grep 'certificate-authority-data:' /kubeconfig/config | awk '{print $2}' | tr -d '\r\n' | head -n1)
CLIENT_CERT=$(grep 'client-certificate-data:' /kubeconfig/config | awk '{print $2}' | tr -d '\r\n' | head -n1)
CLIENT_KEY=$(grep 'client-key-data:' /kubeconfig/config | awk '{print $2}' | tr -d '\r\n' | head -n1)

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
      \"tags\": [\"vcluster\", \"kubeconfig\", \"$ARGOCD_ENVIRONMENT\"],
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
          \"value\": \"$VCLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
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
      \"tags\": [\"vcluster\", \"kubeconfig\", \"$ARGOCD_ENVIRONMENT\"],
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
          \"value\": \"$VCLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
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
		Metadata: resourceMeta(
			config.KubeconfigSyncJobName,
			config.TargetNamespace,
			labels,
			nil,
		),
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
					ServiceAccountName: fmt.Sprintf("%s-kubeconfig-sync", config.Name),
					RestartPolicy:      "OnFailure",
					InitContainers: []Container{
						{
							Name:    "wait-for-kubeconfig",
							Image:   "busybox:1.36",
							Command: []string{"sh", "-c", initCommand},
							VolumeMounts: []VolumeMount{
								{
									Name:      "kubeconfig",
									MountPath: "/kubeconfig",
								},
							},
						},
					},
					Containers: []Container{
						{
							Name:  "sync-to-onepassword",
							Image: "alpine:3.20",
							Env: []EnvVar{
								{Name: "OP_CONNECT_HOST", Value: "https://connect.integratn.tech"},
								{
									Name: "OP_CONNECT_TOKEN",
									ValueFrom: &EnvVarSource{
										SecretKeyRef: &SecretKeySelector{
											Name: fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											Key:  "token",
										},
									},
								},
								{
									Name: "OP_VAULT",
									ValueFrom: &EnvVarSource{
										SecretKeyRef: &SecretKeySelector{
											Name: fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											Key:  "vault",
										},
									},
								},
								{Name: "VCLUSTER_NAME", Value: config.Name},
								{Name: "OP_ITEM_NAME", Value: config.OnePasswordItem},
								{Name: "BASE_DOMAIN", Value: config.BaseDomain},
								{Name: "BASE_DOMAIN_SANITIZED", Value: config.BaseDomainSanitized},
								{Name: "EXTERNAL_SERVER_URL", Value: config.ExternalServerURL},
								{Name: "ARGOCD_ENVIRONMENT", Value: config.ArgoCDEnvironment},
							},
							Command: []string{"sh", "-c", syncCommand},
							VolumeMounts: []VolumeMount{
								{
									Name:      "kubeconfig",
									MountPath: "/kubeconfig",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []Volume{
						{
							Name: "kubeconfig",
							Secret: &SecretVolume{
								SecretName: fmt.Sprintf("vc-%s", config.Name),
								Optional:   false,
							},
						},
					},
				},
			},
		},
	}
}
