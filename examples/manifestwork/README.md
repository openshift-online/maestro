# MaestroGRPCSourceWorkClient

This example shows how to build a `MaestroGRPCSourceWorkClient` with Maestro gRPC service and watch/get/list/create/patch/delete ManifestWorks.

## Programmatic Usage

### Build the MaestroGRPCSourceWorkClient

```golang
sourceID := "mw-client-example"

workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
  ctx,
  logger,
  maestroAPIClient,
  maestroGRPCOptions,
  sourceID,
)

if err != nil {
  log.Fatal(err)
}

// watch/get/list/create/patch/delete by workClient
```

### Paginated List Example

The `List` operation supports pagination, which is useful when there are many works in Maestro:

```golang
workClient, err := ...

// Example: List 2500 works with pagination (1000 per page)

// First page: returns 1000 works and a continuation token
workList, err := workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
  Limit: 1000
})
if err != nil {
  log.Fatal(err)
}

// Second page: returns next 1000 works
workList, err = workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
  Limit: 1000,
  Continue: workList.ListMeta.Continue,
})
if err != nil {
  log.Fatal(err)
}

// Third page: returns remaining 500 works, Continue will be empty
workList, err = workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
  Limit: 1000,
  Continue: workList.ListMeta.Continue,
})
if err != nil {
  log.Fatal(err)
}
```

## Command-line Client (client.go)

The example `client.go` provides a command-line interface for all ManifestWork operations.

### Usage

```bash
go run examples/manifestwork/client.go <command> [arguments]

Commands:
  get <work-name>           Get a specific manifestwork
  list                      List all manifestworks
  apply <manifestwork-file> Create or update a manifestwork from a JSON file
  delete <work-name>        Delete a manifestwork
  watch                     Watch for manifestwork changes

Common Flags:
  --consumer-name string                 The Consumer Name (required)
  --maestro-server string                The maestro server address (default "http://127.0.0.1:30080")
  --grpc-server string                   The grpc server address (default "127.0.0.1:30090")
  --source string                        The source ID for manifestwork client (default "mw-client-example")
  --grpc-server-ca-file string           The CA certificate for grpc server
  --grpc-client-cert-file string         The client certificate to access grpc server
  --grpc-client-key-file string          The client key to access grpc server
  --grpc-client-token-file string        The client token to access grpc server
  --server-healthiness-timeout duration  The server healthiness timeout (default 20s)
  --print-work-details                   Print detailed work information (for watch command)
```

### ManifestWork JSON File

Before creating a manifestwork, review the [manifestwork.json](./manifestwork.json) file. You can modify it as needed.

**Important notes about supported features:**
- **Deletion options**: Only `Foreground` and `Orphan` propagation policies are verified. Others may not work.
- **Update strategy**: `ServerSideApply`, `Update`, `CreateOnly`, and `ReadOnly` are verified. Others are not guaranteed.

