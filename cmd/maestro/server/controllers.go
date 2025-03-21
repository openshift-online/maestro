package server

import (
	"context"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
)

func NewControllersServer(eventServer EventServer, eventFilter controllers.EventFilter) *ControllersServer {
	s := &ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			eventFilter,
			env().Services.Events(),
		),
		StatusController: controllers.NewStatusController(
			env().Services.StatusEvents(),
			dao.NewInstanceDao(&env().Database.SessionFactory),
			dao.NewEventInstanceDao(&env().Database.SessionFactory),
		),
	}

	s.KindControllerManager.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {eventServer.OnCreate},
			api.UpdateEventType: {eventServer.OnUpdate},
			api.DeleteEventType: {eventServer.OnDelete},
		},
	})

	s.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
		api.StatusUpdateEventType: {eventServer.OnStatusUpdate},
		api.StatusDeleteEventType: {eventServer.OnStatusUpdate},
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
