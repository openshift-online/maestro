package server

import (
	"context"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/db"

	"github.com/openshift-online/maestro/pkg/logger"
)

func NewControllersServer(pulseServer *PulseServer) *ControllersServer {

	s := &ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			db.NewAdvisoryLockFactory(env().Database.SessionFactory),
			env().Services.Events(),
		),
		StatusController: controllers.NewStatusController(
			env().Services.StatusEvents(),
		),
	}

	sourceClient := env().Clients.CloudEventsSource
	s.KindControllerManager.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {sourceClient.OnCreate},
			api.UpdateEventType: {sourceClient.OnUpdate},
			api.DeleteEventType: {sourceClient.OnDelete},
		},
	})

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
