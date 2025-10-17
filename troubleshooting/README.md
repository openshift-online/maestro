# Troubleshooting

This directory contains troubleshooting guides, runbooks, and reference materials for diagnosing and resolving issues in Maestro.

## Overview

Maestro is a distributed system that uses CloudEvents to transport Kubernetes resources between the server and agents. When troubleshooting issues, you'll typically need to:

1. **Query resources** - Find resource IDs and work names in the database
2. **Trace requests** - Follow the flow of resource requests through server and agent logs
3. **Inspect manifests** - Examine the actual Kubernetes resources being deployed

## Available Runbooks

This section describes the available runbooks and when to use them for diagnosing Maestro issues.

### 1. Query Resource ([runbooks/query_resource.md](./runbooks/query_resource.md))

**Use this runbook when you need to:**
- Find the Maestro resource bundle ID for a given ManifestWork name
- Find the ManifestWork name for a given resource bundle ID
- Access the Maestro database to inspect resource records

**Common scenarios:**
- You have a work name from the gRPC client and need to trace it in the database
- You have a resource ID from logs and want to see the corresponding work
- You need to prepare resource IDs before running trace scripts

**What it provides:**
- Instructions to connect to the Maestro database (via postgres-breakglass or direct pod access)
- SQL queries to map between work names and resource IDs
- Commands to find work names from manifest metadata

### 2. Trace Resource Request ([runbooks/trace_resource_request.md](./runbooks/trace_resource_request.md))

**Use this runbook when you need to:**
- Trace the complete flow of a resource create/update/delete request
- Diagnose why a resource isn't being delivered to the agent
- Investigate status update failures
- Understand timing and coordination between server instances

**Common scenarios:**
- Resource created via API but not appearing on target cluster
- Status updates from agent not reflected in server database
- gRPC publish errors or MQTT broker connectivity issues
- Multi-instance coordination problems (resource ownership conflicts)

**What it provides:**
- Instructions to collect logs from Kusto or directly from Maestro pods
- Scripts to generate trace logs showing the complete request lifecycle
- Comprehensive error analysis section covering:
  - Server not receiving/publishing spec requests
  - Agent not receiving/handling spec requests
  - Agent not publishing status updates
  - Server not receiving/broadcasting/publishing status updates
- Database queries to verify event records and status events

**Example trace log:**
See [trace_request.create.log](./trace_request.create.log) for a reference trace showing:
- Request received at server: `21:38:21.707Z`
- Published to broker: `21:38:21.772Z` (~65ms latency)
- Agent receives event: `21:38:21.841Z` (~134ms from request)
- First status update: `21:38:22.050Z` (~343ms end-to-end)
- Multi-instance coordination with ownership checks

### 3. Trace Work Manifests ([runbooks/trace_work_manifests.md](./runbooks/trace_work_manifests.md))

**Use this runbook when you need to:**
- Inspect the actual Kubernetes manifests within a ManifestWork
- Verify which resources are being applied to the target cluster
- Check the AppliedManifestWork status

**Common scenarios:**
- Verifying that the correct manifests are included in a work
- Checking the applied status of manifests on the management cluster
- Finding a ManifestWork by the name of a manifest it contains

**What it provides:**
- Scripts to retrieve and display manifests from a ManifestWork
- Commands to search for works by manifest kind/namespace/name
- Instructions to check corresponding AppliedManifestWork resources

## Troubleshooting Workflow

Follow this general workflow when diagnosing Maestro issues:

### Step 1: Identify the Resource

Start with what you know:

- **If you have a work name**: Use the [Query Resource](./runbooks/query_resource.md) runbook to find the resource ID
- **If you have a manifest name**: Use the [Trace Work Manifests](./runbooks/trace_work_manifests.md) runbook to find the work name, then proceed to query the resource ID
- **If you have a resource ID from logs**: Proceed directly to tracing

### Step 2: Trace the Request Flow

Use the [Trace Resource Request](./runbooks/trace_resource_request.md) runbook to:

1. Collect logs from the relevant time window (from Kusto or pods)
2. Run the appropriate trace script (create_request, update_request, or delete_request)
3. Review the generated trace log for timing and flow

### Step 3: Diagnose Specific Issues

If the trace reveals problems, use the error analysis section in the [Trace Resource Request](./runbooks/trace_resource_request.md) runbook:

| Symptom | Likely Cause | Check |
|---------|--------------|-------|
| Server doesn't receive spec request | gRPC client publish error | Search for "PublishError" in client logs |
| Server doesn't publish spec request | Database or notification error | Check events table, psql listener errors |
| Agent doesn't receive spec request | MQTT/gRPC broker issue | Check publish/subscription errors |
| Agent doesn't handle spec request | Application error | Look for event logs (Created/Updated/Deleted) |
| Agent doesn't publish status update | MQTT publish error | Search for "PublishError" in agent logs |
| Server doesn't receive status update | MQTT/gRPC subscription error | Check "failed to receive cloudevents" |
| Server doesn't handle status update | Consumer mismatch, decode error | Check consumer configuration, conversion errors |
| Server doesn't broadcast status update | Database error | Verify status_events table records |
| Status not sent to gRPC client | No registered clients | Check gRPC client connection |

### Step 4: Inspect Manifests (if needed)

If the issue is related to incorrect resources being deployed, use the [Trace Work Manifests](./runbooks/trace_work_manifests.md) runbook to inspect the actual manifests.

## Common CloudEvent Types

Understanding these event types helps when reviewing logs:

- `io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request` - New resource creation
- `io.open-cluster-management.works.v1alpha1.manifestbundles.spec.update_request` - Resource update
- `io.open-cluster-management.works.v1alpha1.manifestbundles.spec.delete_request` - Resource deletion
- `io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request` - Status update from agent

## Prerequisites

Before using the troubleshooting runbooks, ensure you have:

- Access to the service cluster (where Maestro server runs)
- Access to the management cluster (where Maestro agent runs)
- Appropriate KUBECONFIG files configured
- Database access credentials (if querying directly)
- For Kusto logs: Access to the HCPServiceLogs database
- For pod logs: Maestro components should be running with log level 4 (`-v=4`)
