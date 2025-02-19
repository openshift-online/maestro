package cloudevents

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
	"github.com/prometheus/client_golang/prometheus"
	workv1 "open-cluster-management.io/api/work/v1"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	ceoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

// SourceClient is an interface for publishing resource events to consumers
// subscribing to and resyncing resource status from consumers.
type SourceClient interface {
	OnCreate(ctx context.Context, id string) error
	OnUpdate(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
	Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource])
	Resync(ctx context.Context, consumers []string) error
	ReconnectedChan() <-chan struct{}
}

type SourceClientImpl struct {
	Codec                  cegeneric.Codec[*api.Resource]
	CloudEventSourceClient *cegeneric.CloudEventSourceClient[*api.Resource]
	ResourceService        services.ResourceService
}

func NewSourceClient(sourceOptions *ceoptions.CloudEventsSourceOptions, resourceService services.ResourceService) (SourceClient, error) {
	ctx := context.Background()
	codec := NewCodec(sourceOptions.SourceID)
	ceSourceClient, err := cegeneric.NewCloudEventSourceClient[*api.Resource](ctx, sourceOptions,
		resourceService, ResourceStatusHashGetter, codec)
	if err != nil {
		return nil, err
	}

	// register resource resync metrics for cloud event source client
	cegeneric.RegisterCloudEventsMetrics(prometheus.DefaultRegisterer)

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

	logger.V(4).Infof("Publishing resource %s for db row insert", resource.ID)
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

	logger.V(4).Infof("Publishing resource %s for db row update", resource.ID)
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

	// ensure the resource has been marked as deleting
	if resource.Meta.DeletedAt.Time.IsZero() {
		return fmt.Errorf("resource %s has not been marked as deleting", resource.ID)
	}
	logger.V(4).Infof("Publishing resource %s for db row delete", resource.ID)
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

func (s *SourceClientImpl) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
	s.CloudEventSourceClient.Subscribe(ctx, handlers...)
}

func (s *SourceClientImpl) Resync(ctx context.Context, consumers []string) error {
	logger := logger.NewOCMLogger(ctx)

	logger.V(4).Infof("Resyncing resource status from consumers %v", consumers)
	for _, consumer := range consumers {
		if err := s.CloudEventSourceClient.Resync(ctx, consumer); err != nil {
			return err
		}
	}

	return nil
}

func (s *SourceClientImpl) ReconnectedChan() <-chan struct{} {
	return s.CloudEventSourceClient.ReconnectedChan()
}

// ResourceStatusHashGetter returns a hash of the resource status.
// It calculates the hash based on the manifestwork status to ensure consistency
// with the agent's status calculation. The resource status is converted to
// manifestwork status based on resource type before calculating the hash.
func ResourceStatusHashGetter(res *api.Resource) (string, error) {
	if len(res.Status) == 0 {
		return fmt.Sprintf("%x", sha256.Sum256([]byte(""))), nil
	}
	evt, err := api.JSONMAPToCloudEvent(res.Status)
	if err != nil {
		return "", fmt.Errorf("failed to convert resource status to cloud event, %v", err)
	}

	// retrieve stash hash from status CloudEvent extension;
	// if not found, calculate the status hash by itself
	evtExtensions := evt.Context.GetExtensions()
	statusHashVal, ok := evtExtensions[types.ExtensionStatusHash]
	if ok {
		return fmt.Sprintf("%v", statusHashVal), nil
	}

	// calculate the status hash by itself
	eventPayload := &workpayload.ManifestBundleStatus{}
	if err := evt.DataAs(eventPayload); err != nil {
		return "", fmt.Errorf("failed to decode cloudevent data as manifest bundle status: %v", err)
	}
	workStatus := workv1.ManifestWorkStatus{
		Conditions: eventPayload.Conditions,
		ResourceStatus: workv1.ManifestResourceStatus{
			Manifests: eventPayload.ResourceStatus,
		},
	}
	workStatusBytes, err := json.Marshal(workStatus)
	if err != nil {
		return "", fmt.Errorf("failed to marshal work status, %v", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256(workStatusBytes)), nil
}
