package auth

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"

	"github.com/openshift-online/maestro/pkg/errors"
)

type JWTMiddleware interface {
	AuthenticateAccountJWT(next http.Handler) http.Handler
}

type AuthMiddleware struct{}

var _ JWTMiddleware = &AuthMiddleware{}

func NewAuthMiddleware() (*AuthMiddleware, error) {
	middleware := AuthMiddleware{}
	return &middleware, nil
}

// Middleware handler to validate JWT tokens and authenticate users
func (a *AuthMiddleware) AuthenticateAccountJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		payload, err := GetAuthPayload(r)
		if err != nil {
			handleError(ctx, w, errors.ErrorUnauthorized, fmt.Sprintf("Unable to get payload details from JWT token: %s", err))
			return
		}

		// Append the username to the request context
		ctx = SetUsernameContext(ctx, payload.Username)
		*r = *r.WithContext(ctx)

		// Add username to sentry context
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetUser(sentry.User{ID: payload.Username})
			})
		}
		next.ServeHTTP(w, r)
	})
}
