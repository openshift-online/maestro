package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/errors"
)

func ObjectKind(i interface{}) *string {
	result := ""
	switch i.(type) {
	case api.Consumer, *api.Consumer:
		result = "Consumer"
	case api.ConsumerList, *api.ConsumerList, []api.Consumer, []*api.Consumer:
		result = "ConsumerList"
	case api.Resource, *api.Resource:
		result = "ResourceBundle"
	case api.ResourceList, *api.ResourceList, []api.Resource, []*api.Resource:
		result = "ResourceBundleList"
	case errors.ServiceError, *errors.ServiceError:
		result = "Error"
	}

	return openapi.PtrString(result)
}
