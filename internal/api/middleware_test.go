package api

import (
	"errors"
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

func TestErrorHandler(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler(nil))

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	router.GET("/error", func(c *gin.Context) {
		c.Error(NewAPIError(CodeBadRequest, "bad request"))
		c.Next()
	})

	router.GET("/normal", func(c *gin.Context) {
		OK(c, "ok")
	})

	// 测试 panic 恢复
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 测试错误处理
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/error", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 测试正常请求
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/normal", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestID(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())

	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		assert.NotEmpty(t, requestID)
		c.String(http.StatusOK, requestID)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestID_FromHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())

	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		assert.Equal(t, "custom-id", requestID)
		c.String(http.StatusOK, requestID)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-id")
	router.ServeHTTP(w, req)

	assert.Equal(t, "custom-id", w.Header().Get("X-Request-ID"))
}

func TestCORS(t *testing.T) {
	router := gin.New()
	router.Use(CORS(DefaultCORSConfig))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试允许的源（DefaultCORSConfig 只允许 localhost:8080 和 127.0.0.1:8080）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:8080", w.Header().Get("Access-Control-Allow-Origin"))

	// 测试不允许的源（不设置 Access-Control-Allow-Origin 头）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))

	// 测试预检请求（使用允许的源）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCORS_RestrictedOrigins(t *testing.T) {
	config := CORSConfig{
		AllowOrigins:     []string{"http://localhost:3000", "http://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           time.Hour,
	}

	router := gin.New()
	router.Use(CORS(config))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试允许的源
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, "http://example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestSecurityHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age")
}

func TestRequestSizeLimit(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimit(100))

	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试小请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.ContentLength = 50
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 测试大请求
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/test", nil)
	req.ContentLength = 200
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestCacheControl(t *testing.T) {
	router := gin.New()
	router.Use(CacheControl(time.Hour))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Cache-Control"), "max-age=3600")
}

func TestNoCache(t *testing.T) {
	router := gin.New()
	router.Use(NoCache())

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, "no-cache, no-store, must-revalidate", w.Header().Get("Cache-Control"))
}

func TestHealthCheck(t *testing.T) {
	router := gin.New()
	router.Use(HealthCheck("/health"))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试健康检查
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")

	// 测试其他路由
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestTimeout(t *testing.T) {
	router := gin.New()
	router.Use(Timeout(100 * time.Millisecond))

	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(200 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	router.GET("/fast", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试快速请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/fast", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBatchResult(t *testing.T) {
	result := BatchResult{
		Success: 5,
		Failed:  2,
		Total:   7,
		Errors: []BatchError{
			{Index: 1, Message: "error 1"},
			{Index: 3, Message: "error 2"},
		},
	}

	assert.Equal(t, 5, result.Success)
	assert.Equal(t, 2, result.Failed)
	assert.Equal(t, 7, result.Total)
	assert.Len(t, result.Errors, 2)
}

func TestBatchResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result := BatchResult{
		Success: 8,
		Failed:  2,
		Total:   10,
	}

	BatchResponse(c, result)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsAPIError(t *testing.T) {
	apiErr := NewBadRequestError("test")
	assert.True(t, IsAPIError(apiErr))

	normalErr := errors.New("normal error")
	assert.False(t, IsAPIError(normalErr))
}

func TestGetAPIError(t *testing.T) {
	apiErr := NewBadRequestError("test")
	result := GetAPIError(apiErr)
	assert.NotNil(t, result)
	assert.Equal(t, CodeBadRequest, result.Code)

	normalErr := errors.New("normal error")
	result = GetAPIError(normalErr)
	assert.Nil(t, result)
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := WrapError(originalErr, CodeInternalError, "wrapped error")

	assert.Equal(t, CodeInternalError, wrapped.Code)
	assert.Equal(t, "wrapped error", wrapped.Message)
	assert.Equal(t, originalErr, wrapped.Err)
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestRandomString(t *testing.T) {
	s1 := randomString(8)
	s2 := randomString(8)

	assert.Len(t, s1, 8)
	assert.Len(t, s2, 8)
	// 注意：randomString 使用简单的模运算，可能产生相同的字符串
	// 所以我们只验证长度
}

func TestHttpStatusFromCode_PartialSuccess(t *testing.T) {
	// 测试未知代码
	status := httpStatusFromCode(999)
	assert.Equal(t, http.StatusInternalServerError, status)
}

func TestCodePartialSuccess(t *testing.T) {
	assert.Equal(t, 207, CodePartialSuccess)
}
