# H3 spatial search API

## Structure

- `cmd/api/main.go`: entry point, logger, configuration, OS signals.
- `internal/app`: dependency wiring, HTTP timeouts, graceful shutdown.
- `internal/config`: environment configuration and validation.
- `internal/database`: MongoDB connection, startup ping, optional DNS override.
- `internal/router`: route registration and global middleware.
- `internal/middleware`: plain-text access logging, panic recovery, security headers.
- `internal/handler`: request validation and HTTP responses.
- `internal/service`: H3 search-area calculation.
- `internal/repository`: MongoDB H3 filtering and fetching all matching listings.
- `internal/response`: response envelope types.

## Configuration and startup

Use `.env.example` as a template; keep credentials out of source control. The app reads process environment variables; it does not automatically load `.env`.

```sh
set -a
source .env
set +a
go run ./cmd/api
```

`MONGO_URI` and `DB_NAME` are required. Legacy `DB_Name` is accepted; `DB_NAME` takes precedence. `PORT` defaults to `3000`, and `MONGO_COLLECTION` defaults to `listings`. System DNS is used unless `DNS_SERVER` is set (for example `8.8.8.8:53`).

For the onboarding listings database, set `DB_NAME=listingOnboarding` and `MONGO_COLLECTION=listings`. Mongoose's `model("Listings", listingOnboardingSchema)` uses the lowercase collection `listings` by default. If the schema explicitly sets `collection: "Listings"`, set `MONGO_COLLECTION=Listings` instead. MongoDB collection names are case-sensitive. Reload `.env` and restart the server after changing these values; an already-exported `DB_NAME` overrides legacy `DB_Name`.

Search failures log the database, collection, failed operation, and underlying error on the server. Check that log for permissions, connectivity, or query timeouts when the API returns `Unable to search properties`.

## API

```sh
curl 'http://localhost:3000/api/v1/listings/search?lat=28.6139&lng=77.2090&zoom=12&ring=1'
```

Latitude and longitude are required. Zoom defaults to 12 (range 0–22). Search calculates the origin H3 cell, calls `GridDisk(ring)` for all cells within `ring` grid steps of the center, and queries the matching `h3_resN` field using `$in`. Zoom 0–4 uses resolution 6, 5–8 uses 7 (city/district), 9–11 uses 8 (locality/neighborhood), 12–14 uses 9 (street/block), 15–17 uses 10, and 18–22 uses 11 (detailed property area). Zoom 0 is retained as a supported overview level. `ring` defaults to 1 and accepts integers from 0 to 100: 0 searches only the center, 1 includes its immediate neighbors, 2 includes two rings, and so on. For a regular hexagonal grid the cell count is `1 + 3*k*(k + 1)` (1, 7, 19, 37, ...); pentagons can reduce it. `meta.target_hexes_count` reports the actual generated cell count. All matching listing summaries are returned; `page` and `limit` are ignored and the response has no pagination metadata.

```json
{
  "success": true,
  "message": "Data fetched successfully",
  "meta": { "count": 1, "resolution_used": "h3_res9", "zoom": 12, "ring": 1, "target_hexes_count": 7 },
  "data": [{ "title": "Example listing" }]
}
```

MongoDB projects only the fields needed for listing summaries. Each response item contains `_id`, `listing_id`, `title`, `listing_type`, `coverImageKey`, `price`, `currency`, `status`, `lat`, `lng`, `locality`, `city`, `h3_res7`, `h3_res8`, `h3_res9`, `bhk`, `area`, `area_unit`, and `furnishing`. Nested listing details, address, and property price are flattened; listing type, area unit, and furnishing are uppercase. Missing currency defaults to `INR`. Missing text fields are empty strings and missing numeric fields are null. No matches returns HTTP 200 with `data: []` and `meta.count: 0`. `meta.count` is the number of returned listings; metadata also reports the effective zoom, ring, and H3 field. Invalid parameters return HTTP 400. Ring is capped before H3 allocation at 100 (at most 30,301 cells) to bound search-area memory and query size. MongoDB BSONObjectTooLarge errors return HTTP 400 with `Search area is too large. Reduce ring and try again`; other database errors return HTTP 500 with a generic message. Error envelopes use `success: false` and `message`.

