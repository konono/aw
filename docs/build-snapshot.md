# Build & Snapshot の内部動作

`aw build` がイメージをビルドし、ツールを焼き込み、ビルドしたイメージが起動時にどう動くかを説明します。

> **Note:** Dockerfile テンプレートは Go テンプレートとして実装されており、`container_user` の値に応じてユーザー名やホームディレクトリが動的にレンダリングされます。entrypoint.sh と aw-init.sh は静的スクリプトで、ランタイム環境変数（`AW_USER`, `AW_HOME`）で動作します。以下の説明ではデフォルトユーザー `agent`（ホーム `/home/agent`）を使用しています。

## 全体の流れ

```
aw build dev --save image.tar
```

1. **イメージ取得** — 公式イメージを pull、`--from-template` でテンプレートからビルド、`image:` 設定時は既存イメージを使用、`dockerfile:` 設定時はカスタム Dockerfile でビルド
2. **snapshot** — 一時コンテナを起動し、ワークスペースのパッケージをインストールして `docker commit`（`aw-build:<profile>-<hash>` に保存、公式イメージは上書きしない）
3. **tar 出力** — `--save` 指定時のみ `docker save` でイメージを tar に書き出す

## ビルド方式の選び方

`aw build` には 4 つのビルド方式があります。プロファイルの設定とフラグの組み合わせで決まります。

### 一覧

| 方式 | 設定 | ベースイメージ | カスタマイズ手段 | ビルド速度 |
|------|------|---------------|-----------------|-----------|
| **公式イメージ + snapshot** | （デフォルト） | 公式プリビルトイメージ (GHCR) | mise.toml, include, env | 高速 |
| **テンプレートビルド + snapshot** | `--from-template` or `packages` | OS テンプレート Dockerfile | packages, build_env, ca_cert, mise.toml | 遅い |
| **カスタム Dockerfile** | `dockerfile:` | 自分で書いた Dockerfile | Dockerfile 内で自由 | Dockerfile 次第 |
| **既存イメージ + snapshot** | `image:` (dockerfile なし) | 指定したイメージ | mise.toml, include, env | 高速 |

> **Note:** `image:` が設定されていても、`packages` / `ca_cert` / `build_env` などテンプレートビルドが必要なカスタマイズがある場合は、自動的にテンプレートビルドにフォールバックします。

### どれを使うべきか

**「mise.toml にツールを書くだけで十分」→ 公式イメージ + snapshot（デフォルト）**

最もシンプル。mise.toml に必要なツール（go, python, node 等）を書いて `aw build` するだけ。

```yaml
# .aw.yml
profiles:
  dev:
    environment: container
    launch: claude
```

```toml
# mise.toml
[tools]
go = "1.23"
node = "22"
```

```bash
aw build dev --apply
```

**「apt パッケージや CA 証明書が必要」→ テンプレートビルド**

`packages` フィールドや `ca_cert` が必要な場合、テンプレートから Dockerfile をビルドします。`packages` を設定すると自動的にこの方式になります。

```yaml
profiles:
  dev:
    environment: container
    launch: claude
    packages:
      - postgresql-client
      - libpq-dev
    ca_cert: certs/corporate-ca.pem
```

```bash
aw build dev --apply
```

**「Dockerfile を完全にコントロールしたい」→ カスタム Dockerfile**

ベースイメージの選択、マルチステージビルド、独自のレイヤー構成など、Dockerfile レベルの制御が必要な場合。

```yaml
profiles:
  dev:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.dev
```

```bash
aw build dev --apply
```

`image` と `dockerfile` を併用すると、`aw run` は `image` を使い、`aw build` は `dockerfile` でビルドします。ビルド済みイメージで普段は高速起動し、Dockerfile を変更したときだけ再ビルドするワークフローに便利です。

```yaml
profiles:
  dev:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.dev
    image: 'aw-build:dev-xxxx'  # aw build --apply で書き戻された値
```

**「既存イメージにツールを追加したい」→ 既存イメージ + snapshot**

チームで共有するベースイメージや、別リポジトリでビルドしたイメージの上に、リポジトリ固有のツールを mise.toml で追加する場合。

