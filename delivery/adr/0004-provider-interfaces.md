# 4. Git host and model are interfaces, with no default model

- **Status:** accepted
- **Date:** 2026-08-22

## Context

The agent talks to two outside systems: a git host, to comment, label, push and
set commit statuses; and a model, to classify a failure and propose edits.

Both are places where a package like this usually hardcodes whatever its author
happened to use, and both are the first thing a new adopter needs to change.

## Decision

Two narrow interfaces.

**`GitProvider`** — `comment`, `label`, `push`, `setCommitStatus`. Four methods,
chosen because they are what the workflow actually needs and nothing more.
Kargo already models the same axis on its promotion steps, so the value flows
through to both.

**`LLMProvider`** — one call: given a prompt and a schema, return a structured
result. Two implementations, Anthropic Messages and OpenAI chat completions,
with `baseURL` as a value so self-hosted and gateway deployments work without
new code.

**No default model provider.** The values file must name one. A chart that
installs cleanly and then quietly makes paid API calls to a vendor the operator
did not choose is a bad default, however convenient.

## Consequences

**Good.** Adding a git host is one implementation of four methods, with no
change to the triage logic. Adding a model provider is one method. Both are
testable with fakes. An operator running a self-hosted model has a first-class
path, not a workaround.

**Bad.** The narrow `GitProvider` will not cover everything — GitHub's checks
API, GitLab's merge trains and Bitbucket's build statuses are not the same
shape, and pushing them through four methods means the lowest common
denominator. When a host needs something genuinely different, the interface
grows rather than the implementation special-casing.

Requiring an explicit provider costs every operator one values line and one
"why did this fail to install" moment. That is the intended trade.

**Also.** Only one implementation of each is exercised in practice at first.
The others are written against documentation, and should be treated as
unproven until someone runs them.

## Alternatives rejected

- **Shell out to `gh` / `glab`.** Fast to write, and makes the container carry
  a CLI per host plus its auth model. Rejected.
- **One provider behind a proxy.** Pushes an infrastructure dependency onto
  the operator before they can run anything.
- **Default to a hosted model.** Better first-run experience, at the cost of
  spending someone's money by default.
