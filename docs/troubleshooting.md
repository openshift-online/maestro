# Troubleshooting

## Check Health of Maestro Components

To ensure that all Maestro components are functioning correctly, you can check the health of the Maestro server and agent.

### Maestro Server Health Check

Check the Maestro server’s health by port-forwarding its service and querying the health endpoint with the commands below:

```shell
kubectl -n maestro port-forward svc/maestro-healthcheck 8083 &
curl -k https://localhost:8083/healthcheck
```

You can also check the Maestro server's health by verifying the pod status:

```shell
kubectl -n maestro get pods -l app=maestro
```

If the Maestro server pod shows Running, it's healthy. If not, check the logs for more details:

```shell
kubectl -n maestro logs deploy/maestro
```

A healthy pod usually has logs showing it's ready to serve requests, such as:

```shell
# kubectl -n maestro logs deploy/maestro | grep Starting
I0710 08:08:09.280520       1 rotation.go:63] Starting client certificate rotation controller
2025-07-10T08:08:09.290Z	INFO	controllers/framework.go:85	Starting event controller
2025-07-10T08:08:09.290Z	INFO	server/healthcheck_server.go:50	Starting HealthCheck server
2025-07-10T08:08:09.290Z	INFO	event/event.go:77	Starting event broadcaster
2025-07-10T08:08:09.290Z	INFO	server/event_server.go:78	Starting message queue event server
2025-07-10T08:08:09.290Z	INFO	server/grpc_server.go:140	Starting gRPC server
2025-07-10T08:08:09.290Z	INFO	controllers/status_controller.go:45	Starting status event controller
2025-07-10T08:08:09.297Z	INFO	db_session/default.go:198	Starting listener for events
2025-07-10T08:08:09.299Z	INFO	db_session/default.go:198	Starting listener for status_events
```

A healthy Maestro server pod should also show a ready subscriber, indicating it's receiving agent events. Look for a log message like this:

```shell
# kubectl -n maestro logs deploy/maestro | grep subscribing
{"level":"info","ts":1752134889.29144,"logger":"fallback","caller":"v2/protocol.go:133","msg":"subscribing to topics: [{sources/maestro/consumers/+/agentevents 1 0 false false}]"}
```

An unhealthy Maestro server pod may show error or failure logs like:

```shell
# kubectl -n maestro logs deploy/maestro | grep -E "(error|failed)"
[error] failed to initialize database, got error failed to connect to ...
```

If resources are not applied to the Maestro agent or status isn’t returned, the Maestro server might be unhealthy or unable to connect to the MQTT broker. Check the server logs for errors related to the MQTT broker connection, such as:

```shell
# kubectl -n maestro logs deploy/maestro | grep "failed to publish"
2025-07-10T08:08:14.300Z  ERROR   source_client.go    Failed to publish resource ...
```

### Maestro Agent Health Check

To check the Maestro agent’s health, verify the status of its pod:

```shell
kubectl -n maestro get pods -l app=maestro-agent
```

If the Maestro agent pod status is `Running`, it’s healthy. If not, check its logs for details:

```shell
kubectl -n maestro logs deploy/maestro-agent
```

A healthy Maestro agent pod shows logs indicating it’s ready to serve requests, like:

```shell
# kubectl -n maestro logs deploy/maestro-agent | grep -E "(Starting|Caches are synced)"
I0710 08:08:50.930655       1 observer_polling.go:159] Starting file observer
I0710 08:08:51.751145       1 requestheader_controller.go:180] Starting RequestHeaderAuthRequestController
I0710 08:08:51.751207       1 configmap_cafile_content.go:205] "Starting controller" name="client-ca::kube-system::extension-apiserver-authentication::client-ca-file"
I0710 08:08:51.751229       1 configmap_cafile_content.go:205] "Starting controller" name="client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file"
I0710 08:08:51.751588       1 tlsconfig.go:243] "Starting DynamicServingCertificateController"
I0710 08:08:51.764923       1 rotation.go:63] Starting client certificate rotation controller
I0710 08:08:51.771541       1 reflector.go:313] Starting reflector *v1.AppliedManifestWork
I0710 08:08:51.771549       1 reflector.go:313] Starting reflector *v1.ManifestWork
I0710 08:08:51.851508       1 shared_informer.go:320] Caches are synced for client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file
I0710 08:08:51.851645       1 shared_informer.go:320] Caches are synced for RequestHeaderAuthRequestController
I0710 08:08:51.851725       1 shared_informer.go:320] Caches are synced for client-ca::kube-system::extension-apiserver-authentication::client-ca-file
I0710 08:08:51.871447       1 base_controller.go:82] Caches are synced for ManifestWorkAgent
```

A healthy Maestro agent pod also shows a ready subscriber, indicating it’s ready to receive events. Look for a log message like this:

```shell
# kubectl -n maestro logs deploy/maestro-agent | grep "subscribing"
{"level":"info","ts":1752134931.7709265,"logger":"fallback","caller":"v2/protocol.go:133","msg":"subscribing to topics: [{sources/maestro/consumers/c4ce562a-2c27-4038-b570-dd002b1fdeb6/sourceevents 1 0 false false}]"}
```

An unhealthy Maestro agent pod may show errors or failures in the logs, such as:

```shell
# kubectl -n maestro logs deploy/maestro-agent | grep -E "(error|failed)"
F0710 08:43:44.627577       1 cmd.go:182] failed to connect to MQTT broker...
```

## Update Maestro Log Level at Runtime

To aid in troubleshooting, you may need detailed logs from Maestro. Currently, the supported log levels are debug, info, warn, and error, with info as the default. For the complete list of available log levels, refer to [zap log levels](https://github.com/uber-go/zap/blob/master/level.go#L30-L49).

To adjust the log level, create or update the configmap named in `maestro-logging-config ` in maestro namespace. This change will dynamically modify the log level for Maestro without requiring a restart.

```yaml
cat << EOF | kubectl -n maestro apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: maestro-logging-config
data:
  config.yaml: |
    log_level: debug
EOF
```
## Access to Maestro Metrics

### Access maestro server metrics

Access maestro server metrics via maestro-metrics service:

```shell
kubectl -n maestro port-forward svc/maestro-metrics 8080 &
curl http://localhost:8080/metrics
```

### Access maestro agent metrics

1. Apply RBAC resources to access maestro agent metrics

```shell
export maestro_agent_ns=<maestro-agent-namespace>
cat << EOF | kubectl apply -n ${maestro_agent_ns} -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: metrics-reader
rules:
  - nonResourceURLs:
      - "/metrics"
    verbs:
      - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-reader-binding
subjects:
  - kind: ServiceAccount
    name: maestro-agent-sa
    namespace: ${maestro_agent_ns}
roleRef:
  kind: ClusterRole
  name: metrics-reader
  apiGroup: rbac.authorization.k8s.io
EOF
```

2. Get the token to access maestro agent metrics

```shell
export TOKEN=$(kubectl -n ${maestro_agent_ns} create token maestro-agent-sa)
```

3. Access maestro agent metrics via maestro-agent pod:

```shell
kubectl -n ${maestro_agent_ns} port-forward deploy/maestro-agent 8443 &
curl -k -H "Authorization: Bearer $TOKEN" https://localhost:8443/metrics
```