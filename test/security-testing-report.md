# Security Test Report

**Project:** h3-spacial-service  
**Generated:** 2026-10-03 (Asia/Kolkata)  
**Assessment status:** Partial verification completed; no full H3-dependent build or vulnerability-database scan was possible locally.

## Automated checks

| Check | Command | Result |
|---|---|---|
| Unit tests for non-H3 packages | `go test ./internal/cache ./internal/config ./internal/database ./internal/middleware ./internal/repository ./internal/response` | Passed |
| Static analysis for non-H3 packages | `go vet ./internal/cache ./internal/config ./internal/database ./internal/middleware ./internal/repository ./internal/response` | Passed |
| Dependency checksum integrity | `go mod verify` | Passed: all modules verified |
| Race detection | `go test -race ./internal/middleware` | Blocked: `-race` requires CGO |
| Full unit/static suite | `go test ./...`, `go vet ./...` | Blocked: H3 binding requires CGO and `gcc` is unavailable |
| Known-vulnerability database scan | `govulncheck` | Not run: tool is not installed |
| Go SAST scan | `gosec` | Not run: tool is not installed |

## Verified controls

| Area | Evidence | Result |
|---|---|---|
| Input validation | Latitude, longitude, zoom, and ring are parsed as bounded numeric values before H3 or MongoDB work. | Pass |
| Query injection | MongoDB filter field names are derived from a validated resolution range; values are internally generated H3 strings. | Pass |
| Resource exhaustion | Ring radius is limited to 25, MongoDB results to 250, cursor batches to 100, request headers to 8 KiB, and database work has a deadline. | Pass |
| Abuse protection | `/api/v1` has a bounded per-client 120 request/minute limiter; its behavior is covered by a unit test. | Pass |
| Browser boundary | Exact-origin CORS allowlist, rejected untrusted preflights, `nosniff`, frame denial, CSP, referrer, permissions, and opener policies. | Pass |
| Sensitive query caching | Search responses send `Cache-Control: private, no-store`. | Pass |
| Error disclosure | Client errors are generic; request logs exclude query strings, headers, and bodies. | Pass |

## Release conditions and residual risks

1. **Required before release:** run `go test ./...`, `go vet ./...`, and `go test -race ./...` in a CI image with CGO, a C compiler, and the required H3 native dependencies.
2. **Required before release:** run `govulncheck ./...` and `gosec ./...` in CI, and fail the build for actionable findings.
3. **Deployment decision:** the API has no application authentication. This is acceptable only if the projected listing data is intentionally public. Otherwise enforce authentication/authorization at the ingress or application layer before deployment.
4. **Deployment check:** Gin deliberately trusts no proxy headers. If deployed behind a reverse proxy, configure only that proxy's fixed addresses as trusted; otherwise the rate limiter will treat all forwarded traffic as the proxy IP.

## Recommended CI gate

```text
go mod verify
go vet ./...
go test ./...
go test -race ./...
govulncheck ./...
gosec ./...
```
