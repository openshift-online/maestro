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

## Quick Start

### Development Environment (with PostgreSQL and MQTT)

```bash
# Deploy server with embedded database and MQTT
make deploy-dev

# Deploy agent (after creating a consumer)
make deploy-agent CONSUMER_NAME=cluster1
```

### Production Environment

For production, you'll need to:

1. Set up external PostgreSQL database
2. Set up MQTT broker or use gRPC
3. Create required secrets
4. Install the charts

```bash
# Create database secret
kubectl create secret generic maestro-rds \
  --from-literal=db.host=<db-host> \
  --from-literal=db.port=5432 \
  --from-literal=db.name=maestro \
  --from-literal=db.user=<db-user> \
  --from-literal=db.password=<db-password> \
  --from-literal=db.ca_cert=<ca-cert-content> \
  -n maestro

# Create MQTT secret (or configure gRPC)
kubectl create secret generic maestro-mqtt \
  --from-literal=config.yaml='<mqtt-config>' \
  -n maestro

# Deploy server
make deploy

# Deploy agent on target cluster
make deploy-agent CONSUMER_NAME=cluster1
```

## Makefile Targets

The following `make` targets are available for managing Helm deployments:

| Target | Description |
|--------|-------------|
| `make deploy` | Deploy maestro-server (requires external DB/MQTT) |
| `make deploy-dev` | Deploy maestro-server with embedded PostgreSQL and MQTT |
| `make deploy-agent` | Deploy maestro-agent (requires CONSUMER_NAME) |
| `make undeploy` | Undeploy maestro-server |
| `make undeploy-agent` | Undeploy maestro-agent |
| `make lint-charts` | Validate Helm charts |
| `make package-charts` | Package Helm charts |
| `make template-server` | Render server templates (dry-run) |
| `make template-agent` | Render agent templates (dry-run) |

## Migration from OpenShift Templates

If you're currently using the OpenShift templates in the `templates/` directory, here's how to migrate:

### Key Differences

1. **Values Configuration**: Helm uses `values.yaml` instead of template parameters
2. **Template Syntax**: Helm uses Go templates (`{{ }}`) instead of OpenShift template syntax (`${}`)
3. **Installation Method**: Use `helm install` instead of `oc process | oc apply`

### Migration Steps

1. **Export current configuration**:
   ```bash
   # Get current deployment parameters
   oc get deployment maestro -n maestro -o yaml > maestro-deployment-backup.yaml
   ```

2. **Create values file** from your current parameters:
   ```yaml
   # custom-values.yaml
   image:
     registry: <your-registry>
     repository: <your-repository>
     tag: <your-tag>

   replicas: 3

   database:
     secretName: maestro-rds
     sslMode: verify-full

   messageBroker:
     type: mqtt
     secretName: maestro-mqtt
   ```

3. **Uninstall old deployment** (optional):
   ```bash
   make undeploy
   ```

4. **Install using Helm**:
   ```bash
   helm install maestro-server ./charts/maestro-server \
     --namespace maestro \
     --values custom-values.yaml
   ```

### Parameter Mapping

| OpenShift Template | Helm Values |
|-------------------|-------------|
| `IMAGE_REGISTRY` | `image.registry` |
| `IMAGE_REPOSITORY` | `image.repository` |
| `IMAGE_TAG` | `image.tag` |
| `SERVER_REPLICAS` | `replicas` |
| `KLOG_V` | `logging.klogV` |
| `DB_SSLMODE` | `database.sslMode` |
| `MESSAGE_DRIVER_TYPE` | `messageBroker.type` |
| `ENABLE_GRPC_SERVER` | `server.grpc.enabled` |
| `CONSUMER_NAME` | `consumerName` (agent only) |

## Chart Development

### Linting

```bash
make helm/lint
```

### Testing

```bash
# Dry-run template rendering
make helm/template-server
make helm/template-agent

# Install in test namespace
helm install test-server ./charts/maestro-server \
  --namespace maestro-test \
  --create-namespace \
  --dry-run --debug
```

### Packaging

```bash
make helm/package
```

This will create `.tgz` files in the `charts/` directory that can be uploaded to a Helm repository.

## Contributing

When modifying the charts:

1. Update the `Chart.yaml` version following [Semantic Versioning](https://semver.org/)
2. Update the `README.md` in the chart directory
3. Run `make helm/lint` to validate
4. Test the changes in a development environment

## Resources

- [Helm Documentation](https://helm.sh/docs/)
- [Maestro GitHub Repository](https://github.com/openshift-online/maestro)
- [OpenShift Templates Documentation](https://docs.openshift.com/container-platform/latest/openshift_images/using-templates.html)
