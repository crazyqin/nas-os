package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SecurityConfig 安全配置
type SecurityConfig struct {
	AllowedOrigins []string
	CSRFKey        []byte
	EnableRateLimit bool
	RateLimitRPS   int // 每秒请求数
}

// DefaultSecurityConfig 默认安全配置
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		AllowedOrigins: []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		},
		CSRFKey:        []byte("change-this-to-a-32-byte-secret-key-now!"), // TODO: 从环境变量读取
		EnableRateLimit: true,
		RateLimitRPS:   100,
	}
}

// loggerMiddleware 结构化日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成请求追踪 ID
		requestID := uuid.New().String()
		c.Set("requestID", requestID)
		c.Set("startTime", time.Now())

		start := time.Now()
		path := c.Request.URL.Path

		// 执行请求
		c.Next()

		// 计算耗时
		duration := time.Since(start)

		// 结构化日志
		logEntry := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"level":       "info",
			"request_id":  requestID,
			"client_ip":   c.ClientIP(),
			"method":      c.Request.Method,
			"path":        path,
			"status":      c.Writer.Status(),
			"duration_ms": duration.Milliseconds(),
			"user_agent":  c.Request.UserAgent(),
		}

		// JSON 格式输出
		logJSON, err := json.Marshal(logEntry)
		if err != nil {
			log.Printf("[ERROR] Failed to marshal log entry: %v", err)
			return
		}

		// 写入日志文件和控制台
		os.Stdout.Write(logJSON)
		os.Stdout.WriteString("\n")

		// 错误级别日志
		if c.Writer.Status() >= 500 {
			logEntry["level"] = "error"
			logJSON, _ := json.Marshal(logEntry)
			os.Stderr.Write(logJSON)
			os.Stderr.WriteString("\n")
		}
	}
}

// corsMiddleware CORS 跨域中间件 (加固版)
func corsMiddleware(config *SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查是否在允许的源列表中
		allowed := false
		for _, allowedOrigin := range config.AllowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}

		if !allowed {
			// 如果是 OPTIONS 预检请求，仍然允许但不设置具体源
			if c.Request.Method == "OPTIONS" {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 小时

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// securityHeadersMiddleware 安全头中间件
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 防止点击劫持
		c.Header("X-Frame-Options", "DENY")

		// XSS 防护
		c.Header("X-XSS-Protection", "1; mode=block")

		// 内容安全策略
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'")

		// HSTS (仅 HTTPS)
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Referrer 策略
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 权限策略
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// rateLimitMiddleware 简单的速率限制中间件
// 生产环境建议使用 redis 或 memcached 实现分布式限流
func rateLimitMiddleware(config *SecurityConfig) gin.HandlerFunc {
	if !config.EnableRateLimit {
		return func(c *gin.Context) { c.Next() }
	}

	// 简单的内存限流 (生产环境请用 Redis)
	type clientRateLimit struct {
		count     int
		resetTime time.Time
	}

	clients := make(map[string]*clientRateLimit)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		client, exists := clients[clientIP]
		if !exists || now.After(client.resetTime) {
			clients[clientIP] = &clientRateLimit{
				count:     1,
				resetTime: now.Add(time.Second),
			}
			c.Next()
			return
		}

		if client.count >= config.RateLimitRPS {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		client.count++
		c.Next()
	}
}

// csrfMiddleware CSRF 保护中间件
func csrfMiddleware(config *SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只对状态修改操作进行验证
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 从请求头获取 CSRF token
		token := c.Request.Header.Get("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("csrf_token")
		}

		// TODO: 验证 token (需要从 session 或 cookie 中获取期望的 token)
		// 这里提供框架，具体实现需要配合认证系统
		// if !validateCSRFToken(token) {
		//     c.JSON(http.StatusForbidden, gin.H{
		//         "code":    403,
		//         "message": "CSRF token 验证失败",
		//     })
		//     c.Abort()
		//     return
		// }

		c.Set("csrfToken", token)
		c.Next()
	}
}

// auditLogMiddleware 审计日志中间件 (记录关键操作)
func auditLogMiddleware() gin.HandlerFunc {
	// 需要审计的敏感操作路径
	sensitivePaths := []string{
		"/api/v1/volumes",
		"/api/v1/users",
		"/api/v1/shares",
		"/api/v1/raid",
	}

	return func(c *gin.Context) {
		// 只记录敏感操作
		isSensitive := false
		for _, path := range sensitivePaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				isSensitive = true
				break
			}
		}

		if !isSensitive {
			c.Next()
			return
		}

		// 记录审计日志
		auditEntry := map[string]interface{}{
			"timestamp":  time.Now().Format(time.RFC3339),
			"level":      "audit",
			"request_id": c.GetString("requestID"),
			"client_ip":  c.ClientIP(),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"user_agent": c.Request.UserAgent(),
		}

		// 写入审计日志
		auditJSON, err := json.Marshal(auditEntry)
		if err != nil {
			log.Printf("[ERROR] Failed to marshal audit entry: %v", err)
			c.Next()
			return
		}

		// 审计日志写入单独文件
		f, err := os.OpenFile("/var/log/nas-os/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[ERROR] Failed to open audit log: %v", err)
			c.Next()
			return
		}
		defer f.Close()

		f.Write(auditJSON)
		f.WriteString("\n")

		c.Next()
	}
}

// inputValidationMiddleware 输入验证中间件
func inputValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证 Content-Type
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" && !strings.Contains(contentType, "application/json") {
				// 允许其他类型但记录日志
				log.Printf("[WARN] Non-JSON content type: %s", contentType)
			}
		}

		// 验证 URL 长度 (防止过长 URL 攻击)
		if len(c.Request.URL.String()) > 2048 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "URL 过长",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
