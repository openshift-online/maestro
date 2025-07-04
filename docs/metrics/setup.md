# Set Up the Maestro Observability Stack

This guide shows how to set up Prometheus and Grafana to collect and view Maestro metrics. With this setup, you can better understand how Maestro runs, check its performance, and troubleshoot issues.

## Prerequisites

- Prometheus must be installed using the Prometheus Operator, so that scrape targets can be configured using a `ServiceMonitor`.
- Grafana must be installed on the cluster where the maestro agent is running, and a datasource should be properly configured for the local Prometheus.

## Set Up Maestro Metrics for Prometheus Scraping

### Maestro Server

Create a ServiceMonitor to scrape the metrics of maestro server on the cluster where the maestro server is running.

```shell
oc apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: maestro-server
  namespace: maestro
spec:
  endpoints:
  - interval: 30s
    path: /metrics
    port: metrics
    scheme: http
  namespaceSelector:
    matchNames:
    - maestro
  selector:
    matchLabels:
      app: maestro
      port: metrics
EOF
```

### Maestro Agent

Run the following commands on the cluster where the maestro agent is running.

1. Set the namespace for the Maestro Agent. Update the value based on your deployment.

```shell
export maestro_agent_ns=maestro-agent
```

2. Add additional permissions for the `maestro-agent-sa` ServiceAccount.

```shell
oc apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: maestro-agent-tokenreviews
rules:
- apiGroups: ["authentication.k8s.io"]
  resources: ["tokenreviews"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: maestro-agent-tokenreviews
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: maestro-agent-tokenreviews
subjects:
- kind: ServiceAccount
  name: maestro-agent-sa
  namespace: ${maestro_agent_ns}
EOF
```

3. Add `cluster-monitoring` label to the maestro agent namespace.

```shell
oc label --overwrite ns ${maestro_agent_ns} openshift.io/cluster-monitoring=true
```

4. Create a service to expose the metric endpoint of the maestro agent.

```shell
oc apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: maestro-agent
  namespace: ${maestro_agent_ns}
  labels:
    app: maestro-agent
spec:
  ports:
  - name: https
    port: 8443
    protocol: TCP
    targetPort: 8443
  selector:
    app: maestro-agent
EOF
```

5. Create a ServiceMonitor to scrape the metrics of maestro agent.

```shell
oc apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: maestro-agent
  namespace: ${maestro_agent_ns}
spec:
  endpoints:
  - bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    interval: 60s
    port: https
    scheme: https
    scrapeTimeout: 10s
    tlsConfig:
      insecureSkipVerify: true
  jobLabel: maestro-agent
  namespaceSelector:
    matchNames:
    - ${maestro_agent_ns}
  selector:
    matchLabels:
      app: maestro-agent
EOF
```

### Troubleshooting

[Missing Permission for Prometheus Service Account](https://github.com/stolostron/foundation-docs/blob/main/guide/Metrics/Toubleshooting-MissingPermissionForPrometheus.md)

## Set Up Maestro Dashboards

### Load Maestro Dashboard from JSON File

1. Log in to the Grafana console and navigate to `Home > Dashboard`;
2. Click `New` and choose `Import` from the dropdown menu;
3. In the `Import` dashboard view, paste the contenet of `dashboards/maestro-server.json` into the text area labeled Import via dashboard JSON model.
4. Click Load and then Import to load the dashboard.
5. Follow the same step to load maestro agent dashboard from `dashboards/maestro-agent.json`
