# GitHub Actions Workflows

Run your GitHub Actions workflows locally with Torresmo — test your CI/CD pipelines before pushing.

## Overview

Torresmo integrates with [act](https://github.com/nektos/act) to execute `.github/workflows/*.yml` files locally inside Docker containers. This means you can:

- Test workflow changes before pushing to GitHub
- Debug failing CI jobs on your machine
- Run workflows without consuming GitHub Actions minutes
- Pass secrets and environment variables locally

## Prerequisites

Torresmo uses `act` to run workflows. Install it for best performance:

```bash
brew install act    # macOS
```

If `act` is not installed, Torresmo will automatically run it inside a Docker container (slower first time due to image download).

## Quick Start

```bash
# List available workflows
torresmo workflow list

# Run all workflows
torresmo workflow run

# Run a specific workflow
torresmo workflow run --file ci.yml

# Run a specific job
torresmo workflow run --file ci.yml --job build

# Dry run (show what would execute)
torresmo workflow run --file ci.yml --dry-run
```

## Authentication & GitHub Token

Many GitHub Actions require a `GITHUB_TOKEN` to:
- Clone action repositories (e.g., `actions/checkout`, `actions/setup-go`)
- Access private repositories
- Interact with the GitHub API

### When do you need a PAT?

| Scenario | Token Required? |
|---|---|
| Workflow uses `actions/checkout` | ⚠️ Sometimes (act may need it to clone actions) |
| Workflow uses `actions/setup-go`, `actions/setup-node` | ⚠️ Sometimes |
| Workflow pushes to a repository | ✅ Yes |
| Workflow creates releases | ✅ Yes |
| Workflow only runs shell commands | ❌ No |
| Dry run mode | ❌ No (usually) |

### Creating a Personal Access Token (PAT)

1. Go to [GitHub Settings → Tokens](https://github.com/settings/tokens/new)
2. Select scopes:
   - `repo` — for accessing repositories
   - `workflow` — for workflow operations
3. Generate and copy the token

### Passing the token to Torresmo

```bash
# Method 1: Pass as secret (recommended)
torresmo workflow run --secret GITHUB_TOKEN=ghp_your_token_here

# Method 2: Environment variable
export GITHUB_TOKEN=ghp_your_token_here
torresmo workflow run

# Method 3: .secrets file (act convention)
echo "GITHUB_TOKEN=ghp_your_token_here" > .secrets
torresmo workflow run
# Note: Add .secrets to .gitignore!
```

> ⚠️ **Security**: Never commit tokens to your repository. Use `.secrets` files (added to `.gitignore`) or environment variables.

## Command Reference

### `torresmo workflow list`

List all discovered workflows in `.github/workflows/`:

```bash
torresmo workflow list
# NAME                    FILE          JOBS
# CI                      ci.yaml       build, e2e, lint
# Deploy to GitHub Pages  pages.yaml    deploy
# Release                 release.yaml  release
```

### `torresmo workflow run`

Run workflows locally:

```bash
# All flags:
torresmo workflow run [workflow-file] \
  --file ci.yml          # specific workflow file
  --job build            # specific job to run
  --event push           # trigger event (push, pull_request, etc.)
  --verbose              # detailed output
  --dry-run              # show steps without executing
  --secret KEY=VALUE     # pass secrets (repeatable)
  --env KEY=VALUE        # pass env vars (repeatable)
```

## Examples

### Example 1: Running a Go CI workflow

```yaml
# .github/workflows/ci.yml
name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./...
      - run: go vet ./...
```

```bash
# Run the test job locally
torresmo workflow run --file ci.yml --job test --secret GITHUB_TOKEN=$GITHUB_TOKEN
```

### Example 2: Running with environment variables

```bash
torresmo workflow run --file ci.yml \
  --env DATABASE_URL=postgres://localhost:5432/test \
  --env REDIS_URL=localhost:6379
```

### Example 3: Testing a release workflow (dry run)

```bash
torresmo workflow run --file release.yml --dry-run
```

### Example 4: Running a pull_request event

```bash
torresmo workflow run --file ci.yml --event pull_request
```

## Dashboard Integration

Workflow runs are automatically tracked and visible in the Torresmo dashboard:

```bash
torresmo dashboard --web
```

The ⚡ Workflows section shows:
- Run history with status (✅ success / ❌ failed / 🔄 running)
- Workflow file, job name, and trigger event
- Duration
- **Click a run to expand and view full logs**

## Troubleshooting

### "authentication required" error
This means act needs a GitHub token to clone action repositories:
```bash
torresmo workflow run --secret GITHUB_TOKEN=ghp_your_token
```

### "Apple M-series chip" warning
This is a known act warning on Apple Silicon. If you encounter issues:
```bash
act --container-architecture linux/amd64
```
Or pass it via Torresmo:
```bash
torresmo workflow run --env ACT_CONTAINER_ARCHITECTURE=linux/amd64
```

### Workflow works on GitHub but fails locally
Some differences between GitHub-hosted runners and act:
- `act` uses different base images than GitHub
- Some GitHub-specific features may not be available
- Network access may differ

### act not found
```bash
# Install act
brew install act          # macOS
# Or Torresmo will use a Docker container (slower)
```

## See Also

- [act documentation](https://github.com/nektos/act)
- [GitHub Actions documentation](https://docs.github.com/en/actions)
- [Torresmo Pipelines](torresmofile.md) — native Torresmo pipelines via Torresmofile
- [Dashboard](dashboard.md) — view workflow runs in the dashboard
