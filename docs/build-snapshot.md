# Build & Snapshot の内部動作

`aw build` がイメージをビルドし、ツールを焼き込み、ビルドしたイメージが起動時にどう動くかを説明します。

> **Note:** Dockerfile テンプレートは Go テンプレートとして実装されており、`container_user` の値に応じてユーザー名やホームディレクトリが動的にレンダリングされます。entrypoint.sh と aw-init.sh は静的スクリプトで、ランタイム環境変数（`AW_USER`, `AW_HOME`）で動作します。以下の説明ではデフォルトユーザー `agent`（ホーム `/home/agent`）を使用しています。

## 全体の流れ

```
aw build dev --save image.tar
```

1. **イメージ取得** — ビルド入力がなければ公式イメージを pull、ビルド入力があれば OS テンプレートからビルド、`image:` 設定時は既存イメージを使用、`dockerfile:` 設定時はカスタム Dockerfile でビルド
2. **snapshot** — 一時コンテナを起動し、ワークスペースのパッケージをインストールして `docker commit`（`aw-build:<profile>-<hash>` に保存、公式イメージは上書きしない）
3. **tar 出力** — `--save` 指定時のみ `docker save` でイメージを tar に書き出す
4. **config 書き戻し** — プロファイルに `image:` を書き込む（デフォルト。`--no-apply` で抑止）

## ビルド方式の選び方

`aw build` には 4 つのビルド方式があります。プロファイルの設定とフラグの組み合わせで決まります。

### 一覧

| 方式 | 設定 | ベースイメージ | カスタマイズ手段 | ビルド速度 |
|------|------|---------------|-----------------|-----------|
| **公式イメージ + snapshot** | （デフォルト） | 公式プリビルトイメージ (GHCR) | mise.toml, include, env | 高速 |
| **テンプレートビルド + snapshot** | `packages` 等のビルド入力 | OS テンプレート Dockerfile | packages, build_env, ca_cert, mise.toml | 遅い |
| **カスタム Dockerfile** | `dockerfile:` | 自分で書いた Dockerfile | Dockerfile 内で自由 | Dockerfile 次第 |
| **既存イメージ + snapshot** | `image:` (dockerfile なし) | 指定したイメージ | mise.toml, include, env | 高速 |

> **Note:** `aw build` では、`image:` が設定されていても `packages` / `ca_cert` / `build_env` などをイメージに焼き込むため、テンプレートからビルドします。通常起動では `packages` / `packages.txt` を起動時にインストールできます。

通常起動で公式イメージにない OS パッケージを使う場合、新しいコンテナを起動するたびにインストールが必要です。`aw build` で焼き込むと、以降の起動ではパッケージの存在確認だけで済みます。

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
aw build dev
```

**「apt パッケージや CA 証明書が必要」→ テンプレートビルド**

`aw build` で `packages` フィールドや `ca_cert` を焼き込む場合、テンプレートから Dockerfile をビルドします。`packages` を設定して `aw build` を実行するとこの方式になります。

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
aw build dev
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
aw build dev
```

`image` と `dockerfile` を併用すると、`aw run` は `image` を使い、`aw build` は `dockerfile` でビルドします。ビルド済みイメージで普段は高速起動し、Dockerfile を変更したときだけ再ビルドするワークフローに便利です。

```yaml
profiles:
  dev:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.dev
    image: 'aw-build:dev-xxxx'  # aw build で書き戻された値
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
aw build ml-dev
# → golang イメージの上に python + uv を追加した新イメージが作られる
```

> **ベースイメージの要件:** snapshot は `sudo` が使える環境を前提とし、commit 時に `ENTRYPOINT ["/entrypoint.sh"]` を設定します。`aw build` で作成したイメージや aw 公式イメージをベースにすることを推奨します。外部の素の Docker イメージでは snapshot が失敗する可能性があります。

> **packages / ca_cert / build_env との関係:** `aw build` では、`image` が設定されていても `packages`、`packages.txt`、`ca_cert`、`build_env` を焼き込むために `image` を無視し、テンプレートからフルビルドします。通常起動では `packages` / `packages.txt` は既存イメージ上でインストールされます。

### カスタマイズ逆引きリファレンス

「やりたいこと」からどの設定を使えばよいかを引けます。

