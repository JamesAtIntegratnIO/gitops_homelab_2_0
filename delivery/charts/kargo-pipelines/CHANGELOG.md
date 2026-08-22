# Changelog

All notable changes to `kargo-pipelines`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

Generalized from a working single-cluster chart. A repository migrating from
that chart renders byte-identical output with `nameLabel` set to its old value
— verified across 111 objects — so adopting this is a no-op until a target
actually declares `stages`.

### Added

- Documented that verification requires Prometheus to be scraping ArgoCD. The
  AnalysisTemplate queries `argocd_app_info`; with nothing scraping it, every
  AnalysisRun fails with an empty message and no component names the cause.
  Found by running a promotion on a cluster that had no ArgoCD ServiceMonitor.

- `git.insecureSkipTLSVerify`, applied to every git step. Needed more often
  than it looks: Kargo REFUSES to send credentials to a plain-HTTP endpoint
  ("refused to get credentials for insecure HTTP endpoint"), so a self-hosted
  host cannot be reached over `http://` to dodge a certificate problem — it
  has to be `https://`, and the certificate then has to be trusted or skipped.
  The failure without this is `git push` reporting `could not read Username`,
  which names neither cause.

- **Promotion chains.** A target may declare an ordered `stages` list. Each
  stage carries its own `updates` and `verify`, and downstream stages take
  their freight from the one before with `direct: false`. Kargo only offers
  *verified* freight downstream, so the gate needs no orchestration.
- `requiredSoakTime`, written on the stage doing the soaking and rendered onto
  the downstream stage's sources, where Kargo expects it.
- Per-stage `autoMerge`, so a canary can merge itself while the stage that
  reaches production still waits for a human.
- **Triage hook** — an optional `http` step firing when the pull request opens,
  carrying the freight context. `continueOnError` defaults true: a triage
  service that is down must never fail a promotion.
- Bounded `retry` on all three pull-request steps. Kargo's default
  `errorThreshold` is 1, meaning no retries at all, which turns a transient API
  error into a failed promotion.
- `values.schema.json`, including a required `git.repoURL` and canonical
  duration patterns.
- `nameLabel`, so a repository migrating from a differently-named chart does
  not churn the label on every object.

### Changed

- `git.repoURL` has no default and must be supplied.

### Notes

- The **last** stage in a chain keeps the target's bare name. Renaming the
  terminal Stage would discard its freight and verification history and make
  ArgoCD prune and recreate it.
- Each stage parses the first file of **its own** `updates`. Without that, a
  downstream stage compares against the pin its upstream already moved,
  concludes there is nothing to do, and silently never promotes.
