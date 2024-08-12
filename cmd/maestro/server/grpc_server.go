package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/klog/v2"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/client/grpcauthorizer"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/services"
)

// Context key type defined to avoid collisions in other pkgs using context
// See https://golang.org/pkg/context/#WithValue
type contextKey string

const (
	contextUserKey   contextKey = "user"
	contextGroupsKey contextKey = "groups"
)

func newContextWithIdentity(ctx context.Context, user string, groups []string) context.Context {
	ctx = context.WithValue(ctx, contextUserKey, user)
	return context.WithValue(ctx, contextGroupsKey, groups)
}

// identityFromCertificate retrieves the user and groups from the client certificate if they are present.
func identityFromCertificate(ctx context.Context) (string, []string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", nil, status.Error(codes.Unauthenticated, "no peer found")
	}

	tlsAuth, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", nil, status.Error(codes.Unauthenticated, "unexpected peer transport credentials")
	}

	if len(tlsAuth.State.VerifiedChains) == 0 || len(tlsAuth.State.VerifiedChains[0]) == 0 {
		return "", nil, status.Error(codes.Unauthenticated, "could not verify peer certificate")
	}

	if tlsAuth.State.VerifiedChains[0][0] == nil {
		return "", nil, status.Error(codes.Unauthenticated, "could not verify peer certificate")
	}

	user := tlsAuth.State.VerifiedChains[0][0].Subject.CommonName
	groups := tlsAuth.State.VerifiedChains[0][0].Subject.Organization

	if user == "" {
		return "", nil, status.Error(codes.Unauthenticated, "could not find user in peer certificate")
	}

	if len(groups) == 0 {
		return "", nil, status.Error(codes.Unauthenticated, "could not find group in peer certificate")
	}

	return user, groups, nil
}

// identityFromToken retrieves the user and groups from the access token if they are present.
func identityFromToken(ctx context.Context, grpcAuthorizer grpcauthorizer.GRPCAuthorizer) (string, []string, error) {
	// Extract the metadata from the context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", nil, status.Error(codes.InvalidArgument, "missing metadata")
	}

	// Extract the access token from the metadata
	authorization, ok := md["authorization"]
	if !ok || len(authorization) == 0 {
		return "", nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	token := strings.TrimPrefix(authorization[0], "Bearer ")
	// Extract the user and groups from the access token
	return grpcAuthorizer.TokenReview(ctx, token)
}

// newAuthUnaryInterceptor creates a new unary interceptor that looks up the client certificate from the incoming RPC context,
// retrieves the user and groups from it and creates a new context with the user and groups before invoking the provided handler.
// otherwise, it falls back retrieving the user and groups from the access token.
func newAuthUnaryInterceptor(authorizer grpcauthorizer.GRPCAuthorizer) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		user, groups, err := identityFromToken(ctx, authorizer)
		if err != nil {
			glog.Warningf("unable to get user and groups from token: %v, fall back to certificate", err)
			user, groups, err = identityFromCertificate(ctx)
			if err != nil {
				glog.Errorf("unable to get user and groups from certificate: %v", err)
				return nil, err
			}
		}

		return handler(newContextWithIdentity(ctx, user, groups), req)
	}
}

// wrappedStream wraps a grpc.ServerStream associated with an incoming RPC, and
// a custom context containing the user and groups derived from the client certificate
// specified in the incoming RPC metadata
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func newWrappedStream(ctx context.Context, s grpc.ServerStream) grpc.ServerStream {
	return &wrappedStream{s, ctx}
}

