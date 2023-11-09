package test

import (
	"testing"

	gm "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// Register a test
// This should be run before every integration test
func RegisterIntegration(t *testing.T) (*Helper, *openapi.APIClient) {
	// Register the test with gomega
	gm.RegisterTestingT(t)
	// Create a new helper
	helper := NewHelper(t)
	// Reset the database to a seeded blank state
	helper.DBFactory.ResetDB()
	// Create an api client
	client := helper.NewApiClient()

	return helper, client
}
