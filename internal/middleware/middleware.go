package middleware

import (
	"log"
	"net/http"
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
		c.Next()
	}
}
func CORS() gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")

        if origin == "http://localhost:5173" ||
            origin == "https://geoestate-bi8kfodgm-vinayak-codexlab.vercel.app" {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Vary", "Origin")
            c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
            c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        }

        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }

        c.Next()
    }
}
