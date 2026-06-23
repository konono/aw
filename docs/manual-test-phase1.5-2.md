# 手動テスト手順: Phase 1.5 + 2 全機能

## 前提条件

- Podman (or Docker) が動作していること
- このブランチのソースコードがあること

---

## Step 0: ビルド

```bash
cd ~/ghq/github.com/konono/aw/.claude/worktrees/agent-messaging

# ホスト用バイナリ
go build -o /tmp/aw-test ./

# コンテナ用 Linux バイナリ
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/aw-linux ./
```

---

## Step 1: テスト用プロジェクトの作成

```bash
# テスト用ディレクトリ
rm -rf /tmp/aw-manual-test
mkdir -p /tmp/aw-manual-test
cd /tmp/aw-manual-test

# git リポジトリ初期化（worktree 機能のテストに必要）
git init
git checkout -b main
echo "# Manual Test Project" > README.md
echo "package main" > main.go
git add .
git commit -m "initial commit"
```

### .aw.yml を作成

新機能 `delivery` を含む設定。

```bash
cat > /tmp/aw-manual-test/.aw.yml << 'YAML'
environment: container
os: debian12
container_runtime: podman

profiles:
  claude-dev:
    launch: claude
    delivery: turn

  claude-dev-monitor:
    launch: claude
    delivery: monitor

  cursor-review:
    launch: cursor
    # delivery 未指定 → cursor デフォルト = off

  claude-review:
    launch: claude
    delivery: turn

teams:
  # developer + reviewer のペア
  review-team:
    members:
      - profile: claude-dev
        role: developer
        foreground: true
      - profile: claude-review
        role: reviewer

  # monitor モードのチーム
  monitor-team:
    members:
      - profile: claude-dev-monitor
        role: developer
        foreground: true
      - profile: claude-review
        role: reviewer
YAML
```

---

## Step 2: プロファイルとバリデーション確認

### 2-1. profiles 一覧

```bash
cd /tmp/aw-manual-test
/tmp/aw-test profiles
```

**期待:** `claude-dev`, `claude-dev-monitor`, `cursor-review`, `claude-review` が表示される。

### 2-2. 無効な delivery でバリデーションエラー

```bash
cat > /tmp/aw-manual-test/.aw-bad.yml << 'YAML'
profiles:
  bad:
    environment: container
    launch: claude
    delivery: push
YAML

# 一時的に .aw.yml を差し替え
mv /tmp/aw-manual-test/.aw.yml /tmp/aw-manual-test/.aw.yml.bak
mv /tmp/aw-manual-test/.aw-bad.yml /tmp/aw-manual-test/.aw.yml
cd /tmp/aw-manual-test
/tmp/aw-test profiles 2>&1
# 元に戻す
mv /tmp/aw-manual-test/.aw.yml.bak /tmp/aw-manual-test/.aw.yml
```

**期待:** `unknown delivery: "push"` を含むエラー。

---

## Step 3: team start — --task フラグ付き起動

```bash
cd /tmp/aw-manual-test
AW_LINUX_BIN=/tmp/aw-linux /tmp/aw-test team start --task "FizzBuzz を実装してテストを書いてください" review-team
```

**期待:** バックグラウンドの `reviewer-1` が起動し、フォアグラウンドの `developer-1` (claude) が起動する。

> Claude がインストールされていない場合、コンテナ起動後に `claude` コマンドが見つからずエラーになる可能性がある。
> その場合でも、**起動前に注入されたファイルは検証可能**（Step 4 で確認）。

### エラーで停止した場合

Ctrl+C で停止後:

```bash
/tmp/aw-test team status
```

で状態を確認。コンテナが残っていれば:

```bash
/tmp/aw-test team stop review-team
```

---

## Step 4: 注入ファイルの検証

team start が（成功・失敗問わず）実行された後、ステージングディレクトリに注入されたファイルを確認する。

### 4-1. ステージングディレクトリの場所

```bash
# claude 用ステージング
ls ~/.agent-workspace/claude/

# .mcp.json はプロジェクトディレクトリに注入される
cat /tmp/aw-manual-test/.mcp.json 2>/dev/null | python3 -m json.tool
```

**期待:** `.mcp.json` に `aw-msg` サーバーが登録されている。`--internal-mcp-msg`, `--db`, `--agent`, `--team` 引数を持つ。

### 4-2. settings.json (hook 確認)

```bash
cat ~/.agent-workspace/claude/settings.json 2>/dev/null | python3 -m json.tool
```

**期待 (delivery: turn の場合):**
- `hooks.Stop` に `--internal-check-inbox` コマンドが含まれる
- `hooks.PostToolUse` に `--internal-on-commit` コマンドが含まれる (developer ロール)
- `hooks.SessionStart` は存在しない

