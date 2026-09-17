package server

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
)

// NewControllersServer wires event processing and broker-enabled delete recovery controllers.
func NewControllersServer(ctx context.Context, eventServer EventServer, eventFilter controllers.EventFilter) *ControllersServer {
	logger := klog.FromContext(ctx)

	s := &ControllersServer{
		StatusController: controllers.NewStatusController(
			env().Services.StatusEvents(),
			dao.NewInstanceDao(&env().Database.SessionFactory),
			dao.NewEventInstanceDao(&env().Database.SessionFactory),
		),
	}

	// disable the spec controller if the message broker is disabled
	if !env().Config.MessageBroker.Disable {
		logger.V(4).Info("Message broker is enabled, setting up kind controller manager")
		s.KindControllerManager = controllers.NewKindControllerManager(
			eventFilter,
			env().Services.Events(),
		)

		s.KindControllerManager.Add(&controllers.ControllerConfig{
			Source: "Resources",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {eventServer.OnCreate},
				api.UpdateEventType: {eventServer.OnUpdate},
				api.DeleteEventType: {eventServer.OnDelete},
			},
		})

		threshold := env().Config.EventServer.UndeliveredResourceThreshold
		if threshold > 0 {
			s.UndeliveredDetector = controllers.NewUndeliveredDetector(
				env().Services.Resources(),
				env().Services.Events(),
				db.NewAdvisoryLockFactory(env().Database.SessionFactory),
				threshold,
			)
		}

		staleDeleteThreshold := env().Config.EventServer.StaleDeleteEventThreshold
		if staleDeleteThreshold > 0 {
			s.StaleDeleteDetector = controllers.NewStaleDeleteDetector(
				env().Services.Events(),
				db.NewAdvisoryLockFactory(env().Database.SessionFactory),
				staleDeleteThreshold,
			)
		}
		if env().Config.EventServer.DeleteEventRepublishInterval > 0 {
			recovery, err := dao.NewDeleteRecovery(env().Database.SessionFactory, env().Config.EventServer)
			if err != nil {
				// ReadFiles validates these settings before server construction.
				panic(err)
			}
			s.DeleteRecovery = controllers.NewDeleteRecoveryController(recovery)
		}
	}

	s.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
		api.StatusUpdateEventType: {eventServer.OnStatusUpdate},
		api.StatusDeleteEventType: {eventServer.OnStatusUpdate},
	})

	return s
}

type ControllersServer struct {
	KindControllerManager *controllers.KindControllerManager
	StatusController      *controllers.StatusController
	UndeliveredDetector   *controllers.UndeliveredDetector
	StaleDeleteDetector   *controllers.StaleDeleteDetector
	DeleteRecovery        *controllers.DeleteRecoveryController

	DB db.SessionFactory
}

// Start is a blocking call that starts this controller server
func (s ControllersServer) Start(ctx context.Context) {
	logger := klog.FromContext(ctx).WithName("maestro-controllers")
	ctx = klog.NewContext(ctx, logger)
	if s.KindControllerManager != nil {
		logger.Info("Kind controller handling events")
		go s.KindControllerManager.Run(ctx)

		logger.Info("Kind controller listening for events")
		go env().Database.SessionFactory.NewListener(ctx, "events", s.KindControllerManager.AddEvent)
	}

	if s.UndeliveredDetector != nil {
		logger.Info("Starting undelivered resource detector")
		go wait.JitterUntilWithContext(ctx, s.UndeliveredDetector.Run, 2*time.Minute, 0.25, true)
	}

	if s.StaleDeleteDetector != nil {
		logger.Info("Starting stale delete event detector")
		go wait.JitterUntilWithContext(ctx, s.StaleDeleteDetector.Run, 2*time.Minute, 0.25, true)
	}
	if s.DeleteRecovery != nil {
		logger.Info("Starting delete recovery scheduler")
		go wait.UntilWithContext(ctx, s.DeleteRecovery.Run, dao.DeleteRecoveryRoundInterval)
		logger.Info("Starting delete recovery metrics reporter")
		go wait.JitterUntilWithContext(ctx, s.DeleteRecovery.Report, controllers.DeleteRecoverySnapshotReportInterval, 0.25, true)
	}

	logger.Info("Status controller handling events")
	go s.StatusController.Run(ctx)
	logger.Info("Status controller listening for status events")
	go env().Database.SessionFactory.NewListener(ctx, "status_events", s.StatusController.AddStatusEvent)

	// block until the context is done
	<-ctx.Done()
}
