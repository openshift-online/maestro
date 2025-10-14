package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/util/workqueue"
)

// Subsystem used to define the metrics:
const (
	specControllerMetricsSubsystem   = "spec_controller"
	statusControllerMetricsSubsystem = "status_controller"
	workqueueMetricsSubsystem        = "workqueue"
)

// Names of the metrics:
const (
	eventReconcileTotalMetric     = "event_reconcile_total"
	eventReconcileDurationMetric  = "event_reconcile_duration_seconds"
	eventSyncOperationTotalMetric = "event_sync_operation_total"
	DepthMetric                   = "depth"
	AddsTotalMetric               = "adds_total"
	QueueDurationMetric           = "queue_duration_seconds"
	WorkDurationMetric            = "work_duration_seconds"
	UnfinishedWorkSecondsMetric   = "unfinished_work_seconds"
	LongestRunningProcessor       = "longest_running_processor_seconds"
	RetriesTotalMetric            = "retries_total"
)

// Names of the labels added to metrics:
const (
	controllerMetricsTypeLabel   = "event_type"
	controllerMetricsStatusLabel = "status"
	workqueueNameLabel           = "queue_name"
)

type controllerReconciledStatus string

// Possible values for the status label for controller reconciliations:
const (
	controllerReconciledStatusSuccess controllerReconciledStatus = "success"
	controllerReconciledStatusError   controllerReconciledStatus = "error"
	controllerReconciledStatusSkipped controllerReconciledStatus = "skipped"
)

type controllerSyncEventStatus string

// Possible values for the status label for controller sync operations:
const (
	controllerSyncEventStatusSuccess controllerSyncEventStatus = "success"
	controllerSyncEventStatusError   controllerSyncEventStatus = "error"
)

