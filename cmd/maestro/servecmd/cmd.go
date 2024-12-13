package servecmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/dao"
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
		klog.Fatalf("Unable to add environment flags to serve command: %s", err.Error())
	}

	return cmd
}

func runServer(cmd *cobra.Command, args []string) {
	err := environments.Environment().Initialize()
	if err != nil {
		klog.Fatalf("Unable to initialize environment: %s", err.Error())
	}

	healthcheckServer := server.NewHealthCheckServer()

	// Create event broadcaster to broadcast resource status update events to subscribers
	eventBroadcaster := event.NewEventBroadcaster()

	// Create the event server based on the message broker type:
	// For gRPC, create a gRPC broker to handle resource spec and status events.
	// For MQTT, create a Pulse server to handle resource spec and status events.
	var eventServer server.EventServer
	switch environments.Environment().Config.MessageBroker.MessageBrokerType {
	case "mqtt":
		klog.Info("Setting up pulse server")
		var statusDispatcher dispatcher.Dispatcher
		subscriptionType := environments.Environment().Config.EventServer.SubscriptionType
		switch config.SubscriptionType(subscriptionType) {
		case config.SharedSubscriptionType:
			statusDispatcher = dispatcher.NewNoopDispatcher(dao.NewConsumerDao(&environments.Environment().Database.SessionFactory), environments.Environment().Clients.CloudEventsSource)
		case config.BroadcastSubscriptionType:
			statusDispatcher = dispatcher.NewHashDispatcher(environments.Environment().Config.MessageBroker.ClientID, dao.NewInstanceDao(&environments.Environment().Database.SessionFactory),
				dao.NewConsumerDao(&environments.Environment().Database.SessionFactory), environments.Environment().Clients.CloudEventsSource, environments.Environment().Config.EventServer.ConsistentHashConfig)
		default:
			klog.Errorf("Unsupported subscription type: %s", subscriptionType)
		}

		// Set the status dispatcher for the healthcheck server
		healthcheckServer.SetStatusDispatcher(statusDispatcher)
		eventServer = server.NewMQTTEventServer(eventBroadcaster, statusDispatcher)
	case "grpc":
		klog.Info("Setting up grpc broker")
		eventServer = server.NewGRPCBroker(eventBroadcaster)
	default:
		klog.Errorf("Unsupported message broker type: %s", environments.Environment().Config.MessageBroker.MessageBrokerType)
	}

	// Create the servers
	apiserver := server.NewAPIServer(eventBroadcaster)
	metricsServer := server.NewMetricsServer()
	controllersServer := server.NewControllersServer(eventServer)

	ctx, cancel := context.WithCancel(context.Background())

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
		// Received SIGTERM or SIGINT signal, shutting down servers gracefully.
		if err := apiserver.Stop(); err != nil {
			klog.Errorf("Failed to stop api server, %v", err)
		}

		if err := metricsServer.Stop(); err != nil {
			klog.Errorf("Failed to stop metrics server, %v", err)
		}
	}()

	// Start the event broadcaster
	go eventBroadcaster.Start(ctx)

	// Run the servers
	go apiserver.Start()
	go metricsServer.Start()
	go healthcheckServer.Start(ctx)
	go eventServer.Start(ctx)
	go controllersServer.Start(ctx)

	<-ctx.Done()
}
