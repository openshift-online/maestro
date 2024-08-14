package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

type ResourceBundlesStore struct {
	items []openapi.ResourceBundle
}

func (g *ResourceBundlesStore) Get() []openapi.ResourceBundle {
	return g.items
}

func (g *ResourceBundlesStore) Set(items []openapi.ResourceBundle) {
	g.items = items
}

type MaestroMockServer struct {
	server *httptest.Server
}

func NewMaestroMockServer(store *ResourceBundlesStore) *MaestroMockServer {
	mockServer := &MaestroMockServer{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list := &openapi.ResourceBundleList{}
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			size, _ := strconv.Atoi(r.URL.Query().Get("size"))

			items := store.Get()
			index := ((page - 1) * size)
			for i := 0; i < size; i++ {
				if index >= len(items) {
					break
				}
				list.Items = append(list.Items, items[index])
				index = index + 1
			}

			list.Page = int32(page)
			list.Total = int32(len(items))
			list.Size = int32(len(list.Items))
			data, _ := json.Marshal(list)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	mockServer.server = httptest.NewUnstartedServer(handler)
	return mockServer
}

func (m *MaestroMockServer) URL() string {
	return m.server.URL
}

func (m *MaestroMockServer) Start() {
	m.server.Start()
}

func (m *MaestroMockServer) Stop() {
	m.server.Close()
}

func NewMaestroAPIClient(maestroServerAddress string) *openapi.APIClient {
	cfg := &openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         maestroServerAddress,
				Description: "current domain",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	return openapi.NewAPIClient(cfg)
}
