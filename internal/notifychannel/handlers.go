package notifychannel

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for notification channels.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new notification channel handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers notification channel API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	nc := rg.Group("/notify-channels")
	{
		nc.GET("", h.listChannels)
		nc.POST("", h.addChannel)
		nc.PUT("/:id", h.updateChannel)
		nc.DELETE("/:id", h.removeChannel)
		nc.POST("/send/:id", h.send)
		nc.POST("/broadcast", h.broadcast)
	}
}

func (h *Handlers) listChannels(c *gin.Context) {
	channels := h.manager.ListChannels()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": channels})
}

func (h *Handlers) addChannel(c *gin.Context) {
	var ch Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddChannel(ch)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "通知渠道已添加"})
}

func (h *Handlers) updateChannel(c *gin.Context) {
	var ch Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ch.ID = c.Param("id")
	if err := h.manager.UpdateChannel(ch); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "通知渠道已更新"})
}

func (h *Handlers) removeChannel(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveChannel(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "通知渠道已删除"})
}

func (h *Handlers) send(c *gin.Context) {
	id := c.Param("id")
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.Send(id, msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "通知已发送"})
}

func (h *Handlers) broadcast(c *gin.Context) {
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.Broadcast(msg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "广播已发送"})
}
