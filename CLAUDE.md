# Development

- Run tests: `go test ./...`
- After changing Dockerfiles or OS templates: `go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration`
