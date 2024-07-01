package api

type EventInstance struct {
	EventID    string
	InstanceID string
}

type EventInstanceList []*EventInstance
