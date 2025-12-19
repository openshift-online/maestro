# Helm Charts for Maestro

This document describes the Helm charts available for deploying Maestro and how to use them in e2e testing.

## Available Charts

### maestro-server

Helm chart for deploying the Maestro server component, which includes:
- Maestro API server
- Optional embedded PostgreSQL database (for development/testing)
- Optional embedded MQTT broker (for development/testing)
- Services, Routes, and RBAC resources

See [charts/maestro-server/README.md](charts/maestro-server/README.md) for detailed configuration options.

### maestro-agent

Helm chart for deploying the Maestro agent component, which includes:
- Maestro agent deployment
- ServiceAccount and RBAC resources
- AppliedManifestWork CRD

See [charts/maestro-agent/README.md](charts/maestro-agent/README.md) for detailed configuration options.

## E2E Testing with Helm Charts

### Quick Start

Run e2e tests using Helm charts for deployment:

```bash
make e2e-test
```

This will:
1. Clean up any existing test environment
2. Set up a KinD cluster with necessary configurations
3. Deploy PostgreSQL and MQTT/gRPC broker (based on MESSAGE_DRIVER_TYPE)
4. Deploy Maestro server using Helm
5. Create a consumer and deploy Maestro agent using Helm
6. Run the e2e test suite

### Configuration Options

The e2e-test target supports the following environment variables:

```bash
# Run with TLS enabled
ENABLE_MAESTRO_TLS=true make e2e-test

# Run with multiple server replicas
SERVER_REPLICAS=3 make e2e-test

# Run with gRPC message broker
MESSAGE_DRIVER_TYPE=grpc make e2e-test

# Run with broadcast subscription mode
ENABLE_BROADCAST_SUBSCRIPTION=true make e2e-test
```

### Supported Message Brokers

The Helm charts support three message broker types:

1. **MQTT** (default)
   ```bash
   MESSAGE_DRIVER_TYPE=mqtt make e2e-test
   ```

2. **gRPC**
   ```bash
   MESSAGE_DRIVER_TYPE=grpc make e2e-test
   ```

3. **PubSub** (Google Cloud Pub/Sub)
   ```bash
   MESSAGE_DRIVER_TYPE=pubsub make e2e-test
   ```

## Manual Deployment

### Deploy Server

```bash
# Production deployment
make deploy

# Development deployment with embedded PostgreSQL and MQTT
make deploy-dev

# With environment variables
SERVER_REPLICAS=2 make deploy
```

### Deploy Agent

```bash
# Requires server to be deployed first and CONSUMER_NAME to be set
make deploy-agent CONSUMER_NAME=cluster1
```

## Troubleshooting

### Debugging Helm Deployments

View rendered templates without installing:
```bash
helm template maestro-server ./charts/maestro-server \
  --values test/_output/maestro-server-values.yaml \
  --namespace maestro
```

Check Helm release status:
```bash
helm list -n maestro
helm list -n maestro-agent
```

Get values for a deployed release:
```bash
helm get values maestro-server -n maestro
```
