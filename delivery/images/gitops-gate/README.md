# gitops-gate

The deterministic half of the delivery gate. A single binary that answers one
question about a pull request: **does this change what actually gets deployed,
and is what it produces still valid?**

It is CI-agnostic by construction — you run the container, and the exit code is
the verdict. Adapters in [`../../ci`](../../ci) are thin wrappers that pass a
workspace in and turn the exit code into a commit status.

> **Status: not yet implemented.** This README is the contract the
> implementation is written against. See [`../../adr/0002-triage-in-cluster-not-ci.md`](../../adr/0002-triage-in-cluster-not-ci.md)
> for why the AI half is deliberately *not* here.

## Subcommands

| Command | Does |
|---|---|
| `render` | Renders every bootstrap ApplicationSet declared in `.gitops-gate.yaml`, for every cluster in the inventory, expanding the generators. Emits a normalized target table. |
| `diff` | Compares two target tables. Fails on a **cluster-targeting change**; passes with a summary when only versions moved. Emits `render-diff.json`. |
| `validate` | Schema-validates every rendered stream. |
| `clusters export` | Regenerates the cluster inventory from live ArgoCD cluster Secrets. |

## Why targeting is the thing it fails on

ApplicationSet generators resolve selectors against live cluster labels. That
means a values-layer edit can add or remove an entire cluster from an addon's
scope without the text diff showing anything of the sort — the selector did not
change, the cluster labels it matches did. Rendering both sides and diffing the
*expanded* result is the only way to see it.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | No blocking change. |
| `1` | Blocking change — targeting moved, or validation failed. |
| `2` | The gate itself could not run (bad config, unreachable chart repo). Distinct from `1` so CI can tell "this change is bad" from "the gate is broken". |

## Reference

- [`docs/config-reference.md`](docs/config-reference.md) — the full `.gitops-gate.yaml` schema
- [`docs/render-diff-schema.md`](docs/render-diff-schema.md) — the JSON contract the agent consumes
- [`docs/adding-a-ci-provider.md`](docs/adding-a-ci-provider.md)
- [`docs/rendered-manifests.md`](docs/rendered-manifests.md) — the rendered-manifests pattern, and why ArgoCD's source hydrator cannot gate a merge
