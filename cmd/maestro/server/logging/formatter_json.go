package logging

import (
	"encoding/json"
	"io"
	"k8s.io/klog/v2"
	"net/http"
)

func NewJSONLogFormatter() *jsonLogFormatter {
	return &jsonLogFormatter{}
}

type jsonLogFormatter struct{}

var _ LogFormatter = &jsonLogFormatter{}

func (f *jsonLogFormatter) FormatRequestLog(logger klog.Logger, r *http.Request) (string, error) {
	jsonlog := jsonRequestLog{
		Method:     r.Method,
		RequestURI: r.RequestURI,
		RemoteAddr: r.RemoteAddr,
	}
	if logger.V(4).Enabled() {
		jsonlog.Header = r.Header
		jsonlog.Body = r.Body
	}

	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
	}
	return string(log[:]), nil
}

func (f *jsonLogFormatter) FormatResponseLog(logger klog.Logger, info *ResponseInfo) (string, error) {
	jsonlog := jsonResponseLog{Header: nil, Status: info.Status, Elapsed: info.Elapsed}
	if logger.V(4).Enabled() {
		jsonlog.Body = string(info.Body[:])
	}
	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
	}
	return string(log[:]), nil
}

type jsonRequestLog struct {
	Method     string        `json:"request_method"`
	RequestURI string        `json:"request_url"`
	Header     http.Header   `json:"request_header,omitempty"`
	Body       io.ReadCloser `json:"request_body,omitempty"`
	RemoteAddr string        `json:"request_remote_ip,omitempty"`
}

type jsonResponseLog struct {
	Header  http.Header `json:"response_header,omitempty"`
	Status  int         `json:"response_status,omitempty"`
	Body    string      `json:"response_body,omitempty"`
	Elapsed string      `json:"elapsed,omitempty"`
}
