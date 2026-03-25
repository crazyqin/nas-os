// Package middleware provides HTTP middleware for the API
package middleware

import (
	"context"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestRequestLoggerMiddleware 测试请求日志中间件.
func TestRequestLoggerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		path           string
		skipPaths      []string
		expectedStatus int
		expectLogged   bool
	}{
		{
			name:           "normal request",
			method:         "GET",
			path:           "/api/v1/test",
			skipPaths:      []string{},
			expectedStatus: http.StatusOK,
			expectLogged:   true,
		},
		{
			name:           "skipped path",
			method:         "GET",
			path:           "/health",
			skipPaths:      []string{"/health"},
			expectedStatus: http.StatusOK,
			expectLogged:   false,
		},
		{
			name:           "POST request",
			method:         "POST",
			path:           "/api/v1/users",
			skipPaths:      []string{},
			expectedStatus: http.StatusCreated,
			expectLogged:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试日志记录器
			logger := &testLogger{}
			config := RequestLoggerConfig{
				SkipPaths:       tt.skipPaths,
				LogRequestBody:  true,
				LogResponseBody: true,
				MaxBodySize:     1024,
				Logger:          logger,
			}

			router := gin.New()
			router.Use(RequestLoggerMiddleware(config))
			router.Handle(tt.method, tt.path, func(c *gin.Context) {
				if tt.method == "POST" {
					c.Status(http.StatusCreated)
				} else {
					c.Status(http.StatusOK)
				}
			})

			req := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectLogged && logger.entry == nil {
				t.Error("Expected request to be logged")
			}

			if !tt.expectLogged && logger.entry != nil {
				t.Error("Expected request not to be logged")
			}
		})
	}
}

// TestRequestLogEntry 测试日志条目结构.
func TestRequestLogEntry(t *testing.T) {
	now := time.Now()
	entry := &RequestLogEntry{
		Timestamp:    now,
		Method:       "POST",
		Path:         "/api/v1/users",
		Query:        "name=test",
		StatusCode:   201,
		Latency:      15.5,
		ClientIP:     "127.0.0.1",
		UserAgent:    "test-agent",
		RequestID:    "req-123",
		UserID:       "user-456",
		RequestSize:  100,
		ResponseSize: 200,
	}

	if entry.Method != "POST" {
		t.Errorf("Expected Method=POST, got %s", entry.Method)
	}

	if entry.StatusCode != 201 {
		t.Errorf("Expected StatusCode=201, got %d", entry.StatusCode)
	}

	if entry.Latency != 15.5 {
		t.Errorf("Expected Latency=15.5, got %f", entry.Latency)
	}
}

// TestMaskSensitiveFields 测试敏感字段脱敏.
func TestMaskSensitiveFields(t *testing.T) {
	sensitiveFields := []string{"password", "token", "secret", "apiKey"}

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "password field",
			input: map[string]interface{}{
				"username": "admin",
				"password": "secret123",
			},
			expected: map[string]interface{}{
				"username": "admin",
				"password": "***MASKED***",
			},
		},
		{
			name: "token field",
			input: map[string]interface{}{
				"token": "abc123",
				"data":  "value",
			},
			expected: map[string]interface{}{
				"token": "***MASKED***",
				"data":  "value",
			},
		},
		{
			name: "nested fields",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"password": "secret",
					"name":     "test",
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"password": "***MASKED***",
					"name":     "test",
				},
			},
		},
		{
			name: "apiKey field",
			input: map[string]interface{}{
				"apiKey": "key123",
			},
			expected: map[string]interface{}{
				"apiKey": "***MASKED***",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveFields(tt.input, sensitiveFields)
			checkMapEquals(t, tt.expected, result)
		})
	}
}

