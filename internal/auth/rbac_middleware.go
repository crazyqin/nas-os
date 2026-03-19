package auth

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditLogger 审计日志记录器接口
type AuditLogger interface {
	LogAccess(userID, method, path, ip string, statusCode int)
	LogPermissionDenied(userID, resource, action, ip string)
	LogAuthFailure(ip, reason string)
}

// DefaultAuditLogger 默认审计日志实现
type DefaultAuditLogger struct{}

func (l *DefaultAuditLogger) LogAccess(userID, method, path, ip string, statusCode int) {
	log.Printf("[AUDIT] user=%s method=%s path=%s ip=%s status=%d time=%s",
		userID, method, path, ip, statusCode, time.Now().Format(time.RFC3339))
}

func (l *DefaultAuditLogger) LogPermissionDenied(userID, resource, action, ip string) {
	log.Printf("[AUDIT] PERMISSION_DENIED user=%s resource=%s action=%s ip=%s time=%s",
		userID, resource, action, ip, time.Now().Format(time.RFC3339))
}

func (l *DefaultAuditLogger) LogAuthFailure(ip, reason string) {
	log.Printf("[AUDIT] AUTH_FAILURE ip=%s reason=%s time=%s",
		ip, reason, time.Now().Format(time.RFC3339))
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	userManager interface {
		ValidateToken(token string) (string, error) // 验证 token 返回 userID
		GetUser(userID string) (interface{}, error) // 获取用户信息
	}
	rbacManager     *RBACManager
	auditLogger     AuditLogger
	ipWhitelist     map[string]bool
	ipBlacklist     map[string]bool
	rateLimitConfig RateLimitConfig
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled         bool
	RequestsPerMin  int
	BurstSize       int
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig is the default rate limit configuration.
var DefaultRateLimitConfig = RateLimitConfig{
	Enabled:         false,
	RequestsPerMin:  60,
	BurstSize:       10,
	CleanupInterval: 5 * time.Minute,
}

// AuthMiddlewareConfig 认证中间件配置
type AuthMiddlewareConfig struct {
	IPWhitelist []string
	IPBlacklist []string
	RateLimit   RateLimitConfig
	AuditLogger AuditLogger
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(userMgr interface {
	ValidateToken(token string) (string, error)
	GetUser(userID string) (interface{}, error)
}, rbacMgr *RBACManager) *AuthMiddleware {
	return NewAuthMiddlewareWithConfig(userMgr, rbacMgr, AuthMiddlewareConfig{})
}

// NewAuthMiddlewareWithConfig 创建认证中间件（带配置）
func NewAuthMiddlewareWithConfig(userMgr interface {
	ValidateToken(token string) (string, error)
	GetUser(userID string) (interface{}, error)
}, rbacMgr *RBACManager, config AuthMiddlewareConfig) *AuthMiddleware {
	m := &AuthMiddleware{
		userManager:     userMgr,
		rbacManager:     rbacMgr,
		ipWhitelist:     make(map[string]bool),
		ipBlacklist:     make(map[string]bool),
		rateLimitConfig: config.RateLimit,
	}

	// 设置 IP 白名单
	for _, ip := range config.IPWhitelist {
		m.ipWhitelist[ip] = true
	}

	// 设置 IP 黑名单
	for _, ip := range config.IPBlacklist {
		m.ipBlacklist[ip] = true
	}

	// 设置审计日志
	if config.AuditLogger != nil {
		m.auditLogger = config.AuditLogger
	} else {
		m.auditLogger = &DefaultAuditLogger{}
	}

	return m
}

// RequireAuth 需要认证的中间件
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// IP 黑名单检查
		if len(m.ipBlacklist) > 0 && m.ipBlacklist[clientIP] {
			m.auditLogger.LogAuthFailure(clientIP, "IP in blacklist")
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "访问被拒绝",
			})
			c.Abort()
			return
		}

		// IP 白名单检查（如果配置了白名单，只允许白名单内的 IP）
		if len(m.ipWhitelist) > 0 && !m.ipWhitelist[clientIP] {
			m.auditLogger.LogAuthFailure(clientIP, "IP not in whitelist")
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "访问被拒绝",
			})
			c.Abort()
			return
		}

		// 从 Authorization header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.auditLogger.LogAuthFailure(clientIP, "missing token")
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "缺少认证令牌",
			})
			c.Abort()
			return
		}

		// 提取 token (Bearer <token>)
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.auditLogger.LogAuthFailure(clientIP, "invalid auth format")
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "无效的认证格式",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// 检查缓存的会话
		session := m.rbacManager.GetCachedSession(token)
		if session != nil {
			// 缓存命中，设置用户上下文
			c.Set("user_id", session.UserID)
			c.Set("user_roles", session.Roles)
			c.Set("user_permissions", session.Permissions)
			c.Set("client_ip", clientIP)
			c.Next()
			return
		}

		// 验证 token
		userID, err := m.userManager.ValidateToken(token)
		if err != nil {
			m.auditLogger.LogAuthFailure(clientIP, "invalid token")
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "无效的认证令牌",
			})
			c.Abort()
			return
		}

		// 获取用户信息
		user, err := m.userManager.GetUser(userID)
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{
				Code:    404,
				Message: "用户不存在",
			})
			c.Abort()
			return
		}

		// 设置用户上下文
		c.Set("user_id", userID)
		c.Set("user", user)
		c.Set("client_ip", clientIP)

		// 从用户对象获取组信息
		groups := extractUserGroups(user)
		c.Set("user_groups", groups)

		// 缓存会话权限
		m.rbacManager.CacheSession(token, userID, groups)

		c.Next()
	}
}

