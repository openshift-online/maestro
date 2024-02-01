package api

import (
	"encoding/json"
	"strconv"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"

	"github.com/openshift-online/maestro/pkg/errors"
)

type Resource struct {
	Meta
	Version    int32
	ConsumerID string
	Manifest   datatypes.JSONMap
	Status     datatypes.JSONMap
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
	d.ID = NewID()
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

func JSONMapStatusToResourceStatus(jsonMapStatus datatypes.JSONMap) (*ResourceStatus, error) {
	resourceStatusJSON, err := json.Marshal(jsonMapStatus)
	if err != nil {
		return nil, errors.GeneralError("Unable to marshal resource jsonmap status: %s", err)
	}
	resourceStatus := &ResourceStatus{}
	if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
		return nil, errors.GeneralError("Unable to unmarshal resource status: %s", err)
	}

	return resourceStatus, nil
}

func ResourceStatusToJSONMap(status *ResourceStatus) (datatypes.JSONMap, error) {
	resourceStatusJSON, err := json.Marshal(status)
	if err != nil {
		return nil, errors.GeneralError("Unable to marshal resource status: %s", err)
	}
	var resourceStatusJSONMap datatypes.JSONMap
	if err := json.Unmarshal(resourceStatusJSON, &resourceStatusJSONMap); err != nil {
		return nil, errors.GeneralError("Unable to unmarshal resource jsonmap status: %s", err)
	}

	return resourceStatusJSONMap, nil
}
