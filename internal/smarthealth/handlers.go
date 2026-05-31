// Package smarthealth 智能健康巡检 - HTTP API
package smarthealth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/health")
	{
		group.GET("", h.GetHealth)
		group.GET("/trends", h.GetTrends)
		group.GET("/alerts", h.GetAlerts)
		group.POST("/alerts/:id/resolve", h.ResolveAlert)
		group.GET("/config", h.GetConfig)
		group.PUT("/config", h.UpdateConfig)
		group.POST("/check", h.RunManualCheck)
	}
}

// GetHealth 获取当前健康状态
func (h *Handler) GetHealth(c *gin.Context) {
	health := h.manager.GetCurrentHealth()
	if health == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "尚未完成首次巡检", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": health})
}

// GetTrends 获取健康趋势
func (h *Handler) GetTrends(c *gin.Context) {
	hours := 24
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			hours = v
		}
	}
	trends := h.manager.GetTrends(hours)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": trends})
}

// GetAlerts 获取告警列表
func (h *Handler) GetAlerts(c *gin.Context) {
	resolved := c.Query("resolved") == "true"
	alerts := h.manager.GetAlerts(resolved)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": alerts})
}

// ResolveAlert 解决告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.ResolveAlert(alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "告警已解决"})
}

// GetConfig 获取巡检配置
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": config})
}

// UpdateConfig 更新巡检配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var config PatrolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的配置: " + err.Error()})
		return
	}
	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}

// RunManualCheck 手动巡检
func (h *Handler) RunManualCheck(c *gin.Context) {
	health := h.manager.RunManualCheck()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "巡检完成", "data": health})
}
