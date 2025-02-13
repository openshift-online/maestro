package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/klog/v2"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
)

type resourceHandler func(res *api.Resource) error

// subscriber defines a subscriber that can receive and handle resource spec.
type subscriber struct {
	clusterName string
	handler     resourceHandler
	errChan     chan<- error
}

var _ EventServer = &GRPCBroker{}

// GRPCBroker is a gRPC broker that implements the CloudEventServiceServer interface.
// It broadcasts resource spec to Maestro agents and listens for resource status updates from them.
// TODO: Add support for multiple gRPC broker instances. When there are multiple instances of the Maestro server,
// the work agent may be load-balanced across any instance. Each instance needs to handle the resource spec to
// ensure all work agents receive all the resource spec.
type GRPCBroker struct {
	pbv1.UnimplementedCloudEventServiceServer
	grpcServer         *grpc.Server
	instanceID         string
	eventInstanceDao   dao.EventInstanceDao
	resourceService    services.ResourceService
	eventService       services.EventService
	statusEventService services.StatusEventService
	bindAddress        string
	subscribers        map[string]*subscriber  // registered subscribers
	eventBroadcaster   *event.EventBroadcaster // event broadcaster to broadcast resource status update events to subscribers
	mu                 sync.RWMutex
}

// NewGRPCBroker creates a new gRPC broker with the given configuration.
func NewGRPCBroker(eventBroadcaster *event.EventBroadcaster) EventServer {
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

	if !config.DisableTLS {
		// Check tls cert and key path path
		if config.BrokerTLSCertFile == "" || config.BrokerTLSKeyFile == "" {
			check(
				fmt.Errorf("unspecified required --grpc-broker-tls-cert-file, --grpc-broker-tls-key-file"),
				"Can't start gRPC broker",
			)
		}
		// Serve with TLS
		serverCerts, err := tls.LoadX509KeyPair(config.BrokerTLSCertFile, config.BrokerTLSKeyFile)
		if err != nil {
			check(fmt.Errorf("failed to load broker certificates: %v", err), "Can't start gRPC broker")
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{serverCerts},
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
		}
		if config.BrokerClientCAFile != "" {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				check(fmt.Errorf("failed to load system cert pool: %v", err), "Can't start gRPC broker")
			}
			caPEM, err := os.ReadFile(config.BrokerClientCAFile)
			if err != nil {
				check(fmt.Errorf("failed to read broker client CA file: %v", err), "Can't start gRPC broker")
			}
			if ok := certPool.AppendCertsFromPEM(caPEM); !ok {
				check(fmt.Errorf("failed to append broker client CA to cert pool"), "Can't start gRPC broker")
			}
			tlsConfig.ClientCAs = certPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
		klog.Infof("Serving gRPC broker with TLS at %s", config.ServerBindPort)
	} else {
		klog.Infof("Serving gRPC broker without TLS at %s", config.ServerBindPort)
	}

	sessionFactory := env().Database.SessionFactory
	return &GRPCBroker{
		grpcServer:         grpc.NewServer(grpcServerOptions...),
		instanceID:         env().Config.MessageBroker.ClientID,
		eventInstanceDao:   dao.NewEventInstanceDao(&sessionFactory),
		resourceService:    env().Services.Resources(),
		eventService:       env().Services.Events(),
		statusEventService: env().Services.StatusEvents(),
		bindAddress:        env().Config.HTTPServer.Hostname + ":" + config.BrokerBindPort,
		subscribers:        make(map[string]*subscriber),
		eventBroadcaster:   eventBroadcaster,
	}
}

// Start starts the gRPC broker
func (bkr *GRPCBroker) Start(ctx context.Context) {
	klog.Info("Starting gRPC broker")
	lis, err := net.Listen("tcp", bkr.bindAddress)
	if err != nil {
		check(fmt.Errorf("failed to listen: %v", err), "Can't start gRPC broker")
	}
	pbv1.RegisterCloudEventServiceServer(bkr.grpcServer, bkr)
	go func() {
		if err := bkr.grpcServer.Serve(lis); err != nil {
			check(fmt.Errorf("failed to serve gRPC broker: %v", err), "Can't start gRPC broker")
		}
	}()

	// wait until context is canceled
	<-ctx.Done()
	log.Infof("Shutting down gRPC broker")
}

