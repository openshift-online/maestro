# Error Analysis Reference

This document provides detailed troubleshooting guidance for common errors encountered when tracing resource requests through the Maestro system.

## Request Flow Overview

```
Client → Maestro Server → Message Broker → Maestro Agent → Applied Manifests
                ↓                               ↓
           Database (Events)              Status Updates
                ↑                               ↓
           Database                    Message Broker
                ↑                               ↓
           Broadcast ← Status Processing ← Maestro Server
                ↓
         gRPC Clients
```

## Error Categories

### 1. Server Does Not Receive Spec Request

**Symptoms:**
- No log entry with "receive the event from client"
- No matching op-id in server logs

**Diagnostic Steps:**
1. Check gRPC client connection to Maestro server
2. Verify network connectivity between client and server
3. Check if gRPC server is running: `grep "Starting gRPC server" maestro*.log`
4. Review client-side errors for publish failures

**Common Causes:**
- gRPC server not started or crashed
- Network connectivity issues
- Client authentication/authorization failures
- Invalid CloudEvent format from client

**Resolution:**
- Verify gRPC server is running on the correct port (default: 8090)
- Check firewall rules and network policies
- Validate client credentials and permissions
- Review CloudEvent payload format

---

### 2. Server Does Not Publish Spec Request

**Symptoms:**
- Event received but not published to message broker
- Missing "Publishing resource" or "Sending event" log entries
- No database event record created

**Diagnostic Steps:**
1. Check database for resource and event records:
   ```sql
   SELECT jsonb_pretty(payload) FROM resources WHERE id = '<resource_id>';
   -- The reconciled_date should not be null in normal case
   SELECT id, event_type, reconciled_date FROM events WHERE source_id = '<resource_id>';
   ```

2. Check for PostgreSQL notification errors:
   ```bash
   grep "recreate the listener\|stopping channel" maestro*.log
   ```

3. Verify message broker connectivity:
   - MQTT: Check MQTT broker connection logs
   - gRPC: Verify gRPC event server is running

**Common Causes:**
- Database connection lost
- PostgreSQL LISTEN/NOTIFY channel broken
- Message broker (MQTT/gRPC) connection failure
- Database transaction failure during event creation

**Resolution:**
- Restart Maestro server to recreate database listeners
- Check database connection health
- Verify message broker (MQTT/gRPC) is running and accessible
- Review database logs for transaction errors

---

### 3. Agent Does Not Receive Spec Request

**Symptoms:**
- Server publishes but agent doesn't receive
- No "Received event" log in agent logs
- Missing resourceid in agent logs

**Diagnostic Steps:**
1. Check MQTT/gRPC subscription in agent logs:
   ```bash
   grep "subscribed to.*broker" maestro-agent*.log
   grep "failed to receive cloudevents" maestro-agent*.log
   ```

2. Verify message broker connectivity:
   ```bash
   # For MQTT
   grep "mqtt is connected" maestro-agent*.log

   # For gRPC
   grep "start the cloudevents receiver" maestro-agent*.log
   ```

3. Check server publish errors:
   ```bash
   grep "Failed to publish resource" maestro*.log
   ```

**Common Causes:**
- MQTT/gRPC broker not accessible from agent
- Topic/subscription mismatch
- Network connectivity issues
- Agent consumer name configuration mismatch

**Resolution:**
- Verify MQTT broker host and port configuration
- Check topic format: `sources/maestro/consumers/{consumer-id}/sourceevents`
- Validate network policies between broker and agent
- Ensure agent `--consumer-name` matches server configuration

---

### 4. Agent Does Not Handle Spec Request

**Symptoms:**
- Agent receives event but doesn't apply manifests
- No manifest creation/update events in agent logs
- ManifestWork not created or updated

**Diagnostic Steps:**
1. Search for manifest application events:
   ```bash
   # For create/update
   grep "Server Side Applied\|Created\|Updated" maestro-agent*.log

   # For deletion
   grep "Resource.*is removed Successfully\|ResourceDeleted" maestro-agent*.log
   ```

2. Check for controller errors:
   ```bash
   grep "error\|Error\|failed" maestro-agent*.log | grep -i manifest
   ```

3. Verify ManifestWork controller is running:
   ```bash
   grep "Starting worker of controller.*ManifestWorkController" maestro-agent*.log
   ```

