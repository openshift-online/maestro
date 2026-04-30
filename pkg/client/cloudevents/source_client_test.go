package cloudevents

import (
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	ceoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options"
	cepayload "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
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

// mockResourceService implements only the List method needed by resyncConsumer.
type mockResourceService struct {
	resources []*api.Resource
	services.ResourceService
}

func (m *mockResourceService) List(_ context.Context, _ cetypes.ListOptions) ([]*api.Resource, error) {
	return m.resources, nil
}

// Stub out the remaining interface methods to satisfy the compiler.
func (m *mockResourceService) Get(_ context.Context, _ string) (*api.Resource, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) Create(_ context.Context, _ *api.Resource) (*api.Resource, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) Update(_ context.Context, _ *api.Resource) (*api.Resource, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) UpdateStatus(_ context.Context, _ *api.Resource) (*api.Resource, bool, *errors.ServiceError) {
	return nil, false, nil
}
func (m *mockResourceService) MarkAsDeleting(_ context.Context, _ string) *errors.ServiceError {
	return nil
}
func (m *mockResourceService) Delete(_ context.Context, _ string) *errors.ServiceError { return nil }
func (m *mockResourceService) All(_ context.Context) (api.ResourceList, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) FindByIDs(_ context.Context, _ []string) (api.ResourceList, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) FindBySource(_ context.Context, _ string) (api.ResourceList, *errors.ServiceError) {
	return nil, nil
}
func (m *mockResourceService) ListWithArgs(_ context.Context, _ string, _ *services.ListArguments, _ *[]api.Resource) (*api.PagingMeta, *errors.ServiceError) {
	return nil, nil
}

// makeResources builds n resources with empty Status (hash of empty string is used).
func makeResources(n int) []*api.Resource {
	resources := make([]*api.Resource, n)
	for i := range resources {
		resources[i] = &api.Resource{
			Meta: api.Meta{ID: uuid.New().String()},
		}
	}
	return resources
}

func newTestSourceClient(transport *mockTransport, resources []*api.Resource) *SourceClientImpl {
	codec := NewCodec("test-source")
	return &SourceClientImpl{
		Codec:           codec,
		ResourceService: &mockResourceService{resources: resources},
		sourceID:        "test-source",
		transport:       transport,
	}
}

func decodeHashList(t *testing.T, evt cloudevents.Event) *cepayload.ResourceStatusHashList {
	t.Helper()
	list := &cepayload.ResourceStatusHashList{}
	if err := json.Unmarshal(evt.Data(), list); err != nil {
		t.Fatalf("failed to decode ResourceStatusHashList: %v", err)
	}
	return list
}

// TestResyncConsumerHashList verifies that resyncConsumer sends the correct number of batches,
// each within the size limit, and that all hashes are real (no blank placeholders).
func TestResyncConsumerHashList(t *testing.T) {
	cases := []struct {
		name          string
		resourceCount int
		wantEvents    int
	}{
		{"fewer than batch size", 10, 1},
		{"exactly batch size", statusHashBatchSize, 1},
		{"over batch size", statusHashBatchSize + 100, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			transport := &mockTransport{}
			client := newTestSourceClient(transport, makeResources(c.resourceCount))

			if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
				t.Fatalf("resyncConsumer returned unexpected error: %v", err)
			}

			if len(transport.sentEvents) != c.wantEvents {
				t.Fatalf("expected %d events, got %d", c.wantEvents, len(transport.sentEvents))
			}

			totalEntries := 0
			for i, evt := range transport.sentEvents {
				list := decodeHashList(t, evt)
				if len(list.Hashes) > statusHashBatchSize {
					t.Errorf("event %d: %d entries exceeds batch limit of %d", i, len(list.Hashes), statusHashBatchSize)
				}
				for _, h := range list.Hashes {
					if h.ResourceID == "" {
						t.Errorf("event %d: found entry with empty ResourceID", i)
					}
					if h.StatusHash == "" {
						t.Errorf("event %d: resource %s has empty hash — all hashes should be real", i, h.ResourceID)
					}
				}
				totalEntries += len(list.Hashes)
			}

			if totalEntries != c.resourceCount {
				t.Errorf("expected %d total entries across all events, got %d", c.resourceCount, totalEntries)
			}
		})
	}
}

