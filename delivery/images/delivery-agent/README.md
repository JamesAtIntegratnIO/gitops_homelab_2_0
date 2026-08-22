# delivery-agent

The judgement half of the delivery gate. It reads a red gate, explains it on the
pull request, and fixes the cases that are mechanically provable from the
rendered diff. Everything else it escalates.

Triggered by Kargo's `http` promotion step — event-driven, never polled.

> **Status: not yet implemented.** This README is the contract the
> implementation is written against.

## What it will and will not do

**Fixes autonomously** — only where the render diff *proves* the cause:
a chart default that flipped, a pin that must move with another pin, a metrics
port that moved while a NetworkPolicy still names the old number.

**Escalates** — an API version change, a removed CRD, a dropped subchart, an
upstream note mentioning a schema or database migration, a version skip the
chart itself refuses, or any fix needing a file outside the addon's own tree.

It comments, labels, and stops. It never closes a pull request.

## The safety model is code, not prompt

The model does not edit files. It returns a structured verdict and a proposed
edit set; the agent applies those edits deterministically behind a path
allowlist. So "never edit the gate, never weaken a policy to go green" is an
invariant the agent enforces, not an instruction the model is asked to respect.

See [`../../adr/0001-structured-edits-not-agentic-loop.md`](../../adr/0001-structured-edits-not-agentic-loop.md).

## Reference

- [`docs/safety-model.md`](docs/safety-model.md) — allowlist, attempt cap, what is enforced where
- [`docs/classification.md`](docs/classification.md) — mechanical vs escalate, with worked examples
- [`docs/llm-providers.md`](docs/llm-providers.md) — the `LLMProvider` interface; adding one
- [`docs/git-providers.md`](docs/git-providers.md) — the `GitProvider` interface; adding one
