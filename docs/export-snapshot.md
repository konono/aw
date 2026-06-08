# Export & Snapshot の内部動作

`aw export` がイメージをビルドし、`--snapshot` でツールを焼き込み、エクスポートしたイメージが起動時にどう動くかを説明します。

## 全体の流れ

```
aw export dev --snapshot -o image.tar
```

1. **イメージビルド** — Dockerfile でベースイメージを作成（Nix, devbox, mise の基盤のみ）
2. **snapshot** — 一時コンテナを起動し、ワークスペースのパッケージをインストールして `docker commit`
3. **tar 出力** — `docker save` でイメージを tar に書き出す

## イメージビルド（Dockerfile）

`internal/image/embed/Dockerfile.debian12` がベースイメージを定義しています。

```
debian:bookworm-slim
  ├── apt: git, curl, ca-certificates, wget, openssh-client, sudo, xz-utils
  ├── user: agent (sudo NOPASSWD)
  ├── Nix: single-user mode (/nix owned by agent)
  ├── devbox: /usr/local/bin/devbox
  ├── ENV:
  │   ├── HOME=/home/agent
  │   ├── BASH_ENV=/home/agent/.aw_env.sh    ← 非インタラクティブシェルで自動読み込み
  │   └── PATH に .nix-profile/bin, .local/bin を追加
  ├── ~/.config/aw/devbox.json のパッケージをインストール（ビルド時）
  ├── ~/.config/aw/mise.toml のツールをインストール（ビルド時）
  ├── ENTRYPOINT ["/entrypoint.sh"]
  └── WORKDIR /workspace
```

この時点では `.aw_env.sh` ファイルは存在しません。`BASH_ENV` は設定されているが、ファイルが作られるのは entrypoint.sh 実行時（通常起動）または snapshot スクリプト実行時（export 時）です。

## snapshot スクリプトの動作

### マウント構成

```go
// runSnapshot() が設定するマウント:
/workspace     ← ec.OrigWorkDir（プロジェクトディレクトリ）を ro マウント
/tmp/aw-include-0  ← --include の src を ro マウント
/tmp/aw-include-1  ← 同上（複数指定可）
```

全マウントが **read-only** です。export はビルド操作であり、ホスト側のファイルを変更してはいけないためです。

### なぜワークスペースをコピーするか

`devbox install` と `mise install` はカレントディレクトリに中間生成物を書き込みます:

- devbox: `.devbox/gen/shell.nix`, `.devbox/virtenv/` など
- mise: `.mise/` ディレクトリ

`/workspace` は ro マウントなので、これらの書き込みが失敗します。そのため snapshot スクリプトは以下の手順を踏みます:

```bash
WORK="/tmp/aw-snapshot-work"
cp -a "$WORKSPACE/." "$WORK/"    # ro マウントから書き込み可能な場所にコピー
cd "$WORK" && devbox install     # こちらで実行
cd "$WORK" && mise install       # こちらで実行
rm -rf "$WORK"                   # commit 前にクリーンアップ
```

### ツールの焼き込み先

| ツール | インストール先 | 参照方法 |
|--------|-------------|----------|
| devbox (Nix) パッケージ | `/nix/store/`, `/home/agent/.local/share/devbox/` | `devbox global shellenv` が PATH を設定 |
| mise ツール（jq, go 等） | `/home/agent/.local/share/mise/installs/<tool>/<ver>/` | `/home/agent/.local/share/mise/shims/` 経由 |

mise shim がバージョンを解決するには設定ファイルが必要です。snapshot スクリプトはワークスペースの `mise.toml` を `/home/agent/.config/mise/config.toml`（グローバル設定）にコピーして、shim がどのバージョンを使うか分かるようにしています。

### --include のコピー

`--include` で指定されたディレクトリは `/tmp/aw-include-N` に ro マウントされ、スクリプト内で `cp -a` でコンテナ内の宛先パスにコピーします。ro マウントからの読み取りコピーなので問題ありません。

### env ファイルの生成

snapshot スクリプトは以下の 3 ファイルをイメージに焼き込みます:

| ファイル | 内容 |
|---------|------|
| `/home/agent/.aw_env.sh` | PATH 設定、devbox/mise 環境変数 |
| `/home/agent/.bashrc` | `.aw_env.sh` を source する |
| `/home/agent/.bash_profile` | `.bashrc` を source する |

