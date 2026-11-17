# Torresmofile Guide

A `Torresmofile` defines local CI/CD pipelines using [HCL](https://github.com/hashicorp/hcl) syntax. It describes **base images**, **targets** (pipeline steps), their **dependencies**, and how **artifacts** flow between them. Torresmo parses the file, builds a dependency graph, and executes independent targets in parallel.

Think of it as a `Makefile` for containers — each target runs inside a fresh container with the exact image, files, and environment you specify.

---

## Quick Start

**1. Create a `Torresmofile` in your project root:**

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "test" {
  from = "golang"

  copy {
    src  = "."
    dest = "/app"
  }

  run "go test ./..." {}
}
```

**2. Run it:**

```bash
torresmo pipeline run
```

**3. See the results:**

```
🐷 Torresmo Pipeline
  [test] run: go test ./...
  [test] done (4.1s)
Pipeline completed: 1 passed, 0 failed (4.1s total)
```

---

## Syntax Reference

### `version`

**Required.** Specifies the Torresmofile format version. Currently the only supported value is `"0.1"`.

```hcl
version = "0.1"
```

The parser rejects files with a missing `version` field.

---

### `base` Block

Defines a reusable image and working directory that targets can reference. The block label is the base's name.

```hcl
base "<name>" {
  image   = "<docker-image>"
  workdir = "<path>"            # optional
}
```

| Attribute | Required | Description |
|---|---|---|
| `image` | Yes | Docker image to use (e.g., `golang:1.22-alpine`, `node:20-slim`) |
| `workdir` | No | Working directory inside the container |

You can define multiple bases:

```hcl
base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

base "node" {
  image   = "node:20-slim"
  workdir = "/app"
}
```

---

### `target` Block

Defines a pipeline step. The block label is the target's name and must be unique within the file.

```hcl
target "<name>" {
  from         = "<base-name>"         # Reference a base block
  from_image   = "<docker-image>"      # OR use a Docker image directly
  depends_on   = ["target-a", "..."]   # Optional: run after these targets
  needs_cluster = true                 # Optional: requires a running cluster
  addons       = ["redis", "postgres"] # Optional: addons needed for this target

  copy { ... }            # Copy files into the container
  run "<command>" {}      # Execute a command
  artifact { ... }        # Save output files
  copy_artifact { ... }   # Receive artifacts from another target
  env { ... }             # Set environment variables
}
```

Every target must have **either** `from` or `from_image` — not both, not neither. The parser validates this and returns an error if the rule is violated.

---

### `from`

References a `base` block by name. The target inherits the base's `image` and `workdir`.

```hcl
base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "test" {
  from = "golang"   # Uses golang:1.22-alpine with workdir /app
  # ...
}
```

If `from` references a base name that doesn't exist, the parser returns an error.

---

### `from_image`

Uses a Docker image directly, without referencing a base. Use this when a target needs a different image than your bases, or when you don't need a shared base definition.

```hcl
target "docker" {
  from_image = "alpine:3.19"
  # ...
}
```

---

### `depends_on`

A list of target names that must complete successfully before this target runs.

```hcl
target "build" {
  from       = "golang"
  depends_on = ["lint", "test"]   # Waits for both lint and test to finish
  # ...
}
```

The parser validates that:
- Every dependency references an existing target
- There are no circular dependencies (the dependency graph is a DAG)

If a cycle is detected, the parser returns an error like:

```
circular dependency: build -> deploy -> build
```

---

### `copy` Block

Copies files from the host into the container.

```hcl
copy {
  src  = "."       # Source path on host (relative to project root)
  dest = "/app"    # Destination path inside the container
}
```

| Attribute | Required | Description |
|---|---|---|
| `src` | Yes | Source path on the host filesystem |
| `dest` | Yes | Destination path inside the container |

A target can have multiple `copy` blocks:

```hcl
target "build" {
  from = "golang"

  copy {
    src  = "go.mod"
    dest = "/app/go.mod"
  }

  copy {
    src  = "go.sum"
    dest = "/app/go.sum"
  }

  copy {
    src  = "."
    dest = "/app"
  }

  # ...
}
```

---

### `run` Block

Executes a command inside the container. The command itself is the block label.

```hcl
run "go test -race ./..." {}
```

Commands run in order within a target. If any `run` block fails (non-zero exit code), the target fails and dependent targets are not executed.

Multiple `run` blocks execute sequentially:

```hcl
target "lint" {
  from = "golang"

  copy {
    src  = "."
    dest = "/app"
  }

  run "go vet ./..." {}
  run "golangci-lint run" {}
}
```

---

### `artifact` Block

Declares an output file or directory to save from the container after the target completes. Other targets can receive these artifacts using `copy_artifact`.

```hcl
artifact {
  path = "/out/app"    # Path inside the container to save
}
```

| Attribute | Required | Description |
|---|---|---|
| `path` | Yes | Path inside the container to the file or directory |

A target can produce multiple artifacts:

```hcl
target "build" {
  from       = "golang"
  depends_on = ["test"]

  copy {
    src  = "."
    dest = "/app"
  }

  run "CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/app ./cmd/app" {}

  artifact {
    path = "/out/app"
  }
}
```

---

### `copy_artifact` Block

Copies an artifact produced by another target into the current target's container.

```hcl
copy_artifact {
  from = "build"                 # Source target name
  src  = "/out/app"              # Artifact path in the source target
  dest = "/usr/local/bin/app"    # Destination path in this target
}
```

| Attribute | Required | Description |
|---|---|---|
| `from` | Yes | Name of the target that produced the artifact |
| `src` | Yes | Artifact path as declared in the source target's `artifact` block |
| `dest` | Yes | Destination path inside this target's container |

This is how you pass build outputs between pipeline stages:

```hcl
target "build" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go build -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}

