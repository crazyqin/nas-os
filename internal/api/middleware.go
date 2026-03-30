package api

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ========== 错误处理中间件 ==========

// ErrorHandler 错误处理中间件.
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic
				if logger != nil {
					logger.Error("Panic recovered",
						zap.Any("panic", r),
						zap.String("stack", string(debug.Stack())),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
					)
				}

				// 返回 500 错误
				c.JSON(http.StatusInternalServerError, Response{
					Code:    CodeInternalError,
					Message: "服务器内部错误",
				})
				c.Abort()
			}
		}()

		c.Next()

		// 处理请求中的错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// 根据错误类型返回响应
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				c.JSON(httpStatusFromCode(apiErr.Code), Response{
					Code:    apiErr.Code,
					Message: apiErr.Message,
				})
				return
			}

			// 默认返回 500
			c.JSON(http.StatusInternalServerError, Response{
				Code:    CodeInternalError,
				Message: err.Error(),
			})
		}
	}
}

// RecoveryMiddleware 恢复中间件（简化版）.
func RecoveryMiddleware() gin.HandlerFunc {
	return ErrorHandler(nil)
}

// ========== 请求日志中间件 ==========

// RequestLogger 请求日志中间件.
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if logger != nil {
			logger.Info("HTTP Request",
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.Int("status", status),
				zap.Duration("latency", latency),
				zap.String("client_ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			)
		}
	}
}

// ========== 请求ID中间件 ==========

// RequestID 请求ID中间件.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d-%x", time.Now().UnixNano(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

// ========== 超时中间件 ==========

// Timeout 超时中间件.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		done := make(chan struct{})
		go func() {
			defer close(done)
			c.Next()
		}()

		select {
		case <-done:
			return
		case <-time.After(timeout):
			c.JSON(http.StatusServiceUnavailable, Response{
				Code:    CodeServiceUnavailable,
				Message: "请求超时",
			})
			c.Abort()
			return
		case <-ctx.Done():
			c.JSON(http.StatusServiceUnavailable, Response{
				Code:    CodeServiceUnavailable,
				Message: "请求已取消",
			})
			c.Abort()
			return
		}
	}
}

// ========== CORS中间件 ==========

// CORSConfig CORS配置.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// DefaultCORSConfig 默认CORS配置
// 安全加固：默认只允许本地源，生产环境应显式配置允许的源.
var DefaultCORSConfig = CORSConfig{
	AllowOrigins:     []string{"http://localhost:8080", "http://127.0.0.1:8080"},
	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
	AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
	ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
	AllowCredentials: true,
	MaxAge:           12 * time.Hour,
}

// CORS CORS中间件.
func CORS(config CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		// 检查是否允许的源
		allowed := false
		for _, o := range config.AllowOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		if len(config.AllowMethods) > 0 {
			methods := ""
			for i, m := range config.AllowMethods {
				if i > 0 {
					methods += ", "
				}
				methods += m
			}
			c.Header("Access-Control-Allow-Methods", methods)
		}

		if len(config.AllowHeaders) > 0 {
			headers := ""
			for i, h := range config.AllowHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += h
			}
			c.Header("Access-Control-Allow-Headers", headers)
		}

		if len(config.ExposeHeaders) > 0 {
			headers := ""
			for i, h := range config.ExposeHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += h
			}
			c.Header("Access-Control-Expose-Headers", headers)
		}

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", int(config.MaxAge.Seconds())))
		}

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ========== 安全头中间件 ==========

// SecurityHeaders 安全头中间件.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// ========== 请求体大小限制中间件 ==========

// RequestSizeLimit 请求体大小限制中间件.
func RequestSizeLimit(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, Response{
				Code:    413,
				Message: fmt.Sprintf("请求体大小超过限制 (%d bytes)", maxSize),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ========== 缓存控制中间件 ==========

// CacheControl 缓存控制中间件.
func CacheControl(maxAge time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
		c.Next()
	}
}

// NoCache 无缓存中间件.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

// ========== 健康检查中间件 ==========

// HealthCheck 健康检查端点.
func HealthCheck(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == path {
			c.JSON(http.StatusOK, map[string]interface{}{
				"status":    "ok",
				"timestamp": time.Now().Unix(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ========== 辅助函数 ==========

// GetRequestID 从上下文获取请求ID.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}

// SetError 设置错误（用于中间件链）.
func SetError(c *gin.Context, err *APIError) {
	_ = c.Error(err)
}

// IsAPIError 检查是否是API错误.
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

// GetAPIError 获取API错误.
func GetAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}

// WrapError 包装普通错误为API错误.
func WrapError(err error, code int, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ========== 批量操作响应 ==========

// BatchResult 批量操作结果.
type BatchResult struct {
	Success int          `json:"success"`
	Failed  int          `json:"failed"`
	Total   int          `json:"total"`
	Errors  []BatchError `json:"errors,omitempty"`
}

// BatchError 批量操作错误.
type BatchError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

// BatchResponse 返回批量操作响应.
func BatchResponse(c *gin.Context, result BatchResult) {
	code := CodeSuccess
	if result.Failed > 0 && result.Success == 0 {
		code = CodeBadRequest
	} else if result.Failed > 0 {
		code = CodePartialSuccess
	}

	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: fmt.Sprintf("成功 %d，失败 %d", result.Success, result.Failed),
		Data:    result,
	})
}

// CodePartialSuccess 部分成功.
const CodePartialSuccess = 207
