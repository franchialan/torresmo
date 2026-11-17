# Addons

Addons are pre-configured, production-grade services that you can install into any torresmo cluster with a single command. Instead of writing Dockerfiles, Helm charts, or Kubernetes manifests, you run `torresmo addon install <name>` and get a fully configured service — ready for your application to connect to.

Each addon creates a **Pod** and a **ClusterIP Service** inside your cluster. The service provides a stable DNS name (`<addon>.<namespace>`) that your applications use to connect, regardless of pod restarts or rescheduling.

Torresmo ships with **10 built-in addons** covering databases, caching, messaging, monitoring, tracing, and infrastructure. You can also create and install **community addons** using a simple YAML manifest format.

---

## Quick Start

```bash
# 1. Create a cluster (if you don't have one)
torresmo create cluster --name dev

# 2. Install an addon
torresmo addon install redis

# 3. Verify it's running
torresmo exec redis-0 -- redis-cli ping
# → PONG
```

That's it. Redis is running, accessible at `redis.default:6379` from any pod in the cluster.

---

## Built-in Addons Reference

### Redis

In-memory data store for caching and messaging.

| Property | Value |
|---|---|
| Image | `redis:{version}-alpine` |
| Default version | `7.2` |
| Available versions | `7.2`, `7.0`, `6.2` |
| Port | 6379 |

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `maxmemory` | `256mb` | Maximum memory Redis will use. Accepts values like `128mb`, `1gb`, etc. |

Redis starts with the `allkeys-lru` eviction policy, which automatically evicts the least recently used keys when `maxmemory` is reached.

**Install with defaults:**

```bash
torresmo addon install redis
```

**Install with config overrides:**

```bash
torresmo addon install redis --version 7.0 --config maxmemory=512mb
```

**Verify it's working:**

```bash
torresmo exec redis-0 -- redis-cli ping
# → PONG
```

**Connection string for your app:**

```
redis://redis.<namespace>:6379
```

For example, if installed in the `default` namespace:

```
redis://redis.default:6379
```

---

### PostgreSQL

PostgreSQL relational database.

| Property | Value |
|---|---|
| Image | `postgres:{version}-alpine` |
| Default version | `16` |
| Available versions | `16`, `15`, `14` |
| Port | 5432 |

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `user` | `torresmo` | PostgreSQL superuser name (sets `POSTGRES_USER`) |
| `password` | `torresmo` | Superuser password (sets `POSTGRES_PASSWORD`) |
| `database` | `torresmo` | Default database created on init (sets `POSTGRES_DB`) |

**Install with defaults:**

```bash
torresmo addon install postgres
```

**Install with config overrides:**

```bash
torresmo addon install postgres --version 15 \
  --config user=dev \
  --config password=secret123 \
  --config database=myapp
```

**Verify it's working:**

```bash
torresmo exec postgres-0 -- psql -U torresmo -c "SELECT version();"
```

**Connection string for your app:**

```
postgres://<user>:<password>@postgres.<namespace>:5432/<database>
```

With defaults in the `default` namespace:

```
postgres://torresmo:torresmo@postgres.default:5432/torresmo
```

---

### MongoDB

NoSQL document database.

| Property | Value |
|---|---|
| Image | `mongo:{version}` |
| Default version | `7.0` |
| Available versions | `7.0`, `6.0`, `5.0` |
| Port | 27017 |

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `root_user` | `torresmo` | Root username (sets `MONGO_INITDB_ROOT_USERNAME`) |
| `root_password` | `torresmo` | Root password (sets `MONGO_INITDB_ROOT_PASSWORD`) |

**Install with defaults:**

```bash
torresmo addon install mongodb
```

**Install with config overrides:**

```bash
torresmo addon install mongodb --version 6.0 \
  --config root_user=admin \
  --config root_password=secret123
```

**Verify it's working:**

```bash
torresmo exec mongodb-0 -- mongosh --eval "db.runCommand({ping:1})"
```

**Connection string for your app:**

```
mongodb://<root_user>:<root_password>@mongodb.<namespace>:27017
```

With defaults in the `default` namespace:

```
mongodb://torresmo:torresmo@mongodb.default:27017
```

---

### Prometheus

Monitoring and alerting toolkit.

| Property | Value |
|---|---|
| Image | `prom/prometheus:{version}` |
| Default version | `v2.51.0` |
| Available versions | `v2.51.0`, `v2.48.0`, `v2.45.0` |
| Port | 9090 |

Prometheus starts with lifecycle API enabled (`--web.enable-lifecycle`) and stores time-series data at `/prometheus`.

**Configuration:**

No configurable parameters. Prometheus is installed with sensible defaults.

**Install:**

```bash
torresmo addon install prometheus
```

**Install a specific version:**

```bash
torresmo addon install prometheus --version v2.48.0
```

**Access the UI:**

```
http://prometheus.<namespace>:9090
```

Use port forwarding to access from your host machine:

```bash
torresmo proxy prometheus-0 9090:9090
# → Open http://localhost:9090
```

---

### Grafana

Observability dashboards and visualization.

| Property | Value |
|---|---|
| Image | `grafana/grafana:{version}` |
| Default version | `10.4.0` |
| Available versions | `10.4.0`, `10.2.0`, `9.5.0` |
| Port | 3000 |

Grafana starts with anonymous access enabled (`GF_AUTH_ANONYMOUS_ENABLED=true`), so you can access dashboards without logging in. Use the admin credentials for configuration changes.

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `admin_user` | `admin` | Admin username (sets `GF_SECURITY_ADMIN_USER`) |
| `admin_password` | `admin` | Admin password (sets `GF_SECURITY_ADMIN_PASSWORD`) |

**Install with defaults:**

```bash
torresmo addon install grafana
```

**Install with config overrides:**

```bash
torresmo addon install grafana --config admin_password=strongpass
```

**Access the UI:**

```
http://grafana.<namespace>:3000
```

Default login: `admin` / `admin`

Use port forwarding to access from your host:

```bash
torresmo proxy grafana-0 3000:3000
# → Open http://localhost:3000
```

**Tip:** Install Prometheus alongside Grafana for a full monitoring stack. Add Prometheus as a data source in Grafana using `http://prometheus.<namespace>:9090`.

---

### Istio

Service mesh for traffic management, security, and observability.

| Property | Value |
|---|---|
| Image | `istio/pilot:{version}` |
| Default version | `1.21.0` |
| Available versions | `1.21.0`, `1.20.0`, `1.19.0` |
| Ports | 15010 (gRPC), 15017 (webhook) |
| Pod name | `istiod-0` |
| Service name | `istiod` |

Istio installs the **istiod** control plane (Pilot), which handles service discovery, configuration, and certificate management.

**Configuration:**

No configurable parameters.

**Install:**

```bash
torresmo addon install istio
```

**Install a specific version:**

```bash
torresmo addon install istio --version 1.20.0
```

**Service endpoints:**

| Port | Name | Protocol | Description |
|---|---|---|---|
| 15010 | grpc | tcp | xDS and CA services |
| 15017 | webhook | tcp | Webhook container port for sidecar injection |

---

### Jaeger

Distributed tracing system.

| Property | Value |
|---|---|
| Image | `jaegertracing/all-in-one:{version}` |
| Default version | `1.57` |
| Available versions | `1.57`, `1.53` |
| Ports | 16686 (UI), 14268 (collector), 4317 (OTLP gRPC) |

Jaeger uses the all-in-one deployment with OTLP collector enabled (`COLLECTOR_OTLP_ENABLED=true`). This means your applications can send traces using either the Jaeger native format or the OpenTelemetry Protocol (OTLP).

**Configuration:**

No configurable parameters.

**Install:**

```bash
torresmo addon install jaeger
```

**Service endpoints:**

| Port | Name | Description |
|---|---|---|
| 16686 | ui | Jaeger Query UI |
| 14268 | collector | Jaeger collector (accepts `jaeger.thrift` over HTTP) |
| 4317 | otlp-grpc | OpenTelemetry Protocol (gRPC) |

**Access the UI:**

```
http://jaeger.<namespace>:16686
```

Use port forwarding:

```bash
torresmo proxy jaeger-0 16686:16686
# → Open http://localhost:16686
```

**Send traces from your app:**

Configure your OpenTelemetry SDK to export to:

```
jaeger.<namespace>:4317  (OTLP gRPC)
```

---

### Kafka

Distributed event streaming platform.

| Property | Value |
|---|---|
| Image | `bitnami/kafka:{version}` |
| Default version | `3.7` |
| Available versions | `3.7`, `3.6` |
| Port | 9092 |

