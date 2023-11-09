package api

// ErrorType is the name of the type used to report errors.
const ErrorType = "Error"

// Error represents an error reported by the API.
type Error struct {
	Type   string `json:"type,omitempty"`
	ID     string `json:"id,omitempty"`
	HREF   string `json:"href,omitempty"`
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
}
