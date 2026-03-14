// Package middleware provides HTTP middleware for the API
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLoggerConfig 配置请求日志中间件
type RequestLoggerConfig struct {
	// SkipPaths 跳过日志记录的路径
	SkipPaths []string
	// LogRequestBody 是否记录请求体
	LogRequestBody bool
	// LogResponseBody 是否记录响应体
	LogResponseBody bool
	// MaxBodySize 最大记录的请求/响应体大小（字节）
	MaxBodySize int64
	// SensitiveFields 敏感字段列表（会被脱敏）
	SensitiveFields []string
	// Logger 自定义日志器（可选）
	Logger RequestLogger
}

// RequestLogger 日志接口
type RequestLogger interface {
	LogRequest(entry *RequestLogEntry)
}

// RequestLogEntry 请求日志条目
type RequestLogEntry struct {
	Timestamp      time.Time   `json:"timestamp"`
	Method         string      `json:"method"`
	Path           string      `json:"path"`
	Query          string      `json:"query,omitempty"`
	StatusCode     int         `json:"statusCode"`
	Latency        float64     `json:"latencyMs"`
	ClientIP       string      `json:"clientIp"`
	UserAgent      string      `json:"userAgent"`
	RequestID      string      `json:"requestId,omitempty"`
	UserID         string      `json:"userId,omitempty"`
	RequestBody    interface{} `json:"requestBody,omitempty"`
	ResponseBody   interface{} `json:"responseBody,omitempty"`
	RequestSize    int64       `json:"requestSize"`
	ResponseSize   int64       `json:"responseSize"`
	ContentType    string      `json:"contentType,omitempty"`
	Referer        string      `json:"referer,omitempty"`
	Error          string      `json:"error,omitempty"`
	APIVersion     string      `json:"apiVersion,omitempty"`
}

// DefaultRequestLogger 默认日志记录器
type DefaultRequestLogger struct{}

// LogRequest 记录请求日志
func (l *DefaultRequestLogger) LogRequest(entry *RequestLogEntry) {
	logData, _ := json.Marshal(entry)
	log.Printf("[API] %s", string(logData))
}

// DefaultRequestLoggerConfig 默认配置
var DefaultRequestLoggerConfig = RequestLoggerConfig{
	SkipPaths: []string{
		"/health",
		"/metrics",
		"/api/v1/system/health",
	},
	LogRequestBody:  true,
	LogResponseBody: false,
	MaxBodySize:     1024 * 10, // 10KB
	SensitiveFields: []string{
		"password",
		"token",
		"secret",
		"apiKey",
		"api_key",
		"authorization",
		"credential",
		"privateKey",
		"private_key",
	},
	Logger: &DefaultRequestLogger{},
}

