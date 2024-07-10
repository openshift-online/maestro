package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"

	"github.com/openshift-online/maestro/pkg/api"
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

// GRPCBroker is a gRPC broker that implements the CloudEventServiceServer interface.
// It broadcasts resource spec to Maestro agents and listens for resource status updates from them.
type GRPCBroker struct {
	pbv1.UnimplementedCloudEventServiceServer
	grpcServer         *grpc.Server
	resourceService    services.ResourceService
	statusEventService services.StatusEventService
	bindAddress        string
	subscribers        map[string]*subscriber // registered subscribers
	mu                 sync.RWMutex
}

// NewGRPCBroker creates a new gRPC broker with the given configuration.
func NewGRPCBroker() *GRPCBroker {
	config := *env().Config.GRPCServer
	grpcServerOptions := make([]grpc.ServerOption, 0)
	grpcServerOptions = append(grpcServerOptions, grpc.MaxRecvMsgSize(config.MaxReceiveMessageSize))
	grpcServerOptions = append(grpcServerOptions, grpc.MaxSendMsgSize(config.MaxSendMessageSize))
	grpcServerOptions = append(grpcServerOptions, grpc.MaxConcurrentStreams(config.MaxConcurrentStreams))
	grpcServerOptions = append(grpcServerOptions, grpc.ConnectionTimeout(config.ConnectionTimeout))
	grpcServerOptions = append(grpcServerOptions, grpc.WriteBufferSize(config.WriteBufferSize))
	grpcServerOptions = append(grpcServerOptions, grpc.ReadBufferSize(config.ReadBufferSize))
	grpcServerOptions = append(grpcServerOptions, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionAge: config.MaxConnectionAge,
	}))

	if config.EnableTLS {
		// Check tls cert and key path path
		if config.TLSCertFile == "" || config.TLSKeyFile == "" {
			check(
				fmt.Errorf("unspecified required --grpc-tls-cert-file, --grpc-tls-key-file"),
				"Can't start gRPC broker",
			)
		}

		// Serve with TLS
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			glog.Fatalf("Failed to generate credentials %v", err)
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(creds))
		glog.Infof("Serving gRPC broker with TLS at %s", config.BrokerBindPort)
	} else {
		glog.Infof("Serving gRPC broker without TLS at %s", config.BrokerBindPort)
	}

	return &GRPCBroker{
		grpcServer:         grpc.NewServer(grpcServerOptions...),
		resourceService:    env().Services.Resources(),
		statusEventService: env().Services.StatusEvents(),
		bindAddress:        env().Config.HTTPServer.Hostname + ":" + config.BrokerBindPort,
		subscribers:        make(map[string]*subscriber),
	}
}

