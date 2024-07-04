package server

import (
	"context"
	"fmt"
	"net"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/api/meta"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
)

// GRPCBroker is a gRPC broker that implements the CloudEventServiceServer interface.
// It broadcasts resource spec to Maestro agents and listens for resource status updates from them.
type GRPCBroker struct {
	pbv1.UnimplementedCloudEventServiceServer
	grpcServer         *grpc.Server
	eventBroadcaster   *event.EventBroadcaster
	resourceService    services.ResourceService
	statusEventService services.StatusEventService
	bindAddress        string
}

func NewGRPCBroker(eventBroadcaster *event.EventBroadcaster) *GRPCBroker {
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
		eventBroadcaster:   eventBroadcaster,
		resourceService:    env().Services.Resources(),
		statusEventService: env().Services.StatusEvents(),
		bindAddress:        env().Config.HTTPServer.Hostname + ":" + config.BrokerBindPort,
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

// Publish in stub implementation for maestro agent publish reosurce status back to maestro server.
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

	resource, err := decodeResourceStatus(eventType.CloudEventsDataType, evt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	found, svcErr := bkr.resourceService.Get(ctx, resource.ID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get resource %s, %s", resource.ID, svcErr.Error())
	}

	if found.ConsumerName != resource.ConsumerName {
		return nil, fmt.Errorf("unmatched consumer name %s for resource %s", resource.ConsumerName, resource.ID)
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

// Subscribe in stub implementation for maestro agent subscribe resource spec from maestro server.
func (bkr *GRPCBroker) Subscribe(subReq *pbv1.SubscriptionRequest, subServer pbv1.CloudEventService_SubscribeServer) error {
	clientID, errChan := bkr.eventBroadcaster.Register(subReq.Source, func(res *api.Resource) error {
		evt, err := encodeResourceSpec(res)
		if err != nil {
			return fmt.Errorf("failed to encode resource %s to cloudevent: %v", res.ID, err)
		}

		glog.V(4).Infof("send the event with grpc, %s", evt)

		// WARNING: don't use "pbEvt, err := pb.ToProto(evt)" to convert cloudevent to protobuf
		pbEvt := &pbv1.CloudEvent{}
		if err = grpcprotocol.WritePBMessage(context.TODO(), binding.ToMessage(evt), pbEvt); err != nil {
			return fmt.Errorf("failed to convert cloudevent to protobuf: %v", err)
		}

		// send the cloudevent to the subscriber
		// TODO: error handling to address errors beyond network issues.
		if err := subServer.Send(pbEvt); err != nil {
			return err
		}

		return nil
	})

	select {
	case err := <-errChan:
		glog.Errorf("unregister client %s, error= %v", clientID, err)
		bkr.eventBroadcaster.Unregister(clientID)
		return err
	case <-subServer.Context().Done():
		glog.V(10).Infof("unregister client %s", clientID)
		bkr.eventBroadcaster.Unregister(clientID)
		return nil
	}
}

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
	// evt.SetSource("maestro")
	// TODO set resource.Source with a new extension attribute if the agent needs
	evt.SetExtension(types.ExtensionResourceID, resource.ID)
	evt.SetExtension(types.ExtensionResourceVersion, int64(resource.Version))
	evt.SetExtension(types.ExtensionClusterName, resource.ConsumerName)

	if !resource.GetDeletionTimestamp().IsZero() {
		evt.SetExtension(types.ExtensionDeletionTimestamp, resource.GetDeletionTimestamp().Time)
	}

	return evt, nil
}

func (bkr *GRPCBroker) respondResyncSpecRequest(ctx context.Context, eventDataType types.CloudEventsDataType, evt *ce.Event) error {
	log := logger.NewOCMLogger(ctx)
	log.Infof("respondResyncSpecRequest not implemented yet")

	return nil
}
