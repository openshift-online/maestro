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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ktypes "k8s.io/apimachinery/pkg/types"

	workv1 "open-cluster-management.io/api/work/v1"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
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
	Manifest     datatypes.JSONMap
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
	resJSON, err := res.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSONMAP to cloudevent JSON: %v", err)
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

	return res, nil
}

// EncodeManifest converts resource manifest, deleteOption and updateStrategy (map[string]interface{}) into a CloudEvent JSONMap representation.
func EncodeManifest(manifest, deleteOption, updateStrategy map[string]interface{}) (datatypes.JSONMap, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	delOption := &workv1.DeleteOption{
		PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
	}
	if len(deleteOption) != 0 {
		delOption = &workv1.DeleteOption{}
		deleteOptionBytes, err := json.Marshal(deleteOption)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal deleteOption to json: %v", err)
		}
		err = json.Unmarshal(deleteOptionBytes, delOption)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal json to deleteOption: %v", err)
		}
	}

	upStrategy := &workv1.UpdateStrategy{
		Type: workv1.UpdateStrategyTypeServerSideApply,
	}
	if len(updateStrategy) != 0 {
		upStrategy = &workv1.UpdateStrategy{}
		updateStrategyBytes, err := json.Marshal(updateStrategy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updateStrategy to json: %v", err)
		}
		err = json.Unmarshal(updateStrategyBytes, upStrategy)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal json to updateStrategy: %v", err)
		}
		fmt.Println("upStrategy", upStrategy)
	}

	// create a cloud event with the manifest as the data
	evt := cetypes.NewEventBuilder("maestro", cetypes.CloudEventsType{}).NewEvent()
	eventPayload := &workpayload.Manifest{
		Manifest:     unstructured.Unstructured{Object: manifest},
		DeleteOption: delOption,
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
			UpdateStrategy: upStrategy,
		},
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, eventPayload); err != nil {
		return nil, fmt.Errorf("failed to set cloud event data: %v", err)
	}

	// convert cloudevent to JSONMap
	manifest, err := CloudEventToJSONMap(&evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource manifest: %v", err)
	}

	return manifest, nil
}

// DecodeManifest converts a CloudEvent JSONMap representation of a resource manifest
// into resource manifest, deleteOption and updateStrategy (map[string]interface{}).
func DecodeManifest(manifest datatypes.JSONMap) (map[string]interface{}, map[string]interface{}, map[string]interface{}, error) {
	if len(manifest) == 0 {
		return nil, nil, nil, nil
	}

	evt, err := JSONMAPToCloudEvent(manifest)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to convert resource manifest to cloudevent: %v", err)
	}

	eventPayload := &workpayload.Manifest{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode cloudevent payload as resource manifest: %v", err)
	}

	deleteOptionObj := &map[string]interface{}{}
	if eventPayload.DeleteOption != nil {
		deleteOptionJsonData, err := json.Marshal(eventPayload.DeleteOption)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to marshal deleteOption to json: %v", err)
		}
		if err := json.Unmarshal(deleteOptionJsonData, deleteOptionObj); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal deleteOption to cloudevent: %v", err)
		}
	}

	updateStrategyObj := &map[string]interface{}{}
	if eventPayload.ConfigOption != nil && eventPayload.ConfigOption.UpdateStrategy != nil {
		updateStrategyJsonData, err := json.Marshal(eventPayload.ConfigOption.UpdateStrategy)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to marshal updateStrategy to json: %v", err)
		}
		if err := json.Unmarshal(updateStrategyJsonData, updateStrategyObj); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal updateStrategy to cloudevent: %v", err)
		}
	}

	return eventPayload.Manifest.Object, *deleteOptionObj, *updateStrategyObj, nil
}

// DecodeDeleteOption converts a CloudEvent JSONMap representation of a resoure deleteOption
// into resource deleteOption (map[string]interface{}).
func DecodeDeleteOption(deleteOption datatypes.JSONMap) (map[string]interface{}, error) {
	if len(deleteOption) == 0 {
		return nil, nil
	}

	jsonData, err := deleteOption.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deleteOption to json: %v", err)
	}

	obj := &map[string]interface{}{}
	if err := json.Unmarshal(jsonData, obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deleteOption to cloudevent: %v", err)
	}

	return *obj, nil
}

// DecodeManifestBundle converts a CloudEvent JSONMap representation of a list of resource manifest
// into a list of resource manifest (map[string]interface{}).
func DecodeManifestBundle(manifest datatypes.JSONMap) ([]map[string]interface{}, error) {
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

	manifests := make([]map[string]interface{}, 0, len(eventPayload.Manifests))
	for _, m := range eventPayload.Manifests {
		if len(m.Raw) == 0 {
			return nil, fmt.Errorf("manifest in bundle is empty")
		}
		// unmarshal the raw JSON into the object
		obj := &map[string]interface{}{}
		if err := json.Unmarshal(m.Raw, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest in bundle: %v", err)
		}
		manifests = append(manifests, *obj)
	}

	return manifests, nil
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

	eventPayload := &workpayload.ManifestStatus{}
	if err := evt.DataAs(eventPayload); err != nil {
		return nil, fmt.Errorf("failed to decode cloudevent data as resource status: %v", err)
	}

	if eventPayload.Status != nil {
		resourceStatus.ReconcileStatus.Conditions = eventPayload.Status.Conditions
		for _, value := range eventPayload.Status.StatusFeedbacks.Values {
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
		return nil, fmt.Errorf("failed to marshal resource status to JSON: %v", err)
	}
	statusMap := make(map[string]interface{})
	if err := json.Unmarshal(resourceStatusJSON, &statusMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource status JSON to map: %v", err)
	}

	return statusMap, nil
}
