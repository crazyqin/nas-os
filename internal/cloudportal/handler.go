// Package cloudportal HTTP 处理器
package cloudportal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 门户 HTTP 处理器.
type Handler struct {
	manager *PortalManager
}

// NewHandler 创建处理器.
func NewHandler(manager *PortalManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	pg := rg.Group("/cloudportal")
	{
		pg.GET("/stats", h.getStats)
		pg.GET("/devices", h.getDevices)
		pg.GET("/devices/:id", h.getDevice)
		pg.POST("/devices", h.registerDevice)
		pg.DELETE("/devices/:id", h.removeDevice)
		pg.PUT("/devices/:id/status", h.updateDeviceStatus)
		pg.POST("/sessions", h.createSession)
		pg.GET("/sessions/:id/validate", h.validateSession)
		pg.POST("/sync", h.syncConfig)
		pg.GET("/remote/:id", h.getRemoteURL)
	}
}

func (h *Handler) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *Handler) getDevices(c *gin.Context) {
	devices := h.manager.GetDevices()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": devices})
}

func (h *Handler) getDevice(c *gin.Context) {
	deviceID := c.Param("id")
	device, err := h.manager.GetDevice(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": device})
}

func (h *Handler) registerDevice(c *gin.Context) {
	var device Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := h.manager.RegisterDevice(&device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": device})
}

func (h *Handler) removeDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if err := h.manager.RemoveDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) updateDeviceStatus(c *gin.Context) {
	deviceID := c.Param("id")
	var status Device
	if err := c.ShouldBindJSON(&status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := h.manager.UpdateDeviceStatus(deviceID, &status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) createSession(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
		Duration int    `json:"duration_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	duration := time.Duration(req.Duration) * time.Hour
	if duration == 0 {
		duration = 24 * time.Hour
	}

	session, err := h.manager.CreateSession(req.DeviceID, req.UserID, c.ClientIP(), duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

func (h *Handler) validateSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := h.manager.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

func (h *Handler) syncConfig(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	config := &SyncConfig{
		SyncSettings: true,
		SyncUsers:    true,
		SyncShares:   true,
	}

	if err := h.manager.SyncConfig(req.DeviceID, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置同步成功"})
}

func (h *Handler) getRemoteURL(c *gin.Context) {
	deviceID := c.Param("id")
	url, err := h.manager.GetRemoteAccessURL(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"url": url}})
}
