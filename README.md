# aw

AI コーディングエージェントを、使い捨てコンテナで自律的に動かすためのワークスペースランチャー。

![demo](demo.gif)

## なぜ aw が必要か

### 人がボトルネックになっている

AI コーディングエージェントは強力ですが、ホストマシン上で動かすと「この URL を fetch していいですか？」「このファイルに書き込んでいいですか？」と逐一確認を求めてきます。人間が離席すると、その間エージェントは止まったまま待ち続けます。

人間が本当に判断すべきなのは「この設計方針で進めるか？」「この PR をマージするか？」のような重要な意思決定であり、1 ページずつの fetch 許可ではありません。

### 全権限を与えるのは危険

`--dangerously-skip-permissions` をつけてホスト上で走らせれば止まりません。しかしエージェントが悪意あるサイトの内容を読み込んだ場合、プロンプトインジェクションによって任意のコマンドを実行されるリスクがあります。ホストのファイルシステム、SSH 鍵、環境変数など、すべてが攻撃対象になり得ます。

### コンテナで隔離する

`aw` は使い捨てコンテナの中でエージェントを全権限で自律実行させます。コーディング対象のプロジェクトディレクトリだけをコンテナにマウントし、それ以外のホストリソースには触れられないようにします。コンテナ内で何が起きても、捨てれば元通りです。

## 対応 OS

| OS | Docker | Podman | 備考 |
|----|:------:|:------:|------|
| Linux (x86_64, arm64) | ✓ | ✓ | |
| macOS (Apple Silicon, Intel) | ✓ | ✓ | |
| Windows 10/11 | ✓ | ✓ | Docker Desktop または Podman Desktop (WSL2) |

## 前提条件

