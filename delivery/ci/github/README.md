# GitHub Actions adapter

Reference implementation. This is the adapter that is actually exercised.

## Wiring

The workflow runs on `pull_request` for paths that affect what gets deployed,
and exposes a single aggregate job — that is the name to put in branch
protection.

## Branch protection with one committer

If the repository has one human committer, requiring status checks can lock
that person out of their own merges. Two ways through:

- **Classic branch protection:** leave *Include administrators* unticked. Your
  merges are unaffected; the bot's token is still held to the check.
- **Rulesets:** add a bypass for your own account, and **not** for the token
  the automation merges with. Getting this backwards is the failure mode —
  it exempts exactly the actor the gate exists to constrain.

## The token

Agent pushes need a token that is *not* `GITHUB_TOKEN`: pushes made with it do
not trigger workflows, so the gate would never re-run on a pushed fix.

Scope that token to Contents (write), Pull requests (write), Issues (write, for
labels), Commit statuses (write), Metadata (read) — and deliberately **not**
Workflows. Without the Workflows permission GitHub rejects any push touching
`.github/workflows/**`, which makes "the agent cannot edit the gate" a
server-side guarantee rather than a policy the agent is asked to respect.
