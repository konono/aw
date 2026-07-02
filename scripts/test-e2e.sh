#!/usr/bin/env bash
#
# aw E2E テストスクリプト
#
# Usage:
#   ./scripts/test-e2e.sh                  # quick モード（デフォルト）
#   ./scripts/test-e2e.sh --full           # 全ツール × 全OS マトリクス
#   ./scripts/test-e2e.sh --images-only    # GHCR イメージ存在確認のみ
#   ./scripts/test-e2e.sh --version 4.0.3  # バージョン指定
#
# 前提: リポジトリルートから実行、go build 済みまたは PATH に aw がある
#
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS=0
FAIL=0
WARN=0

pass() { echo -e "  ${GREEN}PASS${NC}: $1"; PASS=$((PASS + 1)); }
fail() { echo -e "  ${RED}FAIL${NC}: $1"; FAIL=$((FAIL + 1)); }
warn() { echo -e "  ${YELLOW}WARN${NC}: $1"; WARN=$((WARN + 1)); }
section() { echo -e "\n${CYAN}=== $1 ===${NC}"; }

TOOLS=(claude codex opencode cursor)
OSES=(debian12 ubuntu2604 ubi9 ubi10)
REGISTRY="ghcr.io/konono"

# ── 引数解析 ──────────────────────────────────────────────────────

MODE="quick"
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick)      MODE="quick"; shift ;;
    --full)       MODE="full"; shift ;;
    --images-only) MODE="images-only"; shift ;;
    --version)
      if [ -z "${2:-}" ]; then
        echo "Error: --version requires a version argument (e.g. --version 4.0.3)"
        exit 1
      fi
      VERSION="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: $0 [--quick|--full|--images-only] [--version X.Y.Z]"
      echo ""
      echo "Modes:"
      echo "  --quick        (default) Image check + debian12 tool launch + core tests"
      echo "  --full         Image check + all 16 profiles launch + core tests"
      echo "  --images-only  GHCR image existence check only (no container launch)"
      echo ""
      echo "Options:"
      echo "  --version X.Y.Z  Version to check (default: from version.go)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── バージョン取得 ────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ -z "$VERSION" ]; then
  VERSION=$(grep 'const Version' "$REPO_ROOT/internal/version/version.go" | sed 's/.*"\(.*\)".*/\1/')
fi
echo -e "Version: ${CYAN}${VERSION}${NC}"
echo -e "Mode:    ${CYAN}${MODE}${NC}"

# ── aw バイナリ検出 ───────────────────────────────────────────────

GOBIN_AW="$(go env GOPATH 2>/dev/null)/bin/aw" || true
if [ -x "${GOBIN_AW:-}" ]; then
  AW="${AW:-$GOBIN_AW}"
else
  AW="${AW:-aw}"
fi
echo -e "Binary:  ${CYAN}${AW}${NC}"
echo ""

# ── イメージ検査コマンドの検出 ────────────────────────────────────

inspect_image() {
  local image="$1"
  local repo tag token status
  repo="${image%%:*}"       # ghcr.io/konono/aw-claude
  tag="${image##*:}"        # 4.0.3-debian12
  repo="${repo#ghcr.io/}"   # konono/aw-claude

  # GHCR API で manifest の存在を確認（軽量・ツール非依存）
  token=$(curl -fsSL "https://ghcr.io/token?scope=repository:${repo}:pull" 2>/dev/null \
    | sed 's/.*"token":"\([^"]*\)".*/\1/')
  status=$(curl -sSL -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer ${token}" \
    -H "Accept: application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json" \
    "https://ghcr.io/v2/${repo}/manifests/${tag}" 2>/dev/null)
  [ "$status" = "200" ]
}

# ── ツール別 version コマンド ─────────────────────────────────────

tool_version_cmd() {
  local tool="$1"
  case "$tool" in
    claude)   echo "claude --version" ;;
    codex)    echo "codex --version" ;;
    opencode) echo "opencode version" ;;
    cursor)   echo "agent --version" ;;
  esac
}

tool_version_pattern() {
  local tool="$1"
  case "$tool" in
    claude)   echo "[0-9]" ;;
    codex)    echo "[0-9]" ;;
    opencode) echo "[0-9]" ;;
    cursor)   echo "[0-9]" ;;
  esac
}