Search uses a single `find` cursor with no count, sort, skip, or limit. All cursor batches are read within a 30-second query deadline; the HTTP write timeout is 40 seconds. A timeout is reported as a database failure, not an empty result. Result order is unspecified.

`GET /health/live` is a process liveness endpoint, not a database readiness probe.

## Operations

Access logs use Morgan-style plain text on stdout: `GET /api/v1/listings/search 200 5.123 ms - 45` (method, path, status, duration and response bytes). Startup prints `DB connected successfully` after the database ping and `Server is running on PORT 3000` after binding the HTTP listener. Query strings, headers and bodies are excluded. Proxy headers are not trusted by default; configure explicit trusted proxy addresses in `internal/router` if deploying behind a proxy.

The server drains requests for up to 10 seconds on SIGINT/SIGTERM, then disconnects MongoDB with a separate timeout. Configure TLS at your ingress/reverse proxy. Authentication and rate limiting depend on deployment requirements and are not implemented here.

Create the following indexes in the configured collection before serving substantial traffic (run via your normal database administration workflow):

```js
const listings = db.getSiblingDB("listingOnboarding").getCollection("Listings") // use the exact configured collection name
listings.createIndex({ "h3_res6": 1 })
listings.createIndex({ "h3_res7": 1 })
listings.createIndex({ "h3_res8": 1 })
listings.createIndex({ "h3_res9": 1 })
listings.createIndex({ "h3_res10": 1 })
listings.createIndex({ "h3_res11": 1 })
```

## Checks

```sh
go test ./...
go vet ./...
```

## MongoDB documents

This service only reads MongoDB. Other services own listing schemas and writes. Search requires the appropriate top-level string field (`h3_res6` through `h3_res11`) for the selected resolution. Populate the new `h3_res6`, `h3_res10`, and `h3_res11` fields in the listing writer or backfill them from coordinates; this service does not generate stored fields or indexes. Documents missing the selected field will not match. Summary fields are read from `listing_details`, `listing_address`, and `commercial_details.property_price`, alongside the projected top-level identifiers, currency, cover image, and H3 cells.

Request latitude and longitude are used to calculate the search cells; stored coordinates are not read to perform the H3 lookup.

## Redis search cache

Set `REDIS_URL=redis://localhost:6379/0` to enable caching, or use a `rediss://` URL for TLS. If unset, requests go directly to MongoDB. `SEARCH_CACHE_TTL` defaults to `30s` and must be positive. Reload environment variables and restart after changing configuration.

The cache sorts and deduplicates a copy of the target H3 cells. Keys include the database/collection namespace, schema version, resolution, sorted cells:

```text
h3:listingOnboarding%2Flistings:v5:res8:882d538661ffffff,882d538663ffffff
```

Cache hits return all stored matching listings without querying MongoDB. Successful results, including empty results, are cached as JSON. MongoDB errors are never cached. Cache misses, malformed entries, and Redis failures fall back to MongoDB. Redis operations have a 100ms budget and retries are disabled; clients close on shutdown.

Results can be stale until the TTL expires. Future listing write endpoints must invalidate affected cached queries or accept this staleness. Use separate Redis databases for environments that share MongoDB database/collection names but point at different servers. Cache latency depends on network and deployment; sub-millisecond responses are not guaranteed.

Cache version v5 isolates complete results from older paginated entries. Cached JSON uses number-preserving decoding to avoid rounding large integer fields.

## Why test files exist

Go runs `*_test.go` files only with `go test`; they are excluded from the server build and do not run during `go run`. They check the center and six neighbors, returning all matches, validation, response formatting, logging, cache hits and fallback behavior without connecting to your databases. Keep these regression checks when changing the service.
