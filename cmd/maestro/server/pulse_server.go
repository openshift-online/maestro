package server

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/dispatcher"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

var log = logger.NewOCMLogger(context.Background())

// PulseServer represents a server responsible for periodic heartbeat updates and
// checking the liveness of Maestro instances, triggering status resync based on
// instances' status and other conditions.
type PulseServer struct {
	instanceID       string
	pulseInterval    int64
	instanceDao      dao.InstanceDao
	eventInstanceDao dao.EventInstanceDao
	lockFactory      db.LockFactory
	eventBroadcaster *event.EventBroadcaster
	resourceService  services.ResourceService
	eventService     services.EventService
	sourceClient     cloudevents.SourceClient
	statusDispatcher dispatcher.Dispatcher
}

func NewPulseServer(eventBroadcaster *event.EventBroadcaster) *PulseServer {
	var statusDispatcher dispatcher.Dispatcher
	switch config.SubscriptionType(env().Config.PulseServer.SubscriptionType) {
	case config.SharedSubscriptionType:
		statusDispatcher = dispatcher.NewNoopDispatcher(dao.NewConsumerDao(&env().Database.SessionFactory), env().Clients.CloudEventsSource)
	case config.BroadcastSubscriptionType:
		statusDispatcher = dispatcher.NewHashDispatcher(env().Config.MessageBroker.ClientID, dao.NewInstanceDao(&env().Database.SessionFactory), dao.NewConsumerDao(&env().Database.SessionFactory), env().Clients.CloudEventsSource)
	default:
		glog.Fatalf("Unsupported subscription type: %s", env().Config.PulseServer.SubscriptionType)
	}
	sessionFactory := env().Database.SessionFactory
	return &PulseServer{
		instanceID:       env().Config.MessageBroker.ClientID,
		pulseInterval:    env().Config.PulseServer.PulseInterval,
		instanceDao:      dao.NewInstanceDao(&sessionFactory),
		eventInstanceDao: dao.NewEventInstanceDao(&sessionFactory),
		lockFactory:      db.NewAdvisoryLockFactory(sessionFactory),
		eventBroadcaster: eventBroadcaster,
		resourceService:  env().Services.Resources(),
		eventService:     env().Services.Events(),
		sourceClient:     env().Clients.CloudEventsSource,
		statusDispatcher: statusDispatcher,
	}
}

// Start initializes and runs the pulse server, updating and checking Maestro instances' liveness,
// initializes subscription to status update messages and triggers status resync based on
// instances' status and other conditions.
func (s *PulseServer) Start(ctx context.Context) {
	log.Infof("Starting pulse server")

	// start subscribing to resource status update messages.
	s.startSubscription(ctx)
	// start the status dispatcher
	go s.statusDispatcher.Start(ctx)

	// start a goroutine to periodically update heartbeat for the current maestro instance
	go wait.UntilWithContext(ctx, s.pulse, time.Duration(s.pulseInterval*int64(time.Second)))

	// start a goroutine to periodically check the liveness of maestro instances
	go wait.UntilWithContext(ctx, s.checkInstances, time.Duration(s.pulseInterval/3*int64(time.Second)))

	// wait until context is canceled
	<-ctx.Done()
	log.Infof("Shutting down pulse server")
}

func (s *PulseServer) pulse(ctx context.Context) {
	log.V(10).Infof("Updating heartbeat for maestro instance: %s", s.instanceID)
	instance := &api.ServerInstance{
		Meta: api.Meta{
			ID:        s.instanceID,
			UpdatedAt: time.Now(),
		},
	}
	_, err := s.instanceDao.UpSert(ctx, instance)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to upsert maestro instance: %s", err.Error()))
	}
}

func (s *PulseServer) checkInstances(ctx context.Context) {
	log.V(10).Infof("Checking liveness of maestro instances")
	// lock the Instance with a fail-fast advisory lock context.
	// this allows concurrent processing of many instances by one or more maestro instances exclusively.
	lockOwnerID, acquired, err := s.lockFactory.NewNonBlockingLock(ctx, "maestro-instances-pulse-check", db.Instances)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		log.Error(fmt.Sprintf("error obtaining the instance lock: %v", err))
		return
	}
	// skip if the lock is not acquired
	if !acquired {
		log.Error("failed to acquire the lock as another maestro instance is checking instances")
		return
	}

	instances, err := s.instanceDao.All(ctx)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to get all maestro instances: %s", err.Error()))
		return
	}

	inactiveInstanceIDs := []string{}
	for _, instance := range instances {
		// Instances pulsing within the last three check intervals are considered as active.
		if instance.UpdatedAt.After(time.Now().Add(time.Duration(int64(-3*time.Second) * s.pulseInterval))) {
			if err := s.statusDispatcher.OnInstanceUp(instance.ID); err != nil {
				log.Error(fmt.Sprintf("Error to call OnInstanceUp handler for maestro instance %s: %s", instance.ID, err.Error()))
			}
		} else {
			if err := s.statusDispatcher.OnInstanceDown(instance.ID); err != nil {
				log.Error(fmt.Sprintf("Error to call OnInstanceDown handler for maestro instance %s: %s", instance.ID, err.Error()))
			} else {
				inactiveInstanceIDs = append(inactiveInstanceIDs, instance.ID)
			}
		}
	}

	if len(inactiveInstanceIDs) > 0 {
		// batch delete inactive instances
		if err := s.instanceDao.DeleteByIDs(ctx, inactiveInstanceIDs); err != nil {
			log.Error(fmt.Sprintf("Unable to delete inactive maestro instances (%s): %s", inactiveInstanceIDs, err.Error()))
		}
	}
}

