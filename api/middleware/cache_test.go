package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMemoryCache(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, 5*time.Minute)

	// Test Set and Get
	cache.Set(ctx, "test_key", []byte("test_value"), time.Minute)
	val, ok := cache.Get(ctx, "test_key")
	assert.True(t, ok)
	assert.Equal(t, []byte("test_value"), val)

	// Test Delete
	cache.Delete(ctx, "test_key")
	_, ok = cache.Get(ctx, "test_key")
	assert.False(t, ok)

	// Test Clear
	cache.Set(ctx, "key1", []byte("value1"), time.Minute)
	cache.Set(ctx, "key2", []byte("value2"), time.Minute)
	cache.Clear(ctx)
	_, ok1 := cache.Get(ctx, "key1")
	_, ok2 := cache.Get(ctx, "key2")
	assert.False(t, ok1)
	assert.False(t, ok2)
}

func TestMemoryCacheExpiry(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, 10*time.Millisecond)

	cache.Set(ctx, "expiring_key", []byte("value"), 10*time.Millisecond)

	// Should be available immediately
	_, ok := cache.Get(ctx, "expiring_key")
	assert.True(t, ok)

	// Wait for expiry
	time.Sleep(50 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get(ctx, "expiring_key")
	assert.False(t, ok)
}

func TestMemoryCacheDeleteByPrefix(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(100, time.Minute)

	cache.Set(ctx, "api:users:1", []byte("user1"), time.Minute)
	cache.Set(ctx, "api:users:2", []byte("user2"), time.Minute)
	cache.Set(ctx, "api:volumes:1", []byte("vol1"), time.Minute)

	cache.DeleteByPrefix(ctx, "api:users:")

	_, ok1 := cache.Get(ctx, "api:users:1")
	_, ok2 := cache.Get(ctx, "api:users:2")
	_, ok3 := cache.Get(ctx, "api:volumes:1")

	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.True(t, ok3)
}

func TestCacheMiddleware(t *testing.T) {
	// Setup
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:     cache,
		DefaultTTL:  time.Minute,
		EnableETag:  true,
		GETOnly:     true,
		SkipPaths:   []string{"/health"},
		Enabled:     true,
		KeyPrefix:   "test:",
		StatusCodes: []int{200},
	}

	router := gin.New()
	router.Use(CacheMiddleware(config))

	callCount := 0
	router.GET("/api/data", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{
			"count": callCount,
			"data":  "test",
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request - cache miss
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "MISS", w1.Header().Get("X-Cache"))
	assert.Contains(t, w1.Body.String(), `"count":1`)

	// Second request - cache hit
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "HIT", w2.Header().Get("X-Cache"))
	assert.Contains(t, w2.Body.String(), `"count":1`) // Should still be 1 (cached)

	// Health endpoint should not be cached
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.NotEqual(t, "HIT", w3.Header().Get("X-Cache"))
}

func TestCacheMiddlewareETag(t *testing.T) {
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:      cache,
		DefaultTTL:   time.Minute,
		EnableETag:   true,
		GETOnly:      true,
		Enabled:      true,
		KeyPrefix:    "test:",
		StatusCodes:  []int{200},
		IncludePaths: []string{"/api/"}, // 添加包含路径
	}

	router := gin.New()
	router.Use(CacheMiddleware(config))

	router.GET("/api/etag", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "etag_test"})
	})

	// First request - get ETag
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/etag", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	etag := w1.Header().Get("ETag")
	t.Logf("First request - Status: %d, ETag: %s, Body: %s", w1.Code, etag, w1.Body.String())
	assert.NotEmpty(t, etag)

	// 检查缓存是否存储
	ctx := context.Background()
	cacheKey := "test:GET:/api/etag"
	if data, ok := cache.Get(ctx, cacheKey); ok {
		t.Logf("Cache hit for key %s: %s", cacheKey, string(data))
	} else {
		t.Logf("Cache miss for key %s", cacheKey)
	}

	// Second request with If-None-Match - should return 304
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/etag", nil)
	req2.Header.Set("If-None-Match", etag)
	router.ServeHTTP(w2, req2)
	t.Logf("Second request - Status: %d, X-Cache: %s, ETag response: %s", w2.Code, w2.Header().Get("X-Cache"), w2.Header().Get("ETag"))

	// Debug: 检查第二次请求后的缓存状态
	if data, ok := cache.Get(ctx, cacheKey); ok {
		t.Logf("After second request - Cache still has data")
		var cached cachedResponse
		if err := json.Unmarshal(data, &cached); err == nil {
			t.Logf("Cached ETag: %s", cached.Headers["ETag"])
		}
	}

	assert.Equal(t, http.StatusNotModified, w2.Code)
}

