// Package guidedalert HTTP handlers
package guidedalert

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 引导式告警HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	guided := rg.Group("/alerts/guided")
	{
		guided.GET("", h.List)
		guided.GET("/summary", h.Summary)
		guided.GET("/:id", h.Get)
		guided.POST("/:id/acknowledge", h.Acknowledge)
		guided.POST("/:id/silence", h.Silence)
		guided.POST("/rules", h.CreateRule)
	}
}

// List GET /api/v1/alerts/guided
func (h *Handler) List(c *gin.Context) {
	alerts := h.manager.List()
	c.JSON(http.StatusOK, alerts)
}

// Get GET /api/v1/alerts/guided/:id
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	alert, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	c.JSON(http.StatusOK, alert)
}

// Acknowledge POST /api/v1/alerts/guided/:id/acknowledge
func (h *Handler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Acknowledge(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
}

// Silence POST /api/v1/alerts/guided/:id/silence
func (h *Handler) Silence(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Silence(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "silenced"})
}

// Summary GET /api/v1/alerts/guided/summary
func (h *Handler) Summary(c *gin.Context) {
	summary := h.manager.Summary()
	c.JSON(http.StatusOK, summary)
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name     string   `json:"name" binding:"required"`
	Condition string  `json:"condition" binding:"required"`
	Severity Severity `json:"severity" binding:"required"`
	Category Category `json:"category" binding:"required"`
}

// CreateRule POST /api/v1/alerts/guided/rules
func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule := &AlertRule{
		Name:      req.Name,
		Condition: req.Condition,
		Severity:  req.Severity,
		Category:  req.Category,
	}
	h.manager.RegisterRule(rule)
	c.JSON(http.StatusCreated, rule)
}
