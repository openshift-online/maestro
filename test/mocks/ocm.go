package mocks

import (
	"context"

	"github.com/openshift-online/maestro/pkg/client/ocm"
)

/*
The OCM Validator Mock will simply return true to all access_review requests instead
of reaching out to the AMS system or using the built-in OCM mock. It will record
the action and resourceType sent to it in the struct itself. This can be used
to validate that the expected action/resourceType for a particular endpoint was
determined in the authorization middleware

Use:
	h, client := test.RegisterIntegration(t)
	authzMock, ocmMock := mocks.NewOCMAuthzValidatorMockClient()
	// Use the OCM client mock, re-load services so they pick up the mock
	h.Env().Clients.OCM = ocmMock
	// The built-in mock has to be disabled or the server will use it instead
	h.Env().Config.OCM.EnableMock = false
	// Services and the server should be re-loaded to pick up the client with this mock
	h.Env().LoadServices()
	h.RestartServer()

	// Make a request, then validate the action and resourceType
	Expect(authzMock.Action).To(Equal("get"))
	Expect(authzMock.ResourceType).To(Equal("JQSJobQueue"))
	authzMock.Reset()
*/

var _ ocm.OCMAuthorization = &OCMAuthzValidatorMock{}

type OCMAuthzValidatorMock struct {
	Action       string
	ResourceType string
}

func NewOCMAuthzValidatorMockClient() (*OCMAuthzValidatorMock, *ocm.Client) {
	authz := &OCMAuthzValidatorMock{
		Action:       "",
		ResourceType: "",
	}
	client := &ocm.Client{}
	client.Authorization = authz
	return authz, client
}

func (m *OCMAuthzValidatorMock) SelfAccessReview(ctx context.Context, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m *OCMAuthzValidatorMock) AccessReview(ctx context.Context, username, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m OCMAuthzValidatorMock) Reset() {
	m.Action = ""
	m.ResourceType = ""
}
