package server

import (
	"context"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/db"

	"github.com/openshift-online/maestro/pkg/logger"
)

func NewControllersServer(pulseServer *PulseServer, grpcBroker *GRPCBroker) *ControllersServer {
	var lockFactory db.LockFactory

	// Determine the appropriate lock factory based on whether the gRPC broker is enabled.
	// When the gRPC broker is enabled, create an EventAdvisoryLockFactory.
	// This factory ensures that the consumer is subscribed to the current gRPC broker
	// to correctly handle events across multiple Maestro instances.
	// Without this check, events might be handled by the wrong gRPC broker instance,
	// leading to agents not receiving expected events.
	if grpcBroker != nil {
		// Create an EventAdvisoryLockFactory using the database session factory and
		// the gRPC broker's consumer subscription checker.
		lockFactory = db.NewEventAdvisoryLockFactory(env().Database.SessionFactory, grpcBroker.IsConsumerSubscribed)
	} else {
		// Create an EventAdvisoryLockFactory using the database session factory and
		// the gRPC broker's consumer subscription checker.
		lockFactory = db.NewAdvisoryLockFactory(env().Database.SessionFactory)
	}

	s := &ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			lockFactory,
			env().Services.Events(),
		),
		StatusController: controllers.NewStatusController(
			env().Services.StatusEvents(),
		),
	}

	sourceClient := env().Clients.CloudEventsSource
	controllerConfig := &controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {sourceClient.OnCreate},
			api.UpdateEventType: {sourceClient.OnUpdate},
			api.DeleteEventType: {sourceClient.OnDelete},
		},
	}
	if grpcBroker != nil {
		controllerConfig.Handlers[api.CreateEventType] = append(controllerConfig.Handlers[api.CreateEventType], grpcBroker.OnCreate)
		controllerConfig.Handlers[api.UpdateEventType] = append(controllerConfig.Handlers[api.UpdateEventType], grpcBroker.OnUpdate)
		controllerConfig.Handlers[api.DeleteEventType] = append(controllerConfig.Handlers[api.DeleteEventType], grpcBroker.OnDelete)
	}
	s.KindControllerManager.Add(controllerConfig)

	s.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
		api.StatusUpdateEventType: {pulseServer.OnStatusUpdate},
		api.StatusDeleteEventType: {pulseServer.OnStatusUpdate},
	})

	return s
}

type ControllersServer struct {
	KindControllerManager *controllers.KindControllerManager
	StatusController      *controllers.StatusController

	DB db.SessionFactory
}

// Start is a blocking call that starts this controller server
func (s ControllersServer) Start(ctx context.Context) {
	log := logger.NewOCMLogger(ctx)

	log.Infof("Kind controller handling events")
	go s.KindControllerManager.Run(ctx.Done())
	log.Infof("Status controller handling events")
	go s.StatusController.Run(ctx.Done())

	log.Infof("Kind controller listening for events")
	go env().Database.SessionFactory.NewListener(ctx, "events", s.KindControllerManager.AddEvent)
	log.Infof("Status controller listening for status events")
	go env().Database.SessionFactory.NewListener(ctx, "status_events", s.StatusController.AddStatusEvent)

	// block until the context is done
	<-ctx.Done()
}
