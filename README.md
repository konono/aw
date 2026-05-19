# aw

AI コーディングエージェントを、使い捨てコンテナで自律的に動かすためのワークスペースランチャー。

## なぜ aw を作ったか

Claude Code で作業中、`fetch` のたびに Yes/No を聞かれる。5回 Enter を押して、考え始めたのを見て子どもの寝かしつけに向かう。1時間後に戻ると「このURLをfetchしていいですか？」で止まっていて、タスクはほぼ進んでいない。

**問題の本質は、ホストマシン上でエージェントを動かすと権限を与えるのが怖いこと。** だから毎回確認が入り、人間が離席すると止まる。

`aw` は **どんなに汚れても壊れても困らない使い捨てコンテナ** を立ち上げ、その中でエージェントを**全権限で自律実行**させます。ファイルを書き換えても、パッケージを入れても、ホストには影響しない。だから安心して `--dangerously-skip-permissions` で走らせられる。

```bash
go install github.com/konono/aw@latest
aw    # 使い捨て Debian コンテナで Claude Code が自律起動。あとは放っておくだけ。
```

## こういう場面で威力を発揮する

**離席中にエージェントを走らせたい** — 寝かしつけ、会議、昼休み。戻ったらタスクが終わっている。権限確認で止まることがない。

**ホストを汚したくない** — エージェントが `npm install` しようが `pip install` しようが、コンテナを捨てれば元通り。プロジェクトごとに隔離された環境で安全に実験できる。

**メインブランチを汚さずに作業させたい** — `worktree: {}` を設定しておくだけで、`aw` を実行するたびに自動で git worktree が切られ、独立したブランチで作業が始まる。エージェントがどんなにコードを壊しても、worktree を消せば元通り。気に入った結果だけ PR にすればいい。

**複数タスクを並列で進めたい** — ターミナルを複数開いてそれぞれ `aw` を実行すれば、タスクごとに独立した worktree + コンテナでエージェントが同時に走る。互いに干渉しない。zellij と組み合わせればマルチペインで一覧性も確保できる。

**チームで環境を統一したい** — `.agent-workspace.yml` をコミットすれば、全員が同じエージェント環境を再現できる。「自分の環境では動く」がなくなる。

## 特徴

- **ゼロコンフィグで即起動** — インストール後すぐに `aw` でコンテナが立ち上がる
- **Claude / Codex / OpenCode 対応** — プロファイルを切り替えるだけ
- **Docker / Podman 両対応** — `container_runtime: podman` で切り替え
- **mise / devbox 対応** — エージェントの試行錯誤を `mise.toml` や `devbox.json` に落として再現可能に
- **git worktree 自動生成** — 実行するたびに独立ブランチで作業。壊しても消せば終わり
- **マルチ OS テンプレート** — Debian 12 / UBI 9 / UBI 10 / Ubuntu 26.04
- **SSH Agent 転送** — 鍵ファイルをコンテナに入れずに Git 操作
- **ホスト設定の自動同期** — Git / GitHub CLI / Claude 設定を引き継ぎ

## クイックスタート

```bash
go install github.com/konono/aw@latest
aw        # デフォルトの claude プロファイルで Debian コンテナが起動
```

設定ファイルなしでそのまま使えます。カスタマイズしたくなったら:

```bash
aw init   # ~/.config/aw/config.yml にスターター設定を書き出す
```

## 組み込みプロファイル

| プロファイル | ツール | OS |
|-------------|--------|-----|
| `claude` (デフォルト) | Claude Code | Debian 12 |
| `codex` | Codex | Debian 12 |
| `opencode` | OpenCode | Debian 12 |
| `shell` | bash | Debian 12 |
| `ubi9-shell` | bash | UBI 9 |
| `ubi10-shell` | bash | UBI 10 |
| `ubuntu2604-shell` | bash | Ubuntu 26.04 |

```bash
aw              # claude（デフォルト）
aw codex        # Codex で起動
aw shell        # コンテナ内シェル
aw profiles     # 一覧を表示
```

## 使い方

```bash
aw [profile-name]              # プロファイルを実行
aw profiles                    # 利用可能なプロファイル一覧
aw auth login claude|codex|opencode   # ツールの認証
aw auth status claude          # 認証状態の確認
aw login claude                # auth login の短縮形
aw init                        # スターター設定を書き出す
aw default-dockerfile          # デフォルト Dockerfile を出力
aw update                      # セルフアップデート
aw --version                   # バージョン表示
```

認証の詳細は [docs/authentication.md](docs/authentication.md) を参照してください。

## 設定

設定は以下の順序でマージされます:

```
ビルトインデフォルト → ~/.config/aw/config.yml → .agent-workspace.yml
```

```yaml
default: claude

container_runtime: podman
environment: container
ssh_agent_forwarding: true

profiles:
  claude:
    launch: claude
  codex:
    launch: codex
    auth:
      on_launch:
        check: warn
  worktree-zellij:
    worktree: {}
    launch: zellij
    zellij:
      tool: claude
```

全オプションの詳細は [docs/configuration.md](docs/configuration.md) を参照してください。

## 使用例

### Worktree で使い捨てブランチ作業

`worktree: {}` を付けるだけで、`aw` を実行するたびに自動で worktree が作られる。エージェントの作業はメインブランチから完全に隔離され、結果が気に入らなければ worktree ごと捨てればいい:

```yaml
profiles:
  worktree-claude:
    worktree: {}              # 実行ごとに自動で worktree を作成
    environment: container
    launch: claude
```

ターミナルを複数開いて `aw worktree-claude` を実行すれば、それぞれ独立した worktree + コンテナで並列作業もできる。zellij と組み合わせてマルチペインにすることも可能:

```yaml
profiles:
  worktree-zellij:
    worktree:
      base: origin/main
      on-create: "./scripts/setup.sh"
    environment: container
    launch: zellij
    zellij:
      tool: claude
```

### Vertex AI で Claude を使う

```yaml
profiles:
  claude-vertex:
    environment: container
    launch: claude
    env:
      CLAUDE_CODE_USE_VERTEX: "1"
      CLOUD_ML_REGION: "us-east5"
      ANTHROPIC_VERTEX_PROJECT_ID: "my-gcp-project"
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
        readonly: true
```

### カスタム Dockerfile（Playwright）

```yaml
profiles:
  playwright:
    environment: container
    launch: claude
    dockerfile: playwright-docker/Dockerfile
```

詳細は [docs/custom-dockerfile.md](docs/custom-dockerfile.md) を参照してください。

## 必要なツール

| ツール | 必要な場面 |
|--------|-----------|
| `docker` または `podman` | `environment: container` |
| `git` | `worktree` を使用する場合 |
| `zellij` | `launch: zellij` |

## アンインストール

```bash
rm ~/go/bin/aw                    # バイナリ
rm -rf ~/.agent-workspace         # データ
docker volume rm claude-code-local  # キャッシュ（Podman の場合は podman）
docker rmi claude-code-docker       # イメージ
```

## ドキュメント

| ガイド | 内容 |
|--------|------|
| [設定リファレンス](docs/configuration.md) | 全オプション、バリデーションルール、マージモデル |
| [認証ガイド](docs/authentication.md) | ツール別の認証設定 |
| [コンテナ同期](docs/container-sync.md) | ホスト設定の同期、SSH、データ保存先 |
| [カスタム Dockerfile](docs/custom-dockerfile.md) | 独自イメージの作成方法 |
| [パッケージ管理](docs/mise.md) | mise / devbox によるコンテナ内ツール管理 |

## Acknowledgments

Originally forked from [hiragram/agent-workspace](https://github.com/hiragram/agent-workspace).
