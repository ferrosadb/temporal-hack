#!/bin/bash
# require-branch.sh - Blocks code edits when on main/master branch
# Installed by /op-init as a PreToolUse hook for Edit|Write|NotebookEdit
#
# Exit codes:
#   0 = allow (not on protected branch, or not in a git repo)
#   2 = block (on protected branch)

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

BRANCH=$(git branch --show-current 2>/dev/null)

if [[ "$BRANCH" == "main" || "$BRANCH" == "master" ]]; then
  echo "BLOCKED: Cannot edit files on the '$BRANCH' branch. Create a feature branch first:" >&2
  echo "  git checkout -b feature/<description>" >&2
  exit 2
fi

exit 0
