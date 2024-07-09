package servecmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/server"
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
		glog.Fatalf("Unable to add environment flags to serve command: %s", err.Error())
	}

	return cmd
}

func runServer(cmd *cobra.Command, args []string) {
	err := environments.Environment().Initialize()
	if err != nil {
		glog.Fatalf("Unable to initialize environment: %s", err.Error())
	}

	// Create event broadcaster to broadcast resource status update events to subscribers
	eventBroadcaster := event.NewEventBroadcaster()

	// Create the GRPC broker if enabled
	var grpcBroker *server.GRPCBroker
	if environments.Environment().Config.GRPCServer.EnableGRPCBroker {
		grpcBroker = server.NewGRPCBroker()
	}
	// Create the servers
	apiserver := server.NewAPIServer(eventBroadcaster)
	metricsServer := server.NewMetricsServer()
	healthcheckServer := server.NewHealthCheckServer()
	pulseServer := server.NewPulseServer(eventBroadcaster)
	controllersServer := server.NewControllersServer(pulseServer, grpcBroker)

	ctx, cancel := context.WithCancel(context.Background())

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
		// Received SIGTERM or SIGINT signal, shutting down servers gracefully.
		if err := apiserver.Stop(); err != nil {
			glog.Errorf("Failed to stop api server, %v", err)
		}

		if err := metricsServer.Stop(); err != nil {
			glog.Errorf("Failed to stop metrics server, %v", err)
		}

		if err := healthcheckServer.Stop(); err != nil {
			glog.Errorf("Failed to stop healthcheck server, %v", err)
		}
	}()

	// Start the event broadcaster
	go eventBroadcaster.Start(ctx)

	// Run the servers
	go apiserver.Start()
	go metricsServer.Start()
	go healthcheckServer.Start()
	go pulseServer.Start(ctx)
	go controllersServer.Start(ctx)
	// Start the GRPC broker if enabled
	if grpcBroker != nil {
		go grpcBroker.Start(ctx)
	}

	<-ctx.Done()
}
