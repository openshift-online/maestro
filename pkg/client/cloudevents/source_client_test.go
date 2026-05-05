package cloudevents

import (
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
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

func decodeHashList(evt cloudevents.Event) *cepayload.ResourceStatusHashList {
	list := &cepayload.ResourceStatusHashList{}
	Expect(json.Unmarshal(evt.Data(), list)).To(Succeed())
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
			RegisterTestingT(t)
			transport := &mockTransport{}
			client := newTestSourceClient(transport, makeResources(c.resourceCount))

			Expect(client.resyncConsumer(context.Background(), "consumer-1")).To(Succeed())
			Expect(transport.sentEvents).To(HaveLen(c.wantEvents))

			totalEntries := 0
			for _, evt := range transport.sentEvents {
				list := decodeHashList(evt)
				Expect(len(list.Hashes)).To(BeNumerically("<=", statusHashBatchSize))
				for _, h := range list.Hashes {
					Expect(h.ResourceID).NotTo(BeEmpty())
					Expect(h.StatusHash).NotTo(BeEmpty(), "all hashes should be real, not blank")
				}
				totalEntries += len(list.Hashes)
			}
			Expect(totalEntries).To(Equal(c.resourceCount))
		})
	}
}

// TestResyncConsumerEmpty verifies that one empty event is sent when the consumer has no resources.
func TestResyncConsumerEmpty(t *testing.T) {
	RegisterTestingT(t)
	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(0))

	Expect(client.resyncConsumer(context.Background(), "consumer-1")).To(Succeed())
	Expect(transport.sentEvents).To(HaveLen(1))

	list := decodeHashList(transport.sentEvents[0])
	Expect(list.Hashes).To(BeEmpty())
}

// TestResyncConsumer5000Resources verifies batch counts and logs payload sizes for a large consumer.
func TestResyncConsumer5000Resources(t *testing.T) {
	RegisterTestingT(t)
	const count = 5000
	wantEvents := (count + statusHashBatchSize - 1) / statusHashBatchSize

	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(count))

	Expect(client.resyncConsumer(context.Background(), "consumer-1")).To(Succeed())
	Expect(transport.sentEvents).To(HaveLen(wantEvents))

	totalEntries := 0
	for i, evt := range transport.sentEvents {
		list := decodeHashList(evt)

		hashJSONSize := len(evt.Data())
		fullEvent, err := evt.MarshalJSON()
		Expect(err).NotTo(HaveOccurred())
		t.Logf("event %d: %d entries, hash JSON: %d bytes (%.1f KB), full event: %d bytes (%.1f KB)",
			i, len(list.Hashes), hashJSONSize, float64(hashJSONSize)/1024,
			len(fullEvent), float64(len(fullEvent))/1024)

		for _, h := range list.Hashes {
			Expect(h.StatusHash).NotTo(BeEmpty())
		}
		totalEntries += len(list.Hashes)
	}

	Expect(totalEntries).To(Equal(count))
}

// TestResyncConsumerEventStructure verifies that every batch CloudEvent has the correct
// source, type, and clusterName extension, and that no entry carries a blank hash.
func TestResyncConsumerEventStructure(t *testing.T) {
	RegisterTestingT(t)
	const consumer = "cluster-abc"
	const resourceCount = statusHashBatchSize + 1 // forces exactly 2 events

	transport := &mockTransport{}
	client := newTestSourceClient(transport, makeResources(resourceCount))

	Expect(client.resyncConsumer(context.Background(), consumer)).To(Succeed())
	Expect(transport.sentEvents).To(HaveLen(2))

	wantType := cetypes.CloudEventsType{
		CloudEventsDataType: NewCodec("test-source").EventDataType(),
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.ResyncRequestAction,
	}

	for _, evt := range transport.sentEvents {
		Expect(evt.Source()).To(Equal("test-source"))
		Expect(evt.Type()).To(Equal(wantType.String()))
		gotCluster, err := evt.Context.GetExtension(cetypes.ExtensionClusterName)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotCluster).To(Equal(consumer))
	}

	// First event: full batch
	list1 := decodeHashList(transport.sentEvents[0])
	Expect(list1.Hashes).To(HaveLen(statusHashBatchSize))

	// Second event: remainder
	list2 := decodeHashList(transport.sentEvents[1])
	Expect(list2.Hashes).To(HaveLen(1))
	Expect(list2.Hashes[0].StatusHash).NotTo(BeEmpty())
}
