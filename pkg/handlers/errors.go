package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/api/presenters"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

func NewErrorsHandler() *errorHandler {
	return &errorHandler{}
}

type errorHandler struct{}

var _ RestHandler = errorHandler{}

func (h errorHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			listArgs := services.NewListArguments(r.URL.Query())
			allErrors := errors.Errors()
			list, total := determineListRange(allErrors, listArgs.Page, listArgs.Size)
			errorList := openapi.ErrorList{
				Kind:  "ErrorList",
				Page:  int32(listArgs.Page),
				Size:  int32(len(list)),
				Total: int32(total),
				Items: []openapi.Error{},
			}
			for _, e := range list {
				err := e.(errors.ServiceError)
				errorList.Items = append(errorList.Items, presenters.PresentError(&err))
			}

			return errorList, nil
		},
	}

	handleList(w, r, cfg)
}

func (h errorHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			value, err := strconv.Atoi(id)
			if err != nil {
				return nil, errors.NotFound("No error with id %s exists", id)
			}
			code := errors.ServiceErrorCode(value)
			exists, sErr := errors.Find(code)
			if !exists {
				return nil, errors.NotFound("No error with id %s exists", id)
			}
			return presenters.PresentError(sErr), nil
		},
	}

	handleGet(w, r, cfg)
}

func (h errorHandler) Create(w http.ResponseWriter, r *http.Request) {
	handleError(r.Context(), w, errors.NotImplemented("create"))
}

func (h errorHandler) Patch(w http.ResponseWriter, r *http.Request) {
	handleError(r.Context(), w, errors.NotImplemented("path"))
}

func (h errorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	handleError(r.Context(), w, errors.NotImplemented("delete"))
}
