#!/bin/sh
RALPH_DIR="${RALPH_DIR:-$HOME/.local/state/ralph}"

./logtail 4000 \
  "$RALPH_DIR/logs/ralph-runs.log" \
  "$RALPH_DIR/clones/mgreenly/ikigai/*/.pipeline/cache/ralph.log"
