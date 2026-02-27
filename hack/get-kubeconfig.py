#!/usr/bin/env python3
"""Extract kubeconfig from ArgoCD cluster credentials.

Reads the ArgoCD CLI auth token from ~/.config/argocd/config (you just need
to be logged in via `argocd login`) and pulls the cluster CA + bearer token
from the ArgoCD API to produce a working kubeconfig.

Usage:
    python3 hack/get-kubeconfig.py                     # writes to /tmp/homelab-kubeconfig.yaml
    python3 hack/get-kubeconfig.py /path/to/output.yaml
"""
import json, urllib.request, ssl, base64, sys, os, re, subprocess, shutil, time
from pathlib import Path

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
ARGOCD_SERVER = "argocd.cluster.integratn.tech"
ARGOCD_BASE = f"https://{ARGOCD_SERVER}"
CLUSTER_URL_ENCODED = "https%3A%2F%2Fkubernetes.default.svc"  # in-cluster URL
CLUSTER_NAME = "the-cluster"
FALLBACK_API = "https://10.0.4.101:6443"
OUTPUT_DEFAULT = "/tmp/homelab-kubeconfig.yaml"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE


def _read_token_from_config() -> str:
    """Read the raw auth token string from the ArgoCD CLI config file."""
    config_path = Path.home() / ".config" / "argocd" / "config"
    if not config_path.exists():
        return ""

    text = config_path.read_text()

    # The token may be split across lines in the YAML file. Grab everything
    # after "auth-token:" and concatenate continuation lines (indented or bare
    # continuation of a long value).
    match = re.search(r"auth-token:\s*(\S.*)", text)
    if not match:
        return ""

    # Start with the first part on the auth-token line
    token_parts = [match.group(1).strip()]

    # Find position right after the match and look for continuation lines
    rest = text[match.end():]
    for line in rest.splitlines():
        stripped = line.strip()
        # If the line is indented and doesn't contain a YAML key (no ": "),
        # or starts with characters that look like a JWT continuation
        if stripped and not stripped.startswith("-") and ": " not in stripped and ":" not in stripped:
            token_parts.append(stripped)
        else:
            break

    return "".join(token_parts)


def _jwt_expiry(token: str) -> float | None:
    """Decode a JWT payload and return the 'exp' claim, or None."""
    try:
        payload_b64 = token.split(".")[1]
        # Add padding
        payload_b64 += "=" * (-len(payload_b64) % 4)
        payload = json.loads(base64.urlsafe_b64decode(payload_b64))
        return payload.get("exp")
    except Exception:
        return None


def _token_is_expired(token: str) -> bool:
    """Return True if the JWT token is expired (or unparseable)."""
    exp = _jwt_expiry(token)
    if exp is None:
        return False  # Can't determine — assume it's fine
    return time.time() > exp


def _refresh_token() -> str:
    """Run `argocd login` interactively to refresh the token."""
    argocd_bin = shutil.which("argocd")
    if not argocd_bin:
        print("  ✗ argocd CLI not found in PATH — cannot refresh token")
        print(f"    Install argocd CLI or run: argocd login --grpc-web {ARGOCD_SERVER}")
        sys.exit(1)

    print(f"  → Running: argocd login --grpc-web {ARGOCD_SERVER} --sso --sso-port 8085")
    result = subprocess.run(
        [argocd_bin, "login", "--grpc-web", ARGOCD_SERVER, "--sso", "--sso-port", "8085"],
        stdin=sys.stdin,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    if result.returncode != 0:
        print("  ✗ argocd login failed")
        sys.exit(1)

    token = _read_token_from_config()
    if not token:
        print("  ✗ Token still missing after login")
        sys.exit(1)
    return token


def get_argocd_token() -> str:
    """Read the auth token from the ArgoCD CLI config, refreshing if expired."""
    config_path = Path.home() / ".config" / "argocd" / "config"
    token = _read_token_from_config()

    if not token:
        print(f"  ✗ No auth-token found in {config_path}")
        print(f"    Attempting to log in …")
        token = _refresh_token()

    if _token_is_expired(token):
        exp = _jwt_expiry(token)
        exp_str = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(exp)) if exp else "?"
        print(f"  ⚠ Token expired at {exp_str} — refreshing …")
        token = _refresh_token()

        if _token_is_expired(token):
            print("  ✗ Token still expired after refresh")
            sys.exit(1)

    return token


