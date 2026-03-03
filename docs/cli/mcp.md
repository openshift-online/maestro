# MCP Command

The Model Context Protocol (MCP) server enables AI assistant integration with Maestro, providing tools for managing resources through conversational interfaces. The `maestro mcp` command starts an MCP server that communicates via stdio, allowing AI assistants like Claude Code to interact with Maestro resources.

## Table of Contents

- [Overview](#overview)
- [Synopsis](#synopsis)
- [How It Works](#how-it-works)
- [Available Tools](#available-tools)
  - [Resource Bundle Tools](#resource-bundle-tools)
  - [Consumer Tools](#consumer-tools)
- [Configuration](#configuration)
- [Examples](#examples)
- [Integration](#integration)

## Overview

The MCP server provides a standardized interface for AI assistants to interact with Maestro. It exposes both consumer and resource bundle operations as MCP tools, enabling natural language workflows for managing Kubernetes resources across multiple clusters.

### Key Features

- **Stdio Communication**: Runs on standard input/output for seamless AI integration
- **Full CRUD Operations**: Complete create, read, update, delete support for consumers and resource bundles
- **Search & Filtering**: Advanced SQL-like search capabilities
- **Status Monitoring**: Real-time feedback on resource bundle deployment status

## Synopsis

```bash
maestro mcp [flags]
```

### Required Configuration

The MCP server requires both REST and gRPC client configurations:

| Category | Flag | Environment Variable | Default | Description |
|----------|------|---------------------|---------|-------------|
| **REST API** | `--rest-url` | `MAESTRO_REST_URL` | `https://127.0.0.1:30080` | Maestro REST API base URL |
| | `--insecure-skip-verify` | `MAESTRO_REST_INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| | `--timeout` | `MAESTRO_REST_TIMEOUT` | `30s` | HTTP client timeout |
| **gRPC** | `--grpc-server-address` | `MAESTRO_GRPC_SERVER_ADDRESS` | `127.0.0.1:30090` | gRPC server address |
| | `--grpc-ca-file` | `MAESTRO_GRPC_CA_FILE` | - | Path to CA certificate file for TLS |
| | `--grpc-token-file` | `MAESTRO_GRPC_TOKEN_FILE` | - | Path to authentication token file |
| | `--grpc-client-cert-file` | `MAESTRO_GRPC_CLIENT_CERT_FILE` | - | Path to client certificate for mTLS |
| | `--grpc-client-key-file` | `MAESTRO_GRPC_CLIENT_KEY_FILE` | - | Path to client private key for mTLS |
| | `--grpc-source-id` | `MAESTRO_GRPC_SOURCE_ID` | `maestro-mcp` | Source ID for gRPC client |

## How It Works

The MCP server:

1. **Initializes Clients**: Creates REST and gRPC clients for Maestro API
2. **Registers Tools**: Exposes 12 MCP tools (7 resource bundle + 5 consumer tools)
3. **Listens on Stdio**: Waits for MCP protocol messages from AI assistant
4. **Processes Requests**: Executes tool calls and returns results

### Architecture

```
┌─────────────────┐         ┌──────────────────┐         ┌─────────────────┐
│  AI Assistant   │◄─stdio─►│  MCP Server      │◄─REST──►│  Maestro API    │
│  (Claude Code)  │         │  (maestro mcp)   │         │                 │
└─────────────────┘         │                  │◄─gRPC──►│                 │
                            └──────────────────┘         └─────────────────┘
                                     │
                                     ├─ Resource Bundle Tools (7)
                                     └─ Consumer Tools (5)
```

## Available Tools

### Resource Bundle Tools

#### maestro_list_resource_bundles

List resource bundles with pagination and optional filtering.

**Parameters:**
- `page` (number, optional): Page number for pagination (default: 1)
- `size` (number, optional): Page size for pagination (default: 100)
- `search` (string, optional): SQL search query (e.g., `"consumer_name = 'cluster1' AND version > 5"`)

**Returns:** Paginated list of resource bundles with manifests, status, and metadata

---

#### maestro_get_resource_bundle

Get a single resource bundle by ID.

**Parameters:**
- `id` (string, required): Resource bundle ID

**Returns:** Full bundle details including manifests, status, and metadata

---

#### maestro_apply_resource_bundle

Create or update a resource bundle. If the bundle ID exists, it will be updated; otherwise creation will fail.

**Parameters:**
- `bundle` (object, required): Resource bundle object containing:
  - `id` (string, optional): Bundle ID (omit for new bundles, provide for updates)
  - `consumer_name` (string, required): Target consumer/cluster name
  - `version` (number, optional): Version for optimistic concurrency control
  - `manifests` (array, required): List of Kubernetes manifest objects
  - `metadata` (object, optional): Additional metadata
  - `manifest_configs` (array, optional): Manifest-specific configurations
  - `delete_option` (object, optional): Deletion propagation options

**Returns:** Applied resource bundle with generated ID

**Example Bundle Object:**
```json
{
  "consumer_name": "prod-cluster-01",
  "manifests": [
    {
      "apiVersion": "v1",
      "kind": "ConfigMap",
      "metadata": {
        "name": "my-config",
        "namespace": "default"
      },
      "data": {
        "key": "value"
      }
    }
  ]
}
```

---

#### maestro_delete_resource_bundle

Delete a resource bundle by ID. Triggers deletion on the target cluster.

**Parameters:**
- `id` (string, required): Resource bundle ID to delete

**Returns:** Confirmation of deletion

---

#### maestro_get_resource_bundle_status

Get the status field of a resource bundle, showing feedback from the agent about deployment state.

**Parameters:**
- `id` (string, required): Resource bundle ID

**Returns:** Status object with conditions and agent feedback

---

#### maestro_search_resource_bundles

Search resource bundles using SQL-like query syntax.

**Parameters:**
- `search` (string, required): SQL search query (e.g., `"consumer_name LIKE 'prod-%'"`)
- `page` (number, optional): Page number for pagination (default: 1)
- `size` (number, optional): Page size for pagination (default: 100)

**Returns:** Paginated list of matching resource bundles

**Search Examples:**
- `"consumer_name = 'cluster-1'"`
- `"version > 10"`
- `"consumer_name LIKE 'prod-%' AND version >= 5"`

---

#### maestro_list_resource_bundles_by_consumer

List all resource bundles for a specific consumer (cluster).

**Parameters:**
- `consumer_name` (string, required): Consumer/cluster name
- `page` (number, optional): Page number for pagination (default: 1)
- `size` (number, optional): Page size for pagination (default: 100)

**Returns:** Paginated list of resource bundles for the consumer

---

### Consumer Tools

#### maestro_list_consumers

List consumers (clusters) with pagination and optional filtering.

**Parameters:**
- `page` (number, optional): Page number for pagination (default: 1)
- `size` (number, optional): Page size for pagination (default: 100)
- `search` (string, optional): SQL search query (e.g., `"labels->>'env' = 'production'"`)

**Returns:** Paginated list of consumers with metadata and labels

---

#### maestro_get_consumer

Get a single consumer (cluster) by ID.

**Parameters:**
- `id` (string, required): Consumer ID

**Returns:** Consumer details including labels and metadata

---

#### maestro_create_consumer

Create a new consumer (cluster) registration.

**Parameters:**
- `name` (string, required): Consumer name (RFC 1123 compliant: lowercase alphanumeric, hyphens, max 63 chars)
- `labels` (object, optional): Key-value pairs for consumer labels

**Returns:** Created consumer with generated ID

**Example:**
```json
{
  "name": "prod-cluster-01",
  "labels": {
    "env": "production",
    "region": "us-east-1"
  }
}
```

---

#### maestro_update_consumer_labels

Update labels for an existing consumer using PATCH operation.

**Parameters:**
- `id` (string, required): Consumer ID
- `labels` (object, required): Labels to merge with existing labels

**Returns:** Updated consumer

**Behavior:**
- Labels are merged with existing labels
- Existing label keys are overwritten
- New label keys are added

---

#### maestro_delete_consumer

Delete a consumer (cluster) by ID.

**Parameters:**
- `id` (string, required): Consumer ID to delete

**Returns:** Confirmation of deletion

**Important:** This will fail if the consumer has existing resource bundles. Delete all resource bundles first.

---

## Configuration

### Using Environment Variables

```bash
# Set REST API configuration
export MAESTRO_REST_URL="https://maestro.example.com/api/maestro/v1"
export MAESTRO_REST_INSECURE_SKIP_VERIFY="false"

# Set gRPC configuration
export MAESTRO_GRPC_SERVER_ADDRESS="maestro.example.com:8090"
export MAESTRO_GRPC_CA_FILE="/path/to/ca.crt"
export MAESTRO_GRPC_TOKEN_FILE="/path/to/token"
export MAESTRO_GRPC_SOURCE_ID="my-mcp-client"

# Start MCP server
maestro mcp
```

### Using Command-Line Flags

```bash
maestro mcp \
  --rest-url https://maestro.example.com/api/maestro/v1 \
  --grpc-server-address maestro.example.com:8090 \
  --grpc-ca-file /path/to/ca.crt \
  --grpc-token-file /path/to/token \
  --grpc-source-id my-mcp-client
```

### Insecure Development Setup

For local development with self-signed certificates:

```bash
maestro mcp \
  --rest-url https://127.0.0.1:30080 \
  --insecure-skip-verify \
  --grpc-server-address 127.0.0.1:30090
```

## Examples

### Example 1: Starting MCP Server for Production

```bash
# Production configuration with TLS
maestro mcp \
  --rest-url https://maestro.prod.example.com/api/maestro/v1 \
  --grpc-server-address maestro.prod.example.com:8090 \
  --grpc-ca-file /etc/maestro/certs/ca.crt \
  --grpc-token-file /etc/maestro/tokens/grpc-token \
  --grpc-source-id production-mcp-server
```

### Example 2: Development Setup

```bash
# Local development with insecure connections
maestro mcp \
  --rest-url http://127.0.0.1:8000/api/maestro/v1 \
  --insecure-skip-verify \
  --grpc-server-address 127.0.0.1:30090 \
  --grpc-source-id dev-mcp-server
```

### Example 3: Using Mutual TLS

```bash
# Production with mTLS authentication
maestro mcp \
  --rest-url https://maestro.example.com/api/maestro/v1 \
  --grpc-server-address maestro.example.com:8090 \
  --grpc-ca-file /etc/maestro/certs/ca.crt \
  --grpc-client-cert-file /etc/maestro/certs/client.crt \
  --grpc-client-key-file /etc/maestro/certs/client.key \
  --grpc-source-id secure-mcp-server
```

## Integration

### Claude Code Integration

The MCP server is designed to work with Claude Code and other MCP-compatible AI assistants.

#### Setting Up with Claude Code

You can configure the Maestro MCP server using the `claude mcp add` command (recommended) or by manually editing configuration files.

```bash
# Navigate to your project directory
cd /path/to/your/project

# Add the Maestro MCP server (use -- separator before the command)
claude mcp add maestro -- maestro mcp \
  --rest-url https://maestro.example.com/api/maestro/v1 \
  --grpc-server-address maestro.example.com:8090 \
  --grpc-ca-file /path/to/ca.crt \
  --grpc-token-file /path/to/token

# Or Use -e flag to pass environment variables
claude mcp add -s user maestro -e MAESTRO_REST_URL=https://maestro.example.com/api/maestro/v1 \
  -e MAESTRO_GRPC_SERVER_ADDRESS=maestro.example.com:8090 \
  -- maestro mcp
```

Alternatively, you can manually create a `.mcp.json` file in your project root:

```json
{
  "mcpServers": {
    "maestro": {
      "command": "maestro",
      "args": [
        "mcp",
        "--rest-url", "https://maestro.example.com/api/maestro/v1",
        "--grpc-server-address", "maestro.example.com:8090",
        "--grpc-ca-file", "/path/to/ca.crt",
        "--grpc-token-file", "/path/to/token"
      ]
    }
  }
}
```

##### Activation

After configuration, **restart Claude Code** to load the MCP server

##### Verifying Configuration

```bash
# List all configured MCP servers
claude mcp list

# Or Use the `/mcp` command in Claude Code
```

##### Using the MCP Server

Once configured, **use natural language** to interact with Maestro:
   - "List all consumers in production"
   - "Show me resource bundles for cluster prod-01"
   - "Create a new consumer named staging-cluster with environment label"
   - "Apply this ConfigMap to cluster prod-01"
   - "What's the status of bundle xyz-123?"

#### Example Conversational Workflows

**Creating a Consumer:**
```
User: "Create a new consumer named prod-cluster-01 with labels env=production and region=us-east"

Claude: [Uses maestro_create_consumer tool]
Result: Consumer created successfully with ID 2faPrp3ZoCMkzdHnBBWd9wqwVXd
```

**Deploying a Resource Bundle:**
```
User: "Deploy this ConfigMap to prod-cluster-01"
[Provides ConfigMap YAML]

Claude: [Uses maestro_apply_resource_bundle tool]
Result: Resource bundle created with ID abc-def-123
```

**Monitoring Deployment:**
```
User: "What's the status of bundle abc-def-123?"

Claude: [Uses maestro_get_resource_bundle_status tool]
Result: Applied successfully at 2024-01-15 14:30:00
```

### MCP Protocol Details

The server implements the [Model Context Protocol specification](https://modelcontextprotocol.io/) and communicates using JSON-RPC 2.0 over stdio.

**Server Information:**
- Name: `maestro-mcp-server`
- Version: `1.0.0`
- Protocol: MCP (Model Context Protocol)
- Transport: stdio (standard input/output)

## Troubleshooting

### Verifying MCP Server Status

**Problem:** Not sure if the MCP server is loaded

**Solution:** Use the `/mcp` command in Claude Code:

```
/mcp
```

**Expected output:**
```
Connected MCP servers:
- maestro (12 tools available)
```

**If you see "No MCP servers configured":**
1. Check configured servers: `claude mcp list`
2. Verify the `maestro` binary is in your PATH: `which maestro`
3. If not configured, add it: `claude mcp add maestro -- maestro mcp [...]`
4. For manual configuration, check that the `.mcp.json` file exists in your project root
5. Restart Claude Code or start a new session after configuration changes
6. Run `/mcp` again to verify the server is loaded

### Common Issues

**1. Connection Refused**
```
Error: failed to create REST client: connection refused
```

**Solution:** Verify the REST URL is correct and the Maestro server is running.

```bash
# Test REST API connectivity
curl https://maestro.example.com/api/maestro/v1/consumers
```

---

**2. TLS Certificate Errors**
```
Error: x509: certificate signed by unknown authority
```

**Solution:** Provide the correct CA certificate file via `--grpc-ca-file` or use `--insecure-skip-verify` for development.

## See Also

- [Consumer Commands](consumer.md)
- [ResourceBundle Commands](resourcebundle.md)
- [CLI Overview](README.md)
- [Maestro Architecture](../maestro.md)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
