package environments

import (
	"github.com/openshift-online/maestro/pkg/db/db_session"
)

// devEnvImpl environment is intended for local use while developing features
type devEnvImpl struct {
	env *Env
}

var _ EnvironmentImpl = &devEnvImpl{}

func (e *devEnvImpl) VisitDatabase(c *Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.env.Config.Database)
	return nil
}

func (e *devEnvImpl) VisitMessageBroker(c *MessageBroker) error {
	return nil
}

func (e *devEnvImpl) VisitConfig(c *ApplicationConfig) error {
	c.ApplicationConfig.HTTPServer.EnableJWT = false
	c.ApplicationConfig.HTTPServer.EnableHTTPS = false
	return nil
}

func (e *devEnvImpl) VisitServices(s *Services) error {
	return nil
}

func (e *devEnvImpl) VisitHandlers(h *Handlers) error {
	return nil
}

func (e *devEnvImpl) VisitClients(c *Clients) error {
	return nil
}

func (e *devEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":                      "10",
		"enable-authz":           "false",
		"ocm-debug":              "false",
		"enable-ocm-mock":        "true",
		"enable-https":           "false",
		"enable-metrics-https":   "false",
		"api-server-hostname":    "localhost",
		"api-server-bindaddress": "localhost:8000",
		"enable-sentry":          "false",
		"source-id":              "maestro-development",
	}
}
