package constants

const (
	DefaultSourceID = "maestro"

	AuthMethodPassword       = "password" // Standard postgres username/password authentication.
	AuthMethodMicrosoftEntra = "az-entra" // Microsoft Entra ID-based token authentication.
)
