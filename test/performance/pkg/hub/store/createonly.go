package store

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/store"
)

type CreateOnlyWatcherStore struct {
}

func NewCreateOnlyWatcherStore() *CreateOnlyWatcherStore {
	return &CreateOnlyWatcherStore{}
}

func (s *CreateOnlyWatcherStore) GetWatcher(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("unsupported")
}

func (s *CreateOnlyWatcherStore) HandleReceivedResource(ctx context.Context, work *workv1.ManifestWork) error {
	// do nothing
	return nil
}

func (s *CreateOnlyWatcherStore) Add(work runtime.Object) error {
	// do nothing
	return nil
}

func (s *CreateOnlyWatcherStore) Update(work runtime.Object) error {
	return fmt.Errorf("unsupported")
}

func (s *CreateOnlyWatcherStore) Delete(work runtime.Object) error {
	return fmt.Errorf("unsupported")
}

func (s *CreateOnlyWatcherStore) List(namespace string, opts metav1.ListOptions) (*store.ResourceList[*workv1.ManifestWork], error) {
	return nil, fmt.Errorf("unsupported")
}

func (s *CreateOnlyWatcherStore) ListAll() ([]*workv1.ManifestWork, error) {
	return nil, fmt.Errorf("unsupported")
}

func (s *CreateOnlyWatcherStore) Get(namespace, name string) (*workv1.ManifestWork, bool, error) {
	return nil, false, nil
}

func (s *CreateOnlyWatcherStore) HasInitiated() bool {
	return false
}
