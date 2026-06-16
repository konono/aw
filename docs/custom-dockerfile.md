# カスタム Dockerfile ガイド

`dockerfile` フィールドでカスタム Dockerfile を指定すると、通常の `docker build` と同じ感覚でイメージをカスタマイズできます。Dockerfile が置かれた**ディレクトリ全体**がビルドコンテキストになるため、`COPY` で同じディレクトリ内のファイルを自由に参照できます。

## 設定例

```yaml
profiles:
  playwright:
    environment: container
    launch: claude
    dockerfile: playwright-docker/Dockerfile  # git ルートからの相対パス
```

## aw-init.sh — 共通初期化スクリプト

aw はコンテナ起動時に `/aw-init.sh` を自動的にマウントします。このスクリプトをカスタム entrypoint で `source` するだけで、aw のランタイムセットアップが全て行われます:

- UID マッピング（`/etc/passwd` 動的注入）
- SSH 鍵のコピーとパーミッション修正
- Git credential helper のセットアップ
- ツール設定のシンボリンク
- シェル環境（`.aw_env.sh`, `.bashrc`, `.bash_profile`）の生成

### カスタム entrypoint のテンプレート

```bash
#!/bin/bash
set -e

# aw の共通セットアップ（SSH, git, UID修正 等）
. /aw-init.sh

# ここに独自のセットアップ処理を記述
# 例: npm install, Playwright ブラウザインストール 等

# 最後に aw_exec で起動（HOME/BASH_ENV を正しく設定して exec）
aw_exec "$@"
```

### aw-init.sh が提供する環境変数と関数

| 名前 | 種別 | 説明 |
|------|------|------|
| `$AW_HOME` | 変数 | コンテナユーザーのホームディレクトリ |
| `$AW_USER` | 変数 | コンテナユーザー名 |
| `$AW_WORKSPACE` | 変数 | ワークスペースパス |
| `aw_log` | 関数 | `[aw:entrypoint]` プレフィックス付きログ出力 |
| `run_as_user` | 関数 | コンテナユーザーとしてコマンド実行 |
| `aw_exec` | 関数 | 正しい環境でコマンドを exec |

## Dockerfile の最小要件

カスタム Dockerfile には以下の2行が必要です:

```dockerfile
RUN useradd -m -s /bin/bash agent && \
    echo 'ALL ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
```

- **ユーザー作成**: ホームディレクトリ付きのユーザーを作成
- **sudoers**: aw は `--user UID:0` でコンテナを実行するため、UID ベースの sudo が必要

## 例: Playwright 対応コンテナ

このリポジトリには `playwright-docker/` ディレクトリに実例があります:

```
playwright-docker/
├── Dockerfile       ← dockerfile: で指定
├── entrypoint.sh    ← COPY entrypoint.sh で参照可能
└── mise.toml        ← コンテナ内で使うツール定義
```

### Dockerfile (`playwright-docker/Dockerfile`)

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates wget openssh-client sudo \
      # Playwright browser dependencies
      libglib2.0-0 libnspr4 libnss3 ... && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash agent && \
    echo 'ALL ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
```

### entrypoint.sh (`playwright-docker/entrypoint.sh`)

```bash
#!/bin/bash
set -e
. /aw-init.sh

# Playwright ブラウザのインストール等
if command -v npx > /dev/null 2>&1; then
  npx -y playwright install chromium
fi

aw_exec "$@"
```

## リファレンス用コマンド

```bash
# デフォルトの aw-init.sh を確認
aw default-entrypoint

# デフォルトの Dockerfile を確認
aw default-dockerfile
```

## `container_user` との連携

カスタム Dockerfile で `agent` 以外のユーザーを使う場合、`container_user` でそのユーザー名を aw に伝えてください。aw はこの値を使ってマウント先パス（`.gitconfig`、`.config/gh` など）やスナップショットの実行ユーザーを決定します。

```yaml
profiles:
  custom:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.custom
    container_user: dev   # Dockerfile 内で useradd したユーザー名
```

`container_user` を指定しない場合、aw はデフォルトの `agent` ユーザーを前提とします。Dockerfile 内のユーザー名と一致しないと、マウント先や export --snapshot が正しく動作しません。

## 注意事項

- `os` と `dockerfile` は排他的です。`dockerfile` を指定すると `os` は無視されます
- ビルドコンテキストは Dockerfile が置かれたディレクトリ全体です
- `COPY` で同じディレクトリ内のファイルを参照できます
- sudoers は `ALL ALL=(ALL) NOPASSWD:ALL` を推奨します。`aw` はコンテナをホストの UID で実行するため（`--user`）、ユーザー名ベースの sudoers ではイメージを異なる UID の環境で使い回す際に sudo が動作しません
