package grpcsource

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/store"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/utils"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// RESTFulAPIWatcherStore implements the WorkClientWatcherStore interface, it is
// used to build a source work client. The work client uses this store to
//   - get/list works from Maestro server via RESTfull APIs
//   - receive the work status update and send the updated work to the watch channel
type RESTFulAPIWatcherStore struct {
	sync.RWMutex

	// the context for RESTful API request, it is passed with RESTful API client together
	ctx context.Context

	sourceID  string
	apiClient *openapi.APIClient

	watchers  map[string]*workWatcher
	workQueue cache.Queue
}

var _ store.ClientWatcherStore[*workv1.ManifestWork] = &RESTFulAPIWatcherStore{}

func newRESTFulAPIWatcherStore(ctx context.Context, apiClient *openapi.APIClient, sourceID string) *RESTFulAPIWatcherStore {
	s := &RESTFulAPIWatcherStore{
		ctx:       ctx,
		sourceID:  sourceID,
		apiClient: apiClient,
		watchers:  make(map[string]*workWatcher),
		workQueue: cache.NewFIFO(func(obj interface{}) (string, error) {
			work, ok := obj.(*workv1.ManifestWork)
			if !ok {
				return "", fmt.Errorf("unknown object type %T", obj)
			}

			// ensure there is only one object in the queue for a work
			return string(work.UID), nil
		}),
	}

	// start a goroutine to send works to the watcher
	go wait.Until(s.process, time.Second, ctx.Done())

	return s
}

// GetWatcher returns a watcher to the source work client with a specified namespace (consumer name).
// Using `metav1.NamespaceAll` to specify all namespaces.
func (m *RESTFulAPIWatcherStore) GetWatcher(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	// Only list works from maestro server with the given namespace when a watcher is required
	labelSelector, labelSearch, selectable, err := ToLabelSearch(opts)
	if err != nil {
		return nil, err
	}

	searches := []string{fmt.Sprintf("source='%s'", m.sourceID)}
	if namespace != metav1.NamespaceAll {
		searches = append(searches, fmt.Sprintf("consumer_name='%s'", namespace))
	}

	if selectable {
		searches = append(searches, labelSearch)
	}

	// for watch, we need list all works with the search condition from maestro server
	rbs, _, err := pageList(m.ctx, m.apiClient, strings.Join(searches, " and "), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	watcher := m.registerWatcher(namespace, labelSelector)

	// save the works to a queue
	for _, rb := range rbs.Items {
		work, err := ToManifestWork(&rb)
		if err != nil {
			return nil, err
		}

		if err := m.workQueue.Add(work); err != nil {
			return nil, err
		}
	}

	return watcher, nil
}

// HandleReceivedWork sends the received works to the watch channel
func (m *RESTFulAPIWatcherStore) HandleReceivedResource(action types.ResourceAction, work *workv1.ManifestWork) error {
	switch action {
	case types.StatusModified:
		watchType := watch.Modified
		if meta.IsStatusConditionTrue(work.Status.Conditions, common.ResourceDeleted) {
			watchType = watch.Deleted
		}

		m.sendWatchEvent(watch.Event{Type: watchType, Object: work})
		return nil
	default:
		return fmt.Errorf("unknown resource action %s", action)
	}
}

// Get a work from maestro server with its namespace and name
func (m *RESTFulAPIWatcherStore) Get(namespace, name string) (*workv1.ManifestWork, bool, error) {
	id := utils.UID(m.sourceID, common.ManifestWorkGR.String(), namespace, name)
	rb, resp, err := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(m.ctx, id).Execute()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}

		return nil, false, err
	}

	work, err := ToManifestWork(rb)
	if err != nil {
		return nil, false, err
	}

	return work, true, nil
}

