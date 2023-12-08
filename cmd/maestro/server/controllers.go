package server

import (
	"context"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/db"

	"github.com/openshift-online/maestro/pkg/logger"
)

func NewControllersServer() *ControllersServer {

	s := &ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			db.NewAdvisoryLockFactory(env().Database.SessionFactory),
			env().Services.Events(),
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

	return s
}

type ControllersServer struct {
	KindControllerManager *controllers.KindControllerManager
	DB                    db.SessionFactory
}

// Start is a blocking call that starts this controller server
func (s ControllersServer) Start(stopCh <-chan struct{}) {
	log := logger.NewOCMLogger(context.Background())

	log.Infof("Kind controller handling events")
	go s.KindControllerManager.Run(stopCh)

	log.Infof("Kind controller listening for events")
	// blocking call
	env().Database.SessionFactory.NewListener(context.Background(), "events", s.KindControllerManager.AddEvent)
}
