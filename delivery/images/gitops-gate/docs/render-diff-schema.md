# `render-diff.json`

The machine-readable output of `gitops-gate diff`, and the contract the
delivery agent reads. Treat it as a published interface: the agent depends on
these field names.

```json
{
  "targeting": [
    {
      "kind": "added",
      "cluster": "tenant",
      "app": "goldilocks-tenant",
      "appset": "goldilocks",
      "to": "https://charts.example/goldilocks 11.0.0",
      "detail": "newly generated for this cluster"
    }
  ],
  "introduced": [
    {
      "kind": "introduced",
      "cluster": "hub",
      "app": "kargo-observability-hub",
      "appset": "kargo-observability",
      "to": "charts/kargo-observability (path)",
      "detail": "new addon, first appearance"
    }
  ],
  "versions": [
    {
      "kind": "version",
      "cluster": "hub",
      "app": "cert-manager-hub",
      "appset": "cert-manager",
      "from": "v1.21.1",
      "to": "v1.22.0"
    }
  ],
  "other": [],
  "warnings": ["addons: generators[1]: git generator is not expanded ..."]
}
```

## The three buckets, and why they are separate

**`targeting`** — an Application is generated for a different set of clusters
than before. **Blocking.** This is the finding the gate exists for, because it
is the one a reviewer cannot get from the text diff: the selector did not
change, the set of clusters it matches did.

`kind` is one of:

| `kind` | Meaning |
|---|---|
| `added` | An ApplicationSet that already existed is now generated for a cluster it was not. This is the leak. |
| `removed` | No longer generated. This is a silent uninstall — ArgoCD will prune it. |
| `moved` | Both, for the same ApplicationSet. Reported as one change, because reporting it as an unrelated add plus an unrelated remove buries the actual shape of what happened. |

**`introduced`** — a whole ApplicationSet that did not exist before. **Not
blocking.** Adding an addon is a deliberate act by the author of the pull
request; the dangerous case is an addon that *already existed* quietly changing
which clusters it reaches. Blocking on both would make every new-addon pull
request red for a reason nobody needs to investigate, and a check that gets
routinely overridden stops functioning as a check.

**`versions`** — same Application, same clusters, different `targetRevision`.
**Not blocking**, because this is the entire point of an automated bump
pipeline. Blocking here would park every automated merge forever.

**`other`** — same Application and clusters, but something structural moved: the
chart itself, the source type, the ArgoCD project, the destination namespace.
**Blocking.** A chart swapped underneath an unchanged Application name is not a
version bump and must not be waved through as one.

## `warnings`

Generators the gate could not expand — `git`, `matrix`, `list`. The
Applications they generate are **not** covered by any of the above.

This is deliberately loud. A gate that quietly skips what it cannot handle
reports "no targeting change" with exactly the same words whether it checked
everything or nothing, and the reader has no way to tell which. Anything
consuming this file should surface warnings rather than filtering them out.
