# Development

- Run tests: `go test ./...`
- After changing Dockerfiles or OS templates: `go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration`

# Testing Strategy

## 3 layers of tests

| Layer | Command | What it covers | What it doesn't |
|-------|---------|---------------|-----------------|
| Unit | `go test ./...` | Build arg routing, hash calculation, template rendering, profile merge/validate | Real image build, entrypoint behavior |
| Integration | `go test -tags integration` | Image build, tool --version via entrypoint, runtime install | Podman rootless, mounts, SSH, config.yml resolution |
| Manual (aw -c) | `aw <profile> -c <cmd>` | Full pipeline: config → profile → build → mount → entrypoint → tool | — |

## When to run which tests

### Go code changes (toolinfo, profile, stage, containerenv, cmd, etc.)

Unit tests are sufficient:

```bash
go test ./...
```

### Dockerfile template changes (`internal/image/embed/Dockerfile.*.tmpl`)

Unit tests + integration Smoke:

```bash
go test ./...
go test -v -tags integration -timeout 10m ./internal/image/ -run TestIntegration_Smoke
```

If the change affects a specific OS, run that OS:

```bash
go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_ShellPerOS/ubi9
go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_ToolPerOS/ubi9
```

### Entrypoint changes (`internal/image/embed/entrypoint.sh.tmpl`)

Unit tests + integration E2E (entrypoint is exercised via `runContainerCommand`):

```bash
go test ./...
go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_E2E/debian12
```

### package_manager changes or new tool addition

Unit tests + full integration for the affected mode:

```bash
go test ./...
# apt mode
go test -v -tags integration -timeout 30m ./internal/image/ -run "TestIntegration_ToolPerOS|TestIntegration_E2E"
# devbox mode
go test -v -tags integration -timeout 30m ./internal/image/ -run "TestIntegration_Devbox"
```

### Mount, SSH, container socket, Podman rootless changes

These are NOT covered by go test. Manual `aw -c` testing required:

```bash
aw test-debian12-claude -c claude --version
```

### Pre-release (full matrix)

```bash
go test ./...
go test -v -tags integration -timeout 60m ./internal/image/

# Manual: representative aw -c checks
aw test-debian12-claude -c claude --version
aw test-devbox-claude -c claude --version
podman images | grep aw-container
```

## テストの書き方

テストは「関数の内部実装」ではなく「ユーザーから見た振る舞い」を検証する。

### 原則

- **振る舞いをテストする**: 「`MergeProfile` が空の override で base を返す」ではなく「プロジェクト `.aw.yml` で `launch:` だけ書いたプロファイルが `environment: container` を継承する」
- **内部関数を直接テストしない**: `hasSensitiveFields()` のような非公開ヘルパーは、公開API（`CheckProjectTrust`）経由で間接的にテストする
- **仕様をテスト名で表現する**: `TestProjectConfig_MountsRequireTrust` のように、ユーザーが期待する動作をテスト名にする
- **マージチェーン全体を通す**: 単一関数のユニットテストだけでなく、`builtinConfig → MergeConfig → ApplyDefaults → Validate` の一連の流れを通した振る舞いテストを書く

### 新機能を追加するとき

1. ユーザーとあるべき振る舞いを定義する（「`.aw.yml` に `gh_token: true` を書いたらコンテナ内で `git push` が動く」）
2. その振る舞いを検証するテストを先に書くか、実装と同時に書く
3. テストが内部実装に依存していないか確認する — リファクタリングでテストが壊れるなら実装追従型

### 避けるべきパターン

- `TestFunctionName_InternalBehavior` — 関数名をそのままテスト名にして内部実装をなぞるテスト
- 非公開関数の直接テスト（`hasSensitiveFields`, `stripSensitiveFields` 等）
- 単純なゲッターのテスト（`TestDockerStage_Name` 等）
- デフォルト値のフォールバックに頼るテスト — デフォルト値はマージチェーンで正しく埋まるべき

## Test profiles

`.aw.yml` in the project root defines 13 test profiles (4 OS × 3 tools + 1 devbox) for `aw -c` manual testing. See `aw profiles` for the full list.

# Architecture: Package Manager

The container image supports two package managers, selected via the `package_manager` profile field:

- `apt` (default) — AI tools installed via standalone installers (curl-based install scripts). Lightweight (~400 MB image).
- `devbox` (deprecated) — Nix single-user + devbox. Original behavior. Heavy (~1.8 GB image).

Templates live in `internal/image/embed/`:
- `Dockerfile.<os>.tmpl` + `entrypoint.sh.tmpl` — apt mode
- `Dockerfile.<os>.devbox.tmpl` + `entrypoint.sh.devbox.tmpl` — devbox mode

Selection happens in `embed.go` via `RenderDockerfile(os, pkgMgr, cenv)`.

# Release Rules

- Commit messages MUST follow Conventional Commits: `type(scope): description`
  - `feat:` → minor version bump
  - `fix:` → patch version bump
  - `feat!:` or `BREAKING CHANGE` footer → major version bump
  - `chore:`, `refactor:`, `docs:`, `test:`, `ci:` → no version bump
- All changes to main require a PR with merge commit (no squash, no rebase)
- CI must pass: Go tests (1.22/1.23) + commitlint
- Releases are automated: release-please creates a Release PR → merge it → GoReleaser builds binaries
