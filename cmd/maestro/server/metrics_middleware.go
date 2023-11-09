/*
Copyright (c) 2019 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains an HTTP middleware that generates metrics about API requests:
//
//	api_inbound_request_count - Number of requests served.
//	api_inbound_request_duration_sum - Total time to process requests, in seconds.
//	api_inbound_request_duration_count - Total number of requests measured.
//	api_inbound_request_duration_bucket - Number of requests that processed in less than a given time.
//
// The duration buckets metrics contain an `le` label that indicates the upper. For example if the
// `le` label is `1` then the value will be the number of requests that were processed in less than
// one second.
//
// All the metrics have the following labels:
//
//	method - Name of the HTTP method, for example GET or POST.
//	path - Request path, for example /api/clusters_mgmt/v1/clusters.
//	code - HTTP response code, for example 200 or 500.
//
// To calculate the average request duration during the last 10 minutes, for example, use a
// Prometheus expression like this:
//
//	rate(api_inbound_request_duration_sum[10m]) / rate(api_inbound_request_duration_count[10m])
//
// In order to reduce the cardinality of the metrics the path label is modified to remove the
// identifiers of the objects. For example, if the original path is .../clusters/123 then it will
// be replaced by .../clusters/-, and the values will be accumulated. The line returned by the
// metrics server will be like this:
//
//	api_inbound_request_count{code="200",method="GET",path="/api/clusters_mgmt/v1/clusters/-"} 56
//
// The meaning of that is that there were a total of 56 requests to get specific clusters,
// independently of the specific identifier of the cluster.

package server

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// MetricsMiddleware creates a new handler that collects metrics for the requests processed by the
// given handler.
func MetricsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the origial response writer with one that will allow as to get the response
		// status code:
		wrapper := &metricsResponseWrapper{
			wrapped: w,
		}

		// Call the next handler measuring the time that it takes:
		before := time.Now()
		handler.ServeHTTP(wrapper, r)
		elapsed := time.Since(before)

		// In order to reduce the cardinality of the metrics we need to remove from the
		// request path all the object identifiers:
		path := "/" + PathVarSub
		route := mux.CurrentRoute(r)
		if route != nil {
			template, err := route.GetPathTemplate()
			if err == nil {
				path = metricsPathVarRE.ReplaceAllString(template, PathVarSub)
			}
		}

		// Create the set of labels that we will add to all the requests:
		labels := prometheus.Labels{
			metricsMethodLabel: r.Method,
			metricsPathLabel:   path,
			metricsCodeLabel:   strconv.Itoa(wrapper.code),
		}

		// Update the metric containing the number of requests:
		requestCountMetric.With(labels).Inc()

		// Update the metrics containing the response duration:
		requestDurationMetric.With(labels).Observe(elapsed.Seconds())
	})
}

// ResetMetricCollectors resets all prometheus collectors
func ResetMetricCollectors() {
	requestCountMetric.Reset()
	requestDurationMetric.Reset()
}

// Regular expression used to remove variables from route path templates:
var metricsPathVarRE = regexp.MustCompile(`{[^}]*}`)

// PathVarSub replaces path variables to a same character
var PathVarSub = "-"

// Subsystem used to define the metrics:
const metricsSubsystem = "api_inbound"

// Names of the labels added to metrics:
const (
	metricsMethodLabel = "method"
	metricsPathLabel   = "path"
	metricsCodeLabel   = "code"
)

// MetricsLabels - Array of labels added to metrics:
var MetricsLabels = []string{
	metricsMethodLabel,
	metricsPathLabel,
	metricsCodeLabel,
}

// Names of the metrics:
const (
	requestCount    = "request_count"
	requestDuration = "request_duration"
)

// MetricsNames - Array of Names of the metrics:
var MetricsNames = []string{
	requestCount,
	requestDuration,
}

// Description of the requests count metric:
var requestCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: metricsSubsystem,
		Name:      requestCount,
		Help:      "Number of requests served.",
	},
	MetricsLabels,
)

// Description of the request duration metric:
var requestDurationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: metricsSubsystem,
		Name:      requestDuration,
		Help:      "Request duration in seconds.",
		Buckets: []float64{
			0.1,
			1.0,
			10.0,
			30.0,
		},
	},
	MetricsLabels,
)

// metricsResponseWrapper is an extension of the HTTP response writer that remembers the status code,
// so that we can add to metrics after the response is sent to the client.
type metricsResponseWrapper struct {
	wrapped http.ResponseWriter
	code    int
}

func (w *metricsResponseWrapper) Header() http.Header {
	return w.wrapped.Header()
}

func (w *metricsResponseWrapper) Write(b []byte) (n int, err error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	n, err = w.wrapped.Write(b)
	return
}

func (w *metricsResponseWrapper) WriteHeader(code int) {
	w.code = code
	w.wrapped.WriteHeader(code)
}

func init() {
	// Register the metrics:
	prometheus.MustRegister(requestCountMetric)
	prometheus.MustRegister(requestDurationMetric)
}
