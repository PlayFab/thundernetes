# Copilot Instructions for Thundernetes

## Project Overview

Thundernetes makes it easy to run game servers on Kubernetes. It originates from the Azure PlayFab Multiplayer Servers team and enables running both Windows and Linux game servers on Kubernetes clusters. Key capabilities include:

- Game server auto-scaling based on standingBy levels
- Game Server SDK (GSDK) support across multiple languages/environments (Unity, Unreal, C#, C++, Java, Go)
- A REST API and web UI for managing game server deployments
- Prometheus metrics and Grafana dashboard integration
- Support for intelligent standingBy server count forecasting

**Status:** Beta / experimental — not supported for production environments.

## Repository Structure

```
/
├── cmd/
│   ├── gameserverapi/     # REST API service for managing game servers
│   ├── initcontainer/     # Init container that runs before game server containers
│   ├── latencyserver/     # Latency measurement server
│   ├── nodeagent/         # Per-node agent that communicates with game server processes
│   ├── server-load-simulator/  # Load simulator for testing
│   └── standby-forecaster/    # Intelligent standingBy count forecaster
├── pkg/
│   ├── operator/          # Kubernetes operator (controller-runtime based)
│   │   ├── api/v1alpha1/  # CRD API types (GameServer, GameServerBuild, etc.)
│   │   ├── controllers/   # Kubernetes controllers and allocation API
│   │   └── config/        # Configuration
│   └── log/               # Logging utilities
├── e2e/                   # End-to-end tests
├── docs/                  # Documentation (Jekyll site at playfab.github.io/thundernetes)
├── installfiles/          # Generated Kubernetes YAML install manifests
├── samples/               # Sample game server implementations
├── windows/               # Windows-specific configuration
└── build-env/             # Docker build environment
```

## Build & Test

### Prerequisites
- Go 1.24+
- Docker
- kubectl
- kind (for local e2e tests)

### Go Code Validation
```bash
go fmt ./...
go vet ./...
go mod tidy
```

### Unit Tests
```bash
# GameServer API
cd cmd/gameserverapi && GIN_MODE=release go test -race

# Init container
cd cmd/initcontainer && go test -race

# Node agent
cd cmd/nodeagent && go test -race

# Latency server
cd cmd/latencyserver && go test -race

# Operator (uses controller-runtime envtest)
make -C pkg/operator test
```

### Building Docker Images
```bash
make build              # Build all Docker images
make builddockerlocal   # Build and tag images for local use
```

### End-to-End Tests (local, requires kind)
```bash
make installkind
make createkindcluster
make builddockerlocal
make e2elocal
```

### Install File Generation
```bash
make create-install-files      # Production install files
make create-install-files-dev  # Dev install files (debug logging)
```

## Code Conventions

### Language & Frameworks
- **Go** (module: `github.com/playfab/thundernetes`) for all backend services
- **controller-runtime** for the Kubernetes operator
- **Ginkgo/Gomega** for operator tests (`pkg/operator/`)
- **testify** for unit tests in `cmd/`
- **gin** for HTTP APIs (`cmd/gameserverapi/`, allocation API server)

### Go Style
- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `go mod tidy` to keep dependencies clean
- Race condition detection is enabled in tests (`go test -race`)

### Kubernetes Operator Patterns
- CRD types are defined in `pkg/operator/api/v1alpha1/`
- Controllers are in `pkg/operator/controllers/`
- Use controller-runtime reconciliation patterns
- Metrics are exposed via Prometheus (`pkg/operator/controllers/metrics.go`)
- Port registry managed in `pkg/operator/controllers/port_registry.go`

### Key CRD Types
- `GameServer` — represents an individual game server instance
- `GameServerBuild` — manages a set of game servers with auto-scaling
- `GameServerBuild` spec includes `standingBy` and `max` counts

### Error Handling
- Use `github.com/pkg/errors` for error wrapping
- Log errors with context using structured logging

### Logging
- The operator uses `go.uber.org/zap` via controller-runtime's logr interface
- Services use `github.com/sirupsen/logrus`
- Log level is configurable (info for production, debug for dev)

## Pull Request Checklist

Before submitting a PR:
- [ ] New/changed code includes test additions or changes
- [ ] Documentation updated as required
- [ ] CHANGELOG.md updated
- [ ] `go fmt ./...` and `go vet ./...` pass with no changes
- [ ] `go mod tidy` produces no changes
- [ ] Install files regenerated if operator configuration changed (`make create-install-files`)
- [ ] CLA signed (https://cla.opensource.microsoft.com/microsoft/.github)

## Go Version

The project uses Go **1.24**. The Go version must be consistent across:
- `go.mod`
- `build-env/Dockerfile`
- Windows Dockerfiles (`cmd/nodeagent/Dockerfile.win`, `cmd/initcontainer/Dockerfile.win`)
- GitHub Actions workflow files (`.github/workflows/`)
