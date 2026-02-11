package environments

import (
	"github.com/openshift-online/maestro/pkg/db/db_session"
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
		"v":         "1",
		"source-id": "maestro",
	}
}
