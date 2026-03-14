// Package api 提供 API 响应和错误处理测试
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSuccessResponse(t *testing.T) {
	resp := Success("test data")
	if resp.Code != CodeSuccess {
		t.Errorf("Expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected message 'success', got %s", resp.Message)
	}
	if resp.Data != "test data" {
		t.Errorf("Expected data 'test data', got %v", resp.Data)
	}
}

func TestSuccessWithMessage(t *testing.T) {
	resp := SuccessWithMessage("custom message", "data")
	if resp.Code != CodeSuccess {
		t.Errorf("Expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Message != "custom message" {
		t.Errorf("Expected message 'custom message', got %s", resp.Message)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := Error(CodeBadRequest, "invalid request")
	if resp.Code != CodeBadRequest {
		t.Errorf("Expected code %d, got %d", CodeBadRequest, resp.Code)
	}
	if resp.Message != "invalid request" {
		t.Errorf("Expected message 'invalid request', got %s", resp.Message)
	}
}

func TestErrorWithDetails(t *testing.T) {
	details := map[string]interface{}{"field": "name"}
	resp := ErrorWithDetails(CodeBadRequest, "validation error", details)
	if resp.Code != CodeBadRequest {
		t.Errorf("Expected code %d, got %d", CodeBadRequest, resp.Code)
	}
	if resp.Data == nil {
		t.Error("Expected details in data")
	}
}

func TestAPIError(t *testing.T) {
	err := NewAPIError(CodeNotFound, "resource not found")
	if err.Code != CodeNotFound {
		t.Errorf("Expected code %d, got %d", CodeNotFound, err.Code)
	}
	if err.Message != "resource not found" {
		t.Errorf("Expected message 'resource not found', got %s", err.Message)
	}
	if err.Error() != "resource not found" {
		t.Errorf("Expected error string 'resource not found', got %s", err.Error())
	}
}

func TestAPIErrorWithWrappedError(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewAPIError(CodeInternalError, "internal error", originalErr)
	if err.Err != originalErr {
		t.Error("Expected wrapped error")
	}
	if err.Unwrap() != originalErr {
		t.Error("Expected Unwrap to return original error")
	}
}

func TestPredefinedErrors(t *testing.T) {
	if ErrBadRequest.Code != CodeBadRequest {
		t.Errorf("Expected ErrBadRequest code %d, got %d", CodeBadRequest, ErrBadRequest.Code)
	}
	if ErrUnauthorized.Code != CodeUnauthorized {
		t.Errorf("Expected ErrUnauthorized code %d, got %d", CodeUnauthorized, ErrUnauthorized.Code)
	}
	if ErrForbidden.Code != CodeForbidden {
		t.Errorf("Expected ErrForbidden code %d, got %d", CodeForbidden, ErrForbidden.Code)
	}
	if ErrNotFound.Code != CodeNotFound {
		t.Errorf("Expected ErrNotFound code %d, got %d", CodeNotFound, ErrNotFound.Code)
	}
}

func TestNewBadRequestError(t *testing.T) {
	err := NewBadRequestError("bad request")
	if err.Code != CodeBadRequest {
		t.Errorf("Expected code %d, got %d", CodeBadRequest, err.Code)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("not found")
	if err.Code != CodeNotFound {
		t.Errorf("Expected code %d, got %d", CodeNotFound, err.Code)
	}
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("conflict")
	if err.Code != CodeConflict {
		t.Errorf("Expected code %d, got %d", CodeConflict, err.Code)
	}
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError("internal error")
	if err.Code != CodeInternalError {
		t.Errorf("Expected code %d, got %d", CodeInternalError, err.Code)
	}
}

func TestHttpStatusFromCode(t *testing.T) {
	tests := []struct {
		code     int
		expected int
	}{
		{CodeSuccess, http.StatusOK},
		{CodeBadRequest, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeMethodNotAllowed, http.StatusMethodNotAllowed},
		{CodeConflict, http.StatusConflict},
		{CodeTooManyRequests, http.StatusTooManyRequests},
		{CodeInternalError, http.StatusInternalServerError},
		{CodeServiceUnavailable, http.StatusServiceUnavailable},
		{999, http.StatusInternalServerError}, // unknown code
	}

	for _, tt := range tests {
		result := httpStatusFromCode(tt.code)
		if result != tt.expected {
			t.Errorf("httpStatusFromCode(%d) = %d, expected %d", tt.code, result, tt.expected)
		}
	}
}

func TestPageData(t *testing.T) {
	items := []string{"a", "b", "c"}
	data := PageData{
		Items:      items,
		Total:      100,
		Page:       1,
		PageSize:   10,
		TotalPages: 10,
	}

	if data.Total != 100 {
		t.Errorf("Expected total 100, got %d", data.Total)
	}
	if data.Page != 1 {
		t.Errorf("Expected page 1, got %d", data.Page)
	}
	if data.PageSize != 10 {
		t.Errorf("Expected pageSize 10, got %d", data.PageSize)
	}
}

// Gin context response tests

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	OK(c, "test data")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Created(c, "created")

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NoContent(c)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	BadRequest(c, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Unauthorized(c, "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Forbidden(c, "")

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NotFound(c, "")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	TooManyRequests(c, "")

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	InternalError(c, "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestServiceUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ServiceUnavailable(c, "")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

// Error type checker tests

type notFoundTestError struct{}

func (e notFoundTestError) Error() string    { return "not found" }
func (e notFoundTestError) NotFound() bool   { return true }
func (e notFoundTestError) BadRequest() bool { return false }

type badRequestTestError struct{}

func (e badRequestTestError) Error() string    { return "bad request" }
func (e badRequestTestError) BadRequest() bool { return true }

func TestIsNotFoundError(t *testing.T) {
	err := notFoundTestError{}
	if !isNotFoundError(err) {
		t.Error("Expected isNotFoundError to return true")
	}

	normalErr := errors.New("normal error")
	if isNotFoundError(normalErr) {
		t.Error("Expected isNotFoundError to return false for normal error")
	}
}

func TestIsBadRequestError(t *testing.T) {
	err := badRequestTestError{}
	if !isBadRequestError(err) {
		t.Error("Expected isBadRequestError to return true")
	}

	normalErr := errors.New("normal error")
	if isBadRequestError(normalErr) {
		t.Error("Expected isBadRequestError to return false for normal error")
	}
}