**Common Causes:**
- ManifestWork controller not started
- Invalid manifest format in the resource bundle
- Kubernetes API server not accessible
- RBAC permissions insufficient for applying manifests
- Resource conflicts (e.g., immutable fields changed)

**Resolution:**
- Restart agent to ensure all controllers are running
- Validate manifest YAML syntax and Kubernetes API version
- Check agent ServiceAccount permissions
- Review Kubernetes API server connectivity
- Check for resource ownership conflicts

---

### 5. Agent Does Not Publish Status Update

**Symptoms:**
- Manifests applied but no status update sent
- No "Sending event" log in agent after manifest application

**Diagnostic Steps:**
1. Check for publish errors in agent:
   ```bash
   grep "Failed to publish\|publish error" maestro-agent*.log
   ```

2. Verify message broker connection from agent:
   ```bash
   grep "mqtt is connected\|cloudevents receiver" maestro-agent*.log
   ```

3. Check if status controller is running:
   ```bash
   grep "Starting worker of controller.*AvailableStatusController" maestro-agent*.log
   ```

**Common Causes:**
- Message broker (MQTT/gRPC) connection lost
- Status controller crashed or not started
- CloudEvent serialization failure
- Topic publishing permission issues

**Resolution:**
- Restart agent to restore message broker connection
- Verify MQTT/gRPC broker is accessible
- Check agent topic publishing permissions
- Review CloudEvent payload generation logic

---

### 6. Server Does Not Receive Status Update

**Symptoms:**
- Agent sends status but server doesn't receive
- No "received status update for resource" log in server

**Diagnostic Steps:**
1. Check server subscription to message broker:
   ```bash
   grep "subscribed to.*broker" maestro*.log
   grep "failed to receive cloudevents" maestro*.log
   ```

2. Verify agent is publishing to correct topic:
   ```bash
   grep "Sending event.*status.update_request" maestro-agent*.log
   ```

3. Check message broker logs for delivery issues

**Common Causes:**
- Server MQTT/gRPC subscription lost
- Topic name mismatch between agent publish and server subscribe
- Message broker routing issues
- Network connectivity problems

**Resolution:**
- Restart Maestro server to recreate subscriptions
- Verify topic format: `sources/maestro/consumers/+/agentevents`
- Check message broker health and routing configuration
- Validate network connectivity

---

### 7. Server Does Not Handle Status Update

**Symptoms:**
- Server receives status but doesn't process it
- No "Updating resource status" log
- Database status_events table not updated

**Diagnostic Steps:**
1. Check for consumer name mismatch:
   ```bash
   grep "unmatched consumer name" maestro*.log
   ```

2. Check for decode errors:
   ```bash
   grep "failed to convert resource\|failed to decode cloudevent" maestro*.log
   ```

3. Check for database errors:
   ```bash
   grep "failed to create status event" maestro*.log
   ```

4. Verify status events in database:
   ```sql
   SELECT id, status_event_type FROM status_events WHERE resource_id = '<resource_id>';
   ```

5. Check for PostgreSQL notification errors:
   ```bash
   grep "recreate the listener\|stopping channel" maestro*.log
   ```

**Common Causes:**
- Consumer name mismatch (agent vs server configuration)
- Invalid CloudEvent format from agent
- Database connection issues
- PostgreSQL LISTEN/NOTIFY channel broken
- Status event type not recognized

**Resolution:**
- Verify agent `--consumer-name` matches the consumer record in database
- Validate CloudEvent payload structure
- Restart server to recreate database listeners
- Check database connection and transaction health

---

### 8. Server Does Not Broadcast Status Update

**Symptoms:**
- Status event processed but not broadcasted
- No "Broadcast the resource status" log
- Status not sent to other server instances

**Diagnostic Steps:**
1. Check for database errors:
   ```bash
   grep "failed to get status event\|failed to get resource" maestro*.log
   ```

2. Verify broadcast subscription type:
   ```bash
   grep "subscription-type" maestro*.log
   ```

3. Check database for status events:
   ```sql
   SELECT * FROM status_events WHERE resource_id = '<resource_id>' ORDER BY created_at DESC LIMIT 10;
   ```

**Common Causes:**
- Database query failures
- Resource or status event not found in database
- Broadcast subscription not configured
- Database replication lag

