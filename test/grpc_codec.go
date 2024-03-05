package test

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/openshift-online/maestro/pkg/api"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	agentclient "open-cluster-management.io/sdk-go/pkg/cloudevents/work/agent/client"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

type ResourceCodec struct{}

var _ generic.Codec[*api.Resource] = &ResourceCodec{}

func (c *ResourceCodec) EventDataType() types.CloudEventsDataType {
	return payload.ManifestEventDataType
}

// encode the kubernetes resource to a cloudevent format
func (c *ResourceCodec) Encode(source string, eventType types.CloudEventsType, resource *api.Resource) (*cloudevents.Event, error) {
	if eventType.CloudEventsDataType != payload.ManifestEventDataType {
		return nil, fmt.Errorf("unsupported cloudevents data type %s", eventType.CloudEventsDataType)
	}

	eventBuilder := types.NewEventBuilder(source, eventType).
		WithResourceID(resource.ID).
		WithResourceVersion(int64(resource.Version)).
		WithClusterName(resource.ConsumerID)

	if !resource.GetDeletionTimestamp().IsZero() {
		evt := eventBuilder.WithDeletionTimestamp(resource.GetDeletionTimestamp().Time).NewEvent()
		return &evt, nil
	}

	evt := eventBuilder.NewEvent()

	if err := evt.SetData(cloudevents.ApplicationJSON, &payload.Manifest{Manifest: unstructured.Unstructured{Object: resource.Manifest}}); err != nil {
		return nil, fmt.Errorf("failed to encode manifests to cloud event: %v", err)
	}

	return &evt, nil
}

func (c *ResourceCodec) Decode(evt *cloudevents.Event) (*api.Resource, error) {
	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	if eventType.CloudEventsDataType != payload.ManifestEventDataType {
		return nil, fmt.Errorf("unsupported cloudevents data type %s", eventType.CloudEventsDataType)
	}

	evtExtensions := evt.Context.GetExtensions()

	resourceID, err := cloudeventstypes.ToString(evtExtensions[types.ExtensionResourceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceid extension: %v", err)
	}

	resourceVersion, err := cloudeventstypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
	}

	clusterName, err := cloudeventstypes.ToString(evtExtensions[types.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	manifestStatus := &payload.ManifestStatus{}
	if err := evt.DataAs(manifestStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data %s, %v", string(evt.Data()), err)
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		Version:    int32(resourceVersion),
		ConsumerID: clusterName,
	}

	resourceStatus := &api.ResourceStatus{
		ReconcileStatus: &api.ReconcileStatus{
			ObservedVersion: resourceVersion,
		},
	}

	if manifestStatus.Status != nil {
		resourceStatus.ReconcileStatus.Conditions = manifestStatus.Status.Conditions
		if meta.IsStatusConditionTrue(manifestStatus.Conditions, agentclient.ManifestsDeleted) {
			deletedCondition := meta.FindStatusCondition(manifestStatus.Conditions, agentclient.ManifestsDeleted)
			resourceStatus.ReconcileStatus.Conditions = append(resourceStatus.ReconcileStatus.Conditions, *deletedCondition)
		}
		for _, value := range manifestStatus.Status.StatusFeedbacks.Values {
			if value.Name == "status" {
				contentStatus := make(map[string]interface{})
				if err := json.Unmarshal([]byte(*value.Value.JsonRaw), &contentStatus); err != nil {
					return nil, fmt.Errorf("failed to convert status feedback value to content status: %v", err)
				}
				resourceStatus.ContentStatus = contentStatus
			}
		}
	}

	resourceStatusJSON, err := json.Marshal(resourceStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource status: %v", err)
	}
	err = json.Unmarshal(resourceStatusJSON, &resource.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource status: %v", err)
	}

	return resource, nil
}
