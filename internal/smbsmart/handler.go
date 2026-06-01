// Package smbsmart SMB 多通道优化模块 - HTTP API 处理器
package smbsmart

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler SMB 多通道 HTTP 处理器
type Handler struct {
	mgr *Manager
}

// NewHandler 创建 SMB 多通道处理器
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册 SMB 多通道路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// 通道管理
	r.GET("/channels", h.ListChannels)
	r.GET("/channels/:id", h.GetChannel)
	r.POST("/channels/discover", h.DiscoverChannels)
	r.POST("/channels/:id/enable", h.EnableChannel)
	r.POST("/channels/:id/disable", h.DisableChannel)

	// 会话管理
	r.GET("/sessions", h.ListSessions)
	r.GET("/sessions/:id", h.GetSession)
	r.POST("/sessions/refresh", h.RefreshSessions)

	// 通道绑定
	r.POST("/bonds", h.BondChannels)
	r.GET("/bonds", h.ListBonds)
	r.GET("/bonds/:id", h.GetBond)
	r.DELETE("/bonds/:id", h.UnbondChannels)

	// 带宽监控
	r.GET("/bandwidth", h.GetBandwidth)
	r.GET("/bandwidth/history", h.GetBandwidthHistory)

	// 故障转移配置
	r.GET("/failover", h.GetFailoverConfig)
	r.PUT("/failover", h.ConfigureFailover)
	r.POST("/failover/check", h.RunHealthCheck)

	// 统计
	r.GET("/stats", h.GetStats)
}

// ========== 通道管理 ==========

// ListChannels 列出所有通道
// @Summary 列出 SMB 通道
// @Description 获取所有 SMB 多通道信息
// @Tags smbsmart
// @Produce json
// @Success 200 {object} APIResponse{data=[]SMBChannel}
// @Router /smb/channels [get].
func (h *Handler) ListChannels(c *gin.Context) {
	channels := h.mgr.ListChannels()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: channels})
}

// GetChannel 获取通道详情
// @Summary 获取 SMB 通道
// @Description 根据 ID 获取通道详情
// @Tags smbsmart
// @Produce json
// @Param id path string true "通道 ID"
// @Success 200 {object} APIResponse{data=SMBChannel}
// @Failure 404 {object} APIResponse
// @Router /smb/channels/{id} [get].
func (h *Handler) GetChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := h.mgr.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: ch})
}

// DiscoverChannels 发现 SMB 通道
func (h *Handler) DiscoverChannels(c *gin.Context) {
	channels, err := h.mgr.DiscoverChannels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "发现通道完成", Data: channels})
}

// EnableChannel 启用通道
func (h *Handler) EnableChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.EnableChannel(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "通道已启用"})
}

// DisableChannel 禁用通道
func (h *Handler) DisableChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DisableChannel(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "通道已禁用"})
}

// ========== 会话管理 ==========

// ListSessions 列出所有会话
// @Summary 列出 SMB 会话
// @Description 获取所有 SMB 会话
// @Tags smbsmart
// @Produce json
// @Success 200 {object} APIResponse{data=[]SMBSession}
// @Router /smb/sessions [get].
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.mgr.ListSessions()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: sessions})
}

// GetSession 获取会话详情
// @Summary 获取 SMB 会话
// @Description 根据 ID 获取会话详情
// @Tags smbsmart
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} APIResponse{data=SMBSession}
// @Failure 404 {object} APIResponse
// @Router /smb/sessions/{id} [get].
func (h *Handler) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.mgr.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: session})
}

// RefreshSessions 刷新会话信息
func (h *Handler) RefreshSessions(c *gin.Context) {
	if err := h.mgr.RefreshSessions(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "会话已刷新"})
}

// ========== 通道绑定 ==========

// BondChannels 绑定通道
func (h *Handler) BondChannels(c *gin.Context) {
	var req BondChannelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	bond, err := h.mgr.BondChannels(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: "通道绑定成功", Data: bond})
}

// ListBonds 列出所有绑定
func (h *Handler) ListBonds(c *gin.Context) {
	bonds := h.mgr.ListBonds()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: bonds})
}

// GetBond 获取绑定详情
func (h *Handler) GetBond(c *gin.Context) {
	id := c.Param("id")
	bond, err := h.mgr.GetBond(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: bond})
}

// UnbondChannels 解除绑定
func (h *Handler) UnbondChannels(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.UnbondChannels(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "绑定已解除"})
}

// ========== 带宽监控 ==========

// GetBandwidth 获取带宽统计
// @Summary 获取带宽统计
// @Description 获取当前 SMB 多通道带宽统计
// @Tags smbsmart
// @Produce json
// @Success 200 {object} APIResponse{data=BandwidthStats}
// @Router /smb/bandwidth [get].
func (h *Handler) GetBandwidth(c *gin.Context) {
	stats, err := h.mgr.GetBandwidth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: stats})
}

// GetBandwidthHistory 获取带宽历史
func (h *Handler) GetBandwidthHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}
	history := h.mgr.GetBandwidthHistory(limit)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: history})
}

// ========== 故障转移 ==========

// GetFailoverConfig 获取故障转移配置
// @Summary 获取故障转移配置
// @Description 获取 SMB 多通道故障转移配置
// @Tags smbsmart
// @Produce json
// @Success 200 {object} APIResponse{data=FailoverConfig}
// @Router /smb/failover [get].
func (h *Handler) GetFailoverConfig(c *gin.Context) {
	config := h.mgr.GetFailoverConfig()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: config})
}

// ConfigureFailover 配置故障转移
// @Summary 配置故障转移
// @Description 更新 SMB 多通道故障转移配置
// @Tags smbsmart
// @Accept json
// @Produce json
// @Param request body UpdateFailoverConfigRequest true "配置信息"
// @Success 200 {object} APIResponse{data=FailoverConfig}
// @Failure 400 {object} APIResponse
// @Router /smb/failover [put].
func (h *Handler) ConfigureFailover(c *gin.Context) {
	var req UpdateFailoverConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "请求参数错误: " + err.Error()})
		return
	}
	config, err := h.mgr.ConfigureFailover(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "故障转移配置已更新", Data: config})
}

// RunHealthCheck 执行健康检查
func (h *Handler) RunHealthCheck(c *gin.Context) {
	failed, recovered := h.mgr.RunHealthCheck(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "健康检查完成",
		Data: gin.H{
			"failed_channels":    failed,
			"recovered_channels": recovered,
		},
	})
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.mgr.GetChannelStats()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: stats})
}

// ========== 辅助类型 ==========

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
