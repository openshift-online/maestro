package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
	workv1 "open-cluster-management.io/api/work/v1"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/services"
)

// GRPCServer includes a gRPC server and a resource service
type GRPCServer struct {
	pbv1.UnimplementedCloudEventServiceServer
	grpcServer       *grpc.Server
	eventBroadcaster *event.EventBroadcaster
	resourceService  services.ResourceService
	bindAddress      string
}

// NewGRPCServer creates a new GRPCServer
func NewGRPCServer(resourceService services.ResourceService, eventBroadcaster *event.EventBroadcaster, config config.GRPCServerConfig) *GRPCServer {
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
				"Can't start gRPC server",
			)
		}

		// Serve with TLS
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			glog.Fatalf("Failed to generate credentials %v", err)
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(creds))
		glog.Infof("Serving gRPC service with TLS at %s", config.BindPort)
	} else {
		glog.Infof("Serving gRPC service without TLS at %s", config.BindPort)
	}

	return &GRPCServer{
		grpcServer:       grpc.NewServer(grpcServerOptions...),
		eventBroadcaster: eventBroadcaster,
		resourceService:  resourceService,
		bindAddress:      env().Config.HTTPServer.Hostname + ":" + config.BindPort,
	}
}

// Start starts the gRPC server
func (svr *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", svr.bindAddress)
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
		return err
	}
	pbv1.RegisterCloudEventServiceServer(svr.grpcServer, svr)
	return svr.grpcServer.Serve(lis)
}

// Stop stops the gRPC server
func (svr *GRPCServer) Stop() {
	svr.grpcServer.GracefulStop()
}

// Publish implements the Publish method of the CloudEventServiceServer interface
func (svr *GRPCServer) Publish(ctx context.Context, pubReq *pbv1.PublishRequest) (*emptypb.Empty, error) {
	// WARNING: don't use "evt, err := pb.FromProto(pubReq.Event)" to convert protobuf to cloudevent
	evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pubReq.Event))
	if err != nil {
		return nil, fmt.Errorf("failed to convert protobuf to cloudevent: %v", err)
	}

	res, action, err := decode(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	switch action {

	case config.CreateRequestAction:
		_, err := svr.resourceService.Create(ctx, res)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource: %v", err)
		}
	case config.UpdateRequestAction:
		_, err := svr.resourceService.Update(ctx, res)
		if err != nil {
			return nil, fmt.Errorf("failed to update resource: %v", err)
		}
	case config.DeleteRequestAction:
		err := svr.resourceService.MarkAsDeleting(ctx, res.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update resource: %v", err)
		}
		// TODO: resync request
	}

	return &emptypb.Empty{}, nil
}

// Subscribe implements the Subscribe method of the CloudEventServiceServer interface
func (svr *GRPCServer) Subscribe(subReq *pbv1.SubscriptionRequest, subServer pbv1.CloudEventService_SubscribeServer) error {
	clientID, errChan := svr.eventBroadcaster.Register(subReq.Source, func(res *api.Resource) error {
		evt, err := encode(res)
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
			return err
		}

		return nil
	})

	select {
	case err := <-errChan:
		svr.eventBroadcaster.Unregister(clientID)
		return err
	case <-subServer.Context().Done():
		svr.eventBroadcaster.Unregister(clientID)
		return nil
	}
}

func decode(evt *cloudevents.Event) (*api.Resource, types.EventAction, error) {
	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	if eventType.CloudEventsDataType != payload.ManifestEventDataType {
		return nil, "", fmt.Errorf("unsupported cloudevents data type %s", eventType.CloudEventsDataType)
	}

	evtExtensions := evt.Context.GetExtensions()

	clusterName, err := cloudeventstypes.ToString(evtExtensions[types.ExtensionClusterName])
	if err != nil {
		return nil, "", fmt.Errorf("failed to get clustername extension: %v", err)
	}

	manifest := &payload.Manifest{}
	if err := evt.DataAs(manifest); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal event data %s, %v", string(evt.Data()), err)
	}

	resource := &api.Resource{
		Source:     evt.Source(),
		ConsumerID: clusterName,
		// Version:    resourceVersion,
		Manifest: manifest.Manifest.Object,
	}

	if eventType.Action == config.UpdateRequestAction || eventType.Action == config.DeleteRequestAction {
		resourceID, err := cloudeventstypes.ToString(evtExtensions[types.ExtensionResourceID])
		if err != nil {
			return nil, "", fmt.Errorf("failed to get resourceid extension: %v", err)
		}

		resourceVersion, err := cloudeventstypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
		if err != nil {
			return nil, "", fmt.Errorf("failed to get resourceversion extension: %v", err)
		}

		resource.ID = resourceID
		resource.Version = int32(resourceVersion)
	}

	if deletionTimestampValue, exists := evtExtensions[types.ExtensionDeletionTimestamp]; exists {
		deletionTimestamp, err := cloudeventstypes.ToTime(deletionTimestampValue)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert deletion timestamp %v to time.Time: %v", deletionTimestampValue, err)
		}
		resource.Meta.DeletedAt.Time = deletionTimestamp
	}

	return resource, eventType.Action, nil
}

func encode(resource *api.Resource) (*cloudevents.Event, error) {
	eventType := types.CloudEventsType{
		CloudEventsDataType: payload.ManifestEventDataType,
		SubResource:         types.SubResourceStatus,
		Action:              config.UpdateRequestAction,
	}

	evt := types.NewEventBuilder(resource.Source, eventType).
		WithResourceID(resource.ID).
		WithResourceVersion(int64(resource.Version)).
		WithClusterName(resource.ConsumerID).
		NewEvent()

	resourceStatusJSON, err := json.Marshal(resource.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource status, %v", err)
	}
	resourceStatus := &api.ResourceStatus{}
	if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource status, %v", err)
	}

	contentStatusJSON, err := json.Marshal(resourceStatus.ContentStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content status, %v", err)
	}
	contentStatusJSONStr := string(contentStatusJSON)

	statusPayload := &payload.ManifestStatus{
		Conditions: resourceStatus.ReconcileStatus.Conditions,
		Status: &workv1.ManifestCondition{
			Conditions: resourceStatus.ReconcileStatus.Conditions,
			StatusFeedbacks: workv1.StatusFeedbackResult{
				Values: []workv1.FeedbackValue{
					{
						Name: "status",
						Value: workv1.FieldValue{
							Type:    workv1.JsonRaw,
							JsonRaw: &contentStatusJSONStr,
						},
					},
				},
			},
		},
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, statusPayload); err != nil {
		return nil, fmt.Errorf("failed to encode manifestwork status to a cloudevent: %v", err)
	}

	return &evt, nil
}
