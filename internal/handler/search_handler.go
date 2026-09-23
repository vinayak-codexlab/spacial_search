package handler

import (
	"context"
	"fmt"
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

// SearchTimeout bounds MongoDB work, including fetching all cursor batches.
const SearchTimeout = 30 * time.Second

type PropertySearcher interface {
	SearchByHexagons(context.Context, []string, int) ([]bson.M, error)
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
	// H3 accepts k as a signed 32-bit C integer.
	ring, err := strconv.ParseInt(c.DefaultQuery("ring", "1"), 10, 32)
	if err != nil || ring < 0 {
		response.Error(c, http.StatusBadRequest, "Ring must be a nonnegative integer supported by H3")
		return
	}
	resolution := h.h3Service.MapZoomResolution(zoom)
	hexagons := h.h3Service.GetTargetHexagons(lat, lng, resolution, int(ring))
	if len(hexagons) == 0 {
		response.Error(c, http.StatusInternalServerError, "Unable to calculate search area")
		return
	}
	queryCtx, cancel := context.WithTimeout(c.Request.Context(), SearchTimeout)
	defer cancel()
	listings, err := h.repository.SearchByHexagons(queryCtx, hexagons, resolution)
	if err != nil {
		h.logger.Printf("property search failed: %v", err)
		response.Error(c, http.StatusInternalServerError, "Unable to search properties")
		return
	}
	if listings == nil {
		listings = []bson.M{}
	}
	for i, listing := range listings {
		listings[i] = response.ListingSummary(listing)
	}
	c.JSON(http.StatusOK, response.ListResponse[bson.M]{
		Success: true, Message: "Data fetched successfully",
		Meta: response.SearchMeta{
			Count: len(listings), ResolutionUsed: fmt.Sprintf("h3_res%d", resolution),
			Zoom: zoom, Ring: int(ring), TargetHexesCount: len(hexagons),
		},
		Data: listings,
	})
}
