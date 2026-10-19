package server

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

// capturingSourceClient records the handlers registered through Subscribe.
type capturingSourceClient struct {
	handlers []cegeneric.ResourceHandler[*api.Resource]
}

func (c *capturingSourceClient) OnCreate(ctx context.Context, id string) error { return nil }
func (c *capturingSourceClient) OnUpdate(ctx context.Context, id string) error { return nil }
func (c *capturingSourceClient) OnDelete(ctx context.Context, id string) error { return nil }
func (c *capturingSourceClient) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
	c.handlers = append(c.handlers, handlers...)
}
func (c *capturingSourceClient) Resync(ctx context.Context, consumers []string) error { return nil }
func (c *capturingSourceClient) SubscribedChan() <-chan struct{}                      { return nil }
func (c *capturingSourceClient) IsReady() bool                                        { return true }

// owningDispatcher processes every consumer on this instance.
type owningDispatcher struct{}

func (owningDispatcher) Start(ctx context.Context)         {}
func (owningDispatcher) Dispatch(consumerName string) bool { return true }

// stubResourceService serves one resource; unimplemented methods panic if called.
type stubResourceService struct {
	services.ResourceService
	resource *api.Resource
	deleted  []string
}

func (s *stubResourceService) Get(ctx context.Context, id string) (*api.Resource, *errors.ServiceError) {
	return s.resource, nil
}

func (s *stubResourceService) Delete(ctx context.Context, id string) *errors.ServiceError {
	s.deleted = append(s.deleted, id)
	return nil
}

// stubStatusEventService records created status events.
type stubStatusEventService struct {
	services.StatusEventService
	created []*api.StatusEvent
}

func (s *stubStatusEventService) Create(ctx context.Context, event *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError) {
	s.created = append(s.created, event)
	return event, nil
}

// terminalAckFixtures builds a stored resource and its agent delete acknowledgement.
func terminalAckFixtures(t *testing.T) (resource, ack *api.Resource) {
	t.Helper()

	status := cloudevents.NewEvent()
	status.SetID("status-id")
	status.SetSource("agent")
	status.SetType((&cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.UpdateRequestAction,
	}).String())
	if err := status.SetData(cloudevents.ApplicationJSON, &workpayload.ManifestBundleStatus{Conditions: []metav1.Condition{
		{Type: common.ResourceDeleted, Status: metav1.ConditionTrue, Reason: "Deleted", LastTransitionTime: metav1.Now()},
	}}); err != nil {
		t.Fatal(err)
	}
	statusJSON, err := api.CloudEventToJSONMap(&status)
	if err != nil {
		t.Fatal(err)
	}

	spec := cloudevents.NewEvent()
	spec.SetID("spec-id")
	spec.SetSource("maestro")
	spec.SetType((&cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.CreateRequestAction,
	}).String())
	if err := spec.SetData(cloudevents.ApplicationJSON, &workpayload.ManifestBundle{}); err != nil {
		t.Fatal(err)
	}
	specJSON, err := api.CloudEventToJSONMap(&spec)
	if err != nil {
		t.Fatal(err)
	}

	resource = &api.Resource{Meta: api.Meta{ID: "resource-id"}, ConsumerName: "consumer", Version: 1, Source: "maestro", Payload: specJSON}
	ack = &api.Resource{Meta: api.Meta{ID: "resource-id"}, ConsumerName: "consumer", Version: 1, Status: statusJSON}
	return resource, ack
}

// TestMessageQueueSubscriptionTerminalAckWithoutFinalizer drives a terminal delete
// acknowledgement through the real message-queue subscription wiring with the finalizer
// left nil, the state NewMessageQueueEventServer leaves when --consumer-delete-cascade is
// disabled. The acknowledgement must complete on the legacy path instead of panicking on
// the nil finalizer, which is what happened when the callback passed it unconditionally.
func TestMessageQueueSubscriptionTerminalAckWithoutFinalizer(t *testing.T) {
	resource, ack := terminalAckFixtures(t)
	sourceClient := &capturingSourceClient{}
	resourceService := &stubResourceService{resource: resource}
	statusEventService := &stubStatusEventService{}

	s := &MessageQueueEventServer{
		sourceClient:       sourceClient,
		statusDispatcher:   owningDispatcher{},
		resourceService:    resourceService,
		statusEventService: statusEventService,
		// finalizer intentionally left nil: cascade flag off
	}
	s.startSubscription(context.Background())
	if len(sourceClient.handlers) != 1 {
		t.Fatalf("expected one subscribed status handler, got %d", len(sourceClient.handlers))
	}

	if err := sourceClient.handlers[0](context.Background(), ack); err != nil {
		t.Fatalf("terminal acknowledgement through the subscription failed: %v", err)
	}
	if len(statusEventService.created) != 1 || statusEventService.created[0].StatusEventType != api.StatusDeleteEventType {
		t.Fatalf("expected one %s status event, got %+v", api.StatusDeleteEventType, statusEventService.created)
	}
	if len(resourceService.deleted) != 1 || resourceService.deleted[0] != resource.ID {
		t.Fatalf("expected resource %s to be deleted, got %v", resource.ID, resourceService.deleted)
	}
}
