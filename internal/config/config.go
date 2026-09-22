package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port       string
	MongoURI   string
	DBName     string
	Collection string
	DNSServer  string
	RedisURL   string
	CacheTTL   time.Duration
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:       env("PORT", "3000"),
		MongoURI:   os.Getenv("MONGO_URI"),
		DBName:     env("DB_NAME", os.Getenv("DB_Name")),
		Collection: env("MONGO_COLLECTION", "listings"),
		DNSServer:  os.Getenv("DNS_SERVER"),
		RedisURL:   os.Getenv("REDIS_URL"),
	}
	ttl, err := time.ParseDuration(env("SEARCH_CACHE_TTL", "30s"))
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("SEARCH_CACHE_TTL must be a positive duration, for example 30s")
	}
	cfg.CacheTTL = ttl
	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI is required")
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("DB_NAME is required (DB_Name is also supported)")
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if cfg.DNSServer != "" {
		host, port, err := net.SplitHostPort(cfg.DNSServer)
		n, parseErr := strconv.Atoi(port)
		if err != nil || net.ParseIP(host) == nil || parseErr != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("DNS_SERVER must be an IP and port, for example 8.8.8.8:53")
		}
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
