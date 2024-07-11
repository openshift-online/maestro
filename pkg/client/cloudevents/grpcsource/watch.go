package grpcsource

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	workv1 "open-cluster-management.io/api/work/v1"
)

// workWatcher implements the watch.Interface.
type workWatcher struct {
	sync.RWMutex

	result  chan watch.Event
	done    chan struct{}
	stopped bool

	namespace     string
	labelSelector labels.Selector
}

var _ watch.Interface = &workWatcher{}

func newWorkWatcher(namespace string, labelSelector labels.Selector) *workWatcher {
	return &workWatcher{
		result:        make(chan watch.Event),
		done:          make(chan struct{}),
		namespace:     namespace,
		labelSelector: labelSelector,
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
		klog.Errorf("unknown event object type %T", evt.Object)
		return
	}

	if w.namespace != metav1.NamespaceAll && w.namespace != work.Namespace {
		klog.V(4).Infof("ignore the work %s/%s for the watcher %s", work.Namespace, work.Name, w.namespace)
		return
	}

	if !w.labelSelector.Matches(labels.Set(work.GetLabels())) {
		klog.V(4).Infof("ignore the label unmatched work %s/%s for the watcher %s", work.Namespace, work.Name, w.namespace)
		return
	}

	w.result <- evt
}

func (w *workWatcher) isStopped() bool {
	w.RLock()
	defer w.RUnlock()

	return w.stopped
}
