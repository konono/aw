# バックグラウンドエージェントのツール対応状況

調査日: 2026-06-23

## 背景

`aw team start` でバックグラウンドメンバー（reviewer 等）がメッセージを受信したとき、
非対話モード (print mode) で AI ツールを起動してコードレビューや返信を自動実行する。
このためにはツールが以下の要件を満たす必要がある:

1. 非対話 / print モードの CLI がある（プロンプトを渡して結果を出力して終了）
2. そのモードで MCP ツール (send_message) が使える
3. 権限プロンプトなしで全ツールを自動承認できる

## 調査結果

### Claude Code (`claude`) — サポート対象

```bash
claude -p "prompt" --permission-mode bypassPermissions
```

- `-p` (print mode) で非対話実行、プロンプトを処理して stdout に出力して終了
- `.mcp.json` を読み、MCP ツール (send_message 等) を使用可能
- `--permission-mode bypassPermissions` で全ツール自動承認
- ドキュメント: https://code.claude.com/docs/en/headless

### Cursor Agent (`agent`) — サポート対象

```bash
agent -p "prompt" --force --approve-mcps
```

- `-p` (print mode) で非対話実行
- `--force` で全コマンド/ツール自動承認
- `--approve-mcps` で MCP サーバー自動承認
- ドキュメント: https://cursor.com/docs/cli/headless

### Codex CLI (`codex`) — 非サポート

```bash
codex exec "prompt" --json --sandbox workspace-write
```

- `codex exec` で非対話実行は可能
- MCP ツールの自動承認メカニズムがドキュメント化されていない
- `send_message` が print mode で使えるか未検証
- 将来対応の可能性あり

### OpenCode (`opencode`) — 非サポート

```bash
opencode run "prompt" --dangerously-skip-permissions
```

- `opencode run` は存在するが、非対話パイプラインに既知の問題あり
  - GitHub issue #13851: "Unable to use opencode cli in a non-interactive pipeline"
  - GitHub issue #10411: "[FEATURE]: Add non-interactive mode to 'opencode run'"
- MCP ツール自動承認も不明確
- 代替として `opencode serve` + `--attach` のサーバーモードがあるが、
  agent-loop の起動パターンとは相性が悪い

## 実装方針

Claude Code と Cursor Agent のみ `--internal-agent-loop` を使用。
Codex と OpenCode のバックグラウンドメンバーはツール直接起動のまま (従来通り)。

print mode コマンドは `internal/launcher/tool.go` の `toolPrintCommands` で管理:

```go
var toolPrintCommands = map[string][]string{
    "claude": {"claude", "-p", "--permission-mode", "bypassPermissions"},
    "cursor": {"agent", "-p", "--force", "--approve-mcps"},
}
```

Codex / OpenCode は将来 MCP print mode 対応が確認でき次第、このマップに追加するだけで
agent-loop が有効になる。
