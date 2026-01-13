# Maestro Helm Charts

This directory contains Helm charts for deploying Maestro components.

## Available Charts

### maestro-server
The Maestro Server chart deploys the main server component that:
- Stores resources and their status in a database
- Sends resources to message brokers via CloudEvents
- Provides REST and gRPC APIs

[maestro-server Documentation](./maestro-server/README.md)

### maestro-agent
The Maestro Agent chart deploys the agent component that:
- Receives resources from the server via CloudEvents
- Applies resources to the target cluster
- Reports back resource status

[maestro-agent Documentation](./maestro-agent/README.md)

