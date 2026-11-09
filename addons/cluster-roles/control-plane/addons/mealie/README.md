# mealie

[Mealie](https://mealie.io) is a recipe manager with meal planning and shopping
lists, and a REST API with long-lived tokens. It runs from the official image,
`ghcr.io/mealie-recipes/mealie`, pinned by tag and digest, with its database on
datacron's Postgres (`10.0.3.1`). It is plain manifests.

| File | What it is |
|---|---|
| `deployment.yaml` | the 5Gi `mealie-data` PVC (`config-nfs-client`, `Prune=false`), the Deployment (one replica, `Recreate`) and the Service `mealie.mealie.svc.cluster.local:9000` |
| `externalsecret.yaml` | `mealie-db`: the `POSTGRES_*` variables, from the 1Password item `mealie-db-connection` |
| `httproute.yaml` | `mealie.cluster.integratn.tech`, LAN only, **no authentik forward-auth** (it would block API tokens and the mobile apps) |

Elsewhere: the namespace and its NetworkPolicies are in
[`network-policies/mealie.yaml`](../network-policies/mealie.yaml), including the
egress half for the gateway.

## Before merging

The Deployment migrates the database on start, so merging is the deploy. Until
these exist the pod crashloops and nothing else is affected.

1. **The database.** Declare the role and database in nix_dots_2
   `hosts/datacron/postgresql.nix` (`roles.mealie = { };` and
   `databases.mealie = "mealie";`), then deploy with
   `nixos-rebuild --target-host root@10.0.3.5`. The `k8s-nodes` entry in
   `pg_hba` already admits the cluster for every database and role.
2. **The role's password.** Passwords are not in Nix. On datacron:
   `ALTER ROLE mealie PASSWORD '<password>';` (letters and digits only).
3. **The 1Password item** `mealie-db-connection`, fields by label: `host`
   (`10.0.3.1`), `port` (`5432`), `text` (`mealie`), `password`.

## First run

1. Open `https://mealie.cluster.integratn.tech`. Mealie's docs list a default
   first-start login (`changeme@example.com` / `MyPassword`). **Change both at
   once**, and check the current docs if it does not work.
2. Create the household's users (signup is off).
3. For API use: a dedicated non-admin user, then Profile -> API Tokens ->
   Generate. The token inherits that user's permissions and group, and API
   objects are scoped by group, so create it in the same group the family uses.

## Not done yet

- **OIDC against authentik.** Needs a provider in the authentik blueprint
  ConfigMap (`clusters/the-cluster/addons/authentik/`) and a client secret in
  1Password. Mealie's `OIDC_*` variables are in its backend-config docs.
- **Kargo tracking.** No `mealie` target in `kargo-projects/values.yaml`.
- **Hardening.** No `runAsNonRoot` or read-only root: see the note in
  `deployment.yaml`.
- **The arm64 allowlist.** Not listed, so the pod stays on the control-plane
  nodes (and never lands on the Pi 5).
- **OKF.** `docs/okf/log.md` and `addons/addon-inventory.md` have no entry yet.

## Running it

- `kubectl -n mealie logs deploy/mealie`: a pod that never starts is the
  database (`pg_hba`, the role, the password, the item).
- Recipe import from a URL needs internet egress; `allow-internet-egress` in
  the network policy allows 80 and 443 to non-RFC1918 addresses.
