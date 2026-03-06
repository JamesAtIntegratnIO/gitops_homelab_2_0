set -e
echo "Waiting for certificates to be ready..."

# Wait for CA cert
until kubectl get secret {{NAME}}-etcd-ca -n {{NS}} 2>/dev/null; do
  echo "Waiting for CA certificate..."
  sleep 2
done

# Wait for server cert
until kubectl get secret {{NAME}}-etcd-server -n {{NS}} 2>/dev/null; do
  echo "Waiting for server certificate..."
  sleep 2
done

# Wait for peer cert
until kubectl get secret {{NAME}}-etcd-peer -n {{NS}} 2>/dev/null; do
  echo "Waiting for peer certificate..."
  sleep 2
done

echo "All certificates ready, merging..."

# Extract certs
CA_CRT=$(kubectl get secret {{NAME}}-etcd-ca -n {{NS}} -o jsonpath='{.data.tls\.crt}')
SERVER_CRT=$(kubectl get secret {{NAME}}-etcd-server -n {{NS}} -o jsonpath='{.data.tls\.crt}')
SERVER_KEY=$(kubectl get secret {{NAME}}-etcd-server -n {{NS}} -o jsonpath='{.data.tls\.key}')
PEER_CRT=$(kubectl get secret {{NAME}}-etcd-peer -n {{NS}} -o jsonpath='{.data.tls\.crt}')
PEER_KEY=$(kubectl get secret {{NAME}}-etcd-peer -n {{NS}} -o jsonpath='{.data.tls\.key}')

# Create merged secret
kubectl create secret generic {{NAME}}-etcd-certs -n {{NS}} \
  --from-literal=etcd-ca.crt="$(echo $CA_CRT | base64 -d)" \
  --from-literal=etcd-server.crt="$(echo $SERVER_CRT | base64 -d)" \
  --from-literal=etcd-server.key="$(echo $SERVER_KEY | base64 -d)" \
  --from-literal=etcd-peer.crt="$(echo $PEER_CRT | base64 -d)" \
  --from-literal=etcd-peer.key="$(echo $PEER_KEY | base64 -d)" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Certificate merge complete!"
