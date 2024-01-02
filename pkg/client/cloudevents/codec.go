package cloudevents

import (
	"encoding/json"
	"fmt"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cegeneric "open-cluster-management.io/api/cloudevents/generic"
	cetypes "open-cluster-management.io/api/cloudevents/generic/types"
	agentclient "open-cluster-management.io/api/cloudevents/work/agent/client"
	workpayload "open-cluster-management.io/api/cloudevents/work/payload"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/api"
)

type Codec struct{}

var _ cegeneric.Codec[*api.Resource] = &Codec{}

func (codec *Codec) EventDataType() cetypes.CloudEventsDataType {
	return workpayload.ManifestEventDataType
}

func (codec *Codec) Encode(source string, eventType cetypes.CloudEventsType, obj *api.Resource) (*cloudevents.Event, error) {
	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithResourceID(obj.ID).
		WithResourceVersion(int64(obj.Version)).
		WithClusterName(obj.ConsumerID)

	if !obj.GetDeletionTimestamp().IsZero() {
		evtBuilder.WithDeletionTimestamp(obj.GetDeletionTimestamp().Time)
	}

	evt := evtBuilder.NewEvent()

	resourcePayload := &workpayload.Manifest{
		Manifest: unstructured.Unstructured{Object: obj.Manifest},
		DeleteOption: &workv1.DeleteOption{
			PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
		},
		ConfigOption: &workpayload.ManifestConfigOption{
			FeedbackRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: "status",
							Path: ".status",
						},
					},
				},
			},
			UpdateStrategy: &workv1.UpdateStrategy{
				Type: workv1.UpdateStrategyTypeUpdate,
			},
		},
	}

	resourcePayloadJSON, err := json.Marshal(resourcePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource payload: %v", err)
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, resourcePayloadJSON); err != nil {
		return nil, fmt.Errorf("failed to encode resource to cloud event: %v", err)
	}

	return &evt, nil
}

func (codec *Codec) Decode(evt *cloudevents.Event) (*api.Resource, error) {
	eventType, err := cetypes.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	if eventType.CloudEventsDataType != workpayload.ManifestEventDataType {
		return nil, fmt.Errorf("unsupported cloudevents data type %s", eventType.CloudEventsDataType)
	}

	evtExtensions := evt.Context.GetExtensions()

	resourceID, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionResourceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceid extension: %v", err)
	}

	resourceVersionInt := int64(0)
	resourceVersion, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionResourceVersion])
	if err != nil {
		resourceVersionIntVal, err := cloudeventstypes.ToInteger(evtExtensions[cetypes.ExtensionResourceVersion])
		if err != nil {
			return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
		}
		resourceVersionInt = int64(resourceVersionIntVal)
	} else {
		resourceVersionInt, err = strconv.ParseInt(resourceVersion, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resourceversion - %v to int64", resourceVersion)
		}
	}

	clusterName, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	data := evt.Data()
	resourceStatusPayload := &workpayload.ManifestStatus{}
	if err := json.Unmarshal(data, resourceStatusPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data as resource status: %v", err)
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		ObservedVersion: int32(resourceVersionInt),
		ConsumerID:      clusterName,
	}

	resourceStatus := &api.ResourceStatus{
		ReconcileStatus: &api.ReconcileStatus{
			ObservedVersion: int32(resourceVersionInt),
		},
	}

	if resourceStatusPayload.Status != nil {
		resourceStatus.ReconcileStatus.Conditions = resourceStatusPayload.Status.Conditions
		if meta.IsStatusConditionTrue(resourceStatusPayload.Conditions, agentclient.ManifestsDeleted) {
			deletedCondition := meta.FindStatusCondition(resourceStatusPayload.Conditions, agentclient.ManifestsDeleted)
			resourceStatus.ReconcileStatus.Conditions = append(resourceStatus.ReconcileStatus.Conditions, *deletedCondition)
		}
		for _, value := range resourceStatusPayload.Status.StatusFeedbacks.Values {
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
