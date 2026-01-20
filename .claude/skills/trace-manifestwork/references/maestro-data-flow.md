# Maestro Resource Data Flow

This document describes how ManifestWork resources flow through the Maestro system.

## Overview

Maestro leverages CloudEvents to transport Kubernetes resources to target clusters and relay the resource status back. The system consists of two main parts:

1. **Maestro Server**: Stores resources in database, publishes to message brokers
2. **Maestro Agent**: Receives resources, applies to clusters, reports status

## Complete Resource Flow

### Resource Create/Update Flow

#### 1. User Creates ManifestWork

The user creates a ManifestWork via `MaestroGRPCSourceWorkClient`:

```json
{
  "apiVersion": "work.open-cluster-management.io/v1",
  "kind": "ManifestWork",
  "metadata": {
      "name": "e44ec579-9646-549a-b679-db8d19d6da37",
      ...
  },
  "spec": {
    "workload": {
      "manifests": [
          {
              "kind": "Deployment",
              "apiVersion": "apps/v1",
              "metadata": {
                  "name": "maestro-e2e-upgrade-test",
                  "namespace": "default"
              },
              ...
          }
      ]
    }
  }
}
```

**Key Point**: The user assigns the work name `e44ec579-9646-549a-b679-db8d19d6da37`.

#### 2. Client Sends CloudEvent

The `MaestroGRPCSourceWorkClient` sends this ManifestWork as a CloudEvent to the Maestro server via gRPC:

```json
{
  "source": "mw-client-example",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request",
  "datacontenttype": "application/json",
  "data": {...},
  "metadata": "{\"name\":\"e44ec579-9646-549a-b679-db8d19d6da37\",\"uid\":\"55c61e54-a3f6-563d-9fec-b1fe297bdfdb\",...}",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",
  ...
}
```

**Key Point**: The client generates a UID `55c61e54-a3f6-563d-9fec-b1fe297bdfdb` using:
```
uuid.NewSHA1(uuid.NameSpaceOID, sourceID + manifestwork.GR + manifestwork.Namespace + manifestwork.Name)
```

This UID becomes the CloudEvent `resourceid` extension attribute.

#### 3. Server Stores in Database

The Maestro server receives the CloudEvent and creates a Resource record in PostgreSQL:

**Database Table: `resources`**
- `id` (VARCHAR, PK): Set to CloudEvent `resourceid` = `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- `payload` (JSONB): The complete CloudEvent
  - `payload->'metadata'->>'name'` = `e44ec579-9646-549a-b679-db8d19d6da37` (user work name)
  - `payload->'spec'->'workload'->'manifests'` = Array of manifests
- `created_at`, `updated_at`, `deleted_at`: Timestamps

**Key Point**: The Resource ID in the database is the CloudEvent resourceid, NOT the user-created work name.

#### 4. Server Publishes to Agent

After persisting, the server publishes a CloudEvent to the Maestro agent:

```json
{
  "source": "maestro",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.spec",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",
  "data": {...}
}
```

**Key Point**: The server uses the Resource ID (`55c61e54-a3f6-563d-9fec-b1fe297bdfdb`) as the ManifestWork name when sending to the agent.

#### 5. Agent Applies to Cluster

The Maestro agent:
1. Receives the CloudEvent
2. Converts it back to a ManifestWork (in-memory, not persisted as CR)
3. Applies the manifests to the target Kubernetes cluster
4. Creates an AppliedManifestWork CR

**AppliedManifestWork Structure:**

```yaml
apiVersion: work.open-cluster-management.io/v1
kind: AppliedManifestWork
metadata:
  uid: 1aa9d3c4-74bf-42ff-a8ae-3d7d930da845
  name: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb
  ...
spec:
  agentID: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8
  hubHash: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8
  manifestWorkName: 55c61e54-a3f6-563d-9fec-b1fe297bdfdb
status:
  appliedResources:
  - group: apps
    name: maestro-e2e-upgrade-test
    namespace: default
    resource: deployments
    uid: f7c235c8-f4d5-4d16-9b71-6956291d05c7
    version: v1
```

**Key Points:**
- AppliedManifestWork name format: `{agentID}-{resourceID}`
- `spec.manifestWorkName` = Resource ID = `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- `status.appliedResources[]` lists all applied manifests

**Applied Manifest (Deployment):**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: maestro-e2e-upgrade-test
  namespace: default
  ownerReferences:
  - apiVersion: work.open-cluster-management.io/v1
    kind: AppliedManifestWork
    name: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb
    uid: 1aa9d3c4-74bf-42ff-a8ae-3d7d930da845
