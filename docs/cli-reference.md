# CLI Reference

## Global Flags

| Flag | Description |
|------|-------------|
| `--cluster <name>` | Target cluster (defaults to current) |
| `--namespace <name>` | Target namespace (defaults to `default`) |
| `--output <format>` | Output format: `table`, `json`, `yaml` |
| `--verbose` | Enable verbose output |

## Cluster Management

```
torresmo create cluster [--name <name>] [--nodes <n>] [--runtime docker|podman] [--profile <profile>]
```
Create a new cluster.

```
torresmo delete cluster <name>
```
Delete a cluster and all its resources.

```
torresmo get clusters
```
List all clusters.

```
torresmo get nodes [--cluster <name>]
```
List nodes in a cluster.

## Environment

```
torresmo up [-f <file>]
```
Set up a complete development environment from `torresmo-env.yaml` (or a custom file).

```
torresmo cleanup [--all]
```
Remove all Torresmo resources and clean up.

```
torresmo doctor
```
Check system requirements and diagnose issues.

## Resource Management

```
torresmo apply -f <file>
```
Apply a manifest file to the cluster.

```
torresmo get pods [--namespace <ns>]
```
List pods.

```
torresmo get services [--namespace <ns>]
```
List services.

```
torresmo get deployments [--namespace <ns>]
```
List deployments.

```
torresmo get namespaces
```
List namespaces.

```
torresmo create namespace <name>
```
Create a namespace in the current cluster.

```
torresmo delete namespace <name>
```
Delete a namespace.

```
torresmo delete pod <name> [--namespace <ns>]
```
Delete a pod.

## Addons

```
torresmo addon install <name> [--version <ver>] [--namespace <ns>] [--config key=value...]
```
Install an addon.

```
torresmo addon remove <name>
```
Remove an addon.

```
torresmo addon list
```
List installed addons.

```
torresmo addon search <query>
```
Search available addons in the registry.

```
torresmo addon publish
```
Publish an addon to the registry.

## Pipelines

```
torresmo pipeline run [--target <name>] [--file <Torresmofile>]
```
Run a pipeline from a Torresmofile.

```
torresmo pipeline list
```
List pipeline runs.

## Dev Sessions

```
torresmo dev start [-f <file>] [--deployment <name>]
```
Start a dev session with file sync and hot-reload.

```
torresmo dev stop
```
Stop the current dev session.

## Debugging & Logs

```
torresmo logs <pod> [--follow] [--tail <n>] [--namespace <ns>]
```
View pod logs.

```
torresmo exec [-it] <pod> -- <command>
```
Execute a command in a pod.

```
torresmo events [--namespace <ns>]
```
View cluster events.

## Networking

```
torresmo proxy [--port <port>]
```
Start a local proxy to cluster services.

```
torresmo tunnel [--port <port>]
```
Expose local services via a public tunnel.

## Snapshots

```
torresmo snapshot create [--name <name>]
```
Create a snapshot of the current cluster.

```
torresmo snapshot list
```
List available snapshots.

```
torresmo snapshot restore <name>
```
Restore a cluster from a snapshot.

```
torresmo snapshot delete <name>
```
Delete a snapshot.

```
torresmo snapshot export <name> [--output <file>]
```
Export a snapshot to a file.

```
torresmo snapshot import <file>
```
Import a snapshot from a file.

## Sharing

```
torresmo share export [--output <file>]
```
Export cluster state to a portable bundle.

```
torresmo share import <file>
```
Import cluster state from a bundle.

## Templates

```
torresmo template list
```
List available environment templates.

```
torresmo template save <name>
```
Save current environment as a template.

```
torresmo template export <name> [--output <file>]
```
Export a template to a file.

```
torresmo template import <file>
```
Import a template from a file.

## Profiles

```
torresmo profile list
```
List available resource profiles.

```
torresmo profile set <name>
```
Set the active profile for the current cluster.

```
torresmo profile create <name> [--cpu <limit>] [--memory <limit>]
```
Create a custom resource profile.

## Docker Compose Compatibility

```
torresmo compose import <file>
```
Import a Docker Compose file as Torresmo manifests.

```
torresmo compose up [-f <file>]
```
Start services from a Docker Compose file.

## Mock Services

```
torresmo mock create [--name <name>] [--port <port>]
```
Create a mock service.

```
torresmo mock list
```
List mock services.

```
torresmo mock logs <name>
```
View mock service logs.

```
torresmo mock delete <name>
```
Delete a mock service.

## Testing & Benchmarks

```
torresmo test [--tags <tags>] [--namespace <ns>]
```
Run tests against the cluster.

```
torresmo benchmark [--duration <time>] [--concurrency <n>]
```
Run performance benchmarks on the cluster.

## AI Agent Mode

```
torresmo "<natural language command>"
```
Execute a natural language command via Ollama.

```
torresmo chat
```
Start an interactive AI chat session.

## Utilities

```
torresmo dashboard
```
Open the Torresmo dashboard (TUI).

```
torresmo version
```
Print the version of torresmo.
