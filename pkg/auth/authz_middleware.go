package auth

/*
   The goal of this simple authz middlewre is to provide a way for access review
   parameters to be declared for each route in a microservice. This is not meant
   to handle more complex access review calls in particular scopes, but rather
   just authz calls at the application scope

  This is a big TODO, not ready for consumption
*/

import (
	"fmt"
	"net/http"

	"github.com/openshift-online/maestro/pkg/client/ocm"
)

type AuthorizationMiddleware interface {
	AuthorizeApi(next http.Handler) http.Handler
}

type authzMiddleware struct {
	action       string
	resourceType string

	ocmClient *ocm.Client
}

var _ AuthorizationMiddleware = &authzMiddleware{}

func NewAuthzMiddleware(ocmClient *ocm.Client, action, resourceType string) AuthorizationMiddleware {
	return &authzMiddleware{
		ocmClient:    ocmClient,
		action:       action,
		resourceType: resourceType,
	}
}

func (a authzMiddleware) AuthorizeApi(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get username from context
		username := GetUsernameFromContext(ctx)
		if username == "" {
			_ = fmt.Errorf("Authenticated username not present in request context")
			// TODO
			//body := api.E500.Format(r, "Authentication details not present in context")
			//api.SendError(w, r, &body)
			return
		}

		allowed, err := a.ocmClient.Authorization.AccessReview(
			ctx, username, a.action, a.resourceType, "", "", "")
		if err != nil {
			_ = fmt.Errorf("Unable to make authorization request: %s", err)
			// TODO
			//body := api.E500.Format(r, "Unable to make authorization request")
			//api.SendError(w, r, &body)
			return
		}

		if allowed {
			next.ServeHTTP(w, r)
		}

		// TODO
		//body := api.E403.Format(r, "")
		//api.SendError(w, r, &body)
	})
}
