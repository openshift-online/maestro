package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/getsentry/sentry-go"
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
		err = fmt.Errorf("cannot send response body for request '%s'", r.URL.Path)
		log.Error(err)
		sentry.CaptureException(err)
		return
	}
}

func SendUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	apiError := errors.Unauthorized(message)
	data, err := json.Marshal(apiError)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	w.WriteHeader(http.StatusUnauthorized)
	_, err = w.Write(data)
	if err != nil {
		err = fmt.Errorf("cannot send response body for request '%s'", r.URL.Path)
		log.Error(err)
		sentry.CaptureException(err)
		return
	}
}

// SendPanic sends a panic error response to the client, but it doesn't end the process.
func SendPanic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(panicBody)
	if err != nil {
		err = fmt.Errorf(
			"cannot send panic response for request '%s': %s",
			r.URL.Path,
			err.Error(),
		)
		log.Error(err)
		sentry.CaptureException(err)
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
		log.Error(err)
		sentry.CaptureException(err)
		os.Exit(1)
	}
}