### 4-3. CLAUDE.md (ロールコンテキスト)

```bash
cat ~/.agent-workspace/claude/CLAUDE.md
```

**期待:**
- `## Agent Messaging — Team:` ヘッダがある
- `You are: developer-1 (role: developer)` がある
- `### Task` セクションに `FizzBuzz を実装してテストを書いてください` がある
- `### Team Members` に `developer-1` と `reviewer-1` がリストされる
- MCP ツール一覧 (`send_message`, `read_inbox` 等) がある

### 4-4. worktree が作成されていること

```bash
ls /tmp/aw-manual-test/worktrees/
git -C /tmp/aw-manual-test worktree list
git -C /tmp/aw-manual-test branch
```

**期待:**
- `worktrees/aw-review-team-developer-1` と `worktrees/aw-review-team-reviewer-1` ディレクトリが存在
- `aw/review-team/developer-1` と `aw/review-team/reviewer-1` ブランチが存在

### 4-5. チーム状態ファイル

```bash
cat ~/.config/aw/teams/review-team.state.json | python3 -m json.tool
```

**期待:**
- `members` に `developer-1` と `reviewer-1` がある
- 各メンバーに `worktree_path` と `branch_name` フィールドがある
- `team_scope` が `review-team-{hash}-{session}` 形式

---

## Step 5: メッセージング基盤テスト（コンテナ外）

コンテナ内の MCP と同等の動作をホストから直接テストする。

### 5-1. チームスコープを取得

```bash
export TEAM_SCOPE=$(python3 -c "import json; d=json.load(open('$HOME/.config/aw/teams/review-team.state.json')); print(d['team_scope'])")
echo "Team scope: $TEAM_SCOPE"
```

team start していない場合は手動で設定:

```bash
export TEAM_SCOPE="manual-test-scope"
```

### 5-2. メッセージ送信

```bash
/tmp/aw-test msg send --team $TEAM_SCOPE developer-1 reviewer-1 "PR レビューお願いします。FizzBuzz の実装が完了しました。"
```

**期待:** `Message #1 sent to reviewer-1 at HH:MM:SS`

### 5-3. reviewer のインボックス確認

```bash
/tmp/aw-test msg inbox --team $TEAM_SCOPE reviewer-1
```

**期待:**
```
Unread messages for reviewer-1:
  #1 [HH:MM:SS] developer-1: PR レビューお願いします。FizzBuzz の実装が完了しました。
```

### 5-4. reviewer が返信

```bash
/tmp/aw-test msg send --team $TEAM_SCOPE reviewer-1 developer-1 "コードレビュー完了。LGTM！マージしてください。"
```

### 5-5. 履歴確認

```bash
/tmp/aw-test msg history --team $TEAM_SCOPE
```

**期待:** 2 件のメッセージが時系列で表示される。

---

## Step 6: check-inbox JSON 出力テスト

### 6-1. developer に未読があるとき

```bash
# cooldown マーカーをクリア
rm -f ~/.config/aw/messaging/.lastcheck-developer-1

AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=developer-1 \
  AW_TEAM_NAME=$TEAM_SCOPE \
  AW_MSG_CHECK_INTERVAL=0 \
  /tmp/aw-test --internal-check-inbox
```

**期待:**
```json
{"decision":"block","reason":"1 unread message(s). Use read_inbox tool to check."}
```

### 6-2. cooldown 内の再実行

```bash
# interval=9999 で即座に再実行
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=developer-1 \
  AW_TEAM_NAME=$TEAM_SCOPE \
  AW_MSG_CHECK_INTERVAL=9999 \
  /tmp/aw-test --internal-check-inbox
```

**期待:** 出力なし（cooldown 中）。

### 6-3. 未読がない agent

```bash
rm -f ~/.config/aw/messaging/.lastcheck-reviewer-1
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=reviewer-1 \
  AW_TEAM_NAME=$TEAM_SCOPE \
  AW_MSG_CHECK_INTERVAL=0 \
  /tmp/aw-test --internal-check-inbox
```

**期待:** 出力なし（reviewer の未読は 0、Step 5-3 で表示済み ≠ 既読だが send 側がこのスコープなら未読のまま）。

> 注: `msg inbox` は未読を「表示するだけ」で既読にはしない。`mark_read` が必要。

---

## Step 7: MCP サーバー テスト

