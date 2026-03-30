package smb

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RateLimitMiddleware SMB连接限流中间件.
type RateLimitMiddleware struct {
	sm       *SecurityManager
	logger   *zap.SugaredLogger
	ipLimits map[string]*IPRateLimiter
	mu       sync.RWMutex
}

// IPRateLimiter 单IP限流器.
type IPRateLimiter struct {
	ip         string
	maxConn    int
	windowSize time.Duration
	attempts   []time.Time
	mu         sync.Mutex
}

// NewRateLimitMiddleware 创建限流中间件.
func NewRateLimitMiddleware(sm *SecurityManager, logger *zap.SugaredLogger) *RateLimitMiddleware {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &RateLimitMiddleware{
		sm:       sm,
		logger:   logger,
		ipLimits: make(map[string]*IPRateLimiter),
	}
}

// Middleware 返回Gin中间件.
func (rlm *RateLimitMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取客户端IP
		ip := getClientIP(c)

		// 检查IP是否允许访问
		allowed, reason := rlm.sm.CheckIPAllowed(ip)
		if !allowed {
			rlm.logger.Warnw("IP访问被拒绝", "ip", ip, "reason", reason)
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": reason,
			})
			c.Abort()
			return
		}

		// 检查限流
		allowed, reason = rlm.sm.CheckRateLimit(ip)
		if !allowed {
			rlm.logger.Warnw("IP被限流", "ip", ip, "reason", reason)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": reason,
			})
			c.Abort()
			return
		}

		// 增加连接计数
		rlm.sm.IncrementConnection(ip)

		// 在响应完成后减少计数
		defer func() {
			rlm.sm.DecrementConnection(ip)
		}()

		c.Next()
	}
}

// getClientIP 获取客户端IP.
func getClientIP(c *gin.Context) string {
	// 优先从X-Forwarded-For获取
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		// 取第一个IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 从X-Real-IP获取
	xri := c.GetHeader("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// 使用RemoteAddr
	return c.ClientIP()
}

// SMBConnectionTracker SMB连接跟踪器（用于smbd进程级别的限流）.
type SMBConnectionTracker struct {
	sm          *SecurityManager
	logger      *zap.SugaredLogger
	connections map[string]*SMBConnection
	mu          sync.RWMutex
}

// SMBConnection SMB连接信息.
type SMBConnection struct {
	ID         string
	ClientIP   string
	Username   string
	ShareName  string
	StartTime  time.Time
	LastActive time.Time
}

// NewSMBConnectionTracker 创建连接跟踪器.
func NewSMBConnectionTracker(sm *SecurityManager, logger *zap.SugaredLogger) *SMBConnectionTracker {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	tracker := &SMBConnectionTracker{
		sm:          sm,
		logger:      logger,
		connections: make(map[string]*SMBConnection),
	}

	// 启动清理协程
	go tracker.cleanupLoop()

	return tracker
}

// TrackConnection 开始跟踪连接.
func (ct *SMBConnectionTracker) TrackConnection(connID, clientIP, username, shareName string) error {
	// 检查IP是否允许
	allowed, reason := ct.sm.CheckIPAllowed(clientIP)
	if !allowed {
		ct.logger.Warnw("SMB连接被拒绝", "conn_id", connID, "ip", clientIP, "reason", reason)
		return fmt.Errorf("连接被拒绝: %s", reason)
	}

	// 检查限流
	allowed, reason = ct.sm.CheckRateLimit(clientIP)
	if !allowed {
		ct.logger.Warnw("SMB连接被限流", "conn_id", connID, "ip", clientIP, "reason", reason)
		return fmt.Errorf("连接被限流: %s", reason)
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	conn := &SMBConnection{
		ID:         connID,
		ClientIP:   clientIP,
		Username:   username,
		ShareName:  shareName,
		StartTime:  time.Now(),
		LastActive: time.Now(),
	}

	ct.connections[connID] = conn
	ct.sm.IncrementConnection(clientIP)

	ct.logger.Infow("跟踪SMB连接", "conn_id", connID, "ip", clientIP,
		"username", username, "share", shareName)

	// 记录审计日志
	ct.sm.LogAccess(clientIP, username, shareName, "connect", "success", "")

	return nil
}

// UntrackConnection 结束跟踪连接.
func (ct *SMBConnectionTracker) UntrackConnection(connID string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	conn, exists := ct.connections[connID]
	if !exists {
		return
	}

	ct.sm.DecrementConnection(conn.ClientIP)
	delete(ct.connections, connID)

	ct.logger.Infow("结束SMB连接跟踪", "conn_id", connID, "ip", conn.ClientIP,
		"duration", time.Since(conn.StartTime).String())

	// 记录审计日志
	ct.sm.LogAccess(conn.ClientIP, conn.Username, conn.ShareName, "disconnect", "success",
		fmt.Sprintf("duration: %s", time.Since(conn.StartTime).String()))
}

// GetConnection 获取连接信息.
func (ct *SMBConnectionTracker) GetConnection(connID string) *SMBConnection {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.connections[connID]
}

// GetConnections 获取所有连接.
func (ct *SMBConnectionTracker) GetConnections() []*SMBConnection {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make([]*SMBConnection, 0, len(ct.connections))
	for _, conn := range ct.connections {
		result = append(result, conn)
	}
	return result
}

// GetConnectionsByIP 获取特定IP的连接.
func (ct *SMBConnectionTracker) GetConnectionsByIP(ip string) []*SMBConnection {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make([]*SMBConnection, 0)
	for _, conn := range ct.connections {
		if conn.ClientIP == ip {
			result = append(result, conn)
		}
	}
	return result
}

// UpdateActivity 更新连接活动时间.
func (ct *SMBConnectionTracker) UpdateActivity(connID string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if conn, exists := ct.connections[connID]; exists {
		conn.LastActive = time.Now()
	}
}

// cleanupLoop 定期清理过期连接.
func (ct *SMBConnectionTracker) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ct.cleanupStaleConnections()
		ct.sm.CleanupExpiredBans()
	}
}

