package requests

// CreateRequest represents the request to create a work
type CreateRequest struct {
	WorkBytes []byte `json:"work"`
}
