# Maestro Server Helm Chart

This Helm chart deploys the Maestro Server, a CloudEvents-based system for delivering Kubernetes resources to target clusters.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- OpenShift 4.x (optional, for Route support)

## Installation

### Basic Installation

```bash
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace
```

### Installation with Custom Values

```bash
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace \
  --set image.tag=v0.1.0 \
  --set replicas=3
```

### Installation with External Database

For production deployments, you should use an external PostgreSQL database:

```bash
# Create database secret first
kubectl create secret generic maestro-rds \
  --from-literal=db.host=postgres.example.com \
  --from-literal=db.port=5432 \
  --from-literal=db.name=maestro \
  --from-literal=db.user=maestro \
  --from-literal=db.password=<password> \
  --from-literal=db.ca_cert=<ca-cert-content> \
  -n maestro

# Create MQTT broker config secret
kubectl create secret generic maestro-mqtt \
  --from-literal=config.yaml='
brokerHost: mqtt.example.com:1883
username: maestro
password: <mqtt-password>
topics:
  sourceEvents: sources/maestro/consumers/+/sourceevents
  agentEvents: sources/maestro/consumers/+/agentevents
' \
  -n maestro

# Install the chart
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace \
  --set postgresql.enabled=false \
  --set mqtt.enabled=false
```

### Development Installation (with PostgreSQL and MQTT)

```bash
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace \
  --set postgresql.enabled=true \
  --set mqtt.enabled=true
```

## Uninstallation

```bash
helm uninstall maestro-server --namespace maestro
```

## Configuration

The following table lists the configurable parameters and their default values.

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `environment` | Maestro environment (production/development/testing) | `production` |
| `replicas` | Number of server replicas | `1` |

### Image Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Image registry | `image-registry.openshift-image-registry.svc:5000` |
| `image.repository` | Image repository | `maestro/maestro` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Database Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `database.secretName` | Secret name containing database credentials | `maestro-rds` |
| `database.maxOpenConnections` | Maximum database connections | `50` |
| `database.sslMode` | Database SSL mode | `verify-full` |
| `database.debug` | Enable database debug mode | `false` |

### Message Broker Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `messageBroker.type` | Message broker type (mqtt/grpc) | `mqtt` |
| `messageBroker.secretName` | Secret name containing broker config | `maestro-mqtt` |

### Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.https.enabled` | Enable HTTPS | `false` |
| `server.http.bindPort` | HTTP bind port | `8000` |
| `server.grpc.enabled` | Enable gRPC server | `false` |
| `server.grpc.bindPort` | gRPC bind port | `8090` |
| `server.metrics.bindPort` | Metrics bind port | `8080` |
| `server.healthCheck.bindPort` | Health check bind port | `8083` |

### Service Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.api.type` | API service type | `ClusterIP` |
| `service.api.port` | API service port | `8000` |
| `service.grpc.type` | gRPC service type | `ClusterIP` |
| `service.grpc.port` | gRPC service port | `8090` |

### Route Parameters (OpenShift)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `route.enabled` | Enable OpenShift Route | `true` |
| `route.host` | Route hostname (empty for auto-generation) | `""` |
| `route.tls.enabled` | Enable TLS for route | `true` |
| `route.tls.termination` | TLS termination type | `edge` |

### PostgreSQL (Development Only)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Deploy PostgreSQL | `false` |
| `postgresql.image` | PostgreSQL image | `quay.io/maestro/postgres:17.2` |
| `postgresql.database.name` | Database name | `maestro` |
| `postgresql.database.user` | Database user | `maestro` |
| `postgresql.database.password` | Database password | `TheBlurstOfTimes` |
| `postgresql.persistence.enabled` | Enable persistence | `true` |
| `postgresql.persistence.size` | Persistence size | `512Mi` |

### MQTT (Development Only)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `mqtt.enabled` | Deploy MQTT broker | `false` |
| `mqtt.image` | MQTT image | `quay.io/maestro/eclipse-mosquitto:2.0.18` |
| `mqtt.host` | MQTT broker host | `maestro-mqtt` |
| `mqtt.service.port` | MQTT broker port | `1883` |

## Examples

### Production Deployment with gRPC

```bash
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace \
  --set server.grpc.enabled=true \
  --set messageBroker.type=grpc \
  --set replicas=3 \
  --set resources.requests.memory=1Gi \
  --set resources.limits.memory=2Gi
```

### Development Deployment

```bash
helm install maestro-server ./charts/maestro-server \
  --namespace maestro \
  --create-namespace \
  --set postgresql.enabled=true \
  --set mqtt.enabled=true \
  --set database.sslMode=disable
```

## Upgrading

```bash
helm upgrade maestro-server ./charts/maestro-server \
  --namespace maestro \
  --set image.tag=v0.2.0
```

## Source Code

- [GitHub - openshift-online/maestro](https://github.com/openshift-online/maestro)
