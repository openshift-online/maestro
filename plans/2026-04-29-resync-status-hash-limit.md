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

## Approach

Rather than a hybrid that sends partial real hashes and risks a still-large payload, send an
**empty** `ResourceStatusHashList` (zero entries). The agent's `respondResyncStatusRequest`
explicitly handles this case:

```go
// agentclient.go
if len(statusHashes.Hashes) == 0 {
    // publish all resources status
    for _, obj := range objs {
        if err := c.Publish(ctx, eventType, obj); err != nil {
            return err
        }
    }
    return nil
}
```

An empty list signals the agent to re-publish status for every resource it manages
unconditionally. The outbound message is ~20 bytes regardless of resource count.

**Tradeoff:** on every resync the agent sends back status for all its resources rather than only
changed ones. At ~400 resources (the expected steady-state count once the leak is fixed) this is
acceptable. Resync is triggered on MQTT reconnect, consumer reassignment, and instance failover —
not on a hot path.

## Staged Changes

### 1. `pkg/dao/resource.go` — Fix soft-deleted resource leak

Remove `Unscoped()` from `FindByConsumerName` so that soft-deleted resources are excluded from the
query. Previously, deleted resources were included in the result set, inflating the hash list.

```diff
-g2.Unscoped().Where("consumer_name = ?", consumerName).Find(&resources)
+g2.Where("consumer_name = ?", consumerName).Find(&resources)
```

### 2. `pkg/client/cloudevents/source_client.go` — Empty hash list resync

Replace the SDK's `CloudEventSourceClient.Resync()` call with a custom `resyncConsumer()` that
sends an empty `ResourceStatusHashList` directly via the transport.

Added `sourceID` and `transport` fields to `SourceClientImpl` to construct and send the CloudEvent
without going through the SDK's unexported `publish()` path. Removed the duplicate `types` import
alias (consolidated to `cetypes`).

```go
type SourceClientImpl struct {
	Codec                  cegeneric.Codec[*api.Resource]
	CloudEventSourceClient *ceclients.CloudEventSourceClient[*api.Resource]
	ResourceService        services.ResourceService
	sourceID               string
	transport              ceoptions.CloudEventTransport
}

func NewSourceClient(sourceOptions *ceoptions.CloudEventsSourceOptions, resourceService services.ResourceService) (SourceClient, error) {
	ctx := context.Background()
	codec := NewCodec(sourceOptions.SourceID)
	ceSourceClient, err := ceclients.NewCloudEventSourceClient[*api.Resource](ctx, sourceOptions,
		resourceService, ResourceStatusHashGetter, codec)
	if err != nil {
		return nil, err
	}

	cemetrics.RegisterSourceCloudEventsMetrics(prometheus.DefaultRegisterer)

	return &SourceClientImpl{
		Codec:                  codec,
		CloudEventSourceClient: ceSourceClient,
		ResourceService:        resourceService,
		sourceID:               sourceOptions.SourceID,
		transport:              sourceOptions.CloudEventsTransport,
	}, nil
}

func (s *SourceClientImpl) Resync(ctx context.Context, consumers []string) error {
	logger := klog.FromContext(ctx).WithValues("consumers", consumers)
	ctx = klog.NewContext(ctx, logger)

	logger.Info("Resyncing resource status from consumers")
	for _, consumer := range consumers {
		if err := s.resyncConsumer(ctx, consumer); err != nil {
			return err
		}
	}

	return nil
}

// resyncConsumer sends a status resync request with an empty hash list to the consumer.
// An empty list signals the agent to re-publish status for all its resources unconditionally,
// keeping the outbound message small regardless of resource count.
func (s *SourceClientImpl) resyncConsumer(ctx context.Context, consumer string) error {
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: s.Codec.EventDataType(),
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.ResyncRequestAction,
	}

	evt := cetypes.NewEventBuilder(s.sourceID, eventType).WithClusterName(consumer).NewEvent()
	if err := evt.SetData(cloudevents.ApplicationJSON, &cepayload.ResourceStatusHashList{}); err != nil {
		return fmt.Errorf("failed to set resync event data: %v", err)
	}

	return s.transport.Send(ctx, evt)
}
```

### 3. `pkg/client/cloudevents/source_client_test.go` — New test file

```go
// TestResyncConsumerSendsEmptyHashList verifies that resyncConsumer sends a CloudEvent
// with an empty ResourceStatusHashList, which signals the agent to re-publish status
// for all its resources unconditionally.
func TestResyncConsumerSendsEmptyHashList(t *testing.T) { ... }

// TestAgentDecodesEmptyHashListAsFullResync simulates the agent receiving the CloudEvent
// produced by resyncConsumer and verifies that:
//  1. The event decodes without error using the same payload decoder the agent uses.
//  2. The decoded hash list is empty, which is the condition in the agent's
//     respondResyncStatusRequest that causes it to publish status for ALL its resources.
func TestAgentDecodesEmptyHashListAsFullResync(t *testing.T) { ... }

// TestResyncConsumerEventStructure verifies the CloudEvent has the correct type,
// source, and cluster name extension.
func TestResyncConsumerEventStructure(t *testing.T) { ... }
```

| Test | What it verifies |
|---|---|
| `TestResyncConsumerSendsEmptyHashList` | Payload decodes to a hash list with zero entries |
| `TestAgentDecodesEmptyHashListAsFullResync` | Agent's own decoder (`DecodeStatusResyncRequest`) parses the event and returns an empty list, confirming the full-resync branch is taken |
| `TestResyncConsumerEventStructure` | Event type (`SubResourceStatus` + `ResyncRequestAction`), source, and cluster name extension are correct |

## Known Limitations

- `resyncConsumer` bypasses the SDK rate limiter in `baseClient`. Acceptable because resync fires
  once per consumer per reconnect event, not in a tight loop.
- On every resync the agent sends back status for all its resources, not just changed ones. At the
  expected steady-state of ~400 resources this is negligible.

## Recommended Follow-up

Once the data leak is confirmed resolved and resource counts are stable at ~400, evaluate whether
to restore the SDK's `CloudEventSourceClient.Resync()` (full delta hashes, ~61 KB) for better
efficiency. The `FindByConsumerName` fix in `resource.go` should be kept permanently.
