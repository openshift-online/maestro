package grpcsource

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/openshift-online/maestro/pkg/api/openapi"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

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

	result         chan watch.Event
	done           chan struct{}
	watcherStopped bool

	sourceID  string
	apiClient *openapi.APIClient
}

var _ store.WorkClientWatcherStore = &RESTFulAPIWatcherStore{}

func NewRESTFullAPIWatcherStore(apiClient *openapi.APIClient, sourceID string) *RESTFulAPIWatcherStore {
	return &RESTFulAPIWatcherStore{
		result:         make(chan watch.Event),
		done:           make(chan struct{}),
		watcherStopped: false,

		sourceID:  sourceID,
		apiClient: apiClient,
	}
}

// ResultChan implements watch interface.
func (m *RESTFulAPIWatcherStore) ResultChan() <-chan watch.Event {
	return m.result
}

// Stop implements watch interface.
func (m *RESTFulAPIWatcherStore) Stop() {
	// Call Close() exactly once by locking and setting a flag.
	m.Lock()
	defer m.Unlock()
	// closing a closed channel always panics, therefore check before closing
	select {
	case <-m.done:
		close(m.result)
	default:
		m.watcherStopped = true
		close(m.done)
	}
}

// HandleReceivedWork sends the received works to the watch channel
func (m *RESTFulAPIWatcherStore) HandleReceivedWork(action types.ResourceAction, work *workv1.ManifestWork) error {
	if m.isWatcherStopped() {
		// watcher is stopped, do nothing.
		return nil
	}

	switch action {
	case types.StatusModified:
		watchType := watch.Modified
		if meta.IsStatusConditionTrue(work.Status.Conditions, common.ManifestsDeleted) {
			watchType = watch.Deleted
		}

		m.result <- watch.Event{Type: watchType, Object: work}
		return nil
	default:
		return fmt.Errorf("unknown resource action %s", action)
	}
}

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

func (m *RESTFulAPIWatcherStore) List(opts metav1.ListOptions) ([]*workv1.ManifestWork, error) {
	works := []*workv1.ManifestWork{}

	var size int32 = -1
	if opts.Limit > 0 {
		size = int32(opts.Limit)
	}

	apiRequest := m.apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(context.Background()).
		Search(fmt.Sprintf("source = '%s'", m.sourceID)).
		Page(1). // TODO consider how to support this
		Size(size)

	// TODO filter works by labels

	rbs, _, err := apiRequest.Execute()
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
	return m.List(metav1.ListOptions{})
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

func (m *RESTFulAPIWatcherStore) isWatcherStopped() bool {
	m.RLock()
	defer m.RUnlock()

	return m.watcherStopped
}
