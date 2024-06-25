package grpcsource

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/store"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/utils"
)

// RESTFulAPIWatcherStore implements the WorkClientWatcherStore interface, it is
// used to build a source work client. The work client uses this store to
//   - get/list works from Maestro server via RESTfull APIs
//   - receive the work status update and send the updated work to the watch channel
type RESTFulAPIWatcherStore struct {
	sync.RWMutex

	sourceID  string
	apiClient *openapi.APIClient

	watchers  map[string]*workWatcher
	workQueue cache.Queue
}

var _ store.WorkClientWatcherStore = &RESTFulAPIWatcherStore{}

func newRESTFulAPIWatcherStore(ctx context.Context, apiClient *openapi.APIClient, sourceID string) *RESTFulAPIWatcherStore {
	s := &RESTFulAPIWatcherStore{
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

	rbs, _, err := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(context.Background()).
		Search(strings.Join(searches, " and ")).
		Page(1).
		Size(-1).
		Execute()
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
func (m *RESTFulAPIWatcherStore) HandleReceivedWork(action types.ResourceAction, work *workv1.ManifestWork) error {
	switch action {
	case types.StatusModified:
		watchType := watch.Modified
		if meta.IsStatusConditionTrue(work.Status.Conditions, common.ManifestsDeleted) {
			watchType = watch.Deleted
		}

		m.sendWatchEvent(watch.Event{Type: watchType, Object: work})
		return nil
	default:
		return fmt.Errorf("unknown resource action %s", action)
	}
}

// Get a work from maestro server with its namespace and name
func (m *RESTFulAPIWatcherStore) Get(namespace, name string) (*workv1.ManifestWork, error) {
	id := utils.UID(m.sourceID, namespace, name)
	rb, resp, err := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(context.Background(), id).Execute()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, errors.NewNotFound(common.ManifestWorkGR, id)
		}

		return nil, err
	}

	return ToManifestWork(rb)
}

// List works from maestro server with a specified namespace and list options.
// Using `metav1.NamespaceAll` to specify all namespace
func (m *RESTFulAPIWatcherStore) List(namespace string, opts metav1.ListOptions) ([]*workv1.ManifestWork, error) {
	works := []*workv1.ManifestWork{}

	// TODO consider how to support configuring page
	var page int32 = 1

	var size int32 = -1
	if opts.Limit > 0 {
		size = int32(opts.Limit)
	}

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

	search := strings.Join(searches, " and ")
	klog.V(4).Infof("list works with search=%s", search)

	rbs, _, err := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(context.Background()).
		Search(search).
		Page(page).
		Size(size).
		Execute()
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

	return works, nil
}

func (m *RESTFulAPIWatcherStore) ListAll() ([]*workv1.ManifestWork, error) {
	return m.List(metav1.NamespaceAll, metav1.ListOptions{})
}

func (m *RESTFulAPIWatcherStore) Add(work *workv1.ManifestWork) error {
	// do nothing
	return nil
}

func (m *RESTFulAPIWatcherStore) Update(work *workv1.ManifestWork) error {
	// do nothing
	return nil
}

func (m *RESTFulAPIWatcherStore) Delete(work *workv1.ManifestWork) error {
	// do nothing
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

	rbs, _, err := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(context.Background()).
		Search(strings.Join(search, " or ")).
		Page(1).
		Size(-1).
		Execute()
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
