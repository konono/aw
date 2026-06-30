#!/usr/bin/env bash
#
# PR #120 ホストテストスクリプト
# Usage: ./scripts/test-pr120.sh
#
# 前提: feat/remove-user-config-and-bundle-gh-cli ブランチで go build 済み
#       または PATH に該当ブランチの aw バイナリがある
#
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0

pass() { echo -e "  ${GREEN}PASS${NC}: $1"; PASS=$((PASS + 1)); }
fail() { echo -e "  ${RED}FAIL${NC}: $1"; FAIL=$((FAIL + 1)); }
skip() { echo -e "  ${YELLOW}SKIP${NC}: $1"; SKIP=$((SKIP + 1)); }

section() { echo -e "\n${YELLOW}=== $1 ===${NC}"; }

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

GOBIN_AW="$(go env GOPATH)/bin/aw"
if [ -x "$GOBIN_AW" ]; then
  AW="${AW:-$GOBIN_AW}"
else
  AW="${AW:-aw}"
fi
PROFILE="${PROFILE:-test-claude}"
echo "Using: $AW"

# テスト用の最小プロファイル定義
write_test_config() {
  cat > .aw.yml << 'YAML'
profiles:
  test-claude:
    os: debian12
    launch: claude
  test-claude-dood:
    os: debian12
    launch: claude
    mount_container_sock: true
  test-claude-gh:
    os: debian12
    launch: claude
    gh_token: true
  test-devbox:
    os: debian12
    launch: claude
    package_manager: devbox
YAML
}

# ────────────────────────────────────────────────────────────────
section "1. aw build --apply: ビルド入力なしでエラー"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p empty-project && cd empty-project
write_test_config

OUT=$($AW build "$PROFILE" --apply 2>&1) || true
if echo "$OUT" | grep -q "Pulling official image"; then
  pass "ビルド入力なしで公式イメージを pull"
elif echo "$OUT" | grep -q "No build inputs found"; then
  pass "ビルド入力なしで Warning 表示"
else
  fail "予期しない出力: $(echo "$OUT" | tail -3)"
fi

# ────────────────────────────────────────────────────────────────
section "2. aw build --apply: ワークスペース mise.toml ありで成功"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p mise-project && cd mise-project
write_test_config
cat > mise.toml << 'EOF'
[tools]
jq = "latest"
EOF

OUT=$($AW build "$PROFILE" --apply 2>&1) || true
if echo "$OUT" | grep -q "Applied image"; then
  pass "ワークスペース mise.toml でビルド成功"
  if [ -f .aw.yml ]; then
    pass ".aw.yml にイメージ名が書き込まれた"
  else
    fail ".aw.yml が生成されなかった"
  fi
else
  fail "ワークスペース mise.toml でビルド失敗: $(echo "$OUT" | tail -3)"
fi

# ────────────────────────────────────────────────────────────────
section "3. ユーザーレベル config 廃止: ~/.config/aw/mise.toml が無視される"
# ────────────────────────────────────────────────────────────────

# バックアップ
BACKUP_MISE=""
if [ -f ~/.config/aw/mise.toml ]; then
  BACKUP_MISE=$(cat ~/.config/aw/mise.toml)
fi
BACKUP_PKGS=""
if [ -f ~/.config/aw/packages.txt ]; then
  BACKUP_PKGS=$(cat ~/.config/aw/packages.txt)
fi

# テスト用にグローバル mise.toml を作成
mkdir -p ~/.config/aw
cat > ~/.config/aw/mise.toml << 'EOF'
[tools]
ripgrep = "14"
EOF

cd "$TMPDIR"
mkdir -p no-workspace && cd no-workspace
write_test_config

# aw build --apply がグローバル mise.toml を拾わないことを確認（公式イメージ pull にフォールバック）
OUT=$($AW build "$PROFILE" --apply 2>&1) || true
if echo "$OUT" | grep -q "No build inputs found"; then
  pass "~/.config/aw/mise.toml は無視された（ビルド入力として認識しない）"
else
  fail "~/.config/aw/mise.toml がビルド入力として認識された: $(echo "$OUT" | tail -3)"
fi

# クリーンアップ
if [ -n "$BACKUP_MISE" ]; then
  echo "$BACKUP_MISE" > ~/.config/aw/mise.toml
else
  rm -f ~/.config/aw/mise.toml
fi
if [ -n "$BACKUP_PKGS" ]; then
  echo "$BACKUP_PKGS" > ~/.config/aw/packages.txt
else
  rm -f ~/.config/aw/packages.txt
fi

