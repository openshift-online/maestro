package api

import "time"

// ServerInstance is employed by Maestro to discover active server instances. The updatedAt field
// determines the liveness of the instance; if the instance remains unchanged for three consecutive
// check intervals (30 seconds by default), it is marked as dead.
// However, it is not meant for direct exposure to end users through the API.
type ServerInstance struct {
	Meta
	LastPulse time.Time // LastPulse indicates the last time the instance pulsed.
	Ready     bool      // Ready indicates whether the instance is ready to serve requests.
}

type ServerInstanceList []*ServerInstance

// String returns the identifier of the maestro instance.
func (i *ServerInstance) String() string {
	return i.ID
}
