# v4 → v5 移行ガイド

v5 は `aw` から使われなくなった機能を取り除くリリースです。コンテナを起動して
AI ツールを動かすという中心の機能は変わりません。以下に該当する設定・運用があ
る場合のみ対応が必要です。

## 削除された機能

| 削除されたもの | 代替 |
|---|---|
| `aw team start` / `stop` / `status` / `scope` | なし（下記「チーム機能について」参照） |
| `aw msg send` / `inbox` / `history` / `watch` / `clear` | なし（同上） |
| `teams:` 設定キーと `delivery:` プロファイルフィールド | なし |
| `package_manager: devbox`（Nix + devbox モード） | `mise.toml`、プロファイルの `packages`、[カスタム Dockerfile](custom-dockerfile.md) |
| `devbox_install` / `skip_devbox_install` | 同上 |
| 組み込みテンプレートのイメージでの `devbox.json` 自動インストール | `mise.toml`（[パッケージ管理](mise.md)） |
| `aw export`（`aw build` の非推奨エイリアス） | `aw build` |

## 設定ファイルの対応

### `package_manager` はエラーになります

`package_manager: devbox`（および `apt` 以外の任意の値）はバリデーションエラー
になります。イメージの中身が黙って変わるのを避けるため、キー自体は互換用に残し
て拒否しています。

```
profile "dev": package_manager "devbox" is no longer supported: the devbox
package manager was removed in v5. ...
```

`package_manager` キーを削除し、devbox で入れていたパッケージは `mise.toml`、
プロファイルの `packages`、またはカスタム Dockerfile に移してください。移行後は
apt モードのイメージ（約 400 MB、Nix なし）でビルドされます。

### 黙って無視されるキー

以下は設定に残っていてもエラーになりません。`aw` の設定読み込みは未知のキーを
無視するためです。動作には影響しませんが、削除を推奨します。

- `teams:` ブロック全体
- プロファイルの `delivery:`
- プロファイルの `devbox_install:` / `skip_devbox_install:`

`aw build --apply` は、設定に残った `skip_devbox_install` を次回実行時に自動で
削除します。

## 組み込みテンプレートのイメージは再ビルドされます

`package_manager` はイメージタグのハッシュ入力に含まれており、そのハッシュは
組み込みテンプレートからビルドする場合にのみ計算されます。そのため **`os:` を
使う（= 組み込みテンプレートでビルドする）プロファイルはタグが変わり、v5 で最初
に起動したときにキャッシュが効かず再ビルドが走ります**。公式プレビルドイメージを
使っている場合は再 pull です。

`dockerfile:` でカスタム Dockerfile を使うプロファイルのタグは、この変更では
変わりません。

## カスタム Dockerfile は影響を受けません

ここまでの devbox の話は、すべて `aw` の組み込みテンプレートと snapshot 経路に
限った話です。`dockerfile:` で指定するカスタム Dockerfile の中身に `aw` は関与
しません。Nix と devbox を自分でインストールして、自前の entrypoint で
`devbox.json` を読むことは v5 でも問題なくできます。

このリポジトリの `playwright-docker/` がまさにその例で、Dockerfile 内で Nix と
devbox を入れ、entrypoint がワークスペースの `devbox.json`（なければ
`mise.toml`）を処理します。v5 でもそのまま動作します。

## 残骸の手動削除

`aw` 自体はこれらを掃除しません。必要に応じて手動で削除してください。

### 旧 team コンテナ

```bash
podman ps -a --filter 'name=^aw-' --format '{{.Names}}'
podman rm -f <旧 team コンテナ名>
```

旧 team コンテナは `aw-<team>-<agent>-<数字>` という名前で、`aw save` の一覧に
は表示されます。選択しても保存はできず、削除方法を案内するエラーで停止します。

### messaging データベースと team state

```bash
rm -rf ~/.config/aw/teams
find ~ -name 'messages.db' -path '*aw*'
```

### 旧 team worktree

各メンバーは `<repo>/worktrees/aw-<team>-<agent>` に worktree を作り、
`aw/<team>/<agent>` ブランチを使っていました。

```bash
git worktree list
git worktree remove <path>
git branch -D aw/<team>/<agent>
```

## チーム機能について

v5 で削除した team / messaging は「**代替がある**」のではなく「**なくなった**」
機能です。混同を避けるため、ここを明確にしておきます。

[panecom](https://github.com/konono/panecom) と `mount_zellij` を使うと、zellij
のペインに名前を付けてペイン間でメッセージを送れます。team 機能を使わなくなっ
たのはこれが理由ですが、**同等の置き換えではありません**。panecom にないもの:

- ロールプロンプト（developer / reviewer / lead / partner）の自動注入
- 未読をポーリングしてバックグラウンドのエージェントを自動的に動かす agent-loop
- メンバーごとの git worktree とブランチの自動作成
- メッセージの永続化と履歴

`panecom send <role> <message>` は、こちらが送ると決めたときにペインへ送る手動
の操作です。自律的に動くマルチエージェントの協調が必要な場合は、v4 系を使い続
けるか、外部のオーケストレーターを検討してください。

Slack / Discord からエージェントを操作する用途は
[aw-manager](https://github.com/konono/aw-manager) でカバーされています。
こちらは `aw manifest` が生成する Kubernetes マニフェストを使う構成で、v5 でも
そのまま動作します（`aw manifest`、`environment: container` + `kubernetes:`
ブロック、リソース命名とラベルはいずれも変更していません）。