```yaml
profiles:
  ml-dev:
    environment: container
    launch: claude
    image: 'aw-build:golang-xxxx'  # 別リポでビルドした Go 入りイメージ
```

```toml
# mise.toml
[tools]
python = "3.12"
uv = "latest"
```

```bash
aw build ml-dev --apply
# → golang イメージの上に python + uv を追加した新イメージが作られる
```

> **ベースイメージの要件:** snapshot は `sudo` が使える環境を前提とし、commit 時に `ENTRYPOINT ["/entrypoint.sh"]` を設定します。`aw build` で作成したイメージや aw 公式イメージをベースにすることを推奨します。外部の素の Docker イメージでは snapshot が失敗する可能性があります。

> **packages / ca_cert / build_env との関係:** `image` が設定されていても、`packages`、`packages.txt`、`ca_cert`、`build_env` などテンプレートビルドが必要なカスタマイズがある場合は、`image` は自動的に無視されてテンプレートからフルビルドされます。これらのカスタマイズは Dockerfile レイヤーで処理する必要があるためです。

### カスタマイズ逆引きリファレンス

「やりたいこと」からどの設定を使えばよいかを引けます。

| やりたいこと | 使う設定 | ビルド方式 | 例 |
|---|---|---|---|
| Go, Python, Node 等のランタイムを追加 | `mise.toml` | どの方式でも可 | `[tools]` に `go = "1.23"` |
| jq, ripgrep 等の OS パッケージを追加 | `packages:` or `packages.txt` | テンプレートビルド（自動） | `packages: [jq, ripgrep]` |
| 社内 CA 証明書を組み込む | `ca_cert:` | テンプレートビルド（自動） | `ca_cert: certs/corp-ca.pem` |
| Docker ビルド時に変数を渡す | `build_env:` or `--build-arg` | テンプレートビルド（自動） | `build_env: {GITHUB_TOKEN: xxx}` |
| ホストのファイルをイメージに焼き込む | `--include src:dst` | snapshot で処理 | `--include ./certs:/usr/local/share/ca-certificates` |
| イメージに環境変数を焼き込む | `--env KEY=VAL` | snapshot で処理 | `--env HTTP_PROXY=http://proxy:8080` |
| ベースイメージから完全に制御したい | `dockerfile:` | カスタム Dockerfile | `dockerfile: docker/Dockerfile.dev` |
| 既存イメージにツールだけ追加したい | `image:` + `mise.toml` | 既存イメージ + snapshot | `image: aw-build:base-xxxx` |

**ビルド方式の自動選択ルール:**

`packages`、`packages.txt`、`ca_cert`、`build_env` のいずれかが設定されている場合、Dockerfile のレイヤーで処理する必要があるため、`image:` が設定されていても自動的にテンプレートビルドにフォールバックします。これらの設定と既存イメージ + snapshot を同時に使うことはできません。

```
mise.toml のみ          → 公式イメージ or 既存イメージ + snapshot（高速）
packages / ca_cert あり → テンプレートビルド + snapshot（image: は無視される）
dockerfile あり          → カスタム Dockerfile（image: は aw run 用）
```

### フラグの組み合わせと動作

| コマンド | イメージ取得 | snapshot | tar 生成 | config 書き戻し |
|---|---|---|---|---|
| `aw build <profile>` | o | o | - | - |
| `aw build <profile> --save file.tar` | o | o | o | - |
| `aw build <profile> --apply` | o | o | - | o |
| `aw build <profile> --apply --save file.tar` | o | o | o | o |
| `aw build <profile>`（`image` 設定あり） | o（既存イメージ） | o | - | - |
| `aw build <profile>`（`image` + `packages`） | o（テンプレート） | o | - | - |
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

### --from-template と --no-cache

`--from-template` はテンプレートビルドを強制します。`--no-cache` は `--from-template` を暗黙的に有効にし、Docker のビルドキャッシュも無効にします。`image` が設定されている場合、どちらのフラグも `image` を無視してテンプレートからフルビルドします。

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
