# ResourceBundle Commands

Resource bundles are collections of Kubernetes manifests that are deployed to consumer clusters. The `maestro resourcebundle` command group provides operations for managing resource bundles via both REST API (for read operations) and gRPC (for write operations).

## Table of Contents

- [Synopsis](#synopsis)
- [Commands](#commands)
  - [list](#list)
  - [get](#get)
  - [apply](#apply)
  - [delete](#delete)
  - [status](#status)
- [Manifest File Format](#manifest-file-format)
- [Examples](#examples)

## Synopsis

```bash
maestro resourcebundle [command] [flags]
```

### Global Flags

All resourcebundle commands support these flags:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--rest-url` | `MAESTRO_REST_URL` | `https://127.0.0.1:30080` | Maestro REST API base URL |
| `--insecure-skip-verify` | `MAESTRO_REST_INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--timeout` | `MAESTRO_REST_TIMEOUT` | `30s` | HTTP client timeout |
| `--grpc-server-address` | `MAESTRO_GRPC_SERVER_ADDRESS` | `127.0.0.1:30090` | gRPC server address |
| `--grpc-source-id` | `MAESTRO_GRPC_SOURCE_ID` | `maestro-cli` | Source ID for gRPC client |
| `--grpc-ca-file` | `MAESTRO_GRPC_CA_FILE` | - | Path to CA certificate file |
| `--grpc-token-file` | `MAESTRO_GRPC_TOKEN_FILE` | - | Path to token file |
| `--grpc-client-cert-file` | `MAESTRO_GRPC_CLIENT_CERT_FILE` | - | Path to client certificate |
| `--grpc-client-key-file` | `MAESTRO_GRPC_CLIENT_KEY_FILE` | - | Path to client key |

### Configuration Examples

```bash
# Using environment variables
export MAESTRO_REST_INSECURE_SKIP_VERIFY="true"
maestro resourcebundle list

# Using command-line flags
maestro resourcebundle list --insecure-skip-verify
```

### REST vs gRPC

- **REST API** is used for read operations: `list`, `get`, `status`
- **gRPC** is used for write operations: `apply`, `delete`

This design allows for efficient real-time updates via gRPC while maintaining compatibility with standard REST API tooling for queries.

## Commands

### list

List resource bundles with optional filtering and pagination.

#### Usage

```bash
maestro resourcebundle list [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--page` | int | `1` | Page number |
| `--size` | int | `100` | Page size |
| `--search` | string | - | Search filter (SQL-like syntax) |
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# List all resource bundles
maestro resourcebundle list

# List with custom page size
maestro resourcebundle list --page 1 --size 50

# Filter by consumer name
maestro resourcebundle list --search "consumer_name='prod-cluster-01'"

# Filter by consumer name pattern
maestro resourcebundle list --search "consumer_name like 'prod%'"

# Output as JSON
maestro resourcebundle list --output json

# Combine filtering and pagination
maestro resourcebundle list \
  --search "consumer_name like 'prod%'" \
  --page 2 \
  --size 25
```

#### Output Example (Table)

```
ID                          NAME            CONSUMER           VERSION  STATUS      CREATED AT
2faPrp3ZoCMkzdHnBBWd9wqwVXd  my-configmap    prod-cluster-01    2        Applied     2024-01-15 10:30:00
2faPrp3ZoCMkzdHnBBWd9wqwVXe  nginx-deploy    prod-cluster-01    1        Applied     2024-01-15 10:35:00
2faPrp3ZoCMkzdHnBBWd9wqwVXf  app-secrets     dev-cluster-01     1        Pending     2024-01-15 11:00:00
```

---

### get

Get a single resource bundle by its ID via REST API.

#### Usage

```bash
maestro resourcebundle get <id> [flags]
```

#### Arguments

- `<id>` - Resource bundle ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# Get resource bundle by ID
maestro resourcebundle get 2faPrp3ZoCMkzdHnBBWd9wqwVXd

# Get as JSON
maestro resourcebundle get 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json

# Get and save to file
maestro resourcebundle get 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json > bundle.json
```

#### Output Example (Table)

```
ID:             2faPrp3ZoCMkzdHnBBWd9wqwVXd
Name:           my-configmap
Consumer:       prod-cluster-01
Version:        2
Status:         Applied
Created At:     2024-01-15 10:30:00
Updated At:     2024-01-15 14:20:00

Manifests:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: my-config
      namespace: default
    data:
      key: value
```

---

### apply

Create or update a resource bundle from a manifest file via gRPC.

#### Usage

```bash
maestro resourcebundle apply -f <file> [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --file` | string | - | Path to the manifest file (required) |

#### Examples

```bash
# Apply a resource bundle from a JSON file
maestro resourcebundle apply -f bundle.json

# Apply with custom gRPC server
maestro resourcebundle apply -f bundle.json \
  --grpc-server-address maestro.example.com:8090
```

#### Behavior

- If `id` is **not specified** in the manifest: creates a new resource bundle with a generated UUID
- If `id` **is specified**: updates the existing resource bundle (errors if it doesn't exist)
- The manifest file must be in **JSON format**
- Uses gRPC for efficient real-time delivery

#### Output Example

```
Resource bundle applied successfully:
ID:             2faPrp3ZoCMkzdHnBBWd9wqwVXd
Name:           my-configmap
Consumer:       prod-cluster-01
Version:        1
Status:         Pending
```

---

### delete

Delete a resource bundle by its ID via gRPC.

#### Usage

```bash
maestro resourcebundle delete <id> [flags]
```

#### Arguments

- `<id>` - Resource bundle ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-y, --yes` | bool | `false` | Skip confirmation prompt |

#### Examples

```bash
# Delete a resource bundle
maestro resourcebundle delete 2faPrp3ZoCMkzdHnBBWd9wqwVXd
```

#### Output Example

```
Resource bundle 2faPrp3ZoCMkzdHnBBWd9wqwVXd deleted successfully
```

---

### status

Get the status field of a resource bundle by its ID.

#### Usage

```bash
maestro resourcebundle status <id> [flags]
```

#### Arguments

- `<id>` - Resource bundle ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# Get status
maestro resourcebundle status 2faPrp3ZoCMkzdHnBBWd9wqwVXd

# Get status as JSON
maestro resourcebundle status 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json
```

#### Output Example

```
Resource Bundle Status:
ID:       2faPrp3ZoCMkzdHnBBWd9wqwVXd
Status:   Applied

Conditions:
  - Type: Applied
    Status: True
    Reason: ManifestApplied
    Message: All manifests applied successfully
    LastTransitionTime: 2024-01-15 10:31:00
```

---

## Manifest File Format

**Note**: YAML format is not currently supported. Use JSON for manifest files.

### Basic Structure

```json
{
  "id": "2faPrp3ZoCMkzdHnBBWd9wqwVXd",
  "name": "my-configmap-bundle",
  "consumer_name": "prod-cluster-01",
  "version": 1,
  "manifests": [
    {
      "apiVersion": "v1",
      "kind": "ConfigMap",
      "metadata": {
        "name": "my-config",
        "namespace": "default"
      },
      "data": {
        "key1": "value1",
        "key2": "value2"
      }
    }
  ],
  "metadata": {
    "labels": {
      "app": "myapp",
      "env": "production"
    }
  },
  "manifest_configs": [
    {
      "resourceIdentifier": {
        "group": "",
        "resource": "configmaps",
        "name": "my-config",
        "namespace": "default"
      },
      "updateStrategy": {
        "type": "ServerSideApply"
      }
    }
  ],
  "delete_option": {
    "propagationPolicy": "Foreground"
  }
}
```

### Field Descriptions

| Field | Required | Description |
|-------|----------|-------------|
| `id` | No | Resource bundle ID. Omit for create, include for update |
| `name` | No | User-friendly identifier. Must be globally unique if specified |
| `consumer_name` | Yes | Target consumer/cluster name |
| `version` | No | Resource version. For updates, must match if specified |
| `manifests` | Yes | Array of Kubernetes manifest objects |
| `metadata` | No | Additional metadata for manifests |
| `manifest_configs` | No | Per-manifest configuration (update strategy, etc.) |
| `delete_option` | No | Options for resource deletion |


---

## Examples

### Environment-Specific Configuration

Before using the resourcebundle commands, configure the CLI for your deployment environment:

#### KinD Cluster (Local Development)

When running Maestro in a KinD cluster, TLS certificates are typically self-signed. Skip certificate verification:

```bash
# Set environment variables (recommended)
export MAESTRO_REST_INSECURE_SKIP_VERIFY="true"

# Or use command-line flags
maestro resourcebundle list --insecure-skip-verify
```

#### OpenShift / Production Environments

For OpenShift or production deployments, specify the Maestro service endpoints and provide authentication:

```bash
# Set environment variables
export MAESTRO_REST_URL="https://maestro-api.example.com:443"
export MAESTRO_GRPC_SERVER_ADDRESS="maestro-grpc.example.com:8090"
export MAESTRO_GRPC_CA_FILE="/path/to/ca.crt"
export MAESTRO_GRPC_TOKEN_FILE="/path/to/token"

# Or use command-line flags
maestro resourcebundle apply -f bundle.json \
  --rest-url https://maestro-api.example.com:443 \
  --grpc-server-address maestro-grpc.example.com:8090 \
  --grpc-ca-file /path/to/ca.crt \
  --grpc-token-file /path/to/token
```

For mutual TLS authentication:

```bash
export MAESTRO_GRPC_CLIENT_CERT_FILE="/path/to/client.crt"
export MAESTRO_GRPC_CLIENT_KEY_FILE="/path/to/client.key"
```

**Note**: See the [deployment documentation](../README.md) for details on running Maestro in [KinD](../README.md#run-in-kind-cluster) or [OpenShift](../README.md#run-in-openshift).

---

### Deploying and Managing a Resource Bundle

This example assumes you have configured your environment (see [Environment-Specific Configuration](#environment-specific-configuration) above).

```bash
# 1. Find existing consumers in the maestro server
maestro consumer list

# Output example:
# ID                                    NAME       LABELS    CREATED AT
# 219ac81e-cd5c-4d22-9e03-e4eaa4f55aa1  cluster1             2024-01-15 10:20:14

# 2. Create an nginx deployment manifest (JSON format)
# NOTE: Replace "cluster1" with your actual consumer name from step 1
cat > nginx-bundle.json <<'EOF'
{
  "consumer_name": "cluster1",
  "name": "nginx-work",
  "manifests": [
    {
      "apiVersion": "apps/v1",
      "kind": "Deployment",
      "metadata": {
        "name": "nginx",
        "namespace": "default"
      },
      "spec": {
        "replicas": 1,
        "selector": {
          "matchLabels": {
            "app": "nginx"
          }
        },
        "template": {
          "metadata": {
            "labels": {
              "app": "nginx"
            }
          },
          "spec": {
            "containers": [
              {
                "name": "nginx",
                "image": "quay.io/nginx/nginx-unprivileged:latest",
                "imagePullPolicy": "IfNotPresent"
              }
            ]
          }
        }
      }
    }
  ],
  "manifest_configs": [
    {
      "resourceIdentifier": {
        "group": "apps",
        "resource": "deployments",
        "name": "nginx",
        "namespace": "default"
      },
      "updateStrategy": {
        "type": "ServerSideApply"
      },
      "feedbackRules": [
        {
          "type": "JSONPaths",
          "jsonPaths": [
            {
              "name": "status",
              "path": ".status"
            }
          ]
        }
      ]
    }
  ]
}
EOF

# 3. Apply the resource bundle
maestro resourcebundle apply -f nginx-bundle.json

# Output example:
# Resource bundle applied successfully:
# ID: 916777c0-0950-56c5-bb78-c884a111303b

# 4. Check the resource bundle status
maestro resourcebundle status 916777c0-0950-56c5-bb78-c884a111303b

# Output example:
# Resource Bundle Status:
# ID:       916777c0-0950-56c5-bb78-c884a111303b
# Status:   Applied
#
# Conditions:
#   - Type: Applied
#     Status: True
#     Reason: ManifestApplied
#     Message: All manifests applied successfully

# 5. Get full resource bundle details
maestro resourcebundle get 916777c0-0950-56c5-bb78-c884a111303b --output json

# 6. List all bundles for this consumer (replace 'cluster1' with your consumer name)
maestro resourcebundle list --search "consumer_name='cluster1'"

# 7. Delete the bundle when done
maestro resourcebundle delete 916777c0-0950-56c5-bb78-c884a111303b
```

### Batch Operations

```bash
# List all bundles and export as JSON
maestro resourcebundle list --output json > all-bundles.json

# Filter and process with jq
maestro resourcebundle list --output json | \
  jq -r '.items[] | select(.consumer_name=="prod-cluster-01") | .id'

# Delete all bundles for a specific consumer (careful!)
for id in $(maestro resourcebundle list --search "consumer_name='old-cluster'" --output json | jq -r '.items[].id'); do
  maestro resourcebundle delete "$id" --yes
done
```

## See Also

- [Consumer Commands](consumer.md)
- [CLI Overview](README.md)
- [Maestro Architecture](../maestro.md)
- [Resource Payload Documentation](../resources/resource-payload-in-db.md)
