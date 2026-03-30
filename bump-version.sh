#!/usr/bin/env bash
# bump-version.sh <new-version>
# Updates the plugin manifest files and creates a git tag.
# The binary version is derived automatically from the git tag via debug.ReadBuildInfo.
#
# Usage: ./bump-version.sh 0.2.0
set -euo pipefail

NEW="${1:-}"
if [[ -z "$NEW" ]]; then
  echo "usage: $0 <version>  e.g. $0 0.2.0" >&2
  exit 1
fi

if ! [[ "$NEW" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: version must be X.Y.Z (e.g. 0.2.0)" >&2
  exit 1
fi

REPO="$(cd "$(dirname "$0")" && pwd)"

# If slash commands were edited, sync them to the plugin copy first.
if ! diff -q "$REPO/.claude/commands/llmeval.md" \
             "$REPO/.claude-plugin/plugin/commands/llmeval.md" > /dev/null 2>&1 || \
   ! diff -q "$REPO/commands/llmeval/harden.md" \
             "$REPO/.claude-plugin/plugin/commands/llmeval/harden.md" > /dev/null 2>&1; then
  echo "⚠ Slash commands differ from plugin copy — running scripts/sync-plugin.sh first..."
  bash "$REPO/scripts/sync-plugin.sh"
  echo ""
fi

# Update the three plugin manifest files.
# The binary version is NOT hardcoded — it comes from the git tag at install time.
sed -i'' "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW}\"/" \
  "$REPO/.claude-plugin/plugin.json"

sed -i'' "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW}\"/" \
  "$REPO/.claude-plugin/marketplace.json"

sed -i'' "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW}\"/" \
  "$REPO/.claude-plugin/plugin/.claude-plugin/plugin.json"

echo "Bumped plugin manifests to ${NEW}:"
echo "  .claude-plugin/plugin.json"
echo "  .claude-plugin/marketplace.json"
echo "  .claude-plugin/plugin/.claude-plugin/plugin.json"
echo ""
echo "Next:"
echo "  git add .claude-plugin/"
echo "  git commit -m \"chore: bump version to ${NEW}\""
echo "  git tag v${NEW} && git push && git push --tags"
echo ""
echo "After the tag is pushed, 'go install ...@latest' will report ${NEW} automatically."