**ただし、これらは通常起動時に entrypoint.sh が上書きします。** snapshot が生成した env ファイルはあくまで commit されるイメージに含まれるだけで、実際の起動時には entrypoint.sh が正しいパス（`HOST_WORKSPACE`）で再生成します。

### --env の焼き込み

`--env KEY=VAL` はスクリプト内ではなく、`docker commit --change 'ENV KEY=VAL'` でイメージメタデータとして焼き込まれます。これにより entrypoint.sh に関係なく、コンテナ起動時に環境変数が常に設定されます。

## 通常起動時の entrypoint.sh

エクスポートしたイメージを `aw dev` で起動すると、以下が起きます:

### 1. マウント

通常起動時のワークスペースマウントは snapshot と異なります:

```
# 通常起動: WorkDir → WorkDir（同じパスで）
/Users/kono/project → /Users/kono/project

# snapshot 時: OrigWorkDir → /workspace（固定パスに）
/Users/kono/project → /workspace (ro)
```

通常起動では `HOST_WORKSPACE` 環境変数にホスト側パスが渡されます。

### 2. entrypoint.sh の処理

```
entrypoint.sh
  │
  ├── skip_devbox_install: true の場合 → devbox install をスキップ
  │   （パッケージは snapshot で焼き込み済み）
  │
  ├── skip_mise_install: true の場合 → mise install をスキップ
  │   （ツールは snapshot で焼き込み済み）
  │
  ├── .aw_env.sh を新規生成 ← snapshot が書いたものを上書き
  │   ├── Nix の PATH 設定
  │   ├── devbox global shellenv → 焼き込み済みパッケージの PATH
  │   ├── devbox shellenv (HOST_WORKSPACE) → プロジェクト devbox の PATH
  │   ├── mise shims の PATH
  │   ├── MISE_TRUSTED_CONFIG_PATHS=HOST_WORKSPACE
  │   └── DOCKER_HOST（DooD 用）
  │
  ├── .bashrc を新規生成（.aw_env.sh を source）
  ├── .bash_profile を新規生成（.bashrc を source）
  │
  └── exec bash -lc "$@"  ← ツール起動（claude, codex 等）
```

### 3. .aw_env.sh の読み込みチェーン

```
bash 起動
  ├── login shell → .bash_profile → .bashrc → .aw_env.sh
  └── non-interactive (BASH_ENV) → .aw_env.sh を直接読み込み
```

`BASH_ENV="/home/agent/.aw_env.sh"` が Dockerfile の ENV で設定されているため、非インタラクティブシェル（スクリプト実行等）でも自動的に環境が読み込まれます。

### 4. snapshot の env ファイルが上書きされても問題ない理由

snapshot が生成する `.aw_env.sh` と entrypoint.sh が生成する `.aw_env.sh` はほぼ同じ内容ですが、1 点違いがあります:

- snapshot 版: `MISE_TRUSTED_CONFIG_PATHS="/workspace"`, `devbox shellenv` のパスも `/workspace`
- entrypoint 版: `MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE}"`, `devbox shellenv` のパスも `${HOST_WORKSPACE}`

entrypoint.sh が上書きするため、実行時のパスは常に正しい `HOST_WORKSPACE`（ホスト側のプロジェクトパス）になります。焼き込み済みツール（devbox global、mise global config）は `HOST_WORKSPACE` に依存しないので、どちらのパスでも動作します。

## まとめ: 何がイメージに焼き込まれ、何が起動時に設定されるか

| 内容 | 焼き込み（snapshot） | 起動時（entrypoint） |
|------|---------------------|---------------------|
| devbox パッケージ（/nix/store） | install 済み | skip_devbox_install で省略 |
| mise ツール（/home/agent/.local/share/mise） | install 済み | skip_mise_install で省略 |
| mise グローバル config | コピー済み | 変更なし |
| .aw_env.sh | 生成される | **上書きされる** |
| .bashrc / .bash_profile | 生成される | **上書きされる** |
| --env の環境変数 | docker commit --change | そのまま継承 |
| --include のファイル | コピー済み | 変更なし |