// Publish in stub implementation for maestro agent publish resource status back to maestro server.
func (bkr *GRPCBroker) Publish(ctx context.Context, pubReq *pbv1.PublishRequest) (*emptypb.Empty, error) {
	// WARNING: don't use "evt, err := pb.FromProto(pubReq.Event)" to convert protobuf to cloudevent
	evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pubReq.Event))
	if err != nil {
		return nil, fmt.Errorf("failed to convert protobuf to cloudevent: %v", err)
	}

	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	klog.V(4).Infof("receive the event with grpc broker, %s", evt)

	// handler resync request
	if eventType.Action == types.ResyncRequestAction {
		err := bkr.respondResyncSpecRequest(ctx, evt)
		if err != nil {
			return nil, fmt.Errorf("failed to respond resync spec request: %v", err)
		}
		return &emptypb.Empty{}, nil
	}

	// decode the cloudevent data as resource with status
	resource, err := decodeResourceStatus(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	// handle the resource status update according status update type
	if err := handleStatusUpdate(ctx, resource, bkr.resourceService, bkr.statusEventService); err != nil {
		return nil, fmt.Errorf("failed to handle resource status update %s: %s", resource.ID, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// register registers a subscriber and return client id and error channel.
func (bkr *GRPCBroker) register(clusterName string, handler resourceHandler) (string, <-chan error) {
	bkr.mu.Lock()
	defer bkr.mu.Unlock()

	id := uuid.NewString()
	errChan := make(chan error)
	bkr.subscribers[id] = &subscriber{
		clusterName: clusterName,
		handler:     handler,
		errChan:     errChan,
	}

	klog.V(4).Infof("registered a subscriber %s (cluster name = %s)", id, clusterName)
	return id, errChan
}

// unregister unregisters a subscriber by id
func (bkr *GRPCBroker) unregister(id string) {
	bkr.mu.Lock()
	defer bkr.mu.Unlock()

	close(bkr.subscribers[id].errChan)
	delete(bkr.subscribers, id)
	klog.V(4).Infof("unregistered subscriber %s", id)
}

// Subscribe in stub implementation for maestro agent subscribe resource spec from maestro server.
// Note: It's unnecessary to send a status resync request to Maestro agent subscribers.
// The work agent will continuously attempt to send status updates to the gRPC broker.
// If the broker is down or disconnected, the agent will resend the status once the broker is back up or reconnected.
func (bkr *GRPCBroker) Subscribe(subReq *pbv1.SubscriptionRequest, subServer pbv1.CloudEventService_SubscribeServer) error {
	if len(subReq.ClusterName) == 0 {
		return fmt.Errorf("invalid subscription request: missing cluster name")
	}
	// register the cluster for subscription to the resource spec
	subscriberID, errChan := bkr.register(subReq.ClusterName, func(res *api.Resource) error {
		evt, err := encodeResourceSpec(res)
		if err != nil {
			// return the error to requeue the event if encoding fails (e.g., due to invalid resource spec).
			return fmt.Errorf("failed to encode resource %s to cloudevent: %v", res.ID, err)
		}

		// WARNING: don't use "pbEvt, err := pb.ToProto(evt)" to convert cloudevent to protobuf
		pbEvt := &pbv1.CloudEvent{}
		if err = grpcprotocol.WritePBMessage(context.TODO(), binding.ToMessage(evt), pbEvt); err != nil {
			// return the error to requeue the event if converting to protobuf fails (e.g., due to invalid cloudevent).
			return fmt.Errorf("failed to convert cloudevent to protobuf for resource(%s): %v", res.ID, err)
		}

		// send the cloudevent to the subscriber
		klog.V(4).Infof("sending the event to spec subscribers, %s", evt)
		if err := subServer.Send(pbEvt); err != nil {
			klog.Errorf("failed to send grpc event, %v", err)
			// Return the error without wrapping, as it includes the gRPC error code and message for further handling.
			// For unrecoverable errors, such as a connection closed by an intermediate proxy, push the error to subscriber's
			// error channel to unregister the subscriber.
			return err
		}

		return nil
	})

	select {
	case err := <-errChan:
		// When reaching this point, an unrecoverable error occurred while sending the event,
		// such as the connection being closed. Unregister the subscriber to trigger agent reconnection.
		klog.Infof("unregistering subscriber %s because unrecoverable error= %v", subscriberID, err)
		bkr.unregister(subscriberID)
		return err
	case <-subServer.Context().Done():
		// The context of the stream has been canceled or completed.
		// This could happen if:
		// - The client closed the connection or canceled the stream.
		// - The server closed the stream, potentially due to a shutdown.
		// Regardless of the reason, unregister the subscriber and stop processing.
		// No error is returned here because the stream closure is expected.
		bkr.unregister(subscriberID)
		return nil
	}
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

// Upon receiving the spec resync event, the source responds by sending resource status events to the broker as follows:
//   - If the request event message is empty, the source returns all resources associated with the work agent.
//   - If the request event message contains resource IDs and versions, the source retrieves the resource with the
//     specified ID and compares the versions.
//   - If the requested resource version matches the source's current maintained resource version, the source does not
//     resend the resource.
//   - If the requested resource version is older than the source's current maintained resource version, the source
//     sends the resource.
//   - If the resource does not exist on the source, but exists on the agent, the source sends a delete event for the
//     resource.
func (bkr *GRPCBroker) respondResyncSpecRequest(ctx context.Context, evt *ce.Event) error {
	log := logger.NewOCMLogger(ctx)

	resourceVersions, err := payload.DecodeSpecResyncRequest(*evt)
	if err != nil {
		return err
	}

	clusterNameValue, err := evt.Context.GetExtension(types.ExtensionClusterName)
	if err != nil {
		return err
	}
	clusterName := fmt.Sprintf("%s", clusterNameValue)

	objs, err := bkr.resourceService.List(types.ListOptions{ClusterName: clusterName})
	if err != nil {
		return err
	}

	if len(objs) == 0 {
		log.V(4).Infof("there are is no objs from the list, do nothing")
		return nil
	}

	for _, obj := range objs {
		// respond with the deleting resource regardless of the resource version
		if !obj.GetDeletionTimestamp().IsZero() {
			bkr.handleRes(obj)
			continue
		}

		lastResourceVersion := findResourceVersion(string(obj.GetUID()), resourceVersions.Versions)
		currentResourceVersion, err := strconv.ParseInt(obj.GetResourceVersion(), 10, 64)
		if err != nil {
			log.V(4).Infof("ignore the obj %v since it has a invalid resourceVersion, %v", obj, err)
			continue
		}

		// the version of the work is not maintained on source or the source's work is newer than agent, send
		// the newer work to agent
		if currentResourceVersion == 0 || currentResourceVersion > lastResourceVersion {
			bkr.handleRes(obj)
		}
	}

	// the resources do not exist on the source, but exist on the agent, delete them
	for _, rv := range resourceVersions.Versions {
		_, exists := getObj(rv.ResourceID, objs)
		if exists {
			continue
		}

		obj := &api.Resource{
			Meta: api.Meta{
				ID: rv.ResourceID,
			},
			Version:      int32(rv.ResourceVersion),
			ConsumerName: clusterName,
		}
		// mark the resource as deleting
		obj.Meta.DeletedAt.Time = time.Now()

		// send a delete event for the current resource
		bkr.handleRes(obj)
	}

	return nil
}

// handleRes publish the resource to the correct subscriber.
func (bkr *GRPCBroker) handleRes(resource *api.Resource) error {
	bkr.mu.RLock()
	defer bkr.mu.RUnlock()
	for _, subscriber := range bkr.subscribers {
		if subscriber.clusterName == resource.ConsumerName {
			if err := subscriber.handler(resource); err != nil {
				// check if the error is recoverable. For unrecoverable errors,
				// such as a connection closed by an intermediate proxy, push
				// the error to subscriber's error channel to unregister the subscriber.
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Unavailable {
					// TODO: handle more error codes that can't be recovered
					subscriber.errChan <- err
				}
				return err
			}
		}
	}
	return nil
}

// OnCreate is called by the controller when a resource is created on the maestro server.
func (bkr *GRPCBroker) OnCreate(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		// if the resource is not found, it indicates the resource has been processed.
		if err.Is404() {
			return nil
		}
		return err
	}

	return bkr.handleRes(resource)
}

// OnUpdate is called by the controller when a resource is updated on the maestro server.
func (bkr *GRPCBroker) OnUpdate(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		// if the resource is not found, it indicates the resource has been processed.
		if err.Is404() {
			return nil
		}
		return err
	}

	return bkr.handleRes(resource)
}

// OnDelete is called by the controller when a resource is deleted from the maestro server.
func (bkr *GRPCBroker) OnDelete(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		// if the resource is not found, it indicates the resource has been processed.
		if err.Is404() {
			return nil
		}
		return err
	}

	return bkr.handleRes(resource)
}

// On StatusUpdate will be called on each new status event inserted into db.
// It does two things:
// 1. build the resource status and broadcast it to subscribers
// 2. add the event instance record to mark the event has been processed by the current instance
func (bkr *GRPCBroker) OnStatusUpdate(ctx context.Context, eventID, resourceID string) error {
	return broadcastStatusEvent(
		ctx,
		bkr.statusEventService,
		bkr.resourceService,
		bkr.eventInstanceDao,
		bkr.eventBroadcaster,
		bkr.instanceID,
		eventID,
		resourceID,
	)
}

// PredicateEvent checks if the event should be processed by the current instance
// by verifying the resource consumer name is in the subscriber list, ensuring the
// event will be only processed when the consumer is subscribed to the current broker.
func (bkr *GRPCBroker) PredicateEvent(ctx context.Context, eventID string) (bool, error) {
	evt, err := bkr.eventService.Get(ctx, eventID)
	if err != nil {
		return false, fmt.Errorf("failed to get event %s: %s", eventID, err.Error())
	}

	// fast return if the event is already reconciled
	if evt.ReconciledDate != nil {
		return false, nil
	}

	resource, svcErr := bkr.resourceService.Get(ctx, evt.SourceID)
	if svcErr != nil {
		// if the resource is not found, it indicates the resource has been handled by other instances.
		if svcErr.Is404() {
			klog.V(10).Infof("The resource %s has been deleted, mark the event as reconciled", evt.SourceID)
			now := time.Now()
			evt.ReconciledDate = &now
			if _, svcErr := bkr.eventService.Replace(ctx, evt); svcErr != nil {
				return false, fmt.Errorf("failed to mark event with id (%s) as reconciled: %s", evt.ID, svcErr)
			}
			return false, nil
		}
		return false, fmt.Errorf("failed to get resource %s: %s", evt.SourceID, svcErr.Error())
	}

	// check if the consumer is subscribed to the broker
	return bkr.IsConsumerSubscribed(resource.ConsumerName), nil
}

// IsConsumerSubscribed returns true if the consumer is subscribed to the broker for resource spec.
func (bkr *GRPCBroker) IsConsumerSubscribed(consumerName string) bool {
	bkr.mu.RLock()
	defer bkr.mu.RUnlock()
	for _, subscriber := range bkr.subscribers {
		if subscriber.clusterName == consumerName {
			return true
		}
	}
	return false
}

// findResourceVersion returns the resource version for the given ID from the list of resource versions.
func findResourceVersion(id string, versions []payload.ResourceVersion) int64 {
	for _, version := range versions {
		if id == version.ResourceID {
			return version.ResourceVersion
		}
	}

	return 0
}

// getObj returns the object with the given ID from the list of resources.
func getObj(id string, objs []*api.Resource) (*api.Resource, bool) {
	for _, obj := range objs {
		if obj.ID == id {
			return obj, true
		}
	}

	return nil, false
}
