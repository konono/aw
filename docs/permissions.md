# Container Permissions (UID/GID)

aw のコンテナ権限戦略と、bind mount 上のファイル所有者の仕組みを説明します。

## 基本設計

コンテナは以下のフラグで起動されます:

- **Docker**: `--user <host-uid>:<host-gid> --group-add 0`
- **Podman**: `--userns keep-id --user <host-uid>:<host-gid> --group-add 0`

これにより:

| 対象 | 権限の仕組み |
|------|-------------|
| bind mount 上のファイル | host UID:GID で自然に読み書き |
| イメージ内の writable パス | supplementary group 0 で書き込み |
| 新規作成ファイルの group | host GID（root ではない） |

## なぜ `--group-add 0` が必要か

イメージ内のディレクトリ (`/home/agent`, `/workspace` 等) は、ビルド時に `chgrp -R 0` + `chmod -R g=u` で root グループに書き込み権限が設定されています（OpenShift GID 0 パターン）。

`--group-add 0` により、コンテナプロセスは supplementary group として GID 0 を持ち、これらのパスに書き込めます。primary GID はホストユーザーの GID が使われるため、bind mount 上の新規ファイルは host GID で作成されます。

## /etc/passwd 動的注入

コンテナは固定の UID 1001 でビルドされますが、実行時にはホストの UID で起動されます。`aw-init.sh` が起動時に `/etc/passwd` を書き換え、ランタイム UID に対応するエントリを注入します:

```
agent:x:<runtime-uid>:<runtime-gid>:agent:/home/agent:/bin/bash
```

## トラブルシューティング

### Permission denied でファイルが作れない

**原因**: SELinux が有効な環境で、bind mount に `:z` ラベルがない

aw は自動的に `:z` を付与しますが、カスタムマウントの場合は `options: "z"` を設定してください:

```yaml
mounts:
  - source: ~/data
    target: /data
    mode: rw
    options: "z"
```

### ファイルの group が root になる

**原因**: 旧バージョンの aw を使っている可能性

v3.4.2 以降は `--user UID:GID --group-add 0` を使用し、新規ファイルの group はホスト GID になります。`aw` をアップデートしてください。

### Podman で高番号の GID が表示される

**原因**: `--userns keep-id` なしで GID 0 を使っている

v3.4.2 以降では `--userns keep-id --user UID:GID --group-add 0` が使われ、この問題は解消されています。

### `aw doctor` での確認

```bash
aw doctor -v
```

Official Images セクションで公式イメージのキャッシュ状態を確認できます。
