# Maestro Agent REST Client High Error Rate

## Symptom

REST client requests issued in maestro agent are returning errors at a higher rate than expected.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(rest_client_requests_total{code=~"2.."}[5m]))
  /
  sum(rate(rest_client_requests_total[5m]))
) < 0.95
```

## Meaning

Fewer than 95% of REST client requests returned a 2xx success status code over the past 5 minutes, indicating a high failure rate.

## Impact

Persistent client-side request failures can lead to reconciliation errors, degraded agent to kubernetes apiserver communication, and inconsistent system state.

## Diagnosis

Identify top error-producing endpoints by `host` and `method`:

```promql
topk(10, sum by (host, method) (
  rate(rest_client_requests_total{code!~"2.."}[5m])
))

```

Example output:

```promql
    host         | method | value
-----------------+--------+---------
 [::1]:6443      | POST   | 0.042
 10.96.0.1:443   | PUT    | 0.035
 10.96.0.1:443   | PATCH  | 0.012
 [::1]:6443      | GET    | 0
 172.18.0.2:6443 | DELETE | 0

```

This shows the top 10 `host` + `method` combinations with the highest non-2xx error rate in the last 5 minutes. Values represent error rate in requests/sec.

Drill down into REST client request errors by status code:

```promql
sum by (code, host, method) (
  rate(rest_client_requests_total{method="POST", host="10.96.0.1:443"}[5m])
)
```

This helps identify specific status codes and the source of errors.

Check maestro agent logs for error patterns:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(HTTP|error|failed)" | head -20
```

Look for recurring HTTP status messages, failure messages, or error codes.

To focus on specific status codes:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "status.*[45][0-9][0-9]"
```

To check for connection-level issues:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(connection.*reset|EOF|timeout)"
```

These logs can help identify root causes such as misconfigurations, downstream service failures, or network problems.