```

**Key Point**: The manifest's ownerReference points to the AppliedManifestWork, not directly to the original ManifestWork.

#### Update Flow

Updates to ManifestWorks follow the same flow as creation:

1. User updates ManifestWork via gRPC client
2. Client sends CloudEvent with `type: ...spec.update_request`
3. Server updates database (same `id`, updates `payload` and `updated_at`)
4. Server publishes updated CloudEvent to agent
5. Agent updates AppliedManifestWork and applies manifest changes

**Key Point**: Updates use the same Resource ID - the database row is updated, not replaced.

### Resource Status Update Flow

The status update flow runs continuously to sync resource state from cluster back to server:

#### 1. Agent Watches Resource Status

Agent monitors manifests status on the cluster.

#### 2. Agent Publishes Status CloudEvent

When status changes, the agent updates the corresponding ManifestWork and publishes a CloudEvent:

```json
{
  "source": "agent",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",
  "data": {
    "status": {
      "conditions": [...],
      "resourceStatus": {...}
    }
  }
}
```

#### 3. Server Updates Database

Server receives status update and modifies the database.

**Key Point**:
- Updates `resources.status` with new status
- Sets `resources.updated_at` timestamp

#### 4. Server Publishes Status to Client

Server publishes status CloudEvent to subscribers (MaestroGRPCSourceWorkClient).

#### 5. Client Receives Status Update

MaestroGRPCSourceWorkClient receives and converts status to ManifestWork format.

#### 6. Consumer Processes Updated Work

User's watch handler receives the updated ManifestWork with current status.

**Status Update Timeline:**

```
Cluster State Changes
    ↓
Agent Detects Change
    ↓
Agent → Server (Status CloudEvent)
    ↓
Server Updates DB (payload->status)
    ↓
Server → Client (Status CloudEvent)
    ↓
User Receives Updated Work
```

### Resource Deletion Flow

When a user deletes a ManifestWork, Maestro follows a multi-step deletion flow to ensure proper cleanup across all components:

#### 1. User Initiates Deletion

User deletes a work via MaestroGRPCSourceWorkClient:

```go
client.Delete(workName)
```

#### 2. Server Marks Resource for Deletion (Soft Delete)

The Maestro server receives the delete request and marks the resource in the database.

**Key Point**: This is a **soft delete**. The resource still exists in the database but is marked with `deleted_at` timestamp. This is a transient state (seconds to minutes).

#### 3. Server Publishes Delete Request to Agent

The server publishes a CloudEvent delete request to the agent:

```json
{
  "source": "maestro",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.spec.delete_request",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
}
```

#### 4. Agent Deletes Resources from Cluster

The agent receives the delete request and:

1. Deletes the AppliedManifestWork from the management cluster
2. Kubernetes garbage collection automatically deletes owned manifests (due to ownerReferences)

```bash
# Agent deletes AppliedManifestWork
kubectl delete appliedmanifestwork {agentID}-{resourceID}

# Kubernetes automatically deletes manifests with ownerReferences
# - Deployment/default/maestro-e2e-upgrade-test (deleted automatically)
# - Service/default/maestro-e2e-service (deleted automatically)
# - etc.
```

**Key Point**: The cascade deletion happens automatically because manifests have ownerReferences pointing to the AppliedManifestWork.

#### 5. Agent Reports Deletion Status

The agent publishes a status update confirming deletion:

```json
{
  "source": "agent",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",
  "deletiontimestamp": "2024-01-15T12:00:00Z"
}
```

#### 6. Server Hard Deletes Resource from Database

Upon receiving the deletion confirmation from the agent, the server performs a **hard delete**.

**Key Point**: The resource is now **completely removed** from the database. You cannot query for deleted resources.

#### 7. Server Notifies User of Completion

The server publishes a CloudEvent to the user via gRPC client confirming the work is deleted:

```json
{
  "source": "maestro",
  "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request",
  "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
}
```

**Deletion Timeline:**

```
User Delete Request
    ↓
Server: Soft Delete (deleted_at set) ← Transient state, seconds to minutes
    ↓
Agent: Delete AppliedManifestWork + Manifests
    ↓
Agent: Send Deletion Confirmation
    ↓
Server: Hard Delete (resource removed from DB) ← Final state, resource gone
    ↓
