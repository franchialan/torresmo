# Contributing to Torresmo

Thank you for your interest in contributing to Torresmo! 🐷

## Getting Started

### Prerequisites

- Go 1.22+ (check with `go version`)
- Docker or Podman running
- [act](https://github.com/nektos/act) (optional, for workflow testing)
- [golangci-lint](https://golangci-lint.run/) (optional, for linting)

### Setup

```bash
# Clone the repository
git clone https://github.com/franchialan/torresmo.git
cd torresmo

# Build
go build ./...

# Run tests
go test ./...

# Run E2E tests
go test -tags=e2e -v ./internal/e2e/

# Build the binary
go build -o ./bin/torresmo ./cmd/torresmo

# Build the node image (optional, for DinD testing)
scripts/build-node-image.sh
```

## How to Contribute

### Reporting Bugs

1. Check [existing issues](https://github.com/franchialan/torresmo/issues) first
2. Use the **Bug Report** template
3. Include:
   - Torresmo version (`torresmo version`)
   - OS and architecture
   - Docker/Podman version
   - Steps to reproduce
   - Expected vs actual behavior

### Suggesting Features

1. Check [existing issues](https://github.com/franchialan/torresmo/issues) first
2. Use the **Feature Request** template
3. Describe the use case, not just the solution

### Submitting Code

1. **Fork** the repository
2. **Create a branch** from `main`:
   ```bash
   git checkout -b feat/my-feature
   # or: fix/my-bugfix
   ```
3. **Make your changes** following the code guidelines below
4. **Test your changes**:
   ```bash
   go build ./...
   go vet ./...
   go test ./...
   go test -tags=e2e -v ./internal/e2e/
   ```
5. **Commit** with a descriptive message:
   ```bash
   git commit -m "feat: add cool new feature"
   # or: fix: resolve issue with X
   # or: docs: update Y documentation
   ```
6. **Push** and open a Pull Request

## Code Guidelines

### Project Structure

```
cmd/torresmo/     — CLI entrypoint
internal/cli/     — CLI commands (Cobra)
internal/cluster/ — Cluster management
internal/orchestrator/ — Pod/deployment/service management
internal/runtime/ — Container runtime abstraction (Docker/Podman)
internal/storage/ — SQLite persistence
internal/addon/   — Addon system
internal/pipeline/ — HCL pipeline engine
internal/workflow/ — GitHub Actions workflow runner
internal/dashboard/ — Web dashboard
internal/agent/   — AI agent (Ollama)
api/v1/           — API types (K8s-style)
docs/             — Documentation
```

### Conventions

- **Go version**: Follow `go.mod` version
- **Formatting**: `gofmt` (enforced by CI)
- **Linting**: `golangci-lint run` must pass
- **Error handling**: Always check errors; use `_ =` only for intentionally discarded errors
- **Naming**: Follow Go conventions (camelCase for unexported, PascalCase for exported)
- **Tests**: Add tests for new features; use `t.TempDir()` for temp files
- **CLI commands**: Follow the `init() { addCommand(cmd) }` pattern
- **Imports**: Group stdlib, then external, then internal packages

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new feature
fix: resolve bug
docs: update documentation
test: add tests
ci: update CI configuration
chore: maintenance tasks
refactor: code restructuring
```

### Adding a New CLI Command

1. Create `internal/cli/mycommand.go`
2. Define the Cobra command
3. Register with `func init() { addCommand(myCmd) }`
4. Add to `docs/cli-reference.md`

### Adding a New Addon

1. Create `internal/addon/builtin/myaddon.go`
2. Implement the `addon.Addon` interface
3. Register in `internal/addon/builtin/register.go`
4. Add to `docs/addons.md`

## Pull Request Process

1. PRs must target the `main` branch
2. CI must pass (build, test, lint, E2E)
3. One approval required for merge
4. Keep PRs focused — one feature/fix per PR
5. Update documentation if behavior changes
6. Add tests for new functionality

## Code of Conduct

Be respectful, inclusive, and constructive. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
