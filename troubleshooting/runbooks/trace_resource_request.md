# Trace Resource Request

## Prepare

To trace the resource requests, Maestro logs should be collected in advance, there are two options:

1. Collect the logs from Kusto, or
2. Collect the logs from Maestro pods.

### Export the logs from Kusto

1. Export Maestro server logs from Kusto

```kql
// limit the query time window (e.g., to 5 minutes) to avoid overwhelming log sizes
let start_time = datetime(2025-09-17T21:36:00Z);
let end_time = datetime("2025-09-17T21:40:00Z");
database('HCPServiceLogs').table('kubesystem')
| where TIMESTAMP between (start_time .. end_time)
| where namespace_name == "maestro"
  and container_name contains "service"
| where log contains "receive the event from client"
  or log contains "Publishing resource"
  or log contains "received action STATUSMODIFIED"
  or log contains "skipping resource status update"
  or log contains "status update event was sent"
  or log contains "status delete event was sent"
  or log contains "Broadcast the resource status"
  or log contains "send the event to status subscribers"
  or log contains "io.open-cluster-management.works.v1alpha1.manifestbundles"
  or log contains "metadata"
| project pod_name, log
```

2. Export Maestro agent logs from Kusto

```kql
// limit the query time window (e.g., to 5 minutes) to avoid overwhelming log sizes
let start_time = datetime(2025-09-17T21:36:00Z);
let end_time = datetime("2025-09-17T21:40:00Z");
database('HCPServiceLogs').table('kubesystem')
| where TIMESTAMP between (start_time .. end_time)
| where namespace_name == "maestro"
  and container_name contains "maestro-agent"
| where not (log contains "Object is patched")
  and not (log contains "Patching resource")
  and (log contains "Received event"
  or log contains "Receive the event"
  or log contains "Sending event"
  or log contains "io.open-cluster-management.works.v1alpha1.manifestbundles"
  or log contains "metadata")
| project pod_name, log
```

and also export Maestro agent event logs from Kusto

```kql
let start_time = datetime(2025-09-17T21:36:00Z);
let end_time = datetime("2025-09-17T21:40:00Z");
database('HCPServiceLogs').table('kubesystem')
| where TIMESTAMP between (start_time .. end_time)
| where namespace_name == "maestro"
  and container_name contains "maestro-agent"
| where log contains "Event(v1.ObjectReference"
| project pod_name, log
```

**Note**: It is recommended to install the Ubuntu terminal environment using WSL on your Microsoft Virtual Desktop, and to install dos2unix in that environment (`sudo apt-get install dos2unix`).
After exporting logs from Kusto, use `dos2unix -f <filename>` to convert the line endings in the log files.

### Dump the logs from Maestro pods

1. In the service cluster, dump the Maestro server logs

```sh
export KUBECONFIG=<your_service_cluster_kubeconfig>
namespace="maestro" label="app=maestro" container="service" troubleshooting/scripts/dump_logs.sh
```

2. In the management cluster, dump the Maestro agent logs

```sh
export KUBECONFIG=<your_management_cluster_kubeconfig>
namespace="maestro-agent" label="app=maestro-agent" container="maestro-agent" troubleshooting/scripts/dump_logs.sh
```

**Note**: The log level of Maestro components should set to 4 (`-v=4`)

## Trace the resource request

### Trace the resource create/update/delete request

- If you use the Kusto logs, run following command

```sh
# svr_log: Maestro server log file exported from Kusto
# agent_log: Maestro agent log file exported from Kusto
# work_name: The name of the ManifestWork created by the Maestro gRPC client
# resource_id: The ID of the resource bundle corresponding to the ManifestWork
# resource_request: The type of resource request, it can be create_request, update_request or delete_request
svr_log="<maestro_server_log>" agent_log="<maestro_agent_log>" work_name="<work_name>" resource_id="<resource_id>" resource_request="<resource_request>" scripts/trace_request_kusto.sh
```

- If you use the Maestro pod logs, run following command

```sh
# logs_dir: Maestro logs dumped from pods
# work_name: The name of the ManifestWork created by the Maestro gRPC client
# resource_id: The ID of the resource bundle corresponding to the ManifestWork
# resource_request: The type of resource request, it can be create_request, update_request or delete_request
logs_dir="<logs_dir>" work_name="<work_name>" resource_id="<resource_id>" resource_request="<resource_request>" scripts/trace_request.sh
```

