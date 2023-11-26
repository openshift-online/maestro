package environments

import (
	"os"

	"github.com/openshift-online/maestro/pkg/db/db_session"
)

var _ EnvironmentImpl = &testingEnvImpl{}

// testingEnvImpl is configuration for local integration tests
type testingEnvImpl struct {
	env *Env
}

func (e *testingEnvImpl) VisitDatabase(c *Database) error {
	c.SessionFactory = db_session.NewTestFactory(e.env.Config.Database)
	return nil
}

func (e *testingEnvImpl) VisitMessageBroker(c *MessageBroker) error {
	return nil
}

func (e *testingEnvImpl) VisitConfig(c *ApplicationConfig) error {
	// Support a one-off env to allow enabling db debug in testing
	if os.Getenv("DB_DEBUG") == "true" {
		c.ApplicationConfig.Database.Debug = true
	}
	return nil
}

func (e *testingEnvImpl) VisitServices(s *Services) error {
	return nil
}

func (e *testingEnvImpl) VisitHandlers(h *Handlers) error {
	return nil
}

func (e *testingEnvImpl) VisitClients(c *Clients) error {
	return nil
}

func (e *testingEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":                    "0",
		"logtostderr":          "true",
		"ocm-base-url":         "https://api.integration.openshift.com",
		"enable-https":         "false",
		"enable-metrics-https": "false",
		"enable-authz":         "true",
		"ocm-debug":            "false",
		"enable-ocm-mock":      "true",
		"enable-sentry":        "false",
		"source-id":            "maestro-integration",
	}
}
