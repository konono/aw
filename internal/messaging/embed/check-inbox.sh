#!/bin/bash
# check-inbox.sh — Stop hook: notify agent of unread messages.
# Called at end of each assistant turn. Output is shown to the agent.
set -euo pipefail

DB="${AW_MSG_DB:-/home/agent/.aw-msg/messages.db}"
AGENT="${AW_AGENT_NAME:-}"

[ -z "$AGENT" ] && exit 0
[ ! -f "$DB" ] && exit 0

COUNT=$(sqlite3 "$DB" "SELECT COUNT(*) FROM messages WHERE to_agent='${AGENT}' AND read_at IS NULL;" 2>/dev/null || echo 0)

[ "$COUNT" = "0" ] && exit 0

echo "${COUNT} unread message(s). Use read_inbox tool to check."
