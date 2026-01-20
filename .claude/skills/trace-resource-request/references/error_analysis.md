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
