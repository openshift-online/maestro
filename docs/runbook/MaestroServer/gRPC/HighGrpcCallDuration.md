# Maestro High gRPC Server Call Duration

## Symptom

gRPC server calls in maestro are taking longer than expected to complete.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(grpc_server_processed_duration_seconds_bucket{le="0.5"}[5m]))
  /
  sum(rate(grpc_server_processed_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of gRPC requests are completing within 500ms over the past 5 minutes, indicating elevated call durations.

## Impact

Slow gRPC processing may lead to delayed processing, timeouts, or degraded performance in maestro operations, affecting user experience and system throughput.

## Diagnosis

Check current postgreSQL connection states for the maestro server:

```psql
SELECT state, count(*) 
  FROM pg_stat_activity 
  WHERE usename = 'maestro'
  GROUP BY state;
```

Example output:

```psql
            state              | count 
--------------------------------+-------
 active                        |     2
 idle                          |     5
 idle in transaction           |     1
 idle in transaction (aborted) |     1
```

- `active`: Session is actively running a query.
- `idle`: Session is connected but currently inactive.
- `idle in transaction`: Transaction is open but idle — this may indicate a problem.
- `idle in transaction (aborted)`: Transaction was rolled back or errored but not closed — must be fixed.

Persistent idle in transaction or aborted states can delay backend queries and impact gRPC performance.

For high-throughput maestro deployments, also check postgreSQL connection pool settings:

```shell
kubectl -n maestro get deployment maestro -o jsonpath="{.spec.template.spec.containers[?(@.name=='service')].command}"
```

Example output:

```yaml
[--db-max-open-connections=50]
```

If connections are maxing out, try increasing the limit, e.g.,

```yaml
--db-max-open-connections=100
```

Updating the deployment with more connections can reduce contention and improve gRPC responsiveness.
