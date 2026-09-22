package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"h3-spacial-service/internal/response"
	"h3-spacial-service/internal/service"
)

type searchStub struct {
	called             bool
	resolution         int
	err                error
	page, limit, total int64
	listings           []bson.M
}

func (s *searchStub) SearchByHexagons(ctx context.Context, cells []string, resolution int, page, limit int64) ([]bson.M, int64, error) {
	s.called = true
	s.page, s.limit = page, limit
	s.resolution = resolution
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 3*time.Second || len(cells) == 0 {
		return nil, 0, errors.New("missing search area or query deadline")
	}
	return s.listings, s.total, s.err
}

func TestSearchProperties(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, query        string
		status, resolution int
		repoErr            error
	}{
		{"missing latitude", "lng=77", 400, 0, nil},
		{"NaN latitude", "lat=NaN&lng=77", 400, 0, nil},
		{"out of range latitude", "lat=91&lng=77", 400, 0, nil},
		{"missing longitude", "lat=28", 400, 0, nil},
		{"NaN longitude", "lat=28&lng=NaN", 400, 0, nil},
		{"out of range longitude", "lat=28&lng=181", 400, 0, nil},
		{"empty pagination", "lat=28&lng=77&page=&limit=", 200, 8, nil},
		{"empty page", "lat=28&lng=77&page=&limit=10", 200, 8, nil},
		{"empty limit", "lat=28&lng=77&page=1&limit=", 200, 8, nil},
		{"negative page", "lat=28&lng=77&page=-5", 200, 8, nil},
		{"minimum page", "lat=28&lng=77&page=-9223372036854775808", 200, 8, nil},
		{"negative pagination", "lat=28&lng=77&page=-2&limit=-1", 200, 8, nil},
		{"zero page", "lat=28&lng=77&page=0", 400, 0, nil},
		{"invalid page", "lat=28&lng=77&page=abc", 400, 0, nil},
		{"negative limit", "lat=28&lng=77&limit=-1", 200, 8, nil},
		{"zero limit", "lat=28&lng=77&limit=0", 400, 0, nil},
		{"invalid limit", "lat=28&lng=77&limit=abc", 400, 0, nil},
		{"overflow", "lat=28&lng=77&page=9223372036854775807&limit=100", 400, 0, nil},
		{"invalid zoom", "lat=28&lng=77&zoom=abc", 400, 0, nil},
		{"default zoom", "lat=28&lng=77", 200, 8, nil},
		{"low zoom", "lat=28&lng=77&zoom=11", 200, 7, nil},
		{"high zoom", "lat=28&lng=77&zoom=15", 200, 9, nil},
		{"database failure", "lat=28&lng=77", 500, 8, errors.New("database unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchStub{err: tt.repoErr}
			router := gin.New()
			router.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/search?"+tt.query, nil))
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.status, response.Body.String())
			}
			if repo.called != (tt.resolution != 0) || repo.resolution != tt.resolution {
				t.Fatalf("unexpected repository call: %+v", repo)
			}
			var body struct {
				Success    bool                                           `json:"success"`
				Message    string                                         `json:"message"`
				Data       []bson.M                                       `json:"data"`
				Pagination struct{ Page, Limit, Total, TotalPages int64 } `json:"pagination"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Success != (tt.status == 200) || body.Message == "" {
				t.Fatalf("invalid envelope: %s", response.Body.String())
			}
			if tt.status == 200 && (repo.page != 1 || repo.limit != 10 || body.Data == nil || len(body.Data) != 0 || body.Pagination.Page != 1 || body.Pagination.Limit != 10 || body.Pagination.Total != 0 || body.Pagination.TotalPages != 0) {
				t.Fatalf("invalid empty page: %s", response.Body.String())
			}
		})
	}
}

func TestSearchPagination(t *testing.T) {
	repo := &searchStub{total: 21, listings: []bson.M{{"title": "Example", "extra_field": "preserved", "listing_address": bson.M{"custom_format": "any schema"}}}}
	r := gin.New()
	r.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/search?lat=28&lng=77&page=3&limit=10", nil))
	var body response.ListResponse[bson.M]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !body.Success || body.Message != "listings fetched successfully" || body.Pagination.Page != 3 || body.Pagination.Limit != 10 || body.Pagination.Total != 21 || body.Pagination.TotalPages != 3 || len(body.Data) != 1 || body.Data[0]["title"] != "Example" || body.Data[0]["extra_field"] != "preserved" || repo.page != 3 || repo.limit != 10 {
		t.Fatalf("unexpected page: %s", w.Body.String())
	}
}

func TestSearchLimitCap(t *testing.T) {
	for _, limit := range []string{"100", "101", "800", "10000"} {
		t.Run(limit, func(t *testing.T) {
			repo := &searchStub{total: 201}
			r := gin.New()
			r.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/search?lat=28&lng=77&limit="+limit, nil))
			var body response.ListResponse[bson.M]
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || repo.limit != 100 || body.Pagination.Limit != 100 || body.Pagination.TotalPages != 3 {
				t.Fatalf("limit was not capped correctly: %s", w.Body.String())
			}
		})
	}
}