See the full [ManifestWork API](https://github.com/open-cluster-management-io/api/blob/main/work/v1/types.go) for all available options.

### Quick Start (Local Development)

Use this method when running Maestro locally with `make test-env`:

1. **Prepare the environment:**
   ```bash
   make test-env
   ```

2. **Watch for work changes:**
   ```bash
   go run examples/manifestwork/client.go watch \
     --consumer-name=$(cat test/_output/.consumer_name) \
     --print-work-details \
     --insecure-skip-verify
   ```

3. **Apply a manifestwork from file:**
   ```bash
   go run examples/manifestwork/client.go apply examples/manifestwork/manifestwork.json \
     --consumer-name=$(cat test/_output/.consumer_name) \
     --insecure-skip-verify
   ```

4. **List all works:**
   ```bash
   go run examples/manifestwork/client.go list \
     --consumer-name=$(cat test/_output/.consumer_name) \
     --insecure-skip-verify
   ```

5. **Get a specific work:**
   ```bash
   go run examples/manifestwork/client.go get nginx-work \
     --consumer-name=$(cat test/_output/.consumer_name) \
     --insecure-skip-verify
   ```

6. **Delete a work:**
   ```bash
   go run examples/manifestwork/client.go delete nginx-work \
     --consumer-name=$(cat test/_output/.consumer_name) \
     --insecure-skip-verify
   ```

### Production Setup

1. **Set up port forwarding:**
   ```bash
   kubectl -n maestro port-forward svc/maestro 8000 &
   kubectl -n maestro port-forward svc/maestro-grpc 8090 &
   ```

2. **Run commands with TLS:**

   If your maestro server is running with TLS, use the following examples:

   ```bash
   # Apply a manifestwork with TLS
   go run examples/manifestwork/client.go apply examples/manifestwork/manifestwork.json \
     --consumer-name=$CONSUMER_NAME \
     --maestro-server=https://127.0.0.1:8000 \
     --grpc-server=127.0.0.1:8090 \
     --grpc-server-ca-file=<your grpc server ca.crt> \
     --grpc-client-token-file=<your grpc server token>

   # Watch with TLS
   go run examples/manifestwork/client.go watch \
     --consumer-name=$CONSUMER_NAME \
     --maestro-server=https://127.0.0.1:8000 \
     --grpc-server=127.0.0.1:8090 \
     --grpc-server-ca-file=<your grpc server ca.crt> \
     --grpc-client-token-file=<your grpc server token> \
     --print-work-details
   ```

   **Note:** You need to create a cluster role to grant publish & subscribe permissions to the client by running this command:
   ```bash
   $ cat << EOF | kubectl apply -f -
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: grpc-pub-sub
   rules:
   - nonResourceURLs:
     - /sources/mw-client-example
     verbs:
     - pub
     - sub
   EOF
   ```

3. **Run commands without TLS:**

   If your maestro server is running without TLS, use the following examples:

   ```bash
   # Apply a manifestwork without TLS
   go run examples/manifestwork/client.go apply examples/manifestwork/manifestwork.json \
     --consumer-name=$CONSUMER_NAME \
     --maestro-server=http://127.0.0.1:8000 \
     --grpc-server=127.0.0.1:8090 \
     --insecure-skip-verify

   # Watch without TLS
   go run examples/manifestwork/client.go watch \
     --consumer-name=$CONSUMER_NAME \
     --maestro-server=http://127.0.0.1:8000 \
     --grpc-server=127.0.0.1:8090 \
     --print-work-details \
     --insecure-skip-verify
   ```

### Example Output

**Creating a work:**
```
Apply manifestwork (opid=3bd7d...)
Work 77112687-f4ac-42d1-b8b8-6f999428c19d/nginx-work (uid=abc123...) created successfully
```

**Updating an existing work:**
```
Apply manifestwork (opid=5ad7d...)
Work 77112687-f4ac-42d1-b8b8-6f999428c19d/nginx-work (uid=abc123...) updated successfully
```

**Watching changes with `--print-work-details`:**
```
Watch manifestwork (opid=9ed7d...)
[MODIFIED] Work: 77112687-f4ac-42d1-b8b8-6f999428c19d/nginx-work (uid=abc123...)
{
  "metadata": {
    "name": "nginx-work",
    "namespace": "77112687-f4ac-42d1-b8b8-6f999428c19d",
    ...
  },
  "spec": {
    "workload": {
      "manifests": [
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          ...
        }
      ]
    }
  },
  "status": {
    "conditions": [
      {
        "type": "Applied",
        "status": "True",
        ...
      }
    ]
  }
}
```

**Getting work:**
```
Get manifestwork 114916d9-f470-4905-8793-785d58ee0f8d/nginx-work (opid=7cd7d...)
{
  "metadata": {
    "name": "nginx-work",
    "namespace": "77112687-f4ac-42d1-b8b8-6f999428c19d",
    ...
  }
  "spec": {
    "workload": {
      "manifests": [
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          ...
        }
      ]
    }
  },
  "status": {
    "conditions": [
      {
        "type": "Applied",
        "status": "True",
        ...
      }
    ]
  }
}
```


**Listing works:**
```
List manifestworks (opid=9dd7d...):
Consumer    Name            UID         Created
77112687... nginx-work      941681df... 2026-01-08T17:04:58+08:00
77112687... test-work      941681df... 2026-01-08T17:04:58+08:00
77112687... demo-work      941681df... 2026-01-08T17:04:58+08:00
```
