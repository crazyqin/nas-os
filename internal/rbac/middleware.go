package rbac

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// contextKey 上下文键类型
type contextKey string

const (
	// UserIDKey 用户ID上下文键
	UserIDKey contextKey = "userID"
	// UsernameKey 用户名上下文键
	UsernameKey contextKey = "username"
	// UserRoleKey 用户角色上下文键
	UserRoleKey contextKey = "userRole"
	// PermissionKey 权限结果上下文键
	PermissionKey contextKey = "permission"
)

// UserInfo 用户信息接口
type UserInfo interface {
	GetUserID() string
	GetUsername() string
	GetRole() Role
}

// UserProvider 用户提供者接口
type UserProvider interface {
	GetUserByID(userID string) (UserInfo, error)
	GetUserByUsername(username string) (UserInfo, error)
}

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// SkipPaths 跳过权限检查的路径
	SkipPaths []string
	// PublicPaths 公开路径（无需登录）
	PublicPaths []string
	// TokenExtractor 令牌提取函数
	TokenExtractor func(r *http.Request) string
	// UserLoader 用户加载函数
	UserLoader func(token string) (UserInfo, error)
	// OnDenied 权限拒绝时的处理函数
	OnDenied func(w http.ResponseWriter, r *http.Request, result *CheckResult)
	// OnError 错误处理函数
	OnError func(w http.ResponseWriter, r *http.Request, err error)
}

// DefaultMiddlewareConfig 默认中间件配置
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		SkipPaths: []string{
			"/health",
			"/api/health",
			"/metrics",
		},
		PublicPaths: []string{
			"/api/auth/login",
			"/api/auth/logout",
			"/api/auth/refresh",
		},
		TokenExtractor: ExtractBearerToken,
		OnDenied:       DefaultDeniedHandler,
		OnError:        DefaultErrorHandler,
	}
}

// Middleware 权限检查中间件
type Middleware struct {
	manager *Manager
	config  MiddlewareConfig
}

// NewMiddleware 创建权限中间件
func NewMiddleware(manager *Manager, config MiddlewareConfig) *Middleware {
	return &Middleware{
		manager: manager,
		config:  config,
	}
}

// Handler 返回中间件处理器
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过指定路径
		for _, path := range m.config.SkipPaths {
			if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 公开路径无需认证
		for _, path := range m.config.PublicPaths {
			if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 提取令牌
		token := m.config.TokenExtractor(r)
		if token == "" {
			m.config.OnError(w, r, ErrPermissionDenied)
			return
		}

		// 加载用户
		user, err := m.config.UserLoader(token)
		if err != nil {
			m.config.OnError(w, r, err)
			return
		}

		// 将用户信息存入上下文
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, user.GetUserID())
		ctx = context.WithValue(ctx, UsernameKey, user.GetUsername())
		ctx = context.WithValue(ctx, UserRoleKey, user.GetRole())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission 创建权限检查中间件
func (m *Middleware) RequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserIDFromContext(r.Context())
			if userID == "" {
				m.config.OnError(w, r, ErrPermissionDenied)
				return
			}

			result := m.manager.CheckPermission(userID, resource, action)
			if !result.Allowed {
				m.config.OnDenied(w, r, result)
				return
			}

			// 将权限结果存入上下文
			ctx := context.WithValue(r.Context(), PermissionKey, result)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 创建角色检查中间件
func (m *Middleware) RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetRoleFromContext(r.Context())
			if userRole == "" {
				m.config.OnError(w, r, ErrPermissionDenied)
				return
			}

			allowed := false
			for _, role := range roles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				result := &CheckResult{
					Allowed: false,
					Reason:  "角色不足",
				}
				m.config.OnDenied(w, r, result)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin 管理员权限检查
func (m *Middleware) RequireAdmin() func(http.Handler) http.Handler {
	return m.RequireRole(RoleAdmin)
}

// RequireOperator 运维员及以上权限检查
func (m *Middleware) RequireOperator() func(http.Handler) http.Handler {
	return m.RequireRole(RoleAdmin, RoleOperator)
}

// ========== 上下文辅助函数 ==========

// GetUserIDFromContext 从上下文获取用户ID
func GetUserIDFromContext(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUsernameFromContext 从上下文获取用户名
func GetUsernameFromContext(ctx context.Context) string {
	if v := ctx.Value(UsernameKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRoleFromContext 从上下文获取用户角色
func GetRoleFromContext(ctx context.Context) Role {
	if v := ctx.Value(UserRoleKey); v != nil {
		return v.(Role)
	}
	return ""
}

// GetPermissionResultFromContext 从上下文获取权限检查结果
func GetPermissionResultFromContext(ctx context.Context) *CheckResult {
	if v := ctx.Value(PermissionKey); v != nil {
		return v.(*CheckResult)
	}
	return nil
}

// ========== 令牌提取器 ==========

// ExtractBearerToken 从 Authorization 头提取 Bearer 令牌
func ExtractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

// ExtractCookieToken 从 Cookie 提取令牌
func ExtractCookieToken(cookieName string) func(r *http.Request) string {
	return func(r *http.Request) string {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			return ""
		}
		return cookie.Value
	}
}

// ExtractQueryToken 从查询参数提取令牌
func ExtractQueryToken(paramName string) func(r *http.Request) string {
	return func(r *http.Request) string {
		return r.URL.Query().Get(paramName)
	}
}

// ========== 默认处理器 ==========

// DefaultDeniedHandler 默认权限拒绝处理器
func DefaultDeniedHandler(w http.ResponseWriter, r *http.Request, result *CheckResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	response := map[string]interface{}{
		"code":    403,
		"message": result.Reason,
	}

	if result.DeniedBy != "" {
		response["denied_by"] = result.DeniedBy
	}
	if len(result.MissingPerms) > 0 {
		response["missing_permissions"] = result.MissingPerms
	}

	json.NewEncoder(w).Encode(response)
}

// DefaultErrorHandler 默认错误处理器
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")

	if err == ErrPermissionDenied {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    401,
			"message": "未授权访问",
		})
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    500,
		"message": err.Error(),
	})
}

