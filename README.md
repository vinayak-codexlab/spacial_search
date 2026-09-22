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
- `internal/repository`: MongoDB filtering, count, stable sort and pagination.
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

## API

```sh
curl 'http://localhost:3000/api/v1/listings/search?lat=28.6139&lng=77.2090&zoom=12&page=1&limit=10'
```

Latitude and longitude are required. Zoom defaults to 12 (range 0–22). Missing or empty page defaults to 1; negative page values also reset to 1. Missing or empty limit defaults to 10; values below 0 reset to 10, and values above 100 are capped at 100. Zero and non-integer limits return HTTP 400. Search covers the origin H3 cell and its immediate neighbors. Zoom below 12 uses resolution 7, below 15 uses resolution 8, otherwise resolution 9.

```json
{
  "success": true,
  "message": "listings fetched successfully",
  "pagination": { "page": 1, "limit": 10, "total": 1, "totalPages": 1 },
  "data": [{ "title": "Example listing" }]
}
```

Data includes all fields from matching MongoDB documents, decoded as `bson.M`; no fixed listing schema is enforced. No matches returns `data: []`, `total: 0`, `totalPages: 0`. A page beyond the last page returns an empty array while preserving the total count. Invalid parameters return HTTP 400; database errors return HTTP 500 with a generic message. Error envelopes use `success: false` and `message`.

Results sort by `_id`. Count and fetch are separate operations: concurrent writes may change the count between them. Large offsets may be expensive; cursor pagination is a future option for large datasets.

`GET /health/live` is a process liveness endpoint, not a database readiness probe.

## Operations

Access logs use Morgan-style plain text on stdout: `GET /api/v1/listings/search 200 5.123 ms - 45` (method, path, status, duration and response bytes). Startup prints `DB connected successfully` after the database ping and `Server is running on PORT 3000` after binding the HTTP listener. Query strings, headers and bodies are excluded. Proxy headers are not trusted by default; configure explicit trusted proxy addresses in `internal/router` if deploying behind a proxy.

The server drains requests for up to 10 seconds on SIGINT/SIGTERM, then disconnects MongoDB with a separate timeout. Configure TLS at your ingress/reverse proxy. Authentication and rate limiting depend on deployment requirements and are not implemented here.

Create the following indexes in the configured collection before serving substantial traffic (run via your normal database administration workflow):

```js
db.listings.createIndex({ "h3_res7": 1, "_id": 1 })
db.listings.createIndex({ "h3_res8": 1, "_id": 1 })
db.listings.createIndex({ "h3_res9": 1, "_id": 1 })
```

## Checks

```sh
go test ./...
go vet ./...
```

## MongoDB documents

This service only reads MongoDB. Other services own listing schemas and writes. Search requires the appropriate top-level string field (`h3_res7`, `h3_res8`, or `h3_res9`) for the selected resolution. All matching document fields are returned, regardless of the structure of addresses or other metadata. No migration of address fields is required.

Request latitude and longitude are used to calculate the search cells; stored coordinates are not read to perform the H3 lookup. Pagination still limits the number of returned documents per request.

## Redis search cache

Set `REDIS_URL=redis://localhost:6379/0` to enable caching, or use a `rediss://` URL for TLS. If unset, requests go directly to MongoDB. `SEARCH_CACHE_TTL` defaults to `30s` and must be positive. Reload environment variables and restart after changing configuration.

The cache sorts and deduplicates a copy of the target H3 cells. Keys include the database/collection namespace, schema version, resolution, sorted cells, page and effective limit:

```text
h3:leads%2Flistings:v3:res8:882d538661ffffff,882d538663ffffff:page:1:limit:10
```

Cache hits return stored listings and total counts without running either MongoDB query. Successful results, including empty pages, are cached as JSON. MongoDB errors are never cached. Cache misses, malformed entries, and Redis failures fall back to MongoDB. Redis operations have a 100ms budget and retries are disabled; clients close on shutdown.

Results can be stale until the TTL expires. Future listing write endpoints must invalidate affected cached queries or accept this staleness. Use separate Redis databases for environments that share MongoDB database/collection names but point at different servers. Cache latency depends on network and deployment; sub-millisecond responses are not guaranteed.

Cache version v3 isolates full-document results from older schema-limited entries. Cached JSON uses number-preserving decoding to avoid rounding large integer fields.

## Why test files exist

Go runs `*_test.go` files only with `go test`; they are excluded from the server build and do not run during `go run`. They check pagination defaults, validation, response formatting, logging, cache hits and fallback behavior without connecting to your databases. Keep these regression checks when changing the service.
