package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/server"
	servergrpc "open-cluster-management.io/sdk-go/pkg/cloudevents/server/grpc"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/services"
)

var _ EventServer = &GRPCBroker{}

type GRPCBrokerService struct {
	resourceService    services.ResourceService
	statusEventService services.StatusEventService
}

func NewGRPCBrokerService(resourceService services.ResourceService,
	statusEventService services.StatusEventService) *GRPCBrokerService {
	return &GRPCBrokerService{
		resourceService:    resourceService,
		statusEventService: statusEventService,
	}
}

// Get the cloudEvent based on resourceID from the service
func (s *GRPCBrokerService) Get(ctx context.Context, resourceID string) (*ce.Event, error) {
	resource, err := s.resourceService.Get(ctx, resourceID)
	if err != nil {
		// if the resource is not found, it indicates the resource has been processed.
		if err.Is404() {
			return nil, kubeerrors.NewNotFound(schema.GroupResource{Resource: "manifestbundles"}, resourceID)
		}
		return nil, kubeerrors.NewInternalError(err)
	}

	return encodeResourceSpec(resource)
}

// List the cloudEvent from the service
func (s *GRPCBrokerService) List(listOpts types.ListOptions) ([]*ce.Event, error) {
	resources, err := s.resourceService.List(listOpts)
	if err != nil {
		return nil, err
	}

	evts := []*ce.Event{}
	for _, res := range resources {
		evt, err := encodeResourceSpec(res)
		if err != nil {
			return nil, kubeerrors.NewInternalError(err)
		}
		evts = append(evts, evt)
	}

	return evts, nil
}

