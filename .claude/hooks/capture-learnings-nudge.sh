#!/usr/bin/env bash
# Opt-in Stop hook: nudge to run /capture-learnings after plan work.
# Prints a reminder ONLY when a plan file was modified in the last 30 minutes.
# It never writes the wiki. Enable by referencing it from settings.json "hooks.Stop".
set -euo pipefail
PLANS_DIR="docs/superpowers/plans"
[ -d "$PLANS_DIR" ] || exit 0
if find "$PLANS_DIR" -name '*.md' -mmin -30 -print -quit | grep -q .; then
  echo "A plan under docs/superpowers/plans was touched recently. Consider running /capture-learnings to update the memory wiki."
fi
exit 0