func TestCacheMiddlewareDisabled(t *testing.T) {
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:     cache,
		DefaultTTL:  time.Minute,
		Enabled:     false, // Disabled
		KeyPrefix:   "test:",
		StatusCodes: []int{200},
	}

	router := gin.New()
	router.Use(CacheMiddleware(config))

	callCount := 0
	router.GET("/api/data", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"count": callCount})
	})

	// Both requests should miss cache since caching is disabled
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
		router.ServeHTTP(w, req)
		assert.NotEqual(t, "HIT", w.Header().Get("X-Cache"))
	}
	assert.Equal(t, 2, callCount)
}

func TestCacheManager(t *testing.T) {
	manager := NewCacheManager(nil)

	// Add backend
	backend := NewMemoryCache(100, time.Minute)
	manager.AddBackend("memory", backend)

	// Get backend
	retrieved := manager.GetBackend("memory")
	assert.NotNil(t, retrieved)

	// Set and get
	ctx := context.Background()
	retrieved.Set(ctx, "key1", []byte("value1"), time.Minute)
	val, ok := retrieved.Get(ctx, "key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)

	// Remove backend
	manager.RemoveBackend("memory")
	retrieved = manager.GetBackend("memory")
	assert.Nil(t, retrieved)
}

func TestPolicyManager(t *testing.T) {
	pm := NewPolicyManager()

	// Match existing policy
	policy, ok := pm.MatchPolicy("GET", "/static/css/main.css")
	assert.True(t, ok)
	assert.Equal(t, "static", policy.Name)

	// Add custom policy
	customPolicy := CachePolicy{
		Name:         "custom",
		PathPatterns: []string{"/api/custom/"},
		Methods:      []string{"GET"},
		TTL:          2 * time.Minute,
		Strategy:     InvalidationTTL,
		EnableETag:   true,
	}
	pm.AddPolicy(customPolicy)

	// Match custom policy
	matched, ok := pm.MatchPolicy("GET", "/api/custom/data")
	assert.True(t, ok)
	assert.Equal(t, "custom", matched.Name)

	// List policies
	policies := pm.ListPolicies()
	assert.GreaterOrEqual(t, len(policies), 1)

	// Remove policy
	pm.RemovePolicy("custom")
	_, ok = pm.GetPolicy("custom")
	assert.False(t, ok)
}

func TestStatsCollector(t *testing.T) {
	collector := NewStatsCollector(1000)

	// Record some stats
	collector.RecordHit(1000000)  // 1ms
	collector.RecordHit(2000000)  // 2ms
	collector.RecordMiss(5000000) // 5ms
	collector.RecordSet()
	collector.RecordSet()
	collector.RecordDelete()
	collector.RecordEviction()

	stats := collector.GetStats()
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(2), stats.Sets)
	assert.Equal(t, int64(1), stats.Deletes)
	assert.Equal(t, int64(1), stats.Evictions)
	assert.InDelta(t, 0.666, stats.HitRatio, 0.01)

	// Reset
	collector.Reset()
	stats = collector.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestInvalidationBus(t *testing.T) {
	bus := NewInvalidationBus()

	// Subscribe
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Publish event
	event := CacheInvalidationEvent{
		Type:      "update",
		Resource:  "users",
		ID:        "123",
		Timestamp: time.Now(),
	}
	bus.Publish(event)

	// Receive event
	select {
	case received := <-ch:
		assert.Equal(t, "update", received.Type)
		assert.Equal(t, "users", received.Resource)
		assert.Equal(t, "123", received.ID)
	case <-time.After(time.Second):
		t.Fatal("Did not receive event")
	}
}

func TestGenerateCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		method  string
		path    string
		query   string
		headers map[string]string
		wantSub []string // substrings that should be in key
	}{
		{
			name:    "simple GET",
			method:  "GET",
			path:    "/api/users",
			wantSub: []string{"GET:", "/api/users"},
		},
		{
			name:    "with query params",
			method:  "GET",
			path:    "/api/users",
			query:   "page=1&limit=10",
			wantSub: []string{"GET:", "/api/users?", "page=1&limit=10"},
		},
		{
			name:    "POST method",
			method:  "POST",
			path:    "/api/users",
			wantSub: []string{"POST:", "/api/users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tt.method, tt.path+"?"+tt.query, nil)

			key := generateCacheKey(c, "prefix:")

			for _, sub := range tt.wantSub {
				assert.Contains(t, key, sub, "key should contain %s", sub)
			}
		})
	}
}

