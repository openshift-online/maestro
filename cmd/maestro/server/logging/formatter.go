package logging

import (
	"net/http"

	"k8s.io/klog/v2"
)

type LogFormatter interface {
	FormatRequestLog(logger klog.Logger, request *http.Request) (string, error)
	FormatResponseLog(logger klog.Logger, responseInfo *ResponseInfo) (string, error)
}
