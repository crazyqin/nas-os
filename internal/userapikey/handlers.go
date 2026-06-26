package userapikey

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers API Key 管理 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	keys := r.Group("/apikeys")
	{
		keys.POST("", h.createKey)
		keys.GET("", h.listKeys)
		keys.GET("/:id", h.getKey)
		keys.DELETE("/:id", h.revokeKey)
		keys.POST("/:id/rotate", h.rotateKey)
		keys.POST("/:id/validate", h.validateKey)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createKey 创建 API Key
// POST /apikeys
func (h *Handlers) createKey(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, response{Code: 401, Message: "unauthorized"})
		return
	}

	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	result, err := h.manager.CreateKey(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 201, Message: "created", Data: result})
}

// listKeys 列出用户的 API Key
// GET /apikeys
func (h *Handlers) listKeys(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, response{Code: 401, Message: "unauthorized"})
		return
	}

	var opts ListKeysOptions
	// TODO: 解析查询参数（status、offset、limit）

	keys := h.manager.ListKeys(userID, &opts)
	c.JSON(http.StatusOK, response{Code: 200, Message: "ok", Data: keys})
}

// getKey 获取单个 API Key 详情
// GET /apikeys/:id
func (h *Handlers) getKey(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	key, err := h.manager.GetKey(userID, keyID)
	if err != nil {
		status := http.StatusNotFound
		if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "ok", Data: key})
}

// revokeKey 撤销 API Key
// DELETE /apikeys/:id
func (h *Handlers) revokeKey(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	if err := h.manager.RevokeKey(userID, keyID); err != nil {
		status := http.StatusBadRequest
		if err == ErrKeyNotFound {
			status = http.StatusNotFound
		} else if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "revoked"})
}

// rotateKey 轮换 API Key
// POST /apikeys/:id/rotate
func (h *Handlers) rotateKey(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	result, err := h.manager.RotateKey(userID, keyID)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrKeyNotFound {
			status = http.StatusNotFound
		} else if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "rotated", Data: result})
}

// validateKey 验证 API Key
// POST /apikeys/:id/validate
func (h *Handlers) validateKey(c *gin.Context) {
	var body struct {
		Key string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	key, err := h.manager.ValidateKey(body.Key)
	if err != nil {
		status := http.StatusUnauthorized
		if err == ErrKeyNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "valid", Data: key})
}
