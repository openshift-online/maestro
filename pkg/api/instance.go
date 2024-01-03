package api

type Instance struct {
	Meta
	Name string `json:"name"`
}

type InstanceList []*Instance

// func (i *Instance) BeforeCreate(tx *gorm.DB) error {
// 	i.ID = NewID()
// 	return nil
// }

func (i *Instance) String() string {
	return i.ID
}
