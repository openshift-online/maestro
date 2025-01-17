package api

import (
	"encoding/json"
	"fmt"

	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"gorm.io/datatypes"

	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"
)

type ResourceBundleStatus struct {
	ObservedVersion int32
	SequenceID      string
	*workpayload.ManifestBundleStatus
}

// DecodeManifestBundle converts a CloudEvent JSONMap representation of a list of resource manifest
// into metadata, a list of manifests, a list of manifest configs, and a delete option for openapi output.
func DecodeManifestBundle(manifest datatypes.JSONMap) (map[string]any, []map[string]any, []map[string]any, map[string]any, error) {
	if len(manifest) == 0 {
		return nil, nil, nil, nil, nil
	}

	evt, err := JSONMAPToCloudEvent(manifest)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to convert resource manifest bundle to cloudevent: %v", err)
	}

	metaData := map[string]any{}
	extensions := evt.Extensions()
	if meta, ok := extensions[codec.ExtensionWorkMeta]; ok {
		metaJson, err := cloudeventstypes.ToString(meta)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to get work meta extension: %v", err)
		}

		if err := json.Unmarshal([]byte(metaJson), &metaData); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to unmarshal work meta extension: %v", err)
		}
	}

	eventPayload := &workpayload.ManifestBundle{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to decode cloudevent payload as resource manifest bundle: %v", err)
	}

	manifests := make([]map[string]interface{}, 0, len(eventPayload.Manifests))
	for _, manifest := range eventPayload.Manifests {
		m := map[string]interface{}{}
		if err := json.Unmarshal(manifest.Raw, &m); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to unmarshal manifest raw in bundle: %v", err)
		}
		manifests = append(manifests, m)
	}
	manifestConfigs := make([]map[string]interface{}, 0, len(eventPayload.ManifestConfigs))
	for _, manifestConfig := range eventPayload.ManifestConfigs {
		mbytes, err := json.Marshal(manifestConfig)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to marshal manifest config in bundle: %v", err)
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal(mbytes, &m); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to unmarshal manifest config in bundle: %v", err)
		}
		manifestConfigs = append(manifestConfigs, m)
	}
	deleteOption := map[string]interface{}{}
	if eventPayload.DeleteOption != nil {
		dbytes, err := json.Marshal(eventPayload.DeleteOption)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to marshal delete option in bundle: %v", err)
		}
		if err := json.Unmarshal(dbytes, &deleteOption); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to unmarshal delete option in bundle: %v", err)
		}
	}

	return metaData, manifests, manifestConfigs, deleteOption, nil
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
