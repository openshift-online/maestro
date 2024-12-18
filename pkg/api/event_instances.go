package api

type EventInstance struct {
	EventID     string `gorm:"default:null"`
	SpecEventID string `gorm:"default:null"`
	InstanceID  string
}

type EventInstanceList []*EventInstance
