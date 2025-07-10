# Maestro High REST API Latency

## Symptom

REST API requests in maestro are taking longer than expected to complete.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(rest_api_inbound_request_duration_bucket{le="1.0"}[5m]))
  /
  sum(rate(rest_api_inbound_request_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of REST API requests complete within 1 second over the past 5 minutes, suggesting latency in REST API handling.

## Impact

Slow API responses can lead to REST client timeouts, degraded user experience, and delayed interactions between components or external systems.

## Diagnosis

Identify slow REST API paths:

```promql
sort_desc(
  sum by (path) (
    rate(rest_api_inbound_request_duration_bucket{le="+Inf"}[5m])
    - ignoring(le)
      rate(rest_api_inbound_request_duration_bucket{le="1.0"}[5m])
  )
)
```

This shows the rate of requests taking longer than 1 second, grouped by path, sorted by frequency. A value of 0 means no long-running requests for that path.

Example output:

```promql
{path="/api/maestro/v1/resource-bundles/-"} | 0.015
{path="/api/maestro/v1/resource-bundles"}   | 0
```

If request to path `/resource-bundles/-` is slow, it's often related to the number and size of resources.

Check resource count and size in postgreSQL:

```psql
SELECT 
  count(*) AS total_resources,
  sum(pg_column_size(payload)) AS total_payload_bytes,
  pg_size_pretty(sum(pg_column_size(payload))) AS total_payload_pretty
FROM resources;
```

This helps assess if the data volume is contributing to latency.

When requesting a resource bundle list, especially for large quantities or sizes of resources, consider setting a smaller return size than the default of 100 records.

```shell
# for instance, request resource bundles with 10 resources each time
curl ${MAESTRO_HOST}/api/maestro/v1/resource-bundles?size=10
```