// cleanupStaleConnections 清理过期连接.
func (ct *SMBConnectionTracker) cleanupStaleConnections() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	now := time.Now()
	staleThreshold := 30 * time.Minute // 30分钟无活动的连接视为过期

	for connID, conn := range ct.connections {
		if now.Sub(conn.LastActive) > staleThreshold {
			ct.sm.DecrementConnection(conn.ClientIP)
			delete(ct.connections, connID)

			ct.logger.Warnw("清理过期SMB连接", "conn_id", connID, "ip", conn.ClientIP,
				"last_active", conn.LastActive)

			ct.sm.LogAccess(conn.ClientIP, conn.Username, conn.ShareName, "cleanup",
				"timeout", fmt.Sprintf("stale: %s", time.Since(conn.LastActive).String()))
		}
	}
}

// SMBAuthHandler SMB认证处理器.
type SMBAuthHandler struct {
	sm     *SecurityManager
	logger *zap.SugaredLogger
}

// NewSMBAuthHandler 创建认证处理器.
func NewSMBAuthHandler(sm *SecurityManager, logger *zap.SugaredLogger) *SMBAuthHandler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &SMBAuthHandler{
		sm:     sm,
		logger: logger,
	}
}

// OnAuthSuccess 认证成功回调.
func (ah *SMBAuthHandler) OnAuthSuccess(clientIP, username string) {
	ah.logger.Infow("SMB认证成功", "ip", clientIP, "username", username)

	// 记录审计日志
	ah.sm.LogAccess(clientIP, username, "", "authenticate", "success", "")
}

// OnAuthFailure 认证失败回调.
func (ah *SMBAuthHandler) OnAuthFailure(clientIP, username, reason string) {
	ah.logger.Warnw("SMB认证失败", "ip", clientIP, "username", username, "reason", reason)

	// 记录失败尝试（用于自动封禁）
	ah.sm.RecordFailedAttempt(clientIP, username, reason)
}

// SMBShareAccessHandler SMB共享访问处理器.
type SMBShareAccessHandler struct {
	sm     *SecurityManager
	logger *zap.SugaredLogger
}

// NewSMBShareAccessHandler 创建共享访问处理器.
func NewSMBShareAccessHandler(sm *SecurityManager, logger *zap.SugaredLogger) *SMBShareAccessHandler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &SMBShareAccessHandler{
		sm:     sm,
		logger: logger,
	}
}

// OnShareAccess 共享访问回调.
func (sh *SMBShareAccessHandler) OnShareAccess(clientIP, username, shareName, operation string) bool {
	// 检查IP是否允许
	allowed, reason := sh.sm.CheckIPAllowed(clientIP)
	if !allowed {
		sh.logger.Warnw("共享访问被拒绝", "ip", clientIP, "share", shareName, "reason", reason)
		sh.sm.LogAccess(clientIP, username, shareName, operation, "denied", reason)
		return false
	}

	// 记录审计日志
	sh.sm.LogAccess(clientIP, username, shareName, operation, "success", "")
	return true
}

// OnFileOperation 文件操作回调.
func (sh *SMBShareAccessHandler) OnFileOperation(clientIP, username, shareName, filePath, operation string) {
	// 记录敏感文件操作审计日志
	sensitiveOps := map[string]bool{
		"delete":   true,
		"rename":   true,
		"chmod":    true,
		"chown":    true,
		"write":    true,
		"upload":   true,
		"download": true,
	}

	if sensitiveOps[operation] {
		sh.sm.LogAccess(clientIP, username, shareName, operation, "success", filePath)
	}
}

