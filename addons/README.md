# Addons GitOps: how it works

This folder is the “addons layer” for the cluster(s). It is designed so that:

- a **single bootstrap ApplicationSet** (usually created by Terraform) points Argo CD at this repo
- a **Helm chart** (`charts/application-sets`) renders one **ApplicationSet per addon**
- each addon ApplicationSet generates one **Application per target cluster**, based on labels on the Argo CD cluster Secret
- each addon Application pulls its Helm values from this repo using Argo CD **multi-source** and the special `$values/…` valueFiles syntax

If you’re new to Argo CD + ApplicationSets: the big idea is “declare addons once, select clusters via labels”.

---

## Folder layout

- `charts/application-sets/`
  - Helm chart that renders `ApplicationSet` resources for every addon you define.
- `clusters/<cluster-name>/`
  - cluster-specific configuration (e.g. `clusters/in-cluster/`).
  - `common.yaml`: overrides for the `application-sets` chart (global selectors, repo base path, etc).
  - `addons.yaml`: the list of addons and how to install them.
  - `addons/<addon-name>/values.yaml`: per-addon Helm values for that cluster.
- `default/addons/addons.yaml`
  - default addon list (used by all clusters unless overridden).
  - `default/addons/<addon-name>/values.yaml`
    - defaults shared by all clusters.
- `environments/<env-name>/...`
  - optional environment-level structure used by some setups (your chart currently uses `clusters/<environment>/addons/...`, not `environments/...`).

---

## The bootstrap chain (end-to-end)

### 1) Terraform (or manual bootstrap) creates the “bootstrap” ApplicationSet

Look at [terraform/cluster/bootstrap/addons.yaml](../terraform/cluster/bootstrap/addons.yaml).

This `ApplicationSet`’s job is to:

- select *clusters* (via the `clusters: {}` generator)
- for each cluster, create an Application that points back at this repo
- pass Helm values files that describe what addons exist for that cluster:
  - `$values/addons/clusters/in-cluster/common.yaml`
  - `$values/addons/clusters/in-cluster/addons.yaml`

Those two values files are what feed the `charts/application-sets` Helm chart.

### 2) The `application-sets` Helm chart renders one ApplicationSet per addon

The Helm chart is [addons/charts/application-sets](charts/application-sets).

The key template is [addons/charts/application-sets/templates/application-set.yaml](charts/application-sets/templates/application-set.yaml):

- It loops through each top-level map in the Helm values and finds entries that look like an addon (a map containing `enabled:`).
- For each addon with `enabled: true`, it creates an `ApplicationSet`.

Example addon definition lives in [addons/clusters/in-cluster/addons.yaml](clusters/in-cluster/addons.yaml):

```yaml
kratix:
  enabled: true
  namespace: kratix-platform-system
  chartName: kratix
  chartRepository: https://syntasso.github.io/helm-charts
  defaultVersion: "0.0.1"
  selector:
    matchExpressions:
      - key: enable_kratix
        operator: In
        values: ['true']
```

### 3) Each addon ApplicationSet targets clusters using label selectors

Each addon has a `selector:` block.

That selector is applied to the Argo CD cluster Secret (type `argocd.argoproj.io/secret-type: cluster`).

So `kratix` only gets installed on clusters whose cluster Secret has this label:

- `enable_kratix=true`

There can also be global selectors. For example, in [addons/clusters/in-cluster/common.yaml](clusters/in-cluster/common.yaml) you have:

```yaml
globalSelectors:
  environment: "prod"
```

That means: only clusters with `environment=prod` are eligible for *any* addons in this “in-cluster” config.

---

## How Argo CD finds `clusters/<cluster>/addons/<addon>/values.yaml`

### Multi-source + `$values/…`

The rendered Application uses multiple sources:

1) A Git source with `ref: values` (this repo). This is the “values source”.
2) The actual addon chart source (remote Helm repo, or a `path:` for local charts).

Because the Git source is referenced as `values`, Argo CD lets the chart source refer to files in the Git source using:

- `$values/<path-in-git-repo>`

### The values search paths (default → environment → cluster)

The chart config in [addons/charts/application-sets/values.yaml](charts/application-sets/values.yaml) defines where it searches for per-addon values:

```yaml
valueFiles:
  - default/addons
  - clusters/{{.metadata.labels.environment}}/addons
  - clusters/{{.nameNormalized}}/addons
```

Then the helper template [addons/charts/application-sets/templates/_application_set.tpl](charts/application-sets/templates/_application_set.tpl) builds paths like:

- `$values/<repoBasePath>/<one-of-the-folders>/<addonName>/values.yaml`

Your in-cluster config sets the repo base path here:

- [addons/clusters/in-cluster/common.yaml](clusters/in-cluster/common.yaml) → `repoURLGitBasePath: "addons"`

So for addon `kratix` on cluster `in-cluster`, one of the generated value file paths is:

- `$values/addons/clusters/in-cluster/addons/kratix/values.yaml`

…and that file exists here:

- [addons/clusters/in-cluster/addons/kratix/values.yaml](clusters/in-cluster/addons/kratix/values.yaml)

### Why missing files don’t break everything

The chart sets:

- `ignoreMissingValueFiles: true`

So it is OK if, for example, `clusters/prod/addons/kratix/values.yaml` doesn’t exist. Argo CD will just skip missing files.

### Tenant prefix (optional)

The chart supports tenant-prefixed values paths (useful in multi-tenant setups). The default chart values enable it, but your in-cluster config turns it off:

- chart default: [addons/charts/application-sets/values.yaml](charts/application-sets/values.yaml) → `useValuesFilePrefix: true`
- in-cluster override: [addons/clusters/in-cluster/common.yaml](clusters/in-cluster/common.yaml) → `useValuesFilePrefix: false`

When enabled, it will also look under a prefix like:

- `{{.metadata.labels.tenant}}/clusters/<...>/addons/<addon>/values.yaml`

---

## Adding a new addon (checklist)

### Step 1: Decide the addon key name

Pick a unique key under your cluster’s `addons.yaml`. This becomes the addon name used for:

- the ApplicationSet name
- the per-addon values folder name
- some labels (like `addonName`)

Tip: prefer `kebab-case` or `snake_case`; the templates normalize `_` to `-`.

### Step 2: Add the addon entry in the cluster config

Edit:

- [addons/clusters/in-cluster/addons.yaml](clusters/in-cluster/addons.yaml)

Minimum for a Helm addon:

```yaml
my-addon:
  enabled: true
  namespace: my-namespace
  chartName: my-addon
  chartRepository: https://example.com/helm-charts
  defaultVersion: "1.2.3"
  selector:
    matchExpressions:
      - key: enable_my_addon
        operator: In
        values: ['true']
```

Notes:

- `selector` is required if you want to control which clusters get it.
- `defaultVersion` is used unless overridden via `environments:` blocks.
- Use `valuesObject:` for small templated settings that depend on cluster metadata (`{{.metadata.annotations...}}`).

### Step 3: Add per-addon Helm values (optional but common)

Create a values file in one (or more) of the supported locations:

- Cluster-specific (most specific):
  - `addons/clusters/<cluster-name>/addons/<addon-name>/values.yaml`
- Environment-level (shared by many clusters):
  - `addons/clusters/<environment>/addons/<addon-name>/values.yaml`
- Default (shared by all clusters):
  - `addons/default/addons/<addon-name>/values.yaml`

Helm merges values in order; later files override earlier ones.

### Step 4: Ensure the target cluster is labeled to match

Add labels to the Argo CD cluster Secret for that cluster:

- it must already have `argocd.argoproj.io/secret-type=cluster`
- it must match any `globalSelectors` you are using (e.g. `environment=prod`)
- it must match the addon selector (e.g. `enable_my_addon=true`)

If the selector doesn’t match, the addon’s Application won’t be generated for that cluster.

### Step 5: Sync and verify

In Argo CD UI/CLI, verify:

- the addon’s `ApplicationSet` exists
- an `Application` for your cluster was generated
- under the Application “App Details”, `Helm valueFiles` includes the expected `$values/.../values.yaml`

---

## Common pitfalls / debugging tips

- **Addon installs nowhere**: check cluster Secret labels match `globalSelectors` and the addon’s `selector`.
- **Values not taking effect**: confirm the exact path Argo CD is using. Remember the repo base path (`repoURLGitBasePath`) is part of the `$values/...` path.
- **You expected env-level values**: the chart looks under `clusters/{{.metadata.labels.environment}}/addons/...`, so make sure your cluster Secret actually has an `environment` label.
- **Merging surprises**: if a key is defined in multiple values files, the later file in the list wins.

---

## Where to look in code

- ApplicationSet generator + Helm wiring: [addons/charts/application-sets/templates/application-set.yaml](charts/application-sets/templates/application-set.yaml)
- ValueFiles path construction: [addons/charts/application-sets/templates/_application_set.tpl](charts/application-sets/templates/_application_set.tpl)
- Default valueFiles search list: [addons/charts/application-sets/values.yaml](charts/application-sets/values.yaml)
- Cluster-specific addon declarations (in-cluster): [addons/clusters/in-cluster/addons.yaml](clusters/in-cluster/addons.yaml)
- Cluster-specific overrides (in-cluster): [addons/clusters/in-cluster/common.yaml](clusters/in-cluster/common.yaml)
