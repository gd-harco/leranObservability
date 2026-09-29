package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
)

type closeFunc func() error

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}
		return slog.String("error", fmt.Sprintf("%+v", err))
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