To find the find the `resource_id`
- Search the Maestro server log with `grep metadata | grep <work_name> | grep uid`, or
- Following the steps in [Query Resource](./query_resource.md)

The above command will output a trace log named trace_request.<timestamp>.log, which shows the flow of the resource request between the Maestro server and the agent

If the command outputs errors, refer to the following section for further analysis

### Error analysis

#### ERROR: Maestro server does not receive the spec request

- Check if there is a gRPC publish error in the Maestro gRPC client (error reason: PublishError, and error message contains "Failed to publish resource")

#### ERROR: Maestro server does not publish the spec request
- Check if there are resource and spec event records in the database
```sql
SELECT jsonb_pretty(payload) FROM resources WHERE id = '<resource_id>';
-- the reconciled_date should not be null in normal case
SELECT id, event_type, reconciled_date FROM events WHERE source_id = '<resource_id>';
```
- If the event is created, check if there is psql notification errors in the maestro server log (the log contains "recreate the listener" or "stopping channel")

#### ERROR: Maestro agent does not receive the spec request

- Check if there is a mqtt publish error in the Maestro server (error reason: PublishError, and error message contains "Failed to publish resource")
- Check if there is a mqtt subscription error in the Maestro agent (the error message contains "failed to receive cloudevents")

#### ERROR: Maestro agent does not handle the spec request

- Check if there are any errors in the Maestro agent logs, in normal case, the below event logs should be found

```
# manifest create/update, search the Created/Updated or Server Side Applied
Event(v1.ObjectReference{Kind:""Deployment"", Namespace:""maestro"", Name:""maestro-agent"", ...}): type: 'Normal' reason: 'Server Side Applied' ...."

# manifest deletion
Resource <manifest-type> with key <manifest-namespace>/<manifest-name> is removed Successfully
Event(v1.ObjectReference{Kind:""Deployment"", Namespace:""maestro"", Name:""maestro-agent"", ...}): type: 'Normal' reason: 'ResourceDeleted' Deleted resource work.open-cluster-management.io/v1, Resource=manifestworks with key <manifestwork> because manifestwork <manifestwork-name> is terminating."
```

#### ERROR: Maestro agent does not publish the status update request

- Check if there is a mqtt publish error in the Maestro agent (error reason: PublishError, and error message contains "Failed to publish resource")
- Check if there is a mqtt subscribe error in the Maestro server (the error message contains "failed to receive cloudevents")

#### ERROR: Maestro server does not receive the status update request

- Check if there is a mqtt publish error in the Maestro agent (error reason: PublishError, and error message contains "Failed to publish resource")
- Check if there is a mqtt subscribe error in the Maestro server (the error message contains "failed to receive cloudevents")
- failed to handle resource status update

If the Maestro server `--subscription-type` is `broadcast`, there should only one Maestro server instance handle the status update, in other instances, the skipping log should be found (the log contains "skipping resource status update")

#### ERROR: Maestro server does not handle the status update request

- Check if there is consumer unmatched error (the log contains "unmatched consumer name"), if it occurs, checking the Maestro agent consumer configuration (`--consumer-name`)
- Check if there is decode error (the log contains "failed to convert resource" or "failed to decode cloudevent")
- Check if there is database error (the log contains "failed to create status event"), in normal case, the status event should be created

```sql
SELECT id, status_event_type FROM status_events WHERE resource_id = '<resource_id>';
```

- If the event is created, check if there is psql notification errors in the maestro server log (the log contains "recreate the listener" or "stopping channel")

#### ERROR: Maestro server does not broadcast the status update request

- Check if there are database error (the error message contains "failed to get status event" or "failed to get resource")

#### ERROR: Maestro server does not publish the status update request

- Check the log "no clients registered on this instance", if all maestro server instances have this log, check the connection between maestro gRPC client and maestro server
- Check if there is "failed to handle resource" log
- Check if there is any gRPC connection error (the error message contains "failed to send heartbeat", "failed to send event", or "failed to send event, unregister subscriber")
