package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apiresponse "nas-os/internal/api"
	"nas-os/internal/auth"
)

// Handlers 用户管理 HTTP 处理器
type Handlers struct {
	manager    *Manager
	mfaManager *auth.MFAManager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager, mfaMgr *auth.MFAManager) *Handlers {
	return &Handlers{
		manager:    mgr,
		mfaManager: mfaMgr,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(router *gin.RouterGroup) {
	// ========== 认证相关（公开路由）==========
	router.POST("/login", h.login)
	router.POST("/logout", h.logout)
	router.POST("/refresh", h.refreshToken)
	router.GET("/me", h.getCurrentUser)

	// ========== 用户管理（需要认证和管理员权限）==========
	users := router.Group("/users")
	// 注意：调用方应在应用此路由组前添加认证和权限中间件
	// 示例：router.Group("/users", authMiddleware, adminMiddleware)
	{
		users.GET("", h.listUsers)
		users.POST("", h.createUser) // 需要 admin 权限
		users.GET("/:username", h.getUser)
		users.PUT("/:username", h.updateUser)           // 需要 admin 权限或本人
		users.DELETE("/:username", h.deleteUser)        // 需要 admin 权限
		users.POST("/:username/disable", h.disableUser) // 需要 admin 权限
		users.POST("/:username/enable", h.enableUser)   // 需要 admin 权限
		users.POST("/:username/password", h.changePassword)
		users.POST("/:username/reset-password", h.resetPassword)        // 需要 admin 权限
		users.PUT("/:username/role", h.setUserRole)                     // 需要 admin 权限
		users.POST("/:username/groups/:group", h.addUserToGroup)        // 需要 admin 权限
		users.DELETE("/:username/groups/:group", h.removeUserFromGroup) // 需要 admin 权限
	}

	// ========== 用户组管理（需要认证和管理员权限）==========
	groups := router.Group("/groups")
	// 注意：调用方应在应用此路由组前添加认证和权限中间件
	{
		groups.GET("", h.listGroups)
		groups.POST("", h.createGroup) // 需要 admin 权限
		groups.GET("/:name", h.getGroup)
		groups.PUT("/:name", h.updateGroup)    // 需要 admin 权限
		groups.DELETE("/:name", h.deleteGroup) // 需要 admin 权限
		groups.GET("/:name/users", h.getGroupUsers)
	}
}

// API 请求/响应结构

type LoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	MFACode    string `json:"mfa_code,omitempty"`    // TOTP 或短信验证码
	BackupCode string `json:"backup_code,omitempty"` // 备份码
}

type LoginResponse struct {
	Token       string `json:"token,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	MFARequired bool   `json:"mfa_required"`
	MFAType     string `json:"mfa_type,omitempty"`   // totp, sms, webauthn
	SessionID   string `json:"session_id,omitempty"` // 临时会话 ID
	User        *User  `json:"user,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type SetRoleRequest struct {
	Role Role `json:"role" binding:"required"`
}

// ========== 用户管理 API ==========

func (h *Handlers) listUsers(c *gin.Context) {
	// 支持按角色筛选
	role := c.Query("role")
	if role != "" {
		users := h.manager.GetUsersByRole(Role(role))
		c.JSON(http.StatusOK, apiresponse.Success(users))
		return
	}

	users := h.manager.ListUsers()
	c.JSON(http.StatusOK, apiresponse.Success(users))
}

func (h *Handlers) createUser(c *gin.Context) {
	var req UserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	user, err := h.manager.CreateUser(req)
	if err != nil {
		if err == ErrUserExists {
			c.JSON(http.StatusConflict, apiresponse.Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, apiresponse.Success(user))
}

func (h *Handlers) getUser(c *gin.Context) {
	username := c.Param("username")
	user, err := h.manager.GetUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(user))
}

func (h *Handlers) updateUser(c *gin.Context) {
	username := c.Param("username")
	var req UserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	user, err := h.manager.UpdateUser(username, req)
	if err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(user))
}

func (h *Handlers) deleteUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DeleteUser(username); err != nil {
		switch err {
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		case ErrAdminCannotDelete, ErrLastAdmin:
			c.JSON(http.StatusForbidden, apiresponse.Error(403, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) disableUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DisableUser(username, true); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) enableUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DisableUser(username, false); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) changePassword(c *gin.Context) {
	username := c.Param("username")
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	if err := h.manager.ChangePassword(username, req.OldPassword, req.NewPassword); err != nil {
		if err == ErrUserNotFound || err == ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) resetPassword(c *gin.Context) {
	username := c.Param("username")
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	if err := h.manager.ResetPassword(username, req.NewPassword); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) setUserRole(c *gin.Context) {
	username := c.Param("username")
	var req SetRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	if err := h.manager.SetUserRole(username, req.Role); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		} else if err == ErrLastAdmin {
			c.JSON(http.StatusForbidden, apiresponse.Error(403, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) addUserToGroup(c *gin.Context) {
	username := c.Param("username")
	groupName := c.Param("group")

	if err := h.manager.AddUserToGroup(username, groupName); err != nil {
		switch err {
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "用户不存在"))
		case ErrGroupNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "用户组不存在"))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) removeUserFromGroup(c *gin.Context) {
	username := c.Param("username")
	groupName := c.Param("group")

	if err := h.manager.RemoveUserFromGroup(username, groupName); err != nil {
		switch err {
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "用户不存在"))
		case ErrGroupNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "用户组不存在"))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// ========== 用户组管理 API ==========

func (h *Handlers) listGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, apiresponse.Success(groups))
}

