package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 通用响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// Handlers 用户管理 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	users := api.Group("/users")
	{
		users.GET("", h.listUsers)
		users.POST("", h.createUser)
		users.GET("/:username", h.getUser)
		users.PUT("/:username", h.updateUser)
		users.DELETE("/:username", h.deleteUser)
		users.POST("/:username/disable", h.disableUser)
		users.POST("/:username/enable", h.enableUser)
		users.POST("/:username/password", h.changePassword)
	}

	// 认证相关
	api.POST("/login", h.login)
	api.POST("/logout", h.logout)
	api.GET("/me", h.getCurrentUser)
}

// API 请求/响应结构

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// 处理器实现

func (h *Handlers) listUsers(c *gin.Context) {
	users := h.manager.ListUsers()
	c.JSON(http.StatusOK, Success(users))
}

func (h *Handlers) createUser(c *gin.Context) {
	var req UserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	user, err := h.manager.CreateUser(req)
	if err != nil {
		if err == ErrUserExists {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(user))
}

func (h *Handlers) getUser(c *gin.Context) {
	username := c.Param("username")
	user, err := h.manager.GetUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(user))
}

func (h *Handlers) updateUser(c *gin.Context) {
	username := c.Param("username")
	var req UserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	user, err := h.manager.UpdateUser(username, req)
	if err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(user))
}

func (h *Handlers) deleteUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DeleteUser(username); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		} else if err == ErrAdminCannotDelete {
			c.JSON(http.StatusForbidden, Error(403, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) disableUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DisableUser(username, true); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) enableUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.manager.DisableUser(username, false); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) changePassword(c *gin.Context) {
	username := c.Param("username")
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.ChangePassword(username, req.OldPassword, req.NewPassword); err != nil {
		if err == ErrUserNotFound || err == ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, Error(401, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	token, err := h.manager.Authenticate(req.Username, req.Password)
	if err != nil {
		if err == ErrUserNotFound || err == ErrInvalidPassword {
			c.JSON(http.StatusUnauthorized, Error(401, "用户名或密码错误"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	user, _ := h.manager.GetUser(req.Username)
	c.JSON(http.StatusOK, Success(LoginResponse{
		Token: token.Token,
		User:  user,
	}))
}

func (h *Handlers) logout(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr != "" {
		h.manager.Logout(tokenStr)
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getCurrentUser(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	user, err := h.manager.ValidateToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Error(401, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(user))
}

// AuthMiddleware 认证中间件
func AuthMiddleware(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			// 尝试从查询参数获取
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, Error(401, "需要认证"))
			c.Abort()
			return
		}

		user, err := mgr.ValidateToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, Error(401, err.Error()))
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("user", user)
		c.Next()
	}
}

// RequireRole 角色要求中间件
func RequireRole(mgr *Manager, roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
			c.Abort()
			return
		}

		u, ok := user.(*User)
		if !ok {
			c.JSON(http.StatusInternalServerError, Error(500, "内部错误"))
			c.Abort()
			return
		}

		for _, role := range roles {
			if u.Role == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, Error(403, "权限不足"))
		c.Abort()
	}
}
