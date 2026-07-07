# DooD (Docker outside of Docker) ガイド

`mount_container_sock` を使って、aw コンテナ内から docker-compose 等でコンテナを操作する方法を説明します。

## 仕組み

DooD (Docker outside of Docker) は、ホスト（または Podman VM）のコンテナランタイムソケットをコンテナ内にマウントする方式です。コンテナ内から作成されたコンテナは、aw コンテナの「子」ではなく「兄弟」として動作します。

```
aw container ──mount──> host/VM container socket
                          |
                     docker-compose up
                          |
                  +-------+-------+
                  |               |
              app-container   db-container
              (sibling)       (sibling)
```

## セットアップ

### 1. devbox.json を配置

ワークスペースルートに `devbox.json` を配置してコンテナ内で使うツールを定義します。エントリポイントで自動的にインストールされます。

```json
{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/main/.schema/devbox.schema.json",
  "packages": [
    "docker-compose@latest",
    "docker-client@latest"
  ]
}
```

- `docker-compose` — docker-compose コマンド
- `docker-client` — docker CLI（クライアントのみ、daemon なし）

### 2. aw の設定

`~/.config/aw/config.yml` にプロファイルを追加します。

```yaml
profiles:
  claude-compose:
    environment: container
    launch: claude
    container_runtime: podman     # or docker
    mount_container_sock: true
    ssh_agent_forwarding: true    # Git SSH 操作が必要な場合
```

### 3. 実行

```bash
aw claude-compose
```

コンテナ内に以下が自動設定されます:
- `DOCKER_HOST=unix:///run/container.sock`
- `CONTAINER_HOST=unix:///run/container.sock`（mise 経由の podman-remote 用）
- ランタイムソケットが `/run/container.sock` にマウント

## コンテナ内での使い方

```bash
# docker-compose で複数コンテナを起動
docker-compose up -d

# docker CLI でコンテナ一覧
docker ps

# ソケットの疎通確認
curl --unix-socket /run/container.sock http://localhost/_ping
```

## プラットフォームごとの動作

| ランタイム | OS | ソケットパス | 備考 |
|-----------|----|----|------|
| Docker | Linux | `/var/run/docker.sock` | 直接マウント |
| Docker | macOS | `/var/run/docker.sock` | Docker Desktop が透過的に処理 |
| Podman | Linux | 自動検出 | `podman info` で rootless/rootful を判定 |
| Podman | macOS | `/run/podman/podman.sock` | Podman VM 内のソケットを直接マウント |

## プロジェクトごとの設定

プロジェクトの `.aw.yml` で docker-compose を使うプロファイルを定義することもできます:

```yaml
profiles:
  dev:
    environment: container
    launch: claude
    mount_container_sock: true
    mounts:
      - source: ~/.config/gcloud
        target: /home/agent/.config/gcloud
```

## セキュリティに関する注意

`mount_container_sock: true` を有効にすると、コンテナ内の AI エージェントがホスト（または Podman VM）のコンテナランタイムにフルアクセスできるようになります。具体的には:

- コンテナの作成・起動・停止・削除
- イメージのビルド・プル・プッシュ
- ボリュームやネットワークの操作

有効化時に Warning ログが出力されます。信頼できるプロジェクトでのみ使用してください。
