package handlers

import "net/http"

type mockResponseWriter struct {
	written string
	status  int
}

func (m *mockResponseWriter) Header() http.Header {
	return map[string][]string{}
}
func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.written = string(b)
	return 0, nil
}
func (m *mockResponseWriter) WriteHeader(code int) {
	m.status = code
}