// TestIsSensitiveField 测试敏感字段检测.
func TestIsSensitiveField(t *testing.T) {
	sensitiveFields := []string{"password", "token", "secret"}

	tests := []struct {
		field    string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"apiToken", true},
		{"secretKey", true},
		{"username", false},
		{"email", false},
		{"data", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := isSensitiveField(tt.field, sensitiveFields)
			if result != tt.expected {
				t.Errorf("isSensitiveField(%s) = %v, expected %v", tt.field, result, tt.expected)
			}
		})
	}
}

// TestRequestIDMiddleware 测试请求 ID 中间件.
func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 测试没有请求 ID 的情况
	t.Run("generate request id", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header to be set")
		}
		if len(requestID) != 16 {
			t.Errorf("Expected request ID length 16, got %d", len(requestID))
		}
	})

	// 测试已有请求 ID 的情况
	t.Run("use existing request id", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Request-ID", "existing-id-123")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID != "existing-id-123" {
			t.Errorf("Expected X-Request-ID=existing-id-123, got %s", requestID)
		}
	})
}

// TestRequestLoggerWithSkip 测试带跳过路径的中间件.
func TestRequestLoggerWithSkip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := &testLogger{}
	config := DefaultRequestLoggerConfig
	config.Logger = logger
	config.SkipPaths = []string{"/health", "/metrics"}

	router := gin.New()
	router.Use(RequestLoggerMiddleware(config))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	// 测试跳过的路径
	t.Run("skip health", func(t *testing.T) {
		logger.entry = nil
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if logger.entry != nil {
			t.Error("Expected /health to be skipped")
		}
	})

	// 测试非跳过的路径
	t.Run("log api test", func(t *testing.T) {
		logger.entry = nil
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if logger.entry == nil {
			t.Error("Expected /api/test to be logged")
		}
	})
}

// TestRequestLoggerFull 测试完整日志中间件.
func TestRequestLoggerFull(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestLoggerFull())
	router.POST("/api/test", func(c *gin.Context) {
		var body map[string]interface{}
		c.BindJSON(&body)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := map[string]interface{}{
		"username": "test",
		"password": "secret",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestRequestLoggerMinimal 测试最小日志中间件.
func TestRequestLoggerMinimal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestLoggerMinimal())
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestGenerateRequestID 测试请求 ID 生成.
func TestGenerateRequestID(t *testing.T) {
	ids := make(map[string]bool)

	// 生成多个 ID 并检查唯一性
	for i := 0; i < 100; i++ {
		id := generateRequestID()
		if len(id) != 16 {
			t.Errorf("Expected ID length 16, got %d", len(id))
		}
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

// TestDefaultRequestLogger 测试默认日志记录器.
func TestDefaultRequestLogger(t *testing.T) {
	logger := &DefaultRequestLogger{}

	entry := &RequestLogEntry{
		Method:     "GET",
		Path:       "/test",
		StatusCode: 200,
		Latency:    10.5,
	}

	// 不应 panic
	logger.LogRequest(entry)
}

// Helper functions

type testLogger struct {
	entry *RequestLogEntry
}

func (l *testLogger) LogRequest(entry *RequestLogEntry) {
	l.entry = entry
}

func checkMapEquals(t *testing.T, expected, actual map[string]interface{}) {
	for k, v := range expected {
		actualV, ok := actual[k]
		if !ok {
			t.Errorf("Missing key: %s", k)
			continue
		}

		switch expVal := v.(type) {
		case map[string]interface{}:
			actVal, ok := actualV.(map[string]interface{})
			if !ok {
				t.Errorf("Key %s: expected map, got %T", k, actualV)
				continue
			}
			checkMapEquals(t, expVal, actVal)
		case string:
			if strings.Contains(k, "MASKED") {
				if !strings.Contains(actualV.(string), "MASKED") {
					t.Errorf("Key %s: expected masked value, got %v", k, actualV)
				}
			} else if actualV.(string) != expVal {
				t.Errorf("Key %s: expected %v, got %v", k, expVal, actualV)
			}
		}
	}
}
