package cloudevents

import (
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
)

type Codec struct {
	sourceID string
}

var _ cegeneric.Codec[*api.Resource] = &Codec{}

func (codec *Codec) EventDataType() cetypes.CloudEventsDataType {
	return workpayload.ManifestEventDataType
}

func (codec *Codec) Encode(source string, eventType cetypes.CloudEventsType, res *api.Resource) (*cloudevents.Event, error) {
	evt, err := api.JSONMAPToCloudEvent(res.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource payload to cloudevent: %v", err)
	}

	evt.SetSource(source)
	evt.SetType(eventType.String())
	// TODO set resource.Source with a new extension attribute if the agent needs
	evt.SetExtension(cetypes.ExtensionResourceID, res.ID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, int64(res.Version))
	evt.SetExtension(cetypes.ExtensionClusterName, res.ConsumerName)

	if !res.GetDeletionTimestamp().IsZero() {
		evt.SetExtension(cetypes.ExtensionDeletionTimestamp, res.GetDeletionTimestamp().Time)
	}

	return evt, nil
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

	resourceVersion, err := cloudeventstypes.ToInteger(evtExtensions[cetypes.ExtensionResourceVersion])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
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

	if originalSource != codec.sourceID {
		return nil, fmt.Errorf("unmatched original source id %s for resource %s", originalSource, resourceID)
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		Version:      resourceVersion,
		ConsumerName: clusterName,
		Type:         api.ResourceTypeSingle,
		Status:       status,
	}

	return resource, nil
}
