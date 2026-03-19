// Package budget 提供预算管理 HTTP handlers
package budget

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	budgets := r.Group("/budgets")
	{
		// 预算 CRUD
		budgets.POST("", h.CreateBudget)
		budgets.GET("", h.ListBudgets)
		budgets.GET("/:id", h.GetBudget)
		budgets.PUT("/:id", h.UpdateBudget)
		budgets.DELETE("/:id", h.DeleteBudget)

		// 预算重置
		budgets.POST("/:id/reset", h.ResetBudget)

		// 使用记录
		budgets.POST("/:id/usage", h.RecordUsage)
		budgets.GET("/:id/usage", h.GetUsageHistory)
		budgets.GET("/:id/usage/stats", h.GetUsageStats)

		// 预警
		budgets.GET("/:id/alerts", h.GetBudgetAlerts)
		budgets.GET("/:id/alerts/history", h.GetAlertHistory)
	}

	// 全局预警
	alerts := r.Group("/alerts")
	{
		alerts.GET("", h.GetActiveAlerts)
		alerts.POST("/:id/acknowledge", h.AcknowledgeAlert)
		alerts.POST("/:id/resolve", h.ResolveAlert)
	}

	// 报告和统计
	reports := r.Group("/reports")
	{
		reports.POST("/budget", h.GenerateReport)
	}

	// 统计
	r.GET("/stats", h.GetStats)
}

// ========== 预算 CRUD ==========

// CreateBudget 创建预算
// @Summary 创建预算
// @Tags budget
// @Accept json
// @Produce json
// @Param input body Input true "预算输入"
// @Success 201 {object} Budget
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /budgets [post]
func (h *Handlers) CreateBudget(c *gin.Context) {
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	createdBy := getStringFromContext(c, "user_id")

	budget, err := h.manager.CreateBudget(input, createdBy)
	if err != nil {
		if err == ErrBudgetExists {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, budget)
}

// ListBudgets 列出预算
// @Summary 列出预算
// @Tags budget
// @Produce json
// @Param type query []string false "预算类型"
// @Param scope query []string false "预算范围"
// @Param status query []string false "预算状态"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} ListResponse
// @Router /budgets [get]
func (h *Handlers) ListBudgets(c *gin.Context) {
	query := BudgetQuery{
		Types:     parseTypes(c.QueryArray("type")),
		Scopes:    parseScopes(c.QueryArray("scope")),
		Statuses:  parseBudgetStatuses(c.QueryArray("status")),
		Page:      parseInt(c.Query("page"), 1),
		PageSize:  parseInt(c.Query("page_size"), 20),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	if query.SortBy == "" {
		query.SortBy = "created_at"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	budgets, total, err := h.manager.ListBudgets(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:     budgets,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// GetBudget 获取预算
// @Summary 获取预算详情
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Success 200 {object} Budget
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id} [get]
func (h *Handlers) GetBudget(c *gin.Context) {
	id := c.Param("id")

	budget, err := h.manager.GetBudget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, budget)
}

// UpdateBudget 更新预算
// @Summary 更新预算
// @Tags budget
// @Accept json
// @Produce json
// @Param id path string true "预算ID"
// @Param input body Input true "预算输入"
// @Success 200 {object} Budget
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id} [put]
func (h *Handlers) UpdateBudget(c *gin.Context) {
	id := c.Param("id")

	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	budget, err := h.manager.UpdateBudget(id, input)
	if err != nil {
		if err == ErrBudgetNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, budget)
}

// DeleteBudget 删除预算
// @Summary 删除预算
// @Tags budget
// @Param id path string true "预算ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id} [delete]
func (h *Handlers) DeleteBudget(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteBudget(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ResetBudget 重置预算
// @Summary 重置预算
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Success 200 {object} Budget
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id}/reset [post]
func (h *Handlers) ResetBudget(c *gin.Context) {
	id := c.Param("id")

	budget, err := h.manager.ResetBudget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, budget)
}

// ========== 使用记录 ==========

// RecordUsage 记录使用
// @Summary 记录预算使用
// @Tags budget
// @Accept json
// @Produce json
// @Param id path string true "预算ID"
// @Param input body UsageInput true "使用输入"
// @Success 201 {object} BudgetUsage
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id}/usage [post]
func (h *Handlers) RecordUsage(c *gin.Context) {
	id := c.Param("id")

	var input UsageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	input.BudgetID = id

	usage, err := h.manager.RecordUsage(input)
	if err != nil {
		if err == ErrBudgetNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, usage)
}

// GetUsageHistory 获取使用历史
// @Summary 获取使用历史
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} ListResponse
// @Router /budgets/{id}/usage [get]
func (h *Handlers) GetUsageHistory(c *gin.Context) {
	id := c.Param("id")

	query := UsageQuery{
		BudgetID: id,
		Page:     parseInt(c.Query("page"), 1),
		PageSize: parseInt(c.Query("page_size"), 20),
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			query.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			query.EndTime = &t
		}
	}

	usages, total, err := h.manager.GetUsageHistory(id, query)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:     usages,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// GetUsageStats 获取使用统计
