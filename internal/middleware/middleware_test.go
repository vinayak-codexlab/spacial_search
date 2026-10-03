package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(2))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		want := http.StatusNoContent
		if i == 2 {
			want = http.StatusTooManyRequests
			if w.Header().Get("Retry-After") != "60" {
				t.Fatalf("missing retry header: %q", w.Header().Get("Retry-After"))
			}
		}
		if w.Code != want {
			t.Fatalf("request %d status=%d, want %d", i+1, w.Code, want)
		}
	}
}

func TestCORSRejectsUntrustedPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.OPTIONS("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("untrusted origin accepted: status=%d headers=%v", w.Code, w.Header())
	}
}
