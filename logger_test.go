package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/*
Replace t.Skip with two checks. If either check fails, use t.Errorf to report the failure with a helpful message.

    Compare logBuffer.String() to the expected log string.
    Compare rr.Code to the expected status code.

*/

func Test_requestLogger(t *testing.T) {
	logBuffer := &bytes.Buffer{}

	logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Time(slog.TimeKey, time.Date(2023, 10, 1, 12, 34, 57, 0, time.UTC))
			}
			return a
		},
	}))

	requestLoggerMiddleware := requestLogger(logger)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	loggedHandler := requestLoggerMiddleware(dummyHandler)

	req := httptest.NewRequest("GET", "http://lin.ko/api/stats", nil)
	rr := httptest.NewRecorder()
	loggedHandler.ServeHTTP(rr, req)

	const expectedLogString = `time=2023-10-01T12:34:57.000Z level=INFO msg="Served request" method=GET path=/api/stats client_ip=192.0.2.1:1234` + "\n"
	const expectedStatusCode = http.StatusOK

	// Compare logBuffer.String() to the expected log string.
	// Compare rr.Code to the expected status code.
	if logBuffer.String() != expectedLogString {
		t.Errorf("Fail! logBuffer.string()=%s, want %s\n", logBuffer.String(), expectedLogString)
	}
	if rr.Code != expectedStatusCode {
		t.Errorf("Fail! rr.Code=%d, want %d\n", rr.Code, expectedStatusCode)
	}
}
