# Unit Test and API Response Report

**Generated:** 2026-10-03 (Asia/Kolkata)  
**Endpoint:** `GET /api/v1/listings/search`
**Input:** This GET endpoint has no request body; its payload is query parameters.

The non-H3 unit packages pass. H3 handler/router tests cannot run locally because the H3 binding needs CGO and a C compiler. The following are the exact API request/response contracts asserted in `internal/handler/search_handler_test.go`.

## 1. Valid default search

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=77 HTTP/1.1
```
**Response — 200 OK**
```json
{"success":true,"message":"Data fetched successfully","meta":{"count":0,"resolution_used":"h3_res9","zoom":12,"ring":1,"target_hexes_count":7,"result_limit":250},"data":[]}
```
Response header: `Cache-Control: private, no-store`.

## 2. Valid explicit zoom

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=77&zoom=15&ring=1 HTTP/1.1
```
**Response — 200 OK**
```json
{"success":true,"message":"Data fetched successfully","meta":{"count":0,"resolution_used":"h3_res10","zoom":15,"ring":1,"target_hexes_count":7,"result_limit":250},"data":[]}
```

## 3. Wrong payload: latitude missing

**Request**
```http
GET /api/v1/listings/search?lng=77 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Latitude must be a valid number between -90 and 90"}
```

## 4. Wrong payload: invalid latitude (`NaN` or `91`)

**Request**
```http
GET /api/v1/listings/search?lat=NaN&lng=77 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Latitude must be a valid number between -90 and 90"}
```

## 5. Wrong payload: longitude missing

**Request**
```http
GET /api/v1/listings/search?lat=28 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Longitude must be a valid number between -180 and 180"}
```

## 6. Wrong payload: longitude out of range

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=181 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Longitude must be a valid number between -180 and 180"}
```

## 7. Wrong payload: unsupported zoom

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=77&zoom=23 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Zoom must be an integer between 0 and 22"}
```

## 8. Wrong payload: invalid ring (`1.5`, `-1`, or `26`)

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=77&ring=26 HTTP/1.1
```
**Response — 400 Bad Request**
```json
{"success":false,"message":"Ring must be an integer between 0 and 25"}
```

## 9. Repository/database failure

**Request**
```http
GET /api/v1/listings/search?lat=28&lng=77 HTTP/1.1
```
**Test setup:** repository returns an internal error.
**Response — 500 Internal Server Error**
```json
{"success":false,"message":"Unable to search properties"}
```
Internal database details are not returned to the client.

## 10. MongoDB BSON command too large

**Request:** same as case 9.
**Test setup:** MongoDB error code `10334`.

**Response — 400 Bad Request**
```json
{"success":false,"message":"Search area is too large. Reduce ring size and try again"}
```

## Middleware response tests

| Request | Expected response |
|---|---|
| More than 120 API requests from one source in a minute | `429 Too Many Requests`, `Retry-After: 60` |
| CORS preflight from an untrusted origin | `403 Forbidden`; no `Access-Control-Allow-Origin` |

Run `go test ./...` in CGO-enabled CI to execute the endpoint test file itself.
