package controllers

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-online/maestro/pkg/dao"
)

type deleteRecoveryMetrics struct {
	duration *prometheus.HistogramVec
	work     *prometheus.CounterVec
}

// newDeleteRecoveryMetrics initializes all bounded outcome and committed-work series.
func newDeleteRecoveryMetrics() *deleteRecoveryMetrics {
	m := &deleteRecoveryMetrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "delete_recovery",
			Name:      "round_duration_seconds",
			Help:      "Elapsed recovery runner time through transaction completion or error, not row lock wait time",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60},
		}, []string{"outcome"}),
		work: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "delete_recovery",
			Name:      "work_total",
			Help:      "Committed recovery work by kind (initialized resources, visited consumers, or published events)",
		}, []string{"kind"}),
	}
	for _, outcome := range []string{"committed", "empty", "noop", "error"} {
		m.duration.WithLabelValues(outcome)
	}
	for _, kind := range []string{"initialized", "consumers", "published"} {
		m.work.WithLabelValues(kind)
	}
	return m
}

// observe records elapsed runner time and counts work only for successfully committed rounds.
func (m *deleteRecoveryMetrics) observe(result dao.DeleteRecoveryResult, err error, seconds float64) {
	outcome := "noop"
	switch {
	case err != nil:
		outcome = "error"
	case result.Claimed:
		outcome = "empty"
		if result.Initialized != 0 || result.Consumers != 0 {
			outcome = "committed"
		}
		m.work.WithLabelValues("initialized").Add(float64(result.Initialized))
		m.work.WithLabelValues("consumers").Add(float64(result.Consumers))
		m.work.WithLabelValues("published").Add(float64(result.Published))
	}
	m.duration.WithLabelValues(outcome).Observe(seconds)
}

var recoveryMetrics = newDeleteRecoveryMetrics()
