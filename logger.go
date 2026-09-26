package main

import (
	"bufio"
	"io"
	"log"
	"os"
)

type closeFunc func() error

func initializeLogger()  (*log.Logger, closeFunc, error){
	logFilePath, envSet := os.LookupEnv("LINKO_LOG_FILE")
	if !envSet {
		return log.New(os.Stderr, "", log.LstdFlags), func() error {return nil}, nil
	}
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil{
		return nil, nil, err
	}
	bufferedFile := bufio.NewWriterSize(logFile, 8192)
	multiWriter := io.MultiWriter(bufferedFile, os.Stderr)
	cleanupFunc := func() error  {
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
	return log.New(multiWriter, "", log.LstdFlags), cleanupFunc, nil
}
