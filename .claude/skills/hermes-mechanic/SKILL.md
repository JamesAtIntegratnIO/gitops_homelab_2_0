---
name: hermes-mechanic
description: Act as Hermes Agent's mechanic — read its logs, state, config and skills from outside, find what is degrading it, and fix Hermes itself through PRs. Use for the scheduled mechanic review, when a platform.hermes alert fires, when Hermes's answers get thin, forgetful or slow, when a Hermes skill or MCP server stops working, or before and after a Hermes upgrade.
---

# Hermes mechanic

Hermes Agent runs the cluster; Claude keeps Hermes running well. This is the
pattern from Zack Proser's
["My AI agent has a mechanic agent"](https://zackproser.com/blog/my-ai-agent-has-a-mechanic-agent):
the production agent cannot see its own buffers, defaults and broken skills,
but an agent with access to its config, logs and memory can, and fixes them
at the cause. His one-line fix was a conservative context default that
compounded until "thin" answers were all that reached the model.

What is under the hood: [docs/okf/addons/hermes.md](../../../docs/okf/addons/hermes.md).

## Rules

- **Fix Hermes, not the cluster.** James's direction (2026-11-08): cluster
  problems are Hermes's job. If the review finds a broken cluster thing,
  the fix is to give Hermes the tool, access or skill to handle it, not to
  `kubectl` it yourself.
- **Everything by PR.** Config (`hermes-config` ConfigMap, SOUL.md), skills
  (`skills.yaml` + mount), image (`images/hermes-agent`), alerts. Use
  `ship-change`; never push to `main`. The OKF concept and `log.md` change in
  the same PR (`okf-update` rules).
- **Read-only on the live pod.** `collect.sh` only reads. No `hermes config
  set` (it is overwritten at the next start anyway), no edits in `/opt/data`,
  no `hermes pause` unless James asks.
- **Never print secrets.** Not `.env`, not `auth.json`, not Secret values,
  not `secrets.env`. `/health/detailed` needs `API_SERVER_KEY`; use
  `gateway_state.json` instead.
- **Every kubectl carries `--context`** (`admin@the-cluster`; the script does it).
- **Say what is untested.** Hermes reads config only at start, so nothing is
  verified until the pod has rolled with it on `main`.

## 1. Collect (on holocron)

```bash
.claude/skills/hermes-mechanic/scripts/collect.sh 168   # hours of agent.log to read
```

It prints pods/restarts/rollouts for `hermes`, `cq`, `mcp-system`, ArgoCD
state, backups, `hermes mcp list` / `status`, `gateway_state.json`, an
`agent.log` digest (levels, watched patterns, top WARNING/ERROR signatures),
state.db counts, our context/compression settings next to upstream defaults,
`hermes doctor --live`, recently self-written skills, and upstream releases.

Loki has the log too since 2026-11-26: `{namespace="hermes", container="agent-log"}`
(the pod keeps only ~8 days in `agent.log*`).

## 2. Diagnose — where Hermes degrades

Work through these; each has bitten an agent like this.

| Look at | Healthy | What a problem looks like |
|---|---|---|
| Restarts and rollouts | few; drain lets turns finish | `Gateway drain timed out ... N active agent(s)`; many ReplicaSets in a day (Reloader churn from ConfigMap edits) |
| MCP servers | all listed, none parked | `keepalive failed` repeating, `rejected the last N calls`, a server missing from `hermes mcp list` (a Service created after Hermes started stays parked until restart) |
| Context and compression | compression rare, at the threshold | frequent compressions, compression far below the window, `budget`/`max_iterations` hits, refusals (`content_filter`). Compare our overrides with upstream defaults: conservative defaults compound |
| Skills | git skills read-only; local skills few and good | `Permission denied: /opt/hermes-skills` (lesson lost: route via skill-improve), `same_tool_failure` on `skill_manage`, a local skill shadowing a git skill's name |
| Memory (cq, MEMORY.md) | proposals land in the cq review queue | cq errors, MEMORY.md at its char limit, the same lesson re-learned every week |
| Config | `_config_version` equals the image's | `Migrating config schema 0 ->` at boot; keys upstream renamed or dropped (the migrator logs them) |
| Image | current upstream release, no open advisory | a newer release with a security fix (read its notes; v0.23.x notes are rollups) |
| Cost | Haiku on side tasks, cache TTL auto | Sonnet on aux tasks after an upgrade reset a default; token totals jumping in `gateway_state.json` metrics |

Read the session where a symptom happened before changing anything; the
log's `[session]` tag names it.

## 3. Fix

One PR per theme, smallest change that removes the cause:

- **Config / SOUL.md**: `addons/cluster-roles/control-plane/addons/hermes/hermes-agent.yaml`
  (`hermes-config`). Check every key against the running version's
  `hermes_cli/config_defaults.py` (tag-pinned GitHub source) before using it.
- **Skills**: `skills.yaml` (+ a `subPath` mount in `hermes-agent.yaml` for
  a new one); bump the skill's `version`.
- **Image**: bump the `FROM` digest in `images/hermes-agent/Dockerfile` and
  the tag in `.github/workflows/build-hermes-agent-image.yaml`; after CI
  publishes, pin the new digest in `hermes-agent.yaml` **and**
  `hermes-backup.yaml`, with `_config_version` moved to the image's value.
- **Alerts / log counters**: `platform-alerts/prometheus-rules.yaml`
  (`platform.hermes`) and `promtail/values.yaml` (`hermes_*` counters).

PR body: the evidence (log lines with counts, the setting and its default),
the fix, and what will prove it worked after the roll.

## 4. Verify after merge

ArgoCD syncs `hermes-the-cluster`, Reloader rolls the pod. Re-run
`collect.sh 2` and confirm the symptom's signature stopped, `hermes mcp list`
is complete, and nothing new shows up in the top signatures. Then record
durable lessons in project memory and the OKF bundle.

## Report

End with a short report for James: what was degrading Hermes (with
numbers), the PRs, what is verified, and what is left.