target "docker" {
  depends_on = ["build"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "build"
    src  = "/out/app"
    dest = "/usr/local/bin/app"
  }
}
```

---

### `env` Block

Sets environment variables for the target's container. Uses arbitrary HCL key-value pairs inside the block.

```hcl
env {
  DATABASE_URL = "postgres://dev:dev@postgres.default:5432/testdb"
  REDIS_URL    = "redis.default:6379"
  LOG_LEVEL    = "debug"
}
```

All values must be strings. A target can have multiple `env` blocks — they are merged, with later blocks taking precedence.

---

### `needs_cluster`

Boolean flag. When `true`, the target requires a running torresmo cluster. If no cluster is available, the pipeline runner will create or start one before executing this target.

```hcl
target "integration-test" {
  from          = "golang"
  needs_cluster = true
  # ...
}
```

Default: `false`

---

### `addons`

A list of addon names that the target needs. When `needs_cluster` is `true` and `addons` are specified, the pipeline runner ensures these addons are installed in the cluster before the target executes.

```hcl
target "integration-test" {
  from          = "golang"
  needs_cluster = true
  addons        = ["redis", "postgres"]
  # ...
}
```

See the [Addons documentation](addons.md) for the full list of available addons.

---

## Dependency Graph

Torresmo builds a **directed acyclic graph (DAG)** from the `depends_on` declarations. Targets with no dependencies (or whose dependencies have all completed) run in parallel.

### Example

Given this Torresmofile:

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "lint" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go vet ./..." {}
}

target "test" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go test -race ./..." {}
}

target "build" {
  from       = "golang"
  depends_on = ["lint", "test"]
  copy { src = "." dest = "/app" }
  run "go build -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}

target "deploy" {
  depends_on    = ["build"]
  needs_cluster = true
  from_image    = "bitnami/kubectl:latest"
  run "torresmo apply -f manifests/" {}
}
```

The dependency graph is:

```
lint ──┐
       ├──▶ build ──▶ deploy
test ──┘
```

Execution order:
1. **`lint`** and **`test`** run in parallel (no dependencies)
2. **`build`** starts after both `lint` and `test` succeed
3. **`deploy`** starts after `build` succeeds

If `lint` fails, `build` and `deploy` are skipped. If `test` fails, `build` and `deploy` are skipped. `lint` and `test` are independent — one failing doesn't stop the other.

### Cycle Detection

The parser detects circular dependencies and rejects them:

```hcl
target "a" {
  from_image = "alpine"
  depends_on = ["b"]
  run "echo a" {}
}

target "b" {
  from_image = "alpine"
  depends_on = ["a"]
  run "echo b" {}
}
```

```
Error: circular dependency: a -> b -> a
```

---

## Artifacts

Artifacts are the mechanism for passing files between pipeline targets. They enable multi-stage workflows where one target produces outputs that another target consumes.

### How It Works

1. A target declares output files with `artifact { path = "..." }`
2. After the target's `run` commands complete, torresmo extracts the artifact from the container
3. Another target uses `copy_artifact { from = "..." src = "..." dest = "..." }` to inject the artifact into its own container

### Multi-Stage Build Example

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

# Stage 1: Build the binary
target "build" {
  from = "golang"

  copy {
    src  = "."
    dest = "/app"
  }

  run "CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/app ./cmd/app" {}

  artifact {
    path = "/out/app"
  }
}

