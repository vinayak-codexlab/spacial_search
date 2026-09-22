package router

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"h3-spacial-service/internal/handler"
	"h3-spacial-service/internal/middleware"
	"h3-spacial-service/internal/response"
)

func New(search *handler.SearchHandler, logger *log.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true
	r.Use(middleware.RequestLogger(logger), middleware.Recovery(logger), middleware.SecurityHeaders())
	r.NoRoute(func(c *gin.Context) { response.Error(c, http.StatusNotFound, "Route not found") })
	r.NoMethod(func(c *gin.Context) { response.Error(c, http.StatusMethodNotAllowed, "Method not allowed") })
	r.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true, "message": "Service is running"}) })
	v1 := r.Group("/api/v1")
	v1.GET("/listings/search", search.SearchProperties)
	return r
}