// startSubscription initiates the subscription to resource status update messages.
// It runs asynchronously in the background until the provided context is canceled.
func (s *PulseServer) startSubscription(ctx context.Context) {
	s.sourceClient.Subscribe(ctx, func(action types.ResourceAction, resource *api.Resource) error {
		log.V(4).Infof("received action %s for resource %s", action, resource.ID)

		switch action {
		case types.StatusModified:
			found, svcErr := s.resourceService.Get(ctx, resource.ID)
			if svcErr != nil {
				if svcErr.Is404() {
					log.Warning(fmt.Sprintf("skipping resource %s as it is not found", resource.ID))
					return nil
				}

				return fmt.Errorf("failed to get resource %s, %s", resource.ID, svcErr.Error())
			}

			if found.ConsumerName != resource.ConsumerName {
				return fmt.Errorf("unmatched consumer name %s for resource %s", resource.ConsumerName, resource.ID)
			}

			// set the resource source back for broadcast
			resource.Source = found.Source

			if !s.statusDispatcher.Dispatch(resource.ConsumerName) {
				// the resource is not owned by the current instance, skip
				log.V(4).Infof("skipping resource status update %s as it is not owned by the current instance", resource.ID)
				return nil
			}

			// // convert the resource status to cloudevent
			// evt, err := api.JSONMAPToCloudEvent(resource.Status)
			// if err != nil {
			// 	return fmt.Errorf("failed to convert resource status to cloudevent: %v", err)
			// }

			// // decode the cloudevent data as manifest status
			// statusPayload := &workpayload.ManifestStatus{}
			// if err := evt.DataAs(statusPayload); err != nil {
			// 	return fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
			// }

			// // if the resource has been deleted from agent, delete it from maestro
			// if meta.IsStatusConditionTrue(statusPayload.Conditions, common.ManifestsDeleted) {
			// 	if svcErr := s.resourceService.Delete(ctx, resource.ID); svcErr != nil {
			// 		return svcErr
			// 	}

			// 	log.V(4).Infof("Broadcast:: the resource %s is deleted", resource.ID)
			// 	resource.Payload = found.Payload
			// 	s.eventBroadcaster.Broadcast(resource)
			// 	return nil
			// }
			// update the resource status
			_, updated, svcErr := s.resourceService.UpdateStatus(ctx, resource)
			if svcErr != nil {
				return fmt.Errorf("failed to update resource status %s: %s", resource.ID, svcErr.Error())
			}

			// // broadcast the resource status updated only when the resource is updated
			// if updated {
			// 	log.V(4).Infof("Broadcast:: the resource %s is updated", resource.ID)
			// 	s.eventBroadcaster.Broadcast(updatedResource)
			// }
			if updated {
				evt, sErr := s.eventService.Create(ctx, &api.Event{
					Source:    "Resources",
					SourceID:  resource.ID,
					EventType: api.StatusUpdateEventType,
				})
				if sErr != nil {
					return fmt.Errorf("failed to create event for resource status update %s: %s", resource.ID, sErr.Error())
				}
				instances, err := s.instanceDao.All(ctx)
				if err != nil {
					return fmt.Errorf("failed to get all maestro instances: %s", err)
				}
				eventInstanceList := make([]*api.EventInstance, len(instances))
				for i, instance := range instances {
					eventInstanceList[i] = &api.EventInstance{
						EventID:    evt.ID,
						InstanceID: instance.ID,
					}
				}
				if err := s.eventInstanceDao.CreateList(ctx, eventInstanceList); err != nil {
					return fmt.Errorf("failed to create event instances for resource status update %s: %s", resource.ID, err.Error())
				}
			}
		default:
			return fmt.Errorf("unsupported action %s", action)
		}

		return nil
	})
}

// On StatusUpdate does three things:
// 1. Broadcast the resource status update to subscribers
// 2. Mark the event instance as done
// 3. If the resource has been deleted from agent, delete it from maestro if all event instances are done
func (s *PulseServer) OnStatusUpdate(ctx context.Context, eventID, resourceID string) error {
	resource, sErr := s.resourceService.Get(ctx, resourceID)
	if sErr != nil {
		return fmt.Errorf("failed to get resource %s: %s", resourceID, sErr.Error())
	}

	// broadcast the resource status updated to subscribers
	s.eventBroadcaster.Broadcast(resource)

	// mark the event instance as done
	if err := s.eventInstanceDao.MarkAsDone(ctx, eventID, s.instanceID); err != nil {
		return fmt.Errorf("failed to mark event instance (%s, %s) as done for resource status update %s: %s", eventID, s.instanceID, resourceID, err.Error())
	}

	// convert the resource status to cloudevent
	cloudevt, err := api.JSONMAPToCloudEvent(resource.Status)
	if err != nil {
		return fmt.Errorf("failed to convert resource status to cloudevent: %v", err)
	}

	// decode the cloudevent data as manifest status
	statusPayload := &workpayload.ManifestStatus{}
	if err := cloudevt.DataAs(statusPayload); err != nil {
		return fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
	}

	// if the resource has been deleted from agent, delete it from maestro
	deleted := false
	if meta.IsStatusConditionTrue(statusPayload.Conditions, common.ManifestsDeleted) {
		deleted = true
	}
	if deleted {
		count, err := s.eventInstanceDao.GetUnhandleEventInstances(ctx, eventID)
		if err != nil {
			return fmt.Errorf("failed to get unhandled event instances for event %s: %s", eventID, err.Error())
		}
		if count == 0 {
			if sErr := s.resourceService.Delete(ctx, resourceID); sErr != nil {
				return fmt.Errorf("failed to delete resource %s: %s", resourceID, sErr.Error())
			}
		}
	}

	return nil
}
