#!/usr/bin/env bash
# Read-only evidence pass over Hermes Agent for the hermes-mechanic skill.
#
#   .claude/skills/hermes-mechanic/scripts/collect.sh [HOURS]   # default 168 (7 days)
#
# Changes nothing: kubectl get/logs, and `kubectl exec` of read-only commands
# inside the Hermes pod. Never prints .env, auth.json or any Secret value.
# Needs cluster access (holocron), so it cannot run from a cloud session.
set -uo pipefail
HOURS="${1:-168}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$here/../../homelab-toolchain/scripts/tools.sh" >/dev/null
CTX="${CTX:-admin@the-cluster}"
kk() { "$KUBECTL" --context "$CTX" "$@"; }
hx() { kk -n hermes exec -i deploy/hermes-agent -c hermes -- "$@"; }
# The venv's python, run the way Hermes's launchers do (source tree on the
# path), without writing .pyc files into the image.
hpy() { hx env PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=/opt/hermes /opt/hermes/.venv/bin/python - "$@"; }
section() { printf '\n===== %s =====\n' "$1"; }

section "pods (hermes, cq, mcp-system)"
for ns in hermes cq mcp-system; do
  kk -n "$ns" get pods -o custom-columns='NS:.metadata.namespace,POD:.metadata.name,READY:.status.containerStatuses[*].ready,RESTARTS:.status.containerStatuses[*].restartCount,LAST:.status.containerStatuses[*].lastState.terminated.reason,NODE:.spec.nodeName,AGE:.metadata.creationTimestamp' --no-headers
done
section "hermes-agent rollouts (ReplicaSets, newest last)"
kk -n hermes get rs -l app.kubernetes.io/name=hermes-agent --sort-by=.metadata.creationTimestamp \
  -o custom-columns='RS:.metadata.name,CREATED:.metadata.creationTimestamp,REPLICAS:.status.replicas' --no-headers | tail -10
section "warning events"
for ns in hermes cq mcp-system; do kk -n "$ns" get events --field-selector type=Warning --no-headers 2>/dev/null | tail -5; done
section "argocd apps"
kk -n argocd get applications --no-headers 2>/dev/null | grep -E '^(hermes|cq|mcp-system)-' || true
section "backups (last 3 jobs each)"
kk -n hermes get jobs --sort-by=.metadata.creationTimestamp --no-headers | tail -3
kk -n cq get jobs --sort-by=.metadata.creationTimestamp --no-headers | tail -3

section "hermes version, MCP servers, status"
hx hermes --version 2>&1 | head -3
hx hermes mcp list 2>&1 | head -40
hx hermes status 2>&1 | head -40

section "gateway_state.json (selected keys)"
hpy <<'PY' 2>&1
import json, pathlib
p = pathlib.Path("/opt/data/gateway_state.json")
if not p.exists():
    print("missing")
else:
    d = json.loads(p.read_text())
    def pick(x, depth=0):
        keep = ("state", "needs_attention", "parked_profiles", "last_heartbeat", "active_agents",
                "metrics", "requests", "tokens", "latency_p95", "platforms", "errors", "pid", "started_at")
        if isinstance(x, dict):
            return {k: (pick(v, depth+1) if depth < 3 else "...") for k, v in x.items()
                    if depth > 0 or any(s in k for s in keep)}
        return x
    print(json.dumps(pick(d), indent=1, default=str)[:4000])
PY

section "agent.log, last ${HOURS}h: levels and recurring signatures"
hpy "$HOURS" <<'PY' 2>&1
import re, sys, glob, collections, datetime as dt
hours = float(sys.argv[1]); since = dt.datetime.now() - dt.timedelta(hours=hours)
files = sorted(glob.glob("/opt/data/logs/agent.log*"), reverse=True)
line_re = re.compile(r"^(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d),\d+ (\w+)(?: \[[^\]]*\])? ([\w.]+): (.*)$")
levels = collections.Counter(); sigs = collections.Counter(); first = last = None
watch = {
  "mcp keepalive failed": r"keepalive failed",
  "mcp circuit breaker": r"rejected the last \d+ calls",
  "turns cut by restart": r"Gateway drain timed out",
  "gateway starts": r"Gateway (starting|started)|gateway run starting",
  "compression": r"[Cc]ompress(ed|ion)",
  "content_filter refusals": r"content_filter",
  "max_turns / budget": r"max_iterations|max_turns|budget",
  "tool loop guard": r"same_tool_failure|tool_loop|hard.stop",
  "approval denied/timeout": r"did not approve|approval (timed out|denied)",
  "skill write denied": r"Permission denied: .?/opt/hermes-skills",
  "config migration": r"config-migrate",
  "anthropic 429/529/overloaded": r"\b(429|529)\b.*(anthropic|rate|overload)|overloaded_error",
}
hits = collections.Counter()
for f in files:
    try: fh = open(f, errors="replace")
    except OSError: continue
    for line in fh:
        m = line_re.match(line)
        if not m: continue
        ts = dt.datetime.strptime(m.group(1), "%Y-%m-%d %H:%M:%S")
        if ts < since: continue
        first = min(first or ts, ts); last = max(last or ts, ts)
        lvl, name, msg = m.group(2), m.group(3), m.group(4)
        levels[lvl] += 1
        for k, rx in watch.items():
            if re.search(rx, msg): hits[k] += 1
        if lvl in ("WARNING", "ERROR", "CRITICAL"):
            sig = re.sub(r"\d+(\.\d+)?", "N", msg)
            sig = re.sub(r"'[^']{0,80}'", "'X'", sig)[:160]
            sigs[(lvl, name, sig)] += 1
print("window:", first, "->", last, "| files:", len(files))
print("levels:", dict(levels))
print("watched:", dict(hits))
print("top WARNING/ERROR signatures:")
for (lvl, name, sig), n in sigs.most_common(25):
    print(f"  {n:5d} {lvl:7s} {name}: {sig}")
PY

section "state.db (read-only)"
hpy <<'PY' 2>&1
import sqlite3, os
p = "/opt/data/state.db"
print("size MB:", round(os.path.getsize(p)/1e6, 1))
c = sqlite3.connect(f"file:{p}?mode=ro", uri=True)
tables = [r[0] for r in c.execute("select name from sqlite_master where type='table'")]
for t in ("sessions", "messages"):
    if t in tables:
        print(t, c.execute(f"select count(*) from {t}").fetchone()[0])
PY

section "effective context/compression settings vs upstream defaults"
hpy <<'PY' 2>&1
import yaml
from hermes_cli.config_defaults import DEFAULT_CONFIG as D
ours = yaml.safe_load(open("/opt/data/config.yaml")) or {}
print("_config_version ours/upstream:", ours.get("_config_version"), D.get("_config_version"))
for sec in ("compression", "agent", "prompt_caching", "delegation", "memory"):
    d = D.get(sec, {}); o = ours.get(sec, {}) or {}
    flat = {k: v for k, v in d.items() if not isinstance(v, dict)}
    print(f"[{sec}] overrides:", {k: o[k] for k in o if not isinstance(o.get(k), dict)})
    print(f"[{sec}] upstream defaults:", flat)
PY

section "hermes doctor --live (bounded read-only probes)"
hx hermes doctor --live 2>&1 | tail -40

section "local skills Hermes wrote itself (/opt/data/skills)"
hx sh -c 'find /opt/data/skills -name SKILL.md -newer /opt/data/config.yaml 2>/dev/null | head -20; find /opt/data/skills -name SKILL.md | wc -l'

section "upstream releases"
gh release list -R NousResearch/hermes-agent -L 5 2>/dev/null || echo "gh unavailable"
