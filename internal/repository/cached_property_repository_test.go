package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type memoryCache struct {
	entries        map[string][]byte
	getErr, setErr error
	ttl            time.Duration
	writes         int
}

func (c *memoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	if b, ok := c.entries[key]; ok {
		return b, nil
	}
	return nil, errors.New("miss")
}
func (c *memoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.writes++
	c.ttl = ttl
	if c.setErr != nil {
		return c.setErr
	}
	c.entries[key] = value
	return nil
}

type sourceStub struct {
	calls int
	err   error
	empty bool
}

func (s *sourceStub) SearchByHexagons(context.Context, []string, int) ([]bson.M, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return nil, nil
	}
	return []bson.M{{"title": "Example", "h3_res8": "abc", "custom": bson.M{"rooms": 3}, "large_number": int64(9007199254740993)}}, nil
}

func TestCachedSearch(t *testing.T) {
	ctx := context.Background()
	source := &sourceStub{}
	cache := &memoryCache{entries: map[string][]byte{}}
	repo := NewCachedPropertyRepository(source, cache, "leads/listings", 30*time.Second)
	cells := []string{"b", "a"}
	var firstJSON string
	for _, order := range [][]string{cells, {"a", "b"}} {
		listings, err := repo.SearchByHexagons(ctx, order, 8)
		if err != nil || len(listings) != 1 || listings[0]["h3_res8"] != "abc" {
			t.Fatalf("bad result: %v %v", listings, err)
		}
		raw, marshalErr := json.Marshal(listings)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if firstJSON == "" {
			firstJSON = string(raw)
		} else if string(raw) != firstJSON {
			t.Fatalf("cache changed arbitrary fields or number precision: %s vs %s", firstJSON, raw)
		}

	}
	if source.calls != 1 || cache.writes != 1 || cache.ttl != 30*time.Second || cells[0] != "b" {
		t.Fatal("cache did not reuse sorted key, preserve input or apply TTL")
	}
	if _, err := repo.SearchByHexagons(ctx, cells, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SearchByHexagons(ctx, []string{"c"}, 8); err != nil {
		t.Fatal(err)
	}
	other := NewCachedPropertyRepository(source, cache, "other/listings", 30*time.Second)
	if _, err := other.SearchByHexagons(ctx, cells, 8); err != nil {
		t.Fatal(err)
	}
	if source.calls != 4 {
		t.Fatalf("cache keys collided: %d calls", source.calls)
	}
}

func TestCacheFallback(t *testing.T) {
	for _, mode := range []string{"read failure", "write failure", "corrupt", "database failure", "empty"} {
		t.Run(mode, func(t *testing.T) {
			source := &sourceStub{}
			cache := &memoryCache{entries: map[string][]byte{}}
			repo := NewCachedPropertyRepository(source, cache, "test", time.Second)
			switch mode {
			case "read failure":
				cache.getErr = errors.New("offline")
			case "write failure":
				cache.setErr = errors.New("offline")
			case "corrupt":
				cache.entries[repo.key([]string{"a"}, 8)] = []byte(`{"listings":`)
			case "database failure":
				source.err = errors.New("db failed")
			case "empty":
				source.empty = true
			}
			listings, err := repo.SearchByHexagons(context.Background(), []string{"a"}, 8)
			if mode == "database failure" {
				if !errors.Is(err, source.err) || cache.writes != 0 {
					t.Fatal("database errors must not be cached")
				}
			} else if err != nil || listings == nil || source.calls != 1 {
				t.Fatalf("fallback failed: %v", err)
			}
			if mode == "empty" {
				listings, err := repo.SearchByHexagons(context.Background(), []string{"a"}, 8)
				if err != nil || len(listings) != 0 || source.calls != 1 {
					t.Fatal("empty result was not cached")
				}
			}
		})
	}
}
