package servecmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common"
	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/dispatcher"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
)

var log = logger.GetLogger()

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
	err := environments.Environment().Initialize()
	if err != nil {
		log.Fatalf("Unable to initialize environment: %s", err.Error())
	}

	// Create event broadcaster to broadcast resource status update events to subscribers
	eventBroadcaster := event.NewEventBroadcaster()

	// Create the event server based on the message broker type:
	// For gRPC, create a gRPC broker to handle resource spec and status events.
	// For MQTT/Kafka, create a message queue based event server to handle resource spec and status events.
	var eventServer server.EventServer
	var eventFilter controllers.EventFilter
	if environments.Environment().Config.MessageBroker.MessageBrokerType == "grpc" {
		log.Info("Setting up grpc broker")
		eventServer = server.NewGRPCBroker(eventBroadcaster)
		eventFilter = controllers.NewPredicatedEventFilter(eventServer.PredicateEvent)
	} else {
		log.Info("Setting up message queue event server")
		var statusDispatcher dispatcher.Dispatcher
		subscriptionType := environments.Environment().Config.EventServer.SubscriptionType
		switch config.SubscriptionType(subscriptionType) {
		case config.SharedSubscriptionType:
			statusDispatcher = dispatcher.NewNoopDispatcher(environments.Environment().Database.SessionFactory, environments.Environment().Clients.CloudEventsSource)
		case config.BroadcastSubscriptionType:
			statusDispatcher = dispatcher.NewHashDispatcher(environments.Environment().Config.MessageBroker.ClientID, environments.Environment().Database.SessionFactory,
				environments.Environment().Clients.CloudEventsSource, environments.Environment().Config.EventServer.ConsistentHashConfig, 5*time.Second)
		default:
			log.Errorf("Unsupported subscription type: %s", subscriptionType)
		}
		eventServer = server.NewMessageQueueEventServer(eventBroadcaster, statusDispatcher)
		eventFilter = controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(environments.Environment().Database.SessionFactory))
	}

	// Create the servers
	apiserver := server.NewAPIServer(eventBroadcaster)
	metricsServer := server.NewMetricsServer()
	healthcheckServer := server.NewHealthCheckServer()
	controllersServer := server.NewControllersServer(eventServer, eventFilter)

	ctx, cancel := context.WithCancel(context.Background())

	tracingShutdown := func(context.Context) error { return nil }
	if common.TracingEnabled() {
		tracingShutdown, err = common.InstallOpenTelemetryTracer(ctx, log)
		if err != nil {
			log.Errorf("Can't initialize OpenTelemetry trace provider: %v", err)
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
			log.Errorf("Failed to stop api server, %v", err)
		}

		if err := metricsServer.Stop(); err != nil {
			log.Errorf("Failed to stop metrics server, %v", err)
		}

		if tracingShutdown != nil && tracingShutdown(ctx) != nil {
			log.Warnf(fmt.Sprintf("OpenTelemetry trace provider failed to shutdown: %v", err))
		}
	}()

	// Start the event broadcaster
	go eventBroadcaster.Start(ctx)

	// Run the servers
	go apiserver.Start()
	go metricsServer.Start()
	go healthcheckServer.Start(ctx)
	if !environments.Environment().Config.MessageBroker.Disable {
		// Start the event server if the message broker is not disabled
		go eventServer.Start(ctx)
	}
	go controllersServer.Start(ctx)

	<-ctx.Done()
}
