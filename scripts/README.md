# Scripts

## Git Hooks

Run `./scripts/install-hooks.sh` to install the pre-commit hook that validates no `kind: Secret` resources exist in promise directories.

The hook source lives in [../.githooks/pre-commit](../.githooks/pre-commit).