# Stage 2: Package into a minimal image
target "docker" {
  depends_on = ["build"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "build"
    src  = "/out/app"
    dest = "/usr/local/bin/app"
  }
}
```

This is equivalent to a multi-stage Docker build, but with the pipeline engine managing the stages.

### Multiple Artifacts

A target can produce multiple artifacts, and multiple targets can consume them:

```hcl
target "build" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go build -o /out/server ./cmd/server" {}
  run "go build -o /out/worker ./cmd/worker" {}

  artifact { path = "/out/server" }
  artifact { path = "/out/worker" }
}

target "deploy-server" {
  depends_on = ["build"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "build"
    src  = "/out/server"
    dest = "/usr/local/bin/server"
  }
}

target "deploy-worker" {
  depends_on = ["build"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "build"
    src  = "/out/worker"
    dest = "/usr/local/bin/worker"
  }
}
```

---

## Environment Variables

Use `env` blocks to set environment variables available to `run` commands within a target.

```hcl
target "integration-test" {
  from          = "golang"
  needs_cluster = true
  addons        = ["redis", "postgres"]

  copy {
    src  = "."
    dest = "/app"
  }

  env {
    REDIS_URL    = "redis.default:6379"
    DATABASE_URL = "postgres://dev:dev@postgres.default:5432/testdb"
    LOG_LEVEL    = "debug"
  }

  run "go test -tags=integration -v ./test/integration/..." {}
}
```

### How Env Blocks Work

- Values are parsed from the HCL body as string key-value pairs
- Multiple `env` blocks in the same target are merged
- All `env` values are available to every `run` command in the target
- Values are strings only — use string interpolation for dynamic values

---

## Examples

### Go Project: Lint → Test → Build → Deploy

A complete pipeline for a Go project with linting, testing, building, integration tests with addons, and deployment:

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "lint" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go vet ./..." {}
  run "golangci-lint run" {}
}

target "test" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go test -race -coverprofile=coverage.out ./..." {}
  artifact { path = "coverage.out" }
}

target "build" {
  from       = "golang"
  depends_on = ["lint", "test"]
  copy { src = "." dest = "/app" }
  run "CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}

target "integration-test" {
  from          = "golang"
  depends_on    = ["build"]
  needs_cluster = true
  addons        = ["redis", "postgres"]

  copy { src = "." dest = "/app" }

  env {
    REDIS_URL    = "redis.default:6379"
    DATABASE_URL = "postgres://dev:dev@postgres.default:5432/testdb"
  }

  run "go test -tags=integration -v ./test/integration/..." {}
}

target "docker" {
  depends_on = ["build"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "build"
    src  = "/out/app"
    dest = "/usr/local/bin/app"
  }
}

target "deploy" {
  depends_on    = ["integration-test", "docker"]
  needs_cluster = true
  from_image    = "bitnami/kubectl:latest"
  run "torresmo apply -f manifests/" {}
}
```

**Execution graph:**

```
lint ──┐
       ├──▶ build ──┬──▶ integration-test ──┐
test ──┘            │                        ├──▶ deploy
                    └──▶ docker ─────────────┘
```

---

### Node.js Project: Install → Lint → Test → Build

```hcl
version = "0.1"

base "node" {
  image   = "node:20-slim"
  workdir = "/app"
}

target "install" {
  from = "node"

  copy {
    src  = "package.json"
    dest = "/app/package.json"
  }

  copy {
    src  = "package-lock.json"
    dest = "/app/package-lock.json"
  }

  run "npm ci" {}

  artifact {
    path = "/app/node_modules"
  }
}

target "lint" {
  from       = "node"
  depends_on = ["install"]

  copy { src = "." dest = "/app" }

  copy_artifact {
    from = "install"
    src  = "/app/node_modules"
    dest = "/app/node_modules"
  }

  run "npx eslint src/" {}
}

target "test" {
  from       = "node"
  depends_on = ["install"]

  copy { src = "." dest = "/app" }

  copy_artifact {
    from = "install"
    src  = "/app/node_modules"
    dest = "/app/node_modules"
  }

  run "npx jest --coverage" {}

  artifact {
    path = "/app/coverage"
  }
}

target "build" {
  from       = "node"
  depends_on = ["lint", "test"]

  copy { src = "." dest = "/app" }

  copy_artifact {
    from = "install"
    src  = "/app/node_modules"
    dest = "/app/node_modules"
  }

  run "npm run build" {}

  artifact {
    path = "/app/dist"
  }
}
```

**Execution graph:**

```
install ──┬──▶ lint ──┐
          │           ├──▶ build
          └──▶ test ──┘
```

---

### Integration Tests with Cluster Addons

