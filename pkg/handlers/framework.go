package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/errors"
	loggertracing "github.com/openshift-online/maestro/pkg/logger"
)

// handlerConfig defines the common things each REST controller must do.
// The corresponding handle() func runs the basic handlerConfig.
// This is not meant to be an HTTP framework or anything larger than simple CRUD in handlers.
//
//	MarshalInto is a pointer to the object to hold the unmarshaled JSON.
//	Validate is a list of validation function that run in order, returning fast on the first error.
//	Action is the specific logic a handler must take (e.g, find an object, save an object)
//	ErrorHandler is the way errors are returned to the client
type handlerConfig struct {
	MarshalInto  interface{}
	Validate     []validate
	Action       httpAction
	ErrorHandler errorHandlerFunc
}

type validate func() *errors.ServiceError
type errorHandlerFunc func(ctx context.Context, w http.ResponseWriter, err *errors.ServiceError)
type httpAction func() (interface{}, *errors.ServiceError)

// actionOutcome is a helper struct for deferring finalizing of transactions.
type actionOutcome struct {
	result     any
	serviceErr *errors.ServiceError
}

// finalizeTransaction rolls back or commits the transaction stored in ctx and writes the HTTP response.
//
// If a panic was raised then the transaction is rolled back and the panic is re-raised.
// If actionOutcome.serviceErr is not nil then the transaction is rolled back.
// If the attempt to commit the transaction fails, then actionOutcome.serviceErr is set to the resolve error.
func finalizeTransaction(ctx context.Context, w http.ResponseWriter, r *http.Request, cfg *handlerConfig, httpStatus int, outcome *actionOutcome) {
	if p := recover(); p != nil {
		db.MarkForRollback(ctx, fmt.Errorf("panic: %v", p))
		_ = db.Resolve(ctx)
		panic(p)
	}

	if outcome.serviceErr != nil {
		db.MarkForRollback(ctx, outcome.serviceErr)
	}
	if resolveErr := db.Resolve(ctx); resolveErr != nil && outcome.serviceErr == nil {
		outcome.serviceErr = errors.GeneralError("Error committing transaction: %v", resolveErr)
	}

	switch {
	case outcome.serviceErr != nil:
		cfg.ErrorHandler(r.Context(), w, outcome.serviceErr)
	default:
		writeJSONResponse(w, httpStatus, outcome.result)
	}
}

func handleError(ctx context.Context, w http.ResponseWriter, err *errors.ServiceError) {
	operationID := loggertracing.GetOperationID(ctx)
	logger := klog.FromContext(ctx)
	// If this is a 400 error, its the user's issue, log as info rather than error
	if err.HttpCode >= 400 && err.HttpCode <= 499 {
		logger.Info("user request error", "error", err)
	} else {
		logger.Error(err, "user request error")
	}
	writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(operationID))
}

func handle(w http.ResponseWriter, r *http.Request, cfg *handlerConfig, httpStatus int, session db.SessionFactory) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = handleError
	}

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(r.Context(), w, errors.MalformedRequest("Unable to read request body: %s", err))
		return
	}

	err = json.Unmarshal(bytes, &cfg.MarshalInto)
	if err != nil {
		handleError(r.Context(), w, errors.MalformedRequest("Invalid request format: %s", err))
		return
	}

	for _, v := range cfg.Validate {
		err := v()
		if err != nil {
			cfg.ErrorHandler(r.Context(), w, err)
			return
		}
	}

	// Create a new Context with the transaction stored in it.
	ctx, err := db.NewContext(r.Context(), session)
	if err != nil {
		cfg.ErrorHandler(r.Context(), w, errors.GeneralError("Could not create database transaction: %v", err))
		return
	}

	*r = *r.WithContext(ctx)

	// Resolve transaction once work is complete
	outcome := &actionOutcome{}
	defer finalizeTransaction(ctx, w, r, cfg, httpStatus, outcome)

	outcome.result, outcome.serviceErr = cfg.Action()
}

func handleDelete(w http.ResponseWriter, r *http.Request, cfg *handlerConfig, httpStatus int, session db.SessionFactory) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = handleError
	}
	for _, v := range cfg.Validate {
		err := v()
		if err != nil {
			cfg.ErrorHandler(r.Context(), w, err)
			return
		}
	}

	// Create a new Context with the transaction stored in it.
	ctx, err := db.NewContext(r.Context(), session)
	if err != nil {
		cfg.ErrorHandler(r.Context(), w, errors.GeneralError("Could not create database transaction: %v", err))
		return
	}

	*r = *r.WithContext(ctx)

	// Resolve transaction once work is complete
	outcome := &actionOutcome{}
	defer finalizeTransaction(ctx, w, r, cfg, httpStatus, outcome)

	outcome.result, outcome.serviceErr = cfg.Action()
}

func handleGet(w http.ResponseWriter, r *http.Request, cfg *handlerConfig) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = handleError
	}

	result, serviceErr := cfg.Action()
	switch {
	case serviceErr == nil:
		writeJSONResponse(w, http.StatusOK, result)
	default:
		cfg.ErrorHandler(r.Context(), w, serviceErr)
	}
}

func handleList(w http.ResponseWriter, r *http.Request, cfg *handlerConfig) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = handleError
	}

	results, serviceError := cfg.Action()
	if serviceError != nil {
		cfg.ErrorHandler(r.Context(), w, serviceError)
		return
	}
	writeJSONResponse(w, http.StatusOK, results)
}
