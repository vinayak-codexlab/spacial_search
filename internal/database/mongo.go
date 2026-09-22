package database

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"h3-spacial-service/internal/config"
)

// Connect runs once at startup, before any request handling begins.
func Connect(ctx context.Context, cfg *config.Config) (*mongo.Client, error) {
	if cfg.DNSServer != "" {
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialer := net.Dialer{Timeout: 5 * time.Second}
				return dialer.DialContext(ctx, network, cfg.DNSServer)
			},
		}
	}
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI).SetBSONOptions(&options.BSONOptions{DefaultDocumentM: true}))
	if err != nil {
		return nil, fmt.Errorf("MongoDB client configuration failed")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.Disconnect(cleanupCtx)
		return nil, fmt.Errorf("MongoDB ping failed: %w", err)
	}
	return client, nil
}
