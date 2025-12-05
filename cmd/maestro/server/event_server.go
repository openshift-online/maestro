package server

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/dispatcher"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// EventServer handles resource-related events:
// 1. Resource spec events (create, update and delete) from the resource controller.
// 2. Resource status update events from the agent.
type EventServer interface {
	// Start initiates the EventServer.
	Start(ctx context.Context)

	// OnCreate handles the creation of a resource.
	OnCreate(ctx context.Context, resourceID string) error

	// OnUpdate handles updates to a resource.
	OnUpdate(ctx context.Context, resourceID string) error

	// OnDelete handles the deletion of a resource.
	OnDelete(ctx context.Context, resourceID string) error

	// OnStatusUpdate handles status update events for a resource.
	OnStatusUpdate(ctx context.Context, eventID, resourceID string) error

	// returns true if the event should be processed by the current instance, otherwise false and an error if it occurs.
	PredicateEvent(ctx context.Context, eventID string) (bool, error)
}

var _ EventServer = &MessageQueueEventServer{}

// MessageQueueEventServer represents a event server responsible for publish resource spec events
// from resource controller and handle resource status update events from the message queue.
// It also maintains a status dispatcher to dispatch status update events to the corresponding
// maestro instances.
type MessageQueueEventServer struct {
	instanceID         string
	eventInstanceDao   dao.EventInstanceDao
	lockFactory        db.LockFactory
	eventBroadcaster   *event.EventBroadcaster // event broadcaster to broadcast resource status update events to subscribers
	resourceService    services.ResourceService
	statusEventService services.StatusEventService
	sourceClient       cloudevents.SourceClient
	statusDispatcher   dispatcher.Dispatcher
}

func NewMessageQueueEventServer(eventBroadcaster *event.EventBroadcaster, statusDispatcher dispatcher.Dispatcher) EventServer {
	sessionFactory := env().Database.SessionFactory
	return &MessageQueueEventServer{
		instanceID:         env().Config.MessageBroker.ClientID,
		eventInstanceDao:   dao.NewEventInstanceDao(&sessionFactory),
		lockFactory:        db.NewAdvisoryLockFactory(sessionFactory),
		eventBroadcaster:   eventBroadcaster,
		resourceService:    env().Services.Resources(),
		statusEventService: env().Services.StatusEvents(),
		sourceClient:       env().Clients.CloudEventsSource,
		statusDispatcher:   statusDispatcher,
	}
}

// Start initializes and runs the event server. It starts the subscription
// to resource status update messages and the status dispatcher.
func (s *MessageQueueEventServer) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.Info("Starting message queue event server")

	// start subscribing to resource status update messages.
	s.startSubscription(ctx)
	// start the status dispatcher
	go s.statusDispatcher.Start(ctx)

	// wait until context is canceled
	<-ctx.Done()
	logger.Info("Shutting down message queue event server")
}

// startSubscription initiates the subscription to resource status update messages.
// It runs asynchronously in the background until the provided context is canceled.
func (s *MessageQueueEventServer) startSubscription(ctx context.Context) {
	s.sourceClient.Subscribe(ctx, func(action types.ResourceAction, resource *api.Resource) error {
		logger := klog.FromContext(ctx).WithValues("resourceID", resource.ID, "action", action)
		logger.Info("received action for resource")
		ctx = klog.NewContext(ctx, logger)

		switch action {
		case types.StatusModified:
			if !s.statusDispatcher.Dispatch(resource.ConsumerName) {
				// the resource is not owned by the current instance, skip
				logger.Info("skipping resource status update as it is not owned by the current instance")
				return nil
			}

			// handle the resource status update according status update type
			if err := handleStatusUpdate(ctx, resource, s.resourceService, s.statusEventService); err != nil {
				return fmt.Errorf("failed to handle resource status update %s: %s", resource.ID, err.Error())
			}
		default:
			return fmt.Errorf("failed to handle resource status update %s: unsupported action %s", resource.ID, action)
		}

		return nil
	})
}

// OnCreate will be called on each new resource creation event inserted into db.
func (s *MessageQueueEventServer) OnCreate(ctx context.Context, resourceID string) error {
	return s.sourceClient.OnCreate(ctx, resourceID)
}

// OnUpdate will be called on each new resource update event inserted into db.
func (s *MessageQueueEventServer) OnUpdate(ctx context.Context, resourceID string) error {
	return s.sourceClient.OnUpdate(ctx, resourceID)
}

// OnDelete will be called on each new resource deletion event inserted into db.
func (s *MessageQueueEventServer) OnDelete(ctx context.Context, resourceID string) error {
	return s.sourceClient.OnDelete(ctx, resourceID)
}

// On StatusUpdate will be called on each new status event inserted into db.
// It does two things:
// 1. build the resource status and broadcast it to subscribers
// 2. add the event instance record to mark the event has been processed by the current instance
func (s *MessageQueueEventServer) OnStatusUpdate(ctx context.Context, eventID, resourceID string) error {
	return broadcastStatusEvent(
		ctx,
		s.statusEventService,
		s.resourceService,
		s.eventInstanceDao,
		s.eventBroadcaster,
		s.instanceID,
		eventID,
		resourceID,
	)
}

// EventPredicate for the message queue event server is no-op, as the message queue server filter event based on advisory lock.
func (s *MessageQueueEventServer) PredicateEvent(ctx context.Context, eventID string) (bool, error) {
	return true, nil
}

