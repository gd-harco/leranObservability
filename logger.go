package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"boot.dev/linko/internal/linkioerr"
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
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttr,
	})
	if !envSet {
		return slog.New(stderrHandler), func() error { return nil }, nil
	}
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	bufferedFile := bufio.NewWriterSize(logFile, 8192)
	cleanupFunc := func() error {
		if err := bufferedFile.Flush(); err != nil {
			return err
		}
		return logFile.Close()
	}
	fileHandler := slog.NewJSONHandler(bufferedFile, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceAttr,
	})
	return slog.New(slog.NewMultiHandler(stderrHandler, fileHandler)), cleanupFunc, nil
}
