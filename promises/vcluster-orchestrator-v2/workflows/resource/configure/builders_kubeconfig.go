package main

import "fmt"

func buildKubeconfigExternalSecret(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig",
	}, baseLabels(config, config.Name))

	return map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s-kubeconfig-external", config.Name),
				"template": map[string]interface{}{
					"engineVersion": "v2",
					"data": map[string]string{
						"config": "{{ .kubeconfig }}\n",
					},
				},
			},
			"dataFrom": []map[string]interface{}{
				{
					"extract": map[string]interface{}{
						"key": config.OnePasswordItem,
					},
				},
			},
			"refreshInterval": "15m",
		},
	}
}

func buildKubeconfigSyncRBAC(config *VClusterConfig) []interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	externalSecret := map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-onepassword-token", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
			},
			"data": []map[string]interface{}{
				{
					"secretKey": "token",
					"remoteRef": map[string]interface{}{
						"key":      "onepassword-access-token",
						"property": "credential",
					},
				},
				{
					"secretKey": "vault",
					"remoteRef": map[string]interface{}{
						"key":      "onepassword-access-token",
						"property": "vault",
					},
				},
			},
		},
	}

	baseRBACLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	serviceAccount := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
	}

	role := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "Role",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		"rules": []map[string]interface{}{
			{
				"apiGroups":     []string{""},
				"resources":     []string{"secrets"},
				"resourceNames": []string{fmt.Sprintf("vc-%s", config.Name), fmt.Sprintf("vcluster-%s-onepassword-token", config.Name)},
				"verbs":         []string{"get"},
			},
		},
	}

	roleBinding := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "RoleBinding",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "Role",
			"name":     fmt.Sprintf("%s-kubeconfig-sync", config.Name),
		},
		"subjects": []map[string]interface{}{
			{
				"kind":      "ServiceAccount",
				"name":      fmt.Sprintf("%s-kubeconfig-sync", config.Name),
				"namespace": config.TargetNamespace,
			},
		},
	}

	return []interface{}{externalSecret, serviceAccount, role, roleBinding}
}

func buildKubeconfigSyncJob(config *VClusterConfig) map[string]interface{} {
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

	return map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": resourceMeta(
			config.KubeconfigSyncJobName,
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"backoffLimit":            3,
			"ttlSecondsAfterFinished": 600,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app.kubernetes.io/name":     "kubeconfig-sync",
						"app.kubernetes.io/instance": config.Name,
					},
				},
				"spec": map[string]interface{}{
					"serviceAccountName": fmt.Sprintf("%s-kubeconfig-sync", config.Name),
					"restartPolicy":      "OnFailure",
					"initContainers": []map[string]interface{}{
						{
							"name":    "wait-for-kubeconfig",
							"image":   "busybox:1.36",
							"command": []string{"sh", "-c", initCommand},
							"volumeMounts": []map[string]interface{}{
								{
									"name":      "kubeconfig",
									"mountPath": "/kubeconfig",
								},
							},
						},
					},
					"containers": []map[string]interface{}{
						{
							"name":  "sync-to-onepassword",
							"image": "alpine:3.20",
							"env": []map[string]interface{}{
								{"name": "OP_CONNECT_HOST", "value": "https://connect.integratn.tech"},
								{
									"name": "OP_CONNECT_TOKEN",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											"key":  "token",
										},
									},
								},
								{
									"name": "OP_VAULT",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											"key":  "vault",
										},
									},
								},
								{"name": "VCLUSTER_NAME", "value": config.Name},
								{"name": "OP_ITEM_NAME", "value": config.OnePasswordItem},
								{"name": "BASE_DOMAIN", "value": config.BaseDomain},
								{"name": "BASE_DOMAIN_SANITIZED", "value": config.BaseDomainSanitized},
								{"name": "EXTERNAL_SERVER_URL", "value": config.ExternalServerURL},
								{"name": "ARGOCD_ENVIRONMENT", "value": config.ArgoCDEnvironment},
							},
							"command": []string{"sh", "-c", syncCommand},
							"volumeMounts": []map[string]interface{}{
								{
									"name":      "kubeconfig",
									"mountPath": "/kubeconfig",
									"readOnly":  true,
								},
							},
						},
					},
					"volumes": []map[string]interface{}{
						{
							"name": "kubeconfig",
							"secret": map[string]interface{}{
								"secretName": fmt.Sprintf("vc-%s", config.Name),
								"optional":   false,
							},
						},
					},
				},
			},
		},
	}
}
