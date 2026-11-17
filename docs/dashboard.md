# Dashboard

## Overview

Torresmo includes a built-in dashboard for monitoring your cluster in real-time. It operates in two modes:

- **TUI Mode** — A terminal-based dashboard that runs directly in your shell with auto-refresh.
- **Web Mode** — A browser-based dashboard with interactive features like live log streaming and in-browser terminal.

No external tools or dashboards to install — it's all part of the `torresmo` binary.

## Quick Start

```bash
# Terminal dashboard
torresmo dashboard

# Web dashboard — opens in your browser
torresmo dashboard --web
```

---

## TUI Mode

The terminal dashboard provides a real-time overview of your cluster directly in the terminal.

```bash
torresmo dashboard [--cluster <name>] [--refresh <seconds>]
```

Features:
- Cluster overview with node, pod, service, and addon counts
- Color-coded status indicators
- Auto-refresh at a configurable interval (default: 5 seconds)
- Works over SSH and in remote terminals

Example output:

```
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

Services (4)
  NAMESPACE  NAME        TYPE       CLUSTER-IP    PORT(S)
  default    redis       ClusterIP  10.244.0.11   6379
  default    postgres    ClusterIP  10.244.0.12   5432
  default    prometheus  ClusterIP  10.244.0.13   9090
  default    grafana     ClusterIP  10.244.0.14   3000
```

---

## Web Mode

The web dashboard provides a rich, interactive browser-based experience.

```bash
torresmo dashboard --web [--port <port>] [--cluster <name>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9876` | Port for the web server |
| `--cluster` | Current | Target cluster |

This starts a local web server and opens `http://localhost:9876` in your default browser.

---

## Dashboard Sections

### Cluster Overview

The top of the dashboard shows stat cards with key metrics at a glance:

- **Nodes** — Total number of cluster nodes
- **Pods** — Total running pods across all namespaces
- **Services** — Total services
- **Addons** — Installed addons

### Nodes Table

Displays all cluster nodes with:

| Column | Description |
|--------|-------------|
| Name | Node hostname |
| Role | `control-plane` or `worker` |
| Status | Current status with color indicator |
| Address | Node IP address |

### Pods Table

Lists all pods across namespaces. **Click any pod to open the Pod Detail Panel** with live logs and an interactive terminal.

| Column | Description |
|--------|-------------|
| Namespace | Pod namespace |
| Name | Pod name (clickable in web mode) |
| Image | Container image |
| Status | Running, Pending, Failed, etc. |
| Node | Node where the pod is scheduled |

### Services Table

Lists all services in the cluster.

| Column | Description |
|--------|-------------|
| Namespace | Service namespace |
| Name | Service name |
| Type | ClusterIP, NodePort, etc. |
| Cluster IP | Internal IP |
| Ports | Exposed ports |

### Addons Table

Shows all installed addons and their status.

| Column | Description |
|--------|-------------|
| Name | Addon name |
| Namespace | Installation namespace |
| Version | Addon version |
| Status | Running, Stopped, etc. |

### Pipelines

The Pipelines tab shows:

- **Pipeline history** — Past runs with status, duration, and target results
- **Live progress** — When a pipeline is running, target steps stream in real-time via SSE (Server-Sent Events)

Each pipeline run shows:
- Overall status (success/failed)
- Start time and total duration
- Per-target status with individual durations
- Source Torresmofile path

### Recent Events

The Events tab shows a chronological feed of cluster events:

- Timestamp
- Event type (Normal / Warning) with color coding
- Resource type and name
- Event reason and message

---

## Pod Detail Panel

In the web dashboard, clicking any pod in the Pods table opens a detail panel with three sections:

### Pod Info

Displays detailed metadata about the pod:

- **Namespace** and **Name**
- **Container image**
- **Node** where the pod is running
- **Pod IP** address
- **Labels** as key-value pairs
- **Exposed ports**
- **Status** and restart count

### Live Logs

Stream pod logs in real-time directly in the browser.

- Logs are delivered via **Server-Sent Events (SSE)** for zero-latency streaming
- Auto-scrolls to the latest output
- Supports the same options as the CLI `logs` command (follow, tail, timestamps)

### Interactive Terminal

Execute commands inside the pod directly from the browser.

- Type a command and press Enter to execute
- Output is displayed inline
- Equivalent to `torresmo exec <pod> -- <command>`

---

## Pipeline Live Progress

When a pipeline is running, the Pipelines tab shows a live progress view:

- Each target step appears as it starts
- Status updates stream in real-time via SSE
- Duration counters update live
- Final result appears when the pipeline completes

This lets you monitor `torresmo pipeline run` from the browser without switching to the terminal.

---

## API Reference

The web dashboard is backed by a REST API. All endpoints return JSON unless noted.

### Overview

```
GET /api/overview
```

Returns cluster summary stats (node count, pod count, service count, addon count).

### Cluster

```
GET /api/cluster
```

Returns cluster metadata (name, status, creation time, profile).

### Nodes

```
GET /api/nodes
```

Returns a list of all nodes with role, status, and address.

### Pods

```
GET /api/pods
```

Returns a list of all pods across namespaces.

### Services

```
GET /api/services
```

Returns a list of all services.

### Deployments

```
GET /api/deployments
```

Returns a list of all deployments.

### Namespaces

```
GET /api/namespaces
```

Returns a list of all namespaces.

### Addons

```
GET /api/addons
```

Returns a list of installed addons with status and version.

### Events

```
GET /api/events
```

Returns recent cluster events.

### Pod Detail

```
GET /api/pods/{namespace}/{name}
```

Returns detailed information about a specific pod, including labels, ports, IP, node, and status.

### Pod Logs (SSE)

```
GET /api/pods/{namespace}/{name}/logs
```

Streams pod logs in real-time using Server-Sent Events. Connect with an `EventSource` in the browser or `curl`:

```bash
curl -N http://localhost:9876/api/pods/default/redis-0/logs
```

### Pod Exec

```
POST /api/pods/{namespace}/{name}/exec
```

Execute a command inside a pod. Request body:

```json
{
  "command": "redis-cli ping"
}
```

Response:

```json
{
  "output": "PONG\n",
  "exit_code": 0
}
```

### Pipelines

```
GET /api/pipelines
```

Returns pipeline run history.

### Pipeline Targets

```
GET /api/pipelines/{id}/targets
```

Returns target details for a specific pipeline run.

### Pipeline Live (SSE)

```
GET /api/pipelines/live
```

Streams live pipeline progress updates via Server-Sent Events. Events include target start, status change, and pipeline completion.
