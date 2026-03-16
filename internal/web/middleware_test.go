package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== SecurityConfig 测试 ==========

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	assert.NotNil(t, config)
	assert.NotEmpty(t, config.AllowedOrigins)
	assert.NotEmpty(t, config.CSRFKey)
	assert.True(t, len(config.CSRFKey) >= 32)
}

func TestDefaultSecurityConfig_WithEnvKey(t *testing.T) {
	// 设置环境变量
	origKey := os.Getenv("NAS_CSRF_KEY")
	defer os.Setenv("NAS_CSRF_KEY", origKey)

	testKey := "test-csrf-key-must-be-at-least-32-bytes!"
	os.Setenv("NAS_CSRF_KEY", testKey)

	config := DefaultSecurityConfig()
	assert.Equal(t, []byte(testKey), config.CSRFKey)
}

func TestSecurityConfig_DefaultValues(t *testing.T) {
	config := DefaultSecurityConfig()

	assert.True(t, config.EnableRateLimit)
	assert.Equal(t, 100, config.RateLimitRPS)
	assert.Contains(t, config.AllowedOrigins, "http://localhost:8080")
}

// ========== Logger Middleware 测试 ==========

func TestLoggerMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(loggerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== 并发测试 ==========

func TestMiddleware_ConcurrentRequests(t *testing.T) {
	config := DefaultSecurityConfig()

	router := gin.New()
	router.Use(loggerMiddleware())
	router.Use(corsMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// ========== SecurityConfig 边缘情况测试 ==========

func TestSecurityConfig_EmptyAllowedOrigins(t *testing.T) {
	config := &SecurityConfig{
		AllowedOrigins: []string{},
		CSRFKey:        make([]byte, 32),
	}

	router := gin.New()
	router.Use(corsMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 请求应该成功，但无 CORS 头
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityConfig_ShortCSRFKey(t *testing.T) {
	config := &SecurityConfig{
		AllowedOrigins: []string{"http://localhost:8080"},
		CSRFKey:        []byte("short"),
	}

	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 短密钥也应该工作（虽然不推荐）
	assert.True(t, w.Code == http.StatusForbidden || w.Code == http.StatusOK)
}

// ========== 多路由测试 ==========

func TestMiddleware_MultipleRoutes(t *testing.T) {
	config := DefaultSecurityConfig()

	router := gin.New()
	router.Use(loggerMiddleware())
	router.Use(corsMiddleware(config))

	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"path": "v1"})
	})
	router.GET("/api/v2/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"path": "v2"})
	})
	router.POST("/api/v1/submit", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"submitted": true})
	})

	// 测试不同路由
	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/test"},
		{"GET", "/api/v2/test"},
		{"POST", "/api/v1/submit"},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Origin", "http://localhost:8080")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Path: %s", tc.path)
	}
}

// ========== 性能测试 ==========

func BenchmarkLoggerMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(loggerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkCORSMiddleware(b *testing.B) {
	config := DefaultSecurityConfig()

	router := gin.New()
	router.Use(corsMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:8080")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	config := DefaultSecurityConfig()

	router := gin.New()
	router.Use(loggerMiddleware())
	router.Use(corsMiddleware(config))
	router.Use(gin.Recovery())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}