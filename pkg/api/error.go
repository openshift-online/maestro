package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/errors"
)

// SendNotFound sends a 404 response with some details about the non existing resource.
func SendNotFound(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	id := "404"
	reason := fmt.Sprintf(
		"The requested resource '%s' doesn't exist",
		r.URL.Path,
	)
	body := Error{
		Type:   ErrorType,
		ID:     id,
		HREF:   "/api/maestro/v1/errors/" + id,
		Code:   "maestro-" + id,
		Reason: reason,
	}
	data, err := json.Marshal(body)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	w.WriteHeader(http.StatusNotFound)
	_, err = w.Write(data)
	if err != nil {
		logger := klog.FromContext(r.Context())
		logger.Error(err, "cannot send response body for request", "path", r.URL.Path)
		return
	}
}

func SendUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	apiError := errors.Unauthorized("%s", message)
	data, err := json.Marshal(apiError)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	w.WriteHeader(http.StatusUnauthorized)
	_, err = w.Write(data)
	if err != nil {
		logger := klog.FromContext(r.Context())
		logger.Error(err, "cannot send response body for request", "path", r.URL.Path)
		return
	}
}

// SendPanic sends a panic error response to the client, but it doesn't end the process.
func SendPanic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(panicBody)
	if err != nil {
		logger := klog.FromContext(r.Context())
		logger.Error(err, "cannot send response body for request", "path", r.URL.Path)
	}
}

// panicBody is the error body that will be sent when something unexpected happens while trying to
// send another error response. For example, if sending an error response fails because the error
// description can't be converted to JSON.
var panicBody []byte

func init() {
	var err error

	// Create the panic error body:
	panicID := "1000"
	panicError := Error{
		Type: ErrorType,
		ID:   panicID,
		HREF: "/api/maestro/v1/" + panicID,
		Code: "maestro-" + panicID,
		Reason: "An unexpected error happened, please check the log of the service " +
			"for details",
	}

	// Convert it to JSON:
	panicBody, err = json.Marshal(panicError)
	if err != nil {
		err = fmt.Errorf(
			"cannot create the panic error body: %s",
			err.Error(),
		)
		klog.Error(err)
		os.Exit(1)
	}
}
