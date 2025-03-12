package constants

const (
	DefaultSourceID = "maestro"

	AuthMethodPassword       = "password" // Standard postgres username/password authentication.
	AuthMethodMicrosoftEntra = "az-entra" // Microsoft Entra ID-based token authentication.

	// MinTokenLifeThreshold defines the minimum remaining lifetime (in seconds) of the access token before
	// it should be refreshed.
	MinTokenLifeThreshold = 60.0

	// Tracing IDs
	// TODO: Move to another repo so they can be shareable in the future
	ClusterServiceClusterID = "cs.cluster.uid"
	AROCorrelationID        = "aro.correlation_id"
	AROClientRequestID      = "aro.client.request_id"
	ARORequestID            = "aro.request_id"
)
