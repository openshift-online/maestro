package cloudevents

import (
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	ceoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options"
	cepayload "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// mockTransport captures sent events for assertion.
type mockTransport struct {
	sentEvents []cloudevents.Event
}

func (m *mockTransport) Connect(_ context.Context) error                               { return nil }
func (m *mockTransport) Subscribe(_ context.Context) error                             { return nil }
func (m *mockTransport) Receive(_ context.Context, _ ceoptions.ReceiveHandlerFn) error { return nil }
func (m *mockTransport) Close(_ context.Context) error                                 { return nil }
func (m *mockTransport) ErrorChan() <-chan error                                       { return nil }
func (m *mockTransport) Send(_ context.Context, evt cloudevents.Event) error {
	m.sentEvents = append(m.sentEvents, evt)
	return nil
}

func newTestSourceClient(transport *mockTransport) *SourceClientImpl {
	return &SourceClientImpl{
		Codec:     NewCodec("test-source"),
		sourceID:  "test-source",
		transport: transport,
	}
}

// TestResyncConsumerSendsEmptyHashList verifies that resyncConsumer sends a CloudEvent
// with an empty ResourceStatusHashList, which signals the agent to re-publish status
// for all its resources unconditionally.
func TestResyncConsumerSendsEmptyHashList(t *testing.T) {
	transport := &mockTransport{}
	client := newTestSourceClient(transport)

	if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	if len(transport.sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(transport.sentEvents))
	}

	list := &cepayload.ResourceStatusHashList{}
	if err := json.Unmarshal(transport.sentEvents[0].Data(), list); err != nil {
		t.Fatalf("failed to decode ResourceStatusHashList: %v", err)
	}

	if len(list.Hashes) != 0 {
		t.Errorf("expected empty hash list, got %d entries", len(list.Hashes))
	}
}

// TestAgentDecodesEmptyHashListAsFullResync simulates the agent receiving the CloudEvent
// produced by resyncConsumer and verifies that:
//  1. The event decodes without error using the same payload decoder the agent uses.
//  2. The decoded hash list is empty, which is the condition in the agent's
//     respondResyncStatusRequest that causes it to publish status for ALL its resources
//     (agentclient.go: if len(statusHashes.Hashes) == 0 { publish all }).
func TestAgentDecodesEmptyHashListAsFullResync(t *testing.T) {
	transport := &mockTransport{}
	client := newTestSourceClient(transport)

	if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	evt := transport.sentEvents[0]

	hashes, err := cepayload.DecodeStatusResyncRequest(evt)
	if err != nil {
		t.Fatalf("agent failed to decode resync event: %v", err)
	}

	// An empty hash list is the signal that causes the agent to publish status
	// for every resource it manages, without performing any hash comparison.
	if len(hashes.Hashes) != 0 {
		t.Errorf("expected empty hash list to trigger full agent resync, got %d entries", len(hashes.Hashes))
	}
}

// TestResyncConsumerEventStructure verifies the CloudEvent has the correct type,
// source, and cluster name extension.
func TestResyncConsumerEventStructure(t *testing.T) {
	const consumer = "cluster-abc"

	transport := &mockTransport{}
	client := newTestSourceClient(transport)

	if err := client.resyncConsumer(context.Background(), consumer); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	evt := transport.sentEvents[0]

	if evt.Source() != "test-source" {
		t.Errorf("expected source 'test-source', got %q", evt.Source())
	}

	wantType := cetypes.CloudEventsType{
		CloudEventsDataType: NewCodec("test-source").EventDataType(),
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.ResyncRequestAction,
	}
	if evt.Type() != wantType.String() {
		t.Errorf("expected event type %q, got %q", wantType.String(), evt.Type())
	}

	gotCluster, err := evt.Context.GetExtension(cetypes.ExtensionClusterName)
	if err != nil {
		t.Fatalf("missing clusterName extension: %v", err)
	}
	if gotCluster != consumer {
		t.Errorf("expected clusterName %q, got %q", consumer, gotCluster)
	}
}
