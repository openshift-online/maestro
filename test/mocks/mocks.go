package mocks

import (
	"net/http"
	"net/http/httptest"
	"time"
)

// Returns a server that will wait waitTime when hit at endpoint
func NewMockServerTimeout(endpoint string, waitTime time.Duration) (*httptest.Server, func()) {
	apiHandler := http.NewServeMux()
	apiHandler.HandleFunc(endpoint,
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(waitTime)
		},
	)
	server := httptest.NewServer(apiHandler)
	return server, server.Close
}