// HandleStatusUpdate processes the resource status update from the agent.
func (s *GRPCBrokerService) HandleStatusUpdate(ctx context.Context, evt *ce.Event) error {
	// decode the cloudevent data as resource with status
	resource, err := decodeResourceStatus(evt)
	if err != nil {
		return fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	// handle the resource status update according status update type
	if err := handleStatusUpdate(ctx, resource, s.resourceService, s.statusEventService); err != nil {
		return fmt.Errorf("failed to handle resource status update %s: %s", resource.ID, err.Error())
	}

	return nil
}

// RegisterHandler register the handler to the service.
func (s *GRPCBrokerService) RegisterHandler(ctx context.Context, handler server.EventHandler) {
	// do nothing
}

// GRPCBroker is a gRPC broker that implements the CloudEventServiceServer interface.
// It broadcasts resource spec to Maestro agents and listens for resource status updates from them.
// TODO: Add support for multiple gRPC broker instances. When there are multiple instances of the Maestro server,
// the work agent may be load-balanced across any instance. Each instance needs to handle the resource spec to
// ensure all work agents receive all the resource spec.
type GRPCBroker struct {
	instanceID         string
	bindAddress        string
	grpcServer         *grpc.Server
	eventServer        server.AgentEventServer
	eventInstanceDao   dao.EventInstanceDao
	resourceService    services.ResourceService
	eventService       services.EventService
	statusEventService services.StatusEventService
	eventBroadcaster   *event.EventBroadcaster // event broadcaster to broadcast resource status update events to subscribers
}

// NewGRPCBroker creates a new gRPC broker with the given configuration.
func NewGRPCBroker(ctx context.Context, eventBroadcaster *event.EventBroadcaster) EventServer {
	logger := klog.FromContext(ctx)

	config := *env().Config.GRPCServer
	grpcServerOptions := make([]grpc.ServerOption, 0)
	grpcServerOptions = append(grpcServerOptions, grpc.MaxRecvMsgSize(config.MaxReceiveMessageSize))
	grpcServerOptions = append(grpcServerOptions, grpc.MaxSendMsgSize(config.MaxSendMessageSize))
	grpcServerOptions = append(grpcServerOptions, grpc.MaxConcurrentStreams(config.MaxConcurrentStreams))
	grpcServerOptions = append(grpcServerOptions, grpc.ConnectionTimeout(config.ConnectionTimeout))
	grpcServerOptions = append(grpcServerOptions, grpc.WriteBufferSize(config.WriteBufferSize))
	grpcServerOptions = append(grpcServerOptions, grpc.ReadBufferSize(config.ReadBufferSize))
	grpcServerOptions = append(grpcServerOptions, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             config.ClientMinPingInterval,
		PermitWithoutStream: config.PermitPingWithoutStream,
	}))
	grpcServerOptions = append(grpcServerOptions, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionAge: config.MaxConnectionAge,
		Time:             config.ServerPingInterval,
		Timeout:          config.ServerPingTimeout,
	}))

	disableTLS := (config.BrokerTLSCertFile == "" && config.BrokerTLSKeyFile == "")

	if !disableTLS {
		// Check tls cert and key path path
		if config.BrokerTLSCertFile == "" || config.BrokerTLSKeyFile == "" {
			check(ctx,
				fmt.Errorf("unspecified required --grpc-broker-tls-cert-file, --grpc-broker-tls-key-file"),
				"Can't start gRPC broker",
			)
		}
		// Serve with TLS
		serverCerts, err := tls.LoadX509KeyPair(config.BrokerTLSCertFile, config.BrokerTLSKeyFile)
		if err != nil {
			check(ctx, fmt.Errorf("failed to load broker certificates: %v", err), "Can't start gRPC broker")
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{serverCerts},
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
		}
		if config.BrokerClientCAFile != "" {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				check(ctx, fmt.Errorf("failed to load system cert pool: %v", err), "Can't start gRPC broker")
			}
			caPEM, err := os.ReadFile(config.BrokerClientCAFile)
			if err != nil {
				check(ctx, fmt.Errorf("failed to read broker client CA file: %v", err), "Can't start gRPC broker")
			}
			if ok := certPool.AppendCertsFromPEM(caPEM); !ok {
				check(ctx, fmt.Errorf("failed to append broker client CA to cert pool"), "Can't start gRPC broker")
			}
			tlsConfig.ClientCAs = certPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
		logger.Info("Serving gRPC broker with TLS", "port", config.BrokerBindPort)
	} else {
		logger.Info("Serving gRPC broker without TLS", "port", config.BrokerBindPort)
	}

	sessionFactory := env().Database.SessionFactory
	resourceService := env().Services.Resources()
	statusEventService := env().Services.StatusEvents()

	// TODO after the sdk go support source grpc server
	grpcServer := grpc.NewServer(grpcServerOptions...)
	eventServer := servergrpc.NewGRPCBroker()
	pbv1.RegisterCloudEventServiceServer(grpcServer, eventServer)
	svc := NewGRPCBrokerService(resourceService, statusEventService)
	eventServer.RegisterService(context.Background(), workpayload.ManifestBundleEventDataType, svc)

	return &GRPCBroker{
		instanceID:         env().Config.MessageBroker.ClientID,
		bindAddress:        env().Config.HTTPServer.Hostname + ":" + config.BrokerBindPort,
		grpcServer:         grpcServer,
		eventServer:        eventServer,
		eventInstanceDao:   dao.NewEventInstanceDao(&sessionFactory),
		resourceService:    resourceService,
		eventService:       env().Services.Events(),
		statusEventService: statusEventService,
		eventBroadcaster:   eventBroadcaster,
	}
}

// Start starts the gRPC broker
func (bkr *GRPCBroker) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	ln, err := net.Listen("tcp", bkr.bindAddress)
	if err != nil {
		check(ctx, err, "Failed to start gRPC broker listener")
	}

	go func() {
		if err := bkr.grpcServer.Serve(ln); err != nil {
			check(ctx, err, "gRPC broker terminated with errors")
		}
	}()

	// wait until context is done
	<-ctx.Done()
	logger.Info("Stopping gRPC broker", "bindAddress", bkr.bindAddress)
	bkr.grpcServer.GracefulStop()
}

// OnCreate is called by the controller when a resource is created on the maestro server.
func (s *GRPCBroker) OnCreate(ctx context.Context, resourceID string) error {
	return s.eventServer.OnCreate(ctx, workpayload.ManifestBundleEventDataType, resourceID)
}

