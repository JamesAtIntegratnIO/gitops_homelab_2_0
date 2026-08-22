# Safety model

The agent can write to a repository and spend money. Both are bounded by code,
not by instructions to a model.

## The model never edits anything

It is asked one question and returns one structured answer: a classification,
an explanation, and — only for the mechanical case — a list of proposed scalar
edits. This process decides what, if anything, happens next.

That is the whole design. A model with file-edit tools can make a red gate
green by deleting the check, and that failure is indistinguishable from success.

## What is enforced, and where

| Guarantee | Mechanism |
|---|---|
| Cannot edit CI config, the gate, or the merge policy | `edits.DefaultDeny`, checked before any write and not overridable by configuration |
| Cannot edit outside the configured area | `Policy.Allow`; an empty allowlist refuses everything, and the process refuses to start with one |
| Cannot overwrite a value it misread | the edit's `from` must equal what the file holds |
| Cannot invent a version | version-shaped values must appear in the evidence the model was shown |
| Cannot add or restructure | the key must already resolve to an existing scalar |
| Cannot escape the repository | path traversal is rejected after cleaning |
| Cannot retry forever | attempt cap, tracked by pull-request label |
| Cannot write to the default branch | the only push path targets the pull request's own branch |

## Why the deny-list is not configurable

Every entry is a way to make a red gate green without fixing anything:

```
.github/**            the workflows that run the gate
.gitops-gate.yaml     what the gate renders, and how
.gitops-gate/**       the cluster inventory it compares against
delivery/**           the kit itself, including this agent and its prompt
**/kargo-*/**         the merge policy and version constraints
```

An operator can add to the deny-list. They cannot remove from it.

## Why the attempt cap is a label

Labels live on the pull request, so the cap survives a restart, a rescheduled
pod, and a second replica. In-memory state would reset every time the pod
moved, which is exactly when a loop would be most expensive.

## Failure is always visible

Three rules, because a silent agent is worse than none:

- A refused edit is reported in the pull-request comment, with the reason. A
  silent refusal would let a reader believe a fix had been applied.
- A `mechanical` verdict that applies nothing escalates. The model may be
  wrong; the outcome is still a human being asked.
- A model that is unreachable, slow or misconfigured produces a comment saying
  so. Silence would be indistinguishable from "nothing was wrong".

## What it does not do

It never closes a pull request, never merges one, never touches the cluster.
Its RBAC is read-only, and its entire write surface is a bot branch that still
has to pass the gate and the merge policy to reach anywhere.