| やりたいこと | 使う設定 | ビルド方式 | 例 |
|---|---|---|---|
| Go, Python, Node 等のランタイムを追加 | `mise.toml` | どの方式でも可 | `[tools]` に `go = "1.23"` |
| jq, ripgrep 等の OS パッケージを追加 | `packages:` or `packages.txt` | `aw build` 時にテンプレートビルド | `packages: [jq, ripgrep]` |
| 社内 CA 証明書を組み込む | `ca_cert:` | テンプレートビルド（自動） | `ca_cert: certs/corp-ca.pem` |
| Docker ビルド時に変数を渡す | `build_env:` or `--build-arg` | テンプレートビルド（自動） | `build_env: {GITHUB_TOKEN: xxx}` |
| ホストのファイルをイメージに焼き込む | `--include src:dst` | snapshot で処理 | `--include ./certs:/usr/local/share/ca-certificates` |
| イメージに環境変数を焼き込む | `--env KEY=VAL` | snapshot で処理 | `--env HTTP_PROXY=http://proxy:8080` |
| ベースイメージから完全に制御したい | `dockerfile:` | カスタム Dockerfile | `dockerfile: docker/Dockerfile.dev` |
| 既存イメージにツールだけ追加したい | `image:` + `mise.toml` | 既存イメージ + snapshot | `image: aw-build:base-xxxx` |

**ビルド方式の自動選択ルール:**

`aw build` では `packages`、`packages.txt`、`ca_cert`、`build_env` のいずれかが設定されている場合、Dockerfile のレイヤーに焼き込むため、`image:` が設定されていてもテンプレートからビルドします。これらの設定と既存イメージ + snapshot を同時に使うことはできません。

```
mise.toml のみ          → 公式イメージ or 既存イメージ + snapshot（高速）
packages / ca_cert あり → テンプレートビルド + snapshot（image: は無視される）
dockerfile あり          → カスタム Dockerfile（image: は aw run 用）
```

### フラグの組み合わせと動作

