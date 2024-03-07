package cloudevents

import (
	"fmt"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
)

type BundleCodec struct{}

var _ cegeneric.Codec[*api.Resource] = &BundleCodec{}

func (codec *BundleCodec) EventDataType() cetypes.CloudEventsDataType {
	return workpayload.ManifestBundleEventDataType
}

func (codec *BundleCodec) Encode(source string, eventType cetypes.CloudEventsType, res *api.Resource) (*cloudevents.Event, error) {
	evt, err := api.JSONMAPToCloudEvent(res.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource manifest to cloudevent: %v", err)
	}

	// the resource source takes precedence over the CloudEvent source.
	if res.Source != "" {
		source = res.Source
	}

	evt.SetSource(source)
	evt.SetType(eventType.String())
	evt.SetExtension(cetypes.ExtensionResourceID, res.ID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, int64(res.Version))
	evt.SetExtension(cetypes.ExtensionClusterName, res.ConsumerID)

	if !res.GetDeletionTimestamp().IsZero() {
		evt.SetExtension(cetypes.ExtensionDeletionTimestamp, res.GetDeletionTimestamp().Time)
	}

	return evt, nil
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

	clusterName, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionClusterName])
	if err != nil {
		return nil, fmt.Errorf("failed to get clustername extension: %v", err)
	}

	originalSource, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionOriginalSource])
	if err != nil {
		return nil, fmt.Errorf("failed to get originalsource extension: %v", err)
	}

	status, err := api.CloudEventToJSONMap(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource status: %v", err)
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		Version:    int32(resourceVersionInt),
		Source:     originalSource,
		ConsumerID: clusterName,
		Type:       api.ResourceTypeBundle,
		Status:     status,
	}

	return resource, nil
}
