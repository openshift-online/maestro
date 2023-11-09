package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusMetricsHandler struct {
}

// NewPrometheusMetricsHandler adds custom metrics and proxy to prometheus handler
func NewPrometheusMetricsHandler() *prometheusMetricsHandler {
	return &prometheusMetricsHandler{}
}

func (h *prometheusMetricsHandler) Handler() http.Handler {
	handler := promhttp.Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
}
