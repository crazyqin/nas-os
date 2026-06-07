// Package notificationcenter HTTP API handlers
package notificationcenter

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 通知中心HTTP处理器
type Handlers struct {
	logger *zap.Logger
	center *Center
}

// NewHandlers 创建处理器
func NewHandlers(logger *zap.Logger, center *Center) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		logger: logger,
		center: center,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	nc := rg.Group("/notifications")
	{
		// 通知列表和详情
		nc.GET("", h.List)
		nc.GET("/summary", h.Summary)
		nc.GET("/:id", h.Get)

		// 通知操作
		nc.POST("/send", h.Send)
		nc.POST("/:id/read", h.MarkAsRead)
		nc.POST("/read-all", h.MarkAllAsRead)
		nc.POST("/:id/archive", h.Archive)
		nc.DELETE("/:id", h.Delete)

		// 模板管理
		nc.GET("/templates", h.ListTemplates)
		nc.GET("/templates/:id", h.GetTemplate)
		nc.POST("/templates", h.CreateTemplate)
		nc.DELETE("/templates/:id", h.DeleteTemplate)

		// 规则管理
		nc.GET("/rules", h.ListRules)
		nc.GET("/rules/:id", h.GetRule)
		nc.POST("/rules", h.CreateRule)
		nc.PUT("/rules/:id", h.UpdateRule)
		nc.DELETE("/rules/:id", h.DeleteRule)
		nc.POST("/rules/evaluate", h.EvaluateRules)

		// 静默时段
		nc.GET("/silent-periods", h.ListSilentPeriods)
		nc.POST("/silent-periods", h.CreateSilentPeriod)
		nc.DELETE("/silent-periods/:id", h.DeleteSilentPeriod)

		// 用户偏好
		nc.GET("/preferences/:user_id", h.GetPreference)
		nc.PUT("/preferences/:user_id", h.SetPreference)
	}
}

// ========== 通知列表与详情 ==========

// List GET /api/v1/notifications
func (h *Handlers) List(c *gin.Context) {
	filter := &ListFilter{
		Category: c.Query("category"),
		Keyword:  c.Query("keyword"),
	}
	if s := c.Query("status"); s != "" {
		st := NotificationStatus(s)
		filter.Status = &st
	}
	if p := c.Query("priority"); p != "" {
		prio := Priority(p)
		filter.Priority = &prio
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	notifications := h.center.List(filter)

	total := len(notifications)
	if filter.Offset > 0 && filter.Offset < len(notifications) {
		notifications = notifications[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(notifications) {
		notifications = notifications[:filter.Limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"total":         total,
	})
}

// Get GET /api/v1/notifications/:id
func (h *Handlers) Get(c *gin.Context) {
	id := c.Param("id")
	notif, err := h.center.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, notif)
}

// Summary GET /api/v1/notifications/summary
func (h *Handlers) Summary(c *gin.Context) {
	summary := h.center.Summary()
	c.JSON(http.StatusOK, summary)
}

// ========== 通知操作 ==========

// Send POST /api/v1/notifications/send
func (h *Handlers) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Priority == "" {
		req.Priority = PriorityMedium
	}

	notif, err := h.center.Send(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, notif)
}

// MarkAsRead POST /api/v1/notifications/:id/read
func (h *Handlers) MarkAsRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.MarkAsRead(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MarkAllAsRead POST /api/v1/notifications/read-all
func (h *Handlers) MarkAllAsRead(c *gin.Context) {
	count := h.center.MarkAllAsRead()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"count":  count,
	})
}

// Archive POST /api/v1/notifications/:id/archive
func (h *Handlers) Archive(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.Archive(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Delete DELETE /api/v1/notifications/:id
func (h *Handlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== 模板管理 ==========

// ListTemplates GET /api/v1/notifications/templates
func (h *Handlers) ListTemplates(c *gin.Context) {
	templates := h.center.ListTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetTemplate GET /api/v1/notifications/templates/:id
func (h *Handlers) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	tmpl, err := h.center.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tmpl)
}

// CreateTemplate POST /api/v1/notifications/templates
func (h *Handlers) CreateTemplate(c *gin.Context) {
	var tmpl NotificationTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.center.AddTemplate(&tmpl)
	c.JSON(http.StatusCreated, tmpl)
}

// DeleteTemplate DELETE /api/v1/notifications/templates/:id
func (h *Handlers) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== 规则管理 ==========

// ListRules GET /api/v1/notifications/rules
func (h *Handlers) ListRules(c *gin.Context) {
	rules := h.center.ListRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// GetRule GET /api/v1/notifications/rules/:id
func (h *Handlers) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.center.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// CreateRule POST /api/v1/notifications/rules
func (h *Handlers) CreateRule(c *gin.Context) {
	var rule NotificationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.center.AddRule(&rule); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// UpdateRule PUT /api/v1/notifications/rules/:id
func (h *Handlers) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule NotificationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id
	if err := h.center.UpdateRule(&rule); err != nil {
		if err == ErrRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, rule)
}

// DeleteRule DELETE /api/v1/notifications/rules/:id
func (h *Handlers) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// EvaluateRules POST /api/v1/notifications/rules/evaluate
func (h *Handlers) EvaluateRules(c *gin.Context) {
	var event map[string]interface{}
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fired, err := h.center.EvaluateRules(c.Request.Context(), event)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"fired": fired,
		"count": len(fired),
	})
}

// ========== 静默时段 ==========

// ListSilentPeriods GET /api/v1/notifications/silent-periods
func (h *Handlers) ListSilentPeriods(c *gin.Context) {
	periods := h.center.ListSilentPeriods()
	c.JSON(http.StatusOK, gin.H{
		"silent_periods": periods,
		"total":          len(periods),
	})
}

// CreateSilentPeriod POST /api/v1/notifications/silent-periods
func (h *Handlers) CreateSilentPeriod(c *gin.Context) {
	var sp SilentPeriod
	if err := c.ShouldBindJSON(&sp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.center.AddSilentPeriod(&sp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sp)
}

// DeleteSilentPeriod DELETE /api/v1/notifications/silent-periods/:id
func (h *Handlers) DeleteSilentPeriod(c *gin.Context) {
	id := c.Param("id")
	if err := h.center.DeleteSilentPeriod(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== 用户偏好 ==========

// GetPreference GET /api/v1/notifications/preferences/:user_id
func (h *Handlers) GetPreference(c *gin.Context) {
	userID := c.Param("user_id")
	pref := h.center.GetPreference(userID)
	c.JSON(http.StatusOK, pref)
}

// SetPreference PUT /api/v1/notifications/preferences/:user_id
func (h *Handlers) SetPreference(c *gin.Context) {
	userID := c.Param("user_id")
	var pref UserPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pref.UserID = userID
	h.center.SetPreference(&pref)
	c.JSON(http.StatusOK, pref)
}
