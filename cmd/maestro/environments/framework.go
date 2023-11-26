package environments

import (
	"fmt"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/client/ocm"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/errors"

	mqttoptions "open-cluster-management.io/api/cloudevents/generic/options/mqtt"
)

func init() {
	once.Do(func() {
		environment = &Env{}

		// Create the configuration
		environment.Config = config.NewApplicationConfig()
		environment.ApplicationConfig = ApplicationConfig{config.NewApplicationConfig()}
		environment.Name = GetEnvironmentStrFromEnv()

		environments = map[string]EnvironmentImpl{
			DevelopmentEnv: &devEnvImpl{environment},
			TestingEnv:     &testingEnvImpl{environment},
			ProductionEnv:  &productionEnvImpl{environment},
		}
	})
}

// EnvironmentImpl defines a set of behaviors for an OCM environment.
// Each environment provides a set of flags for basic set/override of the environment.
// Each environment is a set of configured things (services, handlers, clients, etc.) and
// we may expect a stable set of components. Use Visitor pattern to allow external callers (an environment)
// to affect the internal structure of components.
// Each visitor is applied after a component is instantiated with flags set.
// VisitorConfig is applies after instantiation but before ReadFiles is called.
type EnvironmentImpl interface {
	Flags() map[string]string
	VisitConfig(c *ApplicationConfig) error
	VisitDatabase(s *Database) error
	VisitMessageBroker(s *MessageBroker) error
	VisitServices(s *Services) error
	VisitHandlers(c *Handlers) error
	VisitClients(c *Clients) error
}

func GetEnvironmentStrFromEnv() string {
	envStr, specified := os.LookupEnv(EnvironmentStringKey)
	if !specified || envStr == "" {
		envStr = EnvironmentDefault
	}
	return envStr
}

func Environment() *Env {
	return environment
}

// Adds environment flags, using the environment's config struct, to the flagset 'flags'
func (e *Env) AddFlags(flags *pflag.FlagSet) error {
	e.Config.AddFlags(flags)
	return setConfigDefaults(flags, environments[e.Name].Flags())
}

// Initialize loads the environment's resources
// This should be called after the e.Config has been set appropriately though AddFlags and pasing, done elsewhere
// The environment does NOT handle flag parsing
func (e *Env) Initialize() error {
	glog.Infof("Initializing %s environment", e.Name)

	envImpl, found := environments[e.Name]
	if !found {
		glog.Fatalf("Unknown runtime environment: %s", e.Name)
	}

	if err := envImpl.VisitConfig(&e.ApplicationConfig); err != nil {
		glog.Fatalf("Failed to visit ApplicationConfig: %s", err)
	}

	messages := environment.Config.ReadFiles()
	if len(messages) != 0 {
		err := fmt.Errorf("Unable to read configuration files:\n%s", strings.Join(messages, "\n"))
		sentry.CaptureException(err)
		glog.Fatalf("Unable to read configuration files:\n%s", strings.Join(messages, "\n"))
	}

	// each env will set db explicitly because the DB impl has a `once` init section
	if err := envImpl.VisitDatabase(&e.Database); err != nil {
		glog.Fatalf("Failed to visit Database: %s", err)
	}

	if err := envImpl.VisitMessageBroker(&e.MessageBroker); err != nil {
		glog.Fatalf("Failed to visit MessageBroker: %s", err)
	}

	e.LoadServices()
	if err := envImpl.VisitServices(&e.Services); err != nil {
		glog.Fatalf("Failed to visit Services: %s", err)
	}

	// Load clients after services so that clients can use services
	err := e.LoadClients()
	if err != nil {
		return err
	}
	if err := envImpl.VisitClients(&e.Clients); err != nil {
		glog.Fatalf("Failed to visit Clients: %s", err)
	}

	err = e.InitializeSentry()
	if err != nil {
		return err
	}

	seedErr := e.Seed()
	if seedErr != nil {
		return seedErr
	}

	if _, ok := envImpl.(HandlerVisitor); ok {
		if err := (envImpl.(HandlerVisitor)).VisitHandlers(&e.Handlers); err != nil {
			glog.Fatalf("Failed to visit Handlers: %s", err)
		}
	}

	return nil
}

