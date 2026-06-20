// Package smbmultichannel 提供 REST API 处理器
package smbmultichannel

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers SMB Multichannel API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	smb := r.Group("/smb/multichannel")
	{
		// 配置
		smb.GET("/config", h.GetConfig)
		smb.PUT("/config", h.UpdateConfig)

		// 通道管理
		smb.GET("/channels", h.ListChannels)
		smb.POST("/channels/:name/enable", h.EnableChannel)
		smb.POST("/channels/:name/disable", h.DisableChannel)
		smb.GET("/channel/stats", h.GetChannelStats)
		smb.GET("/channel/health", h.GetChannelHealth)

		// 会话管理
		smb.GET("/sessions", h.ListSessions)
		smb.POST("/sessions", h.CreateSession)
		smb.GET("/sessions/:id", h.GetSession)
		smb.DELETE("/sessions/:id", h.CloseSession)
		smb.GET("/sessions/:id/stats", h.GetSessionStats)

		// 故障转移
		smb.POST("/sessions/:id/channels/:channel/failover", h.HandleChannelFailure)
		smb.POST("/rebalance", h.RebalanceChannels)

		// 统计与监控
		smb.GET("/stats", h.GetThroughputStats)
		smb.GET("/bandwidth/history", h.GetBandwidthHistory)
		smb.GET("/manager/stats", h.GetManagerStats)

		// 全局控制
		smb.POST("/enable", h.EnableMultichannel)
		smb.POST("/disable", h.DisableMultichannel)
		smb.POST("/load-balance-mode", h.SetLoadBalanceMode)

		// 审计
		smb.GET("/audit", h.ListAuditEntries)
	}
}

// ========== 配置接口 ==========

// GetConfig 获取 Multichannel 配置
func (h *Handlers) GetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// UpdateConfig 更新 Multichannel 配置
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

// ListChannels 列出所有通道
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

// EnableChannel 启用通道
func (h *Handlers) EnableChannel(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.EnableChannel(name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "enabled", Data: status})
}

// DisableChannel 禁用通道
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

// CreateSession 创建多通道会话
func (h *Handlers) CreateSession(c *gin.Context) {
	var req struct {
		ClientIP string `json:"client_ip" binding:"required"`
		ServerIP string `json:"server_ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	session, err := h.manager.CreateSession(req.ClientIP, req.ServerIP)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "session created", Data: session})
}

// CloseSession 关闭会话
func (h *Handlers) CloseSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CloseSession(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "session closed"})
}

// ListSessions 列出所有会话
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

// GetSession 获取会话详情
func (h *Handlers) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: session})
}

// GetSessionStats 获取会话统计
func (h *Handlers) GetSessionStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetSessionStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 故障转移接口 ==========

// HandleChannelFailure 处理通道故障
func (h *Handlers) HandleChannelFailure(c *gin.Context) {
	sessionID := c.Param("id")
	channelID := c.Param("channel")

	if err := h.manager.HandleChannelFailure(sessionID, channelID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "failover handled"})
}

// RebalanceChannels 重平衡通道
func (h *Handlers) RebalanceChannels(c *gin.Context) {
	result, err := h.manager.RebalanceChannels()
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rebalanced", Data: result})
}

// ========== 统计接口 ==========

// GetThroughputStats 获取吞吐量统计
func (h *Handlers) GetThroughputStats(c *gin.Context) {
	stats := h.manager.GetThroughputStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// GetBandwidthHistory 获取带宽历史
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

// GetChannelStats 获取通道统计信息
func (h *Handlers) GetChannelStats(c *gin.Context) {
	stats := h.manager.GetChannelStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// GetChannelHealth 获取通道健康状态
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

// GetManagerStats 获取管理器全局统计
func (h *Handlers) GetManagerStats(c *gin.Context) {
	stats := h.manager.GetManagerStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 全局控制接口 ==========

// EnableMultichannel 启用 Multichannel
func (h *Handlers) EnableMultichannel(c *gin.Context) {
	clientIP := c.ClientIP()
	result := h.manager.EnableMultichannel(clientIP)
	c.JSON(http.StatusOK, response{Code: 0, Message: result.Message, Data: result})
}

// DisableMultichannel 禁用 Multichannel
func (h *Handlers) DisableMultichannel(c *gin.Context) {
	clientIP := c.ClientIP()
	result := h.manager.DisableMultichannel(clientIP)
	c.JSON(http.StatusOK, response{Code: 0, Message: result.Message, Data: result})
}

// SetLoadBalanceMode 设置负载均衡模式
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

// ========== 审计接口 ==========

// ListAuditEntries 获取审计日志
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
