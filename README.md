# <img src="docs/assets/logo.png" width="64" alt="Torresmo Logo"> Torresmo

**Development infrastructure in one binary.**

Torresmo is a container orchestrator built for local development. It runs on top of Docker or Podman and gives you production-like clusters, one-command addon installs, local CI/CD pipelines, and an AI-powered CLI — all without leaving your terminal.

> *From `git clone` to full stack in one command.*

---

## Why Torresmo?

Setting up a local development environment shouldn't take hours. Yet developers today face a painful choice:

- **Docker Compose** — simple but limited. No orchestration, no service mesh, no pipelines, no dashboards.
- **KinD + kubectl + Helm** — powerful but complex. 5+ tools, steep learning curve, endless YAML.
- **Minikube** — heavy, slow to boot, still requires kubectl/Helm knowledge.

**Torresmo bridges the gap**: the simplicity of Docker Compose with the power of Kubernetes, purpose-built for developers.

### The Developer's Problem

```bash
# Today: Docker Compose (limited)
docker-compose up -d                     # no orchestration, no scheduling
docker exec -it redis redis-cli          # manual container management
# No pipelines, no agent mode, no snapshots, no templates

# Today: KinD + kubectl + Helm (complex)
kind create cluster                      # step 1 of many
kubectl apply -f namespace.yaml          # step 2
helm repo add bitnami https://charts...  # step 3
helm install redis bitnami/redis         # step 4
kubectl port-forward svc/redis 6379      # step 5
# 5+ tools, 10+ commands, 10+ minutes
```

### With Torresmo

```bash
torresmo create cluster -t go-api --name dev
# ✓ Cluster "dev" created with 3 nodes
# ✓ Redis installed
# ✓ Postgres installed
# ✓ Prometheus installed
# Done. 30 seconds. One command.
```

---

## Feature Comparison

| Feature | Docker Compose | KinD + kubectl | **Torresmo** |
|---|:---:|:---:|:---:|
| One-command setup | ❌ | ❌ | ✅ `torresmo up` |
| Multi-node clusters | ❌ | ✅ | ✅ |
| Built-in addons (Redis, Postgres, etc.) | ❌ Manual | ⚠️ Helm | ✅ `addon install redis` |
| Local CI/CD pipelines | ❌ | ❌ | ✅ `pipeline run` |
| AI-powered CLI | ❌ | ❌ | ✅ `torresmo "install redis"` |
| TUI Dashboard | ❌ | ❌ (k9s external) | ✅ `dashboard` |
| Environment snapshots | ❌ | ❌ | ✅ `snapshot create` |
| Pre-built templates | ❌ | ❌ | ✅ 8 templates |
| Hot-reload dev mode | ❌ Volume hacks | ❌ (Tilt/Skaffold) | ✅ `dev start` |
| Mock servers | ❌ | ❌ | ✅ `mock create` |
| Benchmarking | ❌ | ❌ | ✅ `benchmark redis` |
| Docker Compose migration | N/A | ❌ | ✅ `compose import` |
| Team environment sharing | Commit compose file | Commit manifests | ✅ `share export` → `torresmo up` |
| Community plugins | ❌ | Helm charts | ✅ YAML addon manifests |
| Tools required | 1 (Docker) | 5+ | **1** (torresmo) |
| Learning curve | Low | High | **Low** |

---

## Key Features

### 🚀 Zero-to-Stack in One Command

Create a full development stack with a single command using pre-built templates:

```bash
torresmo create cluster -t go-api --name dev
# → 3 nodes + Redis + Postgres + Prometheus in 30 seconds

torresmo create cluster -t microservices --name dev
# → 3 nodes + Istio + Jaeger + Prometheus + Grafana
```

Or define your own stack in a `torresmo.yaml`:

```yaml
apiVersion: torresmo.io/v1
kind: Cluster
metadata:
  name: dev
spec:
  nodes:
    - role: control-plane
    - role: worker
      count: 2
  profile: standard
  addons:
    - name: redis
      namespace: cache
    - name: postgres
    - name: prometheus
    - name: grafana
```

### 🤖 AI-Powered CLI (Unique in the Market)

