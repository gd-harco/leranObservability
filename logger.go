package main

import (
	"bufio"
	"log/slog"
	"os"
)

type closeFunc func() error


func initializeLogger() (*slog.Logger, closeFunc, error) {
	logFilePath, envSet := os.LookupEnv("LINKO_LOG_FILE")
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
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
	fileHandler := slog.NewTextHandler(bufferedFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(slog.NewMultiHandler(stderrHandler, fileHandler)), cleanupFunc, nil
}
