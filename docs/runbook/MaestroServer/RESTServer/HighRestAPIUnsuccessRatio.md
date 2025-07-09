# Maestro High REST API Request Unsuccess Ratio

## Symptom

An increase in 5xx status codes indicates that some REST API requests to the Maestro server are failing.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(rest_api_inbound_request_count{code=~"5.."}[5m]))
  /
  sum(rate(rest_api_inbound_request_count[5m]))
) > 0.01
```

## Meaning

More than 1% of REST API requests returned a 5xx status code in the past 5 minutes, signaling server-side errors.

## Impact

High failure rates can degrade client experience, break automation relying on the API, and signal internal failures or overload conditions in maestro.

## Diagnosis

Verify maestro server pod status:

```shell
kubectl -n maestro get pods -l app=maestro
```

Ensure all pods are running and not restarting due to crashes or panics.

If the maestro server is operating without crashes or panics, review its logs for outstanding errors that indicate improper functioning.

```shell
kubectl -n maestro logs deploy/maestro | grep -E "(Failed|failed|Error|error)
```

This will show any errors in the logs for maestro server that may indicate issues with the REST API handling.
