// Package middleware provides HTTP middleware for the API
package middleware

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// ErrorHandlerConfig configures the error handler middleware
type ErrorHandlerConfig struct {
	// DebugMode enables stack traces in error responses
	DebugMode bool
	// LogErrors logs errors to stdout
	LogErrors bool
	// CustomErrorHandler custom error handler function
	CustomErrorHandler func(c *gin.Context, err error) (int, interface{})
}

// DefaultErrorHandlerConfig default error handler configuration
var DefaultErrorHandlerConfig = ErrorHandlerConfig{
	DebugMode: false,
	LogErrors: true,
}

// APIError represents an API error response
type APIError struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Stack     string      `json:"stack,omitempty"`
}

// Common error codes
const (
	CodeSuccess           = 0
	CodeBadRequest        = 400
	CodeUnauthorized      = 401
	CodeForbidden         = 403
	CodeNotFound          = 404
	CodeConflict          = 409
	CodeInternalError     = 500
	CodeServiceUnavailable = 503
)

// Predefined errors
var (
	ErrBadRequest         = errors.New("bad request")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInternalError      = errors.New("internal server error")
	ErrServiceUnavailable = errors.New("service unavailable")
)

// ErrorHandlerMiddleware creates an error handler middleware
func ErrorHandlerMiddleware(config ...ErrorHandlerConfig) gin.HandlerFunc {
	cfg := DefaultErrorHandlerConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Handle panic
				if cfg.LogErrors {
					log.Printf("[ERROR] Panic recovered: %v\n%s", recovered, debug.Stack())
				}

				var stack string
				if cfg.DebugMode {
					stack = string(debug.Stack())
				}

				requestID, _ := c.Get("requestId")
				response := APIError{
					Code:      CodeInternalError,
					Message:   "Internal server error",
					RequestID: requestID.(string),
					Stack:     stack,
				}

				c.JSON(http.StatusInternalServerError, response)
				c.Abort()
			}
		}()

		c.Next()

		// Handle errors from c.Error()
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			if cfg.LogErrors {
				log.Printf("[ERROR] Request error: %v", err)
			}

			// Check if response was already written
			if c.Writer.Written() {
				return
			}

			// Use custom handler if provided
			if cfg.CustomErrorHandler != nil {
				status, response := cfg.CustomErrorHandler(c, err)
				c.JSON(status, response)
				return
			}

			// Default error response
			apiErr := mapErrorToAPIError(err.Err, c)
			if cfg.DebugMode {
				apiErr.Stack = string(debug.Stack())
			}

			c.JSON(mapErrorToStatus(err.Err), apiErr)
		}
	}
}

// mapErrorToAPIError maps an error to an APIError
func mapErrorToAPIError(err error, c *gin.Context) APIError {
	requestID, _ := c.Get("requestId")

	var message string
	var code int

	switch {
	case errors.Is(err, ErrBadRequest):
		code = CodeBadRequest
		message = "Bad request"
	case errors.Is(err, ErrUnauthorized):
		code = CodeUnauthorized
		message = "Unauthorized"
	case errors.Is(err, ErrForbidden):
		code = CodeForbidden
		message = "Forbidden"
	case errors.Is(err, ErrNotFound):
		code = CodeNotFound
		message = "Not found"
	case errors.Is(err, ErrConflict):
		code = CodeConflict
		message = "Conflict"
	case errors.Is(err, ErrServiceUnavailable):
		code = CodeServiceUnavailable
		message = "Service unavailable"
	default:
		code = CodeInternalError
		message = "Internal server error"
	}

	// Check if error has a custom message
	if err.Error() != "" && err.Error() != message {
		message = err.Error()
	}

	return APIError{
		Code:      code,
		Message:   message,
		RequestID: requestID.(string),
	}
}

// mapErrorToStatus maps an error to an HTTP status code
func mapErrorToStatus(err error) int {
	switch {
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrServiceUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ErrorResponder provides helper methods for error responses
type ErrorResponder struct{}

// NewErrorResponder creates a new error responder
func NewErrorResponder() *ErrorResponder {
	return &ErrorResponder{}
}

// BadRequest responds with a 400 error
func (e *ErrorResponder) BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIError{
		Code:    CodeBadRequest,
		Message: message,
	})
	c.Abort()
}

// Unauthorized responds with a 401 error
func (e *ErrorResponder) Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	c.JSON(http.StatusUnauthorized, APIError{
		Code:    CodeUnauthorized,
		Message: message,
	})
	c.Abort()
}

