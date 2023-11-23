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

	resourceServices := env().Services.Resources()
	sourceClient := env().Clients.SourceClient

	s.KindControllerManager.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {resourceServices.OnUpsert, sourceClient.OnCreate},
			api.UpdateEventType: {resourceServices.OnUpsert, sourceClient.OnUpdate},
			api.DeleteEventType: {resourceServices.OnDelete, sourceClient.OnDelete},
		},
	})

	return s
}

type ControllersServer struct {
	KindControllerManager *controllers.KindControllerManager
	DB                    db.SessionFactory
}

// Start is a blocking call that starts this controller server
func (s ControllersServer) Start() {
	log := logger.NewOCMLogger(context.Background())

	log.Infof("Kind controller listening for events")

	// blocking call
	env().Database.SessionFactory.NewListener(context.Background(), "events", s.KindControllerManager.Handle)
}
