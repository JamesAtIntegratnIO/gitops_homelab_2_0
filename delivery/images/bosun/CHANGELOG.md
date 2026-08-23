# Changelog

All notable changes to `bosun`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

- `AGENT_BRAND` / `AGENT_BRAND_MARK`. Comments lead with the mark and name, so
  a reader knows it is a bot before reaching the verdict rather than after,
  and the footer names the model and says "automated triage, not a review".
  The attempt label follows the brand too -- it was hardcoded to
  `bosun/attempt-`, and since the attempt CAP counts those labels, a
  rename would have silently reset the cap.

- `gitprovider.Gitea`. Gitea's API is deliberately GitHub-shaped, so most of
  it is the same request against a different base — but three places are not,
  and each fails silently rather than loudly: there is no check-runs API, so
  everything including Gitea Actions reports as a commit status; labels attach
  by numeric ID on older versions, and posting names returns 200 and attaches
  nothing, which would break the attempt cap into an infinite loop; and
  self-hosted is the normal case, so the instance URL is required and a
  self-signed certificate is expressible via `GIT_INSECURE_SKIP_TLS_VERIFY`.
- `GIT_INSECURE_SKIP_TLS_VERIFY`. Scoped to the git client and to the clone it
  pushes from — never to the process, and never to a global git config.

- `POST /v1/promotion-opened` — answers `202` immediately and triages
  asynchronously. Kargo's `http` step is synchronous, so a blocking handler
  would put a model round trip inside every promotion's critical path.
  Duplicate calls for the same pull request collapse, because a retried step
  must not start a second triage.
- `llm.Provider` with OpenAI chat-completions and Anthropic Messages
  implementations. `baseURL` is configuration, so self-hosted endpoints
  (LM Studio, Ollama, vLLM) are a first-class path.
- `gitprovider.Provider` with a GitHub implementation. GitLab and Bitbucket
  are documented extension points.
- `edits` — deterministic application behind a path allowlist, a `from`-value
  match, and a corroboration check on version-shaped values.
- `evals` — nine triage cases taken from real incidents, with a harness that
  scores classification, applied edits, and separately whether anything
  **unsafe** landed.

### Notes

- Measured on a 9B local model: 8/9 classification, 8/9 full pass, 0 unsafe.
  The scalar inventory is what makes that possible — 6/9 without it.
- A `mechanical` verdict whose edits are all refused escalates rather than
  reporting success. This is what turns model miscalibration into a safe
  outcome automatically.
- The model is never given file-edit tools, and the deny-list refuses CI
  config, the gate, and the merge policy regardless of how the allowlist is
  configured.
