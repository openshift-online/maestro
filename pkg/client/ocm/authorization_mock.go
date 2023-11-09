package ocm

import (
	"context"
)

// authorizationMock returns allowed=true for every request
type authorizationMock service

var _ OCMAuthorization = &authorizationMock{}

func (a authorizationMock) SelfAccessReview(ctx context.Context, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	return true, nil
}

func (a authorizationMock) AccessReview(ctx context.Context, username, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	return true, nil
}