def argocd_get(path: str, token: str):
    """GET a JSON response from the ArgoCD API."""
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    req = urllib.request.Request(f"{ARGOCD_BASE}{path}", headers=headers)
    try:
        resp = urllib.request.urlopen(req, context=ctx, timeout=10)
        return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode(errors="replace")
        raise RuntimeError(f"HTTP {e.code}: {body[:300]}") from e


def get_ca_via_openssl(host: str, port: int = 6443) -> str:
    """Fallback: grab the serving CA directly with openssl s_client."""
    try:
        result = subprocess.run(
            ["openssl", "s_client", "-showcerts", "-connect", f"{host}:{port}"],
            input=b"",
            capture_output=True,
            timeout=5,
        )
        pem_lines = []
        in_cert = False
        for line in result.stdout.decode().splitlines():
            if "BEGIN CERTIFICATE" in line:
                in_cert = True
            if in_cert:
                pem_lines.append(line)
            if "END CERTIFICATE" in line:
                in_cert = False
        if pem_lines:
            return "\n".join(pem_lines) + "\n"
    except Exception:
        pass
    return ""


# ArgoCD app name for the cluster-admin-sa addon
SA_APP_NAME = f"cluster-admin-sa-{CLUSTER_NAME}"
SA_SECRET_NAME = "cluster-admin-sa-token"
SA_SECRET_NS = "kube-system"


