package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type PropertySearcher interface {
	SearchByHexagons(context.Context, []string, int, int64, int64) ([]bson.M, int64, error)
}

type SearchCache interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
}

type CachedPropertyRepository struct {
	source    PropertySearcher
	cache     SearchCache
	namespace string
	ttl       time.Duration
}

type cachedPage struct {
	Listings []bson.M `json:"listings"`
	Total    int64    `json:"total"`
}

func NewCachedPropertyRepository(source PropertySearcher, cache SearchCache, namespace string, ttl time.Duration) *CachedPropertyRepository {
	return &CachedPropertyRepository{source: source, cache: cache, namespace: namespace, ttl: ttl}
}

func (r *CachedPropertyRepository) key(cells []string, resolution int, page, limit int64) string {
	sorted := slices.Clone(cells)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	return fmt.Sprintf("h3:%s:v3:res%d:%s:page:%d:limit:%d", url.QueryEscape(r.namespace), resolution, strings.Join(sorted, ","), page, limit)
}

func (r *CachedPropertyRepository) SearchByHexagons(ctx context.Context, cells []string, resolution int, page, limit int64) ([]bson.M, int64, error) {
	if resolution < 7 || resolution > 9 || page < 1 || limit < 1 || limit > 100 || page-1 > (1<<63-1)/limit {
		return nil, 0, fmt.Errorf("invalid search parameters")
	}
	key := r.key(cells, resolution, page, limit)
	// Redis failures and malformed cache entries fall through to MongoDB.
	readCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	raw, err := r.cache.Get(readCtx, key)
	cancel()
	if err == nil {
		var result cachedPage
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&result) == nil && result.Listings != nil && result.Total >= 0 {
			return result.Listings, result.Total, nil
		}
	}
	listings, total, err := r.source.SearchByHexagons(ctx, cells, resolution, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if listings == nil {
		listings = []bson.M{}
	}
	if raw, err := json.Marshal(cachedPage{Listings: listings, Total: total}); err == nil {
		writeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_ = r.cache.Set(writeCtx, key, raw, r.ttl)
	}
	return listings, total, nil
}
