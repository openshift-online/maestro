package logging

import "net/http"

type LogFormatter interface {
	FormatRequestLog(request *http.Request) (string, error)
	FormatResponseLog(responseInfo *ResponseInfo) (string, error)
}