Use `needs_cluster` and `addons` to run tests against real infrastructure:

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "integration" {
  from          = "golang"
  needs_cluster = true
  addons        = ["redis", "postgres", "nats"]

  copy { src = "." dest = "/app" }

  env {
    REDIS_URL    = "redis.default:6379"
    DATABASE_URL = "postgres://torresmo:torresmo@postgres.default:5432/torresmo"
    NATS_URL     = "nats://nats.default:4222"
  }

  run "go test -tags=integration -v -count=1 ./test/..." {}
}
```

Before the `integration` target runs, torresmo ensures:
- A cluster is running
- Redis, PostgreSQL, and NATS addons are installed and ready

---

### Multi-Stage Docker Build Equivalent

Traditional multi-stage Dockerfile:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/app ./cmd/app

FROM alpine:3.19
COPY --from=builder /out/app /usr/local/bin/app
CMD ["app"]
```

Equivalent Torresmofile:

```hcl
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "builder" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go build -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}

target "runtime" {
  depends_on = ["builder"]
  from_image = "alpine:3.19"

  copy_artifact {
    from = "builder"
    src  = "/out/app"
    dest = "/usr/local/bin/app"
  }
}
```

The Torresmofile approach gives you independent caching, parallel builds for multiple binaries, and the ability to insert test and lint steps between stages.

---

## CLI Commands

### Run a pipeline

```bash
# Run all targets (respecting dependency order)
torresmo pipeline run

# Run a specific target (and its dependencies)
torresmo pipeline run --target build

# Verbose output (shows command output in real time)
torresmo pipeline run --verbose

# Disable parallel execution (run targets sequentially)
torresmo pipeline run --no-parallel
```

### List pipelines

```bash
torresmo pipeline list
```

Shows discovered Torresmofile(s) and their status.

### List targets

```bash
torresmo pipeline targets

# Output:
# TARGET              DEPENDS ON            NEEDS CLUSTER  ADDONS
# lint                -                     no             -
# test                -                     no             -
# build               lint, test            no             -
# integration-test    build                 yes            redis, postgres
# docker              build                 no             -
# deploy              integration-test,     yes            -
#                     docker
```

### Clear pipeline cache

```bash
torresmo pipeline cache clear
```

Removes cached build layers and artifacts. Useful when you want to force a clean rebuild.

### Run tests

```bash
# Run unit tests (targets without needs_cluster)
torresmo test

# Run integration tests (targets with needs_cluster)
torresmo test --integration
```

---

## Best Practices

### 1. Use small base images

Alpine variants are significantly smaller and faster to pull:

```hcl
base "golang" {
  image = "golang:1.22-alpine"    # ~250MB vs ~800MB for the full image
}
```

### 2. Copy dependency files first

Copy `go.mod`/`go.sum` (or `package.json`/`package-lock.json`) before the full source tree. This lets the container layer cache survive source code changes:

```hcl
target "build" {
  from = "golang"

  copy { src = "go.mod" dest = "/app/go.mod" }
  copy { src = "go.sum" dest = "/app/go.sum" }
  run "go mod download" {}

  copy { src = "." dest = "/app" }
  run "go build -o /out/app ./cmd/app" {}
}
```

### 3. Maximize parallelism

Structure your targets so independent work runs in parallel. Lint and test don't depend on each other — don't chain them:

```hcl
# ✅ Good: lint and test run in parallel
target "lint" { from = "golang" ... }
target "test" { from = "golang" ... }
target "build" { depends_on = ["lint", "test"] ... }

# ❌ Bad: forces sequential execution
target "lint" { from = "golang" ... }
target "test" { depends_on = ["lint"] ... }
target "build" { depends_on = ["test"] ... }
```

### 4. Use artifacts to pass between stages

Don't rebuild in later stages. Build once, pass the artifact:

```hcl
# ✅ Good: build once, copy artifact
target "build" {
  run "go build -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}
target "docker" {
  depends_on = ["build"]
  copy_artifact { from = "build" src = "/out/app" dest = "/usr/local/bin/app" }
}
```

### 5. Use `from_image` for specialized steps

Not every step needs your project's base image. Use `from_image` for deployment, packaging, or tools:

```hcl
target "deploy" {
  from_image = "bitnami/kubectl:latest"
  run "torresmo apply -f manifests/" {}
}
```

### 6. Keep integration tests separate

Separate unit tests from integration tests. Unit tests are fast and don't need a cluster. Integration tests need addons and a running cluster — they're slower and should only run after the build succeeds:

```hcl
target "unit-test" {
  from = "golang"
  run "go test ./..." {}
}

target "integration-test" {
  from          = "golang"
  depends_on    = ["build"]
  needs_cluster = true
  addons        = ["redis", "postgres"]
  run "go test -tags=integration ./test/..." {}
}
```

### 7. Use `--target` during development

When iterating on a specific step, run just that target instead of the full pipeline:

```bash
# Only run the build target (and its dependencies)
torresmo pipeline run --target build

# Only run integration tests
torresmo pipeline run --target integration-test --verbose
```
