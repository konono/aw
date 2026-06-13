# Development

- Unit tests: `go test ./...`
- Integration tests: `go test -v -tags integration -timeout 30m ./internal/image/`
- Manual test: `aw <profile> -c <cmd>` (mounts, SSH, container socket)

# Testing

## Test layers

| Layer | When to run | What it covers |
|-------|-------------|----------------|
| Unit (`go test ./...`) | All Go changes | Config merge, validation, build args, template rendering |
| Integration (`-tags integration`) | Dockerfile, entrypoint, tool changes | Image build, entrypoint, tool --version |
| Manual (`aw -c`) | Mount, SSH, socket changes | Full pipeline end-to-end |

## Writing tests

Tests MUST verify **user-visible behavior**, not internal implementation.

**Unit tests**: Test outcomes through public APIs. Name tests after the behavior: `TestProjectConfig_MountsRequireTrust`, not `TestHasSensitiveFields`. Test the full merge chain (`builtin → user → project → ApplyDefaults → Validate`), not isolated functions.

**Integration tests**: Reproduce `aw <profile> -c <cmd>` flow. Verify tools work (`claude --version`), not that internal functions return expected values.

**Do NOT**: Test non-exported helpers directly. Test simple getters. Write tests that break on refactoring.

# Architecture

- `apt` (default): standalone installers (curl). `devbox` (deprecated): Nix.
- Templates: `internal/image/embed/Dockerfile.<os>.tmpl` + `entrypoint.sh.tmpl`
- Selection: `embed.go` → `RenderDockerfile(os, pkgMgr, cenv)`

# Release

- Conventional Commits required (`feat:` → minor, `fix:` → patch, `feat!:` → major)
- PRs with merge commit (no squash/rebase), CI must pass
- Automated: release-please → GoReleaser