Talk to your cluster in plain English, powered by [Ollama](https://ollama.ai) running locally:

```bash
torresmo "create a redis instance on the staging namespace"
torresmo "show me all failing pods"
torresmo "what's the status of my cluster?"
torresmo "install postgres with user=dev and database=myapp"

# Interactive chat mode
torresmo chat
🐷 Torresmo Chat v0.1.0 (ollama/llama3.1:8b)
> install redis and postgres on the test namespace
✓ Addon "redis" installed in namespace "test"
✓ Addon "postgres" installed in namespace "test"
```

No other container orchestrator offers natural language CLI interaction.

### 📦 10 Built-in Addons

Install production-grade services with a single command:

```bash
torresmo addon install redis          # In-memory cache (port 6379)
torresmo addon install postgres       # Relational DB (port 5432)
torresmo addon install mongodb        # Document DB (port 27017)
torresmo addon install prometheus     # Monitoring (port 9090)
torresmo addon install grafana        # Dashboards (port 3000)
torresmo addon install istio          # Service mesh
torresmo addon install jaeger         # Distributed tracing (port 16686)
torresmo addon install kafka          # Event streaming (port 9092)
torresmo addon install nats           # Messaging (port 4222)
torresmo addon install minio          # S3 storage (port 9000)
```

Each addon is fully configured and ready to use — no Helm charts, no values files, no guessing.

### 🔧 Local CI/CD Pipelines

Run your CI/CD pipelines locally using `Torresmofile` (HCL syntax), with dependency graphs, parallel execution, and artifact passing:

```hcl
# Torresmofile
version = "0.1"

base "golang" {
  image   = "golang:1.22-alpine"
  workdir = "/app"
}

target "test" {
  from = "golang"
  copy { src = "." dest = "/app" }
  run "go test -race ./..." {}
}

target "build" {
  from       = "golang"
  depends_on = ["test"]
  copy { src = "." dest = "/app" }
  run "go build -o /out/app ./cmd/app" {}
  artifact { path = "/out/app" }
}
```

```bash
torresmo pipeline run --verbose
# 🐷 Torresmo Pipeline
#   [test]  run: go test -race ./...
#   [test]  done (5.2s)
#   [build] run: go build -o /out/app ./cmd/app
#   [build] saving artifact /out/app
#   [build] done (3.1s)
# Pipeline completed: 2 passed, 0 failed (8.3s total)
```

Test locally with the **exact same containers** that run in your CI. No more "works on my machine".

### 📸 Environment Snapshots

Save and restore your entire cluster state — perfect for risky migrations:

```bash
torresmo snapshot create --name "before-migration"
# ... run migration, things break ...
torresmo snapshot restore "before-migration"
# Back to the good state.
```

### 🔌 One-Command Team Onboarding

Share your environment with the team. Commit a `torresmo-env.yaml` to your repo:

```bash
# You: export the environment
torresmo share export -o torresmo-env.yaml
git add torresmo-env.yaml && git commit -m "Add dev environment"

# New teammate: one command
git clone my-project
torresmo up
# → Cluster created, addons installed, manifests applied
# → "Works on my machine" becomes "works on everyone's machine"
```

### 🐳 Docker Compose Migration

Already using Docker Compose? Migrate in seconds:

```bash
torresmo compose import docker-compose.yaml -o manifests/
# ℹ Detected addon → use `torresmo addon install redis`
# ℹ Detected addon → use `torresmo addon install postgres`
# ✓ Converted service "web" → Deployment
# ✓ Converted service "api" → Deployment
# ✓ Generated 4 manifests in manifests/
```

Torresmo automatically detects known services (Redis, Postgres, etc.) and suggests using native addons instead.

### 📊 Built-in Dashboard

Monitor your cluster in real-time without external tools:

```bash
torresmo dashboard

🐷 Torresmo Dashboard — cluster: dev          (refreshed: 14:32:05)

Nodes (3)
  NAME               ROLE           STATUS   ADDRESS
  dev-control-plane  control-plane  Running  10.244.0.2
  dev-worker-1       worker         Running  10.244.0.3
  dev-worker-2       worker         Running  10.244.0.4

Pods (4)
  NAMESPACE  NAME          IMAGE                   STATUS   NODE
  default    redis-0       redis:7.2-alpine        Running  dev-worker-1
  default    postgres-0    postgres:16-alpine      Running  dev-worker-2
  default    prometheus-0  prom/prometheus:v2.51.0  Running  dev-worker-1
  default    grafana-0     grafana/grafana:10.4.0  Running  dev-worker-2
```

---

## More Features

| Feature | Command |
|---|---|
| Hot-reload dev mode | `torresmo dev start --pod my-api --sync ./src:/app/src` |
| Mock external APIs | `torresmo mock create -f stripe-api.yaml` |
| Benchmark services | `torresmo benchmark redis --duration 10s` |
| Port forwarding | `torresmo proxy redis-0 6379:6379` |
| Expose to internet | `torresmo tunnel my-api --port 8080 --provider cloudflared` |
| System diagnostics | `torresmo doctor` |
| Resource cleanup | `torresmo cleanup --all` |
| Community addons | `torresmo addon install --from-manifest custom-addon.yaml` |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/getting-started/installation)
- [Ollama](https://ollama.ai) (optional, for AI agent mode)

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap franchialan/torresmo
brew install torresmo
```

### Shell script

```bash
curl -fsSL https://raw.githubusercontent.com/franchialan/torresmo/main/scripts/install.sh | bash
```

### From source

```bash
go install github.com/torresmo-dev/torresmo/cmd/torresmo@latest
```

## Quick Start

```bash
# 1. Check your system
torresmo doctor

# 2. Create a cluster with a template
torresmo create cluster -t go-api --name dev

# 3. Check what's running
torresmo get pods
torresmo dashboard

# 4. Interact with services
torresmo exec redis-0 -- redis-cli ping
torresmo exec postgres-0 -- psql -U torresmo -c "SELECT version();"

# 5. Deploy your app
torresmo apply -f deployment.yaml

# 6. Run pipelines
torresmo pipeline run

# 7. Clean up when done
torresmo delete cluster dev
```

## Available Templates

| Template | Description | Addons |
|---|---|---|
| `minimal` | Single-node, no addons | — |
| `go-api` | Go API backend | Redis, Postgres, Prometheus |
| `node-fullstack` | Node.js + React | Redis, Postgres |
| `java-spring` | Spring Boot microservices | Postgres, Prometheus, Grafana |
| `python-ml` | Python ML/Data stack | Postgres, Redis |
| `microservices` | Service mesh + tracing | Istio, Jaeger, Prometheus, Grafana |
| `data-pipeline` | Streaming + storage | Kafka, Postgres, Redis, MinIO |
| `fullstack` | Full monitoring stack | Redis, Postgres, Prometheus, Grafana |

## Documentation

- [Getting Started](docs/getting-started.md)
- [CLI Reference](docs/cli-reference.md)
- [Dashboard](docs/dashboard.md)
- [Torresmofile Guide](docs/torresmofile.md)
- [Addons](docs/addons.md)

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT
