// Package alertguided HTTP API handlers
package alertguided

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 引导式告警HTTP处理器.
type Handlers struct {
	logger     *zap.Logger
	manager    *Manager
	guide      *GuideEngine
	tracker    *RepairTracker
	aggregator *Aggregator
}

// NewHandlers 创建处理器（使用默认知识库和组件）.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	kb := NewKnowledgeBase()
	return &Handlers{
		logger:     logger,
		manager:    mgr,
		guide:      NewGuideEngine(kb, logger),
		tracker:    NewRepairTracker(logger),
		aggregator: NewAggregator(logger, 0),
	}
}

// NewHandlersFull 创建完整处理器（可注入依赖）.
func NewHandlersFull(logger *zap.Logger, mgr *Manager, guide *GuideEngine, tracker *RepairTracker, agg *Aggregator) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		logger:     logger,
		manager:    mgr,
		guide:      guide,
		tracker:    tracker,
		aggregator: agg,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	guided := rg.Group("/alerts/guided")
	{
		// 告警列表和详情
		guided.GET("", h.List)
		guided.GET("/summary", h.Summary)
		guided.GET("/severity/:severity", h.ListBySeverity)
		guided.GET("/status/:status", h.ListByStatus)
		guided.GET("/:id", h.Get)

		// 告警操作
		guided.POST("/:id/acknowledge", h.Acknowledge)
		guided.POST("/:id/silence", h.Silence)
		guided.POST("/:id/status", h.UpdateStatus)

		// 引导式修复
		guided.GET("/:id/guide", h.GetGuide)
		guided.POST("/:id/resolve", h.Resolve)

		// 聚合和追踪
		guided.GET("/aggregations", h.ListAggregations)
		guided.GET("/repairs", h.ListRepairs)
		guided.GET("/repairs/:id", h.GetRepair)

		// 规则管理
		guided.GET("/rules", h.ListRules)
		guided.POST("/rules", h.CreateRule)
	}
}

// List GET /api/v1/alerts/guided.
func (h *Handlers) List(c *gin.Context) {
	alerts := h.manager.List()
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// Get GET /api/v1/alerts/guided/:id.
func (h *Handlers) Get(c *gin.Context) {
	id := c.Param("id")
	alert, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	c.JSON(http.StatusOK, alert)
}

// ListBySeverity GET /api/v1/alerts/guided/severity/:severity.
func (h *Handlers) ListBySeverity(c *gin.Context) {
	severity := Severity(c.Param("severity"))
	alerts := h.manager.ListBySeverity(severity)
	c.JSON(http.StatusOK, gin.H{
		"alerts":   alerts,
		"total":    len(alerts),
		"severity": severity,
	})
}

// ListByStatus GET /api/v1/alerts/guided/status/:status.
func (h *Handlers) ListByStatus(c *gin.Context) {
	status := AlertStatus(c.Param("status"))
	alerts := h.manager.ListByStatus(status)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
		"status": status,
	})
}

// Summary GET /api/v1/alerts/guided/summary.
func (h *Handlers) Summary(c *gin.Context) {
	summary := h.manager.Summary()
	c.JSON(http.StatusOK, summary)
}

// Acknowledge POST /api/v1/alerts/guided/:id/acknowledge.
func (h *Handlers) Acknowledge(c *gin.Context) {
	id := c.Param("id")

	var req AcknowledgeRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.manager.Acknowledge(id, req.Reason); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
}

// Silence POST /api/v1/alerts/guided/:id/silence.
func (h *Handlers) Silence(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Silence(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "silenced"})
}

// UpdateStatus POST /api/v1/alerts/guided/:id/status.
func (h *Handlers) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateStatus(id, req.Status, req.Reason, ""); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}

// GetGuide GET /api/v1/alerts/guided/:id/guide.
func (h *Handlers) GetGuide(c *gin.Context) {
	id := c.Param("id")
	alert, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}

	guide := h.guide.GetGuide(alert)
	c.JSON(http.StatusOK, guide)
}

// Resolve POST /api/v1/alerts/guided/:id/resolve
// 标记告警已解决.
func (h *Handlers) Resolve(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.manager.UpdateStatus(id, StatusResolved, req.Reason, ""); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 清除聚合
	h.aggregator.ResolveByAlertID(id)

	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

// ListAggregations GET /api/v1/alerts/guided/aggregations.
func (h *Handlers) ListAggregations(c *gin.Context) {
	groups := h.aggregator.ListGroups()
	summary := h.aggregator.Summary()
	c.JSON(http.StatusOK, gin.H{
		"groups":  groups,
		"summary": summary,
	})
}

// ListRepairs GET /api/v1/alerts/guided/repairs.
func (h *Handlers) ListRepairs(c *gin.Context) {
	repairs := h.tracker.ListAll()
	c.JSON(http.StatusOK, gin.H{
		"repairs": repairs,
		"total":   len(repairs),
	})
}

// GetRepair GET /api/v1/alerts/guided/repairs/:id.
func (h *Handlers) GetRepair(c *gin.Context) {
	id := c.Param("id")
	record, ok := h.tracker.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "repair record not found"})
		return
	}
	c.JSON(http.StatusOK, record)
}

// ListRules GET /api/v1/alerts/guided/rules.
func (h *Handlers) ListRules(c *gin.Context) {
	rules := h.manager.GetRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// CreateRule POST /api/v1/alerts/guided/rules.
func (h *Handlers) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &AlertRule{
		Name:           req.Name,
		Condition:      req.Condition,
		Severity:       req.Severity,
		Category:       req.Category,
		AggregationKey: req.AggregationKey,
		Tags:           req.Tags,
	}
	h.manager.RegisterRule(rule)
	c.JSON(http.StatusCreated, rule)
}
