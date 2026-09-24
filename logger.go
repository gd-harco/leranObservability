package main

import (
	"io"
	"log"
	"os"
)

/*
Add an initializeLogger helper. If LINKO_LOG_FILE is set, it should create a logger that writes to both the file and STDERR,
 otherwise, it should create one that only writes to STDERR.
Use this logger for all logging in the application, removing the old loggers entirely.
Remove both the DEBUG: and the INFO: prefixes from the logger.
*/

func initializeLogger() *log.Logger{
	logFilePath, envSet := os.LookupEnv("LINKO_LOG_FILE")
	if !envSet {
		return log.New(os.Stderr, "", log.LstdFlags)
	}
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil{
		log.Fatalf("failed to open log file: %v", err)
	}
	multiWriter := io.MultiWriter(logFile, os.Stderr)
	return log.New(multiWriter, "", log.LstdFlags)
}
