package api

import "github.com/google/uuid"

func NewID() string {
	// resource id will be the k8s resource ".metadata.name",
	// it must be validated with following regex expression:
	// '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'
	// here use uuid as resource id because ksuid is not a valid k8s resource name
	return uuid.NewString()
}
