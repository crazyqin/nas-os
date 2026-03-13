package trash

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 回收站 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	trash := api.Group("/trash")
	{
		trash.GET("", h.list)
		trash.GET("/stats", h.getStats)
		trash.GET("/config", h.getConfig)
		trash.PUT("/config", h.updateConfig)

		trash.POST("/:id/restore", h.restore)
		trash.DELETE("/:id", h.deletePermanently)

		trash.DELETE("", h.empty)
	}
}

// APIResponse 通用响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// TrashResponse 回收站项目响应
type TrashResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OriginalPath string    `json:"original_path"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	DaysLeft     int       `json:"days_left"`
}

// list 列出回收站项目
func (h *Handlers) list(c *gin.Context) {
	items := h.manager.List()

	response := make([]TrashResponse, len(items))
	for i, item := range items {
		daysLeft := int(time.Until(item.ExpiresAt).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}

		response[i] = TrashResponse{
			ID:           item.ID,
			Name:         item.Name,
			OriginalPath: item.OriginalPath,
			Size:         item.Size,
			IsDir:        item.IsDir,
			DeletedAt:    item.DeletedAt,
			ExpiresAt:    item.ExpiresAt,
			DaysLeft:     daysLeft,
		}
	}

	c.JSON(http.StatusOK, success(response))
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, success(stats))
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, success(config))
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled       *bool  `json:"enabled"`
	RetentionDays *int   `json:"retention_days"`
	MaxSize       *int64 `json:"max_size"`
	AutoEmpty     *bool  `json:"auto_empty"`
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	config := h.manager.GetConfig()

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.RetentionDays != nil {
		config.RetentionDays = *req.RetentionDays
	}
	if req.MaxSize != nil {
		config.MaxSize = *req.MaxSize
	}
	if req.AutoEmpty != nil {
		config.AutoEmpty = *req.AutoEmpty
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, apiError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(config))
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	TargetPath string `json:"target_path,omitempty"`
}

// restore 恢复文件
func (h *Handlers) restore(c *gin.Context) {
	id := c.Param("id")

	var req RestoreRequest
	if c.ShouldBindJSON(&req) == nil && req.TargetPath != "" {
		// TODO: 支持恢复到指定路径
	}

	if err := h.manager.Restore(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// deletePermanently 永久删除
func (h *Handlers) deletePermanently(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePermanently(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// empty 清空回收站
func (h *Handlers) empty(c *gin.Context) {
	if err := h.manager.Empty(); err != nil {
		c.JSON(http.StatusInternalServerError, apiError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// MoveToTrashRequest 移动到回收站请求
type MoveToTrashRequest struct {
	Path   string `json:"path" binding:"required"`
	UserID string `json:"user_id"`
}

// MoveToTrash 移动到回收站（外部调用）
func (h *Handlers) MoveToTrash(c *gin.Context) {
	var req MoveToTrashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	item, err := h.manager.MoveToTrash(req.Path, req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(item))
}
