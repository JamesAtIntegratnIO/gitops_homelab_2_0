# Rendered manifests, and ArgoCD's source hydrator

Short version: **the source hydrator cannot feed a pre-merge gate**, but it can
give you an exact baseline, and if your repository renders manifests into git
by any means the gate will diff them at resource level — which is a far
stronger signal than anything derivable from Application definitions.

## Why the hydrator does not gate a merge

ArgoCD's source hydrator (beta since v3.5) renders your dry source and commits
plain YAML to a hydrated branch. It looks like it should solve the diff problem
outright. It does not, for one reason:

> "The hydrator triggers only when a new commit is detected in the dry source."

Hydration is a function of the **configured** dry revision — whatever is on
`main`. It never runs for a pull request's head commit. There is no dry-run
API, no preview endpoint, and no commit-server call that renders without
committing. The only way to hydrate a proposed change is to create a throwaway
Application pointing at the PR branch, which needs a live ArgoCD, git write
credentials, and leaves real commits behind.

`hydrateTo` is often mistaken for the answer. It pushes hydrated output to a
staging branch so something else can PR it onward:

> "Argo CD will only push changes to the hydrateTo branch, it will not create a
> PR or otherwise facilitate moving those changes to the syncSource branch."

That is a **deploy gate**, not a merge gate. The sequence is: merge to `main` →
hydrate → push to `environments/dev-next` → your tooling opens
`dev-next → dev`. By the time a rendered diff exists, the code change is
already in.

Worth noting that Kargo does the thing the hydrator explicitly declines to do:
`helm-template` / `kustomize-build` → `git-commit` → `git-open-pr`. If you want
rendered YAML in a pull request *before* merge, that is the mechanism.

## What this gate does with it

**`type: rendered`** reads manifests already committed to git — hydrator
output, Kargo's rendered promotion branches, or any CI job that commits its
render:

```yaml
sources:
  - name: hydrated
    type: rendered
    paths: ["environments/prod/**/manifest.yaml"]
```

Objects from these sources are diffed at resource level: added, removed,
changed, and — called out separately — **apiVersion changed**, which is the one
that blocks. An API version moving under an existing resource is a migration,
and migrations are precisely the class of change that renders perfectly and
breaks at runtime.

Using a hydrated branch as the **baseline** works well: `hydrator.metadata`
carries `drySha`, so you can tie rendered output back to the commit that
produced it. Since ArgoCD v3.3 the last-hydrated SHA lives in a git note
(`refs/notes/hydrator.metadata`) and the branch has *no commit* when the render
did not change — so map hydrated to dry via the note, never the commit log.

## What is still missing, and it matters

If your repository does **not** render manifests into git, the object diff is
empty. This gate then compares Application definitions — which Applications
exist, on which clusters, at which chart versions — and that is strictly less
information than "what will actually change in the cluster".

Concretely: a chart whose defaults flip, adding NetworkPolicies you did not
ask for, is a one-line version change at Application level and an obvious
addition at object level. The second is what a reviewer needs.

Closing that without rendered manifests means rendering each changed chart at
both versions and diffing the output — the same work ArgoCD's repo-server does.
Until that exists, be clear-eyed that a version-only report is a weaker signal
than it looks, and that a triage agent reading it is reasoning from less than
it appears to have.

## Things ArgoCD does not give you

Worth stating because they look like they should exist:

- **Nothing offline expands ApplicationSets.** `argocd appset generate` looks
  like the offline tool and is an RPC to a live API server, as is
  `--dry-run`. Rendering generators yourself is the only offline route.
- **The hydrator is single-source.** `sourceHydrator` and `sources` are
  mutually exclusive, so Helm values from a second repository do not work with
  it.
- **The hydrator does not sign commits**, so a hydrated branch with signature
  verification enabled will reject them.