### 7-1. tools/list

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent developer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool
```

**期待:** 5 ツール (`send_message`, `read_inbox`, `read_message`, `mark_read`, `list_agents`) が返る。

### 7-2. MCP 経由で send_message

```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_message","arguments":{"to":"reviewer-1","body":"MCP 経由テスト"}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent developer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool
```

**期待:** `id` と `created_at` を含む JSON レスポンス。

### 7-3. MCP 経由で read_inbox → read_message → mark_read

```bash
# read_inbox (reviewer として)
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_inbox","arguments":{}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent reviewer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool
```

レスポンスから `id` を確認し、以下の `ID` を置き換える:

```bash
# read_message
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_message","arguments":{"id":ID}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent reviewer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool

# mark_read
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"mark_read","arguments":{"id":ID}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent reviewer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool

# 再度 read_inbox → 空であること
echo '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"read_inbox","arguments":{}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent reviewer-1 \
    --team $TEAM_SCOPE | python3 -m json.tool
```

**期待:** mark_read 後の read_inbox で空配列 `[]`。

### 7-4. list_agents

```bash
echo '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_agents","arguments":{}}}' | \
  /tmp/aw-test --internal-mcp-msg \
    --db ~/.config/aw/messaging/messages.db \
    --agent anyone \
    --team $TEAM_SCOPE | python3 -m json.tool
```

**期待:** `developer-1` と `reviewer-1` が表示される。

---

## Step 8: on-commit トリガーテスト

### 8-1. worktree 内でコミット

```bash
cd /tmp/aw-manual-test/worktrees/aw-review-team-developer-1
echo "func FizzBuzz() {}" > fizzbuzz.go
git add fizzbuzz.go
git commit -m "feat: implement FizzBuzz"
```

worktree が存在しない場合はメインリポジトリで:

```bash
cd /tmp/aw-manual-test
echo "func FizzBuzz() {}" > fizzbuzz.go
git add fizzbuzz.go
git commit -m "feat: implement FizzBuzz"
```

### 8-2. on-commit を手動実行

```bash
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=developer-1 \
  AW_TEAM_NAME=$TEAM_SCOPE \
  AW_REVIEWERS=reviewer-1 \
  /tmp/aw-test --internal-on-commit
```

**期待:** exit 0、出力なし。

### 8-3. reviewer にコミット通知が届いていること

```bash
/tmp/aw-test msg inbox --team $TEAM_SCOPE reviewer-1
```

**期待:** `developer-1` から `New commit by developer-1: XXXXXXXX` + `feat: implement FizzBuzz` を含むメッセージ。

### 8-4. CLI 引数でも動作すること

```bash
echo "// another fix" >> /tmp/aw-manual-test/fizzbuzz.go 2>/dev/null || echo "// another fix" >> fizzbuzz.go
git add -A
git commit -m "fix: edge case"

/tmp/aw-test --internal-on-commit \
  --db ~/.config/aw/messaging/messages.db \
  --agent developer-1 \
  --team $TEAM_SCOPE
```

**期待:** exit 0。`AW_REVIEWERS` が設定されていれば通知される。

---

## Step 9: --internal-watch テスト（リアルタイム受信）

**ターミナル 1:**

```bash
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=reviewer-1 \
  AW_TEAM_NAME=$TEAM_SCOPE \
  /tmp/aw-test --internal-watch
```

**ターミナル 2:**

```bash
export TEAM_SCOPE="<ターミナル1と同じ値>"
/tmp/aw-test msg send --team $TEAM_SCOPE developer-1 reviewer-1 "watch テスト: リアルタイムで見えますか？"
```

**期待:** ターミナル 1 に 2 秒以内に `[msg from developer-1] watch テスト: リアルタイムで見えますか？` が表示される。

Ctrl+C で停止。

---

## Step 10: monitor モード — settings.json 構造確認

### 10-1. monitor チームで起動

```bash
cd /tmp/aw-manual-test
# 前回の worktree が残っていたら削除
git worktree remove worktrees/aw-monitor-team-developer-1 2>/dev/null
git worktree remove worktrees/aw-monitor-team-reviewer-1 2>/dev/null
git branch -D aw/monitor-team/developer-1 2>/dev/null
git branch -D aw/monitor-team/reviewer-1 2>/dev/null

