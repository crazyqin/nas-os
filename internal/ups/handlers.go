package ups

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for UPS management.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new UPS handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers UPS API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	ups := rg.Group("/ups")
	{
		ups.GET("/status", h.getStatus)
		ups.GET("/config", h.getConfig)
		ups.PUT("/config", h.updateConfig)
		ups.GET("/health", h.getHealthScore)
	}
}

func (h *Handlers) getStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": config})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config UPSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}

func (h *Handlers) getHealthScore(c *gin.Context) {
	status := h.manager.GetStatus()
	score := GetBatteryHealthScore(status)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"score":        score,
			"status":       status.Status,
			"battery":      status.BatteryLevel,
			"temperature":  status.Temperature,
			"load":         status.LoadPercent,
		},
	})
}
