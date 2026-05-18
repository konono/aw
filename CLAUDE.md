# Development

- Run tests: `go test ./...`
- After changing Dockerfiles or OS templates: `go test -tags integration -timeout 30m ./internal/image/ -run TestIntegration`
