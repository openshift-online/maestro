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

var _ RestHandler = dinosaurHandler{}

type dinosaurHandler struct {
	dinosaur services.DinosaurService
	generic  services.GenericService
}

func NewDinosaurHandler(dinosaur services.DinosaurService, generic services.GenericService) *dinosaurHandler {
	return &dinosaurHandler{
		dinosaur: dinosaur,
		generic:  generic,
	}
}

func (h dinosaurHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dinosaur openapi.Dinosaur
	cfg := &handlerConfig{
		&dinosaur,
		[]validate{
			validateEmpty(&dinosaur, "Id", "id"),
			validateNotEmpty(&dinosaur, "Species", "species"),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			dino := presenters.ConvertDinosaur(dinosaur)
			dino, err := h.dinosaur.Create(ctx, dino)
			if err != nil {
				return nil, err
			}
			return presenters.PresentDinosaur(dino), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusCreated)
}

func (h dinosaurHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.DinosaurPatchRequest

	cfg := &handlerConfig{
		&patch,
		[]validate{
			validateDinosaurPatch(&patch),
		},
		func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			dino, err := h.dinosaur.Replace(ctx, &api.Dinosaur{
				Meta:    api.Meta{ID: id},
				Species: *patch.Species,
			})
			if err != nil {
				return nil, err
			}
			return presenters.PresentDinosaur(dino), nil
		},
		handleError,
	}

	handle(w, r, cfg, http.StatusOK)
}

func (h dinosaurHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var dinosaurs = []api.Dinosaur{}
			paging, err := h.generic.List(ctx, "username", listArgs, &dinosaurs)
			if err != nil {
				return nil, err
			}
			dinoList := openapi.DinosaurList{
				Kind:  "DinosaurList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Dinosaur{},
			}

			for _, dino := range dinosaurs {
				converted := presenters.PresentDinosaur(&dino)
				dinoList.Items = append(dinoList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, dinoList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return dinoList, nil
		},
	}

	handleList(w, r, cfg)
}

func (h dinosaurHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			dinosaur, err := h.dinosaur.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return presenters.PresentDinosaur(dinosaur), nil
		},
	}

	handleGet(w, r, cfg)
}

func (h dinosaurHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.dinosaur.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handleDelete(w, r, cfg, http.StatusNoContent)
}
