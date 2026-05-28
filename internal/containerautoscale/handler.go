// Package containerautoscale 提供容器自动扩缩容 REST API 处理器
package containerautoscale

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 容器自动扩缩容 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ca := r.Group("/container-autoscale")
	{
		// 容器管理
		ca.GET("/containers", h.listContainers)
		ca.POST("/containers", h.registerContainer)
		ca.DELETE("/containers/:service", h.unregisterContainer)

		// 手动扩缩
		ca.POST("/scale", h.manualScale)

		// 策略管理
		ca.GET("/policies", h.listPolicies)
		ca.GET("/policies/:service", h.getPolicy)
		ca.POST("/policies", h.setPolicy)
		ca.DELETE("/policies/:service", h.deletePolicy)

		// 配额管理
		ca.GET("/quotas", h.listQuotas)
		ca.GET("/quotas/:service", h.getQuota)
		ca.POST("/quotas", h.setQuota)
		ca.DELETE("/quotas/:service", h.deleteQuota)

		// 指标查询
		ca.GET("/metrics", h.getMetrics)
		ca.POST("/metrics", h.recordMetric)

		// 扩缩历史
		ca.GET("/events", h.getScaleEvents)

		// 预测
		ca.GET("/predict/:service", h.predict)

		// 成本优化
		ca.GET("/cost/suggestions", h.costSuggestions)

		// 告警
		ca.GET("/alerts", h.getAlerts)
		ca.POST("/alerts/:id/resolve", h.resolveAlert)

		// 配置
		ca.GET("/config", h.getConfig)
		ca.PUT("/config", h.updateConfig)
	}
}

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: data})
}

func created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "created", Data: data})
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: msg})
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: msg})
}

func serverError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: msg})
}

// listContainers 列出容器
func (h *Handlers) listContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	ok(c, containers)
}

// registerContainer 注册容器
func (h *Handlers) registerContainer(c *gin.Context) {
	var container Container
	if err := c.ShouldBindJSON(&container); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}
	h.manager.RegisterContainer(&container)
	created(c, container)
}

// unregisterContainer 注销容器
func (h *Handlers) unregisterContainer(c *gin.Context) {
	service := c.Param("service")
	h.manager.UnregisterContainer(service)
	ok(c, nil)
}

// manualScale 手动扩缩
func (h *Handlers) manualScale(c *gin.Context) {
	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}

	event, err := h.manager.ManualScale(c.Request.Context(), &req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	ok(c, event)
}

// listPolicies 列出策略
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	ok(c, policies)
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	service := c.Param("service")
	policy, exists := h.manager.GetPolicy(service)
	if !exists {
		notFound(c, "policy not found for service: "+service)
		return
	}
	ok(c, policy)
}

// setPolicy 设置策略
func (h *Handlers) setPolicy(c *gin.Context) {
	var req PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}

	policy := &ScalePolicy{
		ServiceName:     req.ServiceName,
		Strategy:        req.Strategy,
		Enabled:         true,
		MetricType:      req.MetricType,
		Threshold:       req.Threshold,
		Schedules:       req.Schedules,
		CooldownSec:     req.CooldownSec,
		CooldownUpSec:   req.CooldownUpSec,
		CooldownDownSec: req.CooldownDownSec,
	}
	h.manager.SetPolicy(policy)
	created(c, policy)
}

// deletePolicy 删除策略
func (h *Handlers) deletePolicy(c *gin.Context) {
	service := c.Param("service")
	h.manager.DeletePolicy(service)
	ok(c, nil)
}

// listQuotas 列出配额
func (h *Handlers) listQuotas(c *gin.Context) {
	quotas := h.manager.ListQuotas()
	ok(c, quotas)
}

// getQuota 获取配额
func (h *Handlers) getQuota(c *gin.Context) {
	service := c.Param("service")
	quota, exists := h.manager.GetQuota(service)
	if !exists {
		notFound(c, "quota not found for service: "+service)
		return
	}
	ok(c, quota)
}

// setQuota 设置配额
func (h *Handlers) setQuota(c *gin.Context) {
	var req QuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}

	quota := &ResourceQuota{
		ServiceName:   req.ServiceName,
		MaxCPU:        req.MaxCPU,
		MaxMemoryMB:   req.MaxMemoryMB,
		MaxReplicas:   req.MaxReplicas,
		MinReplicas:   req.MinReplicas,
		MaxCostPerDay: req.MaxCostPerDay,
	}
	h.manager.SetQuota(quota)
	created(c, quota)
}

// deleteQuota 删除配额
func (h *Handlers) deleteQuota(c *gin.Context) {
	service := c.Param("service")
	h.manager.DeleteQuota(service)
	ok(c, nil)
}

// getMetrics 获取指标
func (h *Handlers) getMetrics(c *gin.Context) {
	serviceName := c.Query("service")
	metricType := MetricType(c.Query("type"))

	var startTime, endTime time.Time
	if s := c.Query("start"); s != "" {
		startTime, _ = time.Parse(time.RFC3339, s)
	}
	if e := c.Query("end"); e != "" {
		endTime, _ = time.Parse(time.RFC3339, e)
	}

	query := &MetricsQuery{
		ServiceName: serviceName,
		MetricType:  metricType,
		StartTime:   startTime,
		EndTime:     endTime,
	}
	metrics := h.manager.GetMetrics(query)
	ok(c, metrics)
}

// recordMetric 记录指标
func (h *Handlers) recordMetric(c *gin.Context) {
	var mp MetricPoint
	if err := c.ShouldBindJSON(&mp); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}
	if mp.Timestamp.IsZero() {
		mp.Timestamp = time.Now()
	}
	h.manager.RecordMetric(mp)
	created(c, mp)
}

// getScaleEvents 获取扩缩历史
func (h *Handlers) getScaleEvents(c *gin.Context) {
	service := c.Query("service")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	events := h.manager.GetScaleEvents(service, limit)
	ok(c, events)
}

// predict 预测
func (h *Handlers) predict(c *gin.Context) {
	service := c.Param("service")
	metricType := MetricType(c.DefaultQuery("type", string(MetricCPU)))
	horizon := c.DefaultQuery("horizon", "15m")

	result := h.manager.Predict(c.Request.Context(), service, metricType, horizon)
	if result == nil {
		badRequest(c, "insufficient data for prediction")
		return
	}
	ok(c, result)
}

// costSuggestions 成本优化建议
func (h *Handlers) costSuggestions(c *gin.Context) {
	suggestions := h.manager.GenerateCostSuggestions()
	ok(c, suggestions)
}

// getAlerts 获取告警
func (h *Handlers) getAlerts(c *gin.Context) {
	resolvedStr := c.DefaultQuery("resolved", "false")
	resolved := resolvedStr == "true"
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	alerts := h.manager.GetAlerts(resolved, limit)
	ok(c, alerts)
}

// resolveAlert 解决告警
func (h *Handlers) resolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.ResolveAlert(alertID); err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, nil)
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	ok(c, cfg)
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg AutoScaleConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		badRequest(c, "invalid request: "+err.Error())
		return
	}
	h.manager.UpdateConfig(&cfg)
	ok(c, cfg)
}
