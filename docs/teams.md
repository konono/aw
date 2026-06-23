# チーム & エージェント間メッセージング

複数の AI エージェントをチームとして起動し、メッセージングで協調させる機能です。

## クイックスタート

### 1. `.aw.yml` にチームを定義

```yaml
profiles:
  claude-dev:
    launch: claude

  claude-review:
    launch: claude

teams:
  review-team:
    members:
      - profile: claude-dev
        role: developer
        foreground: true
      - profile: claude-review
        role: reviewer
```

### 2. チームを起動

```bash
aw team start --task "README.md に Hello World と書いてください" review-team
```

- `developer-1` がフォアグラウンドで対話モードの Claude を起動
- `reviewer-1` がバックグラウンドでメッセージ待機（agent-loop）

### 3. 自動レビューフロー

1. developer が README.md を編集してコミット
2. developer が `send_message` MCP ツールで reviewer にレビュー依頼
3. reviewer の agent-loop がメッセージを検知し、`claude -p` でコードレビューを自動実行
4. reviewer が `send_message` で developer にフィードバックを返信
5. developer のターン終了時に Stop hook で未読通知 → inbox を確認

## アーキテクチャ

```
aw team start review-team
  ├─ reviewer-1 (バックグラウンドコンテナ)
  │   ├─ .mcp.json → aw --internal-mcp-msg (MCP サーバー)
  │   ├─ CLAUDE.md にロール情報 + タスク注入
  │   ├─ aw --internal-agent-loop (メッセージ駆動ループ)
  │   │     → メッセージ受信 → claude -p でレビュー → send_message で返信
  │   └─ 環境変数: AW_AGENT_NAME, AW_MSG_DB, AW_TEAM_NAME, AW_TOOL
  │
  └─ developer-1 (フォアグラウンドコンテナ)
      ├─ .mcp.json → aw --internal-mcp-msg (MCP サーバー)
      ├─ settings.json Stop hook → aw --internal-check-inbox
      ├─ CLAUDE.md にロール情報 + タスク注入
      └─ 環境変数: AW_AGENT_NAME, AW_MSG_DB, AW_TEAM_NAME

共有: ~/.config/aw/messaging/messages.db (SQLite WAL)
      → コンテナ内 /home/agent/.aw-msg/messages.db
```

### メッセージング

- SQLite WAL モードで複数コンテナ間を安全に共有
- MCP サーバー (`--internal-mcp-msg`) が 5 つのツールを提供:
  - `send_message(to, body)` — メッセージ送信
  - `read_inbox()` — 未読メッセージ一覧
  - `read_message(id)` — メッセージ全文取得
  - `mark_read(id)` — 既読マーク
  - `list_agents()` — チームメンバー一覧
- リクエストごとに DB を Open/Close し、WAL スナップショット分離を回避

### ブランチ分離

git リポジトリ内で起動すると、メンバーごとに worktree が自動作成されます:

```
project/
├─ worktrees/
│  ├─ aw-review-team-developer-1/   ← developer の作業領域
│  └─ aw-review-team-reviewer-1/    ← reviewer の作業領域
├─ README.md
└─ .aw.yml
```

- ブランチ名: `aw/{team}/{agent}` (例: `aw/review-team/developer-1`)
- `team stop` 後も worktree は保持される（`--resume` で再利用可能）

### チームスコープ

```
{チーム名}-{SHA256(cwd)[:12]}-{セッションID[:12]}
例: review-team-a1b2c3d4e5f6-550e8400e29b
```

- プロジェクトハッシュで異なるプロジェクト間を分離
- セッション ID で再起動ごとに分離（`--resume` 時は再利用）

## ロール

| ロール | 説明 | テンプレート |
|--------|------|-------------|
| `developer` | コード実装とテスト。レビュー依頼を send_message で送信 | developer.md.tmpl |
| `reviewer` | コードレビュー。フィードバックを send_message で返信 | reviewer.md.tmpl |
| `lead` | タスク分解と割り当て。partner にサブタスクを振る | lead.md.tmpl |
| `partner` | lead からのサブタスク実装。完了報告を send_message で送信 | partner.md.tmpl |