# ────────────────────────────────────────────────────────────────
section "4. gh CLI プリインストール"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p gh-test && cd gh-test
write_test_config

GH_OUT=$($AW "$PROFILE" -- gh --version 2>&1) || true
if echo "$GH_OUT" | grep -q "gh version"; then
  GH_VER=$(echo "$GH_OUT" | grep "gh version" | head -1)
  pass "gh CLI インストール済み: $GH_VER"
else
  fail "gh CLI が見つからない: $GH_OUT"
fi

# gh_token: true でも公式イメージパスが使われることを確認
GH_TOKEN_OUT=$($AW test-claude-gh -- gh --version 2>&1) || true
if echo "$GH_TOKEN_OUT" | grep -q "gh version"; then
  pass "gh_token: true でも gh CLI が利用可能"
else
  fail "gh_token: true で gh CLI が見つからない"
fi

# ────────────────────────────────────────────────────────────────
section "5. mise バイナリプリインストール"
# ────────────────────────────────────────────────────────────────

MISE_OUT=$($AW "$PROFILE" -- mise --version 2>&1) || true
if echo "$MISE_OUT" | grep -q "2025"; then
  MISE_VER=$(echo "$MISE_OUT" | grep "2025" | head -1)
  pass "mise インストール済み: $MISE_VER"
else
  fail "mise が見つからないまたはバージョン不一致: $MISE_OUT"
fi

# ────────────────────────────────────────────────────────────────
section "6. ワークスペース mise.toml による entrypoint インストール"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p mise-entrypoint && cd mise-entrypoint
write_test_config
cat > mise.toml << 'EOF'
[tools]
fd = "latest"
EOF

FD_OUT=$($AW "$PROFILE" -- fd --version 2>&1) || true
if echo "$FD_OUT" | grep -q "fd"; then
  pass "ワークスペース mise.toml から fd がインストールされた"
else
  fail "fd のインストールに失敗: $FD_OUT"
fi

# ────────────────────────────────────────────────────────────────
section "7. DOCKER_HOST 自動設定 (mount_container_sock)"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p dood-test && cd dood-test
write_test_config

DHOST_OUT=$($AW test-claude-dood -- bash -c 'echo $DOCKER_HOST' 2>&1) || true
if echo "$DHOST_OUT" | grep -q "unix:///run/container.sock"; then
  pass "DOCKER_HOST が自動設定された"
else
  fail "DOCKER_HOST が設定されていない: $(echo "$DHOST_OUT" | tail -3)"
fi

# ────────────────────────────────────────────────────────────────
section "8. aw init の変更"
# ────────────────────────────────────────────────────────────────

INIT_HOME="$TMPDIR/init-test-home"
mkdir -p "$INIT_HOME"

HOME="$INIT_HOME" $AW init 2>&1 || true

if [ -f "$INIT_HOME/.config/aw/config.yml" ]; then
  pass "config.yml が生成された"
else
  fail "config.yml が生成されなかった"
fi

if [ -f "$INIT_HOME/.config/aw/mise.toml" ]; then
  fail "mise.toml テンプレートが生成された（廃止済みのはず）"
else
  pass "mise.toml テンプレートは生成されない"
fi

if [ -f "$INIT_HOME/.config/aw/devbox.json" ]; then
  fail "devbox.json テンプレートが生成された（廃止済みのはず）"
else
  pass "devbox.json テンプレートは生成されない"
fi

# ────────────────────────────────────────────────────────────────
section "9. aw doctor"
# ────────────────────────────────────────────────────────────────

cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo "$OLDPWD")"
DOCTOR_OUT=$($AW doctor 2>&1) || true
if echo "$DOCTOR_OUT" | grep -qi "panic"; then
  fail "aw doctor でエラー発生: $DOCTOR_OUT"
else
  pass "aw doctor が正常完了"
fi

# ────────────────────────────────────────────────────────────────
section "10. devbox モード（オプション）"
# ────────────────────────────────────────────────────────────────

cd "$TMPDIR"
mkdir -p devbox-test && cd devbox-test
write_test_config

DEVBOX_OUT=$($AW test-devbox -- mise --version 2>&1) || true
if echo "$DEVBOX_OUT" | grep -q "2025"; then
  pass "devbox モードでも mise がインストール済み"
else
  fail "devbox モードで mise が見つからない: $(echo "$DEVBOX_OUT" | tail -3)"
fi

# ────────────────────────────────────────────────────────────────
echo ""
echo "========================================"
echo -e "結果: ${GREEN}PASS=$PASS${NC}  ${RED}FAIL=$FAIL${NC}  ${YELLOW}SKIP=$SKIP${NC}"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
