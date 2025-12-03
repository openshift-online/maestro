package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/openshift-online/maestro/pkg/errors"
	logtracing "github.com/openshift-online/maestro/pkg/logger"
	"k8s.io/klog/v2"
)

func handleError(ctx context.Context, w http.ResponseWriter, code errors.ServiceErrorCode, reason string) {
	logger := klog.FromContext(ctx)

	operationID := logtracing.GetOperationID(ctx)
	err := errors.New(code, "%s", reason)
	if err.HttpCode >= 400 && err.HttpCode <= 499 {
		logger.Info("client error",
			"error", err.Error(),
			"code", err.HttpCode,
			"op-id", operationID,
		)
	} else {
		logger.Error(err, "server error",
			"code", err.HttpCode,
			"op-id", operationID,
		)
	}

	writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(operationID))
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
