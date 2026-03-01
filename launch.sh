#!/bin/sh
set -e

RALPH_DIR="${RALPH_DIR:-$HOME/.local/state/ralph}"

git fetch origin
git reset --hard origin/main

go build -o ralph-logs .

clear

exec ./ralph-logs \
  "$RALPH_DIR/logs/ralph-runs.log" \
  "$RALPH_DIR/logs/ralph-plans.jsonl" \
  "$RALPH_DIR/goals/*/ralph.log"
