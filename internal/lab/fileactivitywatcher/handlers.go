// Package fileactivitywatcher HTTP API 处理器
package fileactivitywatcher

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/file-activity")
	{
		group.GET("/events", h.GetEvents)
		group.GET("/stats", h.GetStats)
		group.GET("/rules", h.ListRules)
		group.POST("/rules", h.CreateRule)
		group.PUT("/rules/:id", h.UpdateRule)
		group.DELETE("/rules/:id", h.DeleteRule)
		group.POST("/watch", h.AddWatch)
		group.DELETE("/watch/:id", h.RemoveWatch)
		group.GET("/watch", h.ListWatch)
	}
}

// GetEvents 查询活动事件.
func (h *Handler) GetEvents(c *gin.Context) {
	eventType := c.Query("type")
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	events := h.manager.GetEvents(eventType, limit)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": events})
}

// GetStats 获取统计信息.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// ListRules 获取告警规则.
func (h *Handler) ListRules(c *gin.Context) {
	rules := h.manager.ListAlertRules()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

// CreateRule 添加告警规则.
func (h *Handler) CreateRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if rule.ID == "" {
		rule.ID = "rule_" + time.Now().Format("20060102150405")
	}
	if err := h.manager.AddAlertRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": rule})
}

// UpdateRule 更新告警规则.
func (h *Handler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	rule.ID = id
	if err := h.manager.UpdateAlertRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

// DeleteRule 删除告警规则.
func (h *Handler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeleteAlertRule(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AddWatch 添加监控目录.
func (h *Handler) AddWatch(c *gin.Context) {
	var dir WatchDir
	if err := c.ShouldBindJSON(&dir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if dir.ID == "" {
		dir.ID = "wd_" + time.Now().Format("20060102150405")
	}
	if err := h.manager.AddWatchDir(&dir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": dir})
}

// RemoveWatch 移除监控目录.
func (h *Handler) RemoveWatch(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.RemoveWatchDir(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "监控目录不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListWatch 列出监控目录.
func (h *Handler) ListWatch(c *gin.Context) {
	dirs := h.manager.ListWatchDirs()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": dirs})
}
