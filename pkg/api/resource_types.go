package api

import (
	"encoding/json"
	"fmt"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"

	workv1 "open-cluster-management.io/api/work/v1"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"
)

type ResourceType string

const (
	ResourceTypeSingle ResourceType = "Single"
	ResourceTypeBundle ResourceType = "Bundle"
)

type Resource struct {
	Meta
	Version      int32
	Source       string
	ConsumerName string
	Type         ResourceType
	Payload      datatypes.JSONMap
	Status       datatypes.JSONMap
	// Name must be unique and not null, it can be treated as the resource external ID.
	// The format of the name should be follow the RFC 1123 (same as the k8s namespace).
	// When creating a resource, if its name is not specified, the resource id will be used as its name.
	// Cannot be updated.
	Name string
}

type ResourceStatus struct {
	ContentStatus   datatypes.JSONMap
	ReconcileStatus *ReconcileStatus
}

type ReconcileStatus struct {
	ObservedVersion int32
	SequenceID      string
	Conditions      []metav1.Condition
}

type ResourceList []*Resource
type ResourceIndex map[string]*Resource

func (l ResourceList) Index() ResourceIndex {
	index := ResourceIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Resource) BeforeCreate(tx *gorm.DB) error {
	// generate a new ID if it doesn't exist
	if d.ID == "" {
		d.ID = NewID()
	}
	if d.Name == "" {
		d.Name = d.ID
	}
	// start the resource version from 1
	if d.Version == 0 {
		d.Version = 1
	}
	return nil
}

func (d *Resource) GetUID() ktypes.UID {
	return ktypes.UID(d.Meta.ID)
}

func (d *Resource) GetResourceVersion() string {
	return strconv.FormatInt(int64(d.Version), 10)
}

func (d *Resource) GetDeletionTimestamp() *metav1.Time {
	return &metav1.Time{Time: d.Meta.DeletedAt.Time}
}

type ResourcePatchRequest struct{}

// JSONMAPToCloudEvent converts a JSONMap (resource manifest or status) to a CloudEvent
func JSONMAPToCloudEvent(res datatypes.JSONMap) (*cloudevents.Event, error) {
	var err error
	var resJSON []byte

	if metadata, ok := res[codec.ExtensionWorkMeta]; ok {
		// cloudevents require its extension value as string, so we need convert the metadata object
		// to string back

		// ensure the original resource will be not changed
		resCopy := datatypes.JSONMap{}
		for key, value := range res {
			resCopy[key] = value
		}

		metaJson, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata to JSON: %v", err)
		}

		resCopy[codec.ExtensionWorkMeta] = string(metaJson)

		resJSON, err = resCopy.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSONMAP to cloudevent JSON: %v", err)
		}
	} else {
		resJSON, err = res.MarshalJSON()
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

	if metadata, ok := res[codec.ExtensionWorkMeta]; ok {
		// cloudevents treat its extension value as string, so we need convert metadata extension
		// to an object for supporting to query the resources with metadata
		objectMeta := map[string]any{}

		if err := json.Unmarshal([]byte(fmt.Sprintf("%s", metadata)), &objectMeta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata extension to object: %v", err)
		}
		res[codec.ExtensionWorkMeta] = objectMeta
	}

	return res, nil
}

// EncodeManifest converts resource manifest, deleteOption and manifestConfig (map[string]interface{}) into a CloudEvent JSONMap representation.
func EncodeManifest(manifest, deleteOption, manifestConfig map[string]interface{}) (datatypes.JSONMap, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifestConfig to json: %v", err)
	}
	mc := workv1.ManifestConfigOption{}
	err = json.Unmarshal(manifestConfigBytes, &mc)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json to manifestConfig: %v", err)
	}

	// set default update strategy to ServerSideApply if not provided
	if mc.UpdateStrategy == nil {
		mc.UpdateStrategy = &workv1.UpdateStrategy{
			Type: workv1.UpdateStrategyTypeServerSideApply,
		}
	}

	// set default feedback rule to the whole status if not provided
	if len(mc.FeedbackRules) == 0 {
		mc.FeedbackRules = []workv1.FeedbackRule{
			{
				Type: workv1.JSONPathsType,
				JsonPaths: []workv1.JsonPath{
					{
						Name: "status",
						Path: ".status",
					},
				},
			},
		}
	}

	// default delete option is Foreground
	delOption := &workv1.DeleteOption{
		PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
	}

	// set delete option to Orphan if update strategy is ReadOnly
	if mc.UpdateStrategy.Type == workv1.UpdateStrategyTypeReadOnly {
		delOption = &workv1.DeleteOption{
			PropagationPolicy: workv1.DeletePropagationPolicyTypeOrphan,
		}
	} else {
		// override delete option if provided
		if len(deleteOption) != 0 {
			deleteOptionBytes, err := json.Marshal(deleteOption)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal deleteOption to json: %v", err)
			}
			err = json.Unmarshal(deleteOptionBytes, delOption)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal json to deleteOption: %v", err)
			}
		}
	}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %v", err)
	}

	// construct the event payload
	eventPayload := &workpayload.ManifestBundle{
		Manifests: []workv1.Manifest{
			{
				RawExtension: runtime.RawExtension{Raw: manifestBytes},
			},
		},
		DeleteOption:    delOption,
		ManifestConfigs: []workv1.ManifestConfigOption{mc},
	}

	// create a cloud event with the manifest as the data
	evt := cetypes.NewEventBuilder("maestro", cetypes.CloudEventsType{}).NewEvent()
	if err := evt.SetData(cloudevents.ApplicationJSON, eventPayload); err != nil {
		return nil, fmt.Errorf("failed to set cloud event data: %v", err)
	}

	// convert cloudevent to JSONMap
	manifest, err = CloudEventToJSONMap(&evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource manifest: %v", err)
	}

	return manifest, nil
}

// DecodeManifest converts a CloudEvent JSONMap representation of a resource manifest
// into resource manifest, deleteOption and manifestConfig (map[string]interface{}) for openapi output.
func DecodeManifest(manifest datatypes.JSONMap) (map[string]interface{}, map[string]interface{}, map[string]interface{}, error) {
	if len(manifest) == 0 {
		return nil, nil, nil, nil
	}

	evt, err := JSONMAPToCloudEvent(manifest)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to convert resource manifest to cloudevent: %v", err)
	}

	eventPayload := &workpayload.ManifestBundle{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode cloudevent payload as resource manifest: %v", err)
	}

	deleteOptionObj := map[string]interface{}{}
	if eventPayload.DeleteOption != nil {
		deleteOptionBytes, err := json.Marshal(eventPayload.DeleteOption)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to marshal deleteOption to json: %v", err)
		}
		if err := json.Unmarshal(deleteOptionBytes, &deleteOptionObj); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal deleteOption to cloudevent: %v", err)
		}
	}
	manifestObj := map[string]interface{}{}
	if len(eventPayload.Manifests) != 1 {
		return nil, nil, nil, fmt.Errorf("invalid number of manifests in the event payload: %d", len(eventPayload.Manifests))
	}
	if err := json.Unmarshal(eventPayload.Manifests[0].Raw, &manifestObj); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmarshal manifest raw to manifest: %v", err)
	}
	if len(eventPayload.ManifestConfigs) != 1 {
		return nil, nil, nil, fmt.Errorf("invalid number of manifestConfigs in the event payload: %d", len(eventPayload.ManifestConfigs))
	}
	manifestConfig := map[string]interface{}{}
	manifestConfigBytes, err := json.Marshal(eventPayload.ManifestConfigs[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal manifestConfig to json: %v", err)
	}
	if err := json.Unmarshal(manifestConfigBytes, &manifestConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmarshal manifestConfig json to object: %v", err)
	}

	return manifestObj, deleteOptionObj, manifestConfig, nil
}

// DecodeStatus converts a CloudEvent JSONMap representation of a resource status
// into resource status (map[string]interface{}).
func DecodeStatus(status datatypes.JSONMap) (map[string]interface{}, error) {
	if len(status) == 0 {
		return nil, nil
	}

	evt, err := JSONMAPToCloudEvent(status)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource status to cloudevent: %v", err)
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

	resourceStatus := &ResourceStatus{
		ReconcileStatus: &ReconcileStatus{
			ObservedVersion: resourceVersion,
			SequenceID:      sequenceID,
		},
	}

	eventPayload := &workpayload.ManifestBundleStatus{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
	}

	if len(eventPayload.ResourceStatus) != 1 {
		return nil, fmt.Errorf("invalid number of resource status in the event payload: %d", len(eventPayload.ResourceStatus))
	}
	resourceStatus.ReconcileStatus.Conditions = eventPayload.ResourceStatus[0].Conditions
	for _, value := range eventPayload.ResourceStatus[0].StatusFeedbacks.Values {
		if value.Name == "status" {
			contentStatus := make(map[string]interface{})
			if err := json.Unmarshal([]byte(*value.Value.JsonRaw), &contentStatus); err != nil {
				return nil, fmt.Errorf("failed to convert status feedback value to content status: %v", err)
			}
			resourceStatus.ContentStatus = contentStatus
		}
	}

	resourceStatusJSON, err := json.Marshal(resourceStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource status to JSON: %v", err)
	}
	statusMap := make(map[string]interface{})
	if err := json.Unmarshal(resourceStatusJSON, &statusMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource status JSON to object: %v", err)
	}

	return statusMap, nil
}