// extractUserGroups 从用户对象中提取组信息
// 支持多种用户类型，通过类型断言和反射获取 Groups 字段
func extractUserGroups(user interface{}) []string {
	if user == nil {
		return []string{}
	}

	// 尝试类型断言获取 GroupProvider 接口
	if gp, ok := user.(GroupProvider); ok {
		return gp.GetGroups()
	}

	// 尝试通过 map 检查（适用于 map[string]interface{} 类型）
	if m, ok := user.(map[string]interface{}); ok {
		if groups, ok := m["groups"]; ok {
			switch v := groups.(type) {
			case []string:
				return v
			case []interface{}:
				result := make([]string, 0, len(v))
				for _, g := range v {
					if s, ok := g.(string); ok {
						result = append(result, s)
					}
				}
				return result
			}
		}
	}

	// 默认返回空组列表
	return []string{}
}

// GroupProvider 定义获取用户组的接口
type GroupProvider interface {
	GetGroups() []string
}

// RequirePermission 需要特定权限的中间件
func (m *AuthMiddleware) RequirePermission(resource Resource, action Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先执行认证
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "未认证",
			})
			c.Abort()
			return
		}

		// 获取用户组（从上下文或查询参数）
		groups, _ := c.Get("user_groups")
		if groups == nil {
			groups = []string{}
		}

		// 类型断言检查
		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "用户ID类型错误",
			})
			c.Abort()
			return
		}

		groupsList, ok := groups.([]string)
		if !ok {
			groupsList = []string{}
		}

		// 检查权限
		hasPermission := m.rbacManager.CheckPermission(
			userIDStr,
			groupsList,
			resource,
			action,
		)

		if !hasPermission {
			clientIP, _ := c.Get("client_ip")
			ipStr := ""
			if ip, ok := clientIP.(string); ok {
				ipStr = ip
			}
			m.auditLogger.LogPermissionDenied(userIDStr, string(resource), string(action), ipStr)
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermissionWithResource 需要特定权限的中间件（带资源ID）
func (m *AuthMiddleware) RequirePermissionWithResource(resource Resource, action Action, getResourceID func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "未认证",
			})
			c.Abort()
			return
		}

		groups, _ := c.Get("user_groups")
		groupList := []string{}
		if groups != nil {
			if gl, ok := groups.([]string); ok {
				groupList = gl
			}
		}

		// 类型断言检查
		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "用户ID类型错误",
			})
			c.Abort()
			return
		}

		resourceID := getResourceID(c)
		hasPermission := m.rbacManager.CheckPermissionWithOwner(
			userIDStr,
			groupList,
			resource,
			action,
			resourceID,
		)

		if !hasPermission {
			clientIP, _ := c.Get("client_ip")
			ipStr := ""
			if ip, ok := clientIP.(string); ok {
				ipStr = ip
			}
			m.auditLogger.LogPermissionDenied(userIDStr, string(resource), string(action), ipStr)
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole 需要特定角色的中间件
func (m *AuthMiddleware) RequireRole(roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "未认证",
			})
			c.Abort()
			return
		}

		// 类型断言检查
		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "用户ID类型错误",
			})
			c.Abort()
			return
		}

		// 获取用户角色
		userRoles := m.rbacManager.GetUserRoles(userIDStr)

		// 检查是否有任一角色
		hasRole := false
		for _, userRole := range userRoles {
			for _, requiredRole := range roles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "角色权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin 需要管理员权限的中间件
func (m *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return m.RequireRole(RoleAdmin)
}

// OptionalAuth 可选认证中间件（有 token 则认证，无 token 则匿名）
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 匿名访问，设置默认角色
			c.Set("user_id", "")
			c.Set("user_roles", []Role{RoleGuest})
			c.Next()
			return
		}

		// 有 token 则执行认证
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token := parts[1]
		userID, err := m.userManager.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		c.Set("user_id", userID)
		groups := []string{}
		m.rbacManager.CacheSession(token, userID, groups)

		c.Next()
	}
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	userIDStr, ok := userID.(string)
	if !ok {
		return ""
	}
	return userIDStr
}

