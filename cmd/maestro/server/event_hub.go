package server

import (
	"context"

	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
)

// EventHub is a resource spec event hub that broadcasts resource spec events to subscribers.
type EventHub struct {
	// instanceID         string
	// eventInstanceDao   dao.EventInstanceDao
	eventBroadcaster *event.EventBroadcaster
	resourceService  services.ResourceService
	// statusEventService services.StatusEventService
}

func NewEventHub(eventBroadcaster *event.EventBroadcaster) *EventHub {
	// sessionFactory := env().Database.SessionFactory
	return &EventHub{
		// instanceID:         env().Config.MessageBroker.ClientID,
		// eventInstanceDao:   dao.NewEventInstanceDao(&sessionFactory),
		eventBroadcaster: eventBroadcaster,
		resourceService:  env().Services.Resources(),
		// statusEventService: env().Services.StatusEvents(),
	}
}

func (eh *EventHub) OnCreate(ctx context.Context, resourceID string) error {
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

	return nil
}

func (eh *EventHub) OnUpdate(ctx context.Context, resourceID string) error {
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

	return nil
}

func (eh *EventHub) OnDelete(ctx context.Context, resourceID string) error {
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

	return nil
}