Kafka runs in **KRaft mode** (no Zookeeper dependency). It operates as both a controller and broker in a single node, which is ideal for local development. Plaintext listeners are enabled by default.

**Configuration:**

No configurable parameters. The KRaft-mode environment variables are set automatically:

| Environment Variable | Value | Description |
|---|---|---|
| `KAFKA_CFG_NODE_ID` | `0` | Broker node ID |
| `KAFKA_CFG_PROCESS_ROLES` | `controller,broker` | Combined controller and broker |
| `KAFKA_CFG_CONTROLLER_QUORUM_VOTERS` | `0@localhost:9093` | KRaft quorum configuration |
| `KAFKA_CFG_LISTENERS` | `PLAINTEXT://:9092,CONTROLLER://:9093` | Listener configuration |
| `KAFKA_CFG_CONTROLLER_LISTENER_NAMES` | `CONTROLLER` | Controller listener name |
| `ALLOW_PLAINTEXT_LISTENER` | `yes` | Allow plaintext connections |

**Install:**

```bash
torresmo addon install kafka
```

**Connection string for your app (bootstrap server):**

```
kafka.<namespace>:9092
```

**Example:** Produce and consume messages using a Kafka client:

```bash
# Using kafkacat/kcat from inside the cluster
torresmo exec kafka-0 -- kafka-topics.sh --create --topic test \
  --bootstrap-server localhost:9092

torresmo exec kafka-0 -- kafka-console-producer.sh --topic test \
  --bootstrap-server localhost:9092
```

---

### NATS

Cloud-native messaging system.

| Property | Value |
|---|---|
| Image | `nats:{version}-alpine` |
| Default version | `2.10` |
| Available versions | `2.10`, `2.9` |
| Ports | 4222 (client), 8222 (monitoring) |

NATS starts with **JetStream enabled** (`--jetstream`), giving you persistent messaging, key-value stores, and object stores out of the box.

**Configuration:**

No configurable parameters.

**Install:**

```bash
torresmo addon install nats
```

**Service endpoints:**

| Port | Name | Description |
|---|---|---|
| 4222 | client | Client connections (publish/subscribe) |
| 8222 | monitor | HTTP monitoring endpoint |

**Connection string for your app:**

```
nats://nats.<namespace>:4222
```

**Verify it's working:**

```bash
# Check the monitoring endpoint
torresmo exec nats-0 -- wget -qO- http://localhost:8222/healthz
```

---

### MinIO

S3-compatible object storage.

| Property | Value |
|---|---|
| Image | `minio/minio:{version}` |
| Default version | `latest` |
| Available versions | `latest` |
| Ports | 9000 (API), 9001 (console) |

MinIO provides an S3-compatible API, so you can use any AWS S3 SDK to interact with it. The data is stored at `/data` inside the container.

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `root_user` | `minioadmin` | Root access key (sets `MINIO_ROOT_USER`) |
| `root_password` | `minioadmin` | Root secret key (sets `MINIO_ROOT_PASSWORD`) |

**Install with defaults:**

```bash
torresmo addon install minio
```

**Install with config overrides:**

```bash
torresmo addon install minio \
  --config root_user=myaccesskey \
  --config root_password=mysecretkey
```

**Service endpoints:**

| Port | Name | Description |
|---|---|---|
| 9000 | api | S3-compatible API |
| 9001 | console | Web-based management console |

**Access the console:**

```
http://minio.<namespace>:9001
```

Default login: `minioadmin` / `minioadmin`

Use port forwarding:

```bash
torresmo proxy minio-0 9001:9001
# → Open http://localhost:9001
```

**S3 endpoint for your app:**

```
http://minio.<namespace>:9000
```

Configure your S3 SDK with:

```
Endpoint:   http://minio.default:9000
Access Key: minioadmin
Secret Key: minioadmin
Region:     us-east-1  (any value works)
Use SSL:    false
```

---

## Managing Addons

### Install an addon

```bash
# Install with defaults
torresmo addon install redis

# Specify a version
torresmo addon install postgres --version 15

# Install into a specific namespace
torresmo addon install redis --namespace cache

# Install with configuration overrides
torresmo addon install postgres --config user=dev --config password=secret --config database=myapp

# Combine options
torresmo addon install mongodb --version 6.0 --namespace data --config root_user=admin
```