func (h *Handlers) createGroup(c *gin.Context) {
	var req GroupInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	group, err := h.manager.CreateGroup(req)
	if err != nil {
		if err == ErrGroupExists {
			c.JSON(http.StatusConflict, apiresponse.Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, apiresponse.Success(group))
}

func (h *Handlers) getGroup(c *gin.Context) {
	name := c.Param("name")
	group, err := h.manager.GetGroup(name)
	if err != nil {
		c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(group))
}

func (h *Handlers) updateGroup(c *gin.Context) {
	name := c.Param("name")
	var req GroupInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	group, err := h.manager.UpdateGroup(name, req)
	if err != nil {
		if err == ErrGroupNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(group))
}

func (h *Handlers) deleteGroup(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteGroup(name); err != nil {
		if err == ErrGroupNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) getGroupUsers(c *gin.Context) {
	name := c.Param("name")
	users, err := h.manager.GetUsersInGroup(name)
	if err != nil {
		c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(users))
}

// ========== 认证 API ==========

func (h *Handlers) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	// 首先验证用户名和密码
	token, err := h.manager.Authenticate(req.Username, req.Password)
	if err != nil {
		if err == ErrUserNotFound || err == ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "用户名或密码错误"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, err.Error()))
		return
	}

	user, _ := h.manager.GetUser(req.Username)

	// 检查是否需要 MFA
	if h.mfaManager != nil && h.mfaManager.RequireMFA(user.ID) {
		// 需要 MFA 验证
		mfaType := h.mfaManager.GetMFAType(user.ID)

		// 如果提供了 MFA 验证码，尝试验证
		if req.MFACode != "" || req.BackupCode != "" {
			verifyCode := req.MFACode
			if req.BackupCode != "" {
				verifyCode = req.BackupCode
			}

			if err := h.mfaManager.VerifyMFA(user.ID, mfaType, verifyCode, nil); err != nil {
				c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "MFA 验证码无效"))
				return
			}

			// MFA 验证成功，返回令牌
			c.JSON(http.StatusOK, apiresponse.Success(LoginResponse{
				Token:       token.Token,
				ExpiresAt:   token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
				MFARequired: false,
				User:        user,
			}))
			return
		}

		// 需要 MFA，但用户还未提供验证码
		c.JSON(http.StatusOK, apiresponse.Success(LoginResponse{
			MFARequired: true,
			MFAType:     mfaType,
			User:        &User{ID: user.ID, Username: user.Username, Email: user.Email, Role: user.Role},
		}))
		return
	}

	// 不需要 MFA，直接返回令牌
	c.JSON(http.StatusOK, apiresponse.Success(LoginResponse{
		Token:       token.Token,
		ExpiresAt:   token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		MFARequired: false,
		User:        user,
	}))
}

func (h *Handlers) logout(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr != "" {
		h.manager.Logout(tokenStr)
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) refreshToken(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "需要认证"))
		return
	}

	token, err := h.manager.RefreshToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apiresponse.Error(401, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(gin.H{
		"token":      token.Token,
		"expires_at": token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}))
}

func (h *Handlers) getCurrentUser(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "未授权"))
		return
	}

	user, err := h.manager.ValidateToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apiresponse.Error(401, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(user))
}

// ========== 中间件 ==========

// AuthMiddleware 认证中间件
func AuthMiddleware(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			// 尝试从查询参数获取
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "需要认证"))
			c.Abort()
			return
		}

		user, err := mgr.ValidateToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, err.Error()))
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("user", user)
		c.Set("username", user.Username)
		c.Next()
	}
}

// RequireRole 角色要求中间件
func RequireRole(mgr *Manager, roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "未授权"))
			c.Abort()
			return
		}

		u, ok := user.(*User)
		if !ok {
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "内部错误"))
			c.Abort()
			return
		}

		for _, role := range roles {
			if u.Role == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, apiresponse.Error(403, "权限不足"))
		c.Abort()
	}
}

// RequireAdmin 管理员中间件
func RequireAdmin(mgr *Manager) gin.HandlerFunc {
	return RequireRole(mgr, RoleAdmin)
}

// RequirePermission 权限检查中间件
func RequirePermission(mgr *Manager, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, apiresponse.Error(401, "未授权"))
			c.Abort()
			return
		}

		u, ok := user.(*User)
		if !ok {
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "内部错误"))
			c.Abort()
			return
		}

		if !mgr.HasPermission(u, resource, action) {
			c.JSON(http.StatusForbidden, apiresponse.Error(403, "权限不足"))
			c.Abort()
			return
		}

		c.Next()
	}
}
