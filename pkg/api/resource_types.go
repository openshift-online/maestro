package api

import (
	"strconv"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type Resource struct {
	Meta
	Version         int32
	ObservedVersion int32
	ConsumerID      string
	Manifest        datatypes.JSONMap
	Status          datatypes.JSONMap
}

type ResourceStatus struct {
	ContentStatus   datatypes.JSONMap
	ReconcileStatus *ReconcileStatus
}

type ReconcileStatus struct {
	ObservedVersion int32
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
