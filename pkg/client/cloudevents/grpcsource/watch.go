package grpcsource

import (
	"context"
	"sync"

	"github.com/openshift-online/ocm-sdk-go/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	workv1 "open-cluster-management.io/api/work/v1"
)

// workWatcher implements the watch.Interface.
type workWatcher struct {
	sync.RWMutex

	result  chan watch.Event
	done    chan struct{}
	stopped bool

	ctx    context.Context
	logger logging.Logger

	source        string
	namespace     string
	labelSelector labels.Selector
}

var _ watch.Interface = &workWatcher{}

func newWorkWatcher(ctx context.Context,
	logger logging.Logger, source, namespace string, labelSelector labels.Selector) *workWatcher {
	return &workWatcher{
		result:        make(chan watch.Event),
		done:          make(chan struct{}),
		source:        source,
		namespace:     namespace,
		labelSelector: labelSelector,
		ctx:           ctx,
		logger:        logger,
	}
}

// ResultChan implements Interface.
func (w *workWatcher) ResultChan() <-chan watch.Event {
	return w.result
}

// Stop implements Interface.
func (w *workWatcher) Stop() {
	// Call Close() exactly once by locking and setting a flag.
	w.Lock()
	defer w.Unlock()
	// closing a closed channel always panics, therefore check before closing
	w.logger.Info(w.ctx, "stop the watcher %s/%s", w.source, w.namespace)
	sourceClientRegisteredWatchersGaugeMetric.WithLabelValues(w.source, w.namespace).Dec()
	select {
	case <-w.done:
		close(w.result)
	default:
		w.stopped = true
		close(w.done)
	}
}

// Receive an event and sends down the result channel.
func (w *workWatcher) Receive(evt watch.Event) {
	if w.isStopped() {
		// this watcher is stopped, do nothing.
		return
	}

	work, ok := evt.Object.(*workv1.ManifestWork)
	if !ok {
		w.logger.Error(w.ctx, "unknown event object type %T", evt.Object)
		return
	}

	if w.namespace != metav1.NamespaceAll && w.namespace != work.Namespace {
		w.logger.Info(w.ctx, "ignore the work %s/%s from the watcher %s/%s", work.Namespace, work.Name, w.source, w.namespace)
		return
	}

	if !w.labelSelector.Matches(labels.Set(work.GetLabels())) {
		w.logger.Info(w.ctx, "ignore the label unmatched work %s/%s from the watcher %s/%s", work.Namespace, work.Name, w.source, w.namespace)
		return
	}

	w.logger.Debug(w.ctx, "send the work %s/%s status update (type=%s) from the watcher %s/%s",
		work.Namespace, work.Name, evt.Type, w.source, w.namespace)
	w.result <- evt
}

func (w *workWatcher) isStopped() bool {
	w.RLock()
	defer w.RUnlock()

	return w.stopped
}
