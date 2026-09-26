package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)


func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {

	appLogger, cleanup, err := initializeLogger()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up logger: %v\n", err)
		}
	}()

	st, err := store.New(dataDir, appLogger)
	if err != nil {
		appLogger.Printf("failed to create store: %v", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, appLogger )
	var serverErr error
	go func() {
		appLogger.Printf("Linko is running on http://localhost:%d", httpPort)
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		appLogger.Printf("failed to shutdown server: %v", err)
		return 1
	}
	if serverErr != nil {
		appLogger.Printf("server error: %v", serverErr)
		return 1
	}
	return 0
}