ロール情報は CLAUDE.md（Claude/Cursor）または AGENTS.md（Codex/OpenCode）に注入されます。

## Delivery モード

メッセージ通知の受信方法を制御します。プロファイルの `delivery` フィールドで指定:

```yaml
profiles:
  claude-dev:
    launch: claude
    delivery: turn      # デフォルト (claude/codex)

  claude-monitor:
    launch: claude
    delivery: monitor   # SessionStart で常時監視

  cursor-review:
    launch: cursor
    # delivery 未指定 → cursor デフォルト = off
```

| モード | 動作 | 用途 |
|--------|------|------|
| `turn` | ターン終了時に Stop hook で未読チェック | フォアグラウンドエージェント |
| `monitor` | SessionStart hook でバックグラウンドウォッチャーを起動 | リアルタイム通知が必要な場合 |
| `off` | MCP pull のみ（自動通知なし） | hook 非対応ツール |

**注:** バックグラウンドメンバーは delivery モードに関係なく agent-loop で動作します。delivery モードはフォアグラウンドメンバーにのみ影響します。

## Agent Loop（バックグラウンドエージェント）

バックグラウンドメンバーは AI ツールを直接起動する代わりに `--internal-agent-loop` を実行します:

1. 起動時に未読メッセージがあれば、サマリーを 1 回のプロンプトで処理し既読にする
2. 以降は 2 秒間隔でポーリングし、新規メッセージを個別に処理
3. 各メッセージに対して `claude -p` / `agent -p`（print mode）を実行
4. AI ツールが CLAUDE.md を読み、コードレビューし、`send_message` で返信
5. 処理成功時のみ既読にする（失敗時は次のポーリングでリトライ）

### サポート対象ツール

| ツール | print mode コマンド | サポート |
|--------|-------------------|---------|
| Claude Code | `claude -p --permission-mode bypassPermissions` | ✅ |
| Cursor Agent | `agent -p --force --approve-mcps` | ✅ |
| Codex CLI | — | ❌（MCP 自動承認の仕様が未確認） |
| OpenCode | — | ❌（非対話パイプラインに既知の制限あり） |

詳細は [docs/background-agent-tool-support.md](background-agent-tool-support.md) を参照。

## コマンドリファレンス

### チーム管理

```bash
aw team start [--resume] [--task <desc>] <team-name>
aw team stop <team-name>
aw team status [team-name]
```

- `--task` — タスクをロールテンプレートに注入
- `--resume` — 前回のセッション（worktree + team scope）を再利用

### メッセージング（ホストから直接操作）

```bash
aw msg send --team <scope> <from> <to> <body>
aw msg inbox --team <scope> <agent>
aw msg history --team <scope> [--agent <name>] [--limit N]
aw msg watch --team <scope>
```

team scope は `aw team status` または CLAUDE.md 内の `Team:` ヘッダーから確認できます。

## 設定例

### developer + reviewer ペア

```yaml
profiles:
  claude-dev:
    launch: claude

  claude-review:
    launch: claude

teams:
  review-team:
    members:
      - profile: claude-dev
        role: developer
        foreground: true
      - profile: claude-review
        role: reviewer
```

### lead + partner チーム

```yaml
profiles:
  claude-lead:
    launch: claude

  claude-partner:
    launch: claude

teams:
  dev-team:
    members:
      - profile: claude-lead
        role: lead
        foreground: true
      - profile: claude-partner
        role: partner
      - profile: claude-partner
        role: partner
```

### Claude developer + Cursor reviewer

```yaml
profiles:
  claude-dev:
    launch: claude

  cursor-review:
    launch: cursor

teams:
  mixed-team:
    members:
      - profile: claude-dev
        role: developer
        foreground: true
      - profile: cursor-review
        role: reviewer
```

## 制限事項

- フォアグラウンドメンバーは各チームに 1 つだけ
- バックグラウンドの agent-loop は Claude Code と Cursor Agent のみ対応
- `team stop` 後の worktree は手動で削除が必要（`git worktree remove`）
- メッセージ DB はホスト上の `~/.config/aw/messaging/messages.db` に保存される
- フォアグラウンドエージェントの未読通知は Stop hook 依存（ターン終了時のみ）
