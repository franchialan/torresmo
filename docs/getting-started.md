# Getting Started with Torresmo

## Prerequisites

- **Docker** (recommended) or **Podman** installed and running
- **Ollama** (optional) for AI agent mode

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap franchialan/torresmo
brew install torresmo
```

### Shell script

```bash
curl -fsSL https://raw.githubusercontent.com/torresmo-dev/torresmo/main/scripts/install.sh | bash
```

### From source

```bash
go install github.com/torresmo-dev/torresmo/cmd/torresmo@latest
```

## Verify Installation

```bash
torresmo doctor
```

This checks that Docker (or Podman) is available and your system is ready.

## Create Your First Cluster

```bash
torresmo create cluster --name my-first-cluster --nodes 3
```

This creates a 3-node cluster (1 control-plane + 2 workers) using Docker containers.

## Check Cluster Status

```bash
# List clusters
torresmo get clusters

# List nodes
torresmo get nodes --cluster my-first-cluster
```

## Install Addons

```bash
# Install Redis
torresmo addon install redis

# Install PostgreSQL
torresmo addon install postgres

# List installed addons
torresmo addon list
```

## Deploy Your Application

Create a manifest file:

```yaml
# my-app.yaml
apiVersion: torresmo.io/v1
kind: Deployment
metadata:
  name: my-api
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: api
          image: my-api:latest
          ports:
            - containerPort: 8080
```

Apply it:

```bash
torresmo apply -f my-app.yaml
```

### Using Existing K8s Manifests

If you already have Kubernetes YAML manifests, they work directly:

```bash
# No changes needed — standard K8s apiVersions are supported
torresmo apply -f k8s/
```

Supported apiVersions include `apps/v1`, `v1`, and `batch/v1` in addition to `torresmo.io/v1`.

## Use AI Agent Mode

If you have Ollama installed:

```bash
# Single command
torresmo "create a redis instance on test namespace"

# Interactive chat
torresmo chat
```

## Run Pipelines

Create a `Torresmofile` in your project root (see [Torresmofile Guide](torresmofile.md)), then:

```bash
torresmo pipeline run
torresmo pipeline run --target test
```

## Run GitHub Actions Locally

If your project has GitHub Actions workflows, you can run them locally:

```bash
# Install act (optional, for better performance)
brew install act

# List workflows
torresmo workflow list

# Run a workflow
torresmo workflow run --file ci.yml

# With GitHub token (needed for some actions)
torresmo workflow run --file ci.yml --secret GITHUB_TOKEN=ghp_your_token
```

See [Workflow Guide](workflows.md) for more details.

## One-Command Environment

Create a `torresmo-env.yaml` in your project and share it with your team:

```bash
# Set up everything from torresmo-env.yaml
torresmo up
```

## Dashboard

### Terminal (TUI)

```bash
torresmo dashboard
```

### Web Browser

```bash
torresmo dashboard --web
# Opens http://localhost:9876 in your browser
# Click on any pod to see logs and run commands
```

See [Dashboard](dashboard.md) for the full guide on live logs, in-browser terminal, and the dashboard API.

## Clean Up

```bash
# Delete a cluster
torresmo delete cluster my-first-cluster

# Clean up all resources
torresmo cleanup --all
```

## Next Steps

- [CLI Reference](cli-reference.md) — All available commands
- [Dashboard](dashboard.md) — TUI and web dashboard with live logs
- [Torresmofile Guide](torresmofile.md) — Pipeline configuration
- [Addons](addons.md) — Available addons and configuration
- [Workflows](workflows.md) — Run GitHub Actions locally
- [Dev Mode](dev-mode.md) — Hot reload for development
- [Templates](templates.md) — Pre-built environment templates
