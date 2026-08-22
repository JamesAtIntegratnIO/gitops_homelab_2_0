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

## `make seed` is a reset, not an update

It force-pushes `sample-repo/` over `main`. That is what makes a run
reproducible, and it also means re-seeding after a demo **discards the merge**
you just watched land. Run `make demo` against a seeded repo; re-seed when you
want to start over.

## Teardown

```bash
make down     # delete the cluster, keep the VM
make clean    # and stop colima
```
