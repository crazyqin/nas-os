package recyclecleaner

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for recycle bin cleaning.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new recycle cleaner handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers recycle cleaner API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	rc := rg.Group("/recycle-cleaner")
	{
		rc.GET("/rules", h.listRules)
		rc.POST("/rules", h.addRule)
		rc.PUT("/rules/:id", h.updateRule)
		rc.DELETE("/rules/:id", h.removeRule)
		rc.GET("/stats/:id", h.getStats)
		rc.POST("/run/:id", h.runNow)
	}
}

func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rules})
}

func (h *Handlers) addRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddRule(rule)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已添加"})
}

func (h *Handlers) updateRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	rule.ID = c.Param("id")
	if err := h.manager.UpdateRule(rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已更新"})
}

func (h *Handlers) removeRule(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveRule(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已删除"})
}

func (h *Handlers) getStats(c *gin.Context) {
	id := c.Param("id")
	stats := h.manager.GetStats(id)
	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则未找到"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

func (h *Handlers) runNow(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.RunCleanNow(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
