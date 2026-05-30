package guidedalerts

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 引导告警HTTP处理器
type Handler struct {
	manager *AlertManager
}

// NewHandler 创建处理器
func NewHandler(manager *AlertManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/guided-alerts")
	{
		group.GET("/alerts", h.ListAlerts)
		group.GET("/alerts/:id", h.GetAlert)
		group.POST("/alerts", h.CreateAlert)
		group.POST("/alerts/:id/ack", h.Acknowledge)
		group.POST("/alerts/:id/resolve", h.Resolve)
		group.GET("/badges", h.GetBadges)
		group.GET("/stats", h.GetStats)
	}
}

// CreateAlertRequest 创建告警请求
type CreateAlertRequest struct {
	Title       string        `json:"title" binding:"required"`
	Description string        `json:"description"`
	Severity    AlertSeverity `json:"severity" binding:"required"`
	Category    AlertCategory `json:"category" binding:"required"`
	Source      string        `json:"source"`
}

// ListAlerts 列出告警
func (h *Handler) ListAlerts(c *gin.Context) {
	severity := AlertSeverity(c.Query("severity"))
	category := AlertCategory(c.Query("category"))
	unresolvedOnly := c.Query("unresolved") == "true"

	alerts := h.manager.ListAlerts(severity, category, unresolvedOnly)
	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

// GetAlert 获取告警详情
func (h *Handler) GetAlert(c *gin.Context) {
	id := c.Param("id")
	alert, ok := h.manager.GetAlert(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	c.JSON(http.StatusOK, alert)
}

// CreateAlert 创建告警
func (h *Handler) CreateAlert(c *gin.Context) {
	var req CreateAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	alert := &Alert{
		Title:       req.Title,
		Description: req.Description,
		Severity:    req.Severity,
		Category:    req.Category,
		Source:      req.Source,
		Guidance: &Guidance{
			Steps: []string{
				"检查相关服务状态",
				"查看详细日志",
				"按指引操作",
			},
			Difficulty: "easy",
		},
		MenuHint: &MenuHint{
			MenuPath: "/" + string(req.Category),
			Badge:    true,
		},
	}

	h.manager.CreateAlert(alert)
	c.JSON(http.StatusCreated, alert)
}

// Acknowledge 确认告警
func (h *Handler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.AcknowledgeAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "alert acknowledged"})
}

// Resolve 解决告警
func (h *Handler) Resolve(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "alert resolved"})
}

// GetBadges 获取菜单徽章
func (h *Handler) GetBadges(c *gin.Context) {
	badges := h.manager.GetMenuBadges()
	c.JSON(http.StatusOK, gin.H{"badges": badges})
}

// GetStats 获取告警统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
