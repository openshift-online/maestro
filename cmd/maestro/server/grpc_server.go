package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/klog/v2"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/client/grpcauthorizer"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/services"
)

// GRPCServer includes a gRPC server and a resource service
type GRPCServer struct {
	pbv1.UnimplementedCloudEventServiceServer
	grpcServer        *grpc.Server
	eventBroadcaster  *event.EventBroadcaster
	resourceService   services.ResourceService
	disableAuthorizer bool
	grpcAuthorizer    grpcauthorizer.GRPCAuthorizer
	bindAddress       string
}

// NewGRPCServer creates a new GRPCServer
func NewGRPCServer(resourceService services.ResourceService, eventBroadcaster *event.EventBroadcaster, config config.GRPCServerConfig, grpcAuthorizer grpcauthorizer.GRPCAuthorizer) *GRPCServer {
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
		if config.TLSCertFile == "" || config.TLSKeyFile == "" {
			check(
				fmt.Errorf("unspecified required --grpc-tls-cert-file, --grpc-tls-key-file"),
				"Can't start gRPC server",
			)
		}

		// Serve with TLS
		serverCerts, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			check(fmt.Errorf("failed to load server certificates: %v", err), "Can't start gRPC server")
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{serverCerts},
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
		}

		// add metrics and auth interceptors
		grpcServerOptions = append(grpcServerOptions,
			grpc.ChainUnaryInterceptor(newMetricsUnaryInterceptor(), newAuthUnaryInterceptor(config.GRPCAuthNType, grpcAuthorizer)),
			grpc.ChainStreamInterceptor(newMetricsStreamInterceptor(), newAuthStreamInterceptor(config.GRPCAuthNType, grpcAuthorizer)))

		if config.GRPCAuthNType == "mtls" {
			if len(config.ClientCAFile) == 0 {
				check(fmt.Errorf("no client CA file specified when using mtls authorization type"), "Can't start gRPC server")
			}

			certPool, err := x509.SystemCertPool()
			if err != nil {
				check(fmt.Errorf("failed to load system cert pool: %v", err), "Can't start gRPC server")
			}

			caPEM, err := os.ReadFile(config.ClientCAFile)
			if err != nil {
				check(fmt.Errorf("failed to read client CA file: %v", err), "Can't start gRPC server")
			}

			if ok := certPool.AppendCertsFromPEM(caPEM); !ok {
				check(fmt.Errorf("failed to append client CA to cert pool"), "Can't start gRPC server")
			}

			tlsConfig.ClientCAs = certPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

			grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
			klog.Infof("Serving gRPC service with mTLS at %s", config.ServerBindPort)
		} else {
			grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
			klog.Infof("Serving gRPC service with TLS at %s", config.ServerBindPort)
		}
	} else {
		// append metrics interceptor
		grpcServerOptions = append(grpcServerOptions,
			grpc.UnaryInterceptor(newMetricsUnaryInterceptor()),
			grpc.StreamInterceptor(newMetricsStreamInterceptor()))
		// Note: Do not use this in production.
		klog.Infof("Serving gRPC service without TLS at %s", config.ServerBindPort)
	}

	return &GRPCServer{
		grpcServer:        grpc.NewServer(grpcServerOptions...),
		eventBroadcaster:  eventBroadcaster,
		resourceService:   resourceService,
		disableAuthorizer: config.DisableTLS,
		grpcAuthorizer:    grpcAuthorizer,
		bindAddress:       env().Config.HTTPServer.Hostname + ":" + config.ServerBindPort,
	}
}

