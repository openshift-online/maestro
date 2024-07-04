package server

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
)

// EventHub is a resource spec event hub that broadcasts resource spec events to subscribers.
type EventHub struct {
	instanceID       string
	eventInstanceDao dao.EventInstanceDao
	eventBroadcaster *event.EventBroadcaster
	resourceService  services.ResourceService
}

func NewEventHub(eventBroadcaster *event.EventBroadcaster) *EventHub {
	sessionFactory := env().Database.SessionFactory
	return &EventHub{
		instanceID:       env().Config.MessageBroker.ClientID,
		eventInstanceDao: dao.NewEventInstanceDao(&sessionFactory),
		eventBroadcaster: eventBroadcaster,
		resourceService:  env().Services.Resources(),
	}
}

func (eh *EventHub) OnCreate(ctx context.Context, eventID, resourceID string) error {
	log := logger.NewOCMLogger(ctx)

	resource, err := eh.resourceService.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	// override the resource source before broadcasting to subscribers
	resource.Source = "maestro"
	// broadcast the resource spec to subscribers
	log.V(4).Infof("Broadcast the resource create %s", resource.ID)
	// TODO: pass the event type to subscribers
	eh.eventBroadcaster.Broadcast(resource)

	// add the event instance record
	if _, sErr := eh.eventInstanceDao.Create(ctx, &api.EventInstance{
		EventID:    eventID,
		InstanceID: eh.instanceID,
	}); sErr != nil {
		return fmt.Errorf("error creating event instance record: %v", sErr)
	}

	return nil
}

func (eh *EventHub) OnUpdate(ctx context.Context, eventID, resourceID string) error {
	log := logger.NewOCMLogger(ctx)

	resource, err := eh.resourceService.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	// override the resource source before broadcasting to subscribers
	resource.Source = "maestro"
	// broadcast the resource spec to subscribers
	log.V(4).Infof("Broadcast the resource update %s", resource.ID)
	// TODO: pass the event type to subscribers
	eh.eventBroadcaster.Broadcast(resource)

	// add the event instance record
	if _, sErr := eh.eventInstanceDao.Create(ctx, &api.EventInstance{
		EventID:    eventID,
		InstanceID: eh.instanceID,
	}); sErr != nil {
		return fmt.Errorf("error creating event instance record: %v", sErr)
	}

	return nil
}

func (eh *EventHub) OnDelete(ctx context.Context, eventID, resourceID string) error {
	log := logger.NewOCMLogger(ctx)

	resource, err := eh.resourceService.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	// override the resource source before broadcasting to subscribers
	resource.Source = "maestro"
	// broadcast the resource spec to subscribers
	log.V(4).Infof("Broadcast the resource delete %s", resource.ID)
	// TODO: pass the event type to subscribers
	eh.eventBroadcaster.Broadcast(resource)

	// add the event instance record
	if _, sErr := eh.eventInstanceDao.Create(ctx, &api.EventInstance{
		EventID:    eventID,
		InstanceID: eh.instanceID,
	}); sErr != nil {
		return fmt.Errorf("error creating event instance record: %v", sErr)
	}

	return nil
}
