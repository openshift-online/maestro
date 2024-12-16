package constants

const (
	DefaultSourceID = "maestro"

	AuthMethodPassword       = "password" // Standard postgres username/password authentication.
	AuthMethodMicrosoftEntra = "az-entra" // Microsoft Entra ID-based token authentication.

	// MinTokenLifeThreshold defines the minimum remaining lifetime (in seconds) of the access token before
	// it should be refreshed.
	MinTokenLifeThreshold = 60.0
)
