#!/usr/bin/env bash
set -euo pipefail

: "${OP_CONNECT_HOST:?Missing OP_CONNECT_HOST}"
: "${OP_CONNECT_TOKEN:?Missing OP_CONNECT_TOKEN}"
: "${OP_VAULT_ID:?Missing OP_VAULT_ID}"
: "${OP_ITEM_NAME:=vcluster-test-kubeconfig}"

API_BASE="${OP_CONNECT_HOST%/}/v1"
AUTH_HEADER="Authorization: Bearer ${OP_CONNECT_TOKEN}"

KUBECONFIG_CONTENT="$(cat <<'EOF'
apiVersion: v1
kind: Config
clusters: []
contexts: []
current-context: ""
users: []
EOF
)"

ITEM_ID=$(curl -fsS -H "$AUTH_HEADER" "$API_BASE/vaults/$OP_VAULT_ID/items" | jq -r --arg title "$OP_ITEM_NAME" '.[] | select(.title==$title) | .id' | head -n1)

if [ -n "$ITEM_ID" ]; then
  echo "Item exists, replacing..."
  ITEM_PAYLOAD=$(jq -n --arg id "$ITEM_ID" --arg title "$OP_ITEM_NAME" --arg vault "$OP_VAULT_ID" --arg notes "$KUBECONFIG_CONTENT" '{id:$id,title:$title,vault:{id:$vault},category:"SECURE_NOTE",fields:[{label:"notesPlain",type:"STRING",purpose:"NOTES",value:$notes}]}')
  RESPONSE=$(curl -sS -w "\n%{http_code}" -X PUT -H "$AUTH_HEADER" -H "Content-Type: application/json" "$API_BASE/vaults/$OP_VAULT_ID/items/$ITEM_ID" -d "$ITEM_PAYLOAD")
else
  echo "Item not found, creating..."
  ITEM_PAYLOAD=$(jq -n --arg title "$OP_ITEM_NAME" --arg vault "$OP_VAULT_ID" --arg notes "$KUBECONFIG_CONTENT" '{title:$title,vault:{id:$vault},category:"SECURE_NOTE",fields:[{label:"notesPlain",type:"STRING",purpose:"NOTES",value:$notes}]}')
  RESPONSE=$(curl -sS -w "\n%{http_code}" -X POST -H "$AUTH_HEADER" -H "Content-Type: application/json" "$API_BASE/vaults/$OP_VAULT_ID/items" -d "$ITEM_PAYLOAD")
fi

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')
if [ "$HTTP_STATUS" -ge 400 ]; then
  echo "Request failed with status $HTTP_STATUS"
  echo "$BODY"
  exit 1
fi

if [ -z "$ITEM_ID" ]; then
  ITEM_ID=$(echo "$BODY" | jq -r '.id')
fi

echo "Success"
# if [ -n "$ITEM_ID" ]; then
#   DELETE_ID="$ITEM_ID"
# else
#   DELETE_ID=$(curl -fsS -H "$AUTH_HEADER" "$API_BASE/vaults/$OP_VAULT_ID/items" | jq -r --arg title "$OP_ITEM_NAME" '.[] | select(.title==$title) | .id' | head -n1)
# fi

# if [ -n "$DELETE_ID" ]; then
#   echo "Deleting item $DELETE_ID..."
#   HTTP_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X DELETE -H "$AUTH_HEADER" "$API_BASE/vaults/$OP_VAULT_ID/items/$DELETE_ID")
#   if [ "$HTTP_STATUS" != "204" ] && [ "$HTTP_STATUS" != "404" ]; then
#     echo "Delete failed with status $HTTP_STATUS"
#     exit 1
#   fi
# fi

