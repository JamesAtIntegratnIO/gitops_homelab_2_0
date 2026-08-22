#!/usr/bin/env bash
# helm lint every chart, and check each one against its own values.schema.json.
#
# The schema check is the point: it is the values contract for people who do
# not have this repository to read, and a schema that drifts from the chart is
# worse than no schema at all.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

command -v helm >/dev/null 2>&1 || { echo "lint needs helm on PATH and it is not there." >&2; exit 2; }

fail=0
ok()  { printf '  ok    %s\n' "$*"; }
bad() { printf '  FAIL  %s\n' "$*"; fail=1; }

shopt -s nullglob
charts=(charts/*/Chart.yaml)
if [ ${#charts[@]} -eq 0 ]; then
  echo "no charts yet -- nothing to lint"
  exit 0
fi

for c in "${charts[@]}"; do
  d="$(dirname "$c")"

  # A chart with required values cannot render bare; Helm's convention is a
  # ci/*-values.yaml fixture, and a chart that needs one but lacks it is a
  # chart nobody can lint.
  lintargs=()
  for v in "$d"/ci/*-values.yaml; do [ -f "$v" ] && lintargs+=(-f "$v"); done

  if helm lint "${lintargs[@]}" "$d" >/dev/null 2>&1; then
    ok "helm lint $d"
  else
    helm lint "${lintargs[@]}" "$d" 2>&1 | sed 's/^/        /'
    bad "helm lint $d"
  fi

  # `helm template` validates values against values.schema.json when present,
  # so a default set that violates its own schema fails here.
  if [ -f "$d/values.schema.json" ]; then
    if helm template lint-check "${lintargs[@]}" "$d" >/dev/null 2>&1; then
      ok "values.schema.json accepts the chart defaults ($d)"
    else
      helm template lint-check "${lintargs[@]}" "$d" 2>&1 | sed 's/^/        /' | head -10
      bad "chart defaults violate values.schema.json ($d)"
    fi
    if python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$d/values.schema.json" 2>/dev/null; then
      ok "values.schema.json is valid JSON ($d)"
    else
      bad "values.schema.json is not valid JSON ($d)"
    fi
  else
    bad "$d/values.schema.json is missing"
  fi
done

echo
[ "$fail" -eq 0 ] || { echo "LINT FAILED"; exit 1; }
echo "lint passed"
