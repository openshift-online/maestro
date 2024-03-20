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
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	ceoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// SourceClient is an interface for publishing resource events to consumers
// subscribing to and resyncing resource status from consumers.
type SourceClient interface {
	OnCreate(ctx context.Context, id string) error
	OnUpdate(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
	Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource])
	Resync(ctx context.Context, consumers []string) error
}

type SourceClientImpl struct {
	Codec                  cegeneric.Codec[*api.Resource]
	BundleCodec            cegeneric.Codec[*api.Resource]
	CloudEventSourceClient *cegeneric.CloudEventSourceClient[*api.Resource]
	ResourceService        services.ResourceService
}

func NewSourceClient(sourceOptions *ceoptions.CloudEventsSourceOptions, resourceService services.ResourceService) (SourceClient, error) {
	ctx := context.Background()
	codec, bundleCodec := &Codec{}, &BundleCodec{}
	ceSourceClient, err := cegeneric.NewCloudEventSourceClient[*api.Resource](ctx, sourceOptions,
		resourceService, ResourceStatusHashGetter, codec, bundleCodec)
	if err != nil {
		return nil, err
	}

	return &SourceClientImpl{
		Codec:                  codec,
		BundleCodec:            bundleCodec,
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
	if resource.Type == api.ResourceTypeBundle {
		eventType.CloudEventsDataType = s.BundleCodec.EventDataType()
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
	if resource.Type == api.ResourceTypeBundle {
		eventType.CloudEventsDataType = s.BundleCodec.EventDataType()
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
	if resource.Type == api.ResourceTypeBundle {
		eventType.CloudEventsDataType = s.BundleCodec.EventDataType()
	}
	if err := s.CloudEventSourceClient.Publish(ctx, eventType, resource); err != nil {
		logger.Error(fmt.Sprintf("Failed to publish resource %s: %s", resource.ID, err))
		return err
	}

	return nil
}

func (s *SourceClientImpl) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
	s.CloudEventSourceClient.Subscribe(ctx, handlers...)
}

func (s *SourceClientImpl) Resync(ctx context.Context, consumers []string) error {
	logger := logger.NewOCMLogger(ctx)

	logger.Infof("Resyncing resource status from consumers")

	for _, consumer := range consumers {
		if err := s.CloudEventSourceClient.Resync(ctx, consumer); err != nil {
			return err
		}
	}

	return nil
}

func ResourceStatusHashGetter(res *api.Resource) (string, error) {
	status, err := api.DecodeStatus(res.Status)
	if err != nil {
		return "", fmt.Errorf("failed to decode resource status, %v", err)
	}
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource status, %v", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256(statusBytes)), nil
}
