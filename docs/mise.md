# コンテナ内のパッケージ管理

## 考え方

コンテナ上の AI エージェントは試行錯誤の中で `npm install`、`pip install`、`apt-get install` など様々なソフトウェアをインストールして問題を解決しようとします。使い捨てコンテナなのでそれ自体は問題ありませんが、**正解がわかったらその状態を再現可能にしたい**はずです。

`aw` のコンテナは [mise](https://mise.jdx.dev/) に対応しています。エージェントが試行錯誤で見つけた正解を `mise.toml` に落とせば、次回以降はコンテナ起動時に自動でインストールされ、チームメンバーの環境でもそのまま再現できます。

```
試行錯誤（コンテナ内で自由にインストール）
  ↓ 正解がわかったら
mise.toml にコミット
  ↓ 次回以降
コンテナ起動時に自動インストール → 誰でも同じ環境
```

## 自動インストールの条件

ワークスペースルートに `mise.toml` または `.mise.toml` があると、コンテナ起動時に `mise install` が実行されます。

> **Note:** `package_manager: devbox`（Nix + devbox モード）は廃止されました。`devbox.json` は読まれません。同等の構成は `mise.toml`、プロファイルの `packages`、または[カスタム Dockerfile](custom-dockerfile.md) で表現してください。

## mise を使う場合

### mise.toml の例

```toml
[tools]
node = "22"      # Claude Code に必要
python = "3.14"
go = "1.25"
gh = "latest"
```

プロジェクトに合わせて不要なツールを削除してください:

```bash
# Python プロジェクト — node（Claude Code 用）と python のみ
cat > mise.toml << 'EOF'
[tools]
node = "22"
python = "3.14"
EOF
```

### install タスク

`mise.toml` に `install` タスクが定義されていれば、ツールインストール後に自動実行されます:

```toml
[tools]
node = "22"

[tasks.install]
run = "npm ci"
```

### ワークスペース mise.toml

ワークスペースルートに `mise.toml`（または `.mise.toml`）を配置すると、コンテナ起動時にエントリポイントが自動的にツールをインストールします。構成が固まったら `aw build --apply` でイメージに焼き込むと、起動時のインストールをスキップできます。

> **Note:** `~/.config/aw/mise.toml` によるグローバル設定は廃止されました。ワークスペースの mise.toml またはプロファイルの `packages` フィールドを使用してください。

## キャッシュ

mise でインストールされたツールはコンテナ内に保存されるため、コンテナ破棄時に消えます。起動のたびに再インストールが実行されます。

構成が固まったら `aw build --apply` でインストール済みの状態をイメージに焼き込むことで、起動時のインストールをスキップできます:

```bash
aw build claude --apply
```