// OnUpdate is called by the controller when a resource is updated on the maestro server.
func (s *GRPCBroker) OnUpdate(ctx context.Context, resourceID string) error {
	return s.eventServer.OnUpdate(ctx, workpayload.ManifestBundleEventDataType, resourceID)
}

// OnDelete is called by the controller when a resource is deleted from the maestro server.
func (s *GRPCBroker) OnDelete(ctx context.Context, resourceID string) error {
	return s.eventServer.OnDelete(ctx, workpayload.ManifestBundleEventDataType, resourceID)
}

// On StatusUpdate will be called on each new status event inserted into db.
// It does two things:
// 1. build the resource status and broadcast it to subscribers
// 2. add the event instance record to mark the event has been processed by the current instance
func (s *GRPCBroker) OnStatusUpdate(ctx context.Context, eventID, resourceID string) error {
	return broadcastStatusEvent(
		ctx,
		s.statusEventService,
		s.resourceService,
		s.eventInstanceDao,
		s.eventBroadcaster,
		s.instanceID,
		eventID,
		resourceID,
	)
}

// PredicateEvent checks if the event should be processed by the current instance
// by verifying the resource consumer name is in the subscriber list, ensuring the
// event will be only processed when the consumer is subscribed to the current broker.
func (s *GRPCBroker) PredicateEvent(ctx context.Context, eventID string) (bool, error) {
	logger := klog.FromContext(ctx)
	evt, err := s.eventService.Get(ctx, eventID)
	if err != nil {
		return false, fmt.Errorf("failed to get event %s: %s", eventID, err.Error())
	}

	// fast return if the event is already reconciled
	if evt.ReconciledDate != nil {
		return false, nil
	}

	resource, svcErr := s.resourceService.Get(ctx, evt.SourceID)
	if svcErr != nil {
		// if the resource is not found, it indicates the resource has been handled by other instances.
		if svcErr.Is404() {
			logger.V(4).Info("The resource has been deleted, mark the event as reconciled", "resourceID", evt.SourceID)
			now := time.Now()
			evt.ReconciledDate = &now
			if _, svcErr := s.eventService.Replace(ctx, evt); svcErr != nil {
				return false, fmt.Errorf("failed to mark event with id (%s) as reconciled: %s", evt.ID, svcErr)
			}
			return false, nil
		}
		return false, fmt.Errorf("failed to get resource %s: %s", evt.SourceID, svcErr.Error())
	}

	// check if the consumer is subscribed to the broker
	return s.eventServer.Subscribers().Has(resource.ConsumerName), nil
}

// decodeResourceStatus translates a CloudEvent into a resource containing the status JSON map.
func decodeResourceStatus(evt *ce.Event) (*api.Resource, error) {
	evtExtensions := evt.Context.GetExtensions()

	clusterName, err := cetypes.ToString(evtExtensions[types.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	resourceID, err := cetypes.ToString(evtExtensions[types.ExtensionResourceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceid extension: %v", err)
	}

	resourceVersion, err := cetypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
	}

	status, err := api.CloudEventToJSONMap(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource status: %v", err)
	}

	resource := &api.Resource{
		Source:       evt.Source(),
		ConsumerName: clusterName,
		Version:      resourceVersion,
		Meta: api.Meta{
			ID: resourceID,
		},
		Status: status,
	}

	return resource, nil
}

// encodeResourceSpec translates a resource spec JSON map into a CloudEvent.
func encodeResourceSpec(resource *api.Resource) (*ce.Event, error) {
	evt, err := api.JSONMAPToCloudEvent(resource.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource payload to cloudevent: %v", err)
	}

	eventType := types.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              types.EventAction("create_request"),
	}
	evt.SetType(eventType.String())
	evt.SetSource("maestro")
	// TODO set resource.Source with a new extension attribute if the agent needs
	evt.SetExtension(types.ExtensionResourceID, resource.ID)
	evt.SetExtension(types.ExtensionResourceVersion, int64(resource.Version))
	evt.SetExtension(types.ExtensionClusterName, resource.ConsumerName)

	if !resource.GetDeletionTimestamp().IsZero() {
		evt.SetExtension(types.ExtensionDeletionTimestamp, resource.GetDeletionTimestamp().Time)
	}

	return evt, nil
}
