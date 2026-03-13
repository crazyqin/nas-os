package webdav

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	h.server.RegisterRoutes(api)
}

// WebDAVConfigResponse WebDAV 配置响应
type WebDAVConfigResponse struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    *Config `json:"data"`
}

// GetConfig 获取 WebDAV 配置（外部调用）
func (h *Handlers) GetConfig(c *gin.Context) {
	config := h.server.GetConfig()
	c.JSON(http.StatusOK, WebDAVConfigResponse{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled    *bool   `json:"enabled"`
	Port       *int    `json:"port"`
	RootPath   *string `json:"root_path"`
	AllowGuest *bool   `json:"allow_guest"`
}

// UpdateConfig 更新 WebDAV 配置（外部调用）
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
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

	if err := h.server.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, WebDAVConfigResponse{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}
