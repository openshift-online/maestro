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
advisory_lock_count{status="OK",type="events"} 11
advisory_lock_count{status="OK",type="instances"} 30
advisory_lock_count{status="OK",type="resource_status"} 29
advisory_lock_count{status="OK",type="resources"} 7
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
advisory_lock_duration_bucket{status="OK",type="events",le="0.1"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="0.2"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="0.5"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="1"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="2"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="10"} 11
advisory_lock_duration_bucket{status="OK",type="events",le="+Inf"} 11
advisory_lock_duration_sum{status="OK",type="events"} 0.06679135
advisory_lock_duration_count{status="OK",type="events"} 11
```

---

### `advisory_unlock_count`

**Type:** `counter`\
**Help:** Number of advisory unlock requests, categorized by status and type.

This counter tracks how many times an advisory unlock has been requested.

**Example:**

```
# HELP advisory_unlock_count Number of advisory unlock requests.
# TYPE advisory_unlock_count counter
advisory_unlock_count{status="OK",type="events"} 11
advisory_unlock_count{status="OK",type="instances"} 30
advisory_unlock_count{status="OK",type="resource_status"} 29
advisory_unlock_count{status="OK",type="resources"} 7
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

---

### `rest_client_exec_plugin_certificate_rotation_age`

**Type:** `histogram`  
**Help:** [ALPHA] Histogram of the number of seconds the last auth exec plugin client certificate lived before being rotated. If auth exec plugin client certificates are unused, histogram will contain no data.

**Example:**
```
# HELP rest_client_exec_plugin_certificate_rotation_age [ALPHA] Histogram of the number of seconds the last auth exec plugin client certificate lived before being rotated. If auth exec plugin client certificates are unused, histogram will contain no data.
# TYPE rest_client_exec_plugin_certificate_rotation_age histogram
rest_client_exec_plugin_certificate_rotation_age_bucket{le="600"} 0
rest_client_exec_plugin_certificate_rotation_age_sum 0
rest_client_exec_plugin_certificate_rotation_age_count 0
```

---

### `rest_client_exec_plugin_ttl_seconds`

**Type:** `gauge`  
**Help:** [ALPHA] Gauge of the shortest TTL (time-to-live) of the client certificate(s) managed by the auth exec plugin. The value is in seconds until certificate expiry (negative if already expired). If auth exec plugins are unused or manage no TLS certificates, the value will be +INF.

**Example:**
```
# HELP rest_client_exec_plugin_ttl_seconds [ALPHA] Gauge of the shortest TTL (time-to-live) of the client certificate(s) managed by the auth exec plugin. The value is in seconds until certificate expiry (negative if already expired). If auth exec plugins are unused or manage no TLS certificates, the value will be +INF.
# TYPE rest_client_exec_plugin_ttl_seconds gauge
rest_client_exec_plugin_ttl_seconds +Inf
```

---

### `rest_client_rate_limiter_duration_seconds`

**Type:** `histogram`  
**Help:** [ALPHA] Client side rate limiter latency in seconds. Broken down by verb, and host.

**Example:**
```
# HELP rest_client_rate_limiter_duration_seconds [ALPHA] Client side rate limiter latency in seconds. Broken down by verb, and host.
# TYPE rest_client_rate_limiter_duration_seconds histogram
rest_client_rate_limiter_duration_seconds_bucket{host="10.96.0.1:443",verb="DELETE",le="0.005"} 9
rest_client_rate_limiter_duration_seconds_sum{host="10.96.0.1:443",verb="DELETE"} 1.3749000000000002e-05
rest_client_rate_limiter_duration_seconds_count{host="10.96.0.1:443",verb="DELETE"} 9
```

---

### `rest_client_request_duration_seconds`

**Type:** `histogram`  
**Help:** [ALPHA] Request latency in seconds. Broken down by verb, and host.

**Example:**
```
# HELP rest_client_request_duration_seconds [ALPHA] Request latency in seconds. Broken down by verb, and host.
# TYPE rest_client_request_duration_seconds histogram
rest_client_request_duration_seconds_bucket{host="10.96.0.1:443",verb="DELETE",le="0.005"} 4
rest_client_request_duration_seconds_sum{host="10.96.0.1:443",verb="DELETE"} 0.06636385300000001
rest_client_request_duration_seconds_count{host="10.96.0.1:443",verb="DELETE"} 9
```

---

### `rest_client_request_size_bytes`

**Type:** `histogram`  
**Help:** [ALPHA] Request size in bytes. Broken down by verb and host.

**Example:**
```
# HELP rest_client_request_size_bytes [ALPHA] Request size in bytes. Broken down by verb and host.
# TYPE rest_client_request_size_bytes histogram
rest_client_request_size_bytes_bucket{host="10.96.0.1:443",verb="DELETE",le="64"} 0
rest_client_request_size_bytes_sum{host="10.96.0.1:443",verb="DELETE"} 931
rest_client_request_size_bytes_count{host="10.96.0.1:443",verb="DELETE"} 9
```

---

### `rest_client_requests_total`

**Type:** `counter`  
**Help:** [ALPHA] Number of HTTP requests, partitioned by status code, method, and host.

**Example:**
```
# HELP rest_client_requests_total [ALPHA] Number of HTTP requests, partitioned by status code, method, and host.
# TYPE rest_client_requests_total counter
rest_client_requests_total{code="200",host="10.96.0.1:443",method="DELETE"} 9
rest_client_requests_total{code="200",host="10.96.0.1:443",method="GET"} 3912
```

---

### `rest_client_response_size_bytes`

