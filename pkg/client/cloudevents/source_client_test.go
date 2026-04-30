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

func (m *mockTransport) Connect(_ context.Context) error                              { return nil }
func (m *mockTransport) Subscribe(_ context.Context) error                            { return nil }
func (m *mockTransport) Receive(_ context.Context, _ ceoptions.ReceiveHandlerFn) error { return nil }
func (m *mockTransport) Close(_ context.Context) error                                { return nil }
func (m *mockTransport) ErrorChan() <-chan error                                      { return nil }
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

// makeResources builds n resources with empty Status (no hash needed).
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

// decodeHashList decodes the ResourceStatusHashList from the first sent event.
func decodeHashList(t *testing.T, transport *mockTransport) *cepayload.ResourceStatusHashList {
	t.Helper()
	if len(transport.sentEvents) == 0 {
		t.Fatal("expected transport to have received an event, got none")
	}
	evt := transport.sentEvents[0]
	list := &cepayload.ResourceStatusHashList{}
	if err := json.Unmarshal(evt.Data(), list); err != nil {
		t.Fatalf("failed to decode ResourceStatusHashList: %v", err)
	}
	return list
}

// TestResyncConsumerHashList verifies that resyncConsumer produces a hash list
// where the first statusHashBatchSize entries have real (non-empty) hashes and
// any remaining entries have an empty hash, forcing the agent to re-publish
// their status unconditionally.
func TestResyncConsumerHashList(t *testing.T) {
	cases := []struct {
		name            string
		resourceCount   int
		wantRealHashes  int
		wantEmptyHashes int
	}{
		{"fewer than batch size", 10, 10, 0},
		{"exactly batch size", statusHashBatchSize, statusHashBatchSize, 0},
		{"over batch size", statusHashBatchSize + 500, statusHashBatchSize, 500},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			transport := &mockTransport{}
			client := newTestSourceClient(transport, makeResources(c.resourceCount))

			if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
				t.Fatalf("resyncConsumer returned unexpected error: %v", err)
			}

			list := decodeHashList(t, transport)

			if len(list.Hashes) != c.resourceCount {
				t.Errorf("expected %d hash entries, got %d", c.resourceCount, len(list.Hashes))
			}

			realCount, emptyCount := 0, 0
			for _, h := range list.Hashes {
				if h.ResourceID == "" {
					t.Error("found entry with empty ResourceID")
				}
				if h.StatusHash == "" {
					emptyCount++
				} else {
					realCount++
				}
			}

			if realCount != c.wantRealHashes {
				t.Errorf("expected %d entries with real hashes, got %d", c.wantRealHashes, realCount)
			}
			if emptyCount != c.wantEmptyHashes {
				t.Errorf("expected %d entries with empty hashes, got %d", c.wantEmptyHashes, emptyCount)
			}
		})
	}
}

// TestResyncConsumer5000Resources verifies the hash list content and prints the
// payload size when the consumer manages 5000 resources (well above the batch size).
func TestResyncConsumer5000Resources(t *testing.T) {
	const count = 5000
	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(count))

	if err := client.resyncConsumer(context.Background(), "consumer-1"); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	list := decodeHashList(t, transport)

	if len(list.Hashes) != count {
		t.Errorf("expected %d hash entries, got %d", count, len(list.Hashes))
	}

	realCount, emptyCount := 0, 0
	for _, h := range list.Hashes {
		if h.StatusHash == "" {
			emptyCount++
		} else {
			realCount++
		}
	}

	if realCount != statusHashBatchSize {
		t.Errorf("expected %d real hashes, got %d", statusHashBatchSize, realCount)
	}
	if emptyCount != count-statusHashBatchSize {
		t.Errorf("expected %d empty hashes, got %d", count-statusHashBatchSize, emptyCount)
	}

	evt := transport.sentEvents[0]

	hashJSONSize := len(evt.Data())
	t.Logf("hash JSON size:          %d bytes (%.1f KB)", hashJSONSize, float64(hashJSONSize)/1024)

	fullEvent, err := evt.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal full CloudEvent: %v", err)
	}
	fullEventSize := len(fullEvent)
	t.Logf("full CloudEvent size:    %d bytes (%.1f KB)", fullEventSize, float64(fullEventSize)/1024)
	t.Logf("envelope overhead:       %d bytes", fullEventSize-hashJSONSize)
}

// TestResyncConsumerEventStructure verifies that resyncConsumer sends a correctly
// formed CloudEvent — right type, source, and cluster name — regardless of whether
// the hash list contains real or empty hashes.
func TestResyncConsumerEventStructure(t *testing.T) {
	const consumer = "cluster-abc"

	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(statusHashBatchSize+1))

	if err := client.resyncConsumer(context.Background(), consumer); err != nil {
		t.Fatalf("resyncConsumer returned unexpected error: %v", err)
	}

	if len(transport.sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(transport.sentEvents))
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

	// Confirm the last entry (beyond the batch) has an empty hash.
	list := decodeHashList(t, transport)
	last := list.Hashes[len(list.Hashes)-1]
	if last.StatusHash != "" {
		t.Errorf("expected last entry (beyond batch) to have empty StatusHash, got %q", last.StatusHash)
	}
}
