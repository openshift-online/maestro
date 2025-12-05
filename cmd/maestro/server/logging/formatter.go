package logging

import (
	"k8s.io/klog/v2"
	"net/http"
)

type LogFormatter interface {
	FormatRequestLog(logger klog.Logger, request *http.Request) (string, error)
	FormatResponseLog(logger klog.Logger, responseInfo *ResponseInfo) (string, error)
}
