package guidedalert

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 引导告警HTTP处理器
type Handler struct {
	manager *GuidedAlertManager
}

// NewHandler 创建处理器
func NewHandler(manager *GuidedAlertManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ga := rg.Group("/guided-alerts")
	{
		ga.GET("", h.List)
		ga.GET("/:id", h.Get)
		ga.POST("/fire", h.Fire)
		ga.POST("/:id/ack", h.Acknowledge)
		ga.POST("/:id/resolve", h.Resolve)
		ga.GET("/menu-indicators", h.MenuIndicators)
	}
}

// List 获取告警列表
func (h *Handler) List(c *gin.Context) {
	status := AlertStatus(c.Query("status"))
	severity := AlertSeverity(c.Query("severity"))
	alerts := h.manager.List(status, severity)
	c.JSON(http.StatusOK, alerts)
}

// Get 获取单个告警
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	alert, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	c.JSON(http.StatusOK, alert)
}

// FireRequest 触发告警请求
type FireRequest struct {
	Code       string `json:"code" binding:"required"`
	Message    string `json:"message" binding:"required"`
	ResourceID string `json:"resourceId"`
}

// Fire 触发告警
func (h *Handler) Fire(c *gin.Context) {
	var req FireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alert := h.manager.Fire(req.Code, req.Message, req.ResourceID)
	c.JSON(http.StatusCreated, alert)
}

// Acknowledge 确认告警
func (h *Handler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	user := c.Query("user")
	if user == "" {
		user = "admin"
	}
	if err := h.manager.Acknowledge(id, user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
}

// Resolve 解决告警
func (h *Handler) Resolve(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Resolve(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

// MenuIndicators 获取菜单指示器
func (h *Handler) MenuIndicators(c *gin.Context) {
	indicators := h.manager.GetMenuIndicators()
	c.JSON(http.StatusOK, indicators)
}