**Type:** `histogram`  
**Help:** [ALPHA] Response size in bytes. Broken down by verb and host.

**Example:**
```
# HELP rest_client_response_size_bytes [ALPHA] Response size in bytes. Broken down by verb and host.
# TYPE rest_client_response_size_bytes histogram
rest_client_response_size_bytes_bucket{host="10.96.0.1:443",verb="DELETE",le="64"} 0
rest_client_response_size_bytes_sum{host="10.96.0.1:443",verb="DELETE"} 8171
rest_client_response_size_bytes_count{host="10.96.0.1:443",verb="DELETE"} 9
```

---

### `rest_client_transport_cache_entries`

**Type:** `gauge`  
**Help:** [ALPHA] Number of transport entries in the internal cache.

**Example:**
```
# HELP rest_client_transport_cache_entries [ALPHA] Number of transport entries in the internal cache.
# TYPE rest_client_transport_cache_entries gauge
rest_client_transport_cache_entries 1
```

---

### `rest_client_transport_create_calls_total`

**Type:** `counter`  
**Help:** [ALPHA] Number of calls to get a new transport, partitioned by the result of the operation hit: obtained from the cache, miss: created and added to the cache, uncacheable: created and not cached

**Example:**
```
# HELP rest_client_transport_create_calls_total [ALPHA] Number of calls to get a new transport, partitioned by the result of the operation hit: obtained from the cache, miss: created and added to the cache, uncacheable: created and not cached
# TYPE rest_client_transport_create_calls_total counter
rest_client_transport_create_calls_total{result="hit"} 7
rest_client_transport_create_calls_total{result="miss"} 1
```

---

### `workqueue_adds_total`

**Type:** `counter`  
**Help:** [ALPHA] Total number of adds handled by workqueue

**Example:**
```
# HELP workqueue_adds_total [ALPHA] Total number of adds handled by workqueue
# TYPE workqueue_adds_total counter
workqueue_adds_total{name="AppliedManifestWorkFinalizer"} 24
workqueue_adds_total{name="AvailableStatusController"} 40
```

---

### `workqueue_depth`

**Type:** `gauge`  
**Help:** [ALPHA] Current depth of workqueue

**Example:**
```
# HELP workqueue_depth [ALPHA] Current depth of workqueue
# TYPE workqueue_depth gauge
workqueue_depth{name="AppliedManifestWorkFinalizer"} 0
workqueue_depth{name="AvailableStatusController"} 0
```

---

### `workqueue_longest_running_processor_seconds`

**Type:** `gauge`  
**Help:** [ALPHA] How many seconds has the longest running processor for workqueue been running.

**Example:**
```
# HELP workqueue_longest_running_processor_seconds [ALPHA] How many seconds has the longest running processor for workqueue been running.
# TYPE workqueue_longest_running_processor_seconds gauge
workqueue_longest_running_processor_seconds{name="AppliedManifestWorkFinalizer"} 0
workqueue_longest_running_processor_seconds{name="AvailableStatusController"} 0
```

---

### `workqueue_queue_duration_seconds`

**Type:** `histogram`  
**Help:** [ALPHA] How long in seconds an item stays in workqueue before being requested.

**Example:**
```
# HELP workqueue_queue_duration_seconds [ALPHA] How long in seconds an item stays in workqueue before being requested.
# TYPE workqueue_queue_duration_seconds histogram
workqueue_queue_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="1e-08"} 0
workqueue_queue_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="0.001"} 22
workqueue_queue_duration_seconds_sum{name="AppliedManifestWorkFinalizer"} 0.19665309500000003
workqueue_queue_duration_seconds_count{name="AppliedManifestWorkFinalizer"} 24
```

---

### `workqueue_retries_total`

**Type:** `counter`  
**Help:** [ALPHA] Total number of retries handled by workqueue

**Example:**
```
# HELP workqueue_retries_total [ALPHA] Total number of retries handled by workqueue
# TYPE workqueue_retries_total counter
workqueue_retries_total{name="AppliedManifestWorkFinalizer"} 4
workqueue_retries_total{name="AvailableStatusController"} 32
```

---

### `workqueue_unfinished_work_seconds`

**Type:** `gauge`  
**Help:** [ALPHA] How many seconds of work has done that is in progress and hasn't been observed by work_duration. Large values indicate stuck threads. One can deduce the number of stuck threads by observing the rate at which this increases.

**Example:**
```
# HELP workqueue_unfinished_work_seconds [ALPHA] How many seconds of work has done that is in progress and hasn't been observed by work_duration. Large values indicate stuck threads. One can deduce the number of stuck threads by observing the rate at which this increases.
# TYPE workqueue_unfinished_work_seconds gauge
workqueue_unfinished_work_seconds{name="AppliedManifestWorkFinalizer"} 0
workqueue_unfinished_work_seconds{name="AvailableStatusController"} 0
```

---

### `workqueue_work_duration_seconds`

**Type:** `histogram`  
**Help:** [ALPHA] How long in seconds processing an item from workqueue takes.

**Example:**
```
# HELP workqueue_work_duration_seconds [ALPHA] How long in seconds processing an item from workqueue takes.
# TYPE workqueue_work_duration_seconds histogram
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="0.001"} 12
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="0.01"} 19
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="0.1"} 24
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="1"} 24
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="10"} 24
workqueue_work_duration_seconds_bucket{name="AppliedManifestWorkFinalizer",le="+Inf"} 24
workqueue_work_duration_seconds_sum{name="AppliedManifestWorkFinalizer"} 0.10679658900000001
workqueue_work_duration_seconds_count{name="AppliedManifestWorkFinalizer"} 24
```