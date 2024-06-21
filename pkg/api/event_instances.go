package api

type EventInstance struct {
	EventID    string
	InstanceID string
	Done       bool
}

type EventInstanceList []*EventInstance
