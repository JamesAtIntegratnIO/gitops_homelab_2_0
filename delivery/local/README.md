# Local proving ground

A disposable cluster where the whole delivery flow runs end to end: a chart
version is discovered, promoted, written onto a branch, opened as a pull
request, gated, triaged by the agent, merged, reconciled, verified, and
observed.

It exists because everything in this package was previously only ever exercised
in production, one merge at a time. The gate had never seen a real pull request.
No promotion had traversed a chain. The agent had never triaged anything.

## What it builds

| Piece | What runs it |
|---|---|
| kind cluster, ArgoCD, Gitea, ingress | [idpbuilder](https://cnoe.io/docs/reference-implementation/local) |
| cert-manager, Argo Rollouts, Prometheus, Grafana, Kargo | helm |
| kargo-pipelines, kargo-observability, delivery-agent | helm, from **this working tree** |
| the repository under test | `sample-repo/`, pushed into Gitea |

The agent image is **built from your working tree**, not pulled. A proving
ground that tests the last published image is testing the past.

## Requirements

- macOS or Linux, ~10 GB free RAM, ~20 GB disk
- Homebrew (the runtime script installs colima, kind and idpbuilder)
- An OpenAI-compatible model endpoint the cluster can reach

```bash
export LLM_BASE_URL=http://<your-host>:1234/v1
make up
make demo
```

`LLM_BASE_URL` has no default on purpose. A demo that silently starts spending
money against a vendor you did not choose is a bad default.

## The flow, and what each step proves

1. **Discovery** — a Warehouse finds a new podinfo chart version
2. **Promotion** — the Stage rewrites the pin and pushes a branch
3. **Pull request** — opened against Gitea. Real PR, real API, not a stand-in
4. **Gate** — renders base and head, diffs the resources, posts a report
   comment and a `gate` commit status
5. **Triage** — the agent reads that comment and decides
6. **Merge**
7. **Reconcile** — ArgoCD syncs podinfo to the new version
8. **Verify** — the AnalysisRun asks Prometheus whether the app is healthy
9. **Observe** — every `kargo_*` metric must **return rows**

Step 9 is the one that earned its place. In production every alert expression
parsed against a live Prometheus and matched nothing for hours, because
kube-state-metrics prefixes custom-resource metrics unless told not to. Parsing
is not evidence. The demo asserts a non-empty result.

## Where this is a stand-in rather than the real thing

**The gate runs as a binary, not as CI.** idpbuilder ships no Actions runner, so
[`scripts/gate-run.sh`](scripts/gate-run.sh) invokes the same binary with the
same inputs and produces the same two artifacts a CI adapter would — the report
comment and the commit status. `sample-repo/.gitea/workflows/gate.yaml` is there
for anyone who wires a runner up. Everything else is the real component.

## Things this turned up

Each of these is a real defect or a real gap, found by running the thing:

- **The agent could not talk to Gitea at all.** `GIT_PROVIDER` accepted only
  `github`. There is a `gitprovider/gitea.go` now.
- **Kargo refuses to send credentials over plain HTTP.** The controller logs
  `refused to get credentials for insecure HTTP endpoint`; the promotion fails
  at `git push` with `could not read Username`, which names nothing. So the
  git host has to be HTTPS, which for a self-hosted instance means a
  certificate — hence `git.insecureSkipTLSVerify` on kargo-pipelines.
- **An in-cluster destination cannot be expressed as an ipBlock.** A ClusterIP
  is DNAT'd to a pod IP before policy evaluation, so the agent's egress rule
  matched nothing and the connection hung with zero bytes. The chart takes
  `networkPolicy.egress.namespaces` now.
- **kube-state-metrics reads its config once, at startup.** Changing the
  ConfigMap changes nothing until it restarts.
- **Verification silently requires Prometheus to scrape ArgoCD.** The
  AnalysisTemplate queries `argocd_app_info`; idpbuilder ships ArgoCD's metrics
  Services but no ServiceMonitor, so every AnalysisRun failed with an empty
  message. `count(argocd_app_info)` went 0 -> 6 once one existed, and the
  verification query started returning 1.

## Seeing it actually fix things

```bash
make demo-scenarios              # all nine
make demo-scenarios CASE=metallb # just one
```

Nine incidents that really happened to this platform, replayed against the
**live** agent on **real** pull requests. They are not invented: each is
already written down once as an eval fixture, and this reads those same
fixtures so the thing the eval measures and the thing you watch cannot drift.

Four of them are mechanical, and on those the agent pushes a commit. MetalLB
0.16.0 swapping its FRR sidecars for a DaemonSet, for instance:

```
==> metallb-frr-defaults-flip  (expected: mechanical)
  bump metallb chart 0.15.2 -> 0.16.0
  pull request #8
  gate report posted, status gate=failure
  ok    pushed a fix: f27ac09 -> 7a11247
      -        enabled: true
      +        enabled: false
      -      enabled: true
      +      enabled: false
      Pushed a fix to `scenario/metallb-frr-defaults-flip` (attempt 1 of 2).
```

The other five are escalations and a no-action, and it should not touch those.

**What is replayed and what is live.** The gate's REPORT is the recorded one
from each incident, posted the way the gate posts it -- reproducing fourteen
upstream chart versions locally would prove nothing extra. The agent, the
model, the pull requests, the reasoning and every commit it pushes are live.

The summary shows whether it *edited*, not whether the edit was *right*. The
exact scalars are checked by the eval suite: `go test ./evals/...` in
`images/delivery-agent`.

## What the agent will and will not fix

`make demo-triage` opens a pull request the gate **refuses** -- a bump
carrying a changed destination namespace -- and the agent escalates rather
than fixing it. That is worth understanding before you call it a limitation of
the model. Measured here against both
`qwen/qwen3.5-9b` and `qwen/qwen3.8-27b`, each independently escalated with a
sound argument -- the 27B's was *"the cause is not provable from the rendered
diff alone"*, which is precisely the judgement the prompt asks for.

The deeper reason is structural, and it is the most useful thing this proving
ground has turned up:

| | Blocks the merge | Agent's mechanical class |
|---|---|---|
| Targeting moved | yes | escalate |
| Source / project / namespace changed | yes | escalate |
| apiVersion migration | yes | **always** escalate |
| A chart default flipped | no, reported only | mechanical |
| Coupled pins | no, reported only | mechanical |
| A port moved under a policy | no, reported only | mechanical |

Everything the gate blocks on is structural, and the agent escalates
structural changes by design. Everything the agent can mechanically fix is a
values conflict, which the gate reports without blocking. **The two sets
barely intersect**, so "gate red, agent fixes it" is close to a null case
today. That is a design question about where each half draws its line, not a
bug in either, and it is worth answering before the agent is trusted to push
fixes anywhere that matters.

## Running it a second time

`make demo` **consumes** the Freight it promotes. Run it again as-is and it
fails at step 2 with `a promotion exists (waited 240s)`, because the Stage is
already fulfilled and Kargo has nothing left to promote.

`make seed` alone does not fix that. It force-pushes `sample-repo/` back over
`main` — discarding the merge you just watched land — but leaves Kargo holding
the Freight it already promoted. Both sides have to go back:

```bash
make reset && make demo
```

## Teardown

```bash
make down     # delete the cluster, keep the VM
make clean    # and stop colima
```
