package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"boot.dev/linko/internal/build"
	"boot.dev/linko/internal/linkioerr"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	pkgerr "github.com/pkg/errors"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
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

var sensitive = []string{"password", "key", "apikey", "secret", "pin", "creditcardno", "user"}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if slices.Contains(sensitive, a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	if strings.Contains(a.Key, "url") {
		url, err := url.Parse(a.Value.String())
		if err != nil {
			return a
		}
		if url.User == nil {
			return a
		}
		if pass, exist := url.User.Password(); exist {
			return slog.String(a.Key, strings.Replace(a.Value.String(), pass, "REDACTED", 1))
		}
		return a
	}
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
	if isatty.IsCygwinTerminal(os.Stderr.Fd()) || isatty.IsTerminal(os.Stderr.Fd()) {
		isTty = true
	}
	stderrHandler := tint.NewTextHandler(os.Stderr, &tint.Options{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttr,
		NoColor:     isTty,
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

const logContextKey contextKey = "log_context"

type LogContext struct {
	Error    error
	Username string
}

func logRequest(logger *slog.Logger, r *http.Request, observation requestObservation) {
	attrs := []slog.Attr{
		slog.String("request_id", observation.requestID),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("client_ip", redactIP(r.RemoteAddr)),
		slog.Duration("duration", observation.duration),
		slog.Int("request_body_bytes", observation.requestBodyBytes),
		slog.Int("response_status", observation.statusCode),
		slog.Int("response_body_bytes", observation.responseBodyBytes),
	}
	if logCtx, ok := r.Context().Value(logContextKey).(*LogContext); ok {
		if logCtx.Username != "" {
			attrs = append(attrs, slog.String("user", logCtx.Username))
		}
		if logCtx.Error != nil {
			attrs = append(attrs, slog.Any("error", logCtx.Error))
		}
	}
	logger.LogAttrs(r.Context(), slog.LevelInfo, "Served request", attrs...)
}

func httpError(ctx context.Context, w http.ResponseWriter, err error, status int) {
	toRedactStatus := []int{
		401, 403, 500,
	}
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
	if slices.Contains(toRedactStatus, status) {
		http.Error(w, http.StatusText(status), status)
		return
	}
	http.Error(w, err.Error(), status)
}

func redactIP(ip string) string {
	var host string
	var err error
	if strings.Contains(ip, ":") {
		host, _, err = net.SplitHostPort(ip)
		if err != nil {
			return ip
		}
	} else {
		host = ip
	}

	parsed := net.ParseIP(host)
	if parsed == nil {
		return ip
	}
	if parsed = parsed.To4(); parsed == nil {
		return ip
	}
	result := fmt.Sprintf("%v.%v.%v.%v", parsed[0], parsed[1], parsed[2], "x")
	return result
}