### List installed addons

```bash
torresmo addon list

# Output:
# NAME        NAMESPACE  VERSION  STATUS
# redis       default    7.2      Running
# postgres    default    16       Running
# prometheus  monitor    v2.51.0  Running
```

### Remove an addon

```bash
torresmo addon remove redis

# Remove from a specific namespace
torresmo addon remove redis --namespace cache
```

This deletes the addon's Pod and Service, and removes the addon record from the store.

### Search available addons

```bash
torresmo addon search

# Output:
# NAME        DESCRIPTION
# grafana     Observability dashboards and visualization
# istio       Service mesh for traffic management, security, and observability
# jaeger      Distributed tracing system
# kafka       Distributed event streaming platform
# minio       S3-compatible object storage
# mongodb     NoSQL document database
# nats        Cloud-native messaging system
# postgres    PostgreSQL relational database
# prometheus  Monitoring and alerting toolkit
# redis       In-memory data store for caching and messaging
```

### Get addon info

```bash
torresmo addon info redis

# Output:
# Name:        redis
# Display:     Redis
# Description: In-memory data store for caching and messaging
# Versions:    7.2 (default), 7.0, 6.2
```

---

## Addons in Cluster Config

Instead of installing addons one by one, declare them in a `torresmo.yaml` file and they'll be installed automatically when the cluster is created:

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
  addons:
    - name: redis
      namespace: cache
    - name: postgres
    - name: prometheus
    - name: grafana
```

Create the cluster with:

```bash
torresmo create cluster -f torresmo.yaml
```

This creates the cluster **and** installs all declared addons in a single operation.

You can also use **templates** that bundle common addon combinations:

```bash
# Go API: Redis + Postgres + Prometheus
torresmo create cluster -t go-api --name dev

# Microservices: Istio + Jaeger + Prometheus + Grafana
torresmo create cluster -t microservices --name dev

# Data pipeline: Kafka + Postgres + Redis + MinIO
torresmo create cluster -t data-pipeline --name dev
```

See the [Getting Started guide](getting-started.md) for the full list of templates and their addon sets.

---

## Community Addons

The community addon system lets anyone create custom addons using a YAML manifest file. Manifests describe an addon's container(s), ports, environment variables, and configurable parameters — everything torresmo needs to install and manage the addon.

### Addon Manifest Format

Manifests use the `torresmo.io/v1` API and `Addon` kind:

```yaml
apiVersion: torresmo.io/v1
kind: Addon
metadata:
  name: elasticsearch         # Addon identifier (used in install commands)
  description: Search engine  # Human-readable description
  author: your-name           # Author or organization
  version: "1.0.0"            # Manifest version

spec:
  # Available versions and default
  versions: ["8.13", "8.12", "7.17"]
  default: "8.13"

  # Container definitions (the {version} placeholder is replaced at install time)
  containers:
    - name: elasticsearch
      image: "elasticsearch:{version}"
      command: []               # Optional: override entrypoint
      args: []                  # Optional: arguments to the entrypoint

  # Ports exposed by the addon
  ports:
    - name: http
      containerPort: 9200
      protocol: tcp
    - name: transport
      containerPort: 9300
      protocol: tcp

  # Environment variables
  # Use "fromConfig" to link a variable to a user-configurable parameter
  env:
    - name: discovery.type
      value: single-node
    - name: ELASTIC_PASSWORD
      value: "changeme"
      fromConfig: "password"     # User can override via --config password=xxx

  # Configurable parameters (exposed via --config key=value)
  config:
    - name: password
      type: string
      default: "changeme"
      description: "Elasticsearch password"

  # Optional: health check endpoint
  healthcheck:
    path: /_cluster/health
    port: 9200
```

### How `fromConfig` Works

The `fromConfig` field on an environment variable links it to a `config` parameter. When a user installs the addon with `--config password=mysecret`, the manifest system:

1. Looks up the config parameter named `password`
2. Finds all `env` entries with `fromConfig: "password"`
3. Sets `ELASTIC_PASSWORD` to `mysecret` instead of the default `changeme`

If the user doesn't provide a `--config` override, the default value from the `config` section is used.

### Creating Your Own Addon

1. **Create the manifest file** (e.g., `my-addon/manifest.yaml`):

```yaml
apiVersion: torresmo.io/v1
kind: Addon
metadata:
  name: my-cache
  description: My custom caching service
  author: my-team
  version: "1.0.0"
