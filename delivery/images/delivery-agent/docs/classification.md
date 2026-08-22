# Mechanical or escalate

The judgement the agent makes, with the worked examples the eval cases encode.

## Mechanical

The rendered diff **proves** the cause, and the fix is changing values that
already exist in the repository.

**A chart default flipped.** `argo-cd` 10.0.0 turns
`global.networkPolicy.create` on; this repository owns NetworkPolicies
elsewhere. Fix: pin it back to `false`. One scalar.

**Coupled pins.** `nginx-gateway-fabric` 2.6.7 requires Gateway API v1.5, and
the CRD addon is pinned at v1.4.0. Fix: move the CRD pin — *provided the exact
target version appears in the evidence*. If it does not, this becomes an
escalation, and the applier refuses the edit even if the model tries.

**A port moved under a policy.** MetalLB 0.16.0 moves metrics from 7472 to
9120 while a NetworkPolicy still names the old port. Scraping stops silently.
Fix: the port number. Note this touches a *different* component and is still
mechanical — what makes something escalate is the kind of change, not where
the fix lives.

## Escalate

**Any apiVersion change.** external-secrets 2.x stops serving `v1beta1` while
39 manifests still declare it. Rewriting those is a migration.

**Removed CRDs or dropped subcharts.** kyverno 3.9.0 drops the cleanup
subcharts several values keys point at, with seven minors of generate-rule
change behind a `failurePolicy: Fail` webhook.

**One-way migrations.** authentik refuses to migrate across major.minor
releases in one step — `ensure_allowed_version()` raises before
`run_migrations()`, so the pods take the database lock, refuse, and crashloop
while the old ones keep serving.

**An unstated version.** "requires Gateway API v1.5" does not say whether to
write v1.5.0, v1.5.1 or v1.5.4.

**Anything uncertain.** A wrong escalation costs two minutes. A wrong
mechanical fix renders perfectly and breaks at runtime, which is the failure
this system exists to prevent.

## No action

The gate is red for something this pull request did not cause — a pre-existing
failure in an untouched addon, for instance. Saying so is useful; changing
something is not.

## Calibration

Deliberately biased toward escalation. The cost is asymmetric and the agent is
not the last line of defence — the gate still has to go green and the merge
policy still applies.

The eval measures this. See [prompt-contract.md](prompt-contract.md).