**Resolution:**
- Verify database connectivity and query performance
- Check resource and status_event records exist in database
- Review server `--subscription-type` configuration
- Monitor database replication status

---

### 9. Server Does Not Publish Status to Clients

**Symptoms:**
- Status broadcasted but not sent to gRPC clients
- "no clients registered on this instance" log message (may be normal)

**Diagnostic Steps:**
1. Check if gRPC clients are connected:
   ```bash
   grep "registered a broadcaster client" maestro*.log
   grep "unregistered broadcaster client" maestro*.log
   ```

2. Check for gRPC publish errors:
   ```bash
   grep "failed to handle resource\|failed to send" maestro*.log
   ```

3. Verify "send the event to status subscribers" log exists

**Common Causes:**
- No gRPC clients currently connected (normal if using broadcast mode)
- gRPC connection errors to clients
- Client disconnected before status update
- Multiple server instances (only one handles each update in broadcast mode)

**Resolution:**
- "no clients registered" is normal when:
  - Using `--subscription-type=broadcast` (other instances handle it)
  - Client disconnected after sending request
- Check gRPC client connection health
- Verify load balancing across multiple server instances

---

### 10. Delete Request Received Repeatedly But Never Completes

**Symptoms:**
- Repeated `"receive the event from client"` / `eventType=...delete_request` log entries for the
  same `resourceid` at short, regular intervals (the source retrying), with **no other server-side
  log entries** in between
- The resource-bundle is never removed; `GET /api/maestro/v1/resource-bundles/<id>` keeps
  returning it
- The `stalled_delete_resources` gauge metric (see below) is non-zero and/or growing

**Root cause:** `MarkAsDeleting` is intentionally idempotent: subsequent source `delete_request`
retries do not enqueue duplicate events. Separately, the `DeleteRecovery` controller republishes
unacknowledged deletes with persistent, bounded exponential backoff (starting at
`--delete-event-republish-interval`, capped by `--delete-event-republish-max-interval`). The
resource remains soft-deleted until the agent reports the `ResourceDeleted` ("Deleted") status
condition. Thus a repeated-request no-op does not mean Maestro has stopped retrying; it means
the background recovery scheduler owns retries. `StalledDeleteMonitor` reports old tombstones
for diagnosis only and does not mutate recovery state.

**Diagnostic Steps:**

1. Confirm Maestro already processed the first delete request (not silently dropping every one):
   ```bash
   grep '"resource marked as deleting; initial delete event enqueued for the agent"' maestro*.log
   grep '"resource is already marked as deleting; skipping duplicate delete event; background recovery remains responsible for retries"' maestro*.log
   ```
   Seeing the first line once, followed by the second line on retries, confirms the original
   request created a delete event and later caller retries were idempotent. It does not prove
   recovery republication or broker delivery; check the delete-recovery backlog/event metrics and
   the agent logs as well. If you see the retry line with no matching first line in earlier logs,
   the resource was already soft-deleted before this log window started; check
   `resources.deleted_at` in the DB (query below) for how long ago.

2. Check whether the agent is even communicating about this resource:
   ```bash
   grep 'resourceid="<resource-id>"' maestro-agent*.log | grep -i 'status.update_request\|Received event'
   ```
   - If the agent **is** sending status updates but they keep getting processed as
     `"Updating resource status"` (not `"resource status delete event was sent"`), the agent's
     ManifestWork has not yet reported the `Deleted` condition - see agent-side diagnostics below.
   - If the agent sends **nothing** for this resource, see §3 (Agent Does Not Receive Spec Request)
     or §6 (Server Does Not Receive Status Update) - the agent may be gone or disconnected.
     `DeleteRecovery` continues retrying according to its configured backoff. The stale-event
     detector can retire an old unreconciled event to prevent it from starving the spec-event
     worker, but a later eligible recovery round can publish a replacement event. The resource
     itself remains soft-deleted until the agent reconnects and confirms deletion or it is
     explicitly cleaned up.

