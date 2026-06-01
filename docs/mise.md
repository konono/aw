# コンテナ内のパッケージ管理

## 考え方

コンテナ上の AI エージェントは試行錯誤の中で `npm install`、`pip install`、`apt-get install` など様々なソフトウェアをインストールして問題を解決しようとします。使い捨てコンテナなのでそれ自体は問題ありませんが、**正解がわかったらその状態を再現可能にしたい**はずです。

`aw` のコンテナは [mise](https://mise.jdx.dev/) と [devbox](https://www.jetify.com/devbox) の両方に対応しています。エージェントが試行錯誤で見つけた正解を `mise.toml` や `devbox.json` に落とせば、次回以降はコンテナ起動時に自動でインストールされ、チームメンバーの環境でもそのまま再現できます。

```
試行錯誤（コンテナ内で自由にインストール）
  ↓ 正解がわかったら
mise.toml / devbox.json にコミット
  ↓ 次回以降
コンテナ起動時に自動インストール → 誰でも同じ環境
```

## mise と devbox の使い分け

`aw` はプロジェクトの設定ファイルを見て自動的に判断します:

| ファイル | 動作 |
|---------|------|
| `devbox.json` がある | `devbox install` を実行 |
| `mise.toml` / `.mise.toml` がある | `mise install` を実行 |
| どちらもない | devbox がアドホックに利用可能（`devbox global add` で随時追加できる） |

`devbox.json` と `mise.toml` の両方がある場合は、両方とも実行されます。

## mise を使う場合

### mise.toml の例

```toml
[tools]
node = "22"      # Claude Code に必要
python = "3.14"
go = "1.23"
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

## devbox を使う場合

### devbox.json の例

```json
{
  "packages": [
    "nodejs@22",
    "python@3",
    "go@1.23",
    "gh"
  ]
}
```

### install スクリプト

`devbox.json` に `install` スクリプトが定義されていれば、パッケージインストール後に自動実行されます。

### グローバル mise 設定

`~/.config/aw/mise.toml` を作成すると、全コンテナイメージのビルド時にこの設定が組み込まれます。全プロジェクト共通で使うツール（言語ランタイム、CLI ツールなど）を定義する場所です。

```toml
[tools]
python = "3.14"
go = "1.23"
gh = "latest"
```

プロジェクトに `mise.toml` がある場合、グローバル設定と両方が適用されます。同じツールが定義されている場合はプロジェクト側が優先されます。

`aw init` を実行するとテンプレートファイルが `~/.config/aw/mise.toml` に生成されます。

### グローバル devbox 設定

`~/.config/aw/devbox.json` を作成すると、全コンテナイメージのビルド時にこの設定が組み込まれます。全プロジェクト共通で使うツールを定義する場所です。

`aw init` を実行するとテンプレートファイルが `~/.config/aw/devbox.json` に生成されます。

### グローバル設定の組み合わせ

`~/.config/aw/mise.toml` と `~/.config/aw/devbox.json` は同時に使用できます。mise は言語バージョン管理（node, python, go）、devbox は Nix パッケージ（より幅広いツール群）と使い分けるのが自然です。

## キャッシュ

mise / devbox でインストールされたツールは Docker/Podman の永続ボリューム `claude-code-local` にキャッシュされます。初回起動時のみインストールが実行され、2回目以降はキャッシュから読み込まれるため即座に起動します。

キャッシュをクリアするにはボリュームを削除してください:

```bash
docker volume rm claude-code-local   # Docker の場合
podman volume rm claude-code-local   # Podman の場合
```