// TestResyncConsumerEmpty verifies that no events are sent when the consumer has no resources.
func TestResyncConsumerEmpty(t *testing.T) {
	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(0))

	if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	if len(transport.sentEvents) != 0 {
		t.Errorf("expected 0 events for empty resource list, got %d", len(transport.sentEvents))
	}
}

// TestResyncConsumer5000Resources verifies batch counts and logs payload sizes for a large consumer.
func TestResyncConsumer5000Resources(t *testing.T) {
	const count = 5000
	wantEvents := (count + statusHashBatchSize - 1) / statusHashBatchSize // ceiling division

	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(count))

	if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	if len(transport.sentEvents) != wantEvents {
		t.Fatalf("expected %d events for %d resources, got %d", wantEvents, count, len(transport.sentEvents))
	}

	totalEntries := 0
	for i, evt := range transport.sentEvents {
		list := decodeHashList(t, evt)

		hashJSONSize := len(evt.Data())
		fullEvent, err := evt.MarshalJSON()
		if err != nil {
			t.Fatalf("event %d: failed to marshal CloudEvent: %v", i, err)
		}
		t.Logf("event %d: %d entries, hash JSON: %d bytes (%.1f KB), full event: %d bytes (%.1f KB)",
			i, len(list.Hashes), hashJSONSize, float64(hashJSONSize)/1024,
			len(fullEvent), float64(len(fullEvent))/1024)

		for _, h := range list.Hashes {
			if h.StatusHash == "" {
				t.Errorf("event %d: resource %s has empty hash", i, h.ResourceID)
			}
		}
		totalEntries += len(list.Hashes)
	}

	if totalEntries != count {
		t.Errorf("expected %d total entries, got %d", count, totalEntries)
	}
}

// TestResyncConsumerEventStructure verifies that every batch CloudEvent has the correct
// source, type, and clusterName extension, and that no entry carries a blank hash.
func TestResyncConsumerEventStructure(t *testing.T) {
	const consumer = "cluster-abc"
	const resourceCount = statusHashBatchSize + 1 // forces exactly 2 events

	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(resourceCount))

	if err := client.resyncConsumer(context.Background(), consumer); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	if len(transport.sentEvents) != 2 {
		t.Fatalf("expected 2 sent events, got %d", len(transport.sentEvents))
	}

	wantType := cetypes.CloudEventsType{
		CloudEventsDataType: NewCodec("test-source").EventDataType(),
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.ResyncRequestAction,
	}

	for i, evt := range transport.sentEvents {
		if evt.Source() != "test-source" {
			t.Errorf("event %d: expected source 'test-source', got %q", i, evt.Source())
		}
		if evt.Type() != wantType.String() {
			t.Errorf("event %d: expected type %q, got %q", i, wantType.String(), evt.Type())
		}
		gotCluster, err := evt.Context.GetExtension(cetypes.ExtensionClusterName)
		if err != nil {
			t.Fatalf("event %d: missing clusterName extension: %v", i, err)
		}
		if gotCluster != consumer {
			t.Errorf("event %d: expected clusterName %q, got %q", i, consumer, gotCluster)
		}
	}

	// First event: full batch
	list1 := decodeHashList(t, transport.sentEvents[0])
	if len(list1.Hashes) != statusHashBatchSize {
		t.Errorf("first event: expected %d entries, got %d", statusHashBatchSize, len(list1.Hashes))
	}

	// Second event: remainder
	list2 := decodeHashList(t, transport.sentEvents[1])
	if len(list2.Hashes) != 1 {
		t.Errorf("second event: expected 1 entry, got %d", len(list2.Hashes))
	}
	if list2.Hashes[0].StatusHash == "" {
		t.Error("second event: entry should have a real hash, got empty")
	}
}