User: Receive Deletion Confirmation
```

**Important Notes About Deleted Resources:**

1. **No Historical Data**: Once deletion completes, the resource is completely removed from the database. There is no historical record.

2. **Soft Delete is Transient**: The `deleted_at` field is only set temporarily during the deletion process (usually seconds to minutes). By the time you query the database, the resource is likely already hard deleted.

3. **Cannot Query Deleted Resources**: Queries like `SELECT * FROM resources WHERE deleted_at IS NOT NULL` will typically return no results, because resources with `deleted_at` set are quickly hard deleted.

4. **Cluster State After Deletion**:
   - AppliedManifestWork: ❌ Not found (deleted)
   - Manifests: ❌ Not found (deleted via ownerReference cascade)
   - Database record: ❌ Not found (hard deleted)

5. **Debugging Active Deletions**: If you catch a resource during the deletion process (deleted_at is set but not yet hard deleted), it means:
   - The server has initiated deletion
   - The agent is processing the delete request
   - The resource will be hard deleted soon

**Example: Checking for Resources in Deletion**

```sql
-- This will usually return 0 rows (resources are quickly hard deleted)
SELECT id,
       payload->'metadata'->>'name' AS work_name,
       deleted_at,
       NOW() - deleted_at AS deletion_age
FROM resources
WHERE deleted_at IS NOT NULL;
```

If you do find resources with `deleted_at` set, they are in the middle of the deletion process and will be removed shortly.

## Identity Mapping

Understanding the relationships between different identifiers:

| Identifier | Value | Location | Purpose |
|------------|-------|----------|---------|
| User Work Name | `e44ec579-9646-549a-b679-db8d19d6da37` | DB: `payload->'metadata'->>'name'` | Name assigned by user |
| Resource ID | `55c61e54-a3f6-563d-9fec-b1fe297bdfdb` | DB: `id` (PK)<br>AppliedManifestWork: `spec.manifestWorkName` | Database primary key, links DB ↔ cluster |
| AppliedManifestWork Name | `{agentID}-{resourceID}` | Cluster: AppliedManifestWork CR | Full name on management cluster |
| Manifest Name | `maestro-e2e-upgrade-test` | Cluster: Deployment/Service/etc | Actual resource name |

## Tracing Strategies

### Trace from User Work Name

```
User Work Name
    ↓ (DB query: WHERE payload->'metadata'->>'name' = ?)
Resource ID
    ↓ (kubectl: WHERE spec.manifestWorkName = ?)
AppliedManifestWork
    ↓ (kubectl: status.appliedResources[])
Manifest List
```

### Trace from Manifest Name

```
Manifest Name
    ↓ (kubectl: metadata.ownerReferences[])
AppliedManifestWork Name
    ↓ (kubectl: spec.manifestWorkName)
Resource ID
    ↓ (DB query: WHERE id = ?)
User Work Name
```

### Trace from Resource ID

```
Resource ID
    ↓ (DB query: WHERE id = ?)
User Work Name + Manifest Definitions
    ↓ (kubectl: WHERE spec.manifestWorkName = ?)
AppliedManifestWork
    ↓ (kubectl: status.appliedResources[])
Applied Manifest List
```

## Common Queries

### Find Resource ID from User Work Name

```sql
SELECT id FROM resources
WHERE payload->'metadata'->>'name' = 'e44ec579-9646-549a-b679-db8d19d6da37';
```

### Find User Work Name from Resource ID

```sql
SELECT payload->'metadata'->>'name' AS user_work_name
FROM resources
WHERE id = '55c61e54-a3f6-563d-9fec-b1fe297bdfdb';
```

### Find AppliedManifestWork by Resource ID

```bash
kubectl get appliedmanifestworks -o json | \
  jq -r ".items[] | select(.spec.manifestWorkName == \"55c61e54-a3f6-563d-9fec-b1fe297bdfdb\") | .metadata.name"
```

### Find Resource ID from Manifest

```bash
# Get AppliedManifestWork name from manifest ownerReference
amw_name=$(kubectl get deployment maestro-e2e-upgrade-test -n default \
  -o jsonpath='{.metadata.ownerReferences[?(@.kind=="AppliedManifestWork")].name}')

# Get Resource ID from AppliedManifestWork
resource_id=$(kubectl get appliedmanifestwork "$amw_name" \
  -o jsonpath='{.spec.manifestWorkName}')
```

## References

- Original documentation: [maestro.md](../../../../docs/maestro.md)
- CloudEvents spec: https://cloudevents.io/
- OCM ManifestWork: https://open-cluster-management.io/concepts/manifestwork/
