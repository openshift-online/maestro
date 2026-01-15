# ManifestWork Troubleshooting Guide

Common issues and solutions when tracing ManifestWorks through the Maestro system.

## Cluster Architecture

**CRITICAL**: Maestro uses a dual-cluster architecture:
- **Service (svc) Cluster**: Runs Maestro Server and Database
- **Management (mgmt) Cluster**: Runs Maestro Agent and manifests

## Prerequisites Issues

Most tracing issues stem from using the wrong cluster context.

### Identifying Which Cluster You Need

Use this decision tree:

| What you want to access | Which cluster | Example context names |
|------------------------|---------------|---------------------|
| Database (postgres-breakglass, maestro-db) | Service cluster | `svc-cluster`, `aro-hcp-int` |
| AppliedManifestWorks | Management cluster | `mgmt-cluster`, `management` |
| Manifests (Deployments, Services, etc.) | Management cluster | `mgmt-cluster`, `management` |
| Maestro server logs | Service cluster | `svc-cluster`, `aro-hcp-int` |
| Maestro agent logs | Management cluster | `mgmt-cluster`, `management` |

### Quick Context Verification

```bash
# Save current context
CURRENT_CONTEXT=$(kubectl config current-context)

# Test service cluster (should find database pod)
kubectl config use-context <svc-cluster-context>
kubectl -n maestro get pods -l app=maestro
echo "✓ Service cluster verified" || echo "✗ Wrong service cluster"

# Test management cluster (should find AppliedManifestWorks)
kubectl config use-context <mgmt-cluster-context>
kubectl get pods -l app=maestro-agent --all-namespaces
kubectl get appliedmanifestworks | head -n 5
echo "✓ Management cluster verified" || echo "✗ Wrong management cluster"

# Restore original context
kubectl config use-context "$CURRENT_CONTEXT"
```

### Context Switching in Traces

When performing a complete trace:

1. **Start on management cluster** (if tracing from manifest)
2. **Switch to service cluster** for database queries
3. **Switch back to management cluster** for AppliedManifestWork queries

Example trace flow:
```bash
# 1. Start on mgmt cluster - get manifest owner
kubectl config use-context mgmt-cluster
AMW_NAME=$(kubectl get deployment test -o jsonpath='{.metadata.ownerReferences[0].name}')
RESOURCE_ID=$(kubectl get appliedmanifestwork $AMW_NAME -o jsonpath='{.spec.manifestWorkName}')

# 2. Switch to svc cluster - query database
kubectl config use-context svc-cluster
kubectl -n maestro exec -it maestro-db-xxx -- psql -U maestro -d maestro \
  -c "SELECT payload->'metadata'->>'name' FROM resources WHERE id='$RESOURCE_ID';"

# 3. Switch back to mgmt cluster - list manifests
kubectl config use-context mgmt-cluster
kubectl get appliedmanifestwork $AMW_NAME -o yaml
```

## Database Access Issues

### No Database Pod Found

**Symptom**: `ERROR: No database pod found`

**Solution**:
```bash
# Verify you're in the correct namespace
kubectl config set-context --current --namespace=maestro

# Check for postgres-breakglass (ARO-HCP INT)
kubectl -n maestro get pods -l app=postgres-breakglass

# Check for maestro-db (Service cluster)
kubectl -n maestro get pods -l name=maestro-db

# If neither exists, verify cluster and namespace
kubectl get namespaces | grep maestro
```

### postgres-breakglass Pod Not Ready

**Symptom**: `pod/postgres-breakglass-xxx not ready`

**Solution**:
```bash
# Scale up the deployment
kubectl -n maestro scale deployment postgres-breakglass --replicas 1

# Wait for pod to be ready (up to 60s)
kubectl -n maestro wait --for=condition=ready pod -l app=postgres-breakglass --timeout=60s

# Check pod status
kubectl -n maestro get pods -l app=postgres-breakglass

# If still not ready, check logs
kubectl -n maestro logs -l app=postgres-breakglass
```

## Resource Not Found Issues

### Work Not Found in Database

**Symptom**: `No resource found with this ID/name`

**Possible Causes**:
1. Work was never created
2. Work name is incorrect (typo)
3. Work was deleted (hard deleted - no record remains)

**Solutions**:

```sql
-- Search by partial name (for active works only)
SELECT id, payload->'metadata'->>'name' AS name, created_at, updated_at
FROM resources
WHERE payload->'metadata'->>'name' LIKE '%partial-name%'
ORDER BY created_at DESC
LIMIT 10;

-- Check for works currently being deleted (transient state - rare)
-- This will usually return 0 rows because deletion happens quickly
SELECT id, payload->'metadata'->>'name' AS name, created_at, deleted_at,
       NOW() - deleted_at AS deletion_age
FROM resources
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC
LIMIT 10;

-- Search by creation time range (active works only)
SELECT id, payload->'metadata'->>'name' AS name, created_at, updated_at
FROM resources
WHERE created_at >= '2024-01-15 10:00:00'
  AND created_at <= '2024-01-15 11:00:00'
ORDER BY created_at DESC;
```

**Important**: Maestro performs **hard deletes**. Once a work is deleted:
- The resource is completely removed from the database
- No historical record exists
- You cannot query for deleted works
- The `deleted_at` field is only set temporarily during deletion (seconds to minutes)
- Once deletion completes, the row is permanently deleted

If you need historical data, implement external audit logging before deletion occurs.

### Manifest Not Found

**Symptom**: `Manifest {kind}/{namespace}/{name} not found`

**Possible Causes**:
1. Manifest was deleted
2. Wrong namespace
3. Wrong kind or name
4. Manifest not yet created

**Solutions**:

```bash
# List all resources of that kind
kubectl get <kind> -n <namespace>

# Search across all namespaces
kubectl get <kind> -A

# Verify manifest is in AppliedManifestWork status
kubectl get appliedmanifestworks -o json | \
  jq '.items[].status.appliedResources[] | select(.name == "<name>")'
```

## Multiple Results Issues

**Symptom**: Multiple resources found with the same work name but different resource IDs

**Explanation**: It's possible to have multiple resources in the database with:
- Same user-created work name (`payload->'metadata'->>'name'`)
- Different resource IDs (primary key `id`)
- Different sources (`payload->>'source'`)

This happens when the same work name is used across different sources or clusters.

**Example Query:**
```sql
-- Show all resources with the same work name
SELECT id, payload->>'source' AS source, payload->'metadata'->>'name' AS name, created_at
FROM resources
WHERE payload->'metadata'->>'name' = 'my-work-name'
ORDER BY created_at DESC;
```

**To identify the correct resource:**

1. **Filter by source:**
```sql
SELECT id, payload->>'source' AS source, payload->'metadata'->>'name' AS name, created_at
FROM resources
WHERE payload->'metadata'->>'name' = 'my-work-name'
  AND payload->>'source' = 'expected-source-name'
ORDER BY created_at DESC;
```

2. **Filter by date range:**
```sql
SELECT id, payload->>'source' AS source, payload->'metadata'->>'name' AS name, created_at
FROM resources
WHERE payload->'metadata'->>'name' = 'my-work-name'
  AND created_at >= '2024-01-15 00:00:00'
  AND created_at <= '2024-01-15 23:59:59'
ORDER BY created_at DESC;
```

3. **Use the resource ID directly if known:**
```sql
SELECT * FROM resources WHERE id = 'specific-resource-id';
```

## Data Inconsistency Issues

### Resource in DB but Not on Cluster

**Symptom**: Database shows resource (with `deleted_at = NULL`), but AppliedManifestWork not found

**Possible Causes**:
1. Server failed to publish the resource
2. Agent hasn't processed the resource yet (new resource) - wait a few seconds
3. Agent failed to apply the resource
4. Work is currently being deleted (`deleted_at` is set) - check for deletion in progress

**Solutions**:

```bash
# Check if work is being deleted
# (Run this SQL query on the database)
SELECT deleted_at FROM resources WHERE id = '<resource-id>';
# If deleted_at is NOT NULL, work is being deleted - wait for hard delete

# Check server logs for errors
kubectl logs -n maestro -l app=maestro --tail=100 | grep <resource-id>

# Check agent logs for errors
kubectl logs -n maestro-agent -l app=maestro-agent --tail=100 | grep <resource-id>

# For new resources, wait a few seconds and retry
# The agent may be processing the work
```

## Getting Help

If issues persist:

1. **Collect diagnostic information**:
   ```bash
   # Resource states
   kubectl get appliedmanifestworks -o yaml > amw-dump.yaml

   # Server logs
   kubectl logs -n maestro -l app=maestro --tail=1000 > server-logs.txt

   # Agent logs
   kubectl logs -n maestro-agent -l app=maestro-agent --tail=1000 > agent-logs.txt
   ```

2. **Check Maestro documentation**: [maestro.md](../../../../docs/maestro.md)

3. **Review CloudEvents spec**: https://cloudevents.io/

4. **Contact Maestro team** with diagnostic information
