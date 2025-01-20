package handlers

import (
	"fmt"
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
			validateNotEmpty(&rs, "ConsumerName", "consumer_name"),
			validateNotEmpty(&rs, "Manifest", "manifest"),
			validateManifestConfig(&rs),
			validateDeleteOptionAndUpdateStrategy(&rs),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			resource, err := presenters.ConvertResource(rs)
			if err != nil {
				return nil, errors.GeneralError("failed to convert resource: %s", err)
			}
			resource, serviceErr := h.resource.Create(ctx, resource)
			if serviceErr != nil {
				return nil, serviceErr
			}
			res, err := presenters.PresentResource(resource)
			if err != nil {
				return nil, errors.GeneralError("failed to present resource: %s", err)
			}
			return res, nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusCreated)
}

func (h resourceHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.ResourcePatchRequest

	cfg := &handlerConfig{
		&patch,
		[]validate{
			validateNotEmpty(&patch, "Version", "version"),
			validateNotEmpty(&patch, "Manifest", "manifest"),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, serviceErr := h.resource.Get(ctx, id)
			if serviceErr != nil {
				return nil, serviceErr
			}
			_, deleteOption, manifestConfig, err := api.DecodeManifest(found.Payload)
			if err != nil {
				return nil, errors.GeneralError("failed to decode existing manifest: %s", err)
			}
			if patch.DeleteOption != nil {
				deleteOption = patch.DeleteOption
			}
			if patch.ManifestConfig != nil {
				manifestConfig = patch.ManifestConfig
			}
			payload, err := presenters.ConvertResourceManifest(patch.Manifest, deleteOption, manifestConfig)
			if err != nil {
				return nil, errors.GeneralError("failed to convert resource manifest: %s", err)
			}
			resource, serviceErr := h.resource.Update(ctx, &api.Resource{
				Meta:    api.Meta{ID: id},
				Version: *patch.Version,
				Type:    api.ResourceTypeSingle,
				Payload: payload,
			})
			if serviceErr != nil {
				return nil, serviceErr
			}
			res, err := presenters.PresentResource(resource)
			if err != nil {
				return nil, errors.GeneralError("failed to present resource: %s", err)
			}
			return res, nil
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
			if listArgs.Search == "" {
				listArgs.Search = fmt.Sprintf("type='%s'", api.ResourceTypeSingle)
			} else {
				listArgs.Search = fmt.Sprintf("%s and type='%s'", listArgs.Search, api.ResourceTypeSingle)
			}
			var resources []api.Resource
			paging, serviceErr := h.generic.List(ctx, "username", listArgs, &resources)
			if serviceErr != nil {
				return nil, serviceErr
			}
			resourceList := openapi.ResourceList{
				Kind:  *presenters.ObjectKind(resources),
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Resource{},
			}

			for _, resource := range resources {
				converted, err := presenters.PresentResource(&resource)
				if err != nil {
					return nil, errors.GeneralError("failed to present resource: %s", err)
				}
				resourceList.Items = append(resourceList.Items, *converted)
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
			resource, serviceErr := h.resource.Get(ctx, id)
			if serviceErr != nil {
				return nil, serviceErr
			}

			res, err := presenters.PresentResource(resource)
			if err != nil {
				return nil, errors.GeneralError("failed to present resource: %s", err)
			}
			return res, nil
		},
	}

	handleGet(w, r, cfg)
}

// Resource Deletion Flow:
// 1. User requests deletion
// 2. Maestro marks resource as deleting, adds delete event to DB
// 3. Maestro handles delete event and sends CloudEvent to work-agent
// 4. Work-agent deletes resource, sends CloudEvent back to Maestro
// 5. Maestro deletes resource from DB
func (h resourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

func (h resourceHandler) GetBundle(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			resource, serviceErr := h.resource.Get(ctx, id)
			if serviceErr != nil {
				return nil, serviceErr
			}

			resBundle, err := presenters.PresentResourceBundle(resource)
			if err != nil {
				return nil, errors.GeneralError("failed to present resource bundle: %s", err)
			}
			return resBundle, nil
		},
	}

	handleGet(w, r, cfg)
}

func (h resourceHandler) ListBundle(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			if listArgs.Search == "" {
				listArgs.Search = fmt.Sprintf("type='%s'", api.ResourceTypeBundle)
			} else {
				listArgs.Search = fmt.Sprintf("%s and type='%s'", listArgs.Search, api.ResourceTypeBundle)
			}
			var resources []api.Resource
			paging, serviceErr := h.resource.ListWithArgs(ctx, "username", listArgs, &resources)
			if serviceErr != nil {
				return nil, serviceErr
			}
			resourceBundleList := openapi.ResourceBundleList{
				Kind:  "ResourceBundleList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.ResourceBundle{},
			}

			for _, resource := range resources {
				converted, err := presenters.PresentResourceBundle(&resource)
				if err != nil {
					return nil, errors.GeneralError("failed to present resource: %s", err)
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