// newAuthStreamInterceptor creates a new stream interceptor that looks up the client certificate from the incoming RPC context,
// retrieves the user and groups from it and creates a new context with the user and groups before invoking the provided handler.
// otherwise, it falls back retrieving the user and groups from the access token.
func newAuthStreamInterceptor(authorizer grpcauthorizer.GRPCAuthorizer) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		user, groups, err := identityFromToken(ss.Context(), authorizer)
		if err != nil {
			glog.Warningf("unable to get user and groups from token: %v, fall back to certificate", err)
			user, groups, err = identityFromCertificate(ss.Context())
			if err != nil {
				glog.Errorf("unable to get user and groups from certificate: %v", err)
				return err
			}
		}

		return handler(srv, newWrappedStream(newContextWithIdentity(ss.Context(), user, groups), ss))
	}
}

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
	grpcServerOptions = append(grpcServerOptions, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionAge: config.MaxConnectionAge,
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

			grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.UnaryInterceptor(newAuthUnaryInterceptor(grpcAuthorizer)), grpc.StreamInterceptor(newAuthStreamInterceptor(grpcAuthorizer)))
			glog.Infof("Serving gRPC service with mTLS at %s", config.ServerBindPort)
		} else {
			grpcServerOptions = append(grpcServerOptions, grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.UnaryInterceptor(newAuthUnaryInterceptor(grpcAuthorizer)), grpc.StreamInterceptor(newAuthStreamInterceptor(grpcAuthorizer)))
			glog.Infof("Serving gRPC service with TLS at %s", config.ServerBindPort)
		}
	} else {
		// Note: Do not use this in production.
		glog.Infof("Serving gRPC service without TLS at %s", config.ServerBindPort)
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
	glog.Info("Starting gRPC server")
	lis, err := net.Listen("tcp", svr.bindAddress)
	if err != nil {
		glog.Errorf("failed to listen: %v", err)
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

	glog.V(4).Infof("receive the event with grpc server, %s", evt)

	// handler resync request
	if eventType.Action == types.ResyncRequestAction {
		err := svr.respondResyncStatusRequest(ctx, eventType.CloudEventsDataType, evt)
		if err != nil {
			return nil, fmt.Errorf("failed to respond resync status request: %v", err)
		}
		return &emptypb.Empty{}, nil
	}

	res, err := decodeResourceSpec(eventType.CloudEventsDataType, evt)
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
		if res.Type == api.ResourceTypeBundle {
			found, err := svr.resourceService.Get(ctx, res.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get resource: %v", err)
			}

			if res.Version == 0 {
				// the resource version is not guaranteed to be increased by source client,
				// using the latest resource version.
				res.Version = found.Version
			}
		}
		_, err := svr.resourceService.Update(ctx, res)
		if err != nil {
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

		glog.V(4).Infof("send the event to status subscribers, %s", evt)

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
		svr.eventBroadcaster.Unregister(clientID)
		return err
	case <-subServer.Context().Done():
		glog.V(10).Infof("unregister client %s", clientID)
		svr.eventBroadcaster.Unregister(clientID)
		return nil
	}
}

// decodeResourceSpec translates a CloudEvent into a resource containing the spec JSON map.
func decodeResourceSpec(eventDataType types.CloudEventsDataType, evt *ce.Event) (*api.Resource, error) {
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

// encodeResourceStatus translates a resource status JSON map into a CloudEvent.
func encodeResourceStatus(resource *api.Resource) (*ce.Event, error) {
	if resource.Type == api.ResourceTypeSingle {
		// single resource, return the status directly
		return api.JSONMAPToCloudEvent(resource.Status)
	}

	statusEvt, err := api.JSONMAPToCloudEvent(resource.Status)
	if err != nil {
		return nil, err
	}

	// set basic fields
	evt := ce.NewEvent()
	evt.SetID(uuid.New().String())
	evt.SetTime(time.Now())
	evt.SetType(statusEvt.Type())
	evt.SetSource(statusEvt.Source())
	for key, val := range statusEvt.Extensions() {
		evt.SetExtension(key, val)
	}

	// set work meta back from status event
	if workMeta, ok := statusEvt.Extensions()[codec.ExtensionWorkMeta]; ok {
		evt.SetExtension(codec.ExtensionWorkMeta, workMeta)
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

	if err := evt.SetData(ce.ApplicationJSON, manifestBundleStatus); err != nil {
		return nil, err
	}

	return &evt, nil
}

// respondResyncStatusRequest responds to the status resync request by comparing the status hash of the resources
// from the database and the status hash in the request, and then respond the resources whose status is changed.
func (svr *GRPCServer) respondResyncStatusRequest(ctx context.Context, eventDataType types.CloudEventsDataType, evt *ce.Event) error {
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

	resyncType := api.ResourceTypeSingle
	if eventDataType == workpayload.ManifestBundleEventDataType {
		resyncType = api.ResourceTypeBundle
	}

	for _, obj := range objs {
		if obj.Type != resyncType {
			continue
		}

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