3. Check the `stalled_delete_resources` gauge and server logs from `StalledDeleteMonitor` (fires
   every ~2 minutes once a soft-deleted resource exceeds `--stalled-delete-warning-threshold`,
   default 300s/5 minutes; purely diagnostic, does not mutate anything). The recovery scheduler
   also exposes aggregate tombstone backlog and age metrics; see `docs/resources/delete-recovery.md`:
   ```bash
   grep '"resource has been soft-deleted longer than the stalled-delete threshold' maestro*.log
   grep '"Found resources stalled in deletion"' maestro*.log
   ```
   These logs include aggregate count/threshold and a bounded sample of deletion timestamps and
   ages, but intentionally omit resource and consumer identifiers. Use the database query below
   to identify specific resources when authorized; the sample is not the full backlog.

4. On the agent/spoke cluster, check whether the ManifestWork still carries the cleanup finalizer:
   ```bash
   oc get manifestwork <name> -n <cluster-namespace> -o jsonpath='{.metadata.finalizers}{"\n"}{.metadata.deletionTimestamp}{"\n"}'
   ```
   If `cluster.open-cluster-management.io/manifest-work-cleanup` is still present, the agent will
   never report `Deleted=true` (see sdk-go's `ManifestWorkAgentClient.deleteWork`, which gates on
   `len(work.Finalizers) == 0`). That finalizer is only removed once the ManifestWork's
   `AppliedManifestWork` is fully gone, which in turn requires every applied resource on the spoke
   to finish finalizing:
   ```bash
   oc get appliedmanifestwork -o wide | grep <manifestwork-uid>
   oc get appliedmanifestwork <name> -o jsonpath='{.status.appliedResources}' | jq .
   ```
   Look for any applied resource stuck in `Terminating` (its own finalizer, a slow controller, or
   blocked garbage collection) - this is the most common reason a delete never completes.

**Database Queries:**

```sql
-- How long has this resource been soft-deleted, and is its delete event reconciled
-- (i.e. was the delete_request successfully handed to the message broker)?
SELECT
    r.id,
    r.consumer_name,
    r.deleted_at,
    now() - r.deleted_at AS age,
    e.id AS delete_event_id,
    e.reconciled_date
FROM resources r
LEFT JOIN events e
    ON e.source_id = r.id AND e.source = 'Resources' AND e.event_type = 'Delete'
WHERE r.id = '<resource_id>';

-- List soft-deleted resources, oldest first, for an authorized per-resource investigation:
SELECT id, consumer_name, deleted_at, now() - deleted_at AS age
FROM resources
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at ASC;
```

**Common Causes:**
- An applied Kubernetes resource on the managed cluster stuck `Terminating` (own finalizer, slow
  controller, or blocked background/foreground garbage collection) - blocks
  `AppliedManifestWork` cleanup, which blocks the ManifestWork's `manifest-work-cleanup` finalizer,
  which blocks the agent from ever reporting `Deleted=true`
- The agent/spoke cluster is unreachable or the work-agent pod is down, so it never processes the
  deletion at all
- An orphaned `AppliedManifestWork` (its `ManifestWork` already gone) - handled separately by the
  agent's `UnManagedAppliedWorkController` with a much longer (default 60 minute) grace period

**Resolution:**
- Identify and manually resolve the stuck applied resource's finalizer/termination on the spoke
  cluster (do not force-delete Maestro-side state without understanding why the resource is stuck,
  or the managed-cluster resource can be orphaned)
- If the agent is confirmed gone permanently and the managed cluster itself no longer exists, this
  is a case for manual cleanup rather than waiting on `StaleDeleteDetector`
- See ARO-28432 / AROSLSRE-1547 / AROSLSRE-1633 for related historical fixes to Maestro's delete
  handling (none of which cause this specific symptom - they addressed Maestro re-publishing a spec
  or starving its own event queue during deletion, not the agent-side finalizer deadlock)

---

## Subscription Type Behavior

### Broadcast Mode (`--subscription-type=broadcast`)

In broadcast mode, only ONE Maestro server instance handles each status update. Other instances will log:
```
"skipping resource status update" resourceID="<resource_id>"
```

This is **normal behavior** and not an error. The status update is still processed by one instance and broadcasted to all registered clients.

---

## Log Patterns Reference

### Success Patterns

