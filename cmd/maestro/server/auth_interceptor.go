package server

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/client/grpcauthorizer"
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

// newAuthUnaryInterceptor creates a unary interceptor that retrieves the user and groups
// based on the specified authentication type. It supports retrieving from either the access
// token or the client certificate depending on the provided authNType.
// The interceptor then adds the retrieved identity information (user and groups) to the
// context and invokes the provided handler.
func newAuthUnaryInterceptor(authNType string, authorizer grpcauthorizer.GRPCAuthorizer) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var user string
		var groups []string
		var err error

		logger := klog.FromContext(ctx)
		switch authNType {
		case "token":
			user, groups, err = identityFromToken(ctx, authorizer)
			if err != nil {
				logger.Error(err, "unable to get user and groups from token")
				return nil, err
			}
		case "mtls":
			user, groups, err = identityFromCertificate(ctx)
			if err != nil {
				logger.Error(err, "unable to get user and groups from certificate")
				return nil, err
			}
		case "mock":
			user = "mock"
			groups = []string{"mock-group"}
		default:
			return nil, fmt.Errorf("unsupported authentication type %s", authNType)
		}

		// call the handler with the new context containing the user and groups
		return handler(newContextWithIdentity(ctx, user, groups), req)
	}
}

// wrappedAuthStream wraps a grpc.ServerStream associated with an incoming RPC, and
// a custom context containing the user and groups derived from the client certificate
// specified in the incoming RPC metadata
type wrappedAuthStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the context associated with the stream
func (w *wrappedAuthStream) Context() context.Context {
	return w.ctx
}

// newWrappedAuthStream creates a new wrappedAuthStream
func newWrappedAuthStream(ctx context.Context, s grpc.ServerStream) grpc.ServerStream {
	return &wrappedAuthStream{s, ctx}
}

// newAuthStreamInterceptor creates a stream interceptor that retrieves the user and groups
// based on the specified authentication type. It supports retrieving from either the access
// token or the client certificate depending on the provided authNType.
// The interceptor then adds the retrieved identity information (user and groups) to the
// context and invokes the provided handler.
func newAuthStreamInterceptor(authNType string, authorizer grpcauthorizer.GRPCAuthorizer) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		var user string
		var groups []string
		var err error

		logger := klog.FromContext(ss.Context())
		switch authNType {
		case "token":
			user, groups, err = identityFromToken(ss.Context(), authorizer)
			if err != nil {
				logger.Error(err, "unable to get user and groups from token")
				return err
			}
		case "mtls":
			user, groups, err = identityFromCertificate(ss.Context())
			if err != nil {
				logger.Error(err, "unable to get user and groups from certificate")
				return err
			}
		case "mock":
			user = "mock"
			groups = []string{"mock-group"}
		default:
			return fmt.Errorf("unsupported authentication Type %s", authNType)
		}

		return handler(srv, newWrappedAuthStream(newContextWithIdentity(ss.Context(), user, groups), ss))
	}
}
