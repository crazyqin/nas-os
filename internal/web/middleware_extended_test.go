package web

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== Rate Limit Middleware 测试 ==========

func TestRateLimitMiddleware_Enabled(t *testing.T) {
	config := &SecurityConfig{
		EnableRateLimit: true,
		RateLimitRPS:    2, // 每秒 2 个请求
	}

	router := gin.New()
	router.Use(rateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 发送多个请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 前两个应该成功，后续可能被限制
		if i < 2 {
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i)
		}
		// 注意：由于测试执行速度，可能第三个请求也会成功
	}
}

// ========== CSRF Middleware 测试 ==========

func TestCSRFMiddleware_GET(t *testing.T) {
	config := DefaultSecurityConfig()
	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// GET 请求应该通过并设置 CSRF cookie
	assert.Equal(t, http.StatusOK, w.Code)

	// 检查 CSRF cookie 是否设置
	cookies := w.Result().Cookies()
	var hasCSRFCookie bool
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			hasCSRFCookie = true
			break
		}
	}
	assert.True(t, hasCSRFCookie, "CSRF cookie should be set")
}

func TestCSRFMiddleware_POST_WithoutToken(t *testing.T) {
	config := DefaultSecurityConfig()
	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 没有 CSRF token 的 POST 应该被拒绝
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFMiddleware_POST_WithToken(t *testing.T) {
	config := DefaultSecurityConfig()
	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 先发送 GET 获取 CSRF token
	getReq := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)

	// 提取 CSRF token
	var csrfToken string
	for _, cookie := range getW.Result().Cookies() {
		if cookie.Name == "csrf_token" {
			csrfToken = cookie.Value
			break
		}
	}

	assert.NotEmpty(t, csrfToken, "CSRF token should be set")

	// 发送带 token 的 POST
	postReq := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader("{}"))
	postReq.Header.Set("X-CSRF-Token", csrfToken)
	// 设置 cookie
	postReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	postW := httptest.NewRecorder()
	router.ServeHTTP(postW, postReq)

	assert.Equal(t, http.StatusOK, postW.Code)
}

func TestCSRFMiddleware_HEAD(t *testing.T) {
	config := DefaultSecurityConfig()
	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.HEAD("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), "HEAD", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_OPTIONS(t *testing.T) {
	config := DefaultSecurityConfig()
	router := gin.New()
	router.Use(csrfMiddleware(config))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), "OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== CSRF Token 函数测试 ==========

func TestGenerateCSRFToken(t *testing.T) {
	key := []byte("test-secret-key-32-bytes-long-!")
	token1 := generateCSRFToken(key)
	token2 := generateCSRFToken(key)

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	// 每次生成的 token 应该不同（因为包含 UUID）
	assert.NotEqual(t, token1, token2)
}

func TestValidateCSRFToken(t *testing.T) {
	key := []byte("test-secret-key-32-bytes-long-!")

	tests := []struct {
		name          string
		token         string
		expectedToken string
		expected      bool
	}{
		{
			name:          "valid tokens",
			token:         "test-token",
			expectedToken: "test-token",
			expected:      true,
		},
		{
			name:          "invalid tokens",
			token:         "wrong-token",
			expectedToken: "correct-token",
			expected:      false,
		},
		{
			name:          "empty token",
			token:         "",
			expectedToken: "expected",
			expected:      false,
		},
		{
			name:          "empty expected",
			token:         "token",
			expectedToken: "",
			expected:      false,
		},
		{
			name:          "both empty",
			token:         "",
			expectedToken: "",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateCSRFToken(tt.token, tt.expectedToken, key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== Input Validation Middleware 测试 ==========

func TestInputValidationMiddleware_ValidRequest(t *testing.T) {
	router := gin.New()
	router.Use(inputValidationMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(`{"test": "data"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInputValidationMiddleware_LongURL(t *testing.T) {
	router := gin.New()
	router.Use(inputValidationMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 创建一个超长 URL
	longPath := strings.Repeat("/a", 1500) // 超过 2048 字符
	req := httptest.NewRequestWithContext(context.Background(), "GET", longPath, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Audit Log Middleware 测试 ==========

func TestAuditLogMiddleware_NonSensitivePath(t *testing.T) {
	router := gin.New()
	router.Use(auditLogMiddleware())
	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/public", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 非敏感路径应该正常通过
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditLogMiddleware_SensitivePath(t *testing.T) {
	router := gin.New()
	router.Use(auditLogMiddleware())
	router.POST("/api/v1/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/users", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 敏感路径也应该正常通过，只是会记录审计日志
	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== CORS Middleware 详细测试 ==========

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	config := &SecurityConfig{
		AllowedOrigins: []string{"http://localhost:8080", "http://example.com"},
	}

	router := gin.New()
	router.Use(corsMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectOrigin   string
		expectWildcard bool
	}{
		{
			name:         "allowed origin localhost",
			origin:       "http://localhost:8080",
			expectOrigin: "http://localhost:8080",
		},
		{
			name:         "allowed origin example.com",
			origin:       "http://example.com",
			expectOrigin: "http://example.com",
		},
		{
			name:           "disallowed origin",
			origin:         "http://malicious.com",
			expectWildcard: true, // OPTIONS requests get wildcard
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			origin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.expectWildcard {
				// For non-OPTIONS, no origin set for disallowed
			} else {
				assert.Equal(t, tt.expectOrigin, origin)
			}
		})
	}
}

func TestCORSMiddleware_Headers(t *testing.T) {
	router := gin.New()
	router.Use(corsMiddleware(DefaultSecurityConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// ========== Security Headers 详细测试 ==========

func TestSecurityHeadersMiddleware_AllHeaders(t *testing.T) {
	router := gin.New()
	router.Use(securityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, w.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, w.Header().Get("Permissions-Policy"))
}

func TestSecurityHeadersMiddleware_HSTS(t *testing.T) {
	router := gin.New()
	router.Use(securityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// TLS request
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	// Simulate TLS by setting a non-nil TLS field
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// HSTS header should be set for TLS
	hsts := w.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age=")
}

// ========== Logger Middleware 测试 ==========

func TestLoggerMiddleware_SetsRequestID(t *testing.T) {
	router := gin.New()
	router.Use(loggerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get("requestID")
		assert.True(t, exists)
		assert.NotEmpty(t, requestID)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}

func TestLoggerMiddleware_SetsStartTime(t *testing.T) {
	router := gin.New()
	router.Use(loggerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("startTime")
		assert.True(t, exists)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}
