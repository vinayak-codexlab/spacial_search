package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"h3-spacial-service/internal/cache"
	"h3-spacial-service/internal/config"
	"h3-spacial-service/internal/database"
	"h3-spacial-service/internal/handler"
	"h3-spacial-service/internal/repository"
	"h3-spacial-service/internal/router"
	"h3-spacial-service/internal/service"
)

func Run(ctx context.Context, cfg *config.Config, logger *log.Logger) error {
	client, err := database.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	logger.Println("DB connected successfully")
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(cleanupCtx); err != nil {
			logger.Printf("MongoDB disconnect failed: %v", err)
		}
	}()
	var repo repository.PropertySearcher = repository.NewPropertyRepository(client.Database(cfg.DBName).Collection(cfg.Collection))
	if cfg.RedisURL != "" {
		redisCache, err := cache.NewRedis(cfg.RedisURL)
		if err != nil {
			return err
		}
		defer redisCache.Close()
		namespace := cfg.DBName + "/" + cfg.Collection
		repo = repository.NewCachedPropertyRepository(repo, redisCache, namespace, cfg.CacheTTL)
		logger.Println("Redis Connected")
	}
	search := handler.NewSearchHandler(service.NewH3Service(), repo, logger)
	server := &http.Server{
		Addr: ":" + cfg.Port, Handler: router.New(search, logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("HTTP listen failed: %w", err)
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	logger.Printf("Server is running on PORT %s", cfg.Port)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-ctx.Done():
		logger.Println("Shutting down HTTP server")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("HTTP shutdown failed: %w", err)
	}
	return nil
}
