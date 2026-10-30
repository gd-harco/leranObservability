package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestObserverLog(t *testing.T) {
	logBuffer := &bytes.Buffer{}

	logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Time(slog.TimeKey, time.Date(2023, 10, 1, 12, 34, 57, 0, time.UTC))
			}
			if a.Key == "duration" {
				return slog.Attr{}
			}
			return a
		},
	}))

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	loggedHandler := requestObserver(logger)(dummyHandler)

	req := httptest.NewRequest("GET", "http://lin.ko/api/stats", nil)
	rr := httptest.NewRecorder()
	loggedHandler.ServeHTTP(rr, req)

	const expectedLogString = `time=2023-10-01T12:34:57.000Z level=INFO msg="Served request" request_id="" method=GET path=/api/stats client_ip=192.0.2.x request_body_bytes=0 response_status=200 response_body_bytes=0` + "\n"
	const expectedStatusCode = http.StatusOK

	if logBuffer.String() != expectedLogString {
		t.Errorf("Fail! logBuffer.string()=%s, want %s\n", logBuffer.String(), expectedLogString)
	}
	if rr.Code != expectedStatusCode {
		t.Errorf("Fail! rr.Code=%d, want %d\n", rr.Code, expectedStatusCode)
	}
}
