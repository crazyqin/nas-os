package wol

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for WOL management.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new WOL handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers WOL API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	wol := rg.Group("/wol")
	{
		wol.GET("/devices", h.listDevices)
		wol.POST("/devices", h.addDevice)
		wol.DELETE("/devices/:mac", h.removeDevice)
		wol.POST("/wake/:mac", h.wakeDevice)
		wol.POST("/wake-group/:group", h.wakeGroup)
	}
}

func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": devices})
}

func (h *Handlers) addDevice(c *gin.Context) {
	var dev Device
	if err := c.ShouldBindJSON(&dev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.AddDevice(dev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备已添加"})
}

func (h *Handlers) removeDevice(c *gin.Context) {
	mac := c.Param("mac")
	h.manager.RemoveDevice(mac)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备已删除"})
}

func (h *Handlers) wakeDevice(c *gin.Context) {
	mac := c.Param("mac")
	if err := h.manager.Wake(mac); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "唤醒包已发送"})
}

func (h *Handlers) wakeGroup(c *gin.Context) {
	group := c.Param("group")
	errs := h.manager.WakeGroup(group)
	if len(errs) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{"code": 207, "message": "部分设备唤醒失败", "errors": len(errs)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "组唤醒包已发送"})
}
