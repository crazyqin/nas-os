package web

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRateLimitMiddleware_ConcurrentAccess exercises the shipped rate-limit
// middleware under concurrent clients. Run with -race to detect map races.
func TestRateLimitMiddleware_ConcurrentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &SecurityConfig{
		EnableRateLimit: true,
		RateLimitRPS:    10000, // high so we measure concurrency, not 429
		CSRFKey:         []byte("test-csrf-key-for-race"),
	}

	r := gin.New()
	r.Use(rateLimitMiddleware(cfg))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	const goroutines = 32
	const perG = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				req := httptest.NewRequest(http.MethodGet, "/ping", nil)
				// Distinct IPs to stress map insert path
				req.RemoteAddr = "10.0.0." + string(rune('1'+id%9)) + ":1234"
				req.Header.Set("X-Forwarded-For", "192.0.2."+string(rune('1'+id%9)))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != http.StatusOK && w.Code != http.StatusTooManyRequests {
					errCh <- errStatus{code: w.Code}
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

type errStatus struct{ code int }

func (e errStatus) Error() string {
	return "unexpected status"
}
