# Workloads

Application definitions deployed **inside** vclusters by the vcluster's own ArgoCD instance.

## Structure

```
workloads/
└── <cluster-name>/
    ├── addons.yaml          # Workload definitions (ApplicationSet values)
    └── addons/
        └── <app-name>/
            └── values.yaml  # Helm values for the app
```

Each subdirectory corresponds to a vcluster's `cluster_name` label. The vcluster's ArgoCD has a `workloads` ApplicationSet that reads `workloads/<cluster_name>/addons.yaml` and generates Applications for each entry.

## How It Works

1. The host ArgoCD deploys addons into the vcluster, including an ArgoCD instance
2. That ArgoCD instance has a `workloads` ApplicationSet (defined in [addons/cluster-roles/vcluster/addons/addons.yaml](../addons/cluster-roles/vcluster/addons/addons.yaml))
3. The workloads ApplicationSet reads from `workloads/<cluster_name>/` in this repo
4. Each entry in `addons.yaml` becomes an ArgoCD Application inside the vcluster

## Adding a Workload

1. Create a values file: `workloads/<cluster>/addons/<app-name>/values.yaml`
2. Add an entry in `workloads/<cluster>/addons.yaml`
3. Commit and push — the vcluster's ArgoCD picks it up automatically

## Current Workloads

- **vcluster-media**: sonarr, radarr, sabnzbd, otterwiki (all using [stakater/application](https://github.com/stakater/application) chart)
