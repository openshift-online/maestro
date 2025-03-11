# Troubleshooting

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
kubectl -n ${maestro_agent_ns} port-forward deploy/maestro-agent 8443
curl -k -H "Authorization: Bearer $TOKEN" https://localhost:8443/metrics
```
