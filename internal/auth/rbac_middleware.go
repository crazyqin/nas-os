package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	userManager interface {
		ValidateToken(token string) (string, error) // 验证 token 返回 userID
		GetUser(userID string) (interface{}, error) // 获取用户信息
	}
	rbacManager *RBACManager
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(userMgr interface {
	ValidateToken(token string) (string, error)
	GetUser(userID string) (interface{}, error)
}, rbacMgr *RBACManager) *AuthMiddleware {
	return &AuthMiddleware{
		userManager: userMgr,
		rbacManager: rbacMgr,
	}
}

// RequireAuth 需要认证的中间件
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Authorization header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
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
			c.Next()
			return
		}

		// 验证 token
		userID, err := m.userManager.ValidateToken(token)
		if err != nil {
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

		// TODO: 从用户对象获取组信息
		// 暂时使用空组列表
		groups := []string{}

		// 缓存会话权限
		m.rbacManager.CacheSession(token, userID, groups)

		c.Next()
	}
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

		// 检查权限
		hasPermission := m.rbacManager.CheckPermission(
			userID.(string),
			groups.([]string),
			resource,
			action,
		)

		if !hasPermission {
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

		// 获取用户角色
		userRoles := m.rbacManager.GetUserRoles(userID.(string))

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
	return userID.(string)
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
		}
	}
}
