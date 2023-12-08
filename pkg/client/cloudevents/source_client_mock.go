package cloudevents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/services"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceClientMock is a mock implementation of the SourceClient interface
type SourceClientMock struct {
	resources       api.ResourceList
	ResourceService services.ResourceService
}

var _ SourceClient = &SourceClientMock{}

func NewSourceClientMock(resourceService services.ResourceService) SourceClient {
	return &SourceClientMock{
		ResourceService: resourceService,
	}
}

func (s *SourceClientMock) OnCreate(ctx context.Context, id string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, id)
	if serviceErr != nil {
		return fmt.Errorf("failed to get resource: %v", serviceErr)
	}

	resourceStatus := &api.ResourceStatus{
		ReconcileStatus: &api.ReconcileStatus{
			ObservedGeneration: resource.Version,
			Conditions: []metav1.Condition{
				{
					Type:               "Applied",
					Status:             "True",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	resourceStatusJSON, err := json.Marshal(resourceStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal resource status: %v", err)
	}
	err = json.Unmarshal(resourceStatusJSON, &resource.Status)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resource status: %v", err)
	}

	newResource, serviceErr := s.ResourceService.UpdateStatus(ctx, resource)
	if serviceErr != nil {
		return fmt.Errorf("failed to update resource status: %v", serviceErr)
	}

	s.resources = append(s.resources, newResource)

	return nil
}

func (s *SourceClientMock) OnUpdate(ctx context.Context, id string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, id)
	if serviceErr != nil {
		return fmt.Errorf("failed to get resource: %v", serviceErr)
	}

	found := false
	for i, r := range s.resources {
		if r.ID == resource.ID {
			resourceStatusJSON, err := json.Marshal(resource.Status)
			if err != nil {
				return fmt.Errorf("failed to marshal resource status: %v", err)
			}
			resourceStatus := &api.ResourceStatus{}
			if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
				return fmt.Errorf("failed to unmarshal resource status: %v", err)
			}
			if resourceStatus.ReconcileStatus == nil {
				resourceStatus.ReconcileStatus = &api.ReconcileStatus{}
			}
			resourceStatus.ReconcileStatus.ObservedGeneration = resource.Version
			condition := metav1.Condition{
				Type:               "Updated",
				Status:             "True",
				LastTransitionTime: metav1.Now(),
			}
			if len(resourceStatus.ReconcileStatus.Conditions) == 0 {
				resourceStatus.ReconcileStatus.Conditions = []metav1.Condition{condition}
			}

			resourceStatus.ReconcileStatus.Conditions = append(resourceStatus.ReconcileStatus.Conditions, condition)
			resourceStatusJSON, err = json.Marshal(resourceStatus)
			if err != nil {
				return fmt.Errorf("failed to marshal resource status: %v", err)
			}
			err = json.Unmarshal(resourceStatusJSON, &resource.Status)
			if err != nil {
				return fmt.Errorf("failed to unmarshal resource status: %v", err)
			}

			newResource, serviceErr := s.ResourceService.UpdateStatus(ctx, resource)
			if serviceErr != nil {
				return fmt.Errorf("failed to update resource status: %v", serviceErr)
			}
			s.resources[i] = newResource
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find resource %s", resource.ID)
	}

	return nil
}

func (s *SourceClientMock) OnDelete(ctx context.Context, id string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, id)
	if serviceErr != nil {
		return fmt.Errorf("failed to get resource: %v", serviceErr)
	}

	for i, r := range s.resources {
		if r.ID == resource.ID {
			if err := s.ResourceService.Delete(ctx, resource.ID); err != nil {
				return fmt.Errorf("failed to delete resource: %v", err)
			}
			s.resources = append(s.resources[:i], s.resources[i+1:]...)
		}
	}

	return nil
}
