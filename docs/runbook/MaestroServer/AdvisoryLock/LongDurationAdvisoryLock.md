# Long-Duration Advisory Lock

## Symptom

Advisory locks are not being released promptly, indicating possible contention or performance issues.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(advisory_lock_duration_bucket{status="OK",le="0.5"}[5m]))
  /
  sum(rate(advisory_lock_duration_count{status="OK"}[5m]))
) < 0.99
```

This query measures the proportion of advisory locks released within 0.5 seconds over a 5-minute window. A result below 0.99 indicates that more than 1% of locks are taking longer than expected to release.

## Meaning

Less than 99% of advisory locks are released within 0.5 seconds, pointing to delays in advisory lock handling for resource delivery, events processing, or other operations.

## Impact

Slow lock release can lead to increased maestro resource request latency, reduced throughput, or blocked operations under load.

## Diagnosis

Identify which advisory lock types are slow:

```promql
sort_desc(
  sum by (type) (
    rate(advisory_lock_duration_bucket{status="OK",le="+Inf"}[5m])
    - ignoring(le)
      rate(advisory_lock_duration_bucket{status="OK",le="0.5"}[5m])
  )
)
```

This shows the rate of locks taking longer than 0.5s, grouped by `type`. A value of 0 means no delays for that type.

If type="instances" has a high value, try increasing the heartbeat interval (default 15s) in the maestro deployment:

```shell
kubectl -n maestro edit deployment maestro
```

Update the container command:

```yaml
containers:
  - name: service
    command:
      - /usr/local/bin/maestro
      - server
      - --heartbeat-interval=30
```

If the issue persists, check for uncommitted or aborted postgreSQL transactions:

```sql
SELECT state, count(*) 
  FROM pg_stat_activity 
  WHERE usename = 'maestro'
  GROUP BY state;
```

This will output something like:

```psql
            state              | count 
--------------------------------+-------
 active                        |     2
 idle                          |     5
 idle in transaction           |     1
 idle in transaction (aborted) |     1
(4 rows)
```

- `active`: Session is actively running a query.
- `idle`: Session is connected but currently inactive.
- `idle in transaction`: Transaction is open but idle — this may indicate a problem.
- `idle in transaction (aborted)`: Transaction was rolled back or errored but not closed — must be fixed.

Watch for idle in transaction or idle in transaction (aborted) states — they indicate issues that should be resolved.

For high-throughput maestro servers, also check the postgreSQL connection pool size, high contention may indicate an insufficient number of connections.

```shell
kubectl -n maestro get deployment maestro -o jsonpath="{.spec.template.spec.containers[?(@.name=='service')].command}"
```

This will output something like:

```yaml
[--db-max-open-connections=50]
```

If needed, increase --db-max-open-connections, e.g., to 100.
