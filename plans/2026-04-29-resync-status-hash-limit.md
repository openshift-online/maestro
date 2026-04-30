# Resync Status Hash Size Limit

**Date:** 2026-04-29

## Problem

The MQTT broker has a `max_packet_size` of 512 KB. When `SourceClientImpl.Resync()` fires (on MQTT
reconnect, consumer reassignment, or instance failover), it delegates to the SDK's
`CloudEventSourceClient.Resync()`, which builds a `ResourceStatusHashList` containing one entry per
resource managed by the consumer. At elevated resource counts this payload exceeds the broker limit,
causing the resync to fail silently.

Measured: 4,029 resources → ~612 KB, which is ~100 KB over the 512 KB limit.

### Root cause of the elevated count

A data leak is causing resources that are no longer real to persist in the database. The resource
count per consumer should be ~400 once the leak is fixed. At 400 resources the full hash list is
~61 KB, well within the broker limit.

## Staged Changes

### 1. `pkg/dao/resource.go` — Fix soft-deleted resource leak

Remove `Unscoped()` from `FindByConsumerName` so that soft-deleted resources are excluded from the
query. Previously, deleted resources were included in the result set, inflating the hash list.

```diff
-g2.Unscoped().Where("consumer_name = ?", consumerName).Find(&resources)
+g2.Where("consumer_name = ?", consumerName).Find(&resources)
```

### 2. `pkg/client/cloudevents/source_client.go` — Hybrid hash list resync

As a safety net for the period before the data leak is fully drained, replace the SDK's
`CloudEventSourceClient.Resync()` call with a custom `resyncConsumer()` that:

- Lists all resources for the consumer from the database.
- Computes real SHA256 status hashes for the first `statusHashBatchSize` (2000) resources.
- Includes remaining resources with an empty `StatusHash` (`""`), which causes the agent to treat
  them as mismatched and re-publish their status unconditionally.
- Sends the CloudEvent directly via the transport, bypassing the SDK's all-or-nothing hash build.

This caps the hash computation cost and keeps the payload within 512 KB even at inflated counts.

Additional changes in the same file:
- Added `cloudevents` and `cepayload` imports.
- Removed the duplicate `types` import alias (consolidated to `cetypes`).
- Added `sourceID` and `transport` fields to `SourceClientImpl` so the custom resync can construct
  and send the CloudEvent without going through the SDK's unexported `publish()` path.

### 3. `pkg/client/cloudevents/source_client_test.go` — New test file

Unit tests for the hybrid resync behaviour:

| Test | What it verifies |
|---|---|
| `TestResyncConsumerHashList` | Under/at/over batch size: correct split of real vs empty hashes, all resource IDs present |
| `TestResyncConsumerEventStructure` | Event type, source, cluster name extension, and that entries beyond the batch have empty hash |
| `TestResyncConsumer5000Resources` | 5000-resource case; logs hash JSON size and full CloudEvent size |

## Known Limitations of the Staged Approach

- The message still contains every resource ID even for empty-hash entries, so payload size still
  scales linearly with resource count. The saving vs full hashes is ~82 bytes per entry beyond the
  batch.
- `resyncConsumer` bypasses the SDK rate limiter in `baseClient`. Acceptable because resync is
  one message per consumer, not a high-frequency operation.
- The hybrid adds complexity (batch constant, DB list call, manual CloudEvent construction) that
  becomes unnecessary once resource counts normalise to ~400.

## Recommended Follow-up

Once the data leak is confirmed resolved and resource counts are stable at ~400:

1. Revert `resyncConsumer` back to `s.CloudEventSourceClient.Resync(ctx, consumer)`.
2. Remove the `sourceID`, `transport` fields and associated imports from `SourceClientImpl`.
3. Remove `statusHashBatchSize` and the hybrid test cases (or simplify to a smoke test).

The `FindByConsumerName` fix in `resource.go` should be kept permanently.
