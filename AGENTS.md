# AGENTS.md

## Development Commands

### Build

- `make build` - Build both frps and frpc binaries
- `make frps` - Build server binary only
- `make frpc` - Build client binary only
- `make all` - Build everything with formatting

### Testing

- `make test` - Run unit tests
- `make e2e` - Run end-to-end tests
- `make e2e-trace` - Run e2e tests with trace logging
- `make alltest` - Run all tests including vet, unit tests, and e2e

### Code Quality

- `make fmt` - Run go fmt
- `make fmt-more` - Run gofumpt for more strict formatting
- `make gci` - Run gci import organizer
- `make vet` - Run go vet
- `golangci-lint run` - Run comprehensive linting (configured in .golangci.yml)

### Assets

- `make web` - Build web dashboards (frps and frpc)
- `make docker-build` - Build both Docker images; frontend assets are built inside Docker

### Cleanup

- `make clean` - Remove built binaries and temporary files

## Testing

- E2E tests using Ginkgo/Gomega framework
- Mock servers in `/test/e2e/mock/`
- Run: `make e2e` or `make alltest`
- E2E management tests use session authentication with temporary admin data directories

## Configuration

- TOML, YAML, and JSON are supported.
- Legacy INI configuration is not supported.
- Use `webServer.sessionAuth` and `FRP_ADMIN_USERNAME`/`FRP_ADMIN_PASSWORD`.
