package server

import (
	"strings"

	"github.com/golang/glog"
	"github.com/openshift-online/maestro/pkg/pulseserver"
)

// NewPulseServer returns a new Pulse Server instance. The server is configured based on whether
// consistent hashing is enabled or not.
//
// If consistent hashing is enabled, the server is created with a consistent hashing status dispatcher.
// Otherwise, it is created with a shared subscription.
func NewPulseServer() pulseserver.PulseServer {
	if env().Config.PulseServer.EnableConsistentHashing {
		if strings.HasPrefix(env().Config.MessageBroker.MQTTOptions.Topics.AgentEvents, "$share") {
			glog.Fatalf("The status topic should not be a shared topic when consistent hashing is enabled")
		}
		return pulseserver.NewPulseServerWithStatusDispatcher(
			&env().Database.SessionFactory,
			env().Services.Resources(),
			env().Config.MessageBroker.ClientID,
			env().Config.PulseServer.PulseInterval,
			env().Config.PulseServer.CheckInterval,
			env().Clients.CloudEventsSource)
	} else {
		return pulseserver.NewPulseServerImpl(
			&env().Database.SessionFactory,
			env().Services.Resources(),
			env().Config.MessageBroker.ClientID,
			env().Config.PulseServer.PulseInterval,
			env().Config.PulseServer.CheckInterval,
			env().Clients.CloudEventsSource)
	}
}
