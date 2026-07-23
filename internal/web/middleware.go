package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SecurityConfig 安全配置.
type SecurityConfig struct {
	AllowedOrigins  []string
	CSRFKey         []byte
	EnableRateLimit bool
	RateLimitRPS    int // 每秒请求数
}

// DefaultSecurityConfig 默认安全配置.
func DefaultSecurityConfig() *SecurityConfig {
	// CSRFKey 从环境变量读取。生产/严格模式必须显式设置，禁止临时密钥。
	csrfKey := os.Getenv("NAS_CSRF_KEY")
	require := os.Getenv("NAS_OS_REQUIRE_CSRF_KEY") == "1" ||
		os.Getenv("NAS_OS_ENV") == "production" ||
		os.Getenv("NAS_OS_ENV") == "prod"
	if csrfKey == "" {
		if require {
			panic("❌ [SECURITY CRITICAL] NAS_CSRF_KEY is required when NAS_OS_ENV=production or NAS_OS_REQUIRE_CSRF_KEY=1")
		}
		log.Println("⚠️  [SECURITY WARNING] NAS_CSRF_KEY 未设置，使用进程内临时密钥（多实例不安全）")
		log.Println("⚠️  生产请设置 NAS_CSRF_KEY，或设置 NAS_OS_ENV=production 强制失败")
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			panic(fmt.Sprintf("❌ [SECURITY CRITICAL] 无法生成 CSRF 随机密钥: %v。请设置 NAS_CSRF_KEY", err))
		}
		csrfKey = hex.EncodeToString(keyBytes)
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

// slowRequestThreshold logs 2xx/3xx under this duration only at sample rate.
const slowRequestThreshold = 500 * time.Millisecond

// accessLogSampleN: log 1 of every N successful fast requests (0 = always).
// Overridable via NAS_ACCESS_LOG_SAMPLE (e.g. "1" always, "20" = 5%).
var accessLogSampleN = func() uint64 {
	if v := os.Getenv("NAS_ACCESS_LOG_SAMPLE"); v != "" {
		var n uint64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return 10
}()

var accessLogCounter atomic.Uint64

// accessLogger is a process-wide zap logger for HTTP access lines (lazy init).
var accessLogger = func() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	l, err := cfg.Build()
	if err != nil {
		return zap.NewNop()
	}
	return l
}()

// loggerMiddleware 结构化访问日志（zap + 成功快路径采样）.
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("requestID", requestID)
		c.Set("startTime", time.Now())

		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)
		ms := duration.Milliseconds()

		// Always log errors and slow requests; sample the rest.
		shouldLog := status >= 400 || duration >= slowRequestThreshold
		if !shouldLog {
			n := accessLogCounter.Add(1)
			if accessLogSampleN == 0 || n%accessLogSampleN != 0 {
				return
			}
		}

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("client_ip", c.ClientIP()),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Int64("duration_ms", ms),
			zap.String("user_agent", c.Request.UserAgent()),
		}
		if status >= 500 {
			accessLogger.Error("http_request", fields...)
		} else if status >= 400 || duration >= slowRequestThreshold {
			accessLogger.Warn("http_request", fields...)
		} else {
			accessLogger.Info("http_request", fields...)
		}
	}
}

// corsMiddleware CORS 跨域中间件 (加固版).
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

		if !allowed && origin != "" {
			// 不在白名单的 Origin，拒绝请求
			log.Printf("⚠️  [SECURITY] 拒绝跨域请求，Origin 不在白名单: %s", origin)
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			// 对于非预检请求，继续处理但不设置 CORS 头
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

// securityHeadersMiddleware 安全头中间件.
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
// 生产环境建议使用 redis 或 memcached 实现分布式限流.
// Concurrency-safe: protects the clients map with a mutex and prunes expired entries.
func rateLimitMiddleware(config *SecurityConfig) gin.HandlerFunc {
	if !config.EnableRateLimit {
		return func(c *gin.Context) { c.Next() }
	}

	type clientRateLimit struct {
		count     int
		resetTime time.Time
	}

	var mu sync.Mutex
	clients := make(map[string]*clientRateLimit)
	lastPrune := time.Now()

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		mu.Lock()
		// Periodic prune to avoid unbounded growth (every ~60s)
		if now.Sub(lastPrune) > time.Minute {
			for ip, cl := range clients {
				if now.After(cl.resetTime) {
					delete(clients, ip)
				}
			}
			lastPrune = now
		}

		client, exists := clients[clientIP]
		if !exists || now.After(client.resetTime) {
			clients[clientIP] = &clientRateLimit{
				count:     1,
				resetTime: now.Add(time.Second),
			}
			mu.Unlock()
			c.Next()
			return
		}

		if client.count >= config.RateLimitRPS {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		client.count++
		mu.Unlock()
		c.Next()
	}
}

// csrfMiddleware CSRF 保护中间件.
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

// setCSRFToken 设置 CSRF token cookie.
func setCSRFToken(c *gin.Context, config *SecurityConfig) {
	token := generateCSRFToken(config.CSRFKey)

	// Secure when request is TLS (or behind TLS-terminating proxy with X-Forwarded-Proto)
	secure := c.Request.TLS != nil ||
		strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")

	// SameSite=Strict via http.Cookie for clearer attributes than SetCookie alone
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
	c.Set("csrfToken", token)
}

// generateCSRFToken builds an HMAC-SHA256 signed token: ts.nonce.sig
// The CSRFKey is required material; forging tokens without the key fails validation.
func generateCSRFToken(key []byte) string {
	timestamp := time.Now().Unix()
	nonce := uuid.New().String()
	payload := fmt.Sprintf("%d.%s", timestamp, nonce)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyCSRFSignature checks token structure and HMAC against key.
func verifyCSRFSignature(token string, key []byte) bool {
	if token == "" || len(key) == 0 {
		return false
	}
	// format: <unix>.<uuid>.<hex-hmac>
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) == 1
}

// validateCSRFToken requires header token == cookie token AND a valid HMAC signature.
func validateCSRFToken(token, expectedToken string, key []byte) bool {
	if token == "" || expectedToken == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return false
	}
	return verifyCSRFSignature(token, key)
}

// auditLogMiddleware 审计日志中间件 (记录关键操作).
func auditLogMiddleware() gin.HandlerFunc {
	// 需要审计的敏感操作路径
	sensitivePaths := []string{
		// 存储管理
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
		defer func() { _ = f.Close() }()

		_, _ = f.Write(auditJSON)
		_, _ = f.WriteString("\n")
	}
}

// inputValidationMiddleware 输入验证中间件.
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