AW_LINUX_BIN=/tmp/aw-linux /tmp/aw-test team start --task "monitor モードのテスト" monitor-team
```

### 10-2. settings.json が monitor 構造であること

```bash
cat ~/.agent-workspace/claude/settings.json | python3 -m json.tool
```

**期待:**
- `hooks.SessionStart` に `--internal-watch &` コマンドが含まれる
- `hooks.Stop` が存在しない
- `hooks.PostToolUse` に `--internal-on-commit` が含まれる (developer)

### 10-3. CLAUDE.md にタスクが含まれること

```bash
grep -A2 "### Task" ~/.agent-workspace/claude/CLAUDE.md
```

**期待:** `monitor モードのテスト` が表示される。

### 10-4. 停止

```bash
/tmp/aw-test team stop monitor-team
```

**期待:** `Worktrees preserved:` にメンバーごとの worktree パスとブランチが表示される。

---

## Step 11: --resume テスト

### 11-1. 状態確認

```bash
/tmp/aw-test team status review-team
```

**期待:** メンバー一覧が表示される。

### 11-2. resume で再起動

```bash
cd /tmp/aw-manual-test
AW_LINUX_BIN=/tmp/aw-linux /tmp/aw-test team start --resume review-team
```

**期待:**
- `Resuming session XXXXXXXXXXXX` と表示される
- 既存の worktree がそのまま再利用される (新規作成エラーにならない)
- 前回と同じ `team_scope` が使われる

---

## Step 12: team stop と worktree 保持確認

```bash
# Ctrl+C でフォアグラウンドを止めた後、または別ターミナルで:
/tmp/aw-test team stop review-team
```

**期待:**
```
[team:review-team] Stopping 2 members...
  Stopping developer-1 (aw-review-team-developer-1-...)...
  Stopping reviewer-1 (aw-review-team-reviewer-1-...)...

Worktrees preserved:
  developer-1: /tmp/aw-manual-test/worktrees/aw-review-team-developer-1 (branch: aw/review-team/developer-1)
  reviewer-1: /tmp/aw-manual-test/worktrees/aw-review-team-reviewer-1 (branch: aw/review-team/reviewer-1)
[team:review-team] Stopped. Use 'aw team start --resume review-team' to resume.
```

### 12-1. worktree がディスクに残っていること

```bash
ls /tmp/aw-manual-test/worktrees/
git -C /tmp/aw-manual-test worktree list
```

---

## Step 13: エンドツーエンド シナリオ

全機能を組み合わせた統合テスト。

```bash
# クリーンな DB で開始
rm -f ~/.config/aw/messaging/messages.db
export E2E_SCOPE="e2e-test-scope"

# 1. developer がコミット
cd /tmp/aw-manual-test
echo "func Hello() { fmt.Println(\"hello\") }" > hello.go
git add hello.go
git commit -m "feat: add Hello function"

# 2. on-commit が reviewer に通知
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=developer-1 \
  AW_TEAM_NAME=$E2E_SCOPE \
  AW_REVIEWERS=reviewer-1 \
  /tmp/aw-test --internal-on-commit

# 3. reviewer の check-inbox → JSON block
rm -f ~/.config/aw/messaging/.lastcheck-reviewer-1
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=reviewer-1 \
  AW_TEAM_NAME=$E2E_SCOPE \
  AW_MSG_CHECK_INTERVAL=0 \
  /tmp/aw-test --internal-check-inbox

# 4. reviewer が inbox を確認
/tmp/aw-test msg inbox --team $E2E_SCOPE reviewer-1

# 5. reviewer が返信
/tmp/aw-test msg send --team $E2E_SCOPE reviewer-1 developer-1 "LGTM! マージしてください。"

# 6. developer の check-inbox → JSON block
rm -f ~/.config/aw/messaging/.lastcheck-developer-1
AW_MSG_DB=~/.config/aw/messaging/messages.db \
  AW_AGENT_NAME=developer-1 \
  AW_TEAM_NAME=$E2E_SCOPE \
  AW_MSG_CHECK_INTERVAL=0 \
  /tmp/aw-test --internal-check-inbox

# 7. 全履歴
/tmp/aw-test msg history --team $E2E_SCOPE
```

**期待される出力の流れ:**
1. (出力なし)
2. (出力なし)
3. `{"decision":"block","reason":"1 unread message(s). Use read_inbox tool to check."}`
4. `Unread messages for reviewer-1:` + コミット通知メッセージ
5. `Message #2 sent to developer-1 at HH:MM:SS`
6. `{"decision":"block","reason":"1 unread message(s). Use read_inbox tool to check."}`
7. 2 件のメッセージ（developer→reviewer、reviewer→developer）が時系列で表示

---

## クリーンアップ

```bash
# テストプロジェクト
rm -rf /tmp/aw-manual-test

# テストバイナリ
rm -f /tmp/aw-test /tmp/aw-linux

# チーム状態
rm -f ~/.config/aw/teams/review-team.state.json
rm -f ~/.config/aw/teams/monitor-team.state.json

# メッセージ DB（他で使っていなければ）
# rm -f ~/.config/aw/messaging/messages.db

# ステージングディレクトリ（他で使っていなければ）
# rm -rf ~/.agent-workspace/claude/
```
