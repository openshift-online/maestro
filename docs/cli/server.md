# Server Command

The `maestro server` command starts the Maestro server, which is the central control plane that stores resources, provides REST/gRPC APIs, and manages communication with agents via message brokers.

## Table of Contents

- [Overview](#overview)
- [Server Endpoints](#server-endpoints)
- [Synopsis](#synopsis)
- [Configuration](#configuration)
- [Quick Start](#quick-start)

## Overview

The Maestro server is the central control plane that:
- Stores resources and their status in a PostgreSQL database
- Provides REST API (default port 8000) and gRPC API (default port 8090)
- Communicates with agents via message brokers (MQTT, gRPC, or Pub/Sub)
- Exposes health check (port 8083) and metrics (port 8080) endpoints

## Server Endpoints

Once the server is running, the following endpoints are available:

### REST API (Port 8000)

- `GET /api/maestro/v1/consumers` - List consumers
- `POST /api/maestro/v1/consumers` - Create consumer
- `GET /api/maestro/v1/consumers/{id}` - Get consumer
- `PATCH /api/maestro/v1/consumers/{id}` - Update consumer
- `DELETE /api/maestro/v1/consumers/{id}` - Delete consumer
- `GET /api/maestro/v1/resource-bundles` - List resource bundles
- `GET /api/maestro/v1/resource-bundles/{id}` - Get resource bundle
- `DELETE /api/maestro/v1/resource-bundles/{id}` - Delete resource bundle

### gRPC API (Port 8090)

- Resource bundle create/update/delete operations
- Real-time resource status updates
- CloudEvents-based communication

### Health Check (Port 8083)

- `GET /healthcheck` - Health check endpoint

### Metrics (Port 8080)

- `GET /metrics` - Prometheus metrics endpoint

## Synopsis

```bash
maestro server [flags]
```

## Configuration

### Database Configuration

#### Database Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db-host-file` | `secrets/db.host` | Database host file |
| `--db-port-file` | `secrets/db.port` | Database port file |
| `--db-name-file` | `secrets/db.name` | Database name file |
| `--db-user-file` | `secrets/db.user` | Database username file |
| `--db-password-file` | `secrets/db.password` | Database password file |
| `--db-sslmode` | `disable` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |
| `--db-max-open-connections` | `50` | Maximum open DB connections |
| `--enable-db-debug` | `false` | Enable database debug logging |

### Message Broker Configuration

#### MQTT Configuration

#### Message Broker Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--message-broker-type` | `mqtt` | Broker type: `mqtt`, `grpc`, or `pubsub` |
| `--message-broker-config-file` | `secrets/mqtt.config` | Broker config file path |
| `--subscription-type` | `shared` | Subscription type: `shared` or `broadcast` |

### HTTP/REST API Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--http-server-bindport` | `8000` | HTTP server port |
| `--enable-https` | `false` | Enable HTTPS |
| `--https-cert-file` | - | Path to TLS certificate |
| `--https-key-file` | - | Path to TLS private key |
| `--http-read-timeout` | `5s` | Read timeout |
| `--http-write-timeout` | `30s` | Write timeout |

### gRPC API Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--enable-grpc-server` | `true` | Enable gRPC server |
| `--grpc-server-bindport` | `8090` | gRPC server port |
| `--grpc-tls-cert-file` | - | Path to TLS certificate |
| `--grpc-tls-key-file` | - | Path to TLS private key |
| `--grpc-authn-type` | `mock` | Auth type: `mock`, `mtls`, `token` |
| `--grpc-max-receive-message-size` | `4194304` | Max receive size (4MB) |
| `--grpc-max-send-message-size` | `2147483647` | Max send size (~2GB) |

### Health Check & Metrics

| Flag | Default | Description |
|------|---------|-------------|
| `--health-check-server-bindport` | `8083` | Health check port |
| `--metrics-server-bindport` | `8080` | Metrics port |
| `--enable-health-check-https` | `false` | Enable HTTPS for health |
| `--enable-metrics-https` | `false` | Enable HTTPS for metrics |


## Quick Start

### Step 1: Set Up Database

```bash
# Start PostgreSQL in Docker
make db/setup

# Verify database is running
make db/login
```

### Step 2: Set Up Message Broker

Choose one of the following:

#### Option A: MQTT (Default)

```bash
# Start MQTT broker in Docker
make mqtt/setup
```

This starts Eclipse Mosquitto on port `1883`.

#### Option B: gRPC (No External Broker Needed)

```bash
# No setup required - gRPC broker is built into the server
# Just start the server with --message-broker-type grpc
```

#### Option C: Pub/Sub Emulator

```bash
# Start Pub/Sub emulator in Docker
make pubsub/setup

# Initialize topics and subscriptions
# Requires: pip3 install google-cloud-pubsub
make pubsub/init
```

### Step 3: Run Database Migrations

```bash
# Run migrations to create database schema
./maestro migration

# Verify migrations
make db/login
```

Expected output:
```sql
maestro=# \dt
                 List of relations
 Schema |       Name       | Type  |  Owner
--------+------------------+-------+---------
 public | consumers        | table | maestro
 public | event_instances  | table | maestro
 public | events           | table | maestro
 public | migrations       | table | maestro
 public | resources        | table | maestro
 public | server_instances | table | maestro
 public | status_events    | table | maestro
(7 rows)
```

### Step 4: Start the Server

```bash
# Start with MQTT broker (default)
make run

# OR start with gRPC broker
MESSAGE_DRIVER_TYPE=grpc make run

# OR start with Pub/Sub emulator
MESSAGE_DRIVER_TYPE=pubsub make run

# OR start directly with the binary
./maestro server
```

### Step 5: Verify Server is Running

```bash
# Check health
curl http://localhost:8083/healthcheck

# List consumers (should be empty initially)
maestro consumer list --rest-url=http://127.0.0.1:8000
```

Expected output:
```
ID    NAME    LABELS    CREATED AT
```

### Step 6: Test with a Consumer

```bash
# Create a consumer
maestro consumer create cluster1 --rest-url=http://127.0.0.1:8000

# List consumers
maestro consumer list --rest-url=http://127.0.0.1:8000
```

Expected output:
```
ID                                    NAME       LABELS    CREATED AT
219ac81e-cd5c-4d22-9e03-e4eaa4f55aa1  cluster1             2024-01-15 10:20:14
```

## Next Steps

After starting the server:

1. Use the [CLI consumer commands](consumer.md) to manage consumers
2. Use the [CLI resourcebundle commands](resourcebundle.md) to deploy resources
3. Deploy agents on target clusters (TODO)
4. Monitor metrics at http://localhost:8080/metrics
5. Check health at http://localhost:8083/healthcheck

## See Also

- [Consumer Commands](consumer.md)
- [ResourceBundle Commands](resourcebundle.md)
- [CLI Overview](README.md)
- [Maestro Architecture](../maestro.md)
- [Troubleshooting](../troubleshooting.md)