| ツール | 必須 | 用途 |
|--------|------|------|
| [Go](https://go.dev/dl/) 1.23+ | ✓ | `go install` によるインストール |
| [Docker](https://docs.docker.com/get-docker/) または [Podman](https://podman.io/docs/installation) | ✓ | コンテナの実行（デフォルトは Podman） |
| `git` | — | `worktree` 機能を使う場合 |

> **Windows ユーザーへ:** Docker Desktop または Podman Desktop（WSL2 バックエンド）が必要です。設定ディレクトリは `%APPDATA%\aw` です。企業プロキシ環境では [`build_env`](#企業プロキシca-証明書) と [`ca_cert`](#企業プロキシca-証明書) オプションを参照してください。

## クイックスタート

> **Podman ユーザーへ:** Linux で Podman を使う場合は、下記の[事前設定](#linux--podman-の事前設定)を先に済ませてください。

```bash
go install github.com/konono/aw@latest
aw        # デフォルトの claude プロファイルで Debian コンテナが起動
```

設定ファイルなしでそのまま使えます。カスタマイズしたくなったら:

```bash
aw init   # ~/.config/aw/config.yml にスターター設定を書き出す
```

### Linux + Podman の事前設定

Linux で Podman をルートレスモードで使う場合、**事前に以下のコマンドを実行してください**。これを行わないとコンテナのビルド時に `Permission denied` で失敗します。

```bash
sudo loginctl enable-linger $(whoami)
```

これは systemd のユーザーセッションを永続化する設定です。Podman のルートレスモードは cgroupv2 の管理に systemd ユーザーセッションを必要としますが、SSH ログインなど一部の環境ではセッションが自動的に作られないため、この設定が必要になります。詳しくは [トラブルシューティング](#podman-でイメージビルド時に-permission-denied-になる) を参照してください。

> **Note:** Podman（ルートレス）で初めて `aw` を起動すると、コンテナレイヤーの UID/GID リマッピング（ID-mapped copy）のため起動に時間がかかります。これは初回のみで、2回目以降は高速に起動します。

## アーキテクチャ

```
┌─ ホスト ──────────────────────────────────────────────────────┐
│                                                               │
│  ┌─ コンテナ（使い捨て）────────────────────────────────────┐  │
│  │                                                          │  │
│  │  AI エージェント（--dangerously-skip-permissions）        │  │
│  │  Claude Code / Codex / OpenCode                          │  │
│  │                                                          │  │
│  │  ├── プロジェクト  ← bind mount (RW)                     │  │
│  │  ├── .gitconfig    ← bind mount (RO)                     │  │
│  │  └── mise / devbox（コンテナ内で完結）                    │  │
│  │                                                          │  │
│  │  ─── オプトインで追加 ────────────────────────────        │  │
│  │  ├── SSH Agent socket   (ssh_agent_forwarding)           │  │
│  │  ├── GitHub CLI config  (mount_gh, RO)                   │  │
│  │  ├── Container socket   (mount_container_sock)           │  │
│  │  └── カスタムマウント    (mounts, デフォルト RO)          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                               │
│  ✕ ホストのファイルシステム（マウントされていない部分）        │
│  ✕ ホストの環境変数・プロセス                                 │
│  ✕ ホストの SSH 鍵（agent forwarding なら鍵自体は渡らない）   │
└───────────────────────────────────────────────────────────────┘
```

デフォルトではプロジェクトディレクトリと `.gitconfig`（読み取り専用）だけがコンテナに入ります。エージェントが何をインストールしても、どのようなコマンドを実行しても、ホストには影響しません。コンテナを捨てれば痕跡も消えます。

## 特徴

- **Claude / Codex / OpenCode 対応** — プロファイルを切り替えるだけ
- **Docker / Podman 両対応** — デフォルトは Podman、`container_runtime: docker` で Docker に切替
- **git worktree 自動生成** — 実行ごとに独立ブランチで作業。壊しても消せば終わり。複数ターミナルで並列実行も可能
- **mise / devbox 対応** — エージェントの試行錯誤を `mise.toml` や `devbox.json` に落として再現可能に
- **プレビルドイメージ対応** — `image:` で事前ビルド済みイメージを指定。エアギャップ環境に対応
- **マルチ OS テンプレート** — Debian 12 / UBI 9 / UBI 10 / Ubuntu 26.04
- **カスタムコンテナユーザー** — `container_user:` でコンテナ内ユーザーを変更可能
- **ホスト設定の自動同期** — Git / Claude 設定を引き継ぎ（GitHub CLI はオプトイン）
- **DooD (Docker outside of Docker) 対応** — `mount_container_sock: true` でコンテナ内から docker-compose 等を操作可能。詳細は [DooD ガイド](docs/dood.md)
- **自動リソースクリーンアップ** — コンテナ終了後に reaper が SSH トンネル・ソケット・コンテナを自動削除。失敗時は `aw reaper recover` で再試行可能
- **環境診断** — `aw doctor` でランタイム・認証・プロファイル・reaper の状態を一括チェック
- **チーム共有** — `.aw.yml` をコミットすれば全員が同じエージェント環境を再現できる

## 利便性とリスクのバランス

デフォルトは最小構成ですが、用途に応じてオプトインでホストリソースを開放できます。すべてデフォルト off で、ユーザーが明示的に有効にした分だけリスクが増える設計です。

| オプション | 何ができるようになるか | リスク | デフォルト |
|-----------|---------------------|--------|----------|
| `ssh_agent_forwarding` | Git push/pull（鍵はコンテナに入らない） | 低 — ソケット転送のみ | off |
| `mount_gh` | `gh pr create` 等の GitHub CLI 操作 | 中 — トークンがエージェントから読める | off |
| `mount_container_sock` | コンテナ内で docker-compose 操作（[DooD](docs/dood.md)） | 高 — コンテナランタイム全体へのアクセス | off |
| `mount_ssh` | SSH 鍵をコンテナにコピー | 高 — 秘密鍵がコンテナ内に存在する | off |
| `mounts` | 任意のディレクトリをマウント | 設定次第 | — |

### 段階的に利便性を上げる設定例

**最小構成（デフォルト）** — プロジェクトだけをマウントします。Git push や GitHub CLI 連携が不要な場合に適しています:

```bash
aw   # 設定不要。プロジェクトディレクトリだけがコンテナに入ります
```

**Git 操作あり** — エージェントに push/pull させたい場合に使います:

```yaml
ssh_agent_forwarding: true   # SSH 鍵はホストに残したまま、ソケットだけ転送
```

**フル装備** — PR 作成や docker-compose 操作もさせたい場合に使います:

```yaml
ssh_agent_forwarding: true
mount_gh: true                # gh CLI でPR作成・レビュー
mount_container_sock: true    # docker-compose up/down
```

## 仕組みの詳細

コードや仕様を読まなくても把握できるよう、ユーザーが気になるポイントをまとめます。

### 設定の同期とセッションの永続化

ホストの `~/.claude/` はコンテナに直接マウントされません。代わりに、ホスト上のステージングディレクトリ（`~/.agent-workspace/claude/`）を経由します。

**起動時の流れ:**

1. ホストの `~/.claude/` から、指定されたファイルだけをステージングディレクトリにコピーする
2. `settings.json` にはコンテナ向けのパッチを適用する（`skipDangerousModePermissionPrompt: true` の追加など）
3. ステージングディレクトリを `/home/agent/.claude/` としてコンテナにマウントする
4. Claude Code を `--permission-mode bypassPermissions` で起動する
5. プロジェクト内に `devbox.json` や `mise.toml` があれば、自動で `devbox install` / `mise install` を実行する

```
ホスト ~/.claude/                         ステージング ~/.agent-workspace/claude/
├── settings.json  ──(コピー+パッチ)──→   ├── settings.json
├── CLAUDE.md      ──(コピー)──────→      ├── CLAUDE.md
├── hooks/         ──(コピー)──────→      ├── hooks/
├── commands/      ──(コピー)──────→      ├── commands/
└── plugins/       ──(コピー)──────→      └── plugins/
                                          ├── projects/  ← 同期対象外（前回のまま残る）
                                          ├── .claude.json
                                          └── ...
                                          │
                                  コンテナに /home/agent/.claude/ としてマウント
```

ポイントは、**同期対象として指定されたファイルだけが毎回上書きされ、それ以外はそのまま残る**ことです。Claude Code がセッション履歴を `/home/agent/.claude/projects/` に書き込むと、実体はホスト上の `~/.agent-workspace/claude/projects/` に保存されます。次回起動時にこのディレクトリは同期対象外なので上書きされず、そのまま残ります。これにより、コンテナを破棄しても `/resume` で過去のセッションを再開できます。

> **Note:** 設定の同期はホスト → ステージングの一方向です。コンテナ内でエージェントが `settings.json` を変更しても、次回起動時にホストの内容で上書きされます。

### ファイルの所有者

コンテナイメージは固定の UID 1001 / GID 0（root グループ）でビルドされ、すべてのディレクトリに `chmod g=u` が適用されています。実行時には `--user <host-uid>:0` でホストユーザーの UID を渡し、entrypoint が `/etc/passwd` にエントリを動的に追加します（OpenShift スタイルの GID 0 パターン）。

この設計により、イメージをリビルドすることなく任意の UID で実行でき、コンテナ内でエージェントが作成・変更したファイルは、ホスト上でもホストユーザーの所有として見えます。

> **Note:** root（UID 0）での実行はサポートされていません。Claude Code が root での動作を拒否するためです。

### 何が残り、何が消えるか

コンテナは毎回破棄されますが（終了後に reaper が自動削除）、以下のデータは保持されます:

| データ | 永続化 | 保存先 |
|--------|--------|--------|
| プロジェクトのコード変更 | ✓ | ホスト上（bind mount） |
| 認証トークン | ✓ | `~/.agent-workspace/<tool>/` |
| セッション履歴（`/resume`） | ✓ | `~/.agent-workspace/<tool>/` |
| ツール設定・プラグイン | ✓ | `~/.agent-workspace/<tool>/` |
| git worktree | ✓ | ホスト上（手動で削除するまで残る） |
| mise / devbox でインストールしたツール | ✗ | コンテナ破棄時に消失 |
| コンテナ内で apt install したもの | ✗ | コンテナ破棄時に消失 |
| コンテナ内の一時ファイル | ✗ | コンテナ破棄時に消失 |

コンテナ内で `apt install` したパッケージを永続化したい場合は、`mise.toml` や `devbox.json` に記述するか、[カスタム Dockerfile](docs/custom-dockerfile.md) を使ってください。

### Reaper（後処理）

`aw` はコンテナを detached モード（`podman run -d`）で起動し、`podman attach --sig-proxy=false` で I/O を接続するラッパーモデルを採用しています。SIGTERM/SIGHUP はラッパーが吸収するため、外部シグナルでコンテナが死ぬことはありません。

コンテナ終了後のクリーンアップ（SSH トンネル停止、VM 内ソケット削除、コンテナ本体の削除）は、コンテナ起動前に spawn される **reaper** バックグラウンドプロセスが担当します。

```
aw プロセス（ラッパー）
  │
  ├─ os.Pipe() で pipe (read, write) を作成
  ├─ reaper 子プロセスを spawn（read 側をパイプとして渡す）
  │    └─ aw --internal-reaper として起動
  │       パイプから ReaperSpec (JSON) を読み取り、EOF を待機
  │
  ├─ pipe write 側に ReaperSpec を書き込む
  ├─ podman run -itd でコンテナを detached 起動
  │
  └─ podman attach --sig-proxy=false で I/O 接続（自動再接続ループ）
       │  SIGTERM/SIGHUP → ラッパーが吸収、attach 再接続
       │  Ctrl+C → PTY 経由で ^C バイト → コンテナ内 SIGINT
       │
       └─ コンテナ終了 → ループ脱出 → pipe write 側を close → EOF
            │
            reaper が EOF を検出してタスク実行:
              ├── SSH トンネル kill (kill_process)
              ├── VM ソケット削除 (remove_file)
              ├── on-end フック (shell)
              └── コンテナ rm
            │
            レポート書き出し → spec 削除（全タスク成功時）
```

ラッパーが先に終了した場合（ターミナルクローズ等）、コンテナは生存し、reaper は `podman wait` でコンテナの実際の終了を待ってからクリーンアップを実行します。reaper は独立プロセス（Unix: `Setpgid` でプロセスグループ分離、Windows: `CREATE_NEW_PROCESS_GROUP`）として動作します。

reaper はタスクの成功/失敗を `~/.config/aw/reaper/` に JSON レポートとして保存します。タスクが失敗した場合は spec ファイルを保持し、後から `aw reaper recover` で再試行できます。

```bash
aw reaper show              # 最新レポートの詳細表示
aw reaper list              # レポート一覧
aw reaper recover <name>    # 失敗したタスクを再実行（成功済みはスキップ）
aw reaper discard <name>    # spec を破棄して復旧を放棄
aw reaper clear             # 全レポートを削除
```

`aw doctor` の Reaper セクションでは、orphaned spec（reaper が異常終了して残った spec ファイル）や直近のセッションの異常終了を検出します。次回の `aw` 起動時にも orphaned spec は自動検出され、リソース管理タスクが自動復旧されます。

reaper の動作はプロファイルの `reaper:` セクションでカスタマイズできます:

```yaml
profiles:
  debug:
    environment: container
    launch: claude
    reaper:
      timeout: 120           # タスク実行タイムアウト（秒、デフォルト 60）
      keep-container: true    # コンテナを削除せず保持（デバッグ用）
      report-retention: 20    # 保持するレポート件数（デフォルト 10）
```

### 認証の仕組み

ホストの認証情報とコンテナの認証情報は独立しています。ホスト上で `claude` にログイン済みでも、コンテナ内では別途認証が必要です。

```bash
aw auth login claude   # コンテナ内で OAuth 認証を実行
aw auth status claude  # 認証状態を確認
```

認証トークンは `~/.agent-workspace/<tool>/` に保存されるため、一度認証すればコンテナを再作成しても維持されます。

### イメージのビルドとキャッシュ

`aw` は Dockerfile の内容・OS テンプレート・コンテナユーザー名・ツール・devbox.json・mise.toml の内容からハッシュを計算し、イメージ名に使います。これらのいずれかが変わると自動的に再ビルドされ、変わらなければキャッシュ済みイメージが再利用されます。イメージは GID 0 パターンで構築されるため、ホストの UID が異なってもリビルドは不要です。

通常のイメージにはベース OS とツール（Claude Code 等）だけが含まれ、`devbox install` や `mise install` はコンテナ起動のたびに実行されます。プロジェクトで使うランタイムが決まったら、`aw export` で環境をイメージに焼き込むことで起動を高速化できます:

```bash
aw export claude --snapshot --apply
```

このコマンドは以下を行います:

1. 一時コンテナを作成し、プロジェクトの `devbox.json` / `mise.toml` に基づいてパッケージをインストール
2. インストール済みの状態をイメージとしてコミット
3. `--apply` により、プロファイルの設定に `image:` と `skip_devbox_install: true` / `skip_mise_install: true` を書き戻す

以降の `aw` 起動では、イメージのビルドと起動時のパッケージインストールの両方がスキップされ、即座にエージェントが立ち上がります。

ランタイム構成を変更した場合は、再度 `aw export --snapshot --apply` を実行してイメージを更新してください。

### コンテナランタイムの選択

ビルトイン設定のデフォルトは **podman** です。Docker を使う場合は明示的に設定してください（自動検出は行いません）:

```yaml
container_runtime: docker
```

### コンテナ内で使えるもの

ベースイメージには以下のツールがプリインストールされています:

- `git`、`curl`、`wget`、`sudo`、`openssh-client`、`ca-certificates`、`xz-utils`
- `mise`（ランタイムマネージャー。`package_manager: devbox` の場合は代わりに `devbox` がインストールされます）

コンテナからインターネットへのアクセスに制限はありません。エージェントは `curl` や `fetch` で外部の情報を取得できます。ネットワークの隔離ではなく、ファイルシステムの隔離によってホストを保護する設計です。

### コンテナにツールを追加する

コンテナ内で `apt install` したパッケージはコンテナ破棄時に消えます。ツールを永続化するには、プロジェクトルートに `mise.toml` や `devbox.json` を置いてください。`aw` はコンテナ起動時にこれらを検出し、自動で `mise install` / `devbox install` を実行します。

```toml
# mise.toml の例
[tools]
node = "22"
python = "3.12"
```

```json
// devbox.json の例
{ "packages": ["ripgrep", "jq", "gh"] }
```

mise / devbox でインストールしたツールはコンテナ内に保存されるため、コンテナ破棄時に消えます。起動のたびに再インストールが走りますが、構成が固まったら `aw export --snapshot --apply` でイメージに焼き込むと、インストール自体をスキップして即座に起動できます。

詳細は [パッケージ管理ガイド](docs/mise.md) を参照してください。

### プロジェクト設定の信頼

`.aw.yml` にセキュリティ上重要なフィールド（`mounts`、`env`、`dockerfile`、`image`、`worktree.on-create`、`worktree.on-end`）が含まれる場合、初回使用時に信頼するかどうかの確認が表示されます。信頼情報はファイル内容のハッシュに基づくため、設定ファイルが変更されると再度確認が求められます。

### Worktree のライフサイクル

worktree はホスト上に作成されるため、コンテナ終了後も残り続けます。不要になったら手動で `git worktree remove` してください。`worktree.on-create` フックはコンテナ起動前にホスト上で実行されます。`worktree.on-end` フックは、`environment: container` の場合は reaper のタスクとして（コンテナ終了後に）、`environment: host` の場合はプロセス終了後にホスト上で実行されます。

詳細は [コンテナ同期ガイド](docs/container-sync.md) を参照してください。

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
aw export <profile> [options]   # イメージをビルドして tar 出力
aw default-dockerfile          # デフォルト Dockerfile を出力
aw default-init-script         # aw-init.sh（共通初期化スクリプト）を出力
aw doctor                      # 環境・設定の診断
aw reaper [show|list|clear|recover|discard]  # コンテナ終了後の cleanup レポート確認・復旧
aw update                      # セルフアップデート
aw --version                   # バージョン表示
```

認証の詳細は [docs/authentication.md](docs/authentication.md) を参照してください。

### 過去のディレクトリから起動する

`--recent` を使うと、過去に `aw` を起動したディレクトリ一覧から選んで、そのディレクトリでプロファイルを起動できます。

```bash
aw --recent                        # 履歴から選択して起動
aw codex --recent                  # 履歴から選択して codex で起動
aw claude --recent --query dotfiles  # 初期クエリ付きで選択
```

`-C` / `--cwd` で特定のディレクトリを直接指定することもできます。

```bash
aw -C ~/src/my-project             # 指定ディレクトリで起動
aw codex --cwd ~/src/my-project    # 指定ディレクトリで codex を起動
```

選択 UI は `aw` に内蔵されているため、`fzf` を別途インストールする必要はありません。

履歴は `${XDG_STATE_HOME:-$HOME/.local/state}/aw/dirs.json` に保存されます。

## 設定

設定は以下の順序でマージされます（後の値が優先）:

```
ビルトインデフォルト → ~/.config/aw/config.yml → .aw.yml
```

`~/.config/aw/` にはグローバル設定のほか、`mise.toml` や `devbox.json` を置くとコンテナイメージのビルド時に反映されます（`aw init` で雛形が生成されます）。プロジェクトルートに `.aw-env` ファイルを置くと、`KEY=VALUE` 形式でコンテナへの環境変数を追加できます（YAML 設定のマージとは別の仕組みで、プロファイルの `env:` より優先されます）。

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
  worktree-claude:
    worktree: {}
    launch: claude
```

全オプションの詳細は [docs/configuration.md](docs/configuration.md) を参照してください。

## 使用例

### Worktree で使い捨てブランチ作業

`worktree: {}` を付けるだけで、`aw` を実行するたびに自動で worktree が作られます。エージェントの作業はメインブランチから完全に隔離され、結果が気に入らなければ worktree ごと捨てられます:

```yaml
profiles:
  worktree-claude:
    worktree: {}              # 実行ごとに自動で worktree を作成
    environment: container
    launch: claude
```

ターミナルを複数開いて `aw worktree-claude` を実行すれば、それぞれ独立した worktree + コンテナで並列作業もできます:

```yaml
profiles:
  worktree-claude:
    worktree:
      base: origin/main
      on-create: "./scripts/setup.sh"
    environment: container
    launch: claude
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

### プレビルドイメージ（エアギャップ環境）

ネットワークのある環境でイメージを書き出し、オフライン環境に持ち込めます:

```bash
# 基本的なエクスポート
aw export claude -o my-image.tar

# --snapshot: ワークスペースのパッケージもイメージに焼き込み
aw export claude --snapshot -o my-image.tar

# --include: ホストのディレクトリをイメージにコピー（--snapshot を暗黙有効化）
aw export claude --include ./certs:/usr/local/share/ca-certificates

# --env: 環境変数をイメージに焼き込み（--snapshot を暗黙有効化）
aw export claude --env HTTP_PROXY=http://proxy.corp:8080

# USB 等で転送
docker load -i my-image.tar             # オフライン環境でロード
```

export オプションはプロファイルの `export:` セクションでも指定できます:

```yaml
profiles:
  airgap:
    environment: container
    launch: claude
    image: 'aw-container:a1b2c3d4e5f6'
    skip_devbox_install: true            # プロジェクトの devbox install をスキップ
    skip_mise_install: true              # プロジェクトの mise install をスキップ
    export:
      snapshot: true
      include:
        - src: ./certs
          dst: /usr/local/share/ca-certificates
      env:
        HTTP_PROXY: http://proxy.corp:8080
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

## ドキュメント

| ガイド | 内容 |
|--------|------|
| [設定リファレンス](docs/configuration.md) | 全オプション、バリデーションルール、マージモデル |
| [認証ガイド](docs/authentication.md) | ツール別の認証設定 |
| [コンテナ同期](docs/container-sync.md) | ホスト設定の同期、SSH、データ保存先 |
| [パッケージ管理](docs/mise.md) | mise / devbox によるコンテナ内ツール管理 |
| [カスタム Dockerfile](docs/custom-dockerfile.md) | 独自イメージの作成方法 |
| [DooD (Docker outside of Docker)](docs/dood.md) | コンテナ内から docker-compose を操作する方法 |
| [Export & Snapshot](docs/export-snapshot.md) | `aw export` のビルド・焼き込み・起動時の動作詳細 |

## 企業プロキシ・CA 証明書

企業ネットワーク環境では、コンテナイメージのビルド時にプロキシ設定や CA 証明書が必要になることがあります。

```yaml
defaults:
  build_env:
    HTTP_PROXY: "http://proxy.corp:8080"
    HTTPS_PROXY: "http://proxy.corp:8080"
    NO_PROXY: "localhost,127.0.0.1"
  ca_cert: "C:/certs/corporate-ca.pem"  # または ~/certs/corp.pem
```

- `build_env`: `docker build` / `podman build` に `--build-arg` として渡されます。Docker/Podman は `HTTP_PROXY` 等を RUN ステップで自動的に使用します
- `ca_cert`: 証明書ファイルをビルドコンテキストにコピーし、ツールインストール前に `update-ca-certificates`（Debian/Ubuntu）または `update-ca-trust`（UBI）を実行します

> **Note:** `build_env` のキーに `AW_` プレフィックスは使用できません（内部ビルド引数と衝突するため）。

## Windows でのパスの書き方

設定ファイル（`.aw.yml`、`config.yml`）ではフォワードスラッシュ `/` を使用してください。バックスラッシュ `\` は YAML のエスケープ文字と衝突します。

```yaml
# ✓ 正しい書き方
ca_cert: "C:/certs/corporate-ca.pem"
mounts:
  - source: "C:/Users/me/.config/gcloud"
    target: /home/agent/.config/gcloud

# ✗ 動作しない書き方
ca_cert: "C:\certs\corporate-ca.pem"     # YAML エスケープ問題
ca_cert: 'C:\certs\corporate-ca.pem'     # シングルクォートなら動くが非推奨
```

チルダ展開 (`~/`) も使えます:

```yaml
ca_cert: "~/certs/corp.pem"   # Linux: ~/.config に展開, Windows: %USERPROFILE% に展開
```

## Windows での既知の制限

| 制限 | 詳細 |
|------|------|
| SSH エージェント転送 (Podman) | 未サポート。Docker Desktop では `SSH_AUTH_SOCK` が設定されていれば動作します |
| `aw update` (自己更新) | Windows では実行中のバイナリを上書きできないため、失敗する場合があります。`go install github.com/konono/aw@latest` で更新してください |
| ホストモードのデフォルトシェル | `cmd.exe` が使用されます。Git Bash 環境では `SHELL` 環境変数が設定されていればそれが使われます |
| 設定ディレクトリ | Unix の `~/.config/aw` ではなく `%APPDATA%\aw` です |
| Reaper のプロセス kill | Unix ではコマンドライン確認後に kill しますが、Windows では PID のみで判断します。reaper spec 由来の信頼された PID を使用するため通常は問題ありません |
| UNC パス (`\\server\share`) | マウントソースとしてはサポートされません。ローカルパスを使用してください |

## トラブルシューティング

### Podman でイメージビルド時に `Permission denied` になる

```
error running container: from /usr/bin/crun creating container for [...]:
sd-bus call: Interactive authentication required.: Permission denied
```

**原因:** Podman のルートレスモードは、コンテナの cgroup を管理するために systemd のユーザーセッション（`systemd --user`）を使います。しかし SSH 経由のログインや、GUI を使わないサーバー環境では、systemd ユーザーセッションが自動的に開始されないことがあります。セッションが存在しない状態でコンテナを作成しようとすると、`crun` が systemd の D-Bus API を呼び出した際に認証エラーとなります。

**解決方法:**

```bash
sudo loginctl enable-linger $(whoami)
```

`loginctl enable-linger` は、対象ユーザーの systemd ユーザーセッションをログイン状態に関係なく永続化します。これにより、SSH セッションやサーバー環境でも Podman がルートレスで正常に動作するようになります。

この設定は一度実行すれば永続的に有効です。再起動後も維持されます。

### Podman（ルートレス）で初回起動が遅い

初めて `aw` を起動したとき（または新しいベースイメージに切り替えたとき）、Podman のルートレスモードはコンテナレイヤーの UID/GID リマッピングのために「ID-mapped copy」を作成します。この処理はイメージサイズに応じて数分かかることがあります。

これは初回のみの処理です。2回目以降の起動ではコピー済みのレイヤーが再利用されるため、高速に起動します。

> **Tip:** この処理中に Ctrl+C で中断するとストレージが不完全な状態になり、次回も最初からやり直しになることがあります。初回は完了まで待ってください。

## アンインストール

```bash
rm ~/go/bin/aw                    # バイナリ
rm -rf ~/.agent-workspace         # セッション・認証データ

# イメージの削除（使用しているランタイムに合わせて実行）
podman rmi aw-container           # Podman の場合（デフォルト）
docker rmi aw-container           # Docker の場合
# タグ付きイメージも含めて削除する場合は podman/docker images で確認
```

## Acknowledgments

Originally forked from [hiragram/agent-workspace](https://github.com/hiragram/agent-workspace).
