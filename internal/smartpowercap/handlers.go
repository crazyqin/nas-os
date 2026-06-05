// Package smartpowercap 提供功耗智能封顶 HTTP 处理器
package smartpowercap

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 功耗智能封顶 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	powerGroup := api.Group("/power-cap")
	{
		// 状态查询
		powerGroup.GET("/status", h.getStatus)
		powerGroup.GET("/reading", h.getCurrentReading)

		// 模式控制
		powerGroup.GET("/mode", h.getMode)
		powerGroup.PUT("/mode", h.setMode)

		// 预算管理
		powerGroup.GET("/budgets", h.listBudgets)
		powerGroup.POST("/budgets", h.addBudget)
		powerGroup.GET("/budgets/:id", h.getBudget)
		powerGroup.PUT("/budgets/:id", h.updateBudget)
		powerGroup.DELETE("/budgets/:id", h.removeBudget)

		// 限制管理
		powerGroup.GET("/limits", h.listLimits)
		powerGroup.POST("/limits", h.addLimit)
		powerGroup.GET("/limits/:id", h.getLimit)
		powerGroup.PUT("/limits/:id", h.updateLimit)

		// 策略管理
		powerGroup.GET("/policies", h.listPolicies)
		powerGroup.POST("/policies", h.addPolicy)
		powerGroup.GET("/policies/:id", h.getPolicy)
		powerGroup.PUT("/policies/:id", h.updatePolicy)
		powerGroup.POST("/policies/:id/apply", h.applyPolicy)

		// 报表和趋势
		powerGroup.GET("/report", h.getReport)
		powerGroup.GET("/trends", h.getTrends)

		// 告警
		powerGroup.GET("/alerts", h.getAlerts)
		powerGroup.DELETE("/alerts", h.clearAlerts)

		// 电费估算
		powerGroup.POST("/estimate-cost", h.estimateCost)
	}
}

// getStatus 获取状态
func (h *Handlers) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"mode":  h.manager.GetMode(),
		"state": h.manager.GetState(),
	})
}

// getCurrentReading 获取当前功耗读数
func (h *Handlers) getCurrentReading(c *gin.Context) {
	reading := h.manager.GetCurrentReading()
	c.JSON(http.StatusOK, reading)
}

// getMode 获取当前模式
func (h *Handlers) getMode(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"mode": h.manager.GetMode(),
	})
}

// setModeRequest 设置模式请求
type setModeRequest struct {
	Mode PowerMode `json:"mode" binding:"required"`
}

// setMode 设置模式
func (h *Handlers) setMode(c *gin.Context) {
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetMode(req.Mode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "模式设置成功",
		"mode":    req.Mode,
	})
}

// listBudgets 列出所有预算
func (h *Handlers) listBudgets(c *gin.Context) {
	budgets := h.manager.ListBudgets()
	c.JSON(http.StatusOK, gin.H{
		"budgets": budgets,
		"total":   len(budgets),
	})
}

// addBudgetRequest 添加预算请求
type addBudgetRequest struct {
	ID       string  `json:"id" binding:"required"`
	Name     string  `json:"name" binding:"required"`
	MaxPower float64 `json:"maxPower" binding:"required"`
	Enabled  bool    `json:"enabled"`
}

// addBudget 添加预算
func (h *Handlers) addBudget(c *gin.Context) {
	var req addBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	budget := &PowerBudget{
		ID:       req.ID,
		Name:     req.Name,
		MaxPower: req.MaxPower,
		Enabled:  req.Enabled,
	}

	if err := h.manager.AddBudget(budget); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "预算添加成功",
		"budget":  budget,
	})
}

// getBudget 获取预算
func (h *Handlers) getBudget(c *gin.Context) {
	budgetID := c.Param("id")
	budget, err := h.manager.GetBudget(budgetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budget)
}

// updateBudgetRequest 更新预算请求
type updateBudgetRequest struct {
	MaxPower float64 `json:"maxPower" binding:"required"`
}

// updateBudget 更新预算
func (h *Handlers) updateBudget(c *gin.Context) {
	budgetID := c.Param("id")

	var req updateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdateBudget(budgetID, req.MaxPower); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "预算更新成功",
	})
}

// removeBudget 移除预算
func (h *Handlers) removeBudget(c *gin.Context) {
	budgetID := c.Param("id")
	if err := h.manager.RemoveBudget(budgetID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "预算已移除",
	})
}

// listLimits 列出所有限制
func (h *Handlers) listLimits(c *gin.Context) {
	limits := h.manager.ListLimits()
	c.JSON(http.StatusOK, gin.H{
		"limits": limits,
		"total":  len(limits),
	})
}

// addLimitRequest 添加限制请求
type addLimitRequest struct {
	ID        string  `json:"id" binding:"required"`
	Name      string  `json:"name" binding:"required"`
	PeakPower float64 `json:"peakPower" binding:"required"`
	Sustained float64 `json:"sustained" binding:"required"`
	Duration  int     `json:"duration" binding:"required"`
	Enabled   bool    `json:"enabled"`
}

