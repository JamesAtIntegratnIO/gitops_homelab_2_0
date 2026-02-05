#!/usr/bin/env bash
# Install git hooks for the repository

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
HOOKS_DIR="${REPO_ROOT}/.githooks"
GIT_HOOKS_DIR="${REPO_ROOT}/.git/hooks"

echo "Installing git hooks..."

# Ensure hooks are executable
chmod +x "${HOOKS_DIR}"/*

# Configure git to use custom hooks directory
git config core.hooksPath "${HOOKS_DIR}"

echo "✓ Git hooks installed successfully"
echo "  Hooks directory: ${HOOKS_DIR}"
echo ""
echo "Installed hooks:"
ls -1 "${HOOKS_DIR}"
