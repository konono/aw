# Build & Snapshot の内部動作

`aw build` がイメージをビルドし、ツールを焼き込み、ビルドしたイメージが起動時にどう動くかを説明します。

> **Note:** Dockerfile テンプレートは Go テンプレートとして実装されており、`container_user` の値に応じてユーザー名やホームディレクトリが動的にレンダリングされます。entrypoint.sh と aw-init.sh は静的スクリプトで、ランタイム環境変数（`AW_USER`, `AW_HOME`）で動作します。以下の説明ではデフォルトユーザー `agent`（ホーム `/home/agent`）を使用しています。

## 全体の流れ

```
aw build dev --save image.tar
```

1. **イメージ取得** — 公式イメージを pull（または `--from-template` でテンプレートからビルド）
2. **snapshot** — 一時コンテナを起動し、ワークスペースのパッケージをインストールして `docker commit`（`aw-build:<profile>-<hash>` に保存、公式イメージは上書きしない）
3. **tar 出力** — `--save` 指定時のみ `docker save` でイメージを tar に書き出す

### フラグの組み合わせと動作

| コマンド | イメージ取得 | snapshot | tar 生成 | config 書き戻し |
|---|---|---|---|---|
| `aw build <profile>` | o | o | - | - |
| `aw build <profile> --save file.tar` | o | o | o | - |
| `aw build <profile> --apply` | o | o | - | o |
| `aw build <profile> --apply --save file.tar` | o | o | o | o |
| `aw build <profile>`（`image` 設定あり） | o（既存イメージ） | o | - | - |
| `aw build <profile> --from-template` | o（テンプレート） | o | - | - |
| `aw build <profile> --push --registry ghcr.io/myorg` | o | o | - | - | レジストリに push |
| `aw build <profile> --push --registry ghcr.io/myorg --apply` | o | o | - | o | push + config 書き戻し |

### レジストリへの push

`--push --registry <registry>` でビルド済みイメージをコンテナレジストリに push できます。K8s manifest 生成 (`aw manifest`) と組み合わせて使用します。

```bash
# ビルド + push
aw build claude --push --registry ghcr.io/myorg

# ビルド + push + config にイメージ名を書き戻し
aw build claude --push --registry ghcr.io/myorg --apply
```

イメージ名のレジストリプレフィックスは `distribution/reference` で正規に解析されるため、`ghcr.io`、`localhost:5000`、ECR/GCR 等のレジストリに対応しています。

### デフォルトと --from-template の使い分け

| 観点 | default（公式イメージベース） | --from-template |
|------|------|------|
| ベースイメージ | ghcr.io/konono/aw-claude:X.Y.Z-osN | テンプレートからビルド |
| ビルド速度 | 高速（pull + commit のみ） | 遅い（Dockerfile ビルド + commit） |
| カスタマイズ | include, env, workspace mise.toml | packages, build_env / --build-arg, ca_cert, workspace mise.toml |
| ユースケース | 一般ユーザー | packages や ca_cert があるパワーユーザー |

## カスタムイメージベースの増分ビルド

`image` が設定されていて `dockerfile` がない場合、`aw build` はそのイメージをベースに snapshot で増分ビルドを実行します。既存のカスタムイメージにワークスペースの mise.toml 等のツールを追加したイメージを作れます。

### ユースケース: 言語別ベースイメージの使い回し

たとえば、Go 開発用のベースイメージを一度ビルドし、それを別リポジトリで Python ツールを追加して使うケースです。

**Step 1: Go ベースイメージをビルド（リポジトリ A）**

```yaml
# リポジトリ A の .aw.yml
profiles:
  golang:
    environment: container
    launch: claude
```

```toml
# リポジトリ A の mise.toml
[tools]
go = "1.23"
```

```bash
aw build golang --apply
# → aw-build:golang-xxxx イメージが作成され、.aw.yml の image に書き戻される
```

**Step 2: Go イメージをベースに Python を追加（リポジトリ B）**

```yaml
# リポジトリ B の .aw.yml
profiles:
  ml-dev:
    environment: container
    launch: claude
    image: 'aw-build:golang-xxxx'  # Step 1 で作ったイメージ
```

```toml
# リポジトリ B の mise.toml
[tools]
python = "3.12"
uv = "latest"
```

```bash
aw build ml-dev --apply
# → aw-build:golang-xxxx をベースに python + uv を snapshot で追加
# → aw-build:ml-dev-yyyy イメージが作成され、.aw.yml の image に書き戻される
```

結果として `ml-dev` プロファイルのイメージには Go + Python + uv がすべて入った状態になります。`aw run ml-dev` はこのイメージをそのまま使うため、起動時の mise install が不要で高速です。

### 動作の仕組み

1. `resolveImage()` が `image` に指定されたイメージをそのまま返す（ビルドしない）
2. `runSnapshot()` がそのイメージで一時コンテナを起動し、ワークスペースの mise.toml をコピーして `mise install` を実行
3. `docker commit` で新しいイメージとして保存

