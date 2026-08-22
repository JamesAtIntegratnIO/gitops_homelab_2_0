#!/usr/bin/env bash
# Proves this package can be lifted out of its host repository unchanged.
#
# Copies delivery/ alone into a scratch directory and asserts that nothing in
# it reaches outside itself, and that everything still builds there. Without
# this, the one-way link rule in CONTRIBUTING.md rots silently and the split
# stops being cheap.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT

fail=0
note() { printf '  %s\n' "$*"; }
bad()  { printf '  FAIL  %s\n' "$*"; fail=1; }
ok()   { printf '  ok    %s\n' "$*"; }

echo "==> copying package to $SCRATCH"
cp -R "$ROOT/." "$SCRATCH/"
cd "$SCRATCH"

# ---------------------------------------------------------------------------
# 1. No path escapes the package.
# ---------------------------------------------------------------------------
echo "==> rule 1: nothing references a path outside the package"

# Every relative markdown link must resolve to something that exists inside
# the copied tree. This is the precise form of the rule -- checking directory
# names by hand gets it wrong, because the package has its own docs/ and
# charts/ directories that look identical to the host's.
if python3 "$ROOT/hack/check_links.py" . ; then
  ok "every markdown link resolves inside the package"
else
  bad "markdown link does not resolve inside the package"
fi

# Host-repository top-level directories. Deliberately excludes docs/, charts/,
# images/ and ci/ -- the package has its own.
ESCAPES='\]\((/|(\.\./)*(addons|promises|workloads|terraform|matchbox|cli)/)'
if grep -rnE "$ESCAPES" --include='*.md' . 2>/dev/null \
     | grep -v '^\./hack/' >/dev/null 2>&1; then
  grep -rnE "$ESCAPES" --include='*.md' . 2>/dev/null \
     | grep -v '^\./hack/' | sed 's/^/        /' | head -20
  bad "link into a host-repository directory"
else
  ok "no links into host-repository directories"
fi

# Dockerfiles must not COPY from above their own context.
if grep -rn '^\s*COPY\s\+\.\./' --include='Dockerfile' . >/dev/null 2>&1; then
  bad "Dockerfile COPY reaches outside its context"
else
  ok "Dockerfile contexts are self-contained"
fi

# ---------------------------------------------------------------------------
# 2. No environment assumptions.
# ---------------------------------------------------------------------------
echo "==> rule 2: no environment assumptions"

# Literals from the host environment that must always be values.
LEAKS='the-cluster|vcluster-media|integratn\.tech|10\.0\.4\.|onepassword-store|jamesatintegratnio'
if grep -rniE "$LEAKS" --include='*.yaml' --include='*.yml' --include='*.go' \
     --include='*.json' --include='Dockerfile' . 2>/dev/null \
     | grep -v '^\./hack/extraction-test.sh' >/dev/null 2>&1; then
  grep -rniE "$LEAKS" --include='*.yaml' --include='*.yml' --include='*.go' \
     --include='*.json' --include='Dockerfile' . 2>/dev/null \
     | grep -v '^\./hack/extraction-test.sh' | sed 's/^/        /' | head -20
  bad "host-environment literal in shipped code"
else
  ok "no host-environment literals"
fi

# ---------------------------------------------------------------------------
# 3. Everything still builds here.
# ---------------------------------------------------------------------------
echo "==> everything builds from the extracted copy"

shopt -s nullglob
charts=(charts/*/Chart.yaml)
if [ ${#charts[@]} -eq 0 ]; then
  note "no charts yet -- skipping"
else
  for c in "${charts[@]}"; do
    d="$(dirname "$c")"
    if helm template test "$d" >/dev/null 2>&1; then
      ok "helm template $d"
    else
      helm template test "$d" 2>&1 | sed 's/^/        /' | head -10
      bad "helm template $d"
    fi
  done
fi

gomods=(images/*/go.mod)
if [ ${#gomods[@]} -eq 0 ]; then
  note "no Go modules yet -- skipping"
else
  for m in "${gomods[@]}"; do
    d="$(dirname "$m")"
    if (cd "$d" && go build ./... >/dev/null 2>&1); then
      ok "go build $d"
    else
      (cd "$d" && go build ./... 2>&1) | sed 's/^/        /' | head -10
      bad "go build $d"
    fi
  done
fi

# ---------------------------------------------------------------------------
# 4. Every unit documents itself.
# ---------------------------------------------------------------------------
echo "==> every unit documents itself"
for c in "${charts[@]}"; do
  d="$(dirname "$c")"
  for f in README.md CHANGELOG.md values.schema.json; do
    [ -f "$d/$f" ] && ok "$d/$f" || bad "$d/$f is missing"
  done
done
for m in "${gomods[@]}"; do
  d="$(dirname "$m")"
  [ -f "$d/README.md" ] && ok "$d/README.md" || bad "$d/README.md is missing"
done

echo
if [ "$fail" -ne 0 ]; then
  echo "EXTRACTION TEST FAILED -- see CONTRIBUTING.md"
  exit 1
fi
echo "extraction test passed"
