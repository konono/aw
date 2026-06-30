# Official Prebuilt Images

aw は公式のプレビルドコンテナイメージを GHCR (GitHub Container Registry) で配布しています。プレビルドイメージを使うと、初回起動時のビルド時間（数分〜10分）が不要になり、pull のみ（数十秒）で環境が使えます。

## 利用方法

デフォルトでは、aw は自動的に公式イメージの利用を試みます。プロファイルに `image:` や `dockerfile:` が設定されておらず、`extra_packages` 等のビルドカスタマイズもない場合、公式イメージが pull されます。

```bash
# 特別な設定なしで使える
aw claude
```

公式イメージが利用できない場合（未公開、ネットワークエラーなど）は、従来通りテンプレートからのローカルビルドにフォールバックします。

## イメージ命名規則

ツール別 × OS 別にイメージが用意されています:

```
ghcr.io/konono/aw-<tool>:<version>-<os>

# 例:
ghcr.io/konono/aw-claude:3.5.0-debian12
ghcr.io/konono/aw-claude:3.5.0-ubi9
ghcr.io/konono/aw-codex:3.5.0-ubuntu2604
ghcr.io/konono/aw-cursor:3.5.0-ubi10
```

ツール: `claude`, `codex`, `opencode`, `cursor`
OS: `debian12`（デフォルト）, `ubuntu2604`, `ubi9`, `ubi10`

### タグ体系

| タグ | 説明 |
|------|------|
| `<version>-debian12` | バージョン + OS 固定 (e.g. `3.5.0-debian12`) |
| `<version>` | バージョン固定、デフォルト OS |
| `debian12` | 最新版 + OS 固定 |
| `latest` | 最新版 + デフォルト OS |
| `<version>-debian12-amd64` | arch 固定 |
| `<version>-debian12-arm64` | arch 固定 |

Multi-arch manifest list により、`docker pull` / `podman pull` 時にホストのアーキテクチャが自動選択されます。

## image_pull_policy

プロファイルで公式イメージの動作を制御できます:

```yaml
profiles:
  claude:
    launch: claude
    image_pull_policy: auto  # デフォルト
```

| 値 | 動作 |
|----|------|
| `auto` | ローカルにあれば使う、なければ pull、失敗したらビルド |
| `always` | 毎回 pull（CI やセキュリティパッチ適用確認向け） |
| `never` | pull しない、ローカルのみ |
| `build` | 公式イメージを使わず、常にテンプレートからビルド |

## 公式イメージが使われない場合

以下のいずれかに該当すると、公式イメージをスキップしてテンプレートからビルドします:

- `image:` が設定されている（カスタムイメージ優先）
- `dockerfile:` が設定されている（カスタム Dockerfile 優先）
- `packages:` でパッケージが追加されている
- `build_env:` でビルド引数が設定されている
- `ca_cert:` で CA 証明書が設定されている
- `package_manager: devbox` が設定されている
- `container_user:` が `agent` 以外に設定されている
- ワークスペースの `packages.txt` にパッケージがある
- `image_pull_policy: build` が設定されている

> **Note:** `gh_token: true` はカスタムビルドのトリガーにはなりません。gh CLI は公式イメージを含む全イメージにプリインストールされています。`gh_token` は実行時の `GITHUB_TOKEN` 環境変数注入のみを制御します。

## カスタマイズが必要な場合

追加パッケージや CA 証明書が必要な場合は、テンプレートからのビルドが自動的に選択されます:

```yaml
profiles:
  claude-custom:
    launch: claude
    packages:
      - jq
      - tree
    # → 公式イメージは使われず、テンプレートからビルド
```

または `image_pull_policy: build` で明示的にビルドを強制できます:

```yaml
profiles:
  claude-dev:
    launch: claude
    image_pull_policy: build
```