### --from-template / --no-cache との関係

`--from-template` または `--no-cache` を指定すると、`image` は無視されてテンプレートからフルビルドされます。カスタムイメージベースの増分ビルドではなく、ゼロからビルドし直したい場合に使います。

## aw save — 対話的なカスタマイズの保存

`aw build` がワークスペースの設定ファイル（mise.toml 等）を元にイメージを焼き込むのに対し、`aw save` はコンテナ内で対話的に行った変更（`apt install`、設定変更など）をそのまま保存します。

```bash
aw claude                  # コンテナを起動
# ... コンテナ内でカスタマイズ ...
aw save                    # fzf でコンテナを選択 → commit → .aw.yml を更新
```

### 動作の流れ

1. docker / podman 両方からコンテナを検索（`--runtime` で限定可能）
2. `aw-<profile>-<timestamp>` パターンに合致するコンテナを fzf ピッカーで一覧表示（snapshot / team コンテナは除外）
3. 選択したコンテナに対して `docker commit` を実行（ENTRYPOINT/CMD を `aw build` と同じ設定にリセット）
4. コンテナの `HOST_WORKSPACE` 環境変数からワークスペースを特定し、git root のプロジェクト config（`.aw.yml` / `.aw.yaml` / `.agent-workspace.yml`）に `image` と `skip_mise_install: true` を書き込む（devbox プロファイルの場合は `skip_devbox_install: true` も設定）

### aw build との比較

| 観点 | `aw build --apply` | `aw save` |
|------|-------------------|-----------|
| 入力 | mise.toml / devbox.json / packages.txt | コンテナ内の手作業 |
| 再現性 | 高（宣言的。同じ設定から同じイメージ） | 低（手動操作の結果） |
| ユースケース | 構成が固まった環境の高速化 | 試行錯誤中の環境保存 |
| イメージ名 | `aw-build:<profile>-<hash>` | `aw-save:<profile>-<timestamp>` |

### `aw build --save` との違い

`aw build --save file.tar` はビルド済みイメージを tar ファイルにエクスポートする機能です。`aw save` とは別のコマンドです。

| コマンド | 動作 |
|---------|------|
| `aw build --save file.tar` | ビルドしたイメージを tar にエクスポート |
| `aw save` | 実行済みコンテナを commit して .aw.yml を更新 |

## イメージビルド（Dockerfile）

`internal/image/embed/Dockerfile.debian12.tmpl` がベースイメージを定義しています（`package_manager: apt` の場合）。

```
debian:bookworm-slim
  ├── apt: git, curl, ca-certificates, wget, openssh-client, sudo, xz-utils
  ├── user: agent (sudo NOPASSWD for all UIDs)
  ├── AI ツール: curl ベースの install script でインストール（claude, codex 等）
  ├── ENV:
  │   ├── HOME=/home/agent
  │   ├── BASH_ENV=/home/agent/.aw_env.sh    ← 非インタラクティブシェルで自動読み込み
  │   └── PATH に .local/bin を追加
  ├── gh CLI + mise バイナリをインストール（ビルド時、バージョン固定）
  ├── ENTRYPOINT ["/entrypoint.sh"]
  └── WORKDIR /workspace
```

> **Note:** `package_manager: devbox` の場合は `Dockerfile.debian12.devbox.tmpl` が使用され、Nix + devbox が追加されます。

この時点では `.aw_env.sh` ファイルは存在しません。`BASH_ENV` は設定されているが、ファイルが作られるのは aw-init.sh 実行時（通常起動）または snapshot スクリプト実行時（build 時）です。

## snapshot スクリプトの動作

### マウント構成

```go
// runSnapshot() が設定するマウント:
/workspace     ← ec.OrigWorkDir（プロジェクトディレクトリ）を ro マウント
/tmp/aw-include-0  ← --include の src を ro マウント
/tmp/aw-include-1  ← 同上（複数指定可）
```

全マウントが **read-only** です。build はビルド操作であり、ホスト側のファイルを変更してはいけないためです。

### なぜワークスペースをコピーするか

`mise install`（および `package_manager: devbox` の場合は `devbox install`）はカレントディレクトリに中間生成物を書き込みます:

- mise: `.mise/` ディレクトリ
- devbox（devbox モード時）: `.devbox/gen/shell.nix`, `.devbox/virtenv/` など

`/workspace` は ro マウントなので、これらの書き込みが失敗します。そのため snapshot スクリプトは以下の手順を踏みます:

```bash
WORK="/tmp/aw-snapshot-work"
cp -r "$WORKSPACE/." "$WORK/"    # ro マウントから書き込み可能な場所にコピー
cd "$WORK" && mise install       # こちらで実行
cd "$WORK" && devbox install     # devbox モード時のみ
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

コピー先のファイルには `chown $(id -u):0` + `chmod -R g=u`（GID 0 パターン）を適用します。これにより、`--save` で書き出したイメージを異なる UID の環境にロードした場合でも、`--group-add 0` によるグループ権限で include ファイルにアクセスできます。

### env ファイルの生成

snapshot スクリプトは以下の 3 ファイルをイメージに焼き込みます:

| ファイル | 内容 |
|---------|------|
| `/home/agent/.aw_env.sh` | PATH 設定、devbox/mise 環境変数 |
| `/home/agent/.bashrc` | `.aw_env.sh` を source する |
| `/home/agent/.bash_profile` | `.bashrc` を source する |

**ただし、これらは通常起動時に aw-init.sh が上書きします。** snapshot が生成した env ファイルはあくまで commit されるイメージに含まれるだけで、実際の起動時には aw-init.sh が正しいパス（`HOST_WORKSPACE`）で再生成します。

### --env の焼き込み

`--env KEY=VAL` はスクリプト内ではなく、`docker commit --change 'ENV KEY=VAL'` でイメージメタデータとして焼き込まれます。これにより entrypoint.sh に関係なく、コンテナ起動時に環境変数が常に設定されます。

## 通常起動時の entrypoint.sh + aw-init.sh

ビルドしたイメージを `aw dev` で起動すると、以下が起きます:

### 1. マウント

通常起動時のワークスペースマウントは snapshot と異なります:

```
# 通常起動: WorkDir → WorkDir（同じパスで）
/Users/kono/project → /Users/kono/project

# snapshot 時: OrigWorkDir → /workspace（固定パスに）
/Users/kono/project → /workspace (ro)
```

通常起動では `HOST_WORKSPACE` 環境変数にコンテナ内から見えるワークスペースパスが渡されます（Linux/macOS ではホストパスと同一、Windows では `/c/Users/...` 形式に変換されます）。
aw CLI はランタイムで `/aw-init.sh` をマウントし、イメージ内蔵版を最新で上書きします。

### 2. aw-init.sh + entrypoint.sh の処理

```
entrypoint.sh → source /aw-init.sh
  │
  ├── [aw-init.sh] UID 不一致の検出と修正
  │   （--user で渡されたホスト UID とイメージ内のホームディレクトリ所有者が
  │   異なる場合、sudo chown -R で修正）
  │
  ├── [aw-init.sh] SSH 鍵コピー、git credential helper 設定
  │
  ├── [aw-init.sh] .aw_env.sh を新規生成 ← snapshot が書いたものを上書き
  │   ├── mise shims の PATH
  │   ├── MISE_TRUSTED_CONFIG_PATHS=HOST_WORKSPACE
  │   └── git credential helper
  │
  ├── [aw-init.sh] .bashrc / .bash_profile を新規生成
  │
  ├── [entrypoint.sh] skip_mise_install: true の場合 → mise install をスキップ
  │   （ツールは snapshot で焼き込み済み）
  │
  ├── [entrypoint.sh.devbox] skip_devbox_install: true の場合 → devbox install をスキップ
  │   （パッケージは snapshot で焼き込み済み）
  │
  ├── [entrypoint.sh.devbox] devbox/nix 固有の env を .aw_env.sh に追記
  │
  └── aw_exec "$@"  ← ツール起動（claude, codex 等）
```

### 3. .aw_env.sh の読み込みチェーン

```
bash 起動
  ├── login shell → .bash_profile → .bashrc → .aw_env.sh
  └── non-interactive (BASH_ENV) → .aw_env.sh を直接読み込み
```

`BASH_ENV="/home/agent/.aw_env.sh"` が Dockerfile の ENV で設定されているため、非インタラクティブシェル（スクリプト実行等）でも自動的に環境が読み込まれます。

### 4. snapshot の env ファイルが上書きされても問題ない理由

snapshot が生成する `.aw_env.sh` と aw-init.sh が生成する `.aw_env.sh` はほぼ同じ内容ですが、1 点違いがあります:

- snapshot 版: `MISE_TRUSTED_CONFIG_PATHS="/workspace"`, `devbox shellenv` のパスも `/workspace`
- aw-init.sh 版: `MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE}"`, `devbox shellenv` のパスも `${HOST_WORKSPACE}`

aw-init.sh が上書きするため、実行時のパスは常に正しい `HOST_WORKSPACE`（コンテナ内から見えるプロジェクトパス）になります。焼き込み済みツール（devbox global、mise global config）は `HOST_WORKSPACE` に依存しないので、どちらのパスでも動作します。

## まとめ: 何がイメージに焼き込まれ、何が起動時に設定されるか

| 内容 | 焼き込み（snapshot） | 起動時（aw-init.sh + entrypoint） |
|------|---------------------|---------------------|
| mise ツール（/home/agent/.local/share/mise） | install 済み | skip_mise_install で省略 |
| devbox パッケージ（/nix/store）※devbox モード時 | install 済み | skip_devbox_install で省略 |
| mise グローバル config | コピー済み | 変更なし |
| .aw_env.sh | 生成される | **上書きされる** |
| .bashrc / .bash_profile | 生成される | **上書きされる** |
| --env の環境変数 | docker commit --change | そのまま継承 |
| --include のファイル | コピー済み | 変更なし |