config 書き戻し（プロファイルへの `image:` の書き込み）は **デフォルトで有効**です。
抑止したい場合は `--no-apply` を付けます。`aw build` が書くのは `image:` だけで、
`skip_mise_install` / `mise_install` は変更しません（[mise install のスキップ判定](#mise-install-のスキップ判定)）。

動作はまず **ビルド入力の有無** で分かれます。入力があれば snapshot を取り、
なければ取るものがないので snapshot を行いません。

#### ビルド入力がある場合

`dockerfile` / `mise.toml` / `.mise.toml` / `packages.txt` / `packages` /
`ca_cert` / `build_env` / `container_user`（既定以外）/
`kubernetes.session_log` / `--include` / `--env` / `--build-arg` のいずれかがある場合。

`mount_zellij` はビルド入力ではありません。公式イメージに zellij / panecom /
aw-sockrelay が同梱されているため、ビルドせずに公式イメージをそのまま使います。

| コマンド | イメージ取得 | snapshot | tar 生成 | config 書き戻し | 備考 |
|---|---|---|---|---|---|
| `aw build <profile>` | o | o | - | o | |
| `aw build <profile> --no-apply` | o | o | - | - | ビルドのみ |
| `aw build <profile> --save file.tar` | o | o | o | o | |
| `aw build <profile> --no-apply --save file.tar` | o | o | o | - | 持ち込み先用の設定スニペットを表示 |
| `aw build <profile>`（`image` + `mise.toml`） | o（既存イメージ） | o | - | o | 既存イメージの上に焼き込む |
| `aw build <profile>`（`image` + `packages`） | o（テンプレート） | o | - | o | `image` は無視される |
| `aw build <profile> --no-cache` | o（テンプレート） | o | - | o | |
| `aw build <profile> --push --registry ghcr.io/myorg` | o | o | - | o | push 先の名前を書き戻す |
| `aw build <profile> --push --registry ghcr.io/myorg --no-apply` | o | o | - | - | push のみ |

#### ビルド入力がない場合

焼き込むものがないため snapshot は行いません。`image:` が設定されていればその
イメージを、なければ公式イメージを対象にします。

| コマンド | イメージ取得 | snapshot | tar 生成 | config 書き戻し | 備考 |
|---|---|---|---|---|---|
| `aw build <profile>` | o（公式イメージを pull） | - | - | o | 公式イメージ名を書き戻す |
| `aw build <profile> --no-apply` | - | - | - | - | Warning を出して何もしない |
| `aw build <profile> --save file.tar` | o（公式イメージを pull） | - | o | o | |
| `aw build <profile> --no-apply --save file.tar` | o（公式イメージを pull） | - | o | - | 設定スニペットを表示 |
| `aw build <profile> --push --registry ghcr.io/myorg` | o（公式イメージを pull） | - | - | o | 公式イメージをそのまま push |
| `aw build <profile>`（`image` 設定あり） | - | - | - | - | 既にそのイメージなので何もしない |
| `aw build <profile> --save file.tar`（`image` 設定あり） | o（`image` を解決） | - | o | - | 書き戻す内容が変わらないため config は触らない |
| `aw build <profile> --push --registry ghcr.io/myorg`（`image` 設定あり） | o（`image` を解決） | - | - | o | push 後は名前が変わるので書き戻す |

`image:` を `--save` / `--push` の対象にする場合、その参照がローカルにあるかを確認し、
なければ `image_pull_policy` に従って pull します。取得できない場合は公式イメージに
切り替えず、エラーで停止します（別のイメージを tar やレジストリに書き込まないため）。
`image_pull_policy: never` でローカルにないときも同様にエラーです。

### --no-apply

config を書き換えずにイメージだけ作りたいときに使います。

- エアギャップ用に tar を書き出すだけで、手元のプロファイルは公式イメージのままにしたい
- CI でビルドの成否だけ確認したい
- レジストリへ push するだけで、`image:` の固定は別途 PR で行いたい

`--no-apply --save file.tar` の場合は、持ち込み先に貼り付けるための config スニペットを
標準エラーに表示します。

## mise install のスキップ判定

`aw build` は焼き込んだ mise 設定の指紋をイメージ内の
`/home/agent/.aw_mise_fingerprint` に記録します。起動時に `aw` がワークスペースの
指紋を計算し直してコンテナへ渡し、entrypoint が 2 つを比較します。

```
aw build  : mise.toml の指紋 → イメージ内に記録
aw <prof> : mise.toml の指紋 → AW_MISE_FINGERPRINT 環境変数で渡す
entrypoint: 2 つが一致 → mise install を呼ばない / 不一致 → mise install
```

指紋はホスト側の config ではなくイメージの中にあるため、tar で持ち出しても
レジストリ経由で配っても判定はそのまま機能します。

### 判定表

| 状況 | 起動時の挙動 |
|---|---|
| 指紋が一致 | `mise install` を呼ばない |
| 指紋が不一致（mise.toml を編集・追記） | `mise install` |
| イメージに指紋がない（公式イメージ、v5 より前の snapshot） | `mise install` |
| 指紋の対象外入力がワークスペースにある | `mise install`（安全側） |
| `mise_install: false` / `skip_mise_install: true` | `mise install` を呼ばない（強制オプトアウト） |

### 指紋の対象

snapshot が実際にイメージへ取り込む `mise.toml` と `.mise.toml` の内容だけです。

以下がワークスペースにある場合、指紋では入力全体を説明できないため、`aw` は指紋を
使わず毎回 `mise install` を実行します（`internal/mise` の `Fingerprint` が
`ok = false` を返します）。

- `mise.lock`
- `.tool-versions`
- `mise.<env>.toml` / `.mise.<env>.toml`
- `mise/config.toml` / `.mise/config.toml`
- `.config/mise.toml` / `.config/mise/config.toml`
- `mise.toml` 内の `include` 指定

両側の指紋は Go の同一関数（`internal/mise.Fingerprint`）が計算します。シェル側に
二つ目の実装はないため、ビルド時と起動時で判定がずれることはありません。

### なぜ毎起動 install ではないのか

`mise install` は `--force` なしなら既存バージョンを入れ直さず不足分だけを入れますが、
それでも mise の起動・設定解決・バージョン確認のコストは毎回かかります。指紋が一致する
間は呼び出し自体を省くことで、焼き込み済み環境の起動を最短にしています。

### ツールを削除したとき

`mise.toml` からツールを削除すると指紋が変わるため `mise install` は走りますが、
**イメージに焼き込み済みのバイナリは残ります**。`mise install` は不要になったツールを
削除しないためです。イメージから取り除くには `aw build` でイメージを作り直してください。

### レジストリへの push

`--push --registry <registry>` でビルド済みイメージをコンテナレジストリに push できます。K8s manifest 生成 (`aw manifest`) と組み合わせて使用します。

```bash
# ビルド + push + config にイメージ名を書き戻し
aw build claude --push --registry ghcr.io/myorg

# ビルド + push のみ（config は書き換えない）
aw build claude --push --registry ghcr.io/myorg --no-apply
```

イメージ名のレジストリプレフィックスは `distribution/reference` で正規に解析されるため、`ghcr.io`、`localhost:5000`、ECR/GCR 等のレジストリに対応しています。

### --no-cache

`--no-cache` は `image:` 設定を無視してテンプレートからビルドし、Docker のビルドキャッシュも無効にします。

ただしビルド入力（`dockerfile`、`mise.toml` / `.mise.toml`、`packages.txt`、`packages`、`build_env`、`--include`、`--env`、`--build-arg`、`kubernetes.session_log`）が 1 つもない場合、`aw build` はテンプレートをビルドせず公式イメージをそのまま使います。`--no-cache` を付けてもこの判定は変わりません。

> **v4 からの変更:** 非推奨だった `--from-template` は削除されました。`--no-cache` を使ってください。

## aw save — 対話的なカスタマイズの保存

`aw build` がワークスペースの設定ファイル（mise.toml 等）を元にイメージを焼き込むのに対し、`aw save` はコンテナ内で対話的に行った変更（`apt install`、設定変更など）をそのまま保存します。

```bash
aw claude                  # コンテナを起動
# ... コンテナ内でカスタマイズ ...
aw save                    # fzf でコンテナを選択 → commit → .aw.yml を更新
```

### 動作の流れ

1. docker / podman 両方からコンテナを検索（`--runtime` で限定可能）
2. `aw-<profile>-<timestamp>` パターンに合致するコンテナを fzf ピッカーで一覧表示（snapshot コンテナは除外）
3. 選択したコンテナに対して `docker commit` を実行（ENTRYPOINT/CMD を `aw build` と同じ設定にリセット）
4. コンテナの `HOST_WORKSPACE` 環境変数からワークスペースを特定し、git root のプロジェクト config（`.aw.yml` / `.aw.yaml` / `.agent-workspace.yml`）に `image` と `mise_install: false` を書き込む

`aw save` が `mise_install: false` を書くのは、commit 対象のコンテナが手作業で
組み上げられたもので、ワークスペースの `mise.toml` と対応付けられないためです。
指紋による自動判定が使えないので、明示的なオプトアウトを固定します。
`aw build` が書くのは `image:` だけで、この点が両者で異なります。

### aw build との比較

| 観点 | `aw build` | `aw save` |
|------|-------------------|-----------|
| 入力 | mise.toml / packages.txt | コンテナ内の手作業 |
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

`internal/image/embed/Dockerfile.debian12.tmpl` がベースイメージを定義しています。

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

`mise install` はカレントディレクトリに中間生成物を書き込みます:

- mise: `.mise/` ディレクトリ

`/workspace` は ro マウントなので、これらの書き込みが失敗します。そのため snapshot スクリプトは以下の手順を踏みます:

```bash
WORK="/tmp/aw-snapshot-work"
cp -r "$WORKSPACE/." "$WORK/"    # ro マウントから書き込み可能な場所にコピー
cd "$WORK" && mise install       # こちらで実行
rm -rf "$WORK"                   # commit 前にクリーンアップ
```

### ツールの焼き込み先

| ツール | インストール先 | 参照方法 |
|--------|-------------|----------|
| mise ツール（jq, go 等） | `/home/agent/.local/share/mise/installs/<tool>/<ver>/` | `/home/agent/.local/share/mise/shims/` 経由 |

mise shim がバージョンを解決するには設定ファイルが必要です。snapshot スクリプトはワークスペースの `mise.toml` を `/home/agent/.config/mise/config.toml`（グローバル設定）にコピーして、shim がどのバージョンを使うか分かるようにしています。

### --include のコピー

`--include` で指定されたディレクトリは `/tmp/aw-include-N` に ro マウントされ、スクリプト内で `cp -a` でコンテナ内の宛先パスにコピーします。ro マウントからの読み取りコピーなので問題ありません。

コピー先のファイルには `chown $(id -u):0` + `chmod -R g=u`（GID 0 パターン）を適用します。これにより、`--save` で書き出したイメージを異なる UID の環境にロードした場合でも、`--group-add 0` によるグループ権限で include ファイルにアクセスできます。

### env ファイルの生成

snapshot スクリプトは以下の 3 ファイルをイメージに焼き込みます:

| ファイル | 内容 |
|---------|------|
| `/home/agent/.aw_mise_fingerprint` | 焼き込んだ mise 設定の指紋 |
| `/home/agent/.aw_env.sh` | PATH 設定、mise 環境変数 |
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
  ├── [entrypoint.sh] mise 指紋が一致 or mise_install: false → mise install をスキップ
  │   （ツールは snapshot で焼き込み済み）
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

- snapshot 版: `MISE_TRUSTED_CONFIG_PATHS="/workspace"`
- aw-init.sh 版: `MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE}"`

aw-init.sh が上書きするため、実行時のパスは常に正しい `HOST_WORKSPACE`（コンテナ内から見えるプロジェクトパス）になります。焼き込み済みツール（mise global config）は `HOST_WORKSPACE` に依存しないので、どちらのパスでも動作します。

## まとめ: 何がイメージに焼き込まれ、何が起動時に設定されるか

| 内容 | 焼き込み（snapshot） | 起動時（aw-init.sh + entrypoint） |
|------|---------------------|---------------------|
| mise ツール（/home/agent/.local/share/mise） | install 済み | 指紋が一致すれば省略 |
| mise グローバル config | コピー済み | 変更なし |
| .aw_env.sh | 生成される | **上書きされる** |
| .bashrc / .bash_profile | 生成される | **上書きされる** |
| --env の環境変数 | docker commit --change | そのまま継承 |
| --include のファイル | コピー済み | 変更なし |
