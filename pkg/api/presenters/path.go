package presenters

import (
	"fmt"

	"github.com/openshift-online/maestro/pkg/api/openapi"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

const (
	BasePath = "/api/maestro/v1"
)

func ObjectPath(id string, obj interface{}) *string {
	return openapi.PtrString(fmt.Sprintf("%s/%s/%s", BasePath, path(obj), id))
}

func path(i interface{}) string {
	switch i.(type) {
	case api.Resource, *api.Resource:
		return "resource-bundles"
	case api.Consumer, *api.Consumer:
		return "consumers"
	case errors.ServiceError, *errors.ServiceError:
		return "errors"
	default:
		return ""
	}
}