| Phase | Log Pattern | What It Means |
|-------|-------------|---------------|
| Spec Request | `"receive the event from client"` + op-id | Server received resource spec from client |
| Spec Publish | `"Publishing resource"` + resourceID | Server publishing to message broker |
| Spec Send | `"Sending event"` + eventType=spec.* | CloudEvent sent to broker |
| Agent Receive | `"Received event"` + eventType=spec.* | Agent received spec from broker |
| Agent Apply | `"Server Side Applied"` or `"Created"` or `"Updated"` | Manifest applied to cluster |
| Agent Status | `"Sending event"` + eventType=status.* | Agent sending status update |
| Server Status | `"received status update for resource"` | Server received status from agent |
| Status Process | `"Updating resource status"` | Server processing status update |
| Status Broadcast | `"Broadcast the resource status"` | Status broadcasted to other instances |
| Status Publish | `"send the event to status subscribers"` | Status sent to gRPC clients |
| Delete Enqueued | `"resource marked as deleting; initial delete event enqueued for the agent"` | First delete request accepted; background recovery owns later retries |
| Delete Retry (expected) | `"resource is already marked as deleting; skipping duplicate delete event; background recovery remains responsible for retries"` | Repeated caller request correctly no-op'd; background recovery continues independently, see §10 |
| Delete Stalled | `"resource has been soft-deleted longer than the stalled-delete threshold"` | `StalledDeleteMonitor` diagnostic: resource has been deleting longer than `--stalled-delete-warning-threshold` (default 5m), see §10 |
| Delete Complete | `"resource status delete event was sent"` | Agent confirmed `Deleted=true`; resource hard-deleted |

### Error Patterns

| Error Pattern | Meaning | Reference Section |
|---------------|---------|-------------------|
| `Failed to publish resource` | Message broker publish failure | §2, §3, §5 |
| `failed to receive cloudevents` | Subscription/receive failure | §3, §6 |
| `unmatched consumer name` | Consumer configuration mismatch | §7 |
| `failed to convert resource` | CloudEvent decode error | §7 |
| `failed to decode cloudevent` | CloudEvent decode error | §7 |
| `failed to create status event` | Database write failure | §7 |
| `recreate the listener` | PostgreSQL notification channel broken | §2, §7 |
| `stopping channel` | PostgreSQL notification channel stopped | §2, §7 |
| `failed to get status event` | Database read failure | §8 |
| `failed to handle resource` | Resource processing error | §9 |
| `failed to send heartbeat` | gRPC connection issue | §9 |
| `failed to send event` | gRPC publish error | §9 |

---

## Database Queries for Debugging

### Check Resource Status
```sql
-- View full resource payload
SELECT
    id,
    consumer_name,
    version,
    created_at,
    updated_at,
    jsonb_pretty(payload) as payload,
    jsonb_pretty(status) as status
FROM resources
WHERE id = '<resource_id>';
```

### Check Events
```sql
-- List all events for a resource
SELECT
    id,
    event_type,
    source_id as resource_id,
    reconciled_date,
    created_at
FROM events
WHERE source_id = '<resource_id>'
ORDER BY created_at DESC;

-- Check for unreconciled events
SELECT COUNT(*) as unreconciled_count
FROM events
WHERE source_id = '<resource_id>'
  AND reconciled_date IS NULL;
```

### Check Status Events
```sql
-- List status events for a resource
SELECT
    id,
    resource_id,
    status_event_type,
    created_at,
    reconciled_at
FROM status_events
WHERE resource_id = '<resource_id>'
ORDER BY created_at DESC;

-- Check for unreconciled status events
SELECT COUNT(*) as unreconciled_status_count
FROM status_events
WHERE resource_id = '<resource_id>'
  AND reconciled_at IS NULL;
```

### Find Resource by Work Name
```sql
-- Find resource ID from work name
SELECT
    id as resource_id,
    payload->'metadata'->>'name' as work_name,
    consumer_name,
    created_at
FROM resources
WHERE payload->'metadata'->>'name' = '<work_name>';
```

---

## Next Steps

After identifying the failure point using this reference:

1. **Review relevant server/agent logs** for detailed error messages
2. **Check database state** using the queries above
3. **Verify component health**: server, agent, message broker, database
4. **Test connectivity** between components
5. **Review configuration** for consumer names, topics, and subscription types
6. **Restart components** if necessary to restore connections

For more information:
- See SKILL.md for step-by-step tracing procedures
- See scripts/trace_request.sh for automated log analysis
- See scripts/dump_logs.sh for collecting logs from Kubernetes clusters
