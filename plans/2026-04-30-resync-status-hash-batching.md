# Resync Status Hash Batching

**Date:** 2026-04-30

## Problem

The staged fix from 2026-04-29 caps hash computation at 2000 resources and sends blank hashes for
the rest. A blank hash always mismatches, so resources beyond the 2000 boundary are unconditionally
re-published every resync cycle — wasteful and unnecessary.

The root cause (data leak inflating resource counts) is being addressed separately. This change
removes the blank-hash shortcut and replaces it with proper batching so that all resources receive
real hashes regardless of count, while each individual CloudEvent stays under the MQTT 512 KB limit.

## OCM SDK Agent Behavior (confirmed)

Before implementing, the OCM SDK agent-side handler was read directly from:
`vendor/open-cluster-management.io/sdk-go/pkg/cloudevents/generic/clients/agentclient.go`
`respondResyncStatusRequest()` (lines 217–274).

Key behavior when a non-empty hash list is received:

```go
for _, obj := range objs {
    lastHash, ok := findStatusHash(string(obj.GetUID()), statusHashes.Hashes)
    if !ok {
        // ignore the resource that is not on the source, but exists on the agent,
        // wait for the source deleting it
        logger.Info("The resource is not found from the source, ignore", "uid", obj.GetUID())
        continue  // ← skipped, no delete, no publish, no side effect
    }
    // compare hash; publish status only if changed
}
```

The only case that triggers publish-all is `len(statusHashes.Hashes) == 0` (empty list). A
non-empty partial list is safe: resources absent from the batch are skipped with a log line and
nothing else. **No deletions are triggered by a missing resource ID.**

This confirms that sending sequential partial-list batches is correct: each batch reconciles only
its resources; resources in later batches are untouched until their batch arrives.

## Implementation

### `pkg/client/cloudevents/source_client.go`

Batch size reduced from 2000 to 1000 to give more headroom under the 512 KB MQTT limit
(each 1000-resource batch measures ~131 KB).

Soft-deleted resources are **included** in the hash list — omitting them would cause the agent to
log them as "not found from source" on every resync cycle unnecessarily.

```go
// statusHashBatchSize is the maximum number of resources per resync CloudEvent.
// Keeping batches at this size ensures each MQTT packet stays within the 512 KB broker limit.
const statusHashBatchSize = 1000

// resyncConsumer sends status resync requests to the consumer in batches of statusHashBatchSize.
// Each batch is a separate CloudEvent containing real status hashes for its slice of resources.
// The OCM SDK agent ignores resources absent from a batch's hash list, so sequential partial-list
// events are safe: each batch reconciles only its resources, and together they cover all resources.
func (s *SourceClientImpl) resyncConsumer(ctx context.Context, consumer string) error {
    objs, err := s.ResourceService.List(ctx, cetypes.ListOptions{
        Source:              s.sourceID,
        ClusterName:         consumer,
        CloudEventsDataType: s.Codec.EventDataType(),
    })
    if err != nil {
        return fmt.Errorf("failed to list resources for consumer %s: %v", consumer, err)
    }

    hashes := make([]cepayload.ResourceStatusHash, 0, len(objs))
    for _, obj := range objs {
        statusHash, err := ResourceStatusHashGetter(obj)
        if err != nil {
            return err
        }
        hashes = append(hashes, cepayload.ResourceStatusHash{
            ResourceID: string(obj.GetUID()),
            StatusHash: statusHash,
        })
    }

    totalBatches := (len(hashes) + statusHashBatchSize - 1) / statusHashBatchSize
    for i := 0; i < len(hashes); i += statusHashBatchSize {
        end := i + statusHashBatchSize
        if end > len(hashes) {
            end = len(hashes)
        }
        batchNum := i/statusHashBatchSize + 1
        if err := s.sendResyncBatch(ctx, consumer, hashes[i:end], batchNum, totalBatches); err != nil {
            return err
        }
    }
    return nil
}

func (s *SourceClientImpl) sendResyncBatch(ctx context.Context, consumer string,
    hashes []cepayload.ResourceStatusHash, batchNum, totalBatches int) error {

    eventType := cetypes.CloudEventsType{
        CloudEventsDataType: s.Codec.EventDataType(),
        SubResource:         cetypes.SubResourceStatus,
        Action:              cetypes.ResyncRequestAction,
    }
    evt := cetypes.NewEventBuilder(s.sourceID, eventType).WithClusterName(consumer).NewEvent()
    if err := evt.SetData(cloudevents.ApplicationJSON, &cepayload.ResourceStatusHashList{Hashes: hashes}); err != nil {
        return fmt.Errorf("failed to set resync event data: %v", err)
    }
    evtBytes, err := evt.MarshalJSON()
    if err != nil {
        return fmt.Errorf("failed to marshal resync event: %v", err)
    }
    klog.FromContext(ctx).V(2).Info("Sending status resync batch",
        "consumer", consumer,
        "batch", fmt.Sprintf("%d/%d", batchNum, totalBatches),
        "resources", len(hashes),
        "bytes", len(evtBytes))
    return s.transport.Send(ctx, evt)
}
```

### `pkg/client/cloudevents/source_client_test.go`

| Test | What it verifies |
|---|---|
| `TestResyncConsumerHashList` | Under/at/over batch size: correct event count, all hashes real, no entry exceeds batch limit |
| `TestResyncConsumerEmpty` | 0 resources → 0 events sent |
| `TestResyncConsumer5000Resources` | 5 events for 5000 resources, all real hashes, logs per-batch sizes |
| `TestResyncConsumerEventStructure` | Every batch event has correct source, type, clusterName; no blank hashes |

## What Does Not Change

- `ResourceStatusHashGetter` — unchanged.
- `sourceID` and `transport` fields on `SourceClientImpl` — still required.
- `resyncOnReconnect` / `processNextResync` in `pkg/dispatcher/hash_dispatcher.go` — unchanged.

## Verification

```bash
make test                    # unit tests
make test-integration-mqtt   # MQTT integration (resync path exercised on reconnect)
```

Manually confirm with a consumer holding >1000 resources that resync triggers N CloudEvents (at
V≥2) and that only truly changed statuses are re-published.
