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
      libglib2.0-0 libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
      libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 \
      libxfixes3 libxrandr2 libgbm1 libxcb1 libpango-1.0-0 \
      libcairo2 libasound2 libcups2 libdrm2 libxshmfence1 \
      libxkbcommon0 libatspi2.0-0 && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash agent && \
    echo 'agent ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

RUN su -s /bin/bash agent -c 'curl https://mise.jdx.dev/install.sh | sh'

ENV PATH="/home/agent/.local/bin:/home/agent/.local/share/mise/shims:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
```

entrypoint.sh で Playwright ブラウザの自動インストールも行われます。詳細は `playwright-docker/entrypoint.sh` を参照してください。

## ベースにする Dockerfile

デフォルトの Dockerfile と entrypoint.sh は `aw default-dockerfile` で確認できます。これをベースにカスタマイズするのが最も簡単です。

## 注意事項

- `os` と `dockerfile` は排他的です。`dockerfile` を指定すると `os` は無視されます
- ビルドコンテキストは Dockerfile が置かれたディレクトリ全体です
- `COPY` で同じディレクトリ内のファイルを参照できます
