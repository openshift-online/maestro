package handlers

import (
	"net/http"
)

type openAPIHandler struct {
	OpenAPIDefinitions []byte
}

func NewOpenAPIHandler(openAPIDefinitions []byte) *openAPIHandler {
	return &openAPIHandler{OpenAPIDefinitions: openAPIDefinitions}
}

func (h openAPIHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.OpenAPIDefinitions)
}
