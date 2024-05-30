package api

import (
	"encoding/json"
	"fmt"

	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"gorm.io/datatypes"

	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

type ResourceBundleStatus struct {
	ObservedVersion int32
	SequenceID      string
	*workpayload.ManifestBundleStatus
}

// DecodeManifestBundle converts a CloudEvent JSONMap representation of a list of resource manifest
// into manifest bundle payload.
func DecodeManifestBundle(manifest datatypes.JSONMap) (*workpayload.ManifestBundle, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	evt, err := JSONMAPToCloudEvent(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource manifest to cloudevent: %v", err)
	}

	eventPayload := &workpayload.ManifestBundle{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent payload as resource manifest bundle: %v", err)
	}

	return eventPayload, nil
}

// DecodeManifestBundleToObjects converts a CloudEvent JSONMap representation of a list of resource manifest
// into a list of resource object (map[string]interface{}).
func DecodeManifestBundleToObjects(manifest datatypes.JSONMap) ([]map[string]interface{}, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	eventPayload, err := DecodeManifestBundle(manifest)
	if err != nil {
		return nil, err
	}

	objects := make([]map[string]interface{}, 0, len(eventPayload.Manifests))
	for _, m := range eventPayload.Manifests {
		if len(m.Raw) == 0 {
			return nil, fmt.Errorf("manifest in bundle is empty")
		}
		// unmarshal the raw JSON into the object
		obj := &map[string]interface{}{}
		if err := json.Unmarshal(m.Raw, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest in bundle: %v", err)
		}
		objects = append(objects, *obj)
	}

	return objects, nil
}

// DecodeBundleStatus converts a CloudEvent JSONMap representation of a resource bundle status
// into resource bundle status (map[string]interface{}) in openapi output.
func DecodeBundleStatus(status datatypes.JSONMap) (map[string]interface{}, error) {
	if len(status) == 0 {
		return nil, nil
	}

	evt, err := JSONMAPToCloudEvent(status)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource bundle status to cloudevent: %v", err)
	}

	evtExtensions := evt.Extensions()
	resourceVersion, err := cloudeventstypes.ToInteger(evtExtensions[cetypes.ExtensionResourceVersion])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
	}

	sequenceID, err := cloudeventstypes.ToString(evtExtensions[cetypes.ExtensionStatusUpdateSequenceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get sequenceid extension: %v", err)
	}

	resourceBundleStatus := &ResourceBundleStatus{
		ObservedVersion: resourceVersion,
		SequenceID:      sequenceID,
	}

	eventPayload := &workpayload.ManifestBundleStatus{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent data as resource bundle status: %v", err)
	}
	resourceBundleStatus.ManifestBundleStatus = eventPayload

	resourceBundleStatusJSON, err := json.Marshal(resourceBundleStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource bundle status to JSON: %v", err)
	}
	statusMap := make(map[string]interface{})
	if err := json.Unmarshal(resourceBundleStatusJSON, &statusMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource bundle status JSON to map: %v", err)
	}

	return statusMap, nil
}
