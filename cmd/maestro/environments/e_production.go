package environments

import (
	"github.com/openshift-online/maestro/pkg/db/db_session"
	mqttoptions "open-cluster-management.io/api/cloudevents/generic/options/mqtt"
)

var _ EnvironmentImpl = &productionEnvImpl{}

// productionEnvImpl is any deployed instance of the service through app-interface
type productionEnvImpl struct {
	env *Env
}

var _ EnvironmentImpl = &productionEnvImpl{}

func (e *productionEnvImpl) VisitDatabase(c *Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.env.Config.Database)
	return nil
}

func (e *productionEnvImpl) VisitMessageBroker(c *MessageBroker) error {
	c.CloudEventsSourceOptions = mqttoptions.NewSourceOptions(e.env.Config.MessageBroker.MQTTOptions, e.env.Config.MessageBroker.SourceID)
	return nil
}

func (e *productionEnvImpl) VisitConfig(c *ApplicationConfig) error {
	return nil
}

func (e *productionEnvImpl) VisitServices(s *Services) error {
	return nil
}

func (e *productionEnvImpl) VisitHandlers(h *Handlers) error {
	return nil
}

func (e *productionEnvImpl) VisitClients(c *Clients) error {
	return nil
}

func (e *productionEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":               "1",
		"ocm-debug":       "false",
		"enable-ocm-mock": "false",
		"enable-sentry":   "true",
		"source-id":       "maestro",
	}
}
