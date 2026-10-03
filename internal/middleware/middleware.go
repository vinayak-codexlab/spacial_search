package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"h3-spacial-service/internal/response"
)

// RequestLogger is the Morgan equivalent: one access log per request.
// Query strings and headers are deliberately excluded from access logs.
func RequestLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}
		logger.Printf("%s %s %d %.3f ms - %d", c.Request.Method, c.Request.URL.EscapedPath(),
			c.Writer.Status(), float64(time.Since(start))/float64(time.Millisecond), size)
	}
}

func Recovery(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				logger.Printf("request panic: %s %s", c.Request.Method, c.Request.URL.EscapedPath())
				response.Error(c, http.StatusInternalServerError, "Internal server error")
			}
		}()
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Next()
	}
}

type clientWindow struct {
	started time.Time
	count   int
}

// RateLimit applies a bounded, per-client fixed window. It deliberately uses
// ClientIP with Gin's trusted proxies disabled, so forwarded headers cannot be
// used to evade it. Configure the ingress IPs explicitly before trusting them.
func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	if requestsPerMinute < 1 {
		requestsPerMinute = 120
	}
	const maxClients = 10_000
	var mu sync.Mutex
	clients := make(map[string]clientWindow)
	return func(c *gin.Context) {
		now := time.Now()
		client := c.ClientIP()
		mu.Lock()
		entry, exists := clients[client]
		if !exists || now.Sub(entry.started) >= time.Minute {
			if !exists && len(clients) >= maxClients {
				for key, candidate := range clients {
					if now.Sub(candidate.started) >= time.Minute {
						delete(clients, key)
					}
				}
			}
			if !exists && len(clients) >= maxClients {
				mu.Unlock()
				response.Error(c, http.StatusTooManyRequests, "Too many requests")
				return
			}
			entry = clientWindow{started: now}
		}
		entry.count++
		clients[client] = entry
		allowed := entry.count <= requestsPerMinute
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			response.Error(c, http.StatusTooManyRequests, "Too many requests")
			return
		}
		c.Next()
	}
}
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin == "http://localhost:5173" ||
			origin == "https://geoestate-teal.vercel.app" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || c.GetHeader("Access-Control-Request-Method") == "" {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			if origin != "http://localhost:5173" && origin != "https://geoestate-teal.vercel.app" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
