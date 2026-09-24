package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uber/h3-go/v4"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"h3-spacial-service/internal/response"
	"h3-spacial-service/internal/service"
)

type searchStub struct {
	called     bool
	resolution int
	err        error
	cells      []string
	listings   []bson.M
}

func (s *searchStub) SearchByHexagons(ctx context.Context, cells []string, resolution int) ([]bson.M, error) {
	s.called = true
	s.cells = cells
	s.resolution = resolution
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > SearchTimeout || time.Until(deadline) < SearchTimeout-time.Second || len(cells) == 0 {
		return nil, errors.New("missing search area or query deadline")
	}
	return s.listings, s.err
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
		{"negative ring", "lat=28&lng=77&ring=-1", 400, 0, nil},
		{"fractional ring", "lat=28&lng=77&ring=1.5", 400, 0, nil},
		{"invalid ring", "lat=28&lng=77&ring=abc", 400, 0, nil},
		{"empty ring", "lat=28&lng=77&ring=", 400, 0, nil},
		{"overflow ring", "lat=28&lng=77&ring=2147483648", 400, 0, nil},
		{"invalid zoom", "lat=28&lng=77&zoom=abc", 400, 0, nil},
		{"default zoom", "lat=28&lng=77", 200, 9, nil},
		{"low zoom", "lat=28&lng=77&zoom=11", 200, 8, nil},
		{"high zoom", "lat=28&lng=77&zoom=15", 200, 10, nil},
		{"database failure", "lat=28&lng=77", 500, 9, errors.New("database unavailable")},
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
				Success bool     `json:"success"`
				Message string   `json:"message"`
				Data    []bson.M `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Success != (tt.status == 200) || body.Message == "" {
				t.Fatalf("invalid envelope: %s", response.Body.String())
			}
			if tt.status == 200 && (body.Data == nil || len(body.Data) != 0) {
				t.Fatalf("invalid empty page: %s", response.Body.String())
			}
		})
	}
}

func TestSearchReturnsAllMatches(t *testing.T) {
	listings := make([]bson.M, 125)
	for i := range listings {
		listings[i] = bson.M{"listing_details": bson.M{"listing_name": "Example"}, "extra_field": "private"}
	}
	repo := &searchStub{listings: listings}
	r := gin.New()
	r.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/search?lat=28&lng=77&page=3&limit=1", nil))
	var body response.ListResponse[bson.M]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Count != 125 || body.Meta.ResolutionUsed != "h3_res9" || body.Meta.Zoom != 12 || body.Meta.Ring != 1 || body.Meta.TargetHexesCount != 7 || body.Message != "Data fetched successfully" {
		t.Fatalf("unexpected metadata: %+v", body)
	}
	if w.Code != 200 || !body.Success || len(body.Data) != 125 || body.Data[0]["title"] != "Example" || body.Data[0]["extra_field"] != nil {
		t.Fatalf("unexpected result: %s", w.Body.String())
	}
	origin, err := h3.LatLngToCell(h3.NewLatLng(28, 77), 9)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, cell := range repo.cells {
		distance, err := origin.GridDistance(h3.CellFromString(cell))
		if err != nil || distance > 1 || seen[cell] {
			t.Fatalf("invalid neighbor %s: %v", cell, err)
		}
		seen[cell] = true
	}
	if len(seen) != 7 || !seen[origin.String()] {
		t.Fatalf("expected origin and six neighbors: %v", repo.cells)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["pagination"]; ok {
		t.Fatal("unexpected pagination")
	}
}

func TestSearchRingMetadata(t *testing.T) {
	for _, k := range []int{0, 1, 2, 3, 6} {
		t.Run(fmt.Sprint(k), func(t *testing.T) {
			repo := &searchStub{}
			r := gin.New()
			r.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/search?lat=28&lng=77&zoom=15&ring=%d", k), nil))
			var body response.ListResponse[bson.M]
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			want := 1 + 3*k*(k+1)
			if w.Code != 200 || body.Message != "Data fetched successfully" || body.Meta.Count != 0 || body.Meta.ResolutionUsed != "h3_res10" || body.Meta.Zoom != 15 || body.Meta.Ring != k || body.Meta.TargetHexesCount != want || len(repo.cells) != want || body.Data == nil {
				t.Fatalf("unexpected ring response: %s", w.Body.String())
			}
			origin, err := h3.LatLngToCell(h3.NewLatLng(28, 77), 10)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, cell := range repo.cells {
				distance, err := origin.GridDistance(h3.CellFromString(cell))
				if err != nil || distance > k || seen[cell] {
					t.Fatalf("invalid cell %s: %v", cell, err)
				}
				seen[cell] = true
			}
			if !seen[origin.String()] {
				t.Fatal("origin missing")
			}
		})
	}
}

func TestSearchErrorResponses(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		err         error
		status      int
		message     string
		called      bool
	}{
		{"ring over limit", "ring=101", nil, 400, "Ring size must be an integer between 0 and 100", false},
		{"huge ring", "ring=2147483647", nil, 400, "Ring size must be an integer between 0 and 100", false},
		{"invalid zoom", "zoom=23", nil, 400, "Zoom must be an integer between 0 and 22", false},
		{"oversized BSON", "ring=1", fmt.Errorf("find listings: %w", mongo.CommandError{Code: 10334, Name: "BSONObjectTooLarge", Message: "private database details"}), 400, "Search area is too large. Reduce ring and try again", true},
		{"other Mongo error", "ring=1", mongo.CommandError{Code: 13, Message: "private database details"}, 500, "Unable to search properties", true},
		{"untyped error", "ring=1", errors.New("BSONObjectTooLarge"), 500, "Unable to search properties", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchStub{err: tt.err}
			r := gin.New()
			r.GET("/search", NewSearchHandler(service.NewH3Service(), repo, log.New(io.Discard, "", 0)).SearchProperties)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/search?lat=28&lng=77&"+tt.query, nil))
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tt.status || len(body) != 2 || body["success"] != false || body["message"] != tt.message || repo.called != tt.called {
				t.Fatalf("unexpected response: status=%d body=%s repository called=%v", w.Code, w.Body.String(), repo.called)
			}
		})
	}
}
