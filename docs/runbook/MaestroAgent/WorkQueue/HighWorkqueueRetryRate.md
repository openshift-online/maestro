# Maestro Agent High Workqueue Retry Rate

## Symptom

Workqueue items are being retried more often than expected, indicating persistent errors during processing.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(workqueue_retries_total[5m]))
  /
  sum(rate(workqueue_adds_total[5m]))
) > 0.01
```

## Meaning

More than 1% of workqueue items are being retried, suggesting that work controllers are repeatedly failing to process items successfully.

## Impact

High retry rates increase controller load, can slow down reconciliation, and indicate underlying controller or system issues (e.g., conflicts, errors, or rate limiting).

## Diagnosis

Identify top retrying workqueues

Check the workqueues with the highest retry rates:

```promql
topk(10, sum by (name) (rate(workqueue_retries_total[5m])))
```

Example output:

```promql
Element | name                         | Value
--------|------------------------------|--------
1       | AvailableStatusController    | 0.106
2       | AppliedManifestWorkFinalizer | 0.013
```

This shows the per-second retry rate over the last 5 minutes for each workqueue. High values point to problematic controllers.

Analyze reconciliation behavior

Controller errors often cause retries when reconciliation fails and items are re-queued with rate limiting.

Check logs for reconciliation errors:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(Reconciler error|reconcile.*error)" | tail -20
```

Investigate underlying causes in logs

Check for resource conflicts or API issues:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(conflict|timeout|connection.*reset)" | tail -20
```

Look for common retry-triggering patterns:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(failed to.*update|failed to.*create|failed to.*delete)" | tail -20
```

Check for API server throttling:

```shell
kubectl -n maestro logs deploy/maestro-agent | grep -E "(429|too.*many.*requests|throttl)" | tail -10
```

Understanding these error patterns helps determine whether issues are controller-specific, API-related, or due to apiserver throttling.
