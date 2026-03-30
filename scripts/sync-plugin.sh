#!/usr/bin/env bash
# scripts/sync-plugin.sh — DEV ONLY
#
# Run after editing .claude/commands/llmeval.md or commands/llmeval/harden.md
# to propagate changes to the plugin copy and reload.
#
# Not needed by end users — install.sh handles first-time plugin setup.
set -e

REPO="$(cd "$(dirname "$0")/.." && pwd)"
PLUGIN_COMMANDS="$REPO/.claude-plugin/plugin/commands"

echo "Syncing plugin commands..."

cp "$REPO/.claude/commands/llmeval.md"    "$PLUGIN_COMMANDS/llmeval.md"
cp "$REPO/commands/llmeval/harden.md"     "$PLUGIN_COMMANDS/llmeval/harden.md"

echo "Reinstalling plugin..."
claude plugins uninstall llmeval --scope project 2>&1
claude plugins install llmeval@llmeval --scope project 2>&1

echo "Done. Run /reload-plugins in Claude Code to pick up changes."
