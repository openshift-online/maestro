# Maestro Slow Resource Spec Resync (Alpha)

## Symptom

Resource spec resyncs in maestro are taking longer than expected to complete.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(resources_spec_resync_duration_seconds_bucket{le="10.0"}[5m]))
  /
  sum(rate(resources_spec_resync_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of resource spec resyncs complete within 10 seconds over the past 5 minutes, indicating possible delays in resource spec sync operations.

## Impact

Slow resource spec resync can cause outdated or inconsistent resource state between maestro server and agents, potentially leading to stale configurations, delayed reconciliation, or system drift.

## Diagnosis

Check total number and size of resources stored in postgreSQL:

```psql
SELECT 
  count(*) AS total_resources,
  sum(pg_column_size(payload)) AS total_payload_bytes,
  pg_size_pretty(sum(pg_column_size(payload))) AS total_payload_pretty
FROM resources;
```

This helps understand if a large number or size of resources is contributing to the delay.

Review MQTT broker (Event Grid) metrics/logs to verify the functionality of current Maestro-MQTT connections.
For instance, in Azure, navigate to "EventGrid | Namespaces" > "Monitoring" -> "Metrics". If possible, examine the "Failed Publish Events" and "MQTT: Dropped Sessions" metrics and drill down into the logs for further error information if needed.

Check postgreSQL connection state for issues with uncommitted or aborted transactions:

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

Persistent idle in transaction or aborted states can delay backend queries and impact resource spec resync performance.

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

Updating the deployment with more connections can reduce contention and improve resource spec resync responsiveness.
