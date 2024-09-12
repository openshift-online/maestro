package auth

import (
	"net/http"

	"k8s.io/klog/v2"
)

type authzMiddlewareMock struct{}

var _ AuthorizationMiddleware = &authzMiddlewareMock{}

func NewAuthzMiddlewareMock() AuthorizationMiddleware {
	return &authzMiddlewareMock{}
}

func (a authzMiddlewareMock) AuthorizeApi(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		klog.V(10).Infof("Mock authz allows <any>/<any> for %q/%q", r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
