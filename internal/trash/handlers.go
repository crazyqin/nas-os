package trash

import (
	"time"

	"nas-os/internal/api"

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
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	trash := apiGroup.Group("/trash")
	{
		trash.GET("", h.list)
		trash.GET("/stats", h.getStats)
		trash.GET("/config", h.getConfig)
		trash.PUT("/config", h.updateConfig)

		trash.GET("/:id", h.get)
		trash.POST("/:id/restore", h.restore)
		trash.DELETE("/:id", h.deletePermanently)

		trash.DELETE("", h.empty)
	}
}

// Response 回收站项目响应
type Response struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OriginalPath string    `json:"original_path"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	DaysLeft     int       `json:"days_left"`
}

// convertToResponse 将 Item 转换为响应格式
func convertToResponse(item *Item) Response {
	daysLeft := int(time.Until(item.ExpiresAt).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	return Response{
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

// get 获取单个回收站项目详情
func (h *Handlers) get(c *gin.Context) {
	id := c.Param("id")

	item, err := h.manager.Get(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, convertToResponse(item))
}

// list 列出回收站项目
func (h *Handlers) list(c *gin.Context) {
	items := h.manager.List()

	response := make([]Response, len(items))
	for i, item := range items {
		response[i] = convertToResponse(item)
	}

	api.OK(c, response)
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	api.OK(c, config)
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
		api.BadRequest(c, err.Error())
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
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, config)
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
		// 恢复到指定路径
		if err := h.manager.RestoreTo(id, req.TargetPath); err != nil {
			api.BadRequest(c, err.Error())
			return
		}
		api.OK(c, gin.H{"restored_to": req.TargetPath})
		return
	}

	// 恢复到原始路径
	if err := h.manager.Restore(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// deletePermanently 永久删除
func (h *Handlers) deletePermanently(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePermanently(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// empty 清空回收站
func (h *Handlers) empty(c *gin.Context) {
	if err := h.manager.Empty(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, nil)
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
		api.BadRequest(c, err.Error())
		return
	}

	item, err := h.manager.MoveToTrash(req.Path, req.UserID)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, item)
}