// Start starts the gRPC server
func (svr *GRPCServer) Start() error {
	klog.Info("Starting gRPC server")
	lis, err := net.Listen("tcp", svr.bindAddress)
	if err != nil {
		klog.Errorf("failed to listen: %v", err)
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

	if !svr.disableAuthorizer {
		// check if the event is from the authorized source
		user := ctx.Value(contextUserKey).(string)
		groups := ctx.Value(contextGroupsKey).([]string)
		allowed, err := svr.grpcAuthorizer.AccessReview(ctx, "pub", "source", evt.Source(), user, groups)
		if err != nil {
			return nil, fmt.Errorf("failed to authorize the request: %v", err)
		}
		if !allowed {
			return nil, fmt.Errorf("unauthorized to publish the event from source %s", evt.Source())
		}
	}

	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	klog.V(4).Infof("receive the event with grpc server, %s", evt)

	// handler resync request
	if eventType.Action == types.ResyncRequestAction {
		err := svr.respondResyncStatusRequest(ctx, evt)
		if err != nil {
			return nil, fmt.Errorf("failed to respond resync status request: %v", err)
		}
		return &emptypb.Empty{}, nil
	}

	res, err := decodeResourceSpec(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent: %v", err)
	}

	switch eventType.Action {
	case common.CreateRequestAction:
		_, err := svr.resourceService.Create(ctx, res)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource: %v", err)
		}
	case common.UpdateRequestAction:
		found, err := svr.resourceService.Get(ctx, res.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get resource: %v", err)
		}

		if res.Version == 0 {
			// the resource version is not guaranteed to be increased by source client,
			// using the latest resource version.
			res.Version = found.Version
		}
		if _, err = svr.resourceService.Update(ctx, res); err != nil {
			return nil, fmt.Errorf("failed to update resource: %v", err)
		}
	case common.DeleteRequestAction:
		err := svr.resourceService.MarkAsDeleting(ctx, res.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete resource: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported action %s", eventType.Action)
	}

	return &emptypb.Empty{}, nil
}

// Subscribe implements the Subscribe method of the CloudEventServiceServer interface
func (svr *GRPCServer) Subscribe(subReq *pbv1.SubscriptionRequest, subServer pbv1.CloudEventService_SubscribeServer) error {
	if !svr.disableAuthorizer {
		// check if the client is authorized to subscribe the event from the source
		ctx := subServer.Context()
		user := ctx.Value(contextUserKey).(string)
		groups := ctx.Value(contextGroupsKey).([]string)
		allowed, err := svr.grpcAuthorizer.AccessReview(ctx, "sub", "source", subReq.Source, user, groups)
		if err != nil {
			return fmt.Errorf("failed to authorize the request: %v", err)
		}
		if !allowed {
			return fmt.Errorf("unauthorized to subscribe the event from source %s", subReq.Source)
		}
	}

	clientID, errChan := svr.eventBroadcaster.Register(subReq.Source, func(res *api.Resource) error {
		evt, err := encodeResourceStatus(res)
		if err != nil {
			return fmt.Errorf("failed to encode resource %s to cloudevent: %v", res.ID, err)
		}

		klog.V(4).Infof("send the event to status subscribers, %s", evt)

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
		klog.Infof("unregistering client %s due to error= %v", clientID, err)
		svr.eventBroadcaster.Unregister(clientID)
		return err
	case <-subServer.Context().Done():
		svr.eventBroadcaster.Unregister(clientID)
		return nil
	}
}

// decodeResourceSpec translates a CloudEvent into a resource containing the spec JSON map.
func decodeResourceSpec(evt *ce.Event) (*api.Resource, error) {
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

	resource := &api.Resource{
		Source:       evt.Source(),
		ConsumerName: clusterName,
		Version:      resourceVersion,
		Meta: api.Meta{
			ID: resourceID,
		},
	}

	if deletionTimestampValue, exists := evtExtensions[types.ExtensionDeletionTimestamp]; exists {
		deletionTimestamp, err := cetypes.ToTime(deletionTimestampValue)
		if err != nil {
			return nil, fmt.Errorf("failed to convert deletion timestamp %v to time.Time: %v", deletionTimestampValue, err)
		}
		resource.Meta.DeletedAt.Time = deletionTimestamp
	}

	payload, err := api.CloudEventToJSONMap(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource payload: %v", err)
	}
	resource.Payload = payload
	// set the resource type to bundle from grpc source
	resource.Type = api.ResourceTypeBundle

	return resource, nil
}

// encodeResourceStatus translates a resource status JSON map into a CloudEvent.
func encodeResourceStatus(resource *api.Resource) (*ce.Event, error) {
	statusEvt, err := api.JSONMAPToCloudEvent(resource.Status)
	if err != nil {
		return nil, err
	}

	// manifest bundle status from the resource status
	manifestBundleStatus := &workpayload.ManifestBundleStatus{}
	if err := statusEvt.DataAs(manifestBundleStatus); err != nil {
		return nil, err
	}

	if len(resource.Payload) > 0 {
		specEvt, err := api.JSONMAPToCloudEvent(resource.Payload)
		if err != nil {
			return nil, err
		}

		// set work spec back from spec event
		manifestBundle := &workpayload.ManifestBundle{}
		if err := specEvt.DataAs(manifestBundle); err != nil {
			return nil, err
		}
		manifestBundleStatus.ManifestBundle = manifestBundle
	}

	if err := statusEvt.SetData(ce.ApplicationJSON, manifestBundleStatus); err != nil {
		return nil, err
	}

	return statusEvt, nil
}

// respondResyncStatusRequest responds to the status resync request by comparing the status hash of the resources
// from the database and the status hash in the request, and then respond the resources whose status is changed.
func (svr *GRPCServer) respondResyncStatusRequest(ctx context.Context, evt *ce.Event) error {
	objs, serviceErr := svr.resourceService.FindBySource(ctx, evt.Source())
	if serviceErr != nil {
		return fmt.Errorf("failed to list resources: %s", serviceErr)
	}

	statusHashes, err := payload.DecodeStatusResyncRequest(*evt)
	if err != nil {
		return fmt.Errorf("failed to decode status resync request: %v", err)
	}

	if len(statusHashes.Hashes) == 0 {
		// publish all resources status
		for _, obj := range objs {
			svr.eventBroadcaster.Broadcast(obj)
		}

		return nil
	}

	for _, obj := range objs {
		lastHash, ok := findStatusHash(string(obj.GetUID()), statusHashes.Hashes)
		if !ok {
			// ignore the resource that is not on the source, but exists on the maestro, wait for the source deleting it
			klog.Infof("The resource %s is not found from the maestro, ignore", obj.GetUID())
			continue
		}

		currentHash, err := cloudevents.ResourceStatusHashGetter(obj)
		if err != nil {
			continue
		}

		if currentHash == lastHash {
			// the status is not changed, do nothing
			continue
		}

		svr.eventBroadcaster.Broadcast(obj)
	}

	return nil
}

// findStatusHash finds the status hash of the resource from the status resync request payload
func findStatusHash(id string, hashes []payload.ResourceStatusHash) (string, bool) {
	for _, hash := range hashes {
		if id == hash.ResourceID {
			return hash.StatusHash, true
		}
	}

	return "", false
}
