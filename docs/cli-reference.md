# CLI Reference

Complete reference for all `torresmo` commands, organized by category.

## Global Flags

These flags are available on most commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--cluster <name>` | | Target cluster (defaults to current) |
| `--verbose` | | Enable verbose output |
| `--output <format>` | `-o` | Output format: `table`, `json`, `yaml` |

---

## Cluster Management

### `torresmo create cluster`

Create a new cluster.

```bash
torresmo create cluster [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--name <name>` | | Cluster name (default: auto-generated) |
| `--nodes <n>` | | Number of worker nodes (default: 2) |
| `--profile <name>` | | Resource profile to use |
| `--template <name>` | `-t` | Template with pre-configured addons |
| `--pod-cidr <cidr>` | | Custom pod CIDR range |
| `-f <file>` | | Create from a cluster manifest file |

```bash
# Create a 3-node cluster with the fullstack template
torresmo create cluster --name dev --nodes 3 -t fullstack

# Create from a manifest file
torresmo create cluster -f cluster.yaml
```

### `torresmo create namespace`

Create a namespace in a cluster.

```bash
torresmo create namespace <name> [--cluster <name>]
```

### `torresmo delete cluster`

Delete a cluster and all its resources.

```bash
torresmo delete cluster <name>
```

### `torresmo delete namespace`

Delete a namespace from a cluster.

```bash
torresmo delete namespace <name> [--cluster <name>]
```

### `torresmo delete pod`

Delete a pod.

```bash
torresmo delete pod <name> [-n <namespace>] [--cluster <name>]
```

### `torresmo get clusters`

List all clusters.

```bash
torresmo get clusters
```

### `torresmo get nodes`

List nodes in a cluster.

```bash
torresmo get nodes [--cluster <name>]
```

### `torresmo get pods`

List pods in a cluster.

```bash
torresmo get pods [-n <namespace>] [--cluster <name>]
```

### `torresmo get services`

List services in a cluster.

```bash
torresmo get services [-n <namespace>] [--cluster <name>]
```

### `torresmo get deployments`

List deployments in a cluster.

```bash
torresmo get deployments [-n <namespace>] [--cluster <name>]
```

### `torresmo get namespaces`

List namespaces in a cluster.

```bash
torresmo get namespaces [--cluster <name>]
```

---

## Resource Management

### `torresmo apply`

Apply a manifest file or directory to the cluster.

```bash
torresmo apply -f <file|dir> [--cluster <name>]
```

**Supported apiVersions:**

| apiVersion | Kinds |
|---|---|
| `torresmo.io/v1` | Pod, Deployment, Service, Namespace |
| `apps/v1` | Deployment |
| `v1` | Pod, Service, Namespace |
| `batch/v1` | Job |

Existing Kubernetes YAML manifests work without modification.

```bash
# Apply a single file
torresmo apply -f deployment.yaml

# Apply all manifests in a directory
torresmo apply -f manifests/

# Standard K8s manifests work as-is
torresmo apply -f k8s/deployment.yaml    # apiVersion: apps/v1 ✅
torresmo apply -f k8s/service.yaml       # apiVersion: v1 ✅
torresmo apply -f k8s/                   # entire directory ✅
```

> **Note:** Complex K8s features (RBAC, CRDs, Operators) are not supported.

### `torresmo logs`

View pod logs.

```bash
torresmo logs <pod> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `-n <namespace>` | | Target namespace (default: `default`) |
| `-f` | | Follow log output in real-time |
| `--tail <n>` | | Number of recent lines to show |
| `--timestamps` | | Show timestamps on each line |
| `--cluster <name>` | | Target cluster |

```bash
# Follow logs from a pod
torresmo logs my-api -f --tail 100

# Show logs with timestamps
torresmo logs redis-0 --timestamps
```

### `torresmo exec`

Execute a command inside a pod.

```bash
torresmo exec [-it] <pod> [-n <namespace>] [--cluster <name>] -- <command>
```

```bash
# Interactive shell
torresmo exec -it my-api -- /bin/sh

# Run a one-off command
torresmo exec redis-0 -- redis-cli ping
```

### `torresmo events`

View cluster events.

```bash
torresmo events [flags]
```

| Flag | Description |
|------|-------------|
| `-n <namespace>` | Filter by namespace |
| `--watch` | Watch for new events in real-time |
| `--type <type>` | Filter by event type (`Normal`, `Warning`) |
| `<resource/name>` | Filter events for a specific resource |
| `--cluster <name>` | Target cluster |

```bash
# Watch all warning events
torresmo events --watch --type Warning

# Events for a specific pod
torresmo events pod/my-api
```

---

## Addons

### `torresmo addon install`

Install an addon into the cluster.

```bash
torresmo addon install <name> [flags]
```

| Flag | Description |
|------|-------------|
| `-n <namespace>` | Target namespace (default: `default`) |
| `--version <ver>` | Addon version |
| `--config key=value` | Configuration key-value pairs (repeatable) |
| `--from-manifest <file>` | Install from a custom addon manifest |
| `--cluster <name>` | Target cluster |

```bash
# Install Redis with defaults
torresmo addon install redis

# Install Postgres with custom config
torresmo addon install postgres --config user=dev --config database=myapp

# Install a community addon from a manifest
torresmo addon install --from-manifest custom-addon.yaml
```

### `torresmo addon remove`

Remove an installed addon.

```bash
torresmo addon remove <name> [-n <namespace>] [--cluster <name>]
```

### `torresmo addon list`

List installed addons.

```bash
torresmo addon list [--cluster <name>]
```

### `torresmo addon search`

Search available addons in the registry.

```bash
torresmo addon search [query]
```

### `torresmo addon info`

Show detailed information about an addon.

```bash
torresmo addon info <name>
```

### `torresmo addon publish`

Publish an addon to the registry.

```bash
torresmo addon publish <manifest-path>
```

---

## Pipelines

### `torresmo pipeline run`

Run pipeline targets from a Torresmofile.

```bash
torresmo pipeline run [flags]
```

| Flag | Description |
|------|-------------|
| `--target <name>` | Run a specific target (default: all) |
| `--verbose` | Show detailed build output |
| `--file <path>` | Path to Torresmofile (default: `./Torresmofile`) |
| `--no-parallel` | Disable parallel execution |

```bash
# Run all targets
torresmo pipeline run --verbose

# Run a specific target
torresmo pipeline run --target test
```

### `torresmo pipeline list`

List pipeline runs and their results.

```bash
torresmo pipeline list [--file <path>]
```

### `torresmo pipeline targets`

List all targets defined in a Torresmofile.

```bash
torresmo pipeline targets [--file <path>]
```

### `torresmo pipeline cache clear`

Clear the pipeline build cache.

```bash
torresmo pipeline cache clear
```

### `torresmo test`

Run test targets from a Torresmofile (shortcut for `pipeline run` with test targets).

```bash
torresmo test [flags]
```

| Flag | Description |
|------|-------------|
| `--integration` | Include integration test targets |
| `--target <name>` | Run a specific test target |
| `--file <path>` | Path to Torresmofile |
| `--verbose` | Show detailed test output |

---

## Workflows

### `torresmo workflow list`

List available GitHub Actions workflows in `.github/workflows/`.

```bash
torresmo workflow list
```

### `torresmo workflow run`

Run GitHub Actions workflows locally using act.

```bash
torresmo workflow run [workflow-file] [flags]
```

**Flags:**

| Flag | Description |
|---|---|
| `-f, --file` | Specific workflow file (e.g., `ci.yml`) |
| `-j, --job` | Specific job to run |
| `--event` | Trigger event (default: `push`) |
| `-v, --verbose` | Verbose output |
| `--dry-run` | Show steps without executing |
| `--secret` | Pass secrets `KEY=VALUE` (repeatable) |
| `--env` | Pass env vars `KEY=VALUE` (repeatable) |

**Examples:**

```bash
torresmo workflow run --file ci.yml --job test
torresmo workflow run --file ci.yml --dry-run
torresmo workflow run --secret GITHUB_TOKEN=ghp_xxx
```

---

## Dev Experience

### `torresmo dev start`

Start a development session with live file sync and hot-reload.

```bash
torresmo dev start --pod <name> --sync <local:remote> [flags]
```

| Flag | Description |
|------|-------------|
| `--pod <name>` | Pod to sync with |
| `--sync <local:remote>` | Local and remote paths to sync (e.g. `./src:/app/src`) |
| `--command <cmd>` | Command to run on file changes |
| `-n <namespace>` | Target namespace |
| `--cluster <name>` | Target cluster |

```bash
torresmo dev start --pod my-api --sync ./src:/app/src --command "go run ."
```

### `torresmo dev stop`

Stop the current development session.

```bash
torresmo dev stop
```

### `torresmo dashboard`

Open the Torresmo dashboard.

```bash
torresmo dashboard [flags]
```

| Flag | Description |
|------|-------------|
| `--cluster <name>` | Target cluster |
| `--refresh <seconds>` | Auto-refresh interval (TUI mode) |
| `--web` | Open the web-based dashboard in a browser |
| `--port <port>` | Port for the web dashboard (default: `9876`) |

```bash
# Terminal (TUI) dashboard
torresmo dashboard

# Web browser dashboard
torresmo dashboard --web
# Opens http://localhost:9876
```

See [Dashboard documentation](dashboard.md) for details on the web dashboard features including live pod logs and in-browser terminal.

### `torresmo up`

Set up a complete development environment from a `torresmo-env.yaml` file.

```bash
torresmo up [-f <file>]
```

```bash
# Use default torresmo-env.yaml
torresmo up

# Use a custom file
torresmo up -f environments/staging.yaml
```

---

## Networking

### `torresmo proxy`

Forward local ports to pods in the cluster.

```bash
torresmo proxy <pod> <localPort:remotePort> [-n <namespace>] [--cluster <name>]
torresmo proxy --auto [--cluster <name>] [--env]
```

| Flag | Description |
|------|-------------|
| `--auto` | Auto-discover and forward all service ports |
| `--env` | Write forwarded ports to a `.env` file |

```bash
# Forward a specific port
torresmo proxy redis-0 6379:6379

# Auto-forward all services and write .env file
torresmo proxy --auto --env
```

### `torresmo tunnel`

Expose a pod to the internet via a tunnel provider.

```bash
torresmo tunnel <pod> --port <port> [flags]
torresmo tunnel --auto [--cluster <name>]
```

| Flag | Description |
|------|-------------|
| `--port <port>` | Remote port to expose |
| `--local-port <port>` | Local port to bind |
| `--provider <name>` | Tunnel provider (e.g. `cloudflared`) |
| `-n <namespace>` | Target namespace |
| `--cluster <name>` | Target cluster |
| `--auto` | Auto-discover and tunnel all services |

```bash
# Expose a pod via cloudflared
torresmo tunnel my-api --port 8080 --provider cloudflared

# Auto-tunnel all services
torresmo tunnel --auto
```

---

## Snapshots

### `torresmo snapshot create`

Create a snapshot of the current cluster state.

```bash
torresmo snapshot create --name <name> [--cluster <name>]
```

### `torresmo snapshot list`

List available snapshots.

```bash
torresmo snapshot list
```

### `torresmo snapshot restore`

Restore a cluster from a snapshot.

```bash
torresmo snapshot restore <name>
```

### `torresmo snapshot delete`

Delete a snapshot.

```bash
torresmo snapshot delete <name>
```

### `torresmo snapshot export`

Export a snapshot to a file.

```bash
torresmo snapshot export <name> -o <file>
```

### `torresmo snapshot import`

Import a snapshot from a file.

```bash
torresmo snapshot import <file>
```

---

## Templates

### `torresmo template list`

List available environment templates.

```bash
torresmo template list
```

### `torresmo template save`

Save the current cluster environment as a reusable template.

```bash
torresmo template save --name <name> [-d <description>] [--cluster <name>]
```

### `torresmo template export`

Export a template to a file.

```bash
torresmo template export <name> -o <file>
```

### `torresmo template import`

Import a template from a file.

```bash
torresmo template import <file>
```

---

## Environment Sharing

### `torresmo share export`

Export the current cluster state to a portable bundle that teammates can import.

```bash
torresmo share export [--cluster <name>] -o <file>
```

### `torresmo share import`

Import a cluster state from a bundle.

```bash
torresmo share import <file>
```

### `torresmo compose import`

Convert a Docker Compose file into Torresmo manifests.

```bash
torresmo compose import <docker-compose.yaml> [-o <dir>]
```

```bash
torresmo compose import docker-compose.yaml -o manifests/
# ℹ Detected addon → use `torresmo addon install redis`
# ✓ Converted service "web" → Deployment
```

### `torresmo compose up`

Start services directly from a Docker Compose file.

```bash
torresmo compose up -f <docker-compose.yaml> [--cluster <name>]
```

---

## Mock Servers

### `torresmo mock create`

Create a mock server from a definition file or inline.

```bash
torresmo mock create -f <mock.yaml> [--cluster <name>]
torresmo mock create --name <name> --port <port> [--cluster <name>]
```

```bash
# From a mock definition file
torresmo mock create -f stripe-api.yaml

# Quick inline mock
torresmo mock create --name payment-api --port 8080
```

### `torresmo mock list`

List running mock servers.

```bash
torresmo mock list [--cluster <name>]
```

### `torresmo mock delete`

Delete a mock server.

```bash
torresmo mock delete <name> [-n <namespace>] [--cluster <name>]
```

### `torresmo mock logs`

View logs from a mock server.

```bash
torresmo mock logs <name> [-n <namespace>] [-f] [--cluster <name>]
```

---

## Benchmarking

### `torresmo benchmark redis`

Run benchmarks against a Redis instance.

```bash
torresmo benchmark redis [-n <namespace>] [--cluster <name>] [--duration <time>] [--concurrency <n>]
```

### `torresmo benchmark postgres`

Run benchmarks against a PostgreSQL instance.

```bash
torresmo benchmark postgres [-n <namespace>] [--cluster <name>] [--duration <time>] [--concurrency <n>]
```

### `torresmo benchmark http`

Run HTTP benchmarks against a pod.

```bash
torresmo benchmark http <pod> --port <port> [flags]
```

| Flag | Description |
|------|-------------|
| `--port <port>` | Target port |
| `--path <path>` | URL path (default: `/`) |
| `--method <method>` | HTTP method (default: `GET`) |
| `--duration <time>` | Benchmark duration (default: `10s`) |
| `--concurrency <n>` | Number of concurrent connections |
| `-n <namespace>` | Target namespace |
| `--cluster <name>` | Target cluster |

```bash
torresmo benchmark http my-api --port 8080 --path /api/health --duration 30s --concurrency 10
```

---

## Profiles

### `torresmo profile list`

List available resource profiles.

```bash
torresmo profile list
```

### `torresmo profile set`

Set the active resource profile for a cluster.

```bash
torresmo profile set <name> [--cluster <name>]
```

### `torresmo profile create`

Create a custom resource profile.

```bash
torresmo profile create --name <name> --cpu <limit> --memory <limit> --max-pods <n>
```

```bash
torresmo profile create --name heavy --cpu 4 --memory 8Gi --max-pods 50
```

---

## Utilities

### `torresmo doctor`

Check system requirements and diagnose issues.

```bash
torresmo doctor
```

### `torresmo version`

Print the current version of Torresmo.

```bash
torresmo version
```

### `torresmo cleanup`

Remove Torresmo resources and clean up.

```bash
torresmo cleanup [flags]
```

| Flag | Description |
|------|-------------|
| `--all` | Remove everything (clusters, images, cache) |
| `--clusters` | Remove all clusters |
| `--images` | Remove cached images |
| `--dry-run` | Show what would be removed without doing it |

```bash
# Preview what would be cleaned
torresmo cleanup --all --dry-run

# Clean everything
torresmo cleanup --all
```

---

## AI Agent

### Natural Language Commands

Execute commands using natural language, powered by Ollama running locally.

```bash
torresmo "<natural language query>"
```

```bash
torresmo "create a redis instance on the staging namespace"
torresmo "show me all failing pods"
torresmo "install postgres with user=dev and database=myapp"
```

### `torresmo chat`

Start an interactive AI chat session.

```bash
torresmo chat [--model <name>]
```

| Flag | Description |
|------|-------------|
| `--model <name>` | Ollama model to use (default: `llama3.1:8b`) |

```bash
torresmo chat
🐷 Torresmo Chat v0.1.0 (ollama/llama3.1:8b)
> install redis and postgres on the test namespace
✓ Addon "redis" installed in namespace "test"
✓ Addon "postgres" installed in namespace "test"
```
