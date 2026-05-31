// Package sysdashboard 系统仪表盘 - HTTP API
package sysdashboard

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
	group := router.Group("/dashboard")
	{
		group.GET("", h.GetDashboard)
		group.GET("/activities", h.GetActivities)
		group.GET("/alerts", h.GetAlerts)
		group.POST("/alerts/:id/resolve", h.ResolveAlert)
	}
}

// GetDashboard 获取仪表盘数据
func (h *Handler) GetDashboard(c *gin.Context) {
	data := h.manager.GetDashboard()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

// GetActivities 获取活动记录
func (h *Handler) GetActivities(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	activities := h.manager.GetActivities(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": activities})
}

// GetAlerts 获取告警
func (h *Handler) GetAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": alerts})
}

// ResolveAlert 解决告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if !h.manager.ResolveAlert(alertID) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "告警不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "告警已解决"})
}