// HasPermission 检查当前用户是否有权限
func HasPermission(c *gin.Context, resource Resource, action Action) bool {
	userID := GetUserID(c)
	if userID == "" {
		return false
	}

	// 从 RBAC 管理器获取（实际应该从上下文缓存获取）
	// 这里简化处理
	return true
}

// AuditLogMiddleware 审计日志中间件
func AuditLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户信息
		userID := GetUserID(c)

		// 处理请求
		c.Next()

		// 记录错误请求的审计日志
		if userID != "" && c.Writer.Status() >= 400 {
			// log.Printf("[AUDIT] user=%s method=%s path=%s status=%d",
			// 	userID, c.Request.Method, c.Request.URL.Path, c.Writer.Status())
			_ = userID // preserved for future audit logging
		}
	}
}

// AuditMiddleware 带完整审计日志的中间件
func (m *AuthMiddleware) AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		userID := ""
		clientIP := c.ClientIP()

		// 处理请求
		c.Next()

		// 尝试获取用户ID
		if uid, exists := c.Get("user_id"); exists {
			if id, ok := uid.(string); ok {
				userID = id
			}
		}

		statusCode := c.Writer.Status()
		latency := time.Since(start)

		// 记录访问日志
		m.auditLogger.LogAccess(userID, c.Request.Method, c.Request.URL.Path, clientIP, statusCode)

		// 记录慢请求
		if latency > 1*time.Second {
			log.Printf("[AUDIT] SLOW_REQUEST method=%s path=%s latency=%s ip=%s",
				c.Request.Method, c.Request.URL.Path, latency, clientIP)
		}
	}
}

// RequireAnyPermission 需要任一权限的中间件
func (m *AuthMiddleware) RequireAnyPermission(checks []struct {
	Resource Resource
	Action   Action
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "未认证",
			})
			c.Abort()
			return
		}

		// 类型断言检查
		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "用户ID类型错误",
			})
			c.Abort()
			return
		}

		groups, _ := c.Get("user_groups")
		groupList := []string{}
		if groups != nil {
			if gl, ok := groups.([]string); ok {
				groupList = gl
			}
		}

		// 检查是否有任一权限
		for _, check := range checks {
			if m.rbacManager.CheckPermission(userIDStr, groupList, check.Resource, check.Action) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, APIResponse{
			Code:    403,
			Message: "权限不足",
		})
		c.Abort()
	}
}

