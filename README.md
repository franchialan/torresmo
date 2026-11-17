# 🐷 Torresmo

A container orchestrator for local development.

Torresmo runs on top of Docker or Podman and provides developers with:

- **Complete local stack** — Redis, Postgres, Istio, Prometheus, Grafana and more in a single command
- **Local pipelines** — Run CI/CD locally inside containers via `Torresmofile` (HCL)
- **Smart CLI** — Agent mode with natural language via Ollama
- **K8s compatibility** — Kubernetes-style YAML manifests
- **Multi-node clusters** — Real multi-node clusters using containers (like KinD)

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/getting-started/installation)
- [Ollama](https://ollama.ai) (optional, for agent mode)

## Installation

```bash
brew install torresmo
```

## Quick Start

```bash
# Create a cluster
torresmo create cluster --name dev --nodes 3

# Install addons
torresmo addon install redis
torresmo addon install postgres

# Deploy your app
torresmo apply -f deployment.yaml

# Or use natural language
torresmo "create a redis instance on test namespace"

# Run pipelines
torresmo pipeline run

# One-command environment setup
torresmo up
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [CLI Reference](docs/cli-reference.md)
- [Torresmofile Guide](docs/torresmofile.md)
- [Addons](docs/addons.md)

## License

MIT