// handleStatusUpdate processes the resource status update from the agent.
// The resource argument contains the updated status.
// The function performs the following steps:
// 1. Verifies if the resource is still in the Maestro server and checks if the consumer name matches.
// 2. Retrieves the resource from Maestro and fills back the work metadata from the spec event to the status event.
// 3. Checks if the resource has been deleted from the agent. If so, creates a status event and deletes the resource from Maestro;
// otherwise, updates the resource status and creates a status event.
func handleStatusUpdate(ctx context.Context, resource *api.Resource, resourceService services.ResourceService, statusEventService services.StatusEventService) error {
	logger := klog.FromContext(ctx)
	logger.Info("handle resource status update by the current instance")

	found, svcErr := resourceService.Get(ctx, resource.ID)
	if svcErr != nil {
		if svcErr.Is404() {
			logger.Info("skipping resource as it is not found")
			return nil
		}

		return fmt.Errorf("failed to get resource %s, %s", resource.ID, svcErr.Error())
	}

	if found.ConsumerName != resource.ConsumerName {
		return fmt.Errorf("unmatched consumer name %s for resource %s", resource.ConsumerName, resource.ID)
	}

	// set the resource source and type back for broadcast
	resource.Source = found.Source

	// convert the resource status to cloudevent
	statusEvent, err := api.JSONMAPToCloudEvent(resource.Status)
	if err != nil {
		return fmt.Errorf("failed to convert resource status to cloudevent: %v", err)
	}

	// convert the resource spec to cloudevent
	specEvent, err := api.JSONMAPToCloudEvent(found.Payload)
	if err != nil {
		return fmt.Errorf("failed to convert resource spec to cloudevent: %v", err)
	}

	// set work meta from spec event to status event
	if workMeta, ok := specEvent.Extensions()[types.ExtensionWorkMeta]; ok {
		statusEvent.SetExtension(types.ExtensionWorkMeta, workMeta)
	}

	// convert the resource status cloudevent back to resource status jsonmap
	resource.Status, err = api.CloudEventToJSONMap(statusEvent)
	if err != nil {
		return fmt.Errorf("failed to convert resource status cloudevent to json: %v", err)
	}

	// decode the cloudevent data as manifest status
	statusPayload := &workpayload.ManifestBundleStatus{}
	if err := statusEvent.DataAs(statusPayload); err != nil {
		return fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
	}

	// if the resource has been deleted from agent, create status event and delete it from maestro
	if meta.IsStatusConditionTrue(statusPayload.Conditions, common.ResourceDeleted) {
		_, sErr := statusEventService.Create(ctx, &api.StatusEvent{
			ResourceID:      resource.ID,
			ResourceSource:  resource.Source,
			ResourceType:    resource.Type,
			Payload:         found.Payload,
			Status:          resource.Status,
			StatusEventType: api.StatusDeleteEventType,
		})
		if sErr != nil {
			return fmt.Errorf("failed to create status event for resource status delete %s: %s", resource.ID, sErr.Error())
		}
		if svcErr := resourceService.Delete(ctx, resource.ID); svcErr != nil {
			return fmt.Errorf("failed to delete resource %s: %s", resource.ID, svcErr.Error())
		}

		logger.Info("resource status delete event was sent")
	} else {
		// update the resource status
		_, updated, svcErr := resourceService.UpdateStatus(ctx, resource)
		if svcErr != nil {
			return fmt.Errorf("failed to update resource status %s: %s", resource.ID, svcErr.Error())
		}

		// create the status event only when the resource is updated
		if updated {
			_, sErr := statusEventService.Create(ctx, &api.StatusEvent{
				ResourceID:      resource.ID,
				StatusEventType: api.StatusUpdateEventType,
			})
			if sErr != nil {
				return fmt.Errorf("failed to create status event for resource status update %s: %s", resource.ID, sErr.Error())
			}

			logger.Info("resource status update event was sent")
		}
	}

	return nil
}

func broadcastStatusEvent(ctx context.Context,
	statusEventService services.StatusEventService,
	resourceService services.ResourceService,
	eventInstanceDao dao.EventInstanceDao,
	eventBroadcaster *event.EventBroadcaster,
	instanceID, eventID, resourceID string) error {
	statusEvent, sErr := statusEventService.Get(ctx, eventID)
	if sErr != nil {
		return fmt.Errorf("failed to get status event %s: %s", eventID, sErr.Error())
	}

	logger := klog.FromContext(ctx).WithValues("resourceID", resourceID, "instanceID", instanceID, "eventID", eventID)

	var resource *api.Resource
	// check if the status event is delete event
	if statusEvent.StatusEventType == api.StatusDeleteEventType {
		// build resource with resource id and delete status
		resource = &api.Resource{
			Meta: api.Meta{
				ID: resourceID,
			},
			Source:  statusEvent.ResourceSource,
			Type:    statusEvent.ResourceType,
			Payload: statusEvent.Payload,
			Status:  statusEvent.Status,
		}
	} else {
		resource, sErr = resourceService.Get(ctx, resourceID)
		if sErr != nil {
			if sErr.Is404() {
				logger.Info("skipping resource as it is not found")
				return nil
			}

			return fmt.Errorf("failed to get resource %s: %s", resourceID, sErr.Error())
		}
	}

	// broadcast the resource status to subscribers
	logger.Info("Broadcast the resource status",
		"source", resource.Source, "statusEventType", statusEvent.StatusEventType)
	eventBroadcaster.Broadcast(resource)

	// add the event instance record
	_, err := eventInstanceDao.Create(ctx, &api.EventInstance{
		EventID:    eventID,
		InstanceID: instanceID,
	})

	return err
}