def _fetch_sa_token_via_argocd(argocd_token: str) -> str:
    """Fetch the cluster-admin SA token Secret via the ArgoCD resource API.

    ArgoCD manages the cluster-admin-sa addon. The live Secret contains a
    K8s-generated bearer token we can use for kubectl access.
    """
    # First try the ArgoCD CLI (handles auth & TLS nicely)
    argocd_bin = shutil.which("argocd")
    if argocd_bin:
        try:
            result = subprocess.run(
                [
                    argocd_bin, "app", "get-resource", SA_APP_NAME,
                    "--grpc-web",
                    "--kind", "Secret",
                    "--resource-name", SA_SECRET_NAME,
                    "--namespace", SA_SECRET_NS,
                    "-o", "json",
                ],
                capture_output=True,
                text=True,
                timeout=15,
            )
            if result.returncode == 0 and result.stdout.strip():
                secret = json.loads(result.stdout)
                token_b64 = secret.get("data", {}).get("token", "")
                if token_b64:
                    return base64.b64decode(token_b64).decode()
        except Exception as exc:
            print(f"    (CLI fallback failed: {exc})")

    # Fallback: direct API call
    try:
        params = (
            f"namespace={SA_SECRET_NS}"
            f"&resourceName={SA_SECRET_NAME}"
            f"&version=v1&kind=Secret"
        )
        resource = argocd_get(
            f"/api/v1/applications/{SA_APP_NAME}/resource?{params}",
            argocd_token,
        )
        manifest = resource.get("manifest", "")
        if manifest:
            secret = json.loads(manifest)
            token_b64 = secret.get("data", {}).get("token", "")
            if token_b64:
                return base64.b64decode(token_b64).decode()
    except Exception as exc:
        print(f"    (API fallback failed: {exc})")

    return ""


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    output_path = sys.argv[1] if len(sys.argv) > 1 else OUTPUT_DEFAULT

    # 1. Get ArgoCD auth token from CLI config
    print("→ Reading ArgoCD token from ~/.config/argocd/config …")
    token = get_argocd_token()
    print(f"  ✓ Token found ({token[:20]}…)")

    # 2. Pull cluster credentials from ArgoCD API
    print("→ Fetching cluster credentials from ArgoCD API …")
    cluster_data = argocd_get(f"/api/v1/clusters/{CLUSTER_URL_ENCODED}", token)
    config = cluster_data.get("config", {})
    tls = config.get("tlsClientConfig", {})

    bearer_token = config.get("bearerToken", "")
    ca_data_b64 = tls.get("caData", "")
    server_version = cluster_data.get("serverVersion", "?")
    print(f"  ✓ Cluster: {cluster_data.get('name', '?')} (K8s {server_version})")
    print(f"  ✓ Bearer token: {'present' if bearer_token else 'MISSING'}")
    print(f"  ✓ CA data: {'present' if ca_data_b64 else 'missing (will fetch via openssl)'}")

    # 3. Determine API server endpoint
    api_server = FALLBACK_API
    # Try to get the real endpoint from the ArgoCD resource tree
    try:
        tree = argocd_get(f"/api/v1/applications/argocd-{CLUSTER_NAME}/resource-tree", token)
        for node in tree.get("nodes", []):
            if node.get("kind") == "Endpoints" and node.get("name") == "kubernetes" and node.get("namespace") == "default":
                info = node.get("networkingInfo", {})
                # Not always populated — fall back
                break
    except Exception:
        pass
    print(f"  ✓ API server: {api_server}")

    # 4. Get CA certificate
    if not ca_data_b64:
        # ArgoCD in-cluster config doesn't store CA – fetch directly
        print("→ Fetching CA cert via openssl …")
        host = api_server.replace("https://", "").split(":")[0]
        port = int(api_server.replace("https://", "").split(":")[1])
        ca_pem = get_ca_via_openssl(host, port)
        if not ca_pem:
            print("  ✗ Could not retrieve CA cert")
            sys.exit(1)
        ca_data_b64 = base64.b64encode(ca_pem.encode()).decode()
        print(f"  ✓ CA cert retrieved ({len(ca_pem)} bytes)")
    else:
        ca_pem = base64.b64decode(ca_data_b64).decode()
        print(f"  ✓ Using CA from ArgoCD ({len(ca_pem)} bytes)")

    # 5. If no bearer token, fetch the cluster-admin SA token via ArgoCD
    if not bearer_token:
        print("→ No bearer token in cluster config (in-cluster auth).")
        print("  Fetching cluster-admin-sa-token via ArgoCD resource API …")
        bearer_token = _fetch_sa_token_via_argocd(token)
        if not bearer_token:
            print("  ✗ Could not retrieve SA token. Ensure the cluster-admin-sa addon is synced:")
            print("    ArgoCD app: cluster-admin-sa-the-cluster")
            sys.exit(1)
        print(f"  ✓ SA bearer token retrieved ({len(bearer_token)} chars)")

    # 6. Write kubeconfig
    kubeconfig = {
        "apiVersion": "v1",
        "kind": "Config",
        "clusters": [
            {
                "name": CLUSTER_NAME,
                "cluster": {
                    "server": api_server,
                    "certificate-authority-data": ca_data_b64,
                },
            }
        ],
        "contexts": [
            {
                "name": CLUSTER_NAME,
                "context": {
                    "cluster": CLUSTER_NAME,
                    "user": f"{CLUSTER_NAME}-admin",
                },
            }
        ],
        "current-context": CLUSTER_NAME,
        "users": [
            {
                "name": f"{CLUSTER_NAME}-admin",
                "user": {
                    "token": bearer_token,
                },
            }
        ],
    }

    Path(output_path).parent.mkdir(parents=True, exist_ok=True)
    with open(output_path, "w") as f:
        # Write as YAML-ish JSON that kubectl understands
        json.dump(kubeconfig, f, indent=2)
    print(f"\n✓ Kubeconfig written to {output_path}")

    # 7. Optionally add to kubecm
    if shutil.which("kubecm"):
        print(f"\nTo add to kubecm:\n  kubecm add -f {output_path}")
    else:
        print(f"\nTo use:\n  export KUBECONFIG={output_path}")
        print(f"  kubectl get nodes")


if __name__ == "__main__":
    main()
