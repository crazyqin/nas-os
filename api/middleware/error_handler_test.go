// Package middleware provides tests for error handler middleware
package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestErrorHandlerMiddleware tests the error handler middleware.
func TestErrorHandlerMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandlerMiddleware())

	router.GET("/error", func(c *gin.Context) {
		c.Error(errors.New("test error"))
	})

	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test error response
	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Test OK response
	req = httptest.NewRequest("GET", "/ok", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestErrorHandlerPanic tests panic recovery.
func TestErrorHandlerPanic(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandlerMiddleware())

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// TestErrorHandlerDebugMode tests debug mode.
func TestErrorHandlerDebugMode(t *testing.T) {
	config := ErrorHandlerConfig{
		DebugMode: true,
		LogErrors: false,
	}

	router := gin.New()
	router.Use(ErrorHandlerMiddleware(config))

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// In debug mode, stack trace should be included
	var response map[string]interface{}
	// Just check that we get a response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
	_ = response // avoid unused variable error
}

// TestAPIError tests APIError structure.
func TestAPIError(t *testing.T) {
	err := APIError{
		Code:      CodeBadRequest,
		Message:   "Test error",
		RequestID: "req-123",
		Details:   map[string]string{"field": "name"},
	}

	if err.Code != CodeBadRequest {
		t.Errorf("Expected code %d, got %d", CodeBadRequest, err.Code)
	}

	if err.Message != "Test error" {
		t.Errorf("Expected message 'Test error', got %s", err.Message)
	}
}

// TestErrorResponder tests the error responder.
func TestErrorResponder(t *testing.T) {
	router := gin.New()
	responder := NewErrorResponder()

	router.GET("/bad-request", func(c *gin.Context) {
		responder.BadRequest(c, "Invalid input")
	})

	router.GET("/unauthorized", func(c *gin.Context) {
		responder.Unauthorized(c, "")
	})

	router.GET("/forbidden", func(c *gin.Context) {
		responder.Forbidden(c, "Access denied")
	})

	router.GET("/not-found", func(c *gin.Context) {
		responder.NotFound(c, "")
	})

	router.GET("/internal", func(c *gin.Context) {
		responder.InternalError(c, "Something went wrong")
	})

	router.GET("/unavailable", func(c *gin.Context) {
		responder.ServiceUnavailable(c, "")
	})

	router.GET("/validation", func(c *gin.Context) {
		responder.ValidationError(c, map[string]string{"name": "required"})
	})

	router.GET("/success", func(c *gin.Context) {
		responder.Success(c, map[string]string{"hello": "world"})
	})

	tests := []struct {
		path         string
		expectedCode int
	}{
		{"/bad-request", http.StatusBadRequest},
		{"/unauthorized", http.StatusUnauthorized},
		{"/forbidden", http.StatusForbidden},
		{"/not-found", http.StatusNotFound},
		{"/internal", http.StatusInternalServerError},
		{"/unavailable", http.StatusServiceUnavailable},
		{"/validation", http.StatusBadRequest},
		{"/success", http.StatusOK},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != tt.expectedCode {
			t.Errorf("Path %s: expected status %d, got %d", tt.path, tt.expectedCode, w.Code)
		}
	}
}

// TestHelperFunctions tests helper functions.
func TestHelperFunctions(t *testing.T) {
	router := gin.New()

	router.GET("/bad-request", func(c *gin.Context) {
		BadRequest(c, "test")
	})

	router.GET("/unauthorized", func(c *gin.Context) {
		Unauthorized(c, "")
	})

	router.GET("/forbidden", func(c *gin.Context) {
		Forbidden(c, "test")
	})

	router.GET("/not-found", func(c *gin.Context) {
		NotFound(c, "")
	})

	router.GET("/internal", func(c *gin.Context) {
		InternalError(c, "test")
	})

	router.GET("/unavailable", func(c *gin.Context) {
		ServiceUnavailable(c, "")
	})

	router.GET("/validation", func(c *gin.Context) {
		ValidationError(c, map[string]string{"test": "error"})
	})

	router.GET("/success", func(c *gin.Context) {
		Success(c, map[string]string{"test": "data"})
	})

	router.GET("/success-msg", func(c *gin.Context) {
		SuccessWithMessage(c, "Done", map[string]string{"test": "data"})
	})

	// Just verify they work without panic
	req := httptest.NewRequest("GET", "/bad-request", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestMapErrorToStatus tests error to status mapping.
func TestMapErrorToStatus(t *testing.T) {
	tests := []struct {
		err      error
		expected int
	}{
		{ErrBadRequest, http.StatusBadRequest},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrNotFound, http.StatusNotFound},
		{ErrConflict, http.StatusConflict},
		{ErrServiceUnavailable, http.StatusServiceUnavailable},
		{ErrInternalError, http.StatusInternalServerError},
		{errors.New("unknown"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		result := mapErrorToStatus(tt.err)
		if result != tt.expected {
			t.Errorf("Error %v: expected status %d, got %d", tt.err, tt.expected, result)
		}
	}
}

// TestRecoverMiddleware tests recover middleware.
func TestRecoverMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(RecoverMiddleware())

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test panic recovery
	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Test normal request
	req = httptest.NewRequest("GET", "/ok", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestCustomErrorHandler tests custom error handler.
func TestCustomErrorHandler(t *testing.T) {
	config := ErrorHandlerConfig{
		LogErrors: false,
		CustomErrorHandler: func(c *gin.Context, err error) (int, interface{}) {
			return http.StatusBadRequest, map[string]string{
				"custom": "handler",
				"error":  err.Error(),
			}
		},
	}

	router := gin.New()
	router.Use(ErrorHandlerMiddleware(config))

	router.GET("/error", func(c *gin.Context) {
		c.Error(errors.New("test error"))
	})

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestAPIErrorFromJSON tests JSON parsing.
func TestAPIErrorFromJSON(t *testing.T) {
	jsonData := `{"code":400,"message":"Bad request","requestId":"req-123"}`

	err, e := APIErrorFromJSON([]byte(jsonData))
	if e != nil {
		t.Fatalf("Failed to parse JSON: %v", e)
	}

	if err.Code != 400 {
		t.Errorf("Expected code 400, got %d", err.Code)
	}

	if err.Message != "Bad request" {
		t.Errorf("Expected message 'Bad request', got %s", err.Message)
	}

	// Test invalid JSON
	_, e = APIErrorFromJSON([]byte("invalid"))
	if e == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestWriteJSON tests WriteJSON helper.
func TestWriteJSON(t *testing.T) {
	router := gin.New()

	router.GET("/json", func(c *gin.Context) {
		WriteJSON(c, http.StatusOK, map[string]string{"hello": "world"})
	})

	req := httptest.NewRequest("GET", "/json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestWriteError tests WriteError helper.
func TestWriteError(t *testing.T) {
	router := gin.New()

	router.GET("/error", func(c *gin.Context) {
		WriteError(c, http.StatusBadRequest, 400, "Bad request")
	})

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// BenchmarkErrorHandlerMiddleware benchmarks the middleware.
func BenchmarkErrorHandlerMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(ErrorHandlerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkErrorResponder benchmarks error responses.
func BenchmarkErrorResponder(b *testing.B) {
	router := gin.New()
	responder := NewErrorResponder()

	router.GET("/error", func(c *gin.Context) {
		responder.BadRequest(c, "test error")
	})

	req := httptest.NewRequest("GET", "/error", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
