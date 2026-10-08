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

> **Note:** `package_manager: devbox`（Nix + devbox モード）は廃止されました。設定に残っていると
> バリデーションエラーになります。組み込みテンプレートのイメージでは `devbox.json` は読まれません。
> 同等の構成は `mise.toml`、プロファイルの `packages`、または[カスタム Dockerfile](custom-dockerfile.md)
> で表現してください（カスタム Dockerfile なら自前で devbox を入れることもできます）。
> 詳細は [v4 → v5 移行ガイド](migration-v5.md)。

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

ワークスペースルートに `mise.toml`（または `.mise.toml`）を配置すると、コンテナ起動時にエントリポイントが自動的にツールをインストールします。構成が固まったら `aw build` でイメージに焼き込むと、起動時のインストールをスキップできます。

> **Note:** `~/.config/aw/mise.toml` によるグローバル設定は廃止されました。ワークスペースの mise.toml またはプロファイルの `packages` フィールドを使用してください。

## キャッシュ

mise でインストールされたツールはコンテナ内に保存されるため、コンテナ破棄時に消えます。起動のたびに再インストールが実行されます。

構成が固まったら `aw build` でインストール済みの状態をイメージに焼き込むことで、起動時のインストールをスキップできます:

```bash
aw build claude
```

`aw build` は焼き込んだ mise 設定の指紋をイメージに記録します。起動時に設定が
変わっていなければ `mise install` を呼ばずに起動し、`mise.toml` を編集した起動では
通常どおりインストールが走ります。設定を変えても常にスキップしたい場合は
`mise_install: false` を明示してください。判定の詳細と指紋の対象範囲は
[Build & Snapshot](build-snapshot.md#mise-install-のスキップ判定) を参照してください。

> **ツールを削除した場合**: `mise.toml` からツールを消しても、イメージに焼き込み済みの
> バイナリは残ります。イメージから取り除くには `aw build` で作り直してください。
