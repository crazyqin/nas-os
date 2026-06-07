// Package smbmultichannel 提供 REST API 处理器
package smbmultichannel

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers SMB Multichannel API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	smb := r.Group("/smb/multichannel")
	{
		smb.GET("/config", h.GetConfig)
		smb.PUT("/config", h.UpdateConfig)
		smb.GET("/channels", h.ListChannels)
		smb.POST("/channels/:name/enable", h.EnableChannel)
		smb.POST("/channels/:name/disable", h.DisableChannel)
		smb.GET("/sessions", h.ListSessions)
		smb.GET("/sessions/:id", h.GetSession)
		smb.GET("/sessions/:id/stats", h.GetSessionStats)
		smb.GET("/stats", h.GetThroughputStats)
		smb.GET("/bandwidth/history", h.GetBandwidthHistory)

		// 增强功能路由
		smb.GET("/channel/stats", h.GetChannelStats)
		smb.GET("/channel/health", h.GetChannelHealth)
		smb.GET("/audit", h.ListAuditEntries)
		smb.POST("/enable", h.EnableMultichannel)
		smb.POST("/disable", h.DisableMultichannel)
		smb.POST("/load-balance-mode", h.SetLoadBalanceMode)
	}
}

// ========== 配置接口 ==========

// GetConfig 获取 Multichannel 配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// UpdateConfig 更新 Multichannel 配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	cfg, err := h.manager.UpdateConfig(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: cfg})
}

// ========== 通道接口 ==========

// ListChannels 列出所有通道.
func (h *Handlers) ListChannels(c *gin.Context) {
	channels := h.manager.DetectChannels()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(channels),
			"channels": channels,
		},
	})
}

// EnableChannel 启用通道.
func (h *Handlers) EnableChannel(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.EnableChannel(name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "enabled", Data: status})
}

// DisableChannel 禁用通道.
func (h *Handlers) DisableChannel(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.DisableChannel(name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "disabled", Data: status})
}

// ========== 会话接口 ==========

// ListSessions 列出所有会话.
func (h *Handlers) ListSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(sessions),
			"sessions": sessions,
		},
	})
}

// GetSession 获取会话详情.
func (h *Handlers) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: session})
}

// GetSessionStats 获取会话统计.
func (h *Handlers) GetSessionStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetSessionStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 统计接口 ==========

// GetThroughputStats 获取吞吐量统计.
func (h *Handlers) GetThroughputStats(c *gin.Context) {
	stats := h.manager.GetThroughputStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// GetBandwidthHistory 获取带宽历史.
func (h *Handlers) GetBandwidthHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "60")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 60
	}

	history := h.manager.GetBandwidthHistory(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// ========== 增强功能接口 ==========

// GetChannelStats 获取通道统计信息.
func (h *Handlers) GetChannelStats(c *gin.Context) {
	stats := h.manager.GetChannelStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// GetChannelHealth 获取通道健康状态.
func (h *Handlers) GetChannelHealth(c *gin.Context) {
	health := h.manager.GetChannelHealth()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(health),
			"health": health,
		},
	})
}

// ListAuditEntries 获取审计日志.
func (h *Handlers) ListAuditEntries(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	entries := h.manager.ListAuditEntries(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: AuditLogResponse{
			Total:   len(entries),
			Entries: entries,
		},
	})
}

// EnableMultichannel 启用 Multichannel.
func (h *Handlers) EnableMultichannel(c *gin.Context) {
	clientIP := c.ClientIP()
	result := h.manager.EnableMultichannel(clientIP)
	c.JSON(http.StatusOK, response{Code: 0, Message: result.Message, Data: result})
}

// DisableMultichannel 禁用 Multichannel.
func (h *Handlers) DisableMultichannel(c *gin.Context) {
	clientIP := c.ClientIP()
	result := h.manager.DisableMultichannel(clientIP)
	c.JSON(http.StatusOK, response{Code: 0, Message: result.Message, Data: result})
}

// SetLoadBalanceMode 设置负载均衡模式.
func (h *Handlers) SetLoadBalanceMode(c *gin.Context) {
	var req SetLoadBalanceModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	clientIP := c.ClientIP()
	err := h.manager.SetLoadBalanceMode(req.Mode, clientIP)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "load balance mode updated",
		Data:    gin.H{"mode": req.Mode},
	})
}
