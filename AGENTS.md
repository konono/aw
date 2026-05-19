# Development

- Run tests: `go test ./...`
- After changing Dockerfiles or OS templates: `go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration`

# Release Rules

- Commit messages MUST follow Conventional Commits: `type(scope): description`
  - `feat:` → minor version bump
  - `fix:` → patch version bump
  - `feat!:` or `BREAKING CHANGE` footer → major version bump
  - `chore:`, `refactor:`, `docs:`, `test:`, `ci:` → no version bump
- All changes to main require a PR with merge commit (no squash, no rebase)
- CI must pass: Go tests (1.22/1.23) + commitlint
- Releases are automated: release-please creates a Release PR → merge it → GoReleaser builds binaries
