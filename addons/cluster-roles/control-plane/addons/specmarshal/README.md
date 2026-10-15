# specmarshal

[Specmarshal](https://github.com/IntegratnIO/specmarshal) works a spec's
tickets with coding agents. This addon installs it the way a licensee does: the
published chart, `registry.integratn.io/integratn/charts/specmarshal`, pinned by
`defaultVersion` in `addons.yaml`. The chart is the orchestrator
(`specmarshal serve`), its home on a PersistentVolumeClaim, a ClusterIP Service,
and a ServiceAccount whose Role lets it run each agent iteration, and each
verification gate, as a Job beside it in the `specmarshal` namespace. The
orchestrator starts the Job through the API server and reaches into it with
`kubectl exec`.

What the chart leaves to the cluster is in this directory:

| File | What it is |
|---|---|
| `values.yaml` | the chart's values: the two Secrets below by name, the home's size and class, and the stop grace period |
| `externalsecret.yaml` | `specmarshal-env`, the orchestrator's environment (the Postgres store and the model's settings); `specmarshal-registry`, the image pull secret; `specmarshal-license`, the key the chart enters; `specmarshal-github-app`, the App the chart stores; and `specmarshal-sign-in`, the App's OAuth client people sign in with |
| `limits.yaml` | LimitRange and ResourceQuota on the namespace. The quota caps how many iterations run at once |
| `httproute.yaml`, `snippetsfilter.yaml` | `specmarshal.cluster.integratn.tech`, behind authentik and then the surface's own GitHub sign-in |

Elsewhere:

- The namespace and its NetworkPolicies are in
  [`network-policies/specmarshal.yaml`](../network-policies/specmarshal.yaml),
  and the gateway's half of the ingress is in `nginx-gateway.yaml`.
- ArgoCD's login to the registry is
  `clusters/the-cluster/addons/argo-cd/manifests/specmarshal-registry-externalsecret.yaml`.
- The authentik provider is `08a-specmarshal-proxy-provider.yaml` in the
  blueprint ConfigMap.

## Before merging

The chart runs one replica, so the merge is the deploy. These have to exist
first, or the app sits Degraded until they do.

### 1. 1Password items

Every item is read by `onepassword-store`, and every field by its **label**.
Renaming a field breaks the ExternalSecret that reads it.

| Item | Fields | Notes |
|---|---|---|
| `specmarshal-license` | `key` | the license key. It is the registry password, with the username `license`, for ArgoCD's chart pull and for the kubelet's image pulls |
| `specmarshal-db-connection` | `host`, `port`, `text` (the role name), `password` | the connection string is assembled in `externalsecret.yaml`, and the password reaches the pod as `PGPASSWORD`, never in the URL |
| `specmarshal-github-app` | `app-id`, `private-key` | the App the orchestrator acts as: 4924067, the same App the Mac's service uses. `private-key` is the whole `.pem` the App's settings page generated, BEGIN and END lines included, as a multi-line text field |
| `specmarshal-sign-in` | `client-id`, `client-secret` | that App's OAuth client, which people sign in to the surface with. On the App's settings page, set the callback URL to exactly `https://specmarshal.cluster.integratn.tech/sign-in/callback`, copy the client ID, and generate a client secret, which GitHub shows once |

### 2. The database

The measurements live in Postgres on `10.0.3.1`. The home is on NFS, and the
file backend is SQLite, whose locking is not safe there. Specmarshal creates and
migrates its own tables; it needs a role that owns its database:

```sql
CREATE ROLE specmarshal WITH LOGIN PASSWORD '<password>';
CREATE DATABASE specmarshal OWNER specmarshal ENCODING 'UTF8' TEMPLATE template0;
REVOKE ALL ON DATABASE specmarshal FROM PUBLIC;
\c specmarshal
ALTER SCHEMA public OWNER TO specmarshal;
```

`pg_hba.conf` on `10.0.3.1` has to admit the cluster's pod network for that
role and database.

### 3. The model

Iterations run Pi against LM Studio on the workstation, `192.168.0.57:1234`,
the same server and DHCP reservation bosun uses. LM Studio needs "Serve on
Local Network" on and `qwen3.6-35b-a3b-splash` available. Nothing is billed per
token, and there is no provider key. To use a hosted model instead, change the
`SPECMARSHAL_PROVIDER` lines in `externalsecret.yaml` and add the provider's key
to the same Secret.

## Once it is running

Each of these is a command in the orchestrator's pod, and what it writes is
kept in the home, so it is done once and survives a restart.

1. **The license enters itself.** `license.secret` in `values.yaml` names the
   Secret `specmarshal-license`, and an init container runs
   `specmarshal license add` with it on every start. Read what it grants with
   `kubectl -n specmarshal exec deploy/specmarshal -- specmarshal license status`.
   A key Keygen does not know stops the pod in its `license` container:
   `kubectl -n specmarshal logs deploy/specmarshal -c license`.

   This orchestrator counts as one against the license. Its identity is in
   the home, so deleting the claim `specmarshal-home` makes the next pod a new
   orchestrator that takes a new slot.

2. **The GitHub App enters itself.** `githubApp.secret` in `values.yaml`
   names the Secret `specmarshal-github-app`, and a `github-app` init
   container runs `specmarshal github-app` with the key on standard input on
   every start. A key that is not a PEM private key stops the pod there:
   `kubectl -n specmarshal logs deploy/specmarshal -c github-app`. The App
   needs Administration, Contents, Issues and Pull requests (write), plus
   Metadata (read), and has to be installed on every repository the service
   holds a project for: sign-in asks GitHub what a person may do on a
   repository as that installation, and refuses everyone but the operators a
   repository the App does not reach.

   **Sign-in is ready** when the license is paid, `signIn.webUrl` is `https`,
   and the client ID and secret are in `specmarshal-sign-in`. Ask the pod:
   `kubectl -n specmarshal exec deploy/specmarshal -- specmarshal sign-in status`
   prints `Sign-in is ready.` or the first thing missing.

3. **Register the projects** on the surface. `serve` stands with none.

## Running it

- The surface is `https://specmarshal.cluster.integratn.tech`, behind
  authentik and then GitHub sign-in. Reading a project needs read on its
  repository; acting on it (marking a spec ready, releasing a ticket) needs
  write; acting on the service (registering a project, the settings) needs a
  login in `signIn.operators`. The capability address the pod prints at
  start-up (`kubectl -n specmarshal logs deploy/specmarshal | grep '^Web'`)
  now counts only from inside the pod: open it through
  `kubectl -n specmarshal port-forward service/specmarshal 4200`.
- The surface's health section asks the API server whether the launcher's
  permissions hold. "launcher unwell" names the missing verb.
- Iterations: `kubectl -n specmarshal get jobs,pods -l app.kubernetes.io/name=specmarshal-iteration`.
  Each Job lives as long as its iteration and is deleted with it.
- Stopping drains. The first SIGTERM lets runs in flight finish, and
  `terminationGracePeriodSeconds` is above the three-hour iteration timeout so
  one can. A rollout therefore waits for work in progress. That is deliberate.
- A new Version is a bump of `defaultVersion` in `addons.yaml`: the chart pins
  the image by digest, so one line moves both. It is not Kargo-tracked, because
  Kargo holds no credential for the registry. Read the chart before bumping:
  `helm show values oci://registry.integratn.io/integratn/charts/specmarshal --version <version>`.
