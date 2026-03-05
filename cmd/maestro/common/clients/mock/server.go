package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// Server wraps a test HTTP server that mocks the Maestro API
type Server struct {
	*httptest.Server
}

// NewMaestroServer creates a new mock Maestro API server
func NewMaestroServer() *Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Route based on path and method
		path := r.URL.Path
		method := r.Method

		switch {
		// Resource Bundle endpoints
		case method == "GET" && path == "/api/maestro/v1/resource-bundles":
			handleListResourceBundles(w, r)
		case method == "GET" && strings.HasPrefix(path, "/api/maestro/v1/resource-bundles/"):
			handleGetResourceBundle(w, r)
		case method == "DELETE" && strings.HasPrefix(path, "/api/maestro/v1/resource-bundles/"):
			handleDeleteResourceBundle(w, r)

		// Consumer endpoints
		case method == "GET" && path == "/api/maestro/v1/consumers":
			handleListConsumers(w, r)
		case method == "GET" && strings.HasPrefix(path, "/api/maestro/v1/consumers/"):
			handleGetConsumer(w, r)
		case method == "POST" && path == "/api/maestro/v1/consumers":
			handleCreateConsumer(w, r)
		case method == "PATCH" && strings.HasPrefix(path, "/api/maestro/v1/consumers/"):
			handleUpdateConsumer(w, r)
		case method == "DELETE" && strings.HasPrefix(path, "/api/maestro/v1/consumers/"):
			handleDeleteConsumer(w, r)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	return &Server{Server: server}
}

func handleListResourceBundles(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	size := r.URL.Query().Get("size")
	search := r.URL.Query().Get("search")

	now := time.Now()
	bundle1 := openapi.ResourceBundle{
		Id:           openapi.PtrString("bundle-1"),
		Name:         openapi.PtrString("test-bundle-1"),
		ConsumerName: openapi.PtrString("test-consumer"),
		Version:      openapi.PtrInt32(1),
		CreatedAt:    &now,
		UpdatedAt:    &now,
		Status: map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Applied",
					"status": "True",
				},
			},
		},
	}

	list := openapi.ResourceBundleList{
		Items: []openapi.ResourceBundle{bundle1},
		Page:  1,
		Size:  1,
		Total: 1,
	}

	// Simple search filter
	if search != "" && !strings.Contains(*bundle1.Name, search) {
		list.Items = []openapi.ResourceBundle{}
		list.Size = 0
		list.Total = 0
	}

	_ = page
	_ = size

	json.NewEncoder(w).Encode(list)
}

func handleGetResourceBundle(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/maestro/v1/resource-bundles/")

	switch id {
	case "bundle-1":
		now := time.Now()
		bundle := openapi.ResourceBundle{
			Id:           openapi.PtrString("bundle-1"),
			Name:         openapi.PtrString("test-bundle-1"),
			ConsumerName: openapi.PtrString("test-consumer"),
			Version:      openapi.PtrInt32(1),
			CreatedAt:    &now,
			UpdatedAt:    &now,
			Status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Applied",
						"status": "True",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(bundle)
	case "not-found":
		w.WriteHeader(http.StatusNotFound)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleDeleteResourceBundle(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/maestro/v1/resource-bundles/")

	switch id {
	case "bundle-1":
		w.WriteHeader(http.StatusNoContent)
	case "not-found":
		w.WriteHeader(http.StatusNotFound)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleListConsumers(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	size := r.URL.Query().Get("size")
	search := r.URL.Query().Get("search")

	now := time.Now()
	consumer1 := openapi.Consumer{
		Id:        openapi.PtrString("consumer-1"),
		Name:      openapi.PtrString("test-consumer-1"),
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	list := openapi.ConsumerList{
		Items: []openapi.Consumer{consumer1},
		Page:  1,
		Size:  1,
		Total: 1,
	}

	// Simple search filter
	if search != "" && !strings.Contains(*consumer1.Name, search) {
		list.Items = []openapi.Consumer{}
		list.Size = 0
		list.Total = 0
	}

	_ = page
	_ = size

	json.NewEncoder(w).Encode(list)
}

func handleGetConsumer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/maestro/v1/consumers/")

	switch id {
	case "consumer-1":
		now := time.Now()
		consumer := openapi.Consumer{
			Id:        openapi.PtrString("consumer-1"),
			Name:      openapi.PtrString("test-consumer-1"),
			CreatedAt: &now,
			UpdatedAt: &now,
		}
		json.NewEncoder(w).Encode(consumer)
	case "not-found":
		w.WriteHeader(http.StatusNotFound)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleCreateConsumer(w http.ResponseWriter, r *http.Request) {
	var consumer openapi.Consumer
	if err := json.NewDecoder(r.Body).Decode(&consumer); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	name := ""
	if consumer.Name != nil {
		name = *consumer.Name
	}

	switch name {
	case "conflict":
		w.WriteHeader(http.StatusConflict)
	case "bad-request":
		w.WriteHeader(http.StatusBadRequest)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		now := time.Now()
		created := openapi.Consumer{
			Id:        openapi.PtrString("new-consumer-id"),
			Name:      consumer.Name,
			CreatedAt: &now,
			UpdatedAt: &now,
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func handleUpdateConsumer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/maestro/v1/consumers/")

	var patch openapi.ConsumerPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch id {
	case "consumer-1":
		now := time.Now()
		updated := openapi.Consumer{
			Id:        openapi.PtrString("consumer-1"),
			Name:      openapi.PtrString("updated-consumer-1"),
			Labels:    patch.Labels,
			CreatedAt: &now,
			UpdatedAt: &now,
		}
		json.NewEncoder(w).Encode(updated)
	case "not-found":
		w.WriteHeader(http.StatusNotFound)
	case "bad-request":
		w.WriteHeader(http.StatusBadRequest)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func handleDeleteConsumer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/maestro/v1/consumers/")

	switch id {
	case "consumer-1":
		w.WriteHeader(http.StatusNoContent)
	case "not-found":
		w.WriteHeader(http.StatusNotFound)
	case "conflict":
		w.WriteHeader(http.StatusConflict)
	case "unauthorized":
		w.WriteHeader(http.StatusUnauthorized)
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}
