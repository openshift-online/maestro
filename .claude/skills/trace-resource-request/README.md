# Trace Resource Request Skill

Complete tracing of resource requests through the Maestro system pipeline.

## Overview

This skill helps trace resource requests through their complete lifecycle in the Maestro system, from client submission through server processing, message broker delivery, agent application, and status updates back to clients.

## What This Skill Does

The skill provides end-to-end tracing through:
- Client → Maestro Server (gRPC request reception)
- Server → Message Broker (MQTT/gRPC publish)
- Message Broker → Maestro Agent (CloudEvent delivery)
- Agent → Kubernetes Cluster (manifest application)
- Agent → Message Broker (status updates)
- Message Broker → Server (status reception)
- Server → Database (status persistence)
- Server → gRPC Clients (status broadcast)

## Quick Start

### Trace by Operation ID (Recommended)

```bash
.claude/skills/trace-resource-request/scripts/trace_request.sh \
  --op-id "2dd9d768-a02d-4539-a99f-8a2c801a1c4b" \
  --logs-dir $HOME/maestro-logs
```

### Trace by Resource ID

```bash
.claude/skills/trace-resource-request/scripts/trace_request.sh \
  --resource-id "9936a444-051a-5658-9b57-af855e27b01b" \
  --logs-dir $HOME/maestro-logs
```

### Trace by Work Name

```bash
.claude/skills/trace-resource-request/scripts/trace_request.sh \
  --work-name "nginx-work" \
  --logs-dir $HOME/maestro-logs
```

### Collect Logs First

```bash
# From Maestro server cluster
namespace=maestro label="app=maestro" container=service logs_dir=$HOME/maestro-logs \
  .claude/skills/trace-resource-request/scripts/dump_logs.sh

# From Maestro agent cluster
namespace=maestro label="app=maestro-agent" container=agent logs_dir=$HOME/maestro-logs \
  .claude/skills/trace-resource-request/scripts/dump_logs.sh
```

## Prerequisites

- Bash shell
- Access to Maestro server and agent logs
- (Optional) `kubectl` for log collection from Kubernetes
- (Optional) PostgreSQL client for database verification

## Files in This Skill

### Scripts

- `scripts/dump_logs.sh` - Collect logs from Kubernetes clusters
- `scripts/trace_request.sh` - Automated log analysis and request tracing

### References

- `references/error_analysis.md` - Comprehensive error troubleshooting guide

## Common Use Cases

1. **Request never completes**: Client sends a request but never receives status update
2. **Agent not applying manifests**: Request reaches agent but manifests don't get created
3. **Status not updating**: Manifests applied but client doesn't receive status
4. **Debugging message broker**: MQTT/gRPC communication failures
5. **Understanding request flow**: Learn how requests propagate through the system
6. **Performance analysis**: Identify bottlenecks and delays in the request pipeline

## Key Concepts

### Identifiers

- **Operation ID (op-id)**: End-to-end request identifier (e.g., `2dd9d768-a02d-4539-a99f-8a2c801a1c4b`)
- **Resource ID**: Database primary key and CloudEvent ID (e.g., `9936a444-051a-5658-9b57-af855e27b01b`)
- **Work Name**: User-assigned ManifestWork name (e.g., `nginx-work`)
- **Manifest Name**: Kubernetes resource name (e.g., `nginx-deployment`)

### Request Flow

```
Client sends spec request (gRPC)
    ↓
Server receives and stores in DB
    ↓
Server publishes to message broker (MQTT/gRPC)
    ↓
Agent receives CloudEvent
    ↓
Agent applies manifests to cluster
    ↓
Agent publishes status update (MQTT/gRPC)
    ↓
Server receives status update
    ↓
Server updates DB and broadcasts
    ↓
Server sends status to gRPC clients
```

## Troubleshooting

See `references/error_analysis.md` for detailed troubleshooting steps covering:

- §1: Server Does Not Receive Spec Request
- §2: Server Does Not Publish Spec Request
- §3: Agent Does Not Receive Spec Request
- §4: Agent Does Not Handle Spec Request
- §5: Agent Does Not Publish Status Update
- §6: Server Does Not Receive Status Update
- §7: Server Does Not Handle Status Update
- §8: Server Does Not Broadcast Status Update
- §9: Server Does Not Publish Status to Clients

Common issues:
- Logs not found → Verify logs directory and file naming
- No matching entries → Check identifier correctness (typos, format)
- All stages fail → Verify logs are from correct time period
- Script errors → Ensure bash and grep are available
- Database access → Use `make db/login` or kubectl port-forward

## Usage Tips

1. **Start with op-id**: Provides the most comprehensive end-to-end view
2. **Collect both server and agent logs**: Issues can occur in either component
3. **Check timestamps**: Identify delays and timeout issues
4. **Compare with working requests**: Understand normal vs abnormal patterns
5. **Read error analysis**: Reference doc has detailed troubleshooting steps
6. **Verify database state**: Use SQL queries to confirm data persistence

## Example Workflow

**Problem**: Client reports resource never applied

1. Collect logs using `dump_logs.sh`
2. Run trace script with op-id or resource-id
3. Review trace output to find failure point
4. Consult `references/error_analysis.md` for that specific failure
5. Follow diagnostic steps and resolution guidance
6. Verify fix by tracing a new request

## Output Interpretation

The trace script outputs:
- ✅ **Found**: Stage completed successfully
- ⚠️ **No matching entries found**: Stage failed (identifies failure point)
- 🔍 **Error Analysis**: Automatically detected error patterns

Use "No matching entries" to identify where the request flow broke, then consult the error analysis reference for that specific section.

## Related Documentation

- SKILL.md: Complete step-by-step usage guide
- Main Maestro docs: Project README.md
- CloudEvents spec: https://cloudevents.io/
- OCM ManifestWork: https://open-cluster-management.io/concepts/manifestwork/

## Support

For issues or questions:
1. Review `SKILL.md` for step-by-step procedures
2. Check `references/error_analysis.md` for error-specific guidance
3. Verify component health (server, agent, broker, database)
4. Review Maestro project documentation
5. Contact Maestro team with trace output and diagnostic info