var (
	// specEventReconciledTotal is a counter of the total number of events
	// reconciled by the spec controller, labeled by type and status:
	specEventReconciledTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: specControllerMetricsSubsystem,
			Name:      eventReconcileTotalMetric,
			Help:      "Total number of events reconciled by the spec controller",
		},
		[]string{controllerMetricsTypeLabel, controllerMetricsStatusLabel},
	)

	// specEventReconcileDuration is a histogram of the time spent reconciling
	// spec events by the spec controller, labeled by type:
	specEventReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: specControllerMetricsSubsystem,
			Name:      eventReconcileDurationMetric,
			Help:      "Time spent reconciling spec events by the spec controller",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{controllerMetricsTypeLabel},
	)

	// specControllerSyncEventOperationsTotal is a counter of the total number of
	// sync operations performed by the spec controller, labeled by status:
	specControllerSyncEventOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: specControllerMetricsSubsystem,
			Name:      eventSyncOperationTotalMetric,
			Help:      "Total number of sync operations performed by the spec controller",
		},
		[]string{controllerMetricsStatusLabel},
	)

	// statusEventReconciledTotal is a counter of the total number of events
	// reconciled by the status controller, labeled by type and status:
	statusEventReconciledTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: statusControllerMetricsSubsystem,
			Name:      eventReconcileTotalMetric,
			Help:      "Total number of events reconciled by the status controller",
		},
		[]string{controllerMetricsTypeLabel, controllerMetricsStatusLabel},
	)

	// statusEventReconcileDuration is a histogram of the time spent reconciling
	// status events by the status controller, labeled by type:
	statusEventReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: statusControllerMetricsSubsystem,
			Name:      eventReconcileDurationMetric,
			Help:      "Time spent reconciling status events by the status controller",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{controllerMetricsTypeLabel},
	)

	// statusControllerSyncEventOperationsTotal is a counter of the total number of
	// sync operations performed by the status controller, labeled by status:
	statusControllerSyncEventOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: statusControllerMetricsSubsystem,
			Name:      eventSyncOperationTotalMetric,
			Help:      "Total number of sync operations performed by the status controller",
		},
		[]string{controllerMetricsStatusLabel},
	)

	// workqueueDepth is a gauge of the current depth of workqueues, labeled by name:
	workqueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      DepthMetric,
			Help:      "Current depth of workqueue",
		},
		[]string{workqueueNameLabel},
	)

	// workqueueAdds is a counter of the total number of adds handled by workqueues, labeled by name:
	workqueueAdds = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      AddsTotalMetric,
			Help:      "Total number of adds handled by workqueue",
		},
		[]string{workqueueNameLabel},
	)

	// workqueueLatency is a histogram of the time items stay in workqueues before being requested, labeled by name:
	workqueueLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      QueueDurationMetric,
			Help:      "How long in seconds an item stays in workqueue before being requested.",
			Buckets:   prometheus.ExponentialBuckets(10e-9, 10, 10),
		},
		[]string{workqueueNameLabel},
	)

	// workqueueWorkDuration is a histogram of the time items take to be processed from workqueues, labeled by name:
	workqueueWorkDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      WorkDurationMetric,
			Help:      "How long in seconds processing an item from workqueue takes.",
			Buckets:   prometheus.ExponentialBuckets(10e-9, 10, 10),
		},
		[]string{workqueueNameLabel},
	)

	// workqueueUnfinishedWorkSeconds is a gauge of the number of seconds of work that
	// is in progress and hasn't been observed by work_duration, labeled by name:
	workqueueUnfinishedWorkSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      UnfinishedWorkSecondsMetric,
			Help: "How many seconds of work has done that " +
				"is in progress and hasn't been observed by work_duration. Large " +
				"values indicate stuck threads. One can deduce the number of stuck " +
				"threads by observing the rate at which this increases.",
		},
		[]string{workqueueNameLabel},
	)

	// workqueueLongestRunningProcessor is a gauge of the number of seconds the longest running
	// processor for workqueue has been running, labeled by name:
	workqueueLongestRunningProcessor = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      LongestRunningProcessor,
			Help: "How many seconds has the longest running " +
				"processor for workqueue been running.",
		},
		[]string{workqueueNameLabel},
	)

	// workqueueRetries is a counter of the total number of retries handled by workqueues, labeled by name:
	workqueueRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: workqueueMetricsSubsystem,
			Name:      RetriesTotalMetric,
			Help:      "Total number of retries handled by workqueue",
		},
		[]string{workqueueNameLabel},
	)

	// workqueueMetrics is the list of all the workqueue metrics defined in this package:
	workqueueMetrics = []prometheus.Collector{
		workqueueDepth,
		workqueueAdds,
		workqueueLatency,
		workqueueWorkDuration,
		workqueueUnfinishedWorkSeconds,
		workqueueLongestRunningProcessor,
		workqueueRetries,
	}
)

func init() {
	// Register the metrics for controllers:
	prometheus.MustRegister(specEventReconciledTotal)
	prometheus.MustRegister(specEventReconcileDuration)
	prometheus.MustRegister(specControllerSyncEventOperationsTotal)
	prometheus.MustRegister(statusEventReconciledTotal)
	prometheus.MustRegister(statusEventReconcileDuration)
	prometheus.MustRegister(statusControllerSyncEventOperationsTotal)

	// Register the Prometheus workqueue metrics globally:
	for _, metric := range workqueueMetrics {
		prometheus.MustRegister(metric)
	}

	// Set the workqueue metrics provider
	workqueue.SetProvider(prometheusMetricsProvider{})
}

var _ workqueue.MetricsProvider = prometheusMetricsProvider{}

// prometheusMetricsProvider is an implementation of workqueue.MetricsProvider that produces
// Prometheus metrics for workqueues based on the above metric definitions.
type prometheusMetricsProvider struct {
}

func (prometheusMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return workqueueDepth.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return workqueueAdds.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return workqueueLatency.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return workqueueWorkDuration.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueUnfinishedWorkSeconds.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueLongestRunningProcessor.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return workqueueRetries.WithLabelValues(name)
}
