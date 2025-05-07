# Maestro Metrics

This document describes the Prometheus metrics exposed by the Maestro server and Maestro agent. Each metric includes its name, type, a short description, and an example of the exported data.

## Maestro Server Metrics

Refer to [Access Maestro Server Metrics](https://github.com/openshift-online/maestro/edit/main/docs/troubleshooting.md#access-maestro-server-metrics) for detailed instructions on accessing the metrics in your runtime environment.

---
### `advisory_lock_count`

**Type:** `counter`\
**Help:** Number of advisory lock requests, categorized by status and type.

This counter tracks how many times an advisory lock has been requested.

**Example:**

```
# HELP advisory_lock_count Number of advisory lock requests.
# TYPE advisory_lock_count counter
advisory_lock_count{status="OK",type="events"} 13
advisory_lock_count{status="OK",type="instances"} 140
advisory_lock_count{status="OK",type="resource_status"} 22
advisory_lock_count{status="OK",type="resources"} 8
```

---

### `advisory_lock_duration`

**Type:** `histogram`\
**Help:** Advisory lock durations in seconds, categorized by status and type.

This histogram tracks how long advisory locks take, with multiple latency buckets.

**Example:**

```
# HELP advisory_lock_duration Advisory Lock durations in seconds.
# TYPE advisory_lock_duration histogram
advisory_lock_duration_bucket{status="OK",type="events",le="0.1"} 13
...
advisory_lock_duration_bucket{status="OK",type="resources",le="+Inf"} 8
advisory_lock_duration_sum{status="OK",type="resources"} 0.035487102
advisory_lock_duration_count{status="OK",type="resources"} 8
```

---

### `cloudevents_received_total`

**Type:** `counter`\
**Help:** The total number of received CloudEvents, categorized by action, cluster, source, subresource, and type.

**Example:**

```
# HELP cloudevents_received_total The total number of received CloudEvents.
# TYPE cloudevents_received_total counter
cloudevents_received_total{action="update_request",cluster="9894...",source="9894...-work-agent",subresource="status",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 22
```

---

### `cloudevents_sent_total`

**Type:** `counter`\
**Help:** The total number of sent CloudEvents, categorized by action, cluster, source, subresource, and type.

**Example:**

```
# HELP cloudevents_sent_total The total number of sent CloudEvents.
# TYPE cloudevents_sent_total counter
cloudevents_sent_total{action="resync_response",cluster="9894...",original_source="none",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 3
```

---

### `grpc_server_called_total`

**Type:** `counter`\
**Help:** Total number of RPCs called on the gRPC server.

**Example:**

```
# HELP grpc_server_called_total ...
# TYPE grpc_server_called_total counter
grpc_server_called_total{source="sourceclient-testr6dfx",type="Publish"} 13
```

---

### `grpc_server_message_received_total`

**Type:** `counter`\
**Help:** Messages received on the server from clients.

**Example:**

```
# HELP grpc_server_message_received_total ...
# TYPE grpc_server_message_received_total counter
grpc_server_message_received_total{source="sourceclient-testr6dfx",type="Publish"} 13
```

---

### `grpc_server_message_sent_total`

**Type:** `counter`\
**Help:** Messages sent from the server to agents.

**Example:**

```
# HELP grpc_server_message_sent_total ...
# TYPE grpc_server_message_sent_total counter
grpc_server_message_sent_total{source="sourceclient-testr6dfx",type="Subscribe"} 117
```

---

### `grpc_server_processed_duration_seconds`

**Type:** `histogram`\
**Help:** Duration of gRPC server calls in seconds.

**Example:**

```
# HELP grpc_server_processed_duration_seconds ...
# TYPE grpc_server_processed_duration_seconds histogram
grpc_server_processed_duration_seconds_bucket{source="sourceclient-testr6dfx",type="Publish",le="0.01"} 12
grpc_server_processed_duration_seconds_sum{source="sourceclient-testr6dfx",type="Publish"} 0.0992
grpc_server_processed_duration_seconds_count{source="sourceclient-testr6dfx",type="Publish"} 13
```

---
### `grpc_server_processed_total`

**Type:** `counter`\
**Help:** Total number of RPCs processed on the server, regardless of success or failure.

**Example:**

```
# HELP grpc_server_processed_total Total number of RPCs processed on the server, regardless of success or failure.
# TYPE grpc_server_processed_total counter
grpc_server_processed_total{code="OK",source="sourceclient-testr6dfx",type="Publish"} 13
grpc_server_processed_total{code="OK",source="sourceclient-testr6dfx",type="Subscribe"} 5
```

---

### `resource_processed_total`

**Type:** `counter`\
**Help:** Number of processed resources.

**Example:**

```
# HELP resource_processed_total Number of processed resources.
# TYPE resource_processed_total counter
resource_processed_total{action="update",id="2c74c9e5-dda7-5b74-a51c-c7a7114b44c3"} 2
resource_processed_total{action="update",id="4bd1408d-36f2-51a8-87aa-76d63dfd1d42"} 2
```

---

### `resources_spec_resync_duration_seconds`

**Type:** `histogram`\
**Help:** The duration of the resource spec resync in seconds.

**Example:**

```
# HELP resources_spec_resync_duration_seconds The duration of the resource spec resync in seconds.
# TYPE resources_spec_resync_duration_seconds histogram
resources_spec_resync_duration_seconds_bucket{cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",type="io.open-cluster-management.works.v1alpha1.manifestbundles",le="0.1"} 1
resources_spec_resync_duration_seconds_bucket{cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",type="io.open-cluster-management.works.v1alpha1.manifestbundles",le="0.2"} 1
resources_spec_resync_duration_seconds_bucket{cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",type="io.open-cluster-management.works.v1alpha1.manifestbundles",le="0.5"} 1
resources_spec_resync_duration_seconds_sum{cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 0.003581495
resources_spec_resync_duration_seconds_count{cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 1
```

---

### `rest_api_inbound_request_count`

**Type:** `counter`\
**Help:** Number of requests served.

**Example:**

```
# HELP rest_api_inbound_request_count Number of requests served.
# TYPE rest_api_inbound_request_count counter
rest_api_inbound_request_count{code="200",method="GET",path="/api/maestro/v1/resource-bundles"} 7
rest_api_inbound_request_count{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-"} 45
rest_api_inbound_request_count{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-"} 5
```

---

### `rest_api_inbound_request_duration`

**Type:** `histogram`\
**Help:** Request duration in seconds.

**Example:**

```
# HELP rest_api_inbound_request_duration Request duration in seconds.
# TYPE rest_api_inbound_request_duration histogram
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles",le="0.1"} 7
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles",le="1"} 7
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles",le="10"} 7
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles",le="30"} 7
rest_api_inbound_request_duration_sum{code="200",method="GET",path="/api/maestro/v1/resource-bundles"} 0.025571774
rest_api_inbound_request_duration_count{code="200",method="GET",path="/api/maestro/v1/resource-bundles"} 7
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-",le="0.1"} 45
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-",le="1"} 45
rest_api_inbound_request_duration_bucket{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-",le="10"} 45
rest_api_inbound_request_duration_sum{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-"} 0.09214890999999999
rest_api_inbound_request_duration_count{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-"} 45
rest_api_inbound_request_duration_bucket{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-",le="0.1"} 5
rest_api_inbound_request_duration_bucket{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-",le="1"} 5
rest_api_inbound_request_duration_sum{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-"} 0.004553652
rest_api_inbound_request_duration_count{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-"} 5
```
---

## Maestro Agent Metrics

Refer to [Access Maestro Agent Metrics](https://github.com/openshift-online/maestro/edit/main/docs/troubleshooting.md#access-maestro-agent-metrics) for detailed instructions on accessing the metrics in your runtime environment.

---

### `cloudevents_received_total`

**Type:** `counter`\
**Help:** The total number of received CloudEvents.

**Example:**

```
# HELP cloudevents_received_total The total number of received CloudEvents.
# TYPE cloudevents_received_total counter
cloudevents_received_total{action="delete_request",cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 2
cloudevents_received_total{action="resync_response",cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 3
```

---

### `cloudevents_sent_total`

**Type:** `counter`\
**Help:** The total number of sent CloudEvents.

**Example:**

```
# HELP cloudevents_sent_total The total number of sent CloudEvents.
# TYPE cloudevents_sent_total counter
cloudevents_sent_total{action="delete_request",cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",original_source="maestro",source="9894f0c9-557a-4048-9e69-ce676ecb0ba2-work-agent",subresource="status",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 3
cloudevents_sent_total{action="resync_request",cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",original_source="none",source="9894f0c9-557a-4048-9e69-ce676ecb0ba2-work-agent",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 1
cloudevents_sent_total{action="update_request",cluster="9894f0c9-557a-4048-9e69-ce676ecb0ba2",original_source="maestro",source="9894f0c9-557a-4048-9e69-ce676ecb0ba2-work-agent",subresource="status",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 8
```

---

### `manifestworks_processed_total`

**Type:** `counter`\
**Help:** The total number of processed manifestworks.

**Example:**

```
# HELP manifestworks_processed_total The total number of processed manifestworks.
# TYPE manifestworks_processed_total counter
manifestworks_processed_total{action="delete",code="Success"} 3
manifestworks_processed_total{action="list",code="Success"} 1
manifestworks_processed_total{action="patch",code="Success"} 10
manifestworks_processed_total{action="watch",code="Success"} 1
```
