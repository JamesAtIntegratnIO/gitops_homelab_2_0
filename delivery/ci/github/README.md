# GitHub Actions adapter

Reference implementation. This is the adapter that is actually exercised.

## Wiring

[`validate-addons.yaml`](validate-addons.yaml) is a copy-and-adjust template:
change the `paths:` filter to match where your addon values live, pin
`GATE_IMAGE` to a digest, and drop it in `.github/workflows/`.

It runs on `pull_request` and exposes a single aggregate job, `addons-gate` —
that is the name to put in branch protection.

Two details in it are not obvious and are load-bearing:

- `fetch-depth: 0`, because the gate renders the merge base as well as the head.
- The gate config is taken from the **head** at both revisions. It describes how
  to render, not what to render, and the base commit may predate it — which is
  exactly the case on the pull request that introduces the gate.

## Branch protection with one committer

If the repository has one human committer, requiring status checks can lock
that person out of their own merges. Two ways through:

- **Classic branch protection:** leave *Include administrators* unticked. Your
  merges are unaffected; the bot's token is still held to the check.
- **Rulesets:** add a bypass for your own account, and **not** for the token
  the automation merges with. Getting this backwards is the failure mode —
  it exempts exactly the actor the gate exists to constrain.

## Rolling it out onto pull requests that already exist

Order matters, and the middle step is the one people skip:

1. **Merge the gate to `main`.** Nothing happens to open pull requests yet —
   merging a workflow does not retroactively trigger runs on them.
2. **Watch it run green on a handful of real pull requests** before protecting
   anything. A gate with no track record is not evidence.
3. **Enable branch protection on `addons-gate`.** Every open pull request is
   now blocked, because none has reported the check. This is the protection —
   it does not require them to have run.
4. **Rebase each open pull request.** That fires `synchronize`, the check
   finally runs, and each gets a verdict. `pull_request` workflows are read
   from the merge commit, so a branch that predates the gate still picks it up.

## The token

Agent pushes need a token that is *not* `GITHUB_TOKEN`: pushes made with it do
not trigger workflows, so the gate would never re-run on a pushed fix.

Scope that token to Contents (write), Pull requests (write), Issues (write, for
labels), Commit statuses (write), Metadata (read) — and deliberately **not**
Workflows. Without the Workflows permission GitHub rejects any push touching
`.github/workflows/**`, which makes "the agent cannot edit the gate" a
server-side guarantee rather than a policy the agent is asked to respect.
