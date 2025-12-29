package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/ghodss/yaml"
	_ "github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
	gorillahandlers "github.com/gorilla/handlers"
	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift-online/ocm-sdk-go/authentication"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/common"
	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/data/generated/openapi"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/event"
)

type apiServer struct {
	httpServer *http.Server
	grpcServer *GRPCServer
}

var _ Server = &apiServer{}

func env() *environments.Env {
	return environments.Environment()
}

func NewAPIServer(ctx context.Context, eventBroadcaster *event.EventBroadcaster) Server {
	s := &apiServer{}

	mainRouter := s.routes(ctx)

	// Sentryhttp middleware performs two operations:
	// 1) Attaches an instance of *sentry.Hub to the request’s context. Accessit by using the sentry.GetHubFromContext() method on the request
	//   NOTE this is the only way middleware, handlers, and services should be reporting to sentry, through the hub
	// 2) Reports panics to the configured sentry service
	if env().Config.Sentry.Enabled {
		sentryhttpOptions := sentryhttp.Options{
			Repanic:         true,
			WaitForDelivery: false,
			Timeout:         env().Config.Sentry.Timeout,
		}
		sentryMW := sentryhttp.New(sentryhttpOptions)
		mainRouter.Use(sentryMW.Handle)
	}

	// referring to the router as type http.Handler allows us to add middleware via more handlers
	var mainHandler http.Handler = mainRouter

	if env().Config.HTTPServer.EnableJWT {
		// Create the logger for the authentication handler:
		authnLogger, err := sdk.NewGlogLoggerBuilder().
			InfoV(glog.Level(1)).
			DebugV(glog.Level(5)).
			Build()
		check(ctx, err, "Unable to create authentication logger")

		// Create the handler that verifies that tokens are valid:
		mainHandler, err = authentication.NewHandler().
			Logger(authnLogger).
			KeysFile(env().Config.HTTPServer.JwkCertFile).
			KeysURL(env().Config.HTTPServer.JwkCertURL).
			ACLFile(env().Config.HTTPServer.ACLFile).
			Public("^/api/maestro/?$").
			Public("^/api/maestro/v1/?$").
			Public("^/api/maestro/v1/openapi/?$").
			Public("^/api/maestro/v1/errors(/.*)?$").
			Next(mainHandler).
			Build()
		check(ctx, err, "Unable to create authentication handler")
	}

	// TODO: remove all cloud.redhat.com once migration to console.redhat.com is complete
	// refer to: https://issues.redhat.com/browse/RHCLOUD-14695
	mainHandler = gorillahandlers.CORS(
		gorillahandlers.AllowedOrigins([]string{
			// OCM UI local development URLs
			"https://qa.foo.redhat.com:1337",
			"https://prod.foo.redhat.com:1337",
			"https://ci.foo.redhat.com:1337",
			"https://cloud.redhat.com",   // TODO: remove
			"https://console.redhat.com", // Production / candidate
			// Staging and test environments
			"https://qaprodauth.cloud.redhat.com", // TODO: remove
			"https://qa.cloud.redhat.com",         // TODO: remove
			"https://ci.cloud.redhat.com",         // TODO: remove
			"https://qaprodauth.console.redhat.com",
			"https://qa.console.redhat.com",
			"https://ci.console.redhat.com",
			"https://console.stage.redhat.com",
			// API docs UI
			"https://api.stage.openshift.com",
			"https://api.openshift.com",
			// Customer portal
			"https://access.qa.redhat.com",
			"https://access.stage.redhat.com",
			"https://access.redhat.com",
		}),
		gorillahandlers.AllowedMethods([]string{
			http.MethodDelete,
			http.MethodGet,
			http.MethodPatch,
			http.MethodPost,
		}),
		gorillahandlers.AllowedHeaders([]string{
			"Authorization",
			"Content-Type",
		}),
		gorillahandlers.MaxAge(int((10 * time.Minute).Seconds())),
	)(mainHandler)

	mainHandler = removeTrailingSlash(mainHandler)
	if common.TracingEnabled() {
		mainHandler = otelhttp.NewHandler(mainHandler, "maestro-api",
			otelhttp.WithSpanNameFormatter(
				func(operation string, r *http.Request) string {
					return fmt.Sprintf("%s %s %s", operation, "HTTP", r.Method)
				},
			),
		)
	}

	s.httpServer = &http.Server{
		Addr:    env().Config.HTTPServer.Hostname + ":" + env().Config.HTTPServer.BindPort,
		Handler: mainHandler,
	}

	if env().Config.GRPCServer.EnableGRPCServer {
		s.grpcServer = NewGRPCServer(ctx, env().Services.Resources(), eventBroadcaster, *env().Config.GRPCServer, env().Clients.GRPCAuthorizer)
	}
	return s
}

// Serve start the blocking call to Serve.
// Useful for breaking up ListenAndServer (Start) when you require the server to be listening before continuing
func (s apiServer) Serve(ctx context.Context, listener net.Listener) {
	logger := klog.FromContext(ctx)
	var err error
	if env().Config.HTTPServer.EnableHTTPS {
		// Check https cert and key path path
		if env().Config.HTTPServer.HTTPSCertFile == "" || env().Config.HTTPServer.HTTPSKeyFile == "" {
			check(ctx,
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
			)
		}

		// Serve with TLS
		logger.Info("Serving with TLS", "port", env().Config.HTTPServer.BindPort)
		err = s.httpServer.ServeTLS(listener, env().Config.HTTPServer.HTTPSCertFile, env().Config.HTTPServer.HTTPSKeyFile)
	} else {
		logger.Info("Serving without TLS", "port", env().Config.HTTPServer.BindPort)
		err = s.httpServer.Serve(listener)
	}

	// Web server terminated.
	check(ctx, err, "Web server terminated with errors")
	logger.Info("Web server terminated")
}

// Listen only start the listener, not the server.
// Useful for breaking up ListenAndServer (Start) when you require the server to be listening before continuing
func (s apiServer) Listen() (listener net.Listener, err error) {
	return net.Listen("tcp", env().Config.HTTPServer.Hostname+":"+env().Config.HTTPServer.BindPort)
}

// Start listening on the configured port and start the server. This is a convenience wrapper for Listen() and Serve(listener Listener)
func (s apiServer) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	if env().Config.GRPCServer.EnableGRPCServer {
		// start the grpc server
		defer s.grpcServer.Stop()
		go s.grpcServer.Start(ctx)
	}

	listener, err := s.Listen()
	if err != nil {
		logger.Error(err, "Unable to start API server")
		return
	}
	s.Serve(ctx, listener)

	// after the server exits but before the application terminates
	// we need to explicitly close Go's sql connection pool.
	// this needs to be called *exactly* once during an app's lifetime.
	env().Database.SessionFactory.Close()
}

func (s apiServer) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}

func (s *apiServer) loadOpenAPISpec(asset string) (data []byte, err error) {
	data, err = openapi.Asset(asset)
	if err != nil {
		err = errors.GeneralError(
			"can't load OpenAPI specification from asset '%s'",
			asset,
		)
		return
	}
	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		err = errors.GeneralError(
			"can't convert OpenAPI specification loaded from asset '%s' from YAML to JSON",
			asset,
		)
		return
	}
	return
}
