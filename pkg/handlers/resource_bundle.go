package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/api/presenters"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

var _ RestHandler = resourceBundleHandler{}

type resourceBundleHandler struct {
	resource services.ResourceService
	generic  services.GenericService
}

func NewResourceBundleHandler(resource services.ResourceService, generic services.GenericService) *resourceBundleHandler {
	return &resourceBundleHandler{
		resource: resource,
		generic:  generic,
	}
}

func (h resourceBundleHandler) Create(w http.ResponseWriter, r *http.Request) {
	// not implemented
	http.Error(w, "Not Implemented Yet", http.StatusNotImplemented)
}

func (h resourceBundleHandler) Patch(w http.ResponseWriter, r *http.Request) {
	// not implemented
	http.Error(w, "Not Implemented Yet", http.StatusNotImplemented)
}

func (h resourceBundleHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			resource, serviceErr := h.resource.Get(ctx, id)
			if serviceErr != nil {
				return nil, serviceErr
			}

			rb, err := presenters.PresentResourceBundle(resource)
			if err != nil {
				return nil, errors.GeneralError("failed to present resource bundle: %s", err)
			}
			return rb, nil
		},
	}

	handleGet(w, r, cfg)
}

func (h resourceBundleHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var resources []api.Resource
			paging, serviceErr := h.resource.ListWithArgs(ctx, "username", listArgs, &resources)
			if serviceErr != nil {
				return nil, serviceErr
			}
			resourceBundleList := openapi.ResourceBundleList{
				Kind:  *presenters.ObjectKind(resources),
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.ResourceBundle{},
			}

			for _, resource := range resources {
				converted, err := presenters.PresentResourceBundle(&resource)
				if err != nil {
					return nil, errors.GeneralError("failed to present resource bundle: %s", err)
				}
				resourceBundleList.Items = append(resourceBundleList.Items, *converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, resourceBundleList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return resourceBundleList, nil
		},
	}

	handleList(w, r, cfg)
}

// Resource Bundle Deletion Flow:
// 1. User requests deletion
// 2. Maestro marks resource bundle as deleting, adds delete event to DB
// 3. Maestro handles delete event and sends CloudEvent to work-agent
// 4. Work-agent deletes resource bundle, sends CloudEvent back to Maestro
// 5. Maestro deletes resource bundle from DB
func (h resourceBundleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.resource.MarkAsDeleting(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handleDelete(w, r, cfg, http.StatusNoContent)
}
