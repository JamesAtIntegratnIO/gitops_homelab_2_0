#!/usr/bin/env bash
# Print the system prompt, so evals measure the prompt that actually ships
# rather than a copy that has drifted from it.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
python3 - <<'PY'
import pathlib, re
s = pathlib.Path("prompt.go").read_text()
print(re.search(r'const systemPrompt = `(.*?)`\n', s, re.S).group(1), end="")
PY
