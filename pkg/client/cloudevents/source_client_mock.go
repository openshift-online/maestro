package cloudevents

import (
	"context"
	"fmt"

	"github.com/bwmarrin/snowflake"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/services"
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
	agent           string
	resources       api.ResourceList
	ResourceService services.ResourceService
}

var _ SourceClient = &SourceClientMock{}

func NewSourceClientMock(resourceService services.ResourceService) SourceClient {
	return &SourceClientMock{
		agent:           "mock-agent",
		ResourceService: resourceService,
	}
}

func (s *SourceClientMock) OnCreate(ctx context.Context, id string) error {
	resource, serviceErr := s.ResourceService.Get(ctx, id)
	if serviceErr != nil {
		return fmt.Errorf("failed to get resource: %v", serviceErr)
	}

	eventType := types.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceStatus,
		Action:              types.EventAction("update_request"),
	}
	evt := types.NewEventBuilder(s.agent, eventType).
		WithResourceID(resource.ID).
		WithStatusUpdateSequenceID(sequenceGenerator.Generate().String()).
		WithResourceVersion(int64(resource.Version)).
		WithClusterName(resource.ConsumerName).
		WithOriginalSource("maestro").
		NewEvent()

	manifestBundleStatus := &payload.ManifestBundleStatus{
		Conditions: []metav1.Condition{
			{
				Type:               "Applied",
				Status:             "True",
				LastTransitionTime: metav1.Now(),
			},
		},
	}
	if err := evt.SetData(cloudevents.ApplicationJSON, manifestBundleStatus); err != nil {
		return fmt.Errorf("failed to encode manifestwork status to a cloudevent: %v", err)
	}

	status, err := api.CloudEventToJSONMap(&evt)
	if err != nil {
		return fmt.Errorf("failed to convert resource status cloudevent to jsonmap: %v", err)
	}

	resource.Status = status
	newResource, _, serviceErr := s.ResourceService.UpdateStatus(ctx, resource)
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
			evt, err := api.JSONMAPToCloudEvent(resource.Status)
			if err != nil {
				return fmt.Errorf("failed to convert resource status to cloudevent: %v", err)
			}

			manifestBundleStatus := &workpayload.ManifestBundleStatus{}
			if err := evt.DataAs(manifestBundleStatus); err != nil {
				return fmt.Errorf("failed to decode cloudevent payload as resource status: %v", err)
			}

			condition := metav1.Condition{
				Type:               "Updated",
				Status:             "True",
				LastTransitionTime: metav1.Now(),
			}
			if len(manifestBundleStatus.Conditions) == 0 {
				manifestBundleStatus.Conditions = []metav1.Condition{condition}
			} else {
				manifestBundleStatus.Conditions = append(manifestBundleStatus.Conditions, condition)
			}

			if err := evt.SetData(cloudevents.ApplicationJSON, manifestBundleStatus); err != nil {
				return fmt.Errorf("failed to encode manifestwork status to a cloudevent: %v", err)
			}

			status, err := api.CloudEventToJSONMap(evt)
			if err != nil {
				return fmt.Errorf("failed to convert resource status cloudevent to jsonmap: %v", err)
			}

			resource.Status = status
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

func (s *SourceClientMock) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
}

func (s *SourceClientMock) Resync(ctx context.Context, consumers []string) error {
	return nil
}

func (s *SourceClientMock) ReconnectedChan() <-chan struct{} {
	return nil
}
