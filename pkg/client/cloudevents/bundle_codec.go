package cloudevents

import (
	"encoding/json"
	"fmt"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	workv1 "open-cluster-management.io/api/work/v1"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
)

type BundleCodec struct{}

var _ cegeneric.Codec[*api.Resource] = &BundleCodec{}

func (codec *BundleCodec) EventDataType() cetypes.CloudEventsDataType {
	return workpayload.ManifestBundleEventDataType
}

func (codec *BundleCodec) Encode(source string, eventType cetypes.CloudEventsType, obj *api.Resource) (*cloudevents.Event, error) {
	// the resource source takes precedence over the CloudEvent source.
	if obj.Source != "" {
		source = obj.Source
	}
	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithResourceID(obj.ID).
		WithResourceVersion(int64(obj.Version)).
		WithClusterName(obj.ConsumerID)

	if !obj.GetDeletionTimestamp().IsZero() {
		evtBuilder.WithDeletionTimestamp(obj.GetDeletionTimestamp().Time)
	}

	evt := evtBuilder.NewEvent()

	workJSON, err := json.Marshal(obj.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifestwork: %v", err)
	}
	work := &workv1.ManifestWork{}
	if err := json.Unmarshal(workJSON, work); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifestwork: %v", err)
	}

	resourcePayload := &workpayload.ManifestBundle{
		Manifests:       work.Spec.Workload.Manifests,
		DeleteOption:    work.Spec.DeleteOption,
		ManifestConfigs: work.Spec.ManifestConfigs,
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, resourcePayload); err != nil {
		return nil, fmt.Errorf("failed to encode resource to cloud event: %v", err)
	}

	return &evt, nil
}

func (codec *BundleCodec) Decode(evt *cloudevents.Event) (*api.Resource, error) {
	eventType, err := cetypes.ParseCloudEventsType(evt.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	if eventType.CloudEventsDataType != workpayload.ManifestBundleEventDataType {
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

	sequenceID, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionStatusUpdateSequenceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get sequenceid extension: %v", err)
	}

	clusterName, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	originalSource, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionOriginalSource])
	if err != nil {
		return nil, fmt.Errorf("failed to get originalsource extension: %v", err)
	}

	resourceStatusPayload := &workpayload.ManifestBundleStatus{}
	if err := json.Unmarshal(evt.Data(), resourceStatusPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data as resource status: %v", err)
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		Version:    int32(resourceVersionInt),
		Source:     originalSource,
		ConsumerID: clusterName,
	}

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ManifestWork",
			APIVersion: workv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceID,
			UID:  kubetypes.UID(resourceID),
		},
	}

	workJSON, err := json.Marshal(work)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifestwork: %v", err)
	}
	err = json.Unmarshal(workJSON, &resource.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifestwork: %v", err)
	}

	resourceStatus := &api.ResourceStatus{
		ReconcileStatus: &api.ReconcileStatus{
			ObservedVersion: int32(resourceVersionInt),
			SequenceID:      sequenceID,
			Conditions:      resourceStatusPayload.Conditions,
		},
	}

	contentStatusMap := make(map[string]interface{})
	contentStatusMap["ManifestStatus"] = resourceStatusPayload.ResourceStatus
	contentStatusJSON, err := json.Marshal(contentStatusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content status: %v", err)
	}
	err = json.Unmarshal(contentStatusJSON, &resourceStatus.ContentStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal content status: %v", err)
	}

	resource.Status, err = api.ResourceStatusToJSONMap(resourceStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource status: %v", err)
	}

	return resource, nil
}
