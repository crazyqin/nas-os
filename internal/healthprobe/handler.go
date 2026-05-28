package healthprobe

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 健康探针 HTTP 处理器
type Handler struct {
	manager *Manager
	version string
}

// NewHandler 创建健康探针处理器
func NewHandler(manager *Manager, version string) *Handler {
	return &Handler{
		manager: manager,
		version: version,
	}
}

// RegisterRoutes 注册路由到 API 组
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1")
	{
		api.GET("/health", h.getHealth)
		api.GET("/health/detailed", h.getDetailedHealth)
		api.GET("/health/probes", h.listProbes)
		api.GET("/health/probes/:name", h.getProbeStatus)
		api.GET("/health/history", h.getHistory)
		api.GET("/health/alerts", h.getAlerts)
		api.POST("/health/alerts/:id/resolve", h.resolveAlert)
		api.GET("/health/rules", h.getRules)
		api.POST("/health/rules", h.addRule)
		api.DELETE("/health/rules/:name", h.removeRule)
		api.POST("/health/check", h.triggerCheck)
		api.GET("/health/report", h.getReport)
	}
}

// RegisterSimpleRoutes 注册简单路由（用于直接挂载到 Engine）
func (h *Handler) RegisterSimpleRoutes(r *gin.Engine) {
	r.GET("/health", h.getHealth)
	r.GET("/health/detailed", h.getDetailedHealth)
	r.GET("/healthz", h.healthz)
	r.GET("/readyz", h.readyz)
}

// APIResponse API 响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse 健康端点简化响应
type HealthResponse struct {
	Status    string  `json:"status"`
	Score     float64 `json:"score"`
	Timestamp string  `json:"timestamp"`
	Version   string  `json:"version,omitempty"`
	Uptime    string  `json:"uptime"`
}

// getHealth /health - 简化健康状态
func (h *Handler) getHealth(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		// 尚未执行检查，触发一次
		status = h.manager.Check(c.Request.Context())
	}

	httpStatus := http.StatusOK
	switch status.Level {
	case LevelCritical:
		httpStatus = http.StatusServiceUnavailable
	case LevelDegraded:
		httpStatus = http.StatusOK // 降级仍返回 200
	}

	resp := HealthResponse{
		Status:    string(status.Level),
		Score:     status.Score,
		Timestamp: status.Timestamp.Format(time.RFC3339),
		Version:   h.version,
		Uptime:    status.Uptime.String(),
	}

	c.JSON(httpStatus, APIResponse{
		Code:    0,
		Message: string(status.Level),
		Data:    resp,
	})
}

// getDetailedHealth /health/detailed - 详细健康状态
func (h *Handler) getDetailedHealth(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		status = h.manager.Check(c.Request.Context())
	}

	httpStatus := http.StatusOK
	switch status.Level {
	case LevelCritical:
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, APIResponse{
		Code:    0,
		Message: string(status.Level),
		Data:    status,
	})
}

// healthz /healthz - Kubernetes 存活探针
func (h *Handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// readyz /readyz - Kubernetes 就绪探针
func (h *Handler) readyz(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		status = h.manager.Check(c.Request.Context())
	}

	if status.Level == LevelCritical {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"level":  string(status.Level),
			"score":  status.Score,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"level":  string(status.Level),
		"score":  status.Score,
	})
}

// listProbes /health/probes - 列出所有探针
func (h *Handler) listProbes(c *gin.Context) {
	probes := h.manager.GetProbes()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    probes,
	})
}

// getProbeStatus /health/probes/:name - 获取单个探针状态
func (h *Handler) getProbeStatus(c *gin.Context) {
	name := c.Param("name")

	status := h.manager.GetStatus()
	if status == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    1,
			Message: "尚未执行健康检查",
		})
		return
	}

	probe, exists := status.Probes[name]
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    2,
			Message: "探针未找到: " + name,
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    probe,
	})
}

// getHistory /health/history - 获取历史记录
func (h *Handler) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	history := h.manager.GetHistory(limit)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    history,
	})
}

// getAlerts /health/alerts - 获取告警列表
func (h *Handler) getAlerts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	includeResolvedStr := c.DefaultQuery("includeResolved", "false")
	includeResolved := includeResolvedStr == "true"

	alerts := h.manager.GetAlerts(limit, includeResolved)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    alerts,
	})
}

// resolveAlert /health/alerts/:id/resolve - 解决告警
func (h *Handler) resolveAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    2,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "告警已解决",
	})
}

// getRules /health/rules - 获取所有规则
func (h *Handler) getRules(c *gin.Context) {
	rules := h.manager.GetRules()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    rules,
	})
}

// RuleRequest 规则请求结构
type RuleRequest struct {
	Name      string  `json:"name" binding:"required"`
	Type      string  `json:"type" binding:"required"`
	Category  string  `json:"category"`
	Threshold float64 `json:"threshold"`
	Level     string  `json:"level"`
	Operator  string  `json:"operator"`
	Weight    float64 `json:"weight"`
	Message   string  `json:"message"`
	Enabled   bool    `json:"enabled"`
}

// addRule /health/rules - 添加规则
func (h *Handler) addRule(c *gin.Context) {
	var req RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    3,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	rule := &Rule{
		Name:      req.Name,
		Type:      MetricType(req.Type),
		Category:  ProbeCategory(req.Category),
		Threshold: req.Threshold,
		Level:     HealthLevel(req.Level),
		Operator:  req.Operator,
		Weight:    req.Weight,
		Message:   req.Message,
		Enabled:   req.Enabled,
	}

	h.manager.AddRule(rule)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "规则已添加",
	})
}

// removeRule /health/rules/:name - 删除规则
func (h *Handler) removeRule(c *gin.Context) {
	name := c.Param("name")
	h.manager.RemoveRule(name)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "规则已删除",
	})
}

// triggerCheck /health/check - 手动触发检查
func (h *Handler) triggerCheck(c *gin.Context) {
	status := h.manager.Check(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "检查完成",
		Data:    status,
	})
}

// getReport /health/report - 获取健康报告
func (h *Handler) getReport(c *gin.Context) {
	report := h.manager.GenerateReport()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    report,
	})
}
