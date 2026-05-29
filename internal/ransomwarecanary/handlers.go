// Package ransomwarecanary 提供 REST API 处理器
package ransomwarecanary

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

// Handlers 金丝雀 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	rc := r.Group("/security/ransomware-canary")
	{
		// 状态
		rc.GET("/status", h.GetStatus)

		// 金丝雀管理
		rc.GET("/canaries", h.ListCanaries)
		rc.POST("/canaries", h.DeployCanary)
		rc.DELETE("/canaries/:id", h.RemoveCanary)
		rc.PUT("/canaries/:id/disable", h.DisableCanary)

		// 检测
		rc.POST("/monitor", h.RunMonitor)

		// 告警
		rc.GET("/alerts", h.GetAlerts)
		rc.POST("/alerts", h.TriggerAlert)
		rc.DELETE("/alerts", h.ClearAlerts)

		// 共享锁定
		rc.POST("/shares/lock", h.LockShare)
		rc.POST("/shares/unlock", h.UnlockShare)
		rc.GET("/shares/locked", h.GetLockedShares)

		// 配置
		rc.GET("/config", h.GetConfig)
		rc.PUT("/config", h.UpdateConfig)
	}
}

// ========== 状态 ==========

// GetStatus 获取系统状态.
func (h *Handlers) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: status})
}

// ========== 金丝雀管理 ==========

// ListCanaries 列出所有金丝雀.
func (h *Handlers) ListCanaries(c *gin.Context) {
	canaries := h.manager.ListCanaries()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(canaries),
			"canaries": canaries,
		},
	})
}

// DeployCanary 部署金丝雀.
func (h *Handlers) DeployCanary(c *gin.Context) {
	var req DeployCanaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	canary, err := h.manager.DeployCanary(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "canary deployed", Data: canary})
}

// RemoveCanary 移除金丝雀.
func (h *Handlers) RemoveCanary(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveCanary(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "canary removed"})
}

// DisableCanary 禁用金丝雀.
func (h *Handlers) DisableCanary(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisableCanary(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "canary disabled"})
}

// ========== 检测 ==========

// RunMonitor 手动运行监控检测.
func (h *Handlers) RunMonitor(c *gin.Context) {
	result, err := h.manager.MonitorCanaries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "monitor completed", Data: result})
}

// ========== 告警 ==========

// GetAlerts 获取告警列表.
func (h *Handlers) GetAlerts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	alerts := h.manager.GetAlerts(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

// TriggerAlert 手动触发告警.
func (h *Handlers) TriggerAlert(c *gin.Context) {
	var req struct {
		CanaryID    string `json:"canary_id" binding:"required"`
		AlertType   string `json:"alert_type" binding:"required"`
		Severity    string `json:"severity" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	alert, err := h.manager.TriggerAlert(req.CanaryID, req.AlertType, req.Severity, req.Description)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "alert triggered", Data: alert})
}

// ClearAlerts 清除所有告警.
func (h *Handlers) ClearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, response{Code: 0, Message: "alerts cleared"})
}

// ========== 共享锁定 ==========

// LockShare 锁定共享.
func (h *Handlers) LockShare(c *gin.Context) {
	var req ShareLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	locked, err := h.manager.AutoLockShare(req.ShareName, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	msg := "share locked"
	if !locked {
		msg = "share already locked"
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: msg, Data: gin.H{"locked": locked}})
}

// UnlockShare 解锁共享.
func (h *Handlers) UnlockShare(c *gin.Context) {
	var req ShareLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.UnlockShare(req.ShareName); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "share unlocked"})
}

// GetLockedShares 获取已锁定共享.
func (h *Handlers) GetLockedShares(c *gin.Context) {
	locked := h.manager.GetLockedShares()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: gin.H{"locked_shares": locked}})
}

// ========== 配置 ==========

// GetConfig 获取配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// UpdateConfig 更新配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var cfg CanaryConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateConfig(cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}
