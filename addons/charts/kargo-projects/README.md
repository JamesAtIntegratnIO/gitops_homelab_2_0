# kargo-projects

Renders Kargo `Project`s, `Warehouse`s and `Stage`s from a compact list of
*update targets*. One target is one artifact (a container image or a Helm
chart) whose version is pinned somewhere in this repository; the chart emits a
Warehouse that watches the artifact and a Stage that rewrites the pin through
a pull request.

It is deployed by the `kargo-projects` addon in
[`addons/cluster-roles/control-plane/addons/addons.yaml`](../../cluster-roles/control-plane/addons/addons.yaml);
the target list is
[`addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml`](../../cluster-roles/control-plane/addons/kargo-projects/values.yaml).
The schema is documented in [`values.yaml`](values.yaml) and the design in
[`docs/kargo.md`](../../../docs/kargo.md).

## What a Stage does

```
git-clone        main
yaml-parse       read the current pin (and the version inside it)
yaml-update      rewrite every listed key            ─┐ only when the
git-commit       chore(<scope>): bump <name> to <v>   │ Warehouse's version
git-push         kargo/promotion/<promotion>          │ differs from the pin
git-open-pr      PR against main                     ─┘
git-merge-pr     if the autoMerge policy allows   ─┐ exactly one of
git-wait-for-pr  otherwise, until a human acts    ─┘ these runs
```

`autoMerge` is evaluated with `semverDiff(new, current)`: `patch` merges
patch/metadata changes, `minor` additionally merges minor bumps when the major
is greater than zero, `always` and `never` do what they say. Strategies without
a semver (Digest, NewestBuild, Lexical) only honour `always`/`never`.

## Render and check

```bash
helm template kargo-projects addons/charts/kargo-projects \
  -f addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
```

Validation against the Kargo CRD schemas is part of the addon's verification
routine in `docs/kargo.md`.