// SecurityHandlers 安全相关API处理器.
type SecurityHandlers struct {
	sm *SecurityManager
}

// NewSecurityHandlers 创建安全处理器.
func NewSecurityHandlers(sm *SecurityManager) *SecurityHandlers {
	return &SecurityHandlers{sm: sm}
}

// RegisterRoutes 注册安全相关路由.
func (sh *SecurityHandlers) RegisterRoutes(api *gin.RouterGroup) {
	security := api.Group("/smb/security")
	{
		security.GET("/config", sh.getConfig)
		security.PUT("/config", sh.updateConfig)
		security.GET("/banned", sh.getBannedIPs)
		security.DELETE("/banned/:ip", sh.unbanIP)
		security.POST("/ban", sh.banIP)
		security.GET("/audit", sh.getAuditLogs)
		security.GET("/whitelist", sh.getWhitelist)
		security.POST("/whitelist", sh.addToWhitelist)
		security.DELETE("/whitelist/:ip", sh.removeFromWhitelist)
		security.GET("/blacklist", sh.getBlacklist)
		security.POST("/blacklist", sh.addToBlacklist)
		security.DELETE("/blacklist/:ip", sh.removeFromBlacklist)
		security.GET("/stats", sh.getSecurityStats)
	}
}

// getConfig 获取安全配置.
func (sh *SecurityHandlers) getConfig(c *gin.Context) {
	config := sh.sm.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// updateConfig 更新安全配置.
func (sh *SecurityHandlers) updateConfig(c *gin.Context) {
	var cfg SMBSecurityConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := sh.sm.UpdateConfig(&cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getBannedIPs 获取封禁IP列表.
func (sh *SecurityHandlers) getBannedIPs(c *gin.Context) {
	banned := sh.sm.GetBannedIPs()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    banned,
	})
}

// unbanIP 解封IP.
func (sh *SecurityHandlers) unbanIP(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "IP地址不能为空",
		})
		return
	}

	if err := sh.sm.UnbanIP(ip); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// banIP 封禁IP.
func (sh *SecurityHandlers) banIP(c *gin.Context) {
	var req struct {
		IP           string `json:"ip" binding:"required"`
		Reason       string `json:"reason"`
		DurationMins int    `json:"duration_mins"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Reason == "" {
		req.Reason = "手动封禁"
	}

	if req.DurationMins == 0 {
		req.DurationMins = 60 // 默认1小时
	}

	if err := sh.sm.BanIP(req.IP, req.Reason, req.DurationMins); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getAuditLogs 获取审计日志.
func (sh *SecurityHandlers) getAuditLogs(c *gin.Context) {
	limit := 100
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	logs, err := sh.sm.GetAuditLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    logs,
	})
}

// getWhitelist 获取白名单.
func (sh *SecurityHandlers) getWhitelist(c *gin.Context) {
	config := sh.sm.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config.IPWhitelist,
	})
}

// addToWhitelist 添加白名单.
func (sh *SecurityHandlers) addToWhitelist(c *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := sh.sm.AddToWhitelist(req.IP); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// removeFromWhitelist 移除白名单.
func (sh *SecurityHandlers) removeFromWhitelist(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "IP地址不能为空",
		})
		return
	}

	if err := sh.sm.RemoveFromWhitelist(ip); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getBlacklist 获取黑名单.
func (sh *SecurityHandlers) getBlacklist(c *gin.Context) {
	config := sh.sm.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config.IPBlacklist,
	})
}

// addToBlacklist 添加黑名单.
func (sh *SecurityHandlers) addToBlacklist(c *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := sh.sm.AddToBlacklist(req.IP); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// removeFromBlacklist 移除黑名单.
func (sh *SecurityHandlers) removeFromBlacklist(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "IP地址不能为空",
		})
		return
	}

	if err := sh.sm.RemoveFromBlacklist(ip); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getSecurityStats 获取安全统计.
func (sh *SecurityHandlers) getSecurityStats(c *gin.Context) {
	banned := sh.sm.GetBannedIPs()
	config := sh.sm.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"banned_count":       len(banned),
			"whitelist_count":    len(config.IPWhitelist),
			"blacklist_count":    len(config.IPBlacklist),
			"rate_limit_enabled": config.RateLimit.Enabled,
			"audit_enabled":      config.AuditEnabled,
			"auto_ban_enabled":   config.AutoBanEnabled,
		},
	})
}

// parseInt 解析整数.
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
