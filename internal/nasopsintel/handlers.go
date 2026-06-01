package nasopsintel

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 运维智能 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	ops := rg.Group("/ops")
	{
		// 事件管理
		ops.GET("/events", h.listEvents)
		ops.POST("/events", h.ingestEvent)

		// 运维事件
		ops.GET("/incidents", h.listIncidents)
		ops.GET("/incidents/:id", h.getIncident)
		ops.PUT("/incidents/:id/resolve", h.resolveIncident)

		// 异常检测
		ops.GET("/anomalies", h.listAnomalies)

		// 健康评分
		ops.GET("/health", h.getHealth)

		// 指标
		ops.GET("/metrics", h.getMetrics)

		// 规则管理
		ops.GET("/rules", h.listRules)
		ops.POST("/rules", h.addRule)
	}
}

// ingestEvent 接收事件请求
type ingestEventRequest struct {
	Source      EventSource            `json:"source" binding:"required"`
	Severity    Severity               `json:"severity" binding:"required"`
	Title       string                 `json:"title" binding:"required"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Host        string                 `json:"host,omitempty"`
	Service     string                 `json:"service,omitempty"`
}

// ingestEvent 接收事件
func (h *Handlers) ingestEvent(c *gin.Context) {
	var req ingestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := OpsEvent{
		Source:      req.Source,
		Severity:    req.Severity,
		Title:       req.Title,
		Description: req.Description,
		Metadata:    req.Metadata,
		Host:        req.Host,
		Service:     req.Service,
	}

	h.manager.IngestEvent(event)
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

// listEvents 列出事件
func (h *Handlers) listEvents(c *gin.Context) {
	limit := parseIntParam(c, "limit", 100)
	source := EventSource(c.Query("source"))
	severity := Severity(c.Query("severity"))

	events := h.manager.ListEvents(limit, source, severity)
	c.JSON(http.StatusOK, gin.H{"events": events, "total": len(events)})
}

// listIncidents 列出运维事件
func (h *Handlers) listIncidents(c *gin.Context) {
	status := IncidentStatus(c.Query("status"))
	incidents := h.manager.ListIncidents(status)
	c.JSON(http.StatusOK, gin.H{"incidents": incidents, "total": len(incidents)})
}

// getIncident 获取运维事件
func (h *Handlers) getIncident(c *gin.Context) {
	id := c.Param("id")
	inc, err := h.manager.GetIncident(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, inc)
}

// resolveIncidentRequest 解决事件请求
type resolveIncidentRequest struct {
	RootCause   string `json:"root_cause" binding:"required"`
	Remediation string `json:"remediation"`
}

// resolveIncident 解决事件
func (h *Handlers) resolveIncident(c *gin.Context) {
	id := c.Param("id")
	var req resolveIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ResolveIncident(id, req.RootCause, req.Remediation); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

// listAnomalies 列出异常
func (h *Handlers) listAnomalies(c *gin.Context) {
	limit := parseIntParam(c, "limit", 50)
	anomalies := h.manager.ListAnomalies(limit)
	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies, "total": len(anomalies)})
}

// getHealth 获取健康评分
func (h *Handlers) getHealth(c *gin.Context) {
	health := h.manager.GetHealthScore()
	c.JSON(http.StatusOK, health)
}

// getMetrics 获取运维指标
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// addRuleRequest 添加规则请求
type addRuleRequest struct {
	ID          string      `json:"id" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description"`
	Source      EventSource `json:"source"`
	Condition   string      `json:"condition"`
	Severity    Severity    `json:"severity"`
	Enabled     bool        `json:"enabled"`
	Actions     []string    `json:"actions"`
}

// addRule 添加规则
func (h *Handlers) addRule(c *gin.Context) {
	var req addRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &Rule{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Source:      req.Source,
		Condition:   req.Condition,
		Severity:    req.Severity,
		Enabled:     req.Enabled,
		Actions:     req.Actions,
	}

	if err := h.manager.AddRule(rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// listRules 列出规则
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules, "total": len(rules)})
}

func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	val := c.Query(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
