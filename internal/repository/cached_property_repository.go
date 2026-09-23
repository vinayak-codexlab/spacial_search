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
	SearchByHexagons(context.Context, []string, int) ([]bson.M, error)
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

type cachedResults struct {
	Listings []bson.M `json:"listings"`
}

func NewCachedPropertyRepository(source PropertySearcher, cache SearchCache, namespace string, ttl time.Duration) *CachedPropertyRepository {
	return &CachedPropertyRepository{source: source, cache: cache, namespace: namespace, ttl: ttl}
}

func (r *CachedPropertyRepository) key(cells []string, resolution int) string {
	sorted := slices.Clone(cells)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	return fmt.Sprintf("h3:%s:v5:res%d:%s", url.QueryEscape(r.namespace), resolution, strings.Join(sorted, ","))
}

func (r *CachedPropertyRepository) SearchByHexagons(ctx context.Context, cells []string, resolution int) ([]bson.M, error) {
	if resolution < 6 || resolution > 11 {
		return nil, fmt.Errorf("invalid search parameters")
	}
	key := r.key(cells, resolution)
	// Redis failures and malformed cache entries fall through to MongoDB.
	readCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	raw, err := r.cache.Get(readCtx, key)
	cancel()
	if err == nil {
		var result cachedResults
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&result) == nil && result.Listings != nil {
			return result.Listings, nil
		}
	}
	listings, err := r.source.SearchByHexagons(ctx, cells, resolution)
	if err != nil {
		return nil, err
	}
	if listings == nil {
		listings = []bson.M{}
	}
	if raw, err := json.Marshal(cachedResults{Listings: listings}); err == nil {
		writeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_ = r.cache.Set(writeCtx, key, raw, r.ttl)
	}
	return listings, nil
}
