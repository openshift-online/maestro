package constants

const (
	DefaultSourceID = "maestro"

	AuthMethodPassword       = "password" // Standard postgres username/password authentication.
	AuthMethodMicrosoftEntra = "az-entra" // Microsoft Entra ID-based token authentication.

	// MinTokenLifeThreshold defines the minimum remaining lifetime (in seconds) of the access token before
	// it should be refreshed.
	MinTokenLifeThreshold = 60.0

	// Tracing IDs
	// TODO: May need to move to another repo so they can be shareable
	ClusterServiceClusterID = "cs.cluster.id"
	AROCorrelationID        = "aro.correlation.id"
	AROClientRequestID      = "aro.client.request.id"
	ARORequestID            = "aro.request.id"
)