// ========== 权限装饰器 ==========

// PermissionFunc 带权限检查的处理函数
type PermissionFunc func(w http.ResponseWriter, r *http.Request)

// WithPermission 为处理函数添加权限检查
func (m *Middleware) WithPermission(resource, action string, handler PermissionFunc) PermissionFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r.Context())
		if userID == "" {
			m.config.OnError(w, r, ErrPermissionDenied)
			return
		}

		result := m.manager.CheckPermission(userID, resource, action)
		if !result.Allowed {
			m.config.OnDenied(w, r, result)
			return
		}

		handler(w, r)
	}
}

// ========== 资源级权限检查 ==========

// ResourcePermission 资源权限检查器
type ResourcePermission struct {
	manager    *Manager
	middleware *Middleware
}

// NewResourcePermission 创建资源权限检查器
func NewResourcePermission(manager *Manager, middleware *Middleware) *ResourcePermission {
	return &ResourcePermission{
		manager:    manager,
		middleware: middleware,
	}
}

// CanRead 检查读权限
func (rp *ResourcePermission) CanRead(resource string) func(http.Handler) http.Handler {
	return rp.middleware.RequirePermission(resource, "read")
}

// CanWrite 检查写权限
func (rp *ResourcePermission) CanWrite(resource string) func(http.Handler) http.Handler {
	return rp.middleware.RequirePermission(resource, "write")
}

// CanAdmin 检查管理权限
func (rp *ResourcePermission) CanAdmin(resource string) func(http.Handler) http.Handler {
	return rp.middleware.RequirePermission(resource, "admin")
}

// ========== 批量权限检查 ==========

// CheckMultiple 批量检查权限
func (m *Manager) CheckMultiple(userID string, checks []struct{ Resource, Action string }) map[string]bool {
	results := make(map[string]bool)
	for _, check := range checks {
		key := PermissionString(check.Resource, check.Action)
		results[key] = m.CheckPermissionFast(userID, check.Resource, check.Action)
	}
	return results
}

// CheckAll 检查是否拥有所有权限
func (m *Manager) CheckAll(userID string, checks []struct{ Resource, Action string }) bool {
	for _, check := range checks {
		if !m.CheckPermissionFast(userID, check.Resource, check.Action) {
			return false
		}
	}
	return true
}

// CheckAny 检查是否拥有任意权限
func (m *Manager) CheckAny(userID string, checks []struct{ Resource, Action string }) bool {
	for _, check := range checks {
		if m.CheckPermissionFast(userID, check.Resource, check.Action) {
			return true
		}
	}
	return false
}
