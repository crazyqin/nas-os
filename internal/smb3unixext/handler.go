// Package smb3unixext 提供 SMB3 Unix 扩展支持。
// HTTP handler 层：提供 REST API 用于启用/禁用扩展、查看支持状态、协商客户端能力。
package smb3unixext

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler SMB3 Unix 扩展 HTTP 处理器.
type Handler struct {
	service *Service
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
// 路由组路径建议: /api/v1/smb3-unix-ext.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/smb3-unix-ext")
	{
		// 获取全局支持状态
		group.GET("/support-status", h.getSupportStatus)
		// 列出所有扩展配置
		group.GET("/extensions", h.listExtensions)
		// 获取指定共享的扩展状态
		group.GET("/extensions/:share_name", h.getExtension)
		// 设置（启用/禁用）指定共享的扩展
		group.POST("/extensions", h.setExtension)
		// 移除指定共享的扩展配置
		group.DELETE("/extensions/:share_name", h.removeExtension)
		// 客户端能力协商
		group.POST("/negotiate", h.negotiateCapabilities)
		// 批量启用所有共享的扩展
		group.POST("/enable-all", h.enableAll)
		// 批量禁用所有共享的扩展
		group.POST("/disable-all", h.disableAll)
	}
}

// response 标准响应.
type response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// getSupportStatus 获取全局支持状态
// GET /api/v1/smb3-unix-ext/support-status.
func (h *Handler) getSupportStatus(c *gin.Context) {
	status := h.service.GetSupportStatus()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// listExtensions 列出所有扩展配置
// GET /api/v1/smb3-unix-ext/extensions.
func (h *Handler) listExtensions(c *gin.Context) {
	configs := h.service.ListExtensions()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]any{
			"configs": configs,
			"count":   len(configs),
		},
	})
}

// getExtension 获取指定共享的扩展状态
// GET /api/v1/smb3-unix-ext/extensions/:share_name.
func (h *Handler) getExtension(c *gin.Context) {
	shareName := c.Param("share_name")
	status, err := h.service.GetExtensionStatus(shareName)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// setExtension 设置（启用/禁用）指定共享的扩展
// POST /api/v1/smb3-unix-ext/extensions.
func (h *Handler) setExtension(c *gin.Context) {
	var req SetExtensionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	cfg, err := h.service.SetExtension(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "扩展配置已更新",
		Data:    cfg,
	})
}

// removeExtension 移除指定共享的扩展配置
// DELETE /api/v1/smb3-unix-ext/extensions/:share_name.
func (h *Handler) removeExtension(c *gin.Context) {
	shareName := c.Param("share_name")
	if err := h.service.RemoveExtension(shareName); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "扩展配置已移除",
	})
}

// negotiateCapabilities 客户端能力协商
// POST /api/v1/smb3-unix-ext/negotiate.
func (h *Handler) negotiateCapabilities(c *gin.Context) {
	var req ClientCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	result, err := h.service.NegotiateClientCapabilities(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "客户端能力协商完成",
		Data:    result,
	})
}

// enableAll 批量启用所有共享的扩展
// POST /api/v1/smb3-unix-ext/enable-all.
func (h *Handler) enableAll(c *gin.Context) {
	count := h.service.EnableAll()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "批量启用完成",
		Data: map[string]any{
			"enabled_count": count,
		},
	})
}

// disableAll 批量禁用所有共享的扩展
// POST /api/v1/smb3-unix-ext/disable-all.
func (h *Handler) disableAll(c *gin.Context) {
	count := h.service.DisableAll()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "批量禁用完成",
		Data: map[string]any{
			"disabled_count": count,
		},
	})
}