// Forbidden responds with a 403 error
func (e *ErrorResponder) Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "Forbidden"
	}
	c.JSON(http.StatusForbidden, APIError{
		Code:    CodeForbidden,
		Message: message,
	})
	c.Abort()
}

// NotFound responds with a 404 error
func (e *ErrorResponder) NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "Not found"
	}
	c.JSON(http.StatusNotFound, APIError{
		Code:    CodeNotFound,
		Message: message,
	})
	c.Abort()
}

// InternalError responds with a 500 error
func (e *ErrorResponder) InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "Internal server error"
	}
	c.JSON(http.StatusInternalServerError, APIError{
		Code:    CodeInternalError,
		Message: message,
	})
	c.Abort()
}

// ServiceUnavailable responds with a 503 error
func (e *ErrorResponder) ServiceUnavailable(c *gin.Context, message string) {
	if message == "" {
		message = "Service unavailable"
	}
	c.JSON(http.StatusServiceUnavailable, APIError{
		Code:    CodeServiceUnavailable,
		Message: message,
	})
	c.Abort()
}

// ValidationError responds with a 400 error for validation failures
func (e *ErrorResponder) ValidationError(c *gin.Context, details interface{}) {
	c.JSON(http.StatusBadRequest, APIError{
		Code:    CodeBadRequest,
		Message: "Validation failed",
		Details: details,
	})
	c.Abort()
}

// CustomError responds with a custom error
func (e *ErrorResponder) CustomError(c *gin.Context, status int, code int, message string) {
	c.JSON(status, APIError{
		Code:    code,
		Message: message,
	})
	c.Abort()
}

// Success responds with a success message
func (e *ErrorResponder) Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": "success",
		"data":    data,
	})
}

// SuccessWithMessage responds with a success message and custom message
func (e *ErrorResponder) SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": message,
		"data":    data,
	})
}

// Global error responder
var responder = NewErrorResponder()

// BadRequest responds with a 400 error
func BadRequest(c *gin.Context, message string) {
	responder.BadRequest(c, message)
}

// Unauthorized responds with a 401 error
func Unauthorized(c *gin.Context, message string) {
	responder.Unauthorized(c, message)
}

// Forbidden responds with a 403 error
func Forbidden(c *gin.Context, message string) {
	responder.Forbidden(c, message)
}

// NotFound responds with a 404 error
func NotFound(c *gin.Context, message string) {
	responder.NotFound(c, message)
}

// InternalError responds with a 500 error
func InternalError(c *gin.Context, message string) {
	responder.InternalError(c, message)
}

// ServiceUnavailable responds with a 503 error
func ServiceUnavailable(c *gin.Context, message string) {
	responder.ServiceUnavailable(c, message)
}

// ValidationError responds with a 400 error for validation failures
func ValidationError(c *gin.Context, details interface{}) {
	responder.ValidationError(c, details)
}

// Success responds with a success message
func Success(c *gin.Context, data interface{}) {
	responder.Success(c, data)
}

// SuccessWithMessage responds with a success message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	responder.SuccessWithMessage(c, message, data)
}

// ParseJSONBody parses JSON body and handles errors
func ParseJSONBody(c *gin.Context, v interface{}) error {
	if err := c.ShouldBindJSON(v); err != nil {
		BadRequest(c, "Invalid JSON body: "+err.Error())
		return err
	}
	return nil
}

// MustParseJSON parses JSON body or panics
func MustParseJSON(c *gin.Context, v interface{}) {
	if err := c.ShouldBindJSON(v); err != nil {
		BadRequest(c, "Invalid JSON body: "+err.Error())
		c.Abort()
	}
}

// WriteJSON writes JSON response
func WriteJSON(c *gin.Context, status int, v interface{}) {
	c.JSON(status, v)
}

// WriteError writes an error response
func WriteError(c *gin.Context, status int, code int, message string) {
	c.JSON(status, APIError{
		Code:    code,
		Message: message,
	})
}

// RecoverMiddleware recovers from panics and returns a 500 error
func RecoverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[PANIC] %v\n%s", recovered, debug.Stack())

				requestID, _ := c.Get("requestId")
				c.JSON(http.StatusInternalServerError, APIError{
					Code:      CodeInternalError,
					Message:   "Internal server error",
					RequestID: requestID.(string),
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// APIErrorFromJSON creates an APIError from JSON bytes
func APIErrorFromJSON(data []byte) (*APIError, error) {
	var err APIError
	if e := json.Unmarshal(data, &err); e != nil {
		return nil, e
	}
	return &err, nil
}