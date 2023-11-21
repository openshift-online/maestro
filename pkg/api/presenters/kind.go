package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/errors"
)

func ObjectKind(i interface{}) *string {
	result := ""
	switch i.(type) {
	case api.Resource, *api.Resource:
		result = "Resource"
	case errors.ServiceError, *errors.ServiceError:
		result = "Error"
	}

	return openapi.PtrString(result)
}
