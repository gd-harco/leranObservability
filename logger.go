package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	"boot.dev/linko/internal/build"
	"boot.dev/linko/internal/linkioerr"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	pkgerr "github.com/pkg/errors"
)

type stackTracer interface {
	error
	StackTrace() pkgerr.StackTrace
}

type multiError interface {
	error
	Unwrap() []error
}

var (
	env         = os.Getenv("ENV")
	hostname, _ = os.Hostname()
)

type closeFunc func() error

func getAllAttr(err error) []slog.Attr {
	innerAttr := linkioerr.Attrs(err)
	innerAttr = append(innerAttr, slog.Attr{
		Key:   "message",
		Value: slog.StringValue(err.Error()),
	})
	if stackErr, ok := errors.AsType[stackTracer](err); ok {
		attributs := slog.Attr{
			Key:   "stack_trace",
			Value: slog.StringValue(fmt.Sprintf("%+v", stackErr.StackTrace())),
		}
		innerAttr = append(innerAttr, attributs)
	}
	return innerAttr
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}
		if multiErr, ok := errors.AsType[multiError](err); ok {
			var attributs []slog.Attr
			for i, current := range multiErr.Unwrap() {
				inner := linkioerr.Attrs(current)
				attributs = append(attributs, slog.GroupAttrs(fmt.Sprintf("error_%d", i+1), inner...))
			}
			return slog.GroupAttrs("errors", attributs...)
		}
		return slog.GroupAttrs("error", getAllAttr(err)...)
	}
	return a
}

func initializeLogger() (*slog.Logger, closeFunc, error) {
	logFilePath, envSet := os.LookupEnv("LINKO_LOG_FILE")
	isTty := false
	if isatty.IsCygwinTerminal(os.Stderr.Fd()) || isatty.IsTerminal(os.Stderr.Fd()){
		isTty = true
	}
	stderrHandler := tint.NewTextHandler(os.Stderr, &tint.Options{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttr,
		NoColor: isTty,
	})
	if !envSet {
		return slog.New(stderrHandler), func() error { return nil }, nil
	}
	logger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    1,
		MaxAge:     28,
		MaxBackups: 10,
		LocalTime:  false,
		Compress:   true,
	}

	cleanupFunc := func() error {
		return logger.Close()
	}
	fileHandler := slog.NewJSONHandler(logger, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceAttr,
	})
	return slog.New(
		slog.NewMultiHandler(stderrHandler, fileHandler)).With(
		slog.String("git_sha", build.GitSHA),
		slog.String("build_time", build.BuildTime),
		slog.String("env", env),
		slog.String("hostname", hostname),
	), cleanupFunc, nil
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

func (w *spyResponseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesSent += n
	return n, err
}

func (w *spyResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

const logContextKey contextKey = "log_context"

type LogContext struct {
	Error    error
	Username string
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			contextStruc := &LogContext{}
			r = r.WithContext(context.WithValue(r.Context(), logContextKey, contextStruc))
			spyReader := &spyReadCloser{ReadCloser: r.Body}
			r.Body = spyReader
			spyWriter := &spyResponseWriter{ResponseWriter: w}
			w = spyWriter
			next.ServeHTTP(w, r)
			contextAttr := []slog.Attr{
				slog.String("request_id", w.Header().Get("X-Request-ID")),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("client_ip", r.RemoteAddr),
				slog.Duration("duration", time.Since(startTime)),
				slog.Int("request_body_bytes", spyReader.bytesRead),
				slog.Int("response_status", spyWriter.statusCode),
				slog.Int("response_body_bytes", spyWriter.bytesSent),
			}
			if contextStruc.Username != "" {
				contextAttr = append(contextAttr, slog.String("user", contextStruc.Username))
			}
			if contextStruc.Error != nil {
				contextAttr = append(contextAttr, slog.Any("error", contextStruc.Error))
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "Served request", contextAttr...)
		})
	}
}

func httpError(ctx context.Context, w http.ResponseWriter, err error, status int) {
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
	http.Error(w, err.Error(), status)
}
