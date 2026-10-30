package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type requestObservation struct {
	requestID         string
	duration          time.Duration
	requestBodyBytes  int
	statusCode        int
	responseBodyBytes int
}

func requestObserver(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			r = r.WithContext(context.WithValue(r.Context(), logContextKey, &LogContext{}))
			reader := &spyReadCloser{ReadCloser: r.Body}
			r.Body = reader
			writer := &spyResponseWriter{ResponseWriter: w}

			next.ServeHTTP(writer, r)

			observation := requestObservation{
				requestID:         writer.Header().Get("X-Request-ID"),
				duration:          time.Since(start),
				requestBodyBytes:  reader.bytesRead,
				statusCode:        writer.status(),
				responseBodyBytes: writer.bytesSent,
			}
			logRequest(logger, r, observation)
			recordHTTPMetrics(r, observation.statusCode)
		})
	}
}

type spyReadCloser struct {
	io.ReadCloser
	bytesRead int
}

func (r *spyReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.bytesRead += n
	return n, err
}

type spyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytesSent  int
}

func (w *spyResponseWriter) status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *spyResponseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesSent += n
	return n, err
}

func (w *spyResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}
	w.ResponseWriter.WriteHeader(statusCode)
	// Informational responses do not commit the final response status.
	if statusCode >= 200 || statusCode == http.StatusSwitchingProtocols {
		w.statusCode = statusCode
	}
}

func (w *spyResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