// addLimit 添加限制
func (h *Handlers) addLimit(c *gin.Context) {
	var req addLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	limit := &PowerLimit{
		ID:        req.ID,
		Name:      req.Name,
		PeakPower: req.PeakPower,
		Sustained: req.Sustained,
		Duration:  req.Duration,
		Enabled:   req.Enabled,
	}

	if err := h.manager.AddLimit(limit); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "限制添加成功",
		"limit":   limit,
	})
}

// getLimit 获取限制
func (h *Handlers) getLimit(c *gin.Context) {
	limitID := c.Param("id")
	limit, err := h.manager.GetLimit(limitID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, limit)
}

// updateLimitRequest 更新限制请求
type updateLimitRequest struct {
	PeakPower float64 `json:"peakPower" binding:"required"`
	Sustained float64 `json:"sustained" binding:"required"`
}

// updateLimit 更新限制
func (h *Handlers) updateLimit(c *gin.Context) {
	limitID := c.Param("id")

	var req updateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdateLimit(limitID, req.PeakPower, req.Sustained); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "限制更新成功",
	})
}

// listPolicies 列出所有策略
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

// addPolicyRequest 添加策略请求
type addPolicyRequest struct {
	ID          string    `json:"id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Mode        PowerMode `json:"mode" binding:"required"`
	MaxPower    float64   `json:"maxPower" binding:"required"`
	CPUThrottle float64   `json:"cpuThrottle"`
	GPUThrottle float64   `json:"gpuThrottle"`
	Enabled     bool      `json:"enabled"`
	AutoApply   bool      `json:"autoApply"`
}

// addPolicy 添加策略
func (h *Handlers) addPolicy(c *gin.Context) {
	var req addPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	policy := &PowerPolicy{
		ID:          req.ID,
		Name:        req.Name,
		Mode:        req.Mode,
		MaxPower:    req.MaxPower,
		CPUThrottle: req.CPUThrottle,
		GPUThrottle: req.GPUThrottle,
		Enabled:     req.Enabled,
		AutoApply:   req.AutoApply,
	}

	if err := h.manager.AddPolicy(policy); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "策略添加成功",
		"policy":  policy,
	})
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	policyID := c.Param("id")
	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// updatePolicyRequest 更新策略请求
type updatePolicyRequest struct {
	Name        string    `json:"name"`
	Mode        PowerMode `json:"mode"`
	MaxPower    float64   `json:"maxPower"`
	CPUThrottle float64   `json:"cpuThrottle"`
	GPUThrottle float64   `json:"gpuThrottle"`
	Enabled     bool      `json:"enabled"`
	AutoApply   bool      `json:"autoApply"`
}

// updatePolicy 更新策略
func (h *Handlers) updatePolicy(c *gin.Context) {
	policyID := c.Param("id")

	var req updatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	policy := &PowerPolicy{
		Name:        req.Name,
		Mode:        req.Mode,
		MaxPower:    req.MaxPower,
		CPUThrottle: req.CPUThrottle,
		GPUThrottle: req.GPUThrottle,
		Enabled:     req.Enabled,
		AutoApply:   req.AutoApply,
	}

	if err := h.manager.UpdatePolicy(policyID, policy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "策略更新成功",
	})
}

// applyPolicy 应用策略
func (h *Handlers) applyPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if err := h.manager.ApplyPolicy(policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "策略应用成功",
	})
}

// getReport 获取报表
func (h *Handlers) getReport(c *gin.Context) {
	period := c.DefaultQuery("period", "hourly")
	report := h.manager.GetReport(period)
	c.JSON(http.StatusOK, report)
}

// getTrends 获取趋势
func (h *Handlers) getTrends(c *gin.Context) {
	hours := 24
	if h, err := parseIntParam(c, "hours"); err == nil && h > 0 {
		hours = h
	}

	duration := time.Duration(hours) * time.Hour
	trends := h.manager.GetTrends(duration)
	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"total":  len(trends),
	})
}

// getAlerts 获取告警
func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// clearAlerts 清除告警
func (h *Handlers) clearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{
		"message": "告警已清除",
	})
}

// estimateCostRequest 估算电费请求
type estimateCostRequest struct {
	EnergyWh    float64 `json:"energyWh" binding:"required"`
	PricePerKWh float64 `json:"pricePerKWh" binding:"required"`
}

// estimateCost 估算电费
func (h *Handlers) estimateCost(c *gin.Context) {
	var req estimateCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	cost := h.manager.EstimateCost(req.EnergyWh, req.PricePerKWh)
	c.JSON(http.StatusOK, gin.H{
		"cost":     cost,
		"currency": "CNY",
	})
}

// parseIntParam 解析整数参数
func parseIntParam(c *gin.Context, name string) (int, error) {
	var val int
	_, err := fmt.Sscanf(c.Query(name), "%d", &val)
	return val, err
}

// UpdateReading 更新功耗读数 (供外部调用)
func (m *Manager) UpdateReading(reading *PowerReading) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reading = reading
}

// SetThresholds 设置告警阈值 (供外部调用)
func (m *Manager) SetThresholds(warning, critical float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新状态
	if m.reading != nil {
		if m.reading.TotalPower >= critical {
			m.currentState = PowerStateCritical
		} else if m.reading.TotalPower >= warning {
			m.currentState = PowerStateWarning
		} else {
			m.currentState = PowerStateNormal
		}
	}
}
