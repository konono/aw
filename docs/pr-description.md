# feat: プレビルドイメージと export --snapshot のサポート

## 概要

エアギャップ環境やコンテナ起動の高速化のために、事前ビルド済みイメージの利用と `aw export` によるイメージ書き出し機能を追加する。

## 背景

aw のコンテナはネットワーク接続を前提としており、起動のたびに devbox/mise のパッケージインストールが走る。以下のケースで問題になる:

- **エアギャップ環境**: ネットワークがないためパッケージインストールが失敗する
- **起動速度**: 毎回のインストールで数分待たされる
- **企業プロキシ環境**: 証明書やプロキシ設定をイメージに焼き込みたい

## 追加した機能

### 1. `image:` フィールド（プレビルドイメージ指定）

プロファイルに `image:` を指定すると、`docker build` をスキップしてそのイメージを直接使用する。

```yaml
profiles:
  airgap:
    environment: container
    launch: claude
    image: 'aw-container:abc123'
    skip_devbox_install: true
    skip_mise_install: true
```

- `image:` がある場合、`os:` と `dockerfile:` は無視される（排他エラーにはしない）
- イメージがローカルに存在しない場合はエラー

### 2. `skip_devbox_install` / `skip_mise_install`

コンテナ起動時の entrypoint.sh で実行される devbox/mise のインストールをスキップする。プレビルドイメージと組み合わせて使う。

### 3. `aw export <profile>` コマンド

プロファイルのコンテナイメージをビルドし、tar ファイルに書き出す。

```bash
aw export claude -o my-image.tar
```

### 4. `--snapshot` フラグ

一時コンテナを起動し、ワークスペースの devbox.json / mise.toml からパッケージ・ツールをインストールした状態を `docker commit` でイメージに焼き込む。

```bash
aw export claude --snapshot -o my-image.tar
```

#### snapshot の内部動作

1. ビルド済みイメージから一時コンテナを起動
2. ワークスペースを `/workspace` に **read-only** マウント
3. ro マウントのため、コンテナ内の `/tmp/aw-snapshot-work` にコピーしてからインストール実行
   - `devbox install` は `.devbox/gen/` をカレントディレクトリに書き込むため ro 上では失敗する
   - `mise install` も同様に中間生成物を書き込む場合がある
4. パッケージは `/nix/store/` と `/home/agent/.local/share/` にインストールされる
5. mise の場合、shim がバージョンを解決できるように `mise.toml` をグローバル設定（`/home/agent/.config/mise/config.toml`）にコピー
6. `docker commit` でコンテナの状態をイメージに保存
7. 一時コンテナを削除

#### env ファイルと entrypoint の関係

snapshot は `.aw_env.sh`, `.bashrc`, `.bash_profile` をイメージに焼き込むが、**通常起動時に entrypoint.sh が同じファイルを毎回再生成して上書きする**。焼き込まれた env ファイルは使われない。

実際にツールが使えるのは以下の仕組みによる:
- devbox パッケージ: `/nix/store/` に焼き込み済み → `devbox global shellenv` が PATH を設定
- mise ツール: `/home/agent/.local/share/mise/installs/` に焼き込み済み → shims が PATH に追加される
- `skip_devbox_install` / `skip_mise_install` でインストールをスキップしても、PATH 設定は entrypoint.sh が行うので焼き込み済みツールが使える

### 5. `--include src:dst` フラグ

ホストのディレクトリをイメージ内にコピーする。`--snapshot` を暗黙的に有効化する。

```bash
aw export claude --include ./certs:/usr/local/share/ca-certificates
```

ro マウントから `cp -a` で読み取りコピーするため、ホスト側に書き込みは発生しない。コピー後のオーナーは `agent:agent`、パーミッションはソースのまま。

### 6. `--env KEY=VAL` フラグ

環境変数を `docker commit --change 'ENV KEY=VAL'` でイメージメタデータに焼き込む。`--snapshot` を暗黙的に有効化する。

```bash
aw export claude --env HTTP_PROXY=http://proxy.corp:8080
```

entrypoint.sh とは独立してイメージレベルで設定されるため、上書きされない。

### 7. `--apply` フラグ

export 完了後、呼び出し元の設定ファイルに `image:` フィールドを自動書き戻しする。

```bash
aw export dev --apply -o my-image.tar
```

- 設定ファイルを `yaml.Node` で操作し、コメントを保持したまま更新する
- `snapshot` 使用時は `skip_devbox_install: true` / `skip_mise_install: true` も追加する
- `os:` や `dockerfile:` は削除しない（共存を許容）
- config ファイルが存在しない場合（builtin のみ）はエラー

### 8. `export:` 設定セクション

CLI フラグの代わりにプロファイルの設定で指定できる。CLI フラグと設定はマージされ、CLI が優先。

```yaml
profiles:
  airgap:
    export:
      snapshot: true
      include:
        - src: ./certs
          dst: /usr/local/share/ca-certificates
      env:
        HTTP_PROXY: http://proxy.corp:8080
```

### 9. `docker build --load` の追加

buildx の `docker-container` ドライバ使用時、`--load` なしだとイメージがビルドキャッシュにしか残らず `docker save` が失敗する問題を修正。

## 変更ファイル一覧

### 新規
- `internal/cmd/export.go` — export コマンド本体、snapshot スクリプト、`--apply` の YAML 書き戻し
- `internal/cmd/export_test.go` — 引数パース、マージ、apply のテスト
- `docs/export-snapshot.md` — snapshot 内部動作の詳細ドキュメント
- `devbox.json` / `devbox.lock` — aw 自身の開発用 devbox 設定
- `.agent-workspace.yml` — aw 自身の開発用プロファイル

### 変更
- `internal/profile/types.go` — `Image`, `SkipDevboxInstall`, `SkipMiseInstall`, `ExportConfig` フィールド追加
- `internal/profile/validate.go` — export バリデーション追加、image + os/dockerfile の排他ルール緩和
- `internal/profile/merge.go` — export 設定のマージロジック追加
- `internal/docker/client.go` — `--load` 追加、`ImageExists`, `Save`, `RunOneShot`, `Commit`, `RemoveContainer` メソッド追加
- `internal/stage/container.go` — `image:` 指定時のビルドスキップ
- `internal/image/embed/entrypoint.sh` — `skip_devbox_install` / `skip_mise_install` 対応
- `internal/profile/embed/config.yml` — テンプレートに export セクション追加
- `internal/cmd/root.go` — export サブコマンド登録、ヘルプテキスト更新
- `README.md` — export の使用例、ドキュメントリンク追加
- `docs/configuration.md` — export フィールドのリファレンス追加

### 削除
- `internal/profile/migrate.go` — 不要になった config マイグレーション
- `internal/profile/migrate_test.go` / `migrate_demo_test.go` — 同上

## テスト

### ユニットテスト

```bash
go test ./...
```

- 引数パース（`--snapshot`, `--include`, `--env`, `--apply` の各パターン）
- export 設定のマージ（CLI + config、CLI 優先、暗黙 snapshot）
- `applyExportResult` の YAML 書き戻し（追加、更新、コメント保持、エラーケース）
- image + os 共存のバリデーション
- skip_devbox_install / skip_mise_install のバリデーション

### 手動テスト

1. `aw export shell -o /tmp/basic.tar` → 基本 export
2. `aw export shell --snapshot` → devbox/mise パッケージ焼き込み
3. `aw export shell --include src:dst` → ファイルコピー
4. `aw export shell --env KEY=VAL` → 環境変数焼き込み
5. `aw export dev --apply` → config への書き戻し
6. config の `export:` セクションからの読み込み
7. エラーケース（存在しないプロファイル、不正なフラグ形式、存在しない include パス）