// @Summary 获取使用统计
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Success 200 {object} UsageStatsResult
// @Failure 404 {object} ErrorResponse
// @Router /budgets/{id}/usage/stats [get]
func (h *Handlers) GetUsageStats(c *gin.Context) {
	id := c.Param("id")

	now := time.Now()
	startTime := now.AddDate(0, -1, 0) // 默认最近一个月
	endTime := now

	if st := c.Query("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = t
		}
	}

	stats, err := h.manager.GetUsageStats(id, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ========== 预警 ==========

// GetBudgetAlerts 获取预算预警
// @Summary 获取预算预警
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Success 200 {array} BudgetAlert
// @Router /budgets/{id}/alerts [get]
func (h *Handlers) GetBudgetAlerts(c *gin.Context) {
	id := c.Param("id")

	alerts, _, err := h.manager.GetActiveAlerts(AlertQuery{BudgetIDs: []string{id}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, alerts)
}

// GetAlertHistory 获取预警历史
// @Summary 获取预警历史
// @Tags budget
// @Produce json
// @Param id path string true "预算ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} ListResponse
// @Router /budgets/{id}/alerts/history [get]
func (h *Handlers) GetAlertHistory(c *gin.Context) {
	id := c.Param("id")

	query := AlertQuery{
		Page:     parseInt(c.Query("page"), 1),
		PageSize: parseInt(c.Query("page_size"), 20),
	}

	alerts, total, err := h.manager.GetAlertHistory(id, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:     alerts,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// GetActiveAlerts 获取所有活跃预警
// @Summary 获取所有活跃预警
// @Tags alerts
// @Produce json
// @Param level query []string false "预警级别"
// @Param status query []string false "预警状态"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} ListResponse
// @Router /alerts [get]
func (h *Handlers) GetActiveAlerts(c *gin.Context) {
	query := AlertQuery{
		Levels:   parseLevels(c.QueryArray("level")),
		Statuses: parseAlertStatuses(c.QueryArray("status")),
		Page:     parseInt(c.Query("page"), 1),
		PageSize: parseInt(c.Query("page_size"), 20),
	}

	alerts, total, err := h.manager.GetActiveAlerts(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:     alerts,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
}

// AcknowledgeAlert 确认预警
// @Summary 确认预警
// @Tags alerts
// @Param id path string true "预警ID"
// @Success 200 {object} BudgetAlert
// @Failure 404 {object} ErrorResponse
// @Router /alerts/{id}/acknowledge [post]
func (h *Handlers) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	acknowledgedBy := getStringFromContext(c, "user_id")

	if err := h.manager.AcknowledgeAlert(id, acknowledgedBy); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "预警已确认"})
}

// ResolveAlert 解决预警
// @Summary 解决预警
// @Tags alerts
// @Param id path string true "预警ID"
// @Success 200 {object} BudgetAlert
// @Failure 404 {object} ErrorResponse
// @Router /alerts/{id}/resolve [post]
func (h *Handlers) ResolveAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "预警已解决"})
}

// ========== 报告和统计 ==========

// GenerateReport 生成报告
// @Summary 生成预算报告
// @Tags reports
// @Accept json
// @Produce json
// @Param request body ReportRequest true "报告请求"
// @Success 200 {object} BudgetReport
// @Router /reports/budget [post]
func (h *Handlers) GenerateReport(c *gin.Context) {
	var request ReportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	report, err := h.manager.GenerateReport(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetStats 获取统计
// @Summary 获取预算统计
// @Tags budget
// @Produce json
// @Success 200 {object} BudgetStats
// @Router /stats [get]
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ========== 辅助类型 ==========

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// ListResponse 列表响应
type ListResponse struct {
	Data     interface{} `json:"data"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ========== 辅助函数 ==========

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseTypes(types []string) []Type {
	var result []Type
	for _, t := range types {
		result = append(result, Type(t))
	}
	return result
}

func parseScopes(scopes []string) []Scope {
	var result []Scope
	for _, s := range scopes {
		result = append(result, Scope(s))
	}
	return result
}

func parseBudgetStatuses(statuses []string) []BudgetStatus {
	var result []BudgetStatus
	for _, s := range statuses {
		result = append(result, BudgetStatus(s))
	}
	return result
}

func parseLevels(levels []string) []Level {
	var result []Level
	for _, l := range levels {
		result = append(result, Level(l))
	}
	return result
}

func parseStatuses(statuses []string) []Status {
	var result []Status
	for _, s := range statuses {
		result = append(result, Status(s))
	}
	return result
}

// parseAlertStatuses 解析警报状态
func parseAlertStatuses(statuses []string) []AlertStatus {
	var result []AlertStatus
	for _, s := range statuses {
		result = append(result, AlertStatus(s))
	}
	return result
}

func getStringFromContext(c *gin.Context, key string) string {
	if val, exists := c.Get(key); exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
