// Package clipboard 提供REST API处理器
package clipboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 剪贴板API处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	clip := r.Group("/clipboard")
	{
		clip.POST("", h.Create)
		clip.GET("/search", h.Search)
		clip.GET("/latest", h.GetLatest)
		clip.GET("/sync", h.Sync)
		clip.GET("/stats", h.Stats)
		clip.GET("/:id", h.Get)
		clip.DELETE("/:id", h.Delete)
		clip.DELETE("/user/:userId", h.ClearUser)
	}
}

// Create 创建剪贴板条目.
func (h *Handlers) Create(c *gin.Context) {
	var req CreateClipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "default"
	}

	item, err := h.manager.Create(req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 201, Message: "created", Data: item})
}

// Get 获取剪贴板条目.
func (h *Handlers) Get(c *gin.Context) {
	id := c.Param("id")
	item, err := h.manager.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Data: item})
}

// GetLatest 获取最新剪贴板.
func (h *Handlers) GetLatest(c *gin.Context) {
	userID := c.Query("user_id")
	deviceID := c.Query("device_id")
	if userID == "" {
		userID = "default"
	}

	item, err := h.manager.GetLatest(userID, deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Data: item})
}

// Sync 同步剪贴板.
func (h *Handlers) Sync(c *gin.Context) {
	userID := c.Query("user_id")
	deviceID := c.Query("device_id")
	lastSyncStr := c.Query("last_sync")

	if userID == "" {
		userID = "default"
	}

	var lastSync time.Time
	if lastSyncStr != "" {
		ts, err := strconv.ParseInt(lastSyncStr, 10, 64)
		if err == nil {
			lastSync = time.Unix(ts, 0)
		}
	}

	items, err := h.manager.Sync(userID, deviceID, lastSync)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Data: items})
}

// Search 搜索剪贴板.
func (h *Handlers) Search(c *gin.Context) {
	query := c.Query("q")
	clipType := ClipType(c.Query("type"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	userID := c.GetString("user_id")
	if userID == "" {
		userID = c.Query("user_id")
	}

	req := SearchRequest{
		Query:    query,
		Type:     clipType,
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.manager.Search(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 200,
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  page,
		},
	})
}

// Stats 获取统计.
func (h *Handlers) Stats(c *gin.Context) {
	stats := h.manager.Stats()
	c.JSON(http.StatusOK, response{Code: 200, Data: stats})
}

// Delete 删除剪贴板条目.
func (h *Handlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Message: "deleted"})
}

// ClearUser 清空用户剪贴板.
func (h *Handlers) ClearUser(c *gin.Context) {
	userID := c.Param("userId")
	if err := h.manager.ClearUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 500, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Message: "cleared"})
}
