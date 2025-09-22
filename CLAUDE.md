# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Maestro is a system that leverages CloudEvents to transport Kubernetes resources to target clusters and return status updates. It consists of two main components:
- **Maestro Server**: Stores resources and their status in a database, sends resources to message brokers via CloudEvents, provides REST and gRPC APIs
- **Maestro Agent**: Receives resources, applies them to target clusters, reports back resource status

The system supports both MQTT and gRPC message brokers and is designed to scale to 200,000+ clusters without linear infrastructure scaling.

## Common Development Commands

### Building and Installation
```bash
# Build the project
make binary

# Install binary to GOPATH
make install

# Build all command binaries
make cmds
```

### Testing
```bash
# Run unit tests
make test

# Run integration tests (both MQTT and gRPC)
make test-integration

# Run specific integration tests
make test-integration-mqtt
make test-integration-grpc

# Run end-to-end tests
make e2e-test
```

### Code Quality
```bash
# Verify source code formatting and go version
make verify

# Run linter (requires golangci-lint)
make lint
```

### Local Development Infrastructure
```bash
# Set up local PostgreSQL database
make db/setup

# Set up local MQTT broker
make mqtt/setup

# Login to database
make db/login

# Teardown database
make db/teardown

# Teardown MQTT broker
make mqtt/teardown
```

### Running the Application
```bash
# Run migrations and start server
make run

# Start API documentation server
make run/docs
```

### OpenAPI Generation
```bash
# Regenerate OpenAPI client models and code
make generate
```

### Container and Deployment
```bash
# Build container image
make image

# Push image to registry
make push

# Deploy to OpenShift/Kubernetes
make deploy

# Deploy agent
make deploy-agent

# Undeploy
make undeploy
```

## Architecture

### Core Components
- **cmd/maestro/**: Main application entry point with subcommands for server, agent, and migrations
- **pkg/api/**: REST API models, presenters, and OpenAPI generated clients
- **pkg/controllers/**: Event processing controllers and framework
- **pkg/client/**: CloudEvents clients for MQTT and gRPC communication
- **pkg/config/**: Configuration management for database, message brokers, and servers
- **pkg/auth/**: Authentication and authorization middleware

### Key Concepts
- **Resources**: Kubernetes manifests stored in the database and transported via CloudEvents
- **Consumers**: Represent target clusters that receive resources
- **Resource Bundles**: Groups of Kubernetes resources with metadata and feedback rules
- **Events**: CloudEvents for resource delivery and status updates

### Database Models
The system uses PostgreSQL with GORM for:
- `resources`: Storage of Kubernetes manifests and metadata
- `events`: CloudEvents history and processing status
- `migrations`: Database schema versioning

### Message Brokers
Supports two message broker types:
- **MQTT**: Uses Eclipse Mosquitto with topic-based routing
- **gRPC**: Direct gRPC communication between server and agents

### Configuration
- Environment-based configuration via `MAESTRO_ENV` (development/testing/production)
- MQTT configuration via `--mqtt-config-file` flag
- Database connection via environment variables or command flags
- Authentication via JWT tokens and Red Hat SSO integration

## Testing Strategy
- Unit tests in `pkg/` directories alongside source code
- Integration tests in `test/integration/` for MQTT and gRPC brokers
- End-to-end tests in `test/e2e/` with setup/teardown scripts
- Performance tests in `test/performance/`

## Development Environment
- Go 1.24.4+ required
- PostgreSQL 17.2 for database
- Eclipse Mosquitto 2.0.18 for MQTT broker (if using MQTT mode)
- OpenShift CLI (`oc`) for deployment
- Container tool (podman or docker) for building images