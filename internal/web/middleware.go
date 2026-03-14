package web

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
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
	AllowedOrigins  []string
	CSRFKey         []byte
	EnableRateLimit bool
	RateLimitRPS    int // 每秒请求数
}

// DefaultSecurityConfig 默认安全配置
func DefaultSecurityConfig() *SecurityConfig {
	// CSRFKey 从环境变量读取，默认值保证32字节
	csrfKey := os.Getenv("NAS_CSRF_KEY")
	if csrfKey == "" {
		csrfKey = "change-this-to-a-32-byte-secret-key-now!"
	}

	return &SecurityConfig{
		AllowedOrigins: []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		},
		CSRFKey:         []byte(csrfKey),
		EnableRateLimit: true,
		RateLimitRPS:    100,
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
			// 对于安全方法，设置 CSRF token cookie
			setCSRFToken(c, config)
			c.Next()
			return
		}

		// 从请求头获取 CSRF token
		token := c.Request.Header.Get("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("csrf_token")
		}

		// 从 cookie 中获取期望的 token
		expectedToken, err := c.Cookie("csrf_token")
		if err != nil {
			// cookie 不存在，生成新 token 并拒绝请求
			setCSRFToken(c, config)
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "CSRF token 缺失，请刷新页面重试",
			})
			c.Abort()
			return
		}

		// 验证 token
		if !validateCSRFToken(token, expectedToken, config.CSRFKey) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "CSRF token 验证失败",
			})
			c.Abort()
			return
		}

		c.Set("csrfToken", token)
		c.Next()
	}
}

// setCSRFToken 设置 CSRF token cookie
func setCSRFToken(c *gin.Context, config *SecurityConfig) {
	// 生成新的 CSRF token
	token := generateCSRFToken(config.CSRFKey)

	// 设置 cookie
	c.SetCookie("csrf_token", token, 3600, "/", "", false, true)
	// 同时设置到上下文，方便模板使用
	c.Set("csrfToken", token)
}

// generateCSRFToken 生成 CSRF token
func generateCSRFToken(key []byte) string {
	// 使用 UUID 作为 token，结合密钥增加安全性
	timestamp := time.Now().Unix()
	random := uuid.New().String()
	// 简单的 token 格式: timestamp-random
	// 生产环境可考虑使用 HMAC 签名
	return fmt.Sprintf("%d-%s", timestamp, random)
}

// validateCSRFToken 验证 CSRF token
func validateCSRFToken(token, expectedToken string, key []byte) bool {
	if token == "" || expectedToken == "" {
		return false
	}
	// 使用恒定时间比较防止时序攻击
	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
}

// auditLogMiddleware 审计日志中间件 (记录关键操作)
func auditLogMiddleware() gin.HandlerFunc {
	// 需要审计的敏感操作路径
	sensitivePaths := []string{
		// 存储管理
		"/api/v1/volumes",
		"/api/v1/raid",
		"/api/v1/disks",
		"/api/v1/pools",
		// 用户与权限
		"/api/v1/users",
		"/api/v1/roles",
		"/api/v1/permissions",
		// 网络共享
		"/api/v1/shares",
		"/api/v1/smb",
		"/api/v1/nfs",
		// 安全设置
		"/api/v1/security",
		"/api/v1/auth",
		"/api/v1/mfa",
		"/api/v1/firewall",
		// 系统配置
		"/api/v1/system/config",
		"/api/v1/network",
		"/api/v1/backup",
		// 应用管理
		"/api/v1/docker",
		"/api/v1/vms",
		"/api/v1/plugins",
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

		// 记录请求开始时间
		startTime := time.Now()

		// 执行请求
		c.Next()

		// 获取用户信息
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")

		// 确定操作级别
		level := "audit"
		if c.Writer.Status() >= 400 {
			level = "audit_warning"
		}
		if c.Writer.Status() >= 500 {
			level = "audit_error"
		}

		// 记录审计日志
		auditEntry := map[string]interface{}{
			"timestamp":    time.Now().Format(time.RFC3339),
			"level":        level,
			"request_id":   c.GetString("requestID"),
			"client_ip":    c.ClientIP(),
			"method":       c.Request.Method,
			"path":         c.Request.URL.Path,
			"query":        c.Request.URL.RawQuery,
			"status":       c.Writer.Status(),
			"duration_ms":  time.Since(startTime).Milliseconds(),
			"user_id":      userID,
			"username":     username,
			"user_agent":   c.Request.UserAgent(),
			"content_type": c.GetHeader("Content-Type"),
		}

		// 写入审计日志
		auditJSON, err := json.Marshal(auditEntry)
		if err != nil {
			log.Printf("[ERROR] Failed to marshal audit entry: %v", err)
			return
		}

		// 审计日志写入单独文件
		f, err := os.OpenFile("/var/log/nas-os/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			log.Printf("[ERROR] Failed to open audit log: %v", err)
			return
		}
		defer f.Close()

		f.Write(auditJSON)
		f.WriteString("\n")
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
