# specmarshal

[Specmarshal](https://github.com/IntegratnIO/specmarshal) works a spec's
tickets with coding agents. This addon runs its standing process,
`specmarshal serve`, in `specmarshal`. Each agent iteration, and each
verification gate, runs as a Job in `specmarshal-agents`. The orchestrator
starts the Job through the API server and reaches into it with `kubectl exec`
(the kubernetes launcher, ADR-0023 in the specmarshal repository).

| File | What it is |
|---|---|
| `deployment.yaml` | the orchestrator. **Ships at `replicas: 0`**, see below |
| `rbac.yaml` | its ServiceAccount, and a Role in `specmarshal-agents` for Jobs, pods/exec and Secrets there only |
| `agents-limits.yaml` | LimitRange and ResourceQuota on the iteration Jobs. The quota caps concurrency |
| `externalsecret.yaml` | every credential, from 1Password |
| `pvc.yaml` | the Specmarshal home (clones, run artifacts, the GitHub App key) on `config-nfs-client` |
| `service.yaml`, `httproute.yaml`, `snippetsfilter.yaml` | `specmarshal.cluster.integratn.tech`, behind authentik |

The namespaces and their NetworkPolicies are in
[`network-policies/specmarshal.yaml`](../network-policies/specmarshal.yaml),
and the gateway's half of the ingress is in `nginx-gateway.yaml`. The authentik
provider is `08a-specmarshal-proxy-provider.yaml` in the blueprint ConfigMap.

## Before scaling up

Scaling the Deployment to 1 is the deploy. It needs all of these first.

### 1. An image with the kubernetes launcher

The pinned `sha-0fe895f…` predates it and refuses `launcher: kubernetes`. Once
IntegratnIO/specmarshal#150 and #152 are merged, set both `image:` lines in
`deployment.yaml` (the `github-app` init container and `specmarshal`) to the
`sha-<merge commit>` tag the publish workflow pushes.

The image is not Kargo-tracked. The package is private, and Kargo has no GHCR
credential for `ghcr.io/integratnio`, so a Warehouse for it would fail forever.
Tracking it needs that credential first.

### 2. The database

The measurements live in Postgres on `10.0.3.1`. The file backend is SQLite,
and SQLite's locking is not safe on NFS. Specmarshal creates and migrates its
own tables; it needs a role that owns its database:

```sql
CREATE ROLE specmarshal WITH LOGIN PASSWORD '<password>';
CREATE DATABASE specmarshal OWNER specmarshal ENCODING 'UTF8' TEMPLATE template0;
REVOKE ALL ON DATABASE specmarshal FROM PUBLIC;
\c specmarshal
ALTER SCHEMA public OWNER TO specmarshal;
```

`pg_hba.conf` on `10.0.3.1` has to admit the cluster's pod network for that
role and database.

### 3. 1Password items

Every item is read by `onepassword-store`, and every field by its **label**.
Renaming a field breaks the ExternalSecret that reads it.

| Item | Fields | Notes |
|---|---|---|
| `specmarshal-db-connection` | `host`, `port`, `text` (the role name), `password` | the connection string is assembled in `externalsecret.yaml`, and the password reaches the pod as `PGPASSWORD`, never in the URL |
| `specmarshal-anthropic` | `api-key` | an Anthropic API key. An iteration Job has no sign-in volume, so Claude Code runs on a key or not at all |
| `specmarshal-github-app` | `app-id`, `installation-id`, `private-key` | the GitHub App Specmarshal acts as. It needs Administration, Contents, Issues and Pull requests (write), plus Metadata (read). `specmarshal github-app` with no flags prints the walk-through |
| `specmarshal-ghcr` | `username`, `token` | a classic PAT with `read:packages`, for the private orchestrator and sandbox images. Used in both namespaces |

### 4. A project

`serve` with no projects says so and exits, so the Deployment would crash-loop.
Set `SPECMARSHAL_PROJECTS` in `deployment.yaml` to the repositories it holds, as
clone URLs, comma separated. Or set it to one, and register the rest on the
surface.

## Running it

- The surface is `https://specmarshal.cluster.integratn.tech`. Reading it needs
  only authentik. Acting on it (marking a spec ready, releasing a ticket) needs
  the capability address the pod prints at start-up:
  `kubectl -n specmarshal logs deploy/specmarshal | grep '^Web'`.
- The surface's health section asks the API server whether the launcher's
  permissions hold. "launcher unwell" names the missing verb.
- Iterations: `kubectl -n specmarshal-agents get jobs,pods`. Each Job lives as
  long as its iteration and is deleted with it. A Job the orchestrator never
  came back for ends at its `activeDeadlineSeconds` (the iteration timeout plus
  30 minutes) and is removed 5 minutes later.
- Stopping drains. The first SIGTERM lets runs in flight finish, and
  `terminationGracePeriodSeconds` is 7500 so a two-hour iteration can. A
  rollout therefore waits for work in progress. That is deliberate.
