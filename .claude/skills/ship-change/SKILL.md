---
name: ship-change
description: Land a change in this repo end to end — branch, commit, PR, merge handoff, post-merge verification, OKF bundle update. Use when starting to commit work here, when asked to open or merge a PR, when asked to "deploy" or "carry it all the way", or when resolving a merge conflict.
---

# Landing a change

**ArgoCD reconciles from `origin/main`.** A commit on a branch changes nothing
in the cluster. Nothing is verified until it is on main and the app is
Synced/Healthy.

## Rules

1. **Never push to `main`.** Push a branch and hand the merge to James. The
   repo is trunk-based and single-committer. The one sanctioned exception is
   Kargo merging its own version-bump PRs under the per-target policy in
   [kargo-projects/values.yaml](../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml).
2. **Small, focused, conventional commits**, scoped like the existing history:
   `fix(mcp-system): …`, `chore(addons): …`, `docs(okf): …`.
3. **Assert every scripted edit.** A `.replace()` that matched nothing shipped a
   version bump that did not exist and released nothing, silently. If a script
   edits a file, check afterwards that the file changed the way you intended.
   This repo's whole thesis is that an operation quietly doing nothing looks
   exactly like one with nothing to do.
4. **`git fetch` before branching.** Merging with `gh` does not update your
   local `origin/main`; `git checkout -B <b> origin/main` then silently branches
   from an old commit.
5. **Session/handoff docs are working input, not repo artifacts.** Read one, act
   on it, and land the durable findings in the OKF bundle. Do not commit a dated
   session doc and then maintain it.

## Merge conflicts

This checkout uses git's **diff3** conflict style, which inserts a third *base*
section. A resolver — or a marker grep — that only knows the textbook three
markers ships a broken file that still validates:

```bash
grep -nE '^(<<<<<<< |\|\|\|\|\|\|\| |=======$|>>>>>>> )' <file>
```

Match the full `<<<<<<< ours … ||||||| base … ======= theirs … >>>>>>>` shape
and drop the base section entirely. For `docs/okf/log.md`, two branches adding
bullets under the same date resolve to the **union** — assert the merged section
equals the union of the sources before pushing.

## Pre-merge checks

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh

# addon targeting -- see the addon-change skill
./.claude/skills/addon-change/scripts/addon-targets.sh

$HELM template addons/charts/application-sets -f <values...>
$HELM template addons/charts/kargo-projects \
  -f addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml

cd promises/vcluster-orchestrator-v2/workflows/resource/configure && $GO build ./...
cd cli && $GO build -o /dev/null . && $GO test ./...     # the only real tests
cd images/platform-status-reconciler && $GO test ./...
```

**Nothing in this repo builds container images pre-merge.** Both Dockerfile
defects that ever shipped could only appear on main, and the second was masked
by the first. If a change touches a Dockerfile or a Go toolchain pin, say so
explicitly — CI's `setup-go` reads `go-version-file`, so every Go check runs on
the right toolchain while a hand-pinned Dockerfile toolchain drifts unseen.

The `addons-gate` check on the PR is run by Bosun in-cluster, not CI — see the
`gate-triage` skill for reading its verdict, and note that a green gate proves
only what the gate could actually run.

## After the merge

"Branch and hand over" is about not surprising James, not about stopping at the
PR. When he says merge and deploy, carry it all the way:

1. merge, wait for the release workflows,
2. refresh the Kargo warehouse:
   `k -n addons annotate warehouse <name> kargo.akuity.io/refresh="$(date -u +%Y-%m-%dT%H:%M:%SZ)" --overwrite`
3. merge the bump PR Kargo opens — **only** the PRs the refresh produced; held
   escalations are held on purpose,
4. wait for the ArgoCD rollout and verify the running pod.

```bash
k -n argocd get applications | grep -v 'Synced.*Healthy'
$ARGOCD app get <name>
```

## Then update the OKF bundle

[docs/okf/](../../../docs/okf/index.md) is the platform's long-term memory. A
stale bundle is worse than none, because it gets trusted. After a change that
alters reality:

1. update the affected concept file(s), keeping `sources`, `generated` and
   `stale_after` frontmatter honest,
2. add a line to [docs/okf/log.md](../../../docs/okf/log.md) under today's date,
3. `/okf:validate docs/okf --strict`.

## Non-negotiables

- **No `kind: Secret` anywhere near `promises/`** — the Kratix state repo is
  public. Use an `ExternalSecret` against the `onepassword-store`
  ClusterSecretStore. Enforced by `.githooks/pre-commit` and the
  `validate-promises` Action.
- **Never read or print `secrets.env`.**
- **Gateway API, not Ingress.**
- **Git is the source of truth** — direct `kubectl` mutation only for orphans,
  said out loud.
