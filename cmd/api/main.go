package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"h3-spacial-service/internal/app"
	"h3-spacial-service/internal/config"
)

func main() {
	logger := log.New(os.Stdout, "", 0)
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Printf("invalid configuration: %v", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Printf("application stopped: %v", err)
		os.Exit(1)
	}
}
