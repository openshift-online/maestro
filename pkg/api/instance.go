package api

// Instance is used by Maestro for discovering running peers,
// but it is not intended for direct exposure to end users via the API.
type Instance struct {
	Meta
	Name string `json:"name"`
}

type InstanceList []*Instance

// String returns the identifier of the maestro instance.
func (i *Instance) String() string {
	return i.ID
}