// responseWriter 包装 gin.ResponseWriter 以捕获响应体
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// RequestLoggerMiddleware 创建请求日志中间件
func RequestLoggerMiddleware(config RequestLoggerConfig) gin.HandlerFunc {
	// 设置默认值
	if config.MaxBodySize == 0 {
		config.MaxBodySize = DefaultRequestLoggerConfig.MaxBodySize
	}
	if config.Logger == nil {
		config.Logger = DefaultRequestLoggerConfig.Logger
	}
	if config.SensitiveFields == nil {
		config.SensitiveFields = DefaultRequestLoggerConfig.SensitiveFields
	}

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		// 检查是否跳过
		if skipMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		startTime := time.Now()

		// 创建日志条目
		entry := &RequestLogEntry{
			Timestamp:   startTime,
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Query:       c.Request.URL.RawQuery,
			ClientIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			RequestSize: c.Request.ContentLength,
			ContentType: c.ContentType(),
			Referer:     c.Request.Referer(),
			RequestID:   c.GetString("requestId"),
			UserID:      c.GetString("userID"),
			APIVersion:  c.GetString("apiVersion"),
		}

		// 记录请求体
		if config.LogRequestBody && c.Request.Body != nil && c.Request.ContentLength > 0 {
			if c.Request.ContentLength <= config.MaxBodySize {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err == nil {
					// 恢复请求体
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// 尝试解析 JSON
					contentType := c.ContentType()
					if strings.Contains(contentType, "application/json") {
						var bodyMap map[string]interface{}
						if json.Unmarshal(bodyBytes, &bodyMap) == nil {
							entry.RequestBody = maskSensitiveFields(bodyMap, config.SensitiveFields)
						} else {
							entry.RequestBody = string(bodyBytes)
						}
					} else {
						entry.RequestBody = string(bodyBytes)
					}
				}
			}
		}

		// 包装响应写入器
		var writer *responseWriter
		if config.LogResponseBody {
			writer = &responseWriter{
				ResponseWriter: c.Writer,
				body:           bytes.NewBuffer(nil),
			}
			c.Writer = writer
		}

		// 处理请求
		c.Next()

		// 记录响应
		entry.StatusCode = c.Writer.Status()
		entry.Latency = float64(time.Since(startTime).Microseconds()) / 1000.0 // 毫秒
		entry.ResponseSize = int64(c.Writer.Size())

		// 记录错误
		if len(c.Errors) > 0 {
			errorMessages := make([]string, len(c.Errors))
			for i, e := range c.Errors {
				errorMessages[i] = e.Error()
			}
			entry.Error = strings.Join(errorMessages, "; ")
		}

		// 记录响应体
		if config.LogResponseBody && writer != nil && writer.body.Len() > 0 {
			if int64(writer.body.Len()) <= config.MaxBodySize {
				contentType := c.Writer.Header().Get("Content-Type")
				if strings.Contains(contentType, "application/json") {
					var bodyMap map[string]interface{}
					if json.Unmarshal(writer.body.Bytes(), &bodyMap) == nil {
						entry.ResponseBody = maskSensitiveFields(bodyMap, config.SensitiveFields)
					} else {
						entry.ResponseBody = writer.body.String()
					}
				} else {
					entry.ResponseBody = writer.body.String()
				}
			}
		}

		// 写入日志
		config.Logger.LogRequest(entry)
	}
}

// maskSensitiveFields 脱敏敏感字段
func maskSensitiveFields(data map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		if isSensitiveField(k, sensitiveFields) {
			result[k] = "***MASKED***"
		} else {
			switch val := v.(type) {
			case map[string]interface{}:
				result[k] = maskSensitiveFields(val, sensitiveFields)
			default:
				result[k] = v
			}
		}
	}
	return result
}

// isSensitiveField 检查字段是否为敏感字段
func isSensitiveField(field string, sensitiveFields []string) bool {
	fieldLower := strings.ToLower(field)
	for _, sf := range sensitiveFields {
		if strings.Contains(fieldLower, strings.ToLower(sf)) {
			return true
		}
	}
	return false
}

// RequestLoggerWithSkip 创建带跳过路径的请求日志中间件
func RequestLoggerWithSkip(skipPaths ...string) gin.HandlerFunc {
	config := DefaultRequestLoggerConfig
	config.SkipPaths = append(config.SkipPaths, skipPaths...)
	return RequestLoggerMiddleware(config)
}

// RequestLoggerFull 创建完整请求日志中间件（包含请求和响应体）
func RequestLoggerFull() gin.HandlerFunc {
	config := DefaultRequestLoggerConfig
	config.LogRequestBody = true
	config.LogResponseBody = true
	return RequestLoggerMiddleware(config)
}

// RequestLoggerMinimal 创建最小请求日志中间件
func RequestLoggerMinimal() gin.HandlerFunc {
	config := DefaultRequestLoggerConfig
	config.LogRequestBody = false
	config.LogResponseBody = false
	return RequestLoggerMiddleware(config)
}

// GetRequestLogEntry 从上下文获取请求日志条目（用于扩展）
func GetRequestLogEntry(c *gin.Context) *RequestLogEntry {
	if entry, exists := c.Get("requestLogEntry"); exists {
		if e, ok := entry.(*RequestLogEntry); ok {
			return e
		}
	}
	return nil
}

// SetRequestLogEntry 设置请求日志条目到上下文（用于扩展）
func SetRequestLogEntry(c *gin.Context, entry *RequestLogEntry) {
	c.Set("requestLogEntry", entry)
}

// RequestIDMiddleware 请求 ID 中间件
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(b)
}