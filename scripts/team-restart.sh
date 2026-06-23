#!/bin/bash
# Example helper for development testing. Adjust paths for your environment.
set -e
TEAM_NAME="${1:-review-team}"
TASK="${2:-README.md に Hello World と書いてください}"
SRC_DIR="${AW_SRC_DIR:-$(cd "$(dirname "$0")/.." && pwd)}"
PROJECT_DIR="${AW_PROJECT_DIR:-/tmp/aw-manual-test}"

echo "=== リビルド ==="
cd "$SRC_DIR"
go build -o /tmp/aw-test ./
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o "$HOME/aw-linux" ./

echo "=== 既存チーム停止 ==="
/tmp/aw-test team stop "$TEAM_NAME" 2>/dev/null || true
podman ps --filter "name=aw-${TEAM_NAME}" --format "{{.Names}}" | xargs -r podman stop 2>/dev/null || true

echo "=== worktree 掃除 ==="
cd "$PROJECT_DIR"
git worktree list --porcelain | grep "^worktree " | grep "aw-${TEAM_NAME}" | cut -d' ' -f2 | while read wt; do
    git worktree remove --force "$wt" 2>/dev/null || true
done
git worktree prune
git branch --list "aw/${TEAM_NAME}/*" | xargs -r git branch -D 2>/dev/null || true

echo "=== 起動 ==="
AW_LINUX_BIN="$HOME/aw-linux" /tmp/aw-test team start --task "$TASK" "$TEAM_NAME"
