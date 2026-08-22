# Targets

A target says: *watch this artifact, and when a new version appears, rewrite
these lines and open a pull request.*

```yaml
projects:
  addons:                       # a Kargo Project, one namespace
    targets:
      cert-manager:             # a Warehouse + Stage pair
        chart:
          repoURL: https://charts.jetstack.io
          name: cert-manager
          constraint: ">=1.19.0 <2.0.0"
        autoMerge: patch
        updates:
          - file: clusters/hub/addons.yaml
            keys: [cert-manager.defaultVersion]
        verify:
          apps: [cert-manager-hub]
```

Exactly one of `image`, `chart` or `git` is required, and at least one entry in
`updates` (or in each entry of `stages` — see [chaining.md](chaining.md)).

## Merge policy

`autoMerge` is evaluated against the semver difference between the new version
and what is pinned:

| Value | Merges |
|---|---|
| `always` | anything the Warehouse produced |
| `minor` | patch and metadata, plus minors when major > 0 (0.x minors are breaking, per semver) |
| `patch` | patch and metadata only |
| `never` | nothing; always waits for a human |

For non-semver strategies (`Digest`, `NewestBuild`, `Lexical`) only `always`
and `never` mean anything; `minor` and `patch` behave as `never`, because there
is no difference to measure.

## The `yaml-update` rules, which are the fiddly part

These come from how Kargo rewrites files, not from this chart, and getting one
wrong fails in a way that is easy to miss.

- **The key must live in the FIRST YAML document of the file.** Kargo parses
  only that one. In a multi-document file, the resource you track has to come
  first — otherwise the path resolves against the wrong object and every
  promotion errors with something like `cannot fetch spec from <nil>`, from the
  day the target was written, reporting nothing anywhere else.
- **The key must already exist and address a scalar.** Kargo rewrites a line in
  place; it does not create structure.
- **No trailing `# comment` on that line.** It is deleted on the first update.
  Put the comment on the line above.
- **List indices are positional.** `additionalResources.10.image` means the
  eleventh element. Insert an entry above it and the target now edits something
  else — the failure is silent if the new element happens to have the same
  shape.
- **Write durations in Go's canonical form** — `12h0m0s`, not `12h`. Kargo's
  webhook rewrites them, and ArgoCD reads the difference as permanent drift.

## The semver prerelease trap

A tag with a `-suffix` is a semver **prerelease**. `3.6.8-0` and `7.4.11-alpine`
both parse that way, and a plain range constraint excludes prereleases by rule:

```
>=3.6.0 <3.7.0        3.6.8-0 -> no match
>=3.6.0-0 <3.7.0-0    3.6.8-0 -> matches, 3.7.0-0 still excluded
```

Pair such tags either with no constraint at all, or with bounds that carry
their own prerelease. Getting it wrong matches *nothing*: the Warehouse sits at
`NoImageReferencesDiscovered` with zero Freight and reports no error, so the
target is simply dead and looks idle.

## Verification

`verify.apps` names the ArgoCD Applications this pin feeds. After the pull
request merges, an Argo Rollouts AnalysisTemplate checks they are Synced and
Healthy.

Two things to be clear-eyed about:

- It runs **after** the merge. It records an outcome; it cannot prevent one.
  For that you want a pre-merge gate.
- Kargo *builds* AnalysisRuns but does not execute them. The Argo Rollouts
  **controller** must be installed. With only the CRDs present, runs sit with
  no phase forever and verification silently never happens.
