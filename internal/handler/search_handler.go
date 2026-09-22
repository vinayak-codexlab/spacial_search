package handler

import (
	"context"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"h3-spacial-service/internal/response"
	"h3-spacial-service/internal/service"

	"github.com/gin-gonic/gin"
)

type PropertySearcher interface {
	SearchByHexagons(context.Context, []string, int, int64, int64) ([]bson.M, int64, error)
}

type SearchHandler struct {
	h3Service  *service.H3Service
	repository PropertySearcher
	logger     *log.Logger
}

func NewSearchHandler(h3Service *service.H3Service, repository PropertySearcher, logger *log.Logger) *SearchHandler {
	return &SearchHandler{h3Service: h3Service, repository: repository, logger: logger}
}

func (h *SearchHandler) SearchProperties(c *gin.Context) {
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		response.Error(c, http.StatusBadRequest, "Latitude must be a valid number between -90 and 90")
		return
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		response.Error(c, http.StatusBadRequest, "Longitude must be a valid number between -180 and 180")
		return
	}
	zoom, err := strconv.Atoi(c.DefaultQuery("zoom", "12"))
	if err != nil || zoom < 0 || zoom > 22 {
		response.Error(c, http.StatusBadRequest, "Zoom must be an integer between 0 and 22")
		return
	}
	pageValue := c.Query("page")
	if pageValue == "" {
		pageValue = "1"
	}
	page, err := strconv.ParseInt(pageValue, 10, 64)
	if err == nil && page < 0 {
		page = 1
	}
	if err != nil || page < 1 {
		response.Error(c, http.StatusBadRequest, "Page must be a positive integer")
		return
	}
	limitValue := c.Query("limit")
	if limitValue == "" {
		limitValue = "10"
	}
	limit, err := strconv.ParseInt(limitValue, 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Limit must be an integer")
		return
	}
	if limit < 0 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}
	if limit == 0 || page-1 > math.MaxInt64/limit {
		response.Error(c, http.StatusBadRequest, "Limit must be between 1 and 100 and page must not overflow")
		return
	}
	resolution := h.h3Service.MapZoomResolution(zoom)
	hexagons := h.h3Service.GetTargetHexagons(lat, lng, resolution, 1)
	if len(hexagons) == 0 {
		response.Error(c, http.StatusInternalServerError, "Unable to calculate search area")
		return
	}
	queryCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	listings, total, err := h.repository.SearchByHexagons(queryCtx, hexagons, resolution, page, limit)
	if err != nil {
		h.logger.Printf("property search failed: %v", err)
		response.Error(c, http.StatusInternalServerError, "Unable to search properties")
		return
	}
	if listings == nil {
		listings = []bson.M{}
	}
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	c.JSON(http.StatusOK, response.ListResponse[bson.M]{
		Success: true, Message: "listings fetched successfully",
		Pagination: response.Pagination{Page: page, Limit: limit, Total: total, TotalPages: totalPages},
		Data:       listings,
	})
}
