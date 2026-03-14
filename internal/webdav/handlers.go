package webdav

import (
	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// Handlers WebDAV HTTP 处理器
type Handlers struct {
	server *Server
}

// NewHandlers 创建处理器
func NewHandlers(srv *Server) *Handlers {
	return &Handlers{server: srv}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	h.server.RegisterRoutes(apiGroup)
}

// GetConfig 获取 WebDAV 配置（外部调用）
func (h *Handlers) GetConfig(c *gin.Context) {
	config := h.server.GetConfig()
	api.OK(c, config)
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled       *bool   `json:"enabled"`
	Port          *int    `json:"port"`
	RootPath      *string `json:"root_path"`
	AllowGuest    *bool   `json:"allow_guest"`
	MaxUploadSize *int64  `json:"max_upload_size"`
}

// UpdateConfig 更新 WebDAV 配置（外部调用）
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	config := h.server.GetConfig()

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.Port != nil {
		config.Port = *req.Port
	}
	if req.RootPath != nil {
		config.RootPath = *req.RootPath
	}
	if req.AllowGuest != nil {
		config.AllowGuest = *req.AllowGuest
	}
	if req.MaxUploadSize != nil {
		config.MaxUploadSize = *req.MaxUploadSize
	}

	if err := h.server.UpdateConfig(config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, config)
}

// GetStatus 获取 WebDAV 服务器状态
func (h *Handlers) GetStatus(c *gin.Context) {
	status := h.server.GetStatus()
	api.OK(c, status)
}

// GetLocks 获取所有锁
func (h *Handlers) GetLocks(c *gin.Context) {
	h.server.lockManager.mu.RLock()
	locks := make([]*Lock, 0, len(h.server.lockManager.locks))
	for _, lock := range h.server.lockManager.locks {
		locks = append(locks, lock)
	}
	h.server.lockManager.mu.RUnlock()

	api.OK(c, locks)
}

// DeleteLock 删除锁
func (h *Handlers) DeleteLock(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		api.BadRequest(c, "缺少锁令牌")
		return
	}

	if err := h.server.lockManager.RemoveLock(token); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "锁已删除", nil)
}
