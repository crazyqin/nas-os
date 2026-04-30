package webhook

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for webhook management.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new webhook handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers webhook API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	wh := rg.Group("/webhooks")
	{
		wh.GET("", h.list)
		wh.POST("", h.create)
		wh.PUT("/:id", h.update)
		wh.DELETE("/:id", h.delete)
		wh.POST("/test/:id", h.test)
	}
}

func (h *Handlers) list(c *gin.Context) {
	webhooks := h.manager.ListWebhooks()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": webhooks})
}

func (h *Handlers) create(c *gin.Context) {
	var wh Webhook
	if err := c.ShouldBindJSON(&wh); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddWebhook(wh)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Webhook已创建"})
}

func (h *Handlers) update(c *gin.Context) {
	var wh Webhook
	if err := c.ShouldBindJSON(&wh); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	wh.ID = c.Param("id")
	if err := h.manager.UpdateWebhook(wh); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Webhook已更新"})
}

func (h *Handlers) delete(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveWebhook(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Webhook已删除"})
}

func (h *Handlers) test(c *gin.Context) {
	_ = c.Param("id") // future: target specific webhook
	h.manager.FireEvent(Event{
		Type:      "test",
		Timestamp: time.Now(),
		Data:      map[string]string{"message": "测试事件"},
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "测试事件已发送"})
}
