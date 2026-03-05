set -e

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

echo "✓ Kubeconfig synced to 1Password successfully"
