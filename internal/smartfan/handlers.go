// Package smartfan 智能风扇控制 - HTTP API
package smartfan

import (
	"net/http"

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
	group := router.Group("/fan")
	{
		group.GET("", h.GetFans)
		group.GET("/sensors", h.GetSensors)
		group.GET("/profiles", h.GetProfiles)
		group.POST("/profiles/:id/apply", h.SetProfile)
		group.PUT("/:id/mode", h.SetFanMode)
		group.GET("/alerts", h.GetAlerts)
	}
}

// GetFans 获取风扇状态
func (h *Handler) GetFans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetFans()})
}

// GetSensors 获取温度传感器
func (h *Handler) GetSensors(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetSensors()})
}

// GetProfiles 获取配置曲线
func (h *Handler) GetProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetProfiles()})
}

// SetProfile 设置活跃配置
func (h *Handler) SetProfile(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.SetProfile(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已应用"})
}

// SetFanMode 设置风扇模式
func (h *Handler) SetFanMode(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Mode        FanMode `json:"mode"`
		DutyPercent int     `json:"duty_percent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	if err := h.manager.SetFanMode(id, req.Mode, req.DutyPercent); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "风扇模式已更新"})
}

// GetAlerts 获取告警
func (h *Handler) GetAlerts(c *gin.Context) {
	resolved := c.Query("resolved") == "true"
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetAlerts(resolved)})
}