// Start starts the gRPC broker
func (bkr *GRPCBroker) Start(ctx context.Context) {
	lis, err := net.Listen("tcp", bkr.bindAddress)
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}
	pbv1.RegisterCloudEventServiceServer(bkr.grpcServer, bkr)
	go func() {
		if err := bkr.grpcServer.Serve(lis); err != nil {
			glog.Fatalf("failed to start gRPC broker: %v", err)
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

	glog.V(4).Infof("receive the event with grpc, %s", evt)

	// handler resync request
	if eventType.Action == types.ResyncRequestAction {
		err := bkr.respondResyncSpecRequest(ctx, eventType.CloudEventsDataType, evt)
		if err != nil {
			return nil, fmt.Errorf("failed to respond resync spec request: %v", err)
		}
		return &emptypb.Empty{}, nil
	}

	resourceID, err := cetypes.ToString(evt.Extensions()[types.ExtensionResourceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceid extension: %v", err)
	}

	consumerName, err := cetypes.ToString(evt.Extensions()[types.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	found, svcErr := bkr.resourceService.Get(ctx, resourceID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get resource %s, %s", resourceID, svcErr.Error())
	}

	if found.ConsumerName != consumerName {
		return nil, fmt.Errorf("unmatched consumer name %s for resource %s", consumerName, resourceID)
	}

	specEvent, err := api.JSONMAPToCloudEvent(found.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource spec to cloudevent: %v", err)
	}

	// set work meta from spec event to status event
	if workMeta, ok := specEvent.Extensions()[codec.ExtensionWorkMeta]; ok {
		evt.SetExtension(codec.ExtensionWorkMeta, workMeta)
	}

	// decode the cloudevent data as resource with status
	resource, err := decodeResourceStatus(eventType.CloudEventsDataType, evt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	// set the resource source and type back for broadcast
	resource.Source = found.Source
	resource.Type = found.Type

	// decode the cloudevent data as manifest status
	statusPayload := &workpayload.ManifestStatus{}
	if err := evt.DataAs(statusPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
	}

	// if the resource has been deleted from agent, create status event and delete it from maestro
	if meta.IsStatusConditionTrue(statusPayload.Conditions, common.ManifestsDeleted) {
		_, sErr := bkr.statusEventService.Create(ctx, &api.StatusEvent{
			ResourceID:      resource.ID,
			ResourceSource:  resource.Source,
			ResourceType:    resource.Type,
			Status:          resource.Status,
			StatusEventType: api.StatusDeleteEventType,
		})
		if sErr != nil {
			return nil, fmt.Errorf("failed to create status event for resource status delete %s: %s", resource.ID, sErr.Error())
		}
		if svcErr := bkr.resourceService.Delete(ctx, resource.ID); svcErr != nil {
			return nil, fmt.Errorf("failed to delete resource %s: %s", resource.ID, svcErr.Error())
		}
	} else {
		// update the resource status
		_, updated, svcErr := bkr.resourceService.UpdateStatus(ctx, resource)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to update resource status %s: %s", resource.ID, svcErr.Error())
		}

		// create the status event only when the resource is updated
		if updated {
			_, sErr := bkr.statusEventService.Create(ctx, &api.StatusEvent{
				ResourceID:      resource.ID,
				StatusEventType: api.StatusUpdateEventType,
			})
			if sErr != nil {
				return nil, fmt.Errorf("failed to create status event for resource status update %s: %s", resource.ID, sErr.Error())
			}
		}
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

	glog.V(4).Infof("register a subscriber %s (cluster name = %s)", id, clusterName)

	return id, errChan
}

// unregister unregisters a subscriber by id
func (bkr *GRPCBroker) unregister(id string) {
	bkr.mu.Lock()
	defer bkr.mu.Unlock()

	close(bkr.subscribers[id].errChan)
	delete(bkr.subscribers, id)
}

// Subscribe in stub implementation for maestro agent subscribe resource spec from maestro server.
func (bkr *GRPCBroker) Subscribe(subReq *pbv1.SubscriptionRequest, subServer pbv1.CloudEventService_SubscribeServer) error {
	if len(subReq.ClusterName) == 0 {
		return fmt.Errorf("invalid subscription request: missing cluster name")
	}
	// subscribe the cluster for the resource spec
	subscriberID, errChan := bkr.register(subReq.ClusterName, func(res *api.Resource) error {
		evt, err := encodeResourceSpec(res)
		if err != nil {
			return fmt.Errorf("failed to encode resource %s to cloudevent: %v", res.ID, err)
		}

		// WARNING: don't use "pbEvt, err := pb.ToProto(evt)" to convert cloudevent to protobuf
		pbEvt := &pbv1.CloudEvent{}
		if err = grpcprotocol.WritePBMessage(context.TODO(), binding.ToMessage(evt), pbEvt); err != nil {
			return fmt.Errorf("failed to convert cloudevent to protobuf: %v", err)
		}

		// send the cloudevent to the subscriber
		// TODO: error handling to address errors beyond network issues.
		if err := subServer.Send(pbEvt); err != nil {
			klog.Errorf("failed to send grpc event, %v", err)
		}

		return nil
	})

	select {
	case err := <-errChan:
		glog.Errorf("unregister subscriber %s, error= %v", subscriberID, err)
		bkr.unregister(subscriberID)
		return err
	case <-subServer.Context().Done():
		glog.V(10).Infof("unregister subscriber %s", subscriberID)
		bkr.unregister(subscriberID)
		return nil
	}
}

// decodeResourceStatus translates a cloudevent to a resource containing the resource status.
func decodeResourceStatus(eventDataType types.CloudEventsDataType, evt *ce.Event) (*api.Resource, error) {
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

	switch eventDataType {
	case workpayload.ManifestEventDataType:
		resource.Type = api.ResourceTypeSingle
	case workpayload.ManifestBundleEventDataType:
		resource.Type = api.ResourceTypeBundle
	default:
		return nil, fmt.Errorf("unsupported cloudevents data type %s", eventDataType)
	}

	return resource, nil
}

// encode translates a resource spec to a cloudevent
func encodeResourceSpec(resource *api.Resource) (*ce.Event, error) {
	evt, err := api.JSONMAPToCloudEvent(resource.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource payload to cloudevent: %v", err)
	}

	eventType := types.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              types.EventAction("create_request"),
	}
	if resource.Type == api.ResourceTypeBundle {
		eventType.CloudEventsDataType = workpayload.ManifestBundleEventDataType
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
func (bkr *GRPCBroker) respondResyncSpecRequest(ctx context.Context, eventDataType types.CloudEventsDataType, evt *ce.Event) error {
	log := logger.NewOCMLogger(ctx)

	resyncType := api.ResourceTypeSingle
	if eventDataType == workpayload.ManifestBundleEventDataType {
		resyncType = api.ResourceTypeBundle
	}

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
		// only respond with the resource of the resync type
		if obj.Type != resyncType {
			continue
		}
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
			Type:         resyncType,
		}
		// mark the resource as deleting
		obj.Meta.DeletedAt.Time = time.Now()

		// send a delete event for the current resource
		bkr.handleRes(obj)
	}

	return nil
}

// handleRes publish the resource to the correct subscriber.
func (bkr *GRPCBroker) handleRes(resource *api.Resource) {
	bkr.mu.RLock()
	defer bkr.mu.RUnlock()
	for _, subscriber := range bkr.subscribers {
		if subscriber.clusterName == resource.ConsumerName {
			if err := subscriber.handler(resource); err != nil {
				subscriber.errChan <- err
			}
		}
	}
}

// OnCreate is called by the controller when a resource is created on the maestro server.
func (bkr *GRPCBroker) OnCreate(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	bkr.handleRes(resource)

	return nil
}

// OnUpdate is called by the controller when a resource is updated on the maestro server.
func (bkr *GRPCBroker) OnUpdate(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	bkr.handleRes(resource)

	return nil
}

// OnDelete is called by the controller when a resource is deleted from the maestro server.
func (bkr *GRPCBroker) OnDelete(ctx context.Context, id string) error {
	resource, err := bkr.resourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	bkr.handleRes(resource)

	return nil
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
