package environments

import (
	"sync"

	"github.com/openshift-online/maestro/pkg/auth"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/client/grpcauthorizer"
	"github.com/openshift-online/maestro/pkg/client/ocm"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
)

const (
	TestingEnv     string = "testing"
	DevelopmentEnv string = "development"
	ProductionEnv  string = "production"

	EnvironmentStringKey string = "MAESTRO_ENV"
	EnvironmentDefault   string = DevelopmentEnv
)

type Env struct {
	Name          string
	Services      Services
	Handlers      Handlers
	Clients       Clients
	Database      Database
	MessageBroker MessageBroker
	// packaging requires this construct for visiting
	ApplicationConfig ApplicationConfig
	// most code relies on env.Config
	Config *config.ApplicationConfig
}

type ApplicationConfig struct {
	ApplicationConfig *config.ApplicationConfig
}

type Database struct {
	SessionFactory db.SessionFactory
}

type MessageBroker struct {
}

type Handlers struct {
	AuthMiddleware auth.JWTMiddleware
}

type Services struct {
	Resources    ResourceServiceLocator
	Generic      GenericServiceLocator
	Events       EventServiceLocator
	StatusEvents StatusEventServiceLocator
	Consumers    ConsumerServiceLocator
}

type Clients struct {
	OCM               *ocm.Client
	GRPCAuthorizer    grpcauthorizer.GRPCAuthorizer
	CloudEventsSource cloudevents.SourceClient
}

type ConfigDefaults struct {
	Server   map[string]interface{}
	Metrics  map[string]interface{}
	Database map[string]interface{}
	OCM      map[string]interface{}
	Options  map[string]interface{}
}

var environment *Env
var once sync.Once
var environments map[string]EnvironmentImpl

// ApplicationConfig visitor
var _ ConfigVisitable = &ApplicationConfig{}

type ConfigVisitable interface {
	Accept(v ConfigVisitor) error
}

type ConfigVisitor interface {
	VisitConfig(c *ApplicationConfig) error
}

func (c *ApplicationConfig) Accept(v ConfigVisitor) error {
	return v.VisitConfig(c)
}

// Database visitor
var _ DatabaseVisitable = &Database{}

type DatabaseVisitable interface {
	Accept(v DatabaseVisitor) error
}

type DatabaseVisitor interface {
	VisitDatabase(s *Database) error
}

func (d *Database) Accept(v DatabaseVisitor) error {
	return v.VisitDatabase(d)
}

// Message Broker visitor
var _ MessageBrokerVisitable = &MessageBroker{}

type MessageBrokerVisitable interface {
	Accept(v MessageBrokerVisitor) error
}

type MessageBrokerVisitor interface {
	VisitMessageBroker(s *MessageBroker) error
}

func (m *MessageBroker) Accept(v MessageBrokerVisitor) error {
	return v.VisitMessageBroker(m)
}

// Services visitor
var _ ServiceVisitable = &Services{}

type ServiceVisitable interface {
	Accept(v ServiceVisitor) error
}

type ServiceVisitor interface {
	VisitServices(s *Services) error
}

func (s *Services) Accept(v ServiceVisitor) error {
	return v.VisitServices(s)
}

// Handlers visitor
var _ HandlerVisitable = &Handlers{}

type HandlerVisitor interface {
	VisitHandlers(c *Handlers) error
}

type HandlerVisitable interface {
	Accept(v HandlerVisitor) error
}

func (c *Handlers) Accept(v HandlerVisitor) error {
	return v.VisitHandlers(c)
}

// Clients visitor
var _ ClientVisitable = &Clients{}

type ClientVisitor interface {
	VisitClients(c *Clients) error
}

type ClientVisitable interface {
	Accept(v ClientVisitor) error
}

func (c *Clients) Accept(v ClientVisitor) error {
	return v.VisitClients(c)
}
