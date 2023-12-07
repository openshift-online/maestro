package cloudevents

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/api/meta"
	cegeneric "open-cluster-management.io/api/cloudevents/generic"
	ceoptions "open-cluster-management.io/api/cloudevents/generic/options"
	cetypes "open-cluster-management.io/api/cloudevents/generic/types"
)

type SourceClient interface {
	OnCreate(ctx context.Context, id string) error
	OnUpdate(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

type SourceClientImpl struct {
	Codec                  cegeneric.Codec[*api.Resource]
	CloudEventSourceClient *cegeneric.CloudEventSourceClient[*api.Resource]
	ResourceService        services.ResourceService
}

func NewSourceClient(sourceOptions *ceoptions.CloudEventsSourceOptions, resourceService services.ResourceService) (SourceClient, error) {
	ctx := context.Background()
	codec := &Codec{}
	ceSourceClient, err := cegeneric.NewCloudEventSourceClient[*api.Resource](ctx, sourceOptions,
		resourceService, ResourceStatusHashGetter, codec)
	if err != nil {
		return nil, err
	}

	logger := logger.NewOCMLogger(ctx)
	go func() {
		ceSourceClient.Subscribe(ctx, func(action cetypes.ResourceAction, resource *api.Resource) error {
			logger.Infof("received action %s for resource %s", action, resource.ID)
			switch action {
			case cetypes.StatusModified:
				resourceStatusJSON, err := json.Marshal(resource.Status)
				if err != nil {
					return err
				}
				resourceStatus := &api.ResourceStatus{}
				if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
					return err
				}

				// if the resource has been deleted from agent, delete it from maestro
				if resourceStatus.ReconcileStatus != nil && meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Deleted") {
					if err := resourceService.Delete(ctx, resource.ID); err != nil {
						return err
					}
				} else {
					// update the resource status
					if _, err := resourceService.UpdateStatus(ctx, resource); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unsupported action %s", action)
			}
			return nil
		})
	}()

	return &SourceClientImpl{
		Codec:                  codec,
		CloudEventSourceClient: ceSourceClient,
		ResourceService:        resourceService,
	}, nil
}

func (s *SourceClientImpl) OnCreate(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	resource, err := s.ResourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Publishing resource %s for db row insert", resource.ID)
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: s.Codec.EventDataType(),
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.EventAction("create_request"),
	}
	if err := s.CloudEventSourceClient.Publish(ctx, eventType, resource); err != nil {
		logger.Error(fmt.Sprintf("Failed to publish resource %s: %s", resource.ID, err))
		return err
	}

	return nil
}

func (s *SourceClientImpl) OnUpdate(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	resource, err := s.ResourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Publishing resource %s for db row update", resource.ID)
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: s.Codec.EventDataType(),
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.EventAction("update_request"),
	}
	if err := s.CloudEventSourceClient.Publish(ctx, eventType, resource); err != nil {
		logger.Error(fmt.Sprintf("Failed to publish resource %s: %s", resource.ID, err))
		return err
	}

	return nil
}

func (s *SourceClientImpl) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	resource, err := s.ResourceService.Get(ctx, id)
	if err != nil {
		return err
	}

	// mark the resource as deleting
	resource.Meta.DeletedAt.Time = time.Now()
	logger.Infof("Publishing resource %s for db row delete", resource.ID)
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: s.Codec.EventDataType(),
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.EventAction("delete_request"),
	}
	if err := s.CloudEventSourceClient.Publish(ctx, eventType, resource); err != nil {
		logger.Error(fmt.Sprintf("Failed to publish resource %s: %s", resource.ID, err))
		return err
	}

	return nil
}

func ResourceStatusHashGetter(res *api.Resource) (string, error) {
	statusBytes, err := json.Marshal(res.Status)
	if err != nil {
		return "", fmt.Errorf("failed to marshal work status, %v", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256(statusBytes)), nil
}
