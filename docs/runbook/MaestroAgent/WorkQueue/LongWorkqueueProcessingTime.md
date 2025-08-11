# Maestro Agent Long Workqueue Processing Time

## Symptom

Workqueue items are taking too long to be processed after dequeuing.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(workqueue_work_duration_seconds_bucket{le="1.0"}[5m]))
  /
  sum(rate(workqueue_work_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of workqueue items are processed within 1 second over the past 5 minutes, suggesting that processing logic or external dependencies may be slow.

## Impact

Delayed processing can result in backlogs, increased queue depth, retries, or missed event deadlines in high-throughput scenarios.

## Diagnosis

Identify slow workqueues

Check workqueues with high 99th percentile processing time:

```promql
histogram_quantile(0.99, rate(workqueue_work_duration_seconds_bucket[5m])) > 1
```

Break down by workqueue name:

```promql
histogram_quantile(0.99, rate(workqueue_work_duration_seconds_bucket[5m])) by (name) > 1
```

Check current average processing times:

```promql
sum by (name) (rate(workqueue_work_duration_seconds_sum[5m]))
/
sum by (name) (rate(workqueue_work_duration_seconds_count[5m]))
```

Check for stuck or long-running processors

Monitor current unfinished work per queue:

```promql
workqueue_unfinished_work_seconds by (name)
```

Check longest running processors:

```promql
workqueue_longest_running_processor_seconds by (name)
```

Alerting thresholds:

```promql
workqueue_unfinished_work_seconds > 10
workqueue_longest_running_processor_seconds > 30
```

Monitor increasing unfinished work (indicates potential processing backlog):

```promql
rate(workqueue_unfinished_work_seconds[5m]) by (name)
```

This helps identify whether threads are stuck or processing is consistently slow.
