// Package storagehealthgrade - HTTP API 处理器
package storagehealthgrade

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 存储健康评分 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	health := rg.Group("/storage-health")
	{
		// 健康评估
		health.POST("/assess", h.runAssessment)
		health.GET("/current", h.getCurrent)
		health.GET("/history", h.getHistory)
		health.GET("/alerts", h.getAlerts)
		health.GET("/stats", h.getStats)
	}
}

func (h *Handlers) runAssessment(c *gin.Context) {
	report := h.manager.RunAssessment()
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) getCurrent(c *gin.Context) {
	report := h.manager.GetCurrent()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no assessment available, run /assess first"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) getHistory(c *gin.Context) {
	history := h.manager.GetHistory()
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}
