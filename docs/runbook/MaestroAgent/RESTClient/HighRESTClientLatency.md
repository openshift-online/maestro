# Maestro Agent REST Client High Latency

## Symptom

REST client requests issued in maestro agent are taking longer than expected to complete.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(rest_client_request_duration_seconds_bucket{le="0.5"}[5m]))
  /
  sum(rate(rest_client_request_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of REST client requests complete within 500ms over the past 5 minutes, indicating latency when communicating with Kubernetes APIs.

## Impact

High client-side request latency may slow down reconciliation loops, event reporting, or interactions with the Kubernetes API server, leading to degraded performance and responsiveness.

## Diagnosis

Identify the slow hosts and HTTP verbs used in delayed REST client requests:

```promql
sort_desc(
  sum by (host, verb) (
    rate(rest_client_request_duration_seconds_bucket{le="+Inf"}[5m])
    - ignoring(le)
      rate(rest_client_request_duration_seconds_bucket{le="0.5"}[5m])
  )
)
```

Example output:

```promql
  host         | verb | value
---------------+------+--------
 10.96.0.1:443 | POST | 0.025
 [::1]:6443    | PUT  | 0.018
 10.96.0.1:443 | GET  | 0.012
 [::1]:6443    | GET  | 0.000
```

This shows the rate of requests slower than 0.5s, grouped by `host` and `verb`, sorted by frequency. A value of `0` means no slow requests for that combination.

Enable detailed HTTP tracing for maestro agent by editing the maestro agent deployment:

```shell
# Note: This will cause the Maestro agent to restart, which may result in lost previous logs and make some issues unreproducible.
kubectl -n maestro edit deploy/maestro-agent
```

In the container spec for maestro-agent, update the command to include log verbosity:

```yaml
containers:
  - name: maestro-agent
    image: maestro-image
    command:
      - /usr/local/bin/maestro
      - agent
      - --v=6
```

Verbosity levels:

- `--v=4`: Basic request error logging
- `--v=6`: URL timing information
- `--v=9`: Full HTTP trace with DNS lookup, connection setup, TLS, and server timings

Save and exit, kubernetes will automatically restart the pod with updated configuration.

Then use logs to analyze which part of the request lifecycle contributes to the delay.
