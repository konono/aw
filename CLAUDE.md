# Development

- Run tests: `go test ./...`
- After changing Dockerfiles or OS templates: `go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration`
- Before pushing, always run `golangci-lint run ./...` — CI runs staticcheck (SA5011 etc.) and will fail on lint errors
- Common lint pitfall: `if x == nil { t.Fatal(...) }` then `x.Field` → staticcheck SA5011. Add `return` after `t.Fatal` in tests

# Testing Strategy

## 3 layers of tests

| Layer | Command | What it covers | What it doesn't |
|-------|---------|---------------|-----------------|
| Unit | `go test ./...` | Build arg routing, hash calculation, template rendering, profile merge/validate | Real image build, entrypoint behavior |
| Integration | `go test -tags integration` | Image build, tool --version via entrypoint, runtime install | Podman rootless, mounts, SSH, config.yml resolution |
| Manual (aw --) | `aw <profile> -- <cmd>` | Full pipeline: config → profile → build → mount → entrypoint → tool | — |

## When to run which tests

### Go code changes (toolinfo, profile, stage, containerenv, cmd, etc.)

Unit tests are sufficient:

```bash
go test ./...
```

### Kubernetes manifest changes (`internal/manifest/`)

Unit tests are sufficient (golden file comparison, no K8s cluster needed):

```bash
go test ./internal/manifest/...
```

For OpenShift validation (optional):

```bash
aw manifest <profile> --name test | oc apply --dry-run=client -f -
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

### Container launch command changes (`internal/launcher/tool.go`)

Unit tests + flag validation integration:

```bash
go test ./...
go test -v -tags integration -timeout 10m ./internal/image/ -run TestIntegration_ContainerLaunchFlags
```

### Mount, SSH, container socket, Podman rootless changes

These are NOT covered by go test. Manual `aw --` testing required:

```bash
aw test-debian12-claude -- claude --version
```

### Pre-release (full matrix)

```bash
go test ./...
go test -v -tags integration -timeout 60m ./internal/image/

# Manual: representative aw -- checks
aw test-debian12-claude -- claude --version
aw test-devbox-claude -- claude --version
podman images | grep aw-container
```

## Test profiles

`.aw.yml` in the project root defines 13 test profiles (4 OS × 3 tools + 1 devbox) for `aw -- <cmd>` manual testing. See `aw profiles` for the full list.

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
- All changes to main require a PR with squash merge (PR title = Conventional Commit message)
- CI must pass: Go tests (1.25/1.26) + commitlint
- Releases are automated: release-please creates a Release PR → merge it → GoReleaser builds binaries
