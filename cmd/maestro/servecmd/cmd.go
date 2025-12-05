package servecmd

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/common"
	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/dispatcher"
	"github.com/openshift-online/maestro/pkg/event"
)

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the maestro server",
		Long:  "Start the maestro server.",
		Run:   runServer,
	}
	err := environments.Environment().AddFlags(cmd.PersistentFlags())
	if err != nil {
		log.Fatalf("Unable to add environment flags to serve command: %s", err.Error())
	}

	return cmd
}

func runServer(cmd *cobra.Command, args []string) {
	// Print the git commit hash if available
	klog.Infof("Git Commit: %s", os.Getenv("GIT_COMMIT"))

	err := environments.Environment().Initialize()
	if err != nil {
		klog.Fatalf("Unable to initialize environment: %s", err.Error())
	}

	// Create event broadcaster to broadcast resource status update events to subscribers
	eventBroadcaster := event.NewEventBroadcaster()

	// initialize context and logger
	logger := klog.NewKlogr().WithName("maestro-server")
	ctx, cancel := context.WithCancel(context.Background())
	ctx = klog.NewContext(ctx, logger)

	// Create the event server based on the message broker type:
	// For gRPC, create a gRPC broker to handle resource spec and status events.
	// For MQTT/Kafka, create a message queue based event server to handle resource spec and status events.
	var eventServer server.EventServer
	var eventFilter controllers.EventFilter
	if environments.Environment().Config.MessageBroker.MessageBrokerType == "grpc" {
		logger.Info("Setting up grpc broker")
		eventServer = server.NewGRPCBroker(ctx, eventBroadcaster)
		eventFilter = controllers.NewPredicatedEventFilter(eventServer.PredicateEvent)
	} else {
		logger.Info("Setting up message queue event server")
		var statusDispatcher dispatcher.Dispatcher
		subscriptionType := environments.Environment().Config.EventServer.SubscriptionType
		switch config.SubscriptionType(subscriptionType) {
		case config.SharedSubscriptionType:
			statusDispatcher = dispatcher.NewNoopDispatcher(environments.Environment().Database.SessionFactory, environments.Environment().Clients.CloudEventsSource)
		case config.BroadcastSubscriptionType:
			statusDispatcher = dispatcher.NewHashDispatcher(environments.Environment().Config.MessageBroker.ClientID, environments.Environment().Database.SessionFactory,
				environments.Environment().Clients.CloudEventsSource, environments.Environment().Config.EventServer.ConsistentHashConfig, 5*time.Second)
		default:
			logger.Error(errors.New("Unsupported subscription type"), "failed to configure event server", "subscriptionType", subscriptionType)
			os.Exit(1)
		}
		eventServer = server.NewMessageQueueEventServer(eventBroadcaster, statusDispatcher)
		eventFilter = controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(environments.Environment().Database.SessionFactory))
	}

	// Create the servers
	apiserver := server.NewAPIServer(ctx, eventBroadcaster)
	metricsServer := server.NewMetricsServer()
	healthcheckServer := server.NewHealthCheckServer(ctx)
	controllersServer := server.NewControllersServer(ctx, eventServer, eventFilter)

	tracingShutdown := func(context.Context) error { return nil }
	if common.TracingEnabled() {
		tracingShutdown, err = common.InstallOpenTelemetryTracer(ctx, logger)
		if err != nil {
			logger.Error(err, "Can't initialize OpenTelemetry trace provider")
			os.Exit(1)
		}
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
		// Received SIGTERM or SIGINT signal, shutting down servers gracefully.
		if err := apiserver.Stop(); err != nil {
			logger.Error(err, "Failed to stop api server")
		}

		if err := metricsServer.Stop(); err != nil {
			logger.Error(err, "Failed to stop metrics server")
		}

		if tracingShutdown != nil {
			if shutdownErr := tracingShutdown(ctx); shutdownErr != nil {
				logger.Error(shutdownErr, "OpenTelemetry trace provider failed to shutdown")
			}
		}
	}()

	// Start the event broadcaster
	go eventBroadcaster.Start(ctx)

	// Run the servers
	go apiserver.Start(ctx)
	go metricsServer.Start(ctx)
	go healthcheckServer.Start(ctx)
	if !environments.Environment().Config.MessageBroker.Disable {
		// Start the event server if the message broker is not disabled
		go eventServer.Start(ctx)
	}
	go controllersServer.Start(ctx)

	<-ctx.Done()
}
