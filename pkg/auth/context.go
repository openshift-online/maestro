package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/openshift-online/ocm-sdk-go/authentication"
)

// Context key type defined to avoid collisions in other pkgs using context
// See https://golang.org/pkg/context/#WithValue
type contextKey string

const (
	ContextUsernameKey contextKey = "username"

	// Does not use contextKey type because the jwt middleware improperly updates context with string key type
	// See https://github.com/auth0/go-jwt-middleware/blob/master/jwtmiddleware.go#L232
	ContextAuthKey string = "user"
)

// AuthPayload defines the structure of the JWT payload we expect from
// RHD JWT tokens
type AuthPayload struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Issuer    string `json:"iss"`
	ClientID  string `json:"clientId"`
}

func SetUsernameContext(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ContextUsernameKey, username)
}

func GetUsernameFromContext(ctx context.Context) string {
	username := ctx.Value(ContextUsernameKey)
	if username == nil {
		return ""
	}
	return username.(string)
}

// Get authorization payload api object from context
func GetAuthPayloadFromContext(ctx context.Context) (*AuthPayload, error) {
	// Get user token from request context and validate
	userToken, err := authentication.TokenFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve JWT token from request context: %v", err)
	}

	if userToken == nil {
		return nil, fmt.Errorf("JWT token in context is nil. Unauthorized.")
	}

	// Username is stored in token claim with key 'sub'
	claims, ok := userToken.Claims.(jwt.MapClaims)
	if !ok {
		err := fmt.Errorf("Unable to parse JWT token claims.  %#v", userToken.Claims)
		return nil, err
	}

	// TODO figure out how to unmarshal jwt.mapclaims into the struct to avoid all the
	// type assertions
	//
	//var accountAuth api.AuthPayload
	//err := json.Unmarshal([]byte(claims), &accountAuth)
	//if err != nil {
	//	err := fmt.Errorf("Unable to parse JWT token claims")
	//	return nil, err
	//}

	payload := &AuthPayload{}
	// default to the values we expect from RHSSO
	payload.Username, _ = claims["username"].(string)
	payload.FirstName, _ = claims["first_name"].(string)
	payload.LastName, _ = claims["last_name"].(string)
	payload.Email, _ = claims["email"].(string)
	payload.ClientID, _ = claims["clientId"].(string)

	// Check values, if empty, use alternative claims from RHD
	if payload.Username == "" {
		payload.Username, _ = claims["preferred_username"].(string)
	}

	if payload.FirstName == "" {
		payload.FirstName, _ = claims["given_name"].(string)
	}

	if payload.LastName == "" {
		payload.LastName, _ = claims["family_name"].(string)
	}

	// If given and family names are not present, use the name field
	if payload.FirstName == "" || payload.LastName == "" {
		name, _ := claims["name"].(string)
		names := strings.Split(name, " ")
		if len(names) > 1 {
			payload.FirstName = names[0]
			payload.LastName = names[1]
		} else {
			payload.FirstName = names[0]
		}
	}

	return payload, nil
}

func GetAuthPayload(r *http.Request) (*AuthPayload, error) {
	return GetAuthPayloadFromContext(r.Context())
}
