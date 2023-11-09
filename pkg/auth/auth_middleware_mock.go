package auth

import (
	"net/http"
)

type AuthMiddlewareMock struct{}

var _ JWTMiddleware = &AuthMiddlewareMock{}

func (a *AuthMiddlewareMock) AuthenticateAccountJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO need to append a username to the request context
		next.ServeHTTP(w, r)
	})
}
