# Maestro Agent High Workqueue Queue Duration

## Symptom

Items in the workqueue are waiting too long before being processed.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(workqueue_queue_duration_seconds_bucket{le="1.0"}[5m]))
  /
  sum(rate(workqueue_queue_duration_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of items are dequeued within 1 second, suggesting a delay between manifestwork enqueueing and processing.

## Impact

Queue latency can cause delayed reconciliation, slow work event handling, and eventual system drift — especially in large-scale environments.

## Diagnosis

Identify workqueues with high queue latency

Check the 99th percentile queue duration:

```promql
histogram_quantile(0.99, rate(workqueue_queue_duration_seconds_bucket[5m])) > 1
```

Break it down by workqueue name:

```promql
histogram_quantile(0.99, rate(workqueue_queue_duration_seconds_bucket[5m])) by (name) > 1
```

Check the average queue duration:

```promql
sum by (name) (rate(workqueue_queue_duration_seconds_sum[5m]))
/
sum by (name) (rate(workqueue_queue_duration_seconds_count[5m]))
```

Monitor queue depth and backlog

Current queue depth (items waiting):

```promql
workqueue_depth by (name)
```

Check if items are being added faster than processed:

```promql
rate(workqueue_adds_total[5m]) by (name)
-
rate(workqueue_queue_duration_seconds_count[5m]) by (name)
```

Retry rate (can indicate processing failures):

```promql
rate(workqueue_retries_total[5m]) by (name)
```

Analyze processing capacity vs demand

Compare total adds vs completions:

```promql
workqueue_adds_total | workqueue_queue_duration_seconds_count
```

Check for workqueue processing bottlenecks

Compare queue duration vs work duration:

```promql
histogram_quantile(0.99, rate(workqueue_queue_duration_seconds_bucket[5m])) by (name)
/
histogram_quantile(0.99, rate(workqueue_work_duration_seconds_bucket[5m])) by (name)
```

High ratio means items wait longer than they take to process — a potential bottleneck.

Check if work processing itself is slow:

```promql
histogram_quantile(0.99, rate(workqueue_work_duration_seconds_bucket[5m])) by (name)
```

This helps pinpoint whether the slowness is in queuing build-up or actual work processing.
