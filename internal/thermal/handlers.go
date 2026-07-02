// handlers.go - 温控管理 HTTP 接口
package thermal

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 温控接口处理器.
type Handlers struct {
	logger  *zap.Logger
	manager *Manager
}

// NewHandlers 创建温控接口处理器.
func NewHandlers(logger *zap.Logger, manager *Manager) *Handlers {
	return &Handlers{logger: logger, manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	thermal := rg.Group("/thermal")
	{
		thermal.GET("/overview", h.getOverview)
		thermal.GET("/zones", h.getZones)
		thermal.GET("/fans", h.getFans)
		thermal.GET("/alerts", h.getAlerts)
		thermal.GET("/history", h.getHistory)
		thermal.GET("/policy", h.getPolicy)
		thermal.PUT("/policy", h.updatePolicy)
		thermal.POST("/refresh", h.refresh)
		thermal.PUT("/fan/:id/mode", h.setFanMode)
		thermal.PUT("/fan/:id/speed", h.setFanSpeed)
		thermal.DELETE("/alerts", h.clearAlerts)
	}
}

// getOverview 获取散热总览.
func (h *Handlers) getOverview(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": overview})
}

// getZones 获取温度区域列表.
func (h *Handlers) getZones(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": overview.Zones})
}

// getFans 获取风扇信息.
func (h *Handlers) getFans(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": overview.Fans})
}

// getAlerts 获取温度告警.
func (h *Handlers) getAlerts(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	alerts := h.manager.GetAlerts(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": alerts})
}

// getHistory 获取温度历史.
func (h *Handlers) getHistory(c *gin.Context) {
	minutes := 60
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}
	history := h.manager.GetHistory(minutes)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": history})
}

// getPolicy 获取温控策略.
func (h *Handlers) getPolicy(c *gin.Context) {
	policy := h.manager.GetPolicy()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": policy})
}

// updatePolicy 更新温控策略.
func (h *Handlers) updatePolicy(c *gin.Context) {
	var policy ThermalPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的策略参数"})
		return
	}
	h.manager.UpdatePolicy(policy)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "策略已更新"})
}

// refresh 刷新温度数据.
func (h *Handlers) refresh(c *gin.Context) {
	h.manager.Refresh()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "温度数据已刷新", "time": time.Now()})
}

// setFanMode 设置风扇模式.
func (h *Handlers) setFanMode(c *gin.Context) {
	fanID := c.Param("id")
	var req struct {
		Mode FanMode `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求参数"})
		return
	}
	if err := h.manager.SetFanMode(fanID, req.Mode); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "风扇模式已设置"})
}

// setFanSpeed 手动设置风扇转速.
func (h *Handlers) setFanSpeed(c *gin.Context) {
	fanID := c.Param("id")
	var req struct {
		Percent float64 `json:"percent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求参数"})
		return
	}
	if err := h.manager.SetFanSpeed(fanID, req.Percent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "风扇转速已设置"})
}

// clearAlerts 清空告警.
func (h *Handlers) clearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "告警已清空"})
}
