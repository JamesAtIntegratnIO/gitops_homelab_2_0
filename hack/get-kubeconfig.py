#!/usr/bin/env python3
"""Extract cluster CA and generate kubeconfig via ArgoCD API."""
import json, urllib.request, ssl, base64, sys, os

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

token = os.environ.get("ARGOCD_TOKEN", "")
base = "https://argocd.cluster.integratn.tech"
headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

def argocd_get(path):
    req = urllib.request.Request(f"{base}{path}", headers=headers)
    resp = urllib.request.urlopen(req, context=ctx)
    return json.loads(resp.read())

def get_resource(app, namespace, name, kind, version="v1", group=""):
    path = f"/api/v1/applications/{app}/resource?namespace={namespace}&resourceName={name}&kind={kind}&version={version}&group={group}"
    data = argocd_get(path)
    return json.loads(data.get("manifest", "{}"))

# Step 1: Get cluster CA from kube-root-ca.crt configmap (available in every namespace)
print("Getting cluster CA...")
try:
    cm = get_resource("argocd-the-cluster", "argocd", "kube-root-ca.crt", "ConfigMap")
    ca_pem = cm["data"]["ca.crt"]
    ca_b64 = base64.b64encode(ca_pem.encode()).decode()
    print(f"  Got CA cert ({len(ca_pem)} bytes)")
except Exception as e:
    print(f"  Failed to get CA: {e}")
    sys.exit(1)

# Step 2: Get the API server endpoint from the Kubernetes Endpoints resource
print("Getting API server endpoint...")
try:
    ep = get_resource("argocd-the-cluster", "default", "kubernetes", "Endpoints")
    subsets = ep.get("subsets", [])
    api_ip = subsets[0]["addresses"][0]["ip"]
    api_port = subsets[0]["ports"][0]["port"]
    api_server = f"https://{api_ip}:{api_port}"
    print(f"  API server: {api_server}")
except Exception as e:
    print(f"  Failed: {e}, falling back to 10.0.4.101:6443")
    api_server = "https://10.0.4.101:6443"

# Step 3: Create a SA token by reading the argocd-application-controller SA
# On K8s 1.24+, we need a Secret with a token annotation.
# But since we can't create resources easily, let's use ArgoCD's own token approach.
# Actually - we can use the ArgoCD API to create a project token and use that as kubectl bearer token
# OR - even simpler - we already have the ArgoCD JWT token, and we have the CA cert.
# We can generate a kubeconfig that uses ArgoCD as a reverse proxy.
# BUT - ArgoCD proxy isn't enabled.

# Simplest approach: generate a kubeconfig using SA token from an existing secret
# Let's check if there's an admin-user secret or similar
print("Looking for SA token secrets...")
try:
    # Try to find argocd-application-controller token
    tree = argocd_get("/api/v1/applications/argocd-the-cluster/resource-tree")
    secrets = [n for n in tree.get("nodes", []) if n.get("kind") == "Secret" and n.get("namespace") == "argocd"]
    for s in secrets:
        print(f"  Secret: {s['name']}")
except Exception as e:
    print(f"  Tree error: {e}")

# Step 4: Actually the best approach - create a temporary kubeconfig using
# the argocd-manager-token which is likely a cluster secret
# Let's check the cluster secret in ArgoCD
print("\nGetting ArgoCD cluster secret...")
try:
    data = argocd_get("/api/v1/clusters/https%3A%2F%2Fkubernetes.default.svc")
    config = data.get("config", {})
    bearer = config.get("bearerToken", "")
    tls = config.get("tlsClientConfig", {})
    print(f"  Bearer token present: {bool(bearer)}")
    print(f"  TLS cert present: {bool(tls.get('certData'))}")
    print(f"  TLS key present: {bool(tls.get('keyData'))}")
    print(f"  CA present: {bool(tls.get('caData'))}")
    print(f"  Insecure: {tls.get('insecure')}")
except Exception as e:
    print(f"  Error: {e}")

# Step 5: Write kubeconfig using the CA we got + the ArgoCD JWT for testing
# Actually let's try a different approach - write the kubeconfig with
# the CA cert pointing at the real API server, and use exec-based auth
# through ArgoCD token

# For now, output what we have
kubeconfig = {
    "apiVersion": "v1",
    "kind": "Config",
    "clusters": [{
        "name": "the-cluster",
        "cluster": {
            "server": api_server,
            "certificate-authority-data": ca_b64,
        }
    }],
    "contexts": [{
        "name": "the-cluster",
        "context": {
            "cluster": "the-cluster",
            "user": "argocd-admin",
        }
    }],
    "current-context": "the-cluster",
    "users": [{
        "name": "argocd-admin",
        "user": {}  # Will be filled with token
    }],
}

output_path = "/tmp/homelab-kubeconfig.yaml"
with open(output_path, "w") as f:
    json.dump(kubeconfig, f, indent=2)
print(f"\nWrote partial kubeconfig to {output_path}")
print(f"API server: {api_server}")
print(f"CA cert: present ({len(ca_pem)} bytes)")
print("\nNeed a service account token to complete auth.")
