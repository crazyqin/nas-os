package ransommldetect

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for ransomware ML detection.
type Handlers struct {
	detector *Detector
}

// NewHandlers creates new ransomware detection handlers.
func NewHandlers(detector *Detector) *Handlers {
	return &Handlers{detector: detector}
}

// RegisterRoutes registers ransomware detection API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	rm := rg.Group("/ransom-detection")
	{
		rm.GET("/alerts", h.getAlerts)
		rm.GET("/config", h.getConfig)
		rm.PUT("/config", h.updateConfig)
		rm.POST("/record", h.recordActivity)
	}
}

func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.detector.GetAlerts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": alerts})
}

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.detector.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": config})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config DetectorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.detector.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}

func (h *Handlers) recordActivity(c *gin.Context) {
	var activity FileActivity
	if err := c.ShouldBindJSON(&activity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now()
	}
	h.detector.RecordActivity(activity)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "活动已记录"})
}