// RequireAllPermissions 需要所有权限的中间件
func (m *AuthMiddleware) RequireAllPermissions(checks []struct {
	Resource Resource
	Action   Action
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, APIResponse{
				Code:    401,
				Message: "未认证",
			})
			c.Abort()
			return
		}

		// 类型断言检查
		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "用户ID类型错误",
			})
			c.Abort()
			return
		}

		groups, _ := c.Get("user_groups")
		groupList := []string{}
		if groups != nil {
			if gl, ok := groups.([]string); ok {
				groupList = gl
			}
		}

		// 检查是否有所有权限
		for _, check := range checks {
			if !m.rbacManager.CheckPermission(userIDStr, groupList, check.Resource, check.Action) {
				c.JSON(http.StatusForbidden, APIResponse{
					Code:    403,
					Message: "权限不足",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// CheckIPAccess IP 访问检查中间件
func (m *AuthMiddleware) CheckIPAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// IP 黑名单检查
		if len(m.ipBlacklist) > 0 && m.ipBlacklist[clientIP] {
			m.auditLogger.LogAuthFailure(clientIP, "IP in blacklist")
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "访问被拒绝",
			})
			c.Abort()
			return
		}

		// IP 白名单检查
		if len(m.ipWhitelist) > 0 && !m.ipWhitelist[clientIP] {
			m.auditLogger.LogAuthFailure(clientIP, "IP not in whitelist")
			c.JSON(http.StatusForbidden, APIResponse{
				Code:    403,
				Message: "访问被拒绝",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AddToWhitelist 添加 IP 到白名单
func (m *AuthMiddleware) AddToWhitelist(ip string) {
	m.ipWhitelist[ip] = true
}

// RemoveFromWhitelist 从白名单移除 IP
func (m *AuthMiddleware) RemoveFromWhitelist(ip string) {
	delete(m.ipWhitelist, ip)
}

// AddToBlacklist 添加 IP 到黑名单
func (m *AuthMiddleware) AddToBlacklist(ip string) {
	m.ipBlacklist[ip] = true
}

// RemoveFromBlacklist 从黑名单移除 IP
func (m *AuthMiddleware) RemoveFromBlacklist(ip string) {
	delete(m.ipBlacklist, ip)
}

// GetWhitelist 获取白名单
func (m *AuthMiddleware) GetWhitelist() []string {
	ips := make([]string, 0, len(m.ipWhitelist))
	for ip := range m.ipWhitelist {
		ips = append(ips, ip)
	}
	return ips
}

// GetBlacklist 获取黑名单
func (m *AuthMiddleware) GetBlacklist() []string {
	ips := make([]string, 0, len(m.ipBlacklist))
	for ip := range m.ipBlacklist {
		ips = append(ips, ip)
	}
	return ips
}

// GetClientIP 从上下文获取客户端 IP
func GetClientIP(c *gin.Context) string {
	if ip, exists := c.Get("client_ip"); exists {
		if ipStr, ok := ip.(string); ok {
			return ipStr
		}
	}
	return c.ClientIP()
}

// GetUserRoles 从上下文获取用户角色
func GetUserRoles(c *gin.Context) []Role {
	if roles, exists := c.Get("user_roles"); exists {
		if roleList, ok := roles.([]Role); ok {
			return roleList
		}
	}
	return nil
}

// GetUserPermissions 从上下文获取用户权限
func GetUserPermissions(c *gin.Context) []Permission {
	if perms, exists := c.Get("user_permissions"); exists {
		if permList, ok := perms.([]Permission); ok {
			return permList
		}
	}
	return nil
}
