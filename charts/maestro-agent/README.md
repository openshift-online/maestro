# Maestro Agent Helm Chart

This Helm chart deploys the Maestro Agent, which receives Kubernetes resources via CloudEvents and applies them to the target cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Maestro Server deployed and accessible
- MQTT broker or gRPC server configured (depending on message broker type)

## Installation

### Basic Installation with MQTT

```bash
helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=cluster1 \
  --set messageBroker.type=mqtt \
  --set messageBroker.mqtt.host=mqtt.example.com \
  --set messageBroker.mqtt.port=1883
```

### Installation with gRPC

```bash
helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=cluster1 \
  --set messageBroker.type=grpc \
  --set messageBroker.grpc.url=maestro-grpc.maestro:8090
```

### Installation with MQTT and TLS

```bash
helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=cluster1 \
  --set messageBroker.type=mqtt \
  --set messageBroker.mqtt.host=mqtt.example.com \
  --set messageBroker.mqtt.port=8883 \
  --set messageBroker.mqtt.user=maestro \
  --set messageBroker.mqtt.password=<password> \
  --set messageBroker.mqtt.rootCert=/path/to/ca.crt \
  --set messageBroker.mqtt.clientCert=/path/to/client.crt \
  --set messageBroker.mqtt.clientKey=/path/to/client.key
```

## Uninstallation

```bash
helm uninstall maestro-agent --namespace maestro-agent
```

## Configuration

The following table lists the configurable parameters and their default values.

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `environment` | Maestro environment | `production` |
| `consumerName` | Consumer/cluster name (required) | `cluster1` |
| `replicas` | Number of agent replicas | `1` |

### Image Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Image registry | `image-registry.openshift-image-registry.svc:5000` |
| `image.repository` | Image repository | `maestro/maestro` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Message Broker Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `messageBroker.type` | Message broker type (mqtt/grpc) | `mqtt` |

### MQTT Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `messageBroker.mqtt.host` | MQTT broker host | `""` |
| `messageBroker.mqtt.port` | MQTT broker port | `1883` |
| `messageBroker.mqtt.user` | MQTT username | `""` |
| `messageBroker.mqtt.password` | MQTT password | `""` |
| `messageBroker.mqtt.rootCert` | CA certificate path | `""` |
| `messageBroker.mqtt.clientCert` | Client certificate path | `""` |
| `messageBroker.mqtt.clientKey` | Client key path | `""` |

### gRPC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `messageBroker.grpc.url` | gRPC server URL | `maestro-grpc-broker.maestro:8091` |

### RBAC Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `maestro-agent-sa` |

### CRD Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.create` | Create AppliedManifestWork CRD | `true` |

### Logging Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `logging.klogV` | Log verbosity level | `"2"` |

## Examples

### Production Deployment with MQTT

```bash
# First, create a secret with MQTT credentials if needed
kubectl create secret generic mqtt-credentials \
  --from-literal=username=maestro-agent \
  --from-literal=password=<secure-password> \
  -n maestro-agent

helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=production-cluster-001 \
  --set messageBroker.type=mqtt \
  --set messageBroker.mqtt.host=mqtt.production.example.com \
  --set messageBroker.mqtt.port=8883 \
  --set messageBroker.mqtt.user=maestro-agent \
  --set messageBroker.mqtt.password=<secure-password> \
  --set logging.klogV=4
```

### Production Deployment with gRPC

```bash
helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=production-cluster-001 \
  --set messageBroker.type=grpc \
  --set messageBroker.grpc.url=maestro-grpc.maestro.svc.cluster.local:8090 \
  --set logging.klogV=2
```

### Development Deployment

```bash
helm install maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --create-namespace \
  --set consumerName=dev-cluster \
  --set messageBroker.type=mqtt \
  --set messageBroker.mqtt.host=maestro-mqtt.maestro \
  --set messageBroker.mqtt.port=1883 \
  --set logging.klogV=4
```

## Upgrading

```bash
helm upgrade maestro-agent ./charts/maestro-agent \
  --namespace maestro-agent \
  --set image.tag=v0.2.0
```

## Important Notes

1. **Consumer Name**: The `consumerName` parameter must be unique for each cluster and match the consumer name registered in the Maestro Server.

2. **Message Broker Configuration**: Ensure the agent can reach the MQTT broker or gRPC server configured in the Maestro Server.

3. **RBAC Permissions**: The agent is deployed with cluster-admin permissions by default to allow it to create any resource type. Review and adjust RBAC permissions based on your security requirements.

4. **CRD Installation**: The `AppliedManifestWork` CRD is installed automatically. If you're upgrading or the CRD already exists, set `crds.create=false`.

## Troubleshooting

### Agent Cannot Connect to Broker

Check the agent logs:
```bash
kubectl logs -n maestro-agent deployment/maestro-agent
```

Verify broker configuration:
```bash
kubectl get secret -n maestro-agent maestro-agent-mqtt -o yaml
# or
kubectl get secret -n maestro-agent maestro-agent-grpc -o yaml
```

### Resources Not Being Applied

1. Check agent logs for errors
2. Verify the consumer name matches what's registered in Maestro Server
3. Ensure RBAC permissions are correctly configured
4. Check AppliedManifestWork resources:
   ```bash
   kubectl get appliedmanifestworks
   ```

## Source Code

- [GitHub - openshift-online/maestro](https://github.com/openshift-online/maestro)
