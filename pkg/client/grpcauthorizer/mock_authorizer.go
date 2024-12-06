package grpcauthorizer

import "context"

// MockGRPCAuthorizer returns allowed=true for every request
type MockGRPCAuthorizer struct {
}

func NewMockGRPCAuthorizer() GRPCAuthorizer {
	return &MockGRPCAuthorizer{}
}

var _ GRPCAuthorizer = &MockGRPCAuthorizer{}

// TokenReview returns an empty user and groups
func (m *MockGRPCAuthorizer) TokenReview(ctx context.Context, token string) (user string, groups []string, err error) {
	return "", []string{}, nil
}

// SelfAccessReview returns allowed=true for every request
func (m *MockGRPCAuthorizer) AccessReview(ctx context.Context, action, resourceType, resource, user string, groups []string) (allowed bool, err error) {
	return true, nil
}
