package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterMiddleware(t *testing.T) {
	for _, tt := range []struct {
		name, method, path string
		status             int
	}{
		{"health", "GET", "/health/live?secret=hidden", 200},
		{"not found", "GET", "/missing", 404},
		{"method", "POST", "/health/live", 405},
		{"panic", "GET", "/panic", 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			r := New(nil, log.New(&logs, "", 0))
			r.GET("/panic", func(c *gin.Context) { panic("private panic detail") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var body struct {
				Success bool
				Message string
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Success != (tt.status == 200) || body.Message == "" {
				t.Fatalf("bad envelope: %s", w.Body.String())
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing security headers")
			}
			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			pattern := fmt.Sprintf(`^%s %s %d [0-9]+\.[0-9]{3} ms - %d$`, tt.method, regexp.QuoteMeta(strings.Split(tt.path, "?")[0]), tt.status, w.Body.Len())
			if !regexp.MustCompile(pattern).MatchString(lines[len(lines)-1]) {
				t.Fatalf("bad access log: %s", logs.String())
			}

			if strings.Contains(logs.String(), "secret") || strings.Contains(w.Body.String(), "private panic detail") {
				t.Fatal("private details leaked")
			}
		})
	}
}
