// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 配额管理 HTTP 处理器
type Handlers struct {
	manager   *Manager
	monitor   *Monitor
	cleanup   *CleanupManager
	reportGen *ReportGenerator
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	return &Handlers{
		manager:   mgr,
		monitor:   monitor,
		cleanup:   cleanup,
		reportGen: reportGen,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	// ========== v2.1.0 新增配额管理 API ==========
	quotasV2 := api.Group("/quotas")
	{
		// 用户配额
		quotasV2.GET("/users", h.listUserQuotas)
		quotasV2.POST("/users/:id", h.setUserQuota)
		quotasV2.DELETE("/users/:id", h.deleteUserQuota)

		// 组配额
		quotasV2.GET("/groups", h.listGroupQuotas)
		quotasV2.POST("/groups/:id", h.setGroupQuota)
		quotasV2.DELETE("/groups/:id", h.deleteGroupQuota)

		// 目录配额
		quotasV2.GET("/directories", h.listDirectoryQuotas)
		quotasV2.POST("/directories", h.setDirectoryQuota)
		quotasV2.DELETE("/directories/:id", h.deleteDirectoryQuota)

		// 配额预警和使用报告
		quotasV2.GET("/alerts", h.getQuotaAlerts)
		quotasV2.GET("/report", h.getQuotaReport)
	}

	// ========== 配额管理 ==========
	quotas := api.Group("/quotas")
	{
		quotas.GET("", h.listQuotas)
		quotas.POST("", h.createQuota)
		quotas.GET("/:id", h.getQuota)
		quotas.PUT("/:id", h.updateQuota)
		quotas.DELETE("/:id", h.deleteQuota)
		quotas.GET("/:id/usage", h.getQuotaUsage)
		quotas.POST("/check", h.checkQuota)
	}

	// ========== 配额使用统计 ==========
	usage := api.Group("/quota-usage")
	{
		usage.GET("", h.getAllUsage)
		usage.GET("/users/:username", h.getUserUsage)
		usage.GET("/groups/:groupname", h.getGroupUsage)
		usage.GET("/directories", h.getDirectoryUsage)
	}

	// ========== 告警管理 ==========
	alerts := api.Group("/quota-alerts")
	{
		alerts.GET("", h.getAlerts)
		alerts.GET("/history", h.getAlertHistory)
		alerts.POST("/:id/silence", h.silenceAlert)
		alerts.POST("/:id/resolve", h.resolveAlert)
		alerts.GET("/config", h.getAlertConfig)
		alerts.PUT("/config", h.setAlertConfig)
	}

	// ========== 清理策略 ==========
	policies := api.Group("/cleanup-policies")
	{
		policies.GET("", h.listPolicies)
		policies.POST("", h.createPolicy)
		policies.GET("/:id", h.getPolicy)
		policies.PUT("/:id", h.updatePolicy)
		policies.DELETE("/:id", h.deletePolicy)
		policies.POST("/:id/enable", h.enablePolicy)
		policies.POST("/:id/disable", h.disablePolicy)
		policies.POST("/:id/run", h.runPolicy)
		policies.GET("/:id/preview", h.previewPolicy) // 清理预览（dry-run）
		policies.GET("/:id/tasks", h.getPolicyTasks)
	}

	// ========== 清理任务 ==========
	tasks := api.Group("/cleanup-tasks")
	{
		tasks.GET("", h.listTasks)
		tasks.GET("/:id", h.getTask)
		tasks.POST("/auto", h.runAutoCleanup)
		tasks.GET("/stats", h.getCleanupStats)
	}

	// ========== 监控状态 ==========
	monitor := api.Group("/quota-monitor")
	{
		monitor.GET("/status", h.getMonitorStatus)
		monitor.POST("/start", h.startMonitor)
		monitor.POST("/stop", h.stopMonitor)
		monitor.GET("/trends/:quotaId", h.getTrend)
	}

	// ========== 报告生成 ==========
	reports := api.Group("/quota-reports")
	{
		reports.GET("", h.listReports)
		reports.POST("", h.generateReport)
		reports.GET("/:id", h.getReport)
		reports.GET("/:id/export", h.exportReport)
	}

	// ========== Webhook 配置 ==========
	webhook := api.Group("/quota-webhook")
	{
		webhook.GET("/config", h.getWebhookConfig)
		webhook.PUT("/config", h.setWebhookConfig)
		webhook.POST("/test", h.testWebhook)
	}

	// ========== 报告调度 ==========
	schedule := api.Group("/quota-schedule")
	{
		schedule.GET("", h.getScheduleConfig)
		schedule.POST("", h.setScheduleConfig)
		schedule.DELETE("", h.cancelSchedule)
	}
}

// ========== 通用响应 ==========

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 配额管理 API ==========

// listQuotas 列出配额
// @Summary 列出配额
// @Description 获取配额列表，支持按类型和卷名过滤
// @Tags quota
// @Accept json
// @Produce json
// @Param type query string false "配额类型 (user/group/directory)"
// @Param volume query string false "卷名"
// @Param username query string false "用户名"
// @Param groupname query string false "组名"
// @Success 200 {object} Response{data=[]Quota}
// @Router /quotas [get]
// @Security BearerAuth
func (h *Handlers) listQuotas(c *gin.Context) {
	quotaType := c.Query("type")
	volumeName := c.Query("volume")

	var quotas []*Quota
	switch quotaType {
	case "user":
		username := c.Query("username")
		if username != "" {
			quotas = h.manager.ListUserQuotas(username)
		} else {
			quotas = h.manager.ListQuotas()
		}
	case "group":
		groupName := c.Query("groupname")
		if groupName != "" {
			quotas = h.manager.ListGroupQuotas(groupName)
		} else {
			quotas = h.manager.ListQuotas()
		}
	default:
		quotas = h.manager.ListQuotas()
	}

	// 按卷过滤
	if volumeName != "" {
		filtered := make([]*Quota, 0)
		for _, q := range quotas {
			if q.VolumeName == volumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	c.JSON(http.StatusOK, Success(quotas))
}

// createQuota 创建配额
// @Summary 创建配额
// @Description 创建新的配额配置
// @Tags quota
// @Accept json
// @Produce json
// @Param quota body QuotaInput true "配额配置"
// @Success 201 {object} Response{data=Quota}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Failure 409 {object} Response
// @Router /quotas [post]
// @Security BearerAuth
func (h *Handlers) createQuota(c *gin.Context) {
	var req QuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	quota, err := h.manager.CreateQuota(req)
	if err != nil {
		switch err {
		case ErrUserNotFound, ErrGroupNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrQuotaExists:
			c.JSON(http.StatusConflict, Error(409, err.Error()))
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusCreated, Success(quota))
}

// getQuota 获取配额
// @Summary 获取配额详情
// @Description 获取指定配额的详细信息
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=Quota}
// @Failure 404 {object} Response
// @Router /quotas/{id} [get]
// @Security BearerAuth
func (h *Handlers) getQuota(c *gin.Context) {
	id := c.Param("id")
	quota, err := h.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(quota))
}

// updateQuota 更新配额
// @Summary 更新配额
// @Description 更新指定的配额配置
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Param quota body QuotaInput true "配额配置"
// @Success 200 {object} Response{data=Quota}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /quotas/{id} [put]
// @Security BearerAuth
func (h *Handlers) updateQuota(c *gin.Context) {
	id := c.Param("id")
	var req QuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	quota, err := h.manager.UpdateQuota(id, req)
	if err != nil {
		switch err {
		case ErrQuotaNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, Success(quota))
}

// deleteQuota 删除配额
// @Summary 删除配额
// @Description 删除指定的配额配置
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /quotas/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteQuota(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteQuota(id); err != nil {
		if err == ErrQuotaNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// getQuotaUsage 获取配额使用情况
// @Summary 获取配额使用情况
// @Description 获取指定配额的使用统计
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=QuotaUsage}
// @Failure 404 {object} Response
// @Router /quotas/{id}/usage [get]
// @Security BearerAuth
func (h *Handlers) getQuotaUsage(c *gin.Context) {
	id := c.Param("id")
	usage, err := h.manager.GetUsage(id)
	if err != nil {
		if err == ErrQuotaNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(usage))
}

func (h *Handlers) checkQuota(c *gin.Context) {
	var req struct {
		Username       string `json:"username" binding:"required"`
		VolumeName     string `json:"volume_name"`
		AdditionalSize uint64 `json:"additional_size"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	err := h.manager.CheckQuota(req.Username, req.VolumeName, req.AdditionalSize)
	if err != nil {
		if err == ErrQuotaExceeded {
			c.JSON(http.StatusForbidden, Error(403, "超出配额限制"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{"allowed": true}))
}

// ========== 配额使用统计 API ==========

func (h *Handlers) getAllUsage(c *gin.Context) {
	usages, err := h.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(usages))
}

func (h *Handlers) getUserUsage(c *gin.Context) {
	username := c.Param("username")
	usages, err := h.manager.GetUserUsage(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(usages))
}

func (h *Handlers) getGroupUsage(c *gin.Context) {
	groupName := c.Param("groupname")
	usages := h.manager.ListGroupQuotas(groupName)

	// 计算使用量
	result := make([]*QuotaUsage, 0, len(usages))
	for _, q := range usages {
		usage, err := h.manager.GetUsage(q.ID)
		if err == nil {
			result = append(result, usage)
		}
	}

	c.JSON(http.StatusOK, Success(result))
}

func (h *Handlers) getDirectoryUsage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		// 返回所有目录配额
		usages := h.manager.ListDirectoryQuotas()
		result := make([]*QuotaUsage, 0, len(usages))
		for _, q := range usages {
			usage, err := h.manager.GetUsage(q.ID)
			if err == nil {
				result = append(result, usage)
			}
		}
		c.JSON(http.StatusOK, Success(result))
		return
	}

	// 返回指定目录的使用情况
	quota, err := h.manager.GetDirectoryQuota(path)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	usage, err := h.manager.GetUsage(quota.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(usage))
}

// ========== 告警管理 API ==========

func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, Success(alerts))
}

func (h *Handlers) getAlertHistory(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	alerts := h.manager.GetAlertHistory(limit)
	c.JSON(http.StatusOK, Success(alerts))
}

func (h *Handlers) silenceAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.SilenceAlert(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getAlertConfig(c *gin.Context) {
	config := h.manager.GetAlertConfig()
	c.JSON(http.StatusOK, Success(config))
}

func (h *Handlers) setAlertConfig(c *gin.Context) {
	var config AlertConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	h.manager.SetAlertConfig(config)
	c.JSON(http.StatusOK, Success(nil))
}

// ========== 清理策略 API ==========

func (h *Handlers) listPolicies(c *gin.Context) {
	volumeName := c.Query("volume")
	policies := h.cleanup.ListPolicies()

	if volumeName != "" {
		filtered := make([]*CleanupPolicy, 0)
		for _, p := range policies {
			if p.VolumeName == volumeName {
				filtered = append(filtered, p)
			}
		}
		policies = filtered
	}

	c.JSON(http.StatusOK, Success(policies))
}

func (h *Handlers) createPolicy(c *gin.Context) {
	var req CleanupPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	policy, err := h.cleanup.CreatePolicy(req)
	if err != nil {
		switch err {
		case ErrVolumeNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		default:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		}
		return
	}

	c.JSON(http.StatusCreated, Success(policy))
}

func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.cleanup.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(policy))
}

func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req CleanupPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	policy, err := h.cleanup.UpdatePolicy(id, req)
	if err != nil {
		switch err {
		case ErrCleanupPolicyNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		default:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, Success(policy))
}

func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.cleanup.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) enablePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.cleanup.EnablePolicy(id, true); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) disablePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.cleanup.EnablePolicy(id, false); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) runPolicy(c *gin.Context) {
	id := c.Param("id")
	task, err := h.cleanup.RunPolicy(id)
	if err != nil {
		if err == ErrCleanupPolicyNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(task))
}

func (h *Handlers) previewPolicy(c *gin.Context) {
	id := c.Param("id")
	preview, err := h.cleanup.PreviewPolicy(id, 100)
	if err != nil {
		if err == ErrCleanupPolicyNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(preview))
}

func (h *Handlers) getPolicyTasks(c *gin.Context) {
	id := c.Param("id")
	tasks := h.cleanup.ListTasks(0)

	// 过滤出该策略的任务
	filtered := make([]*CleanupTask, 0)
	for _, t := range tasks {
		if t.PolicyID == id {
			filtered = append(filtered, t)
		}
	}

	c.JSON(http.StatusOK, Success(filtered))
}

// ========== 清理任务 API ==========

func (h *Handlers) listTasks(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	tasks := h.cleanup.ListTasks(limit)
	c.JSON(http.StatusOK, Success(tasks))
}

func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.cleanup.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(task))
}

func (h *Handlers) runAutoCleanup(c *gin.Context) {
	tasks, err := h.cleanup.RunAutoCleanup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(tasks))
}

func (h *Handlers) getCleanupStats(c *gin.Context) {
	stats := h.cleanup.GetCleanupStats()
	c.JSON(http.StatusOK, Success(stats))
}

// ========== 监控状态 API ==========

func (h *Handlers) getMonitorStatus(c *gin.Context) {
	status := h.monitor.GetMonitorStatus()
	c.JSON(http.StatusOK, Success(status))
}

func (h *Handlers) startMonitor(c *gin.Context) {
	h.monitor.Start()
	c.JSON(http.StatusOK, Success(gin.H{"running": true}))
}

func (h *Handlers) stopMonitor(c *gin.Context) {
	h.monitor.Stop()
	c.JSON(http.StatusOK, Success(gin.H{"running": false}))
}

func (h *Handlers) getTrend(c *gin.Context) {
	quotaID := c.Param("quotaId")
	duration := 24 * time.Hour
	if d := c.Query("duration"); d != "" {
		if h, err := time.ParseDuration(d); err == nil {
			duration = h
		}
	}

	trend := h.monitor.GetTrend(quotaID, duration)
	c.JSON(http.StatusOK, Success(trend))
}

// ========== 报告生成 API ==========

var reports = make(map[string]*Report)

func (h *Handlers) listReports(c *gin.Context) {
	result := make([]*Report, 0, len(reports))
	for _, r := range reports {
		result = append(result, r)
	}
	c.JSON(http.StatusOK, Success(result))
}

func (h *Handlers) generateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	report, err := h.reportGen.GenerateReport(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	reports[report.ID] = report
	c.JSON(http.StatusOK, Success(report))
}

func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, exists := reports[id]
	if !exists {
		c.JSON(http.StatusNotFound, Error(404, "报告不存在"))
		return
	}
	c.JSON(http.StatusOK, Success(report))
}

func (h *Handlers) exportReport(c *gin.Context) {
	id := c.Param("id")
	report, exists := reports[id]
	if !exists {
		c.JSON(http.StatusNotFound, Error(404, "报告不存在"))
		return
	}

	format := c.DefaultQuery("format", string(report.Format))
	report.Format = ReportFormat(format)

	// 设置响应头
	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=quota-report.csv")
	case "html":
		c.Header("Content-Type", "text/html")
		c.Header("Content-Disposition", "attachment; filename=quota-report.html")
	default:
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=quota-report.json")
	}

	// 直接输出
	switch format {
	case "csv":
		data, _ := json.Marshal(report.Summary)
		c.String(http.StatusOK, string(data))
	case "html":
		_ = h.reportGen.ExportReport(report, "/tmp/report.html")
		c.File("/tmp/report.html")
	default:
		c.JSON(http.StatusOK, report)
	}
}

// ========== Webhook 配置 API ==========

// WebhookConfigRequest Webhook 配置请求
type WebhookConfigRequest struct {
	Enabled     bool     `json:"enabled"`
	WebhookURLs []string `json:"webhook_urls"`
	AlertLevels []string `json:"alert_levels"` // warning, critical
	TimeoutSecs int      `json:"timeout_secs"`
	RetryCount  int      `json:"retry_count"`
}

func (h *Handlers) getWebhookConfig(c *gin.Context) {
	config := map[string]interface{}{
		"enabled":          h.monitor.config.NotifyWebhook,
		"webhook_url":      h.monitor.config.WebhookURL,
		"check_interval":   h.monitor.config.CheckInterval.String(),
		"silence_duration": h.monitor.config.SilenceDuration.String(),
	}
	c.JSON(http.StatusOK, Success(config))
}

func (h *Handlers) setWebhookConfig(c *gin.Context) {
	var req WebhookConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	webhookURL := h.monitor.config.WebhookURL
	if len(req.WebhookURLs) > 0 {
		webhookURL = req.WebhookURLs[0]
	}

	h.monitor.UpdateConfig(AlertConfig{
		Enabled:            req.Enabled,
		SoftLimitThreshold: h.monitor.config.SoftLimitThreshold,
		HardLimitThreshold: h.monitor.config.HardLimitThreshold,
		CheckInterval:      h.monitor.config.CheckInterval,
		NotifyEmail:        h.monitor.config.NotifyEmail,
		NotifyWebhook:      req.Enabled,
		WebhookURL:         webhookURL,
		SilenceDuration:    h.monitor.config.SilenceDuration,
	})

	c.JSON(http.StatusOK, Success(map[string]string{"status": "updated"}))
}

func (h *Handlers) testWebhook(c *gin.Context) {
	// 创建测试告警
	testAlert := &Alert{
		ID:           "test-alert",
		QuotaID:      "test",
		Type:         "warning",
		Status:       "active",
		TargetID:     "test",
		TargetName:   "Test Quota",
		VolumeName:   "test-volume",
		Path:         "/test",
		UsedBytes:    1000000,
		LimitBytes:   2000000,
		UsagePercent: 50.0,
		Message:      "NAS-OS 配额 Webhook 测试",
		CreatedAt:    time.Now(),
	}

	// 临时设置 webhook URL
	url := c.Query("url")
	if url != "" {
		originalURL := h.monitor.config.WebhookURL
		h.monitor.config.WebhookURL = url
		h.monitor.sendWebhook(testAlert)
		h.monitor.config.WebhookURL = originalURL
	} else {
		if h.monitor.config.WebhookURL == "" {
			c.JSON(http.StatusBadRequest, Error(400, "请提供 webhook URL"))
			return
		}
		h.monitor.sendWebhook(testAlert)
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "sent"}))
}

// ========== 报告调度 API ==========

// ScheduleConfigRequest 报告调度配置请求
type ScheduleConfigRequest struct {
	ReportRequest
	Schedule   string `json:"schedule"`    // cron 表达式（秒级）
	OutputPath string `json:"output_path"` // 导出路径
	Enabled    bool   `json:"enabled"`
}

func (h *Handlers) getScheduleConfig(c *gin.Context) {
	// 返回当前调度配置状态
	config := map[string]interface{}{
		"enabled": h.reportGen.scheduledID != 0,
		"status":  "active",
	}
	c.JSON(http.StatusOK, Success(config))
}

func (h *Handlers) setScheduleConfig(c *gin.Context) {
	var req ScheduleConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if !req.Enabled {
		h.reportGen.Stop()
		c.JSON(http.StatusOK, Success(map[string]string{"status": "cancelled"}))
		return
	}

	err := h.reportGen.ScheduleReport(req.ReportRequest, req.Schedule, req.OutputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "scheduled"}))
}

func (h *Handlers) cancelSchedule(c *gin.Context) {
	h.reportGen.Stop()
	c.JSON(http.StatusOK, Success(map[string]string{"status": "cancelled"}))
}

// ========== v2.1.0 新增 API 处理函数 ==========

// UserQuotaInput 用户配额输入
type UserQuotaInput struct {
	VolumeName string `json:"volume_name" binding:"required"`
	HardLimit  uint64 `json:"hard_limit" binding:"required"`
	SoftLimit  uint64 `json:"soft_limit"`
}

// GroupQuotaInput 组配额输入
type GroupQuotaInput struct {
	VolumeName string `json:"volume_name" binding:"required"`
	HardLimit  uint64 `json:"hard_limit" binding:"required"`
	SoftLimit  uint64 `json:"soft_limit"`
}

// DirectoryQuotaInput 目录配额输入
type DirectoryQuotaInput struct {
	Path       string `json:"path" binding:"required"`
	VolumeName string `json:"volume_name"`
	HardLimit  uint64 `json:"hard_limit" binding:"required"`
	SoftLimit  uint64 `json:"soft_limit"`
}

// UserQuotaResponse 用户配额响应
type UserQuotaResponse struct {
	Username     string      `json:"username"`
	VolumeName   string      `json:"volume_name"`
	HardLimit    uint64      `json:"hard_limit"`
	SoftLimit    uint64      `json:"soft_limit"`
	UsedBytes    uint64      `json:"used_bytes"`
	UsagePercent float64     `json:"usage_percent"`
	IsOverSoft   bool        `json:"is_over_soft"`
	IsOverHard   bool        `json:"is_over_hard"`
	Quota        *Quota      `json:"quota,omitempty"`
	Usage        *QuotaUsage `json:"usage,omitempty"`
}

// GroupQuotaResponse 组配额响应
type GroupQuotaResponse struct {
	GroupName    string      `json:"group_name"`
	VolumeName   string      `json:"volume_name"`
	HardLimit    uint64      `json:"hard_limit"`
	SoftLimit    uint64      `json:"soft_limit"`
	UsedBytes    uint64      `json:"used_bytes"`
	UsagePercent float64     `json:"usage_percent"`
	IsOverSoft   bool        `json:"is_over_soft"`
	IsOverHard   bool        `json:"is_over_hard"`
	Quota        *Quota      `json:"quota,omitempty"`
	Usage        *QuotaUsage `json:"usage,omitempty"`
}

// DirectoryQuotaResponse 目录配额响应
type DirectoryQuotaResponse struct {
	Path         string      `json:"path"`
	VolumeName   string      `json:"volume_name"`
	HardLimit    uint64      `json:"hard_limit"`
	SoftLimit    uint64      `json:"soft_limit"`
	UsedBytes    uint64      `json:"used_bytes"`
	UsagePercent float64     `json:"usage_percent"`
	IsOverSoft   bool        `json:"is_over_soft"`
	IsOverHard   bool        `json:"is_over_hard"`
	Quota        *Quota      `json:"quota,omitempty"`
	Usage        *QuotaUsage `json:"usage,omitempty"`
}

// listUserQuotas 列出所有用户配额
func (h *Handlers) listUserQuotas(c *gin.Context) {
	username := c.Query("username")
	volumeName := c.Query("volume")

	var quotas []*Quota
	if username != "" {
		quotas = h.manager.ListUserQuotas(username)
	} else {
		quotas = h.manager.ListQuotas()
		// 过滤出用户配额
		userQuotas := make([]*Quota, 0)
		for _, q := range quotas {
			if q.Type == QuotaTypeUser {
				userQuotas = append(userQuotas, q)
			}
		}
		quotas = userQuotas
	}

	// 按卷过滤
	if volumeName != "" {
		filtered := make([]*Quota, 0)
		for _, q := range quotas {
			if q.VolumeName == volumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	// 构建响应
	responses := make([]*UserQuotaResponse, 0, len(quotas))
	for _, q := range quotas {
		resp := &UserQuotaResponse{
			Username:   q.TargetID,
			VolumeName: q.VolumeName,
			HardLimit:  q.HardLimit,
			SoftLimit:  q.SoftLimit,
			Quota:      q,
		}

		// 获取使用情况
		usage, err := h.manager.GetUsage(q.ID)
		if err == nil {
			resp.UsedBytes = usage.UsedBytes
			resp.UsagePercent = usage.UsagePercent
			resp.IsOverSoft = usage.IsOverSoft
			resp.IsOverHard = usage.IsOverHard
			resp.Usage = usage
		}

		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, Success(responses))
}

// setUserQuota 设置用户配额
func (h *Handlers) setUserQuota(c *gin.Context) {
	username := c.Param("id")
	var req UserQuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 检查用户是否存在
	if h.manager.userProvider != nil && !h.manager.userProvider.UserExists(username) {
		c.JSON(http.StatusNotFound, Error(404, "用户不存在"))
		return
	}

	// 查找现有配额
	existingQuotas := h.manager.ListUserQuotas(username)
	var existingQuota *Quota
	for _, q := range existingQuotas {
		if q.VolumeName == req.VolumeName {
			existingQuota = q
			break
		}
	}

	var quota *Quota
	var err error

	if existingQuota != nil {
		// 更新现有配额
		quota, err = h.manager.UpdateQuota(existingQuota.ID, QuotaInput{
			Type:       QuotaTypeUser,
			TargetID:   username,
			VolumeName: req.VolumeName,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	} else {
		// 创建新配额
		quota, err = h.manager.CreateQuota(QuotaInput{
			Type:       QuotaTypeUser,
			TargetID:   username,
			VolumeName: req.VolumeName,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	}

	if err != nil {
		switch err {
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, Success(quota))
}

// deleteUserQuota 删除用户配额
func (h *Handlers) deleteUserQuota(c *gin.Context) {
	username := c.Param("id")
	volumeName := c.Query("volume")

	quotas := h.manager.ListUserQuotas(username)
	if len(quotas) == 0 {
		c.JSON(http.StatusNotFound, Error(404, "未找到用户配额"))
		return
	}

	deleted := false
	for _, q := range quotas {
		if volumeName == "" || q.VolumeName == volumeName {
			if err := h.manager.DeleteQuota(q.ID); err == nil {
				deleted = true
			}
		}
	}

	if !deleted {
		c.JSON(http.StatusNotFound, Error(404, "未找到匹配的配额"))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

// listGroupQuotas 列出所有组配额
func (h *Handlers) listGroupQuotas(c *gin.Context) {
	groupName := c.Query("groupname")
	volumeName := c.Query("volume")

	var quotas []*Quota
	if groupName != "" {
		quotas = h.manager.ListGroupQuotas(groupName)
	} else {
		quotas = h.manager.ListQuotas()
		// 过滤出组配额
		groupQuotas := make([]*Quota, 0)
		for _, q := range quotas {
			if q.Type == QuotaTypeGroup {
				groupQuotas = append(groupQuotas, q)
			}
		}
		quotas = groupQuotas
	}

	// 按卷过滤
	if volumeName != "" {
		filtered := make([]*Quota, 0)
		for _, q := range quotas {
			if q.VolumeName == volumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	// 构建响应
	responses := make([]*GroupQuotaResponse, 0, len(quotas))
	for _, q := range quotas {
		resp := &GroupQuotaResponse{
			GroupName:  q.TargetID,
			VolumeName: q.VolumeName,
			HardLimit:  q.HardLimit,
			SoftLimit:  q.SoftLimit,
			Quota:      q,
		}

		// 获取使用情况
		usage, err := h.manager.GetUsage(q.ID)
		if err == nil {
			resp.UsedBytes = usage.UsedBytes
			resp.UsagePercent = usage.UsagePercent
			resp.IsOverSoft = usage.IsOverSoft
			resp.IsOverHard = usage.IsOverHard
			resp.Usage = usage
		}

		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, Success(responses))
}

// setGroupQuota 设置组配额
func (h *Handlers) setGroupQuota(c *gin.Context) {
	groupName := c.Param("id")
	var req GroupQuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 检查组是否存在
	if h.manager.userProvider != nil && !h.manager.userProvider.GroupExists(groupName) {
		c.JSON(http.StatusNotFound, Error(404, "用户组不存在"))
		return
	}

	// 查找现有配额
	existingQuotas := h.manager.ListGroupQuotas(groupName)
	var existingQuota *Quota
	for _, q := range existingQuotas {
		if q.VolumeName == req.VolumeName {
			existingQuota = q
			break
		}
	}

	var quota *Quota
	var err error

	if existingQuota != nil {
		// 更新现有配额
		quota, err = h.manager.UpdateQuota(existingQuota.ID, QuotaInput{
			Type:       QuotaTypeGroup,
			TargetID:   groupName,
			VolumeName: req.VolumeName,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	} else {
		// 创建新配额
		quota, err = h.manager.CreateQuota(QuotaInput{
			Type:       QuotaTypeGroup,
			TargetID:   groupName,
			VolumeName: req.VolumeName,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	}

	if err != nil {
		switch err {
		case ErrGroupNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, Success(quota))
}

// deleteGroupQuota 删除组配额
func (h *Handlers) deleteGroupQuota(c *gin.Context) {
	groupName := c.Param("id")
	volumeName := c.Query("volume")

	quotas := h.manager.ListGroupQuotas(groupName)
	if len(quotas) == 0 {
		c.JSON(http.StatusNotFound, Error(404, "未找到组配额"))
		return
	}

	deleted := false
	for _, q := range quotas {
		if volumeName == "" || q.VolumeName == volumeName {
			if err := h.manager.DeleteQuota(q.ID); err == nil {
				deleted = true
			}
		}
	}

	if !deleted {
		c.JSON(http.StatusNotFound, Error(404, "未找到匹配的配额"))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

// listDirectoryQuotas 列出所有目录配额
func (h *Handlers) listDirectoryQuotas(c *gin.Context) {
	volumeName := c.Query("volume")

	quotas := h.manager.ListDirectoryQuotas()

	// 按卷过滤
	if volumeName != "" {
		filtered := make([]*Quota, 0)
		for _, q := range quotas {
			if q.VolumeName == volumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	// 构建响应
	responses := make([]*DirectoryQuotaResponse, 0, len(quotas))
	for _, q := range quotas {
		resp := &DirectoryQuotaResponse{
			Path:       q.Path,
			VolumeName: q.VolumeName,
			HardLimit:  q.HardLimit,
			SoftLimit:  q.SoftLimit,
			Quota:      q,
		}

		// 获取使用情况
		usage, err := h.manager.GetUsage(q.ID)
		if err == nil {
			resp.UsedBytes = usage.UsedBytes
			resp.UsagePercent = usage.UsagePercent
			resp.IsOverSoft = usage.IsOverSoft
			resp.IsOverHard = usage.IsOverHard
			resp.Usage = usage
		}

		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, Success(responses))
}

// setDirectoryQuota 设置目录配额
func (h *Handlers) setDirectoryQuota(c *gin.Context) {
	var req DirectoryQuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 查找现有配额
	existingQuota, _ := h.manager.GetDirectoryQuota(req.Path)

	var quota *Quota
	var err error

	if existingQuota != nil {
		// 更新现有配额
		quota, err = h.manager.UpdateQuota(existingQuota.ID, QuotaInput{
			Type:       QuotaTypeDirectory,
			TargetID:   req.Path,
			VolumeName: req.VolumeName,
			Path:       req.Path,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	} else {
		// 创建新配额
		quota, err = h.manager.CreateQuota(QuotaInput{
			Type:       QuotaTypeDirectory,
			TargetID:   req.Path,
			VolumeName: req.VolumeName,
			Path:       req.Path,
			HardLimit:  req.HardLimit,
			SoftLimit:  req.SoftLimit,
		})
	}

	if err != nil {
		switch err {
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, Success(quota))
}

// deleteDirectoryQuota 删除目录配额
func (h *Handlers) deleteDirectoryQuota(c *gin.Context) {
	quotaID := c.Param("id")

	if err := h.manager.DeleteQuota(quotaID); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

// getQuotaAlerts 获取配额预警列表
func (h *Handlers) getQuotaAlerts(c *gin.Context) {
	alertType := c.Query("type")
	status := c.Query("status")

	alerts := h.manager.GetAlerts()

	// 按类型过滤
	if alertType != "" {
		filtered := make([]*Alert, 0)
		for _, a := range alerts {
			if string(a.Type) == alertType {
				filtered = append(filtered, a)
			}
		}
		alerts = filtered
	}

	// 按状态过滤
	if status != "" {
		filtered := make([]*Alert, 0)
		for _, a := range alerts {
			if string(a.Status) == status {
				filtered = append(filtered, a)
			}
		}
		alerts = filtered
	}

	// 如果没有活跃告警，检查是否需要返回历史告警
	includeHistory := c.Query("history") == "true"
	if includeHistory && len(alerts) == 0 {
		limit := 100
		if l := c.Query("limit"); l != "" {
			_, _ = fmt.Sscanf(l, "%d", &limit)
		}
		alerts = h.manager.GetAlertHistory(limit)
	}

	c.JSON(http.StatusOK, Success(alerts))
}

// getQuotaReport 获取配额使用报告
func (h *Handlers) getQuotaReport(c *gin.Context) {
	reportType := c.DefaultQuery("type", "summary")
	volumeName := c.Query("volume")
	userID := c.Query("user")
	groupID := c.Query("group")

	req := ReportRequest{
		Type:       ReportType(reportType),
		Format:     ReportFormatJSON,
		VolumeName: volumeName,
		UserID:     userID,
		GroupID:    groupID,
	}

	// 解析时间范围
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			req.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			req.EndTime = &t
		}
	}

	report, err := h.reportGen.GenerateReport(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	// 检查是否需要导出
	format := c.Query("format")
	if format != "" && format != "json" {
		report.Format = ReportFormat(format)
		switch format {
		case "csv":
			c.Header("Content-Type", "text/csv")
			c.Header("Content-Disposition", "attachment; filename=quota-report.csv")
		case "html":
			c.Header("Content-Type", "text/html")
			c.Header("Content-Disposition", "attachment; filename=quota-report.html")
		}
	}

	c.JSON(http.StatusOK, Success(report))
}
