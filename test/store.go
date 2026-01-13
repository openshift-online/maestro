package test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

type MemoryStore struct {
	sync.RWMutex
	resources map[string]*api.Resource
}

func NewStore() *MemoryStore {
	return &MemoryStore{
		resources: make(map[string]*api.Resource),
	}
}

func (s *MemoryStore) Add(resource *api.Resource) {
	s.Lock()
	defer s.Unlock()

	if resource.ID == "" {
		return
	}

	_, ok := s.resources[resource.ID]
	if !ok {
		s.resources[resource.ID] = resource
	}
}

func (s *MemoryStore) Update(resource *api.Resource) error {
	s.Lock()
	defer s.Unlock()

	if resource.ID == "" {
		return fmt.Errorf("the resource ID is empty")
	}

	_, ok := s.resources[resource.ID]
	if !ok {
		return fmt.Errorf("the resource %s does not exist", resource.ID)
	}

	s.resources[resource.ID] = resource

	return nil
}

func (s *MemoryStore) UpSert(resource *api.Resource) {
	s.Lock()
	defer s.Unlock()

	if resource.ID == "" {
		return
	}

	s.resources[resource.ID] = resource
}

func (s *MemoryStore) UpdateStatus(resource *api.Resource) error {
	s.Lock()
	defer s.Unlock()

	if resource.ID == "" {
		return fmt.Errorf("the resource ID is empty")
	}

	last, ok := s.resources[resource.ID]
	if !ok {
		return fmt.Errorf("the resource %s does not exist", resource.ID)
	}

	last.Status = resource.Status
	s.resources[resource.ID] = last

	return nil
}

func (s *MemoryStore) Delete(resourceID string) {
	s.Lock()
	defer s.Unlock()

	if resourceID == "" {
		return
	}

	delete(s.resources, resourceID)
}

func (s *MemoryStore) Get(resourceID string) (*api.Resource, error) {
	s.RLock()
	defer s.RUnlock()

	resource, ok := s.resources[resourceID]
	if !ok {
		return nil, fmt.Errorf("failed to find resource %s", resourceID)
	}

	return resource, nil
}

func (s *MemoryStore) ListByNamespace(namespace string) []*api.Resource {
	s.RLock()
	defer s.RUnlock()

	resources := make([]*api.Resource, len(s.resources))
	i := 0
	for _, res := range s.resources {
		if res.ConsumerName != namespace {
			continue
		}

		resources[i] = res
		i++
	}

	return resources
}

func (s *MemoryStore) List(_ context.Context, listOpts types.ListOptions) ([]*api.Resource, error) {
	return s.ListByNamespace(listOpts.ClusterName), nil
}

func resourceStatusHashGetter(obj *api.Resource) (string, error) {
	statusBytes, err := json.Marshal(obj.Status)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource status, %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(statusBytes)), nil
}