func TestGenerateETag(t *testing.T) {
	data1 := []byte("test data")
	data2 := []byte("different data")

	etag1 := generateETag(data1)
	etag2 := generateETag(data1) // Same data
	etag3 := generateETag(data2) // Different data

	// Same data should produce same ETag
	assert.Equal(t, etag1, etag2)

	// Different data should produce different ETag
	assert.NotEqual(t, etag1, etag3)

	// ETag should be quoted
	assert.Contains(t, etag1, `"`)
}

func TestShouldCache(t *testing.T) {
	allowedCodes := []int{200, 201, 301, 302, 404}

	assert.True(t, shouldCache(200, allowedCodes))
	assert.True(t, shouldCache(201, allowedCodes))
	assert.True(t, shouldCache(404, allowedCodes))
	assert.False(t, shouldCache(500, allowedCodes))
	assert.False(t, shouldCache(401, allowedCodes))
}

func TestIsHopByHopHeader(t *testing.T) {
	assert.True(t, isHopByHopHeader("connection"))
	assert.True(t, isHopByHopHeader("Connection"))
	assert.True(t, isHopByHopHeader("transfer-encoding"))
	assert.False(t, isHopByHopHeader("content-type"))
	assert.False(t, isHopByHopHeader("authorization"))
}

func TestCacheMiddlewareWithAuthenticatedRequest(t *testing.T) {
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:            cache,
		DefaultTTL:         time.Minute,
		GETOnly:            true,
		CacheAuthenticated: false, // Should skip authenticated requests
		Enabled:            true,
		KeyPrefix:          "test:",
		StatusCodes:        []int{200},
	}

	router := gin.New()
	router.Use(CacheMiddleware(config))

	callCount := 0
	router.GET("/api/secure", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"count": callCount})
	})

	// Request with Authorization header - should not be cached
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/secure", nil)
		req.Header.Set("Authorization", "Bearer token")
		router.ServeHTTP(w, req)
	}
	assert.Equal(t, 2, callCount) // Handler called twice (not cached)
}

func TestCacheMiddlewareWithTTLGenerator(t *testing.T) {
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:     cache,
		DefaultTTL:  time.Minute,
		Enabled:     true,
		KeyPrefix:   "test:",
		StatusCodes: []int{200},
		TTLGenerator: func(c *gin.Context) time.Duration {
			// Different TTL based on path
			if c.Request.URL.Path == "/api/short" {
				return 10 * time.Second
			}
			return time.Minute
		},
	}

	router := gin.New()
	router.Use(CacheMiddleware(config))

	router.GET("/api/short", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "short_ttl"})
	})
	router.GET("/api/long", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "long_ttl"})
	})

	// Both endpoints should work
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/short", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/long", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestCacheMiddlewareWithCustomKeyGenerator(t *testing.T) {
	cache := NewMemoryCache(100, time.Minute)
	config := &CacheConfig{
		Backend:            cache,
		DefaultTTL:         time.Minute,
		Enabled:            true,
		KeyPrefix:          "test:",
		StatusCodes:        []int{200},
		CacheAuthenticated: true, // Allow caching authenticated requests
		KeyGenerator: func(c *gin.Context) string {
			// Custom key including user ID if available
			userID, exists := c.Get("user_id")
			if exists {
				return "custom:" + userID.(string) + ":" + c.Request.URL.Path
			}
			return "custom:anon:" + c.Request.URL.Path
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Simulate setting user_id in some requests
		if c.GetHeader("X-User-ID") != "" {
			c.Set("user_id", c.GetHeader("X-User-ID"))
		}
		c.Next()
	})
	router.Use(CacheMiddleware(config))

	callCount := 0
	router.GET("/api/custom", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"count": callCount})
	})

	// Request with user ID
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/custom", nil)
	req1.Header.Set("X-User-ID", "user123")
	router.ServeHTTP(w1, req1)
	assert.Equal(t, "MISS", w1.Header().Get("X-Cache"))

	// Same user - should hit
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/custom", nil)
	req2.Header.Set("X-User-ID", "user123")
	router.ServeHTTP(w2, req2)
	assert.Equal(t, "HIT", w2.Header().Get("X-Cache"))

	// Different user - should miss
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/custom", nil)
	req3.Header.Set("X-User-ID", "user456")
	router.ServeHTTP(w3, req3)
	assert.Equal(t, "MISS", w3.Header().Get("X-Cache"))
}
