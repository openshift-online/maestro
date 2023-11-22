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

var _ RestHandler = resourceHandler{}

type resourceHandler struct {
	resource services.ResourceService
	generic  services.GenericService
}

func NewResourceHandler(resource services.ResourceService, generic services.GenericService) *resourceHandler {
	return &resourceHandler{
		resource: resource,
		generic:  generic,
	}
}

func (h resourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rs openapi.Resource
	cfg := &handlerConfig{
		&rs,
		[]validate{
			validateEmpty(&rs, "Id", "id"),
			validateNotEmpty(&rs, "ConsumerId", "consumer_id"),
			validateNotEmpty(&rs, "Manifest", "manifest"),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			resource := presenters.ConvertResource(rs)
			resource, err := h.resource.Create(ctx, resource)
			if err != nil {
				return nil, err
			}
			return presenters.PresentResource(resource), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusCreated)
}

func (h resourceHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.Resource

	cfg := &handlerConfig{
		&patch,
		[]validate{
			validateNotEmpty(&patch, "Version", "version"),
			validateNotEmpty(&patch, "Manifest", "manifest"),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			resource, err := h.resource.Replace(ctx, &api.Resource{
				Meta:     api.Meta{ID: id},
				Version:  *patch.Version,
				Manifest: patch.Manifest,
			})
			if err != nil {
				return nil, err
			}
			return presenters.PresentResource(resource), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusOK)
}

func (h resourceHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var resources []api.Resource
			paging, err := h.generic.List(ctx, "username", listArgs, &resources)
			if err != nil {
				return nil, err
			}
			resourceList := openapi.ResourceList{
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Resource{},
			}

			for _, resource := range resources {
				converted := presenters.PresentResource(&resource)
				resourceList.Items = append(resourceList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, resourceList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return resourceList, nil
		},
	}

	handleList(w, r, cfg)
}

func (h resourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			resource, err := h.resource.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return presenters.PresentResource(resource), nil
		},
	}

	handleGet(w, r, cfg)
}

func (h resourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.resource.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handleDelete(w, r, cfg, http.StatusNoContent)
}
