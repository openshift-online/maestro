package api

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"gorm.io/datatypes"

	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// ResourceBundleStatus defines resource bundle status
type ResourceBundleStatus struct {
	ObservedVersion int32
	SequenceID      string
	*workpayload.ManifestBundleStatus
}

// ManifestBundleWrapper is a wrapper used for openapi output that contains:
// * metadata - manifestwork metadata
// * manifests - resource manifests
// * manifest configs - manifest configs
// * delete option - delete option
type ManifestBundleWrapper struct {
	Meta            map[string]interface{}
	Manifests       []map[string]interface{}
	ManifestConfigs []map[string]interface{}
	DeleteOption    map[string]interface{}
}

// DecodeManifestBundle converts a CloudEvent JSONMap representation of a list of resource manifests
// into manifests and manifestconfigs that will be used in openapi output.
func DecodeManifestBundle(manifestBundle datatypes.JSONMap) (*ManifestBundleWrapper, error) {
	if len(manifestBundle) == 0 {
		return nil, nil
	}

	evt, err := JSONMAPToCloudEvent(manifestBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource manifest bundle to cloudevent: %v", err)
	}

	metaData := map[string]any{}
	extensions := evt.Extensions()
	if meta, ok := extensions[types.ExtensionWorkMeta]; ok {
		metaJson, err := cloudeventstypes.ToString(meta)
		if err != nil {
			return nil, fmt.Errorf("failed to get work meta extension: %v", err)
		}

		if err := json.Unmarshal([]byte(metaJson), &metaData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal work meta extension: %v", err)
		}
	}

	eventPayload := &workpayload.ManifestBundle{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent payload: %v", err)
	}

	manifests := make([]map[string]interface{}, 0, len(eventPayload.Manifests))
	for _, manifest := range eventPayload.Manifests {
		m := map[string]interface{}{}
		if err := json.Unmarshal(manifest.Raw, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest raw: %v", err)
		}
		manifests = append(manifests, m)
	}
	manifestConfigs := make([]map[string]interface{}, 0, len(eventPayload.ManifestConfigs))
	for _, manifestConfig := range eventPayload.ManifestConfigs {
		mbytes, err := json.Marshal(manifestConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest config: %v", err)
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal(mbytes, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest config: %v", err)
		}
		manifestConfigs = append(manifestConfigs, m)
	}
	deleteOption := map[string]interface{}{}
	if eventPayload.DeleteOption != nil {
		dbytes, err := json.Marshal(eventPayload.DeleteOption)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal delete option: %v", err)
		}
		if err := json.Unmarshal(dbytes, &deleteOption); err != nil {
			return nil, fmt.Errorf("failed to unmarshal delete option: %v", err)
		}
	}

	return &ManifestBundleWrapper{
		Meta:            metaData,
		Manifests:       manifests,
		ManifestConfigs: manifestConfigs,
		DeleteOption:    deleteOption,
	}, nil
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
	resourceVersion, err := cloudeventstypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
	if err != nil {
		return nil, fmt.Errorf("failed to get resourceversion extension: %v", err)
	}

	sequenceID, err := cloudeventstypes.ToString(evtExtensions[types.ExtensionStatusUpdateSequenceID])
	if err != nil {
		return nil, fmt.Errorf("failed to get sequenceid extension: %v", err)
	}

	resourceBundleStatus := &ResourceBundleStatus{
		ObservedVersion:      resourceVersion,
		SequenceID:           sequenceID,
		ManifestBundleStatus: &workpayload.ManifestBundleStatus{},
	}
	if err := evt.DataAs(resourceBundleStatus.ManifestBundleStatus); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent payload: %v", err)
	}
	resourceBundleStatusJSON, err := json.Marshal(resourceBundleStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource status: %v", err)
	}
	resourceBundleStatusMap := make(map[string]interface{})
	if err := json.Unmarshal(resourceBundleStatusJSON, &resourceBundleStatusMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource status: %v", err)
	}

	return resourceBundleStatusMap, nil
}

// JSONMAPToCloudEvent converts a JSONMap (resource manifest or status) to a CloudEvent
func JSONMAPToCloudEvent(jsonmap datatypes.JSONMap) (*cloudevents.Event, error) {
	if len(jsonmap) == 0 {
		return nil, fmt.Errorf("failed to convert empty jsonmap to cloudevent")
	}

	var err error
	var resJSON []byte

	if metadata, ok := jsonmap[types.ExtensionWorkMeta]; ok {
		// cloudevents require its extension value as string, so we need convert the metadata object
		// to string back

		// ensure the original resource will be not changed
		resCopy := datatypes.JSONMap{}
		for key, value := range jsonmap {
			resCopy[key] = value
		}

		metaJson, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata to JSON: %v", err)
		}

		resCopy[types.ExtensionWorkMeta] = string(metaJson)

		resJSON, err = resCopy.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSONMAP to cloudevent JSON: %v", err)
		}
	} else {
		resJSON, err = jsonmap.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSONMAP to cloudevent JSON: %v", err)
		}
	}

	evt := &cloudevents.Event{}
	if err := json.Unmarshal(resJSON, evt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSONMAP to cloudevent: %v", err)
	}

	return evt, nil
}

// CloudEventToJSONMap converts a CloudEvent to a JSONMap (resource manifest or status)
func CloudEventToJSONMap(evt *cloudevents.Event) (datatypes.JSONMap, error) {
	evtJSON, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cloudevent to JSONMAP: %v", err)
	}

	var res datatypes.JSONMap
	if err := res.UnmarshalJSON(evtJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cloudevent JSON to JSONMAP: %v", err)
	}

	if metadata, ok := res[types.ExtensionWorkMeta]; ok {
		// cloudevents treat its extension value as string, so we need convert metadata extension
		// to an object for supporting to query the resources with metadata
		objectMeta := map[string]any{}

		if err := json.Unmarshal([]byte(fmt.Sprintf("%s", metadata)), &objectMeta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata extension to object: %v", err)
		}
		res[types.ExtensionWorkMeta] = objectMeta
	}

	return res, nil
}
