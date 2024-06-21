package cloudevents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/snowflake"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/services"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var sequenceGenerator *snowflake.Node

func init() {
	// init the snowflake id generator with node id 1 for each single agent. Each single agent has its own consumer id
	// to be identified, and we can ensure the order of status update event from the same agent via sequence id. The
	// events from different agents are independent, hence the ordering among them needs not to be guaranteed.
	//
	// The snowflake `NewNode` returns error only when the snowflake node id is less than 1 or great than 1024, so the
	// error can be ignored here.
	sequenceGenerator, _ = snowflake.NewNode(1)
}

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

func (s *SourceClientMock) OnCreate(ctx context.Context, eventID, resourceID string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, resourceID)
	if serviceErr != nil {
		return fmt.Errorf("failed to get resource: %v", serviceErr)
	}

	resourceStatus := &api.ResourceStatus{
		ReconcileStatus: &api.ReconcileStatus{
			ObservedVersion: resource.Version,
			SequenceID:      sequenceGenerator.Generate().String(),
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

	newResource, _, serviceErr := s.ResourceService.UpdateStatus(ctx, resource)
	if serviceErr != nil {
		return fmt.Errorf("failed to update resource status: %v", serviceErr)
	}

	s.resources = append(s.resources, newResource)

	return nil
}

func (s *SourceClientMock) OnUpdate(ctx context.Context, eventID, resourceID string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, resourceID)
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
			resourceStatus.ReconcileStatus.ObservedVersion = resource.Version
			resourceStatus.ReconcileStatus.SequenceID = sequenceGenerator.Generate().String()
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

			newResource, _, serviceErr := s.ResourceService.UpdateStatus(ctx, resource)
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

func (s *SourceClientMock) OnDelete(ctx context.Context, eventID, resourceID string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, resourceID)
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

func (s *SourceClientMock) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
}

func (s *SourceClientMock) Resync(ctx context.Context, consumers []string) error {
	return nil
}

func (s *SourceClientMock) ReconnectedChan() <-chan struct{} {
	return nil
}
