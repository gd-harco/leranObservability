package main

import (
	"bufio"
	"io"
	"log/slog"
	"os"
)

type closeFunc func() error

func initializeLogger() (*slog.Logger, closeFunc, error) {
	logFilePath, envSet := os.LookupEnv("LINKO_LOG_FILE")
	if !envSet {
		return slog.New(slog.NewTextHandler(os.Stderr, nil)), func() error { return nil }, nil
	}
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	bufferedFile := bufio.NewWriterSize(logFile, 8192)
	multiWriter := io.MultiWriter(bufferedFile, os.Stderr)
	cleanupFunc := func() error {
		err := bufferedFile.Flush()
		if err != nil {
			return err
		}
		err = logFile.Close()
		if err != nil {
			return err
		}
		return nil
	}
	return slog.New(slog.NewTextHandler(multiWriter, nil)), cleanupFunc, nil
}