spec:
  versions: ["2.0", "1.5"]
  default: "2.0"
  containers:
    - name: my-cache
      image: "my-registry/my-cache:{version}"
  ports:
    - name: cache
      containerPort: 11211
      protocol: tcp
  env:
    - name: CACHE_MAX_MEMORY
      value: "256"
      fromConfig: "max_memory"
  config:
    - name: max_memory
      type: string
      default: "256"
      description: "Max memory in MB"
```

2. **Test locally:**

```bash
torresmo addon install --from-manifest my-addon/manifest.yaml
```

3. **Verify it's running:**

```bash
torresmo addon list
torresmo get pods
```

### Publishing a Community Addon

```bash
torresmo addon publish my-addon/manifest.yaml
```

This makes your addon discoverable to other torresmo users.

### Installing from a Manifest

```bash
# Install from a local manifest file
torresmo addon install --from-manifest ./addons/elasticsearch/manifest.yaml

# Install with version and config overrides
torresmo addon install --from-manifest ./addons/elasticsearch/manifest.yaml \
  --version 8.12 \
  --config password=mysecret
```

### Real-World Manifest Example

Here is the built-in MongoDB addon expressed as a community manifest (from `addons/mongodb/manifest.yaml`):

```yaml
apiVersion: torresmo.io/v1
kind: Addon
metadata:
  name: mongodb
  description: NoSQL document database
  author: torresmo-dev
  version: "1.0.0"
spec:
  versions: ["7.0", "6.0", "5.0"]
  default: "7.0"
  containers:
    - name: mongodb
      image: "mongo:{version}"
  ports:
    - name: mongodb
      containerPort: 27017
      protocol: tcp
  env:
    - name: MONGO_INITDB_ROOT_USERNAME
      value: "torresmo"
      fromConfig: "root_user"
    - name: MONGO_INITDB_ROOT_PASSWORD
      value: "torresmo"
      fromConfig: "root_password"
  config:
    - name: root_user
      type: string
      default: "torresmo"
      description: "Root username"
    - name: root_password
      type: string
      default: "torresmo"
      description: "Root password"
```

---

## Addon Configuration

### How `--config` Works

When you install an addon with `--config key=value`, the value is passed to the addon's `Install` method as part of the `InstallOptions.Config` map. Each addon checks for specific config keys and applies them.

```bash
# Single config value
torresmo addon install redis --config maxmemory=512mb

# Multiple config values
torresmo addon install postgres \
  --config user=dev \
  --config password=secret \
  --config database=myapp
```

### How Config Maps to Environment Variables

Each addon maps config keys to container environment variables internally:

| Addon | Config Key | Environment Variable |
|---|---|---|
| postgres | `user` | `POSTGRES_USER` |
| postgres | `password` | `POSTGRES_PASSWORD` |
| postgres | `database` | `POSTGRES_DB` |
| mongodb | `root_user` | `MONGO_INITDB_ROOT_USERNAME` |
| mongodb | `root_password` | `MONGO_INITDB_ROOT_PASSWORD` |
| grafana | `admin_user` | `GF_SECURITY_ADMIN_USER` |
| grafana | `admin_password` | `GF_SECURITY_ADMIN_PASSWORD` |
| minio | `root_user` | `MINIO_ROOT_USER` |
| minio | `root_password` | `MINIO_ROOT_PASSWORD` |
| redis | `maxmemory` | Passed as `--maxmemory` command-line arg |

For community addons, the config-to-env mapping is defined in the manifest's `env[].fromConfig` field.

### Config Persistence

Addon configuration is stored in the torresmo database. When you run `torresmo addon list`, you can see the installed version and current status. The config is saved as JSON alongside the addon record, so torresmo can track what configuration was used for each installation.

### Quick Reference: All Configurable Addons

| Addon | Configurable Keys |
|---|---|
| redis | `maxmemory` |
| postgres | `user`, `password`, `database` |
| mongodb | `root_user`, `root_password` |
| grafana | `admin_user`, `admin_password` |
| minio | `root_user`, `root_password` |
| prometheus | — (none) |
| istio | — (none) |
| jaeger | — (none) |
| kafka | — (none) |
| nats | — (none) |