func (e *Env) Seed() *errors.ServiceError {
	return nil
}

func (e *Env) LoadServices() {
	e.Services.Generic = NewGenericServiceLocator(e)
	e.Services.Resources = NewResourceServiceLocator(e)
	e.Services.Events = NewEventServiceLocator(e)
	e.Services.Consumers = NewConsumerServiceLocator(e)
}

func (e *Env) LoadClients() error {
	var err error

	ocmConfig := ocm.Config{
		BaseURL:      e.Config.OCM.BaseURL,
		ClientID:     e.Config.OCM.ClientID,
		ClientSecret: e.Config.OCM.ClientSecret,
		SelfToken:    e.Config.OCM.SelfToken,
		TokenURL:     e.Config.OCM.TokenURL,
		Debug:        e.Config.OCM.Debug,
	}

	// Create OCM Authz client
	if e.Config.OCM.EnableMock {
		glog.Infof("Using Mock OCM Authz Client")
		e.Clients.OCM, err = ocm.NewClientMock(ocmConfig)
	} else {
		e.Clients.OCM, err = ocm.NewClient(ocmConfig)
	}
	if err != nil {
		glog.Errorf("Unable to create OCM Authz client: %s", err.Error())
		return err
	}

	cloudEventsSourceOptions := mqttoptions.NewSourceOptions(e.Config.MessageBroker.MQTTOptions, e.Config.MessageBroker.SourceID)
	e.Clients.CloudEventsSource, err = cloudevents.NewSourceClient(cloudEventsSourceOptions, e.Services.Resources())
	if err != nil {
		glog.Errorf("Unable to create CloudEvents Source client: %s", err.Error())
		return err
	}

	return nil
}

func (e *Env) InitializeSentry() error {
	options := sentry.ClientOptions{}

	if e.Config.Sentry.Enabled {
		key := e.Config.Sentry.Key
		url := e.Config.Sentry.URL
		project := e.Config.Sentry.Project
		glog.Infof("Sentry error reporting enabled to %s on project %s", url, project)
		options.Dsn = fmt.Sprintf("https://%s@%s/%s", key, url, project)
	} else {
		// Setting the DSN to an empty string effectively disables sentry
		// See https://godoc.org/github.com/getsentry/sentry-go#ClientOptions Dsn
		glog.Infof("Disabling Sentry error reporting")
		options.Dsn = ""
	}

	transport := sentry.NewHTTPTransport()
	transport.Timeout = e.Config.Sentry.Timeout
	// since sentry.HTTPTransport is asynchronous, Sentry needs a buffer to cache pending requests.
	// the BufferSize is the size of the buffer. Sentry drops requests when the buffer is full:
	// https://github.com/getsentry/sentry-go/blob/4f72d7725080f61e924409c8ddd008739fd4a837/transport.go#L312
	// errors in our system are relatively sparse, we don't need a large BufferSize.
	transport.BufferSize = 10
	options.Transport = transport
	options.Debug = e.Config.Sentry.Debug
	options.AttachStacktrace = true
	options.Environment = e.Name

	hostname, err := os.Hostname()
	if err != nil && hostname != "" {
		options.ServerName = hostname
	}
	// TODO figure out some way to set options.Release and options.Dist

	err = sentry.Init(options)
	if err != nil {
		glog.Errorf("Unable to initialize sentry integration: %s", err.Error())
		return err
	}
	return nil
}

func (e *Env) Teardown() {
	if e.Name != TestingEnv {
		if err := e.Database.SessionFactory.Close(); err != nil {
			glog.Fatalf("Unable to close db connection: %s", err.Error())
		}
		e.Clients.OCM.Close()
	}
}

func setConfigDefaults(flags *pflag.FlagSet, defaults map[string]string) error {
	for name, value := range defaults {
		if err := flags.Set(name, value); err != nil {
			glog.Errorf("Error setting flag %s: %v", name, err)
			return err
		}
	}
	return nil
}