// List works from maestro server with a specified namespace and list options.
// Using `metav1.NamespaceAll` to specify all namespace.
func (m *RESTFulAPIWatcherStore) List(namespace string, opts metav1.ListOptions) (*store.ResourceList[*workv1.ManifestWork], error) {
	works := []*workv1.ManifestWork{}

	_, labelSearch, selectable, err := ToLabelSearch(opts)
	if err != nil {
		return nil, err
	}

	searches := []string{fmt.Sprintf("source='%s'", m.sourceID)}
	if namespace != metav1.NamespaceAll {
		searches = append(searches, fmt.Sprintf("consumer_name='%s'", namespace))
	}

	if selectable {
		searches = append(searches, labelSearch)
	}

	rbs, nextPage, err := pageList(m.ctx, m.apiClient, strings.Join(searches, " and "), opts)
	if err != nil {
		return nil, err
	}

	for _, rb := range rbs.Items {
		work, err := ToManifestWork(&rb)
		if err != nil {
			return nil, err
		}

		works = append(works, work)
	}

	return &store.ResourceList[*workv1.ManifestWork]{
		ListMeta: metav1.ListMeta{Continue: nextPage},
		Items:    works,
	}, nil
}

func (m *RESTFulAPIWatcherStore) ListAll() ([]*workv1.ManifestWork, error) {
	// for RESTFulAPIWatcherStore, this will not be called by manifestwork client, do nothing
	return nil, nil
}

func (m *RESTFulAPIWatcherStore) Add(work runtime.Object) error {
	// for RESTFulAPIWatcherStore, this will not be called by manifestwork client, do nothing
	return nil
}

func (m *RESTFulAPIWatcherStore) Update(work runtime.Object) error {
	// for RESTFulAPIWatcherStore, this will not be called by manifestwork client, do nothing
	return nil
}

func (m *RESTFulAPIWatcherStore) Delete(work runtime.Object) error {
	// for RESTFulAPIWatcherStore, this will not be called by manifestwork client, do nothing
	return nil
}

func (m *RESTFulAPIWatcherStore) HasInitiated() bool {
	return true
}

func (m *RESTFulAPIWatcherStore) Sync() error {
	m.RLock()
	defer m.RUnlock()

	if len(m.watchers) == 0 {
		// there are no watchers, do nothing
		return nil
	}

	hasAll := false
	namespaces := []string{}
	for namespace := range m.watchers {
		if namespace == metav1.NamespaceAll {
			hasAll = true
			break
		}

		namespaces = append(namespaces, fmt.Sprintf("consumer_name='%s'", namespace))
	}

	search := []string{fmt.Sprintf("source='%s'", m.sourceID)}
	if !hasAll {
		search = append(search, namespaces...)
	}

	// for sync, we need list all works with the search condition from maestro server
	rbs, _, err := pageList(m.ctx, m.apiClient, strings.Join(search, " or "), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// save the works to a queue
	for _, rb := range rbs.Items {
		work, err := ToManifestWork(&rb)
		if err != nil {
			return err
		}

		if err := m.workQueue.Add(work); err != nil {
			return err
		}
	}

	return nil
}

// process drains the work queue and send the work to the watch channel.
func (m *RESTFulAPIWatcherStore) process() {
	for {
		// this will be blocked until the work queue has works
		obj, err := m.workQueue.Pop(func(interface{}, bool) error {
			// do nothing
			return nil
		})
		if err != nil {
			if err == cache.ErrFIFOClosed {
				return
			}

			klog.Warningf("failed to pop the %v requeue it, %v", obj, err)
			// this is the safe way to re-enqueue.
			if err := m.workQueue.AddIfNotPresent(obj); err != nil {
				klog.Errorf("failed to requeue the obj %v, %v", obj, err)
				return
			}
		}

		work, ok := obj.(*workv1.ManifestWork)
		if !ok {
			klog.Errorf("unknown the object type %T from the event queue", obj)
			return
		}

		m.sendWatchEvent(watch.Event{Type: watch.Modified, Object: work})
	}
}

func (m *RESTFulAPIWatcherStore) registerWatcher(namespace string, labelSelector labels.Selector) watch.Interface {
	m.Lock()
	defer m.Unlock()

	watcher, ok := m.watchers[namespace]
	if ok {
		return watcher
	}

	watcher = newWorkWatcher(namespace, labelSelector)
	m.watchers[namespace] = watcher
	return watcher
}

func (m *RESTFulAPIWatcherStore) sendWatchEvent(evt watch.Event) {
	m.RLock()
	defer m.RUnlock()

	for _, w := range m.watchers {
		// this will be blocked until this work is consumed
		w.Receive(evt)
	}
}
