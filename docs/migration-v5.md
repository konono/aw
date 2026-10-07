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
| ワークスペースの `devbox.json` の自動インストール | `mise.toml`（[パッケージ管理](mise.md)） |
| `aw export`（`aw build` の非推奨エイリアス） | `aw build` |

## 設定ファイルの対応

`aw` の設定読み込みは未知のキーを無視します。削除されたキーが `.aw.yml` や
`~/.config/aw/config.yml` に残っていても**エラーにはなりませんが、黙って無視
されます**。意図しない挙動を避けるため、以下のキーは削除してください。

- `teams:` ブロック全体
- プロファイルの `delivery:`
- プロファイルの `package_manager:`
- プロファイルの `devbox_install:` / `skip_devbox_install:`

`package_manager: devbox` を使っていたプロファイルは、何もしなければ apt モー
ドのイメージ（約 400 MB、Nix なし）でビルドされます。devbox で入れていたパッ
ケージは `mise.toml` かカスタム Dockerfile に移してください。

`aw build --apply` は、設定に残った `skip_devbox_install` を次回実行時に自動で
削除します。

## イメージの再ビルドが発生します

イメージタグのハッシュ入力から `package_manager` が外れたため、**すべてのプロ
ファイルでイメージタグが変わります**。v5 で最初に起動したときは、キャッシュが
効かず再ビルドが走ります。公式プレビルドイメージを使っている場合は再 pull です。

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