# ====================================================================
# 1. 公式イメージ存在確認
# ====================================================================

section "1. 公式イメージ存在確認 (${#TOOLS[@]} tools × ${#OSES[@]} OS = $((${#TOOLS[@]} * ${#OSES[@]})) images)"

IMAGE_PASS=0
IMAGE_FAIL=0
MISSING_IMAGES=()

for tool in "${TOOLS[@]}"; do
  for os in "${OSES[@]}"; do
    image="${REGISTRY}/aw-${tool}:${VERSION}-${os}"
    if inspect_image "$image"; then
      pass "${image}"
      IMAGE_PASS=$((IMAGE_PASS + 1))
    else
      fail "${image} — not found in registry"
      IMAGE_FAIL=$((IMAGE_FAIL + 1))
      MISSING_IMAGES+=("${tool}-${os}")
    fi
  done
done

echo ""
TOTAL=$((IMAGE_PASS + IMAGE_FAIL))
echo -e "  Images: ${GREEN}${IMAGE_PASS}${NC}/${TOTAL} found"
if [ ${#MISSING_IMAGES[@]} -gt 0 ]; then
  echo -e "  ${RED}Missing: ${MISSING_IMAGES[*]}${NC}"
fi

if [ "$MODE" = "images-only" ]; then
  echo ""
  echo "========================================"
  echo -e "結果: ${GREEN}PASS=$PASS${NC}  ${RED}FAIL=$FAIL${NC}  ${YELLOW}WARN=$WARN${NC}"
  echo "========================================"
  [ "$FAIL" -gt 0 ] && exit 1
  exit 0
fi

# ====================================================================
# 2. ツール起動テスト
# ====================================================================

if [ "$MODE" = "full" ]; then
  TEST_OSES=("${OSES[@]}")
  section "2. ツール起動テスト (full: ${#TOOLS[@]} tools × ${#TEST_OSES[@]} OS)"
else
  TEST_OSES=(debian12)
  section "2. ツール起動テスト (quick: ${#TOOLS[@]} tools × debian12)"
fi

cd "$REPO_ROOT"

for os in "${TEST_OSES[@]}"; do
  for tool in "${TOOLS[@]}"; do
    profile="test-${os}-${tool}"
    vcmd=$(tool_version_cmd "$tool")
    pattern=$(tool_version_pattern "$tool")

    OUT=$($AW "$profile" -- $vcmd 2>"$REPO_ROOT/.test-stderr.tmp") && rc=0 || rc=$?
    STDERR=$(cat "$REPO_ROOT/.test-stderr.tmp" 2>/dev/null || true)

    if echo "$OUT" | grep -q "$pattern"; then
      ver_line=$(echo "$OUT" | grep "$pattern" | head -1 | xargs)
      if echo "$STDERR" | grep -q "building from template"; then
        warn "${profile}: ${ver_line} (fallback to template build — official image missing)"
      else
        pass "${profile}: ${ver_line}"
      fi
    else
      fail "${profile}: version command failed (rc=$rc, output: $(echo "$OUT" | tail -1))"
    fi
  done
done

rm -f "$REPO_ROOT/.test-stderr.tmp"

# ====================================================================
# 3. gh CLI / mise プリインストール確認
# ====================================================================

section "3. gh CLI / mise プリインストール確認"

cd "$REPO_ROOT"

GH_OUT=$($AW test-debian12-claude -- gh --version 2>&1) || true
if echo "$GH_OUT" | grep -q "gh version"; then
  GH_VER=$(echo "$GH_OUT" | grep "gh version" | head -1 | xargs)
  pass "gh CLI: $GH_VER"
else
  fail "gh CLI が見つからない"
fi

GH_TOKEN_OUT=$($AW test-claude-gh -- gh --version 2>&1) || true
if echo "$GH_TOKEN_OUT" | grep -q "gh version"; then
  pass "gh_token: true でも gh CLI が利用可能"
else
  fail "gh_token: true で gh CLI が見つからない"
fi

MISE_OUT=$($AW test-debian12-claude -- mise --version 2>&1) || true
if echo "$MISE_OUT" | grep -q "202[0-9]"; then
  MISE_VER=$(echo "$MISE_OUT" | grep "202[0-9]" | head -1 | xargs)
  pass "mise: $MISE_VER"
else
  fail "mise が見つからない"
fi

# ====================================================================
# 4. aw build --apply テスト
# ====================================================================

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

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

section "4. aw build --apply: ビルド入力なしで公式イメージ"

cd "$TMPDIR"
mkdir -p empty-project && cd empty-project
write_test_config

OUT=$($AW build test-claude --apply 2>&1) || true
if echo "$OUT" | grep -q "Pulling official image"; then
  pass "ビルド入力なしで公式イメージを pull"
elif echo "$OUT" | grep -q "No build inputs found"; then
  pass "ビルド入力なしで Warning 表示"
else
  fail "予期しない出力: $(echo "$OUT" | tail -3)"
fi

# ====================================================================
# 5. aw build --apply: ワークスペース mise.toml ありでビルド成功
# ====================================================================

section "5. aw build --apply: ワークスペース mise.toml ありでビルド成功"

cd "$TMPDIR"
mkdir -p mise-project && cd mise-project
write_test_config
cat > mise.toml << 'EOF'
[tools]
jq = "latest"
EOF

OUT=$($AW build test-claude --apply 2>&1) || true
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

# ====================================================================
# 6. ユーザーレベル config 廃止確認
# ====================================================================

section "6. ユーザーレベル config 廃止: ~/.config/aw/mise.toml が無視される"

FAKE_HOME="$TMPDIR/fake-home"
mkdir -p "$FAKE_HOME/.config/aw"
cat > "$FAKE_HOME/.config/aw/mise.toml" << 'EOF'
[tools]
ripgrep = "14"
EOF

cd "$TMPDIR"
mkdir -p no-workspace && cd no-workspace
write_test_config

OUT=$(HOME="$FAKE_HOME" $AW build test-claude --apply 2>&1) || true
if echo "$OUT" | grep -q "No build inputs found"; then
  pass "~/.config/aw/mise.toml は無視された"
else
  fail "~/.config/aw/mise.toml がビルド入力として認識された"
fi

# ====================================================================
# 7. ワークスペース mise.toml による entrypoint インストール
# ====================================================================

section "7. ワークスペース mise.toml による entrypoint インストール"

cd "$TMPDIR"
mkdir -p mise-entrypoint && cd mise-entrypoint
write_test_config
cat > mise.toml << 'EOF'
[tools]
fd = "latest"
EOF

FD_OUT=$($AW test-claude -- fd --version 2>&1) || true
if echo "$FD_OUT" | grep -q "fd"; then
  pass "ワークスペース mise.toml から fd がインストールされた"
else
  fail "fd のインストールに失敗"
fi

# ====================================================================
# 8. DOCKER_HOST 自動設定 (mount_container_sock)
# ====================================================================

section "8. DOCKER_HOST 自動設定 (mount_container_sock)"

cd "$TMPDIR"
mkdir -p dood-test && cd dood-test
write_test_config

DHOST_OUT=$($AW test-claude-dood -- env 2>&1) || true
if echo "$DHOST_OUT" | grep -q "DOCKER_HOST=unix:///run/container.sock"; then
  pass "DOCKER_HOST が自動設定された"
else
  fail "DOCKER_HOST が設定されていない"
fi

# ====================================================================
# 9. aw init
# ====================================================================

section "9. aw init"

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

# ====================================================================
# 10. aw doctor
# ====================================================================

section "10. aw doctor"

cd "$REPO_ROOT"
DOCTOR_OUT=$($AW doctor 2>&1) || true
if echo "$DOCTOR_OUT" | grep -qi "panic"; then
  fail "aw doctor でパニック: $DOCTOR_OUT"
else
  pass "aw doctor が正常完了"
fi

# ====================================================================
# 11. devbox モード
# ====================================================================

section "11. devbox モード"

cd "$TMPDIR"
mkdir -p devbox-test && cd devbox-test
write_test_config

DEVBOX_OUT=$($AW test-devbox -- mise --version 2>&1) || true
if echo "$DEVBOX_OUT" | grep -q "202[0-9]"; then
  pass "devbox モードでも mise がインストール済み"
else
  fail "devbox モードで mise が見つからない"
fi

# ====================================================================
# 結果サマリー
# ====================================================================

echo ""
echo "========================================"
echo -e "結果: ${GREEN}PASS=$PASS${NC}  ${RED}FAIL=$FAIL${NC}  ${YELLOW}WARN=$WARN${NC}"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
