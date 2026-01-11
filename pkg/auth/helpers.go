package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/errors"
	maestrologger "github.com/openshift-online/maestro/pkg/logger"
)

func handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, code errors.ServiceErrorCode, reason string) {
	opID := r.Header.Get(maestrologger.OpIDHeader)
	logger := klog.FromContext(ctx)
	err := errors.New(code, "%s", reason)
	if err.HttpCode >= 400 && err.HttpCode <= 499 {
		logger.Info("client error",
			"error", err.Error(),
			"code", err.HttpCode,
			"op-id", opID,
		)
	} else {
		logger.Error(err, "server error",
			"code", err.HttpCode,
			"op-id", opID,
		)
	}

	writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(opID))
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
