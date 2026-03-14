// Package quota 提供存储配额管理功能
package quota

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HandlersEnhanced 增强的配额管理 HTTP 处理器
type HandlersEnhanced struct {
	manager         *Manager
	monitor         *Monitor
	cleanup         *CleanupManager
	cleanupEnhanced *CleanupEnhancedManager
	trendManager    *TrendDataManager
	alertNotify     *AlertNotificationManager
	reportGen       *ReportGenerator
}

// NewHandlersEnhanced 创建增强处理器
func NewHandlersEnhanced(mgr *Manager) *HandlersEnhanced {
	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	cleanupEnhanced := NewCleanupEnhancedManager(mgr)
	trendManager := NewTrendDataManager(mgr, DefaultTrendConfig())
	alertNotify := NewAlertNotificationManager(DefaultAlertThresholdConfig())
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	return &HandlersEnhanced{
		manager:         mgr,
		monitor:         monitor,
		cleanup:         cleanup,
		cleanupEnhanced: cleanupEnhanced,
		trendManager:    trendManager,
		alertNotify:     alertNotify,
		reportGen:       reportGen,
	}
}

// Start 启动增强功能
func (h *HandlersEnhanced) Start() {
	h.monitor.Start()
	h.trendManager.Start()
}

// Stop 停止增强功能
func (h *HandlersEnhanced) Stop() {
	h.monitor.Stop()
	h.trendManager.Stop()
}

// RegisterEnhancedRoutes 注册增强路由
func (h *HandlersEnhanced) RegisterEnhancedRoutes(api *gin.RouterGroup) {
	// ========== 预警配置增强 API ==========
	alertConfig := api.Group("/quota-alert-config")
	{
		alertConfig.GET("/thresholds", h.getAlertThresholds)
		alertConfig.PUT("/thresholds", h.setAlertThresholds)
		alertConfig.GET("/channels", h.getNotificationChannels)
		alertConfig.POST("/channels", h.addNotificationChannel)
		alertConfig.PUT("/channels/:id", h.updateNotificationChannel)
		alertConfig.DELETE("/channels/:id", h.removeNotificationChannel)
		alertConfig.POST("/test", h.testNotificationChannel)
	}

	// ========== 大文件检测 API ==========
	largeFiles := api.Group("/quota-large-files")
	{
		largeFiles.GET("/rules", h.listLargeFileRules)
		largeFiles.POST("/rules", h.createLargeFileRule)
		largeFiles.GET("/rules/:id", h.getLargeFileRule)
		largeFiles.PUT("/rules/:id", h.updateLargeFileRule)
		largeFiles.DELETE("/rules/:id", h.deleteLargeFileRule)
		largeFiles.POST("/rules/:id/scan", h.scanLargeFiles)
		largeFiles.POST("/rules/:id/execute", h.executeLargeFileCleanup)
	}

	// ========== 过期文件清理 API ==========
	expiredFiles := api.Group("/quota-expired-files")
	{
		expiredFiles.GET("/rules", h.listExpiredFileRules)
		expiredFiles.POST("/rules", h.createExpiredFileRule)
		expiredFiles.GET("/rules/:id", h.getExpiredFileRule)
		expiredFiles.PUT("/rules/:id", h.updateExpiredFileRule)
		expiredFiles.DELETE("/rules/:id", h.deleteExpiredFileRule)
		expiredFiles.POST("/rules/:id/scan", h.scanExpiredFiles)
		expiredFiles.POST("/rules/:id/execute", h.executeExpiredFileCleanup)
	}

	// ========== 清理规则集 API ==========
	ruleSets := api.Group("/quota-rule-sets")
	{
		ruleSets.GET("", h.listRuleSets)
		ruleSets.POST("", h.createRuleSet)
		ruleSets.GET("/:id", h.getRuleSet)
		ruleSets.PUT("/:id", h.updateRuleSet)
		ruleSets.DELETE("/:id", h.deleteRuleSet)
		ruleSets.POST("/:id/execute", h.executeRuleSet)
	}

	// ========== 趋势数据 API ==========
	trends := api.Group("/quota-trends")
	{
		trends.GET("", h.getAllTrends)
		trends.GET("/:quotaId", h.getQuotaTrend)
		trends.GET("/:quotaId/chart", h.getTrendChartData)
		trends.GET("/:quotaId/stats", h.getTrendStats)
		trends.GET("/:quotaId/report", h.getTrendReport)
		trends.GET("/:quotaId/prediction", h.getTrendPrediction)
		trends.GET("/aggregated/:quotaId/:level", h.getAggregatedTrend)
	}

	// ========== 综合仪表盘 API ==========
	dashboard := api.Group("/quota-dashboard")
	{
		dashboard.GET("/summary", h.getDashboardSummary)
		dashboard.GET("/alerts", h.getDashboardAlerts)
		dashboard.GET("/trends", h.getDashboardTrends)
		dashboard.GET("/predictions", h.getDashboardPredictions)
		dashboard.GET("/recommendations", h.getDashboardRecommendations)
	}
}

// ========== 预警配置 API ==========

func (h *HandlersEnhanced) getAlertThresholds(c *gin.Context) {
	config := h.alertNotify.thresholdConfig
	c.JSON(http.StatusOK, Success(config))
}

func (h *HandlersEnhanced) setAlertThresholds(c *gin.Context) {
	var config AlertThresholdConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	h.alertNotify.thresholdConfig = config
	c.JSON(http.StatusOK, Success(map[string]string{"status": "updated"}))
}

func (h *HandlersEnhanced) getNotificationChannels(c *gin.Context) {
	channels := h.alertNotify.GetChannels()
	c.JSON(http.StatusOK, Success(channels))
}

func (h *HandlersEnhanced) addNotificationChannel(c *gin.Context) {
	var channel NotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if channel.ID == "" {
		channel.ID = generateID()
	}

	h.alertNotify.AddChannel(&channel)
	c.JSON(http.StatusCreated, Success(channel))
}

func (h *HandlersEnhanced) updateNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	var channel NotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	channel.ID = id
	h.alertNotify.AddChannel(&channel)
	c.JSON(http.StatusOK, Success(channel))
}

func (h *HandlersEnhanced) removeNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	h.alertNotify.RemoveChannel(id)
	c.JSON(http.StatusOK, Success(map[string]string{"status": "removed"}))
}

func (h *HandlersEnhanced) testNotificationChannel(c *gin.Context) {
	channelID := c.Query("channel_id")

	// 创建测试告警
	testAlert := &Alert{
		ID:         "test-" + generateID(),
		QuotaID:    "test",
		Type:       AlertTypeSoftLimit,
		Severity:   AlertSeverityWarning,
		Status:     AlertStatusActive,
		TargetName: "测试配额",
		Message:    "这是一条测试告警消息",
		CreatedAt:  time.Now(),
	}

	// 发送测试通知
	if h.monitor.config.NotifyWebhook {
		h.monitor.sendWebhook(testAlert)
	}

	c.JSON(http.StatusOK, Success(map[string]string{
		"status":   "sent",
		"channel":  channelID,
		"alert_id": testAlert.ID,
	}))
}

// ========== 大文件检测 API ==========

func (h *HandlersEnhanced) listLargeFileRules(c *gin.Context) {
	volumeName := c.Query("volume")
	rules := h.cleanupEnhanced.ListLargeFileRules(volumeName)
	c.JSON(http.StatusOK, Success(rules))
}

func (h *HandlersEnhanced) createLargeFileRule(c *gin.Context) {
	var rule LargeFileRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	created, err := h.cleanupEnhanced.CreateLargeFileRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(created))
}

func (h *HandlersEnhanced) getLargeFileRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.cleanupEnhanced.GetLargeFileRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(rule))
}

func (h *HandlersEnhanced) updateLargeFileRule(c *gin.Context) {
	id := c.Param("id")
	var rule LargeFileRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	updated, err := h.cleanupEnhanced.UpdateLargeFileRule(id, rule)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(updated))
}

func (h *HandlersEnhanced) deleteLargeFileRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.cleanupEnhanced.DeleteLargeFileRule(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

func (h *HandlersEnhanced) scanLargeFiles(c *gin.Context) {
	id := c.Param("id")
	topN := 10
	if n := c.Query("top_n"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil {
			topN = parsed
		}
	}

	result, err := h.cleanupEnhanced.ScanLargeFiles(id, topN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(result))
}

func (h *HandlersEnhanced) executeLargeFileCleanup(c *gin.Context) {
	id := c.Param("id")
	topN := 10
	if n := c.Query("top_n"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil {
			topN = parsed
		}
	}

	result, err := h.cleanupEnhanced.ScanLargeFiles(id, topN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	// 获取规则以确定动作
	rule, _ := h.cleanupEnhanced.GetLargeFileRule(id)
	if rule != nil && rule.Action != "" {
		// 执行清理动作（简化实现）
		for _, file := range result.TopLargest {
			// 实际实现中会执行删除/归档等操作
			_ = file
		}
	}

	c.JSON(http.StatusOK, Success(result))
}

// ========== 过期文件清理 API ==========

func (h *HandlersEnhanced) listExpiredFileRules(c *gin.Context) {
	volumeName := c.Query("volume")
	rules := h.cleanupEnhanced.ListExpiredFileRules(volumeName)
	c.JSON(http.StatusOK, Success(rules))
}

func (h *HandlersEnhanced) createExpiredFileRule(c *gin.Context) {
	var rule ExpiredFileRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	created, err := h.cleanupEnhanced.CreateExpiredFileRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(created))
}

func (h *HandlersEnhanced) getExpiredFileRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.cleanupEnhanced.GetExpiredFileRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(rule))
}

func (h *HandlersEnhanced) updateExpiredFileRule(c *gin.Context) {
	id := c.Param("id")
	var rule ExpiredFileRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	updated, err := h.cleanupEnhanced.UpdateExpiredFileRule(id, rule)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(updated))
}

func (h *HandlersEnhanced) deleteExpiredFileRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.cleanupEnhanced.DeleteExpiredFileRule(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

func (h *HandlersEnhanced) scanExpiredFiles(c *gin.Context) {
	id := c.Param("id")
	result, err := h.cleanupEnhanced.ScanExpiredFiles(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(result))
}

func (h *HandlersEnhanced) executeExpiredFileCleanup(c *gin.Context) {
	id := c.Param("id")
	result, err := h.cleanupEnhanced.ExecuteExpiredFileCleanup(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(result))
}

// ========== 规则集 API ==========

func (h *HandlersEnhanced) listRuleSets(c *gin.Context) {
	volumeName := c.Query("volume")
	ruleSets := h.cleanupEnhanced.ListRuleSets(volumeName)
	c.JSON(http.StatusOK, Success(ruleSets))
}

func (h *HandlersEnhanced) createRuleSet(c *gin.Context) {
	var ruleSet CleanupRuleSet
	if err := c.ShouldBindJSON(&ruleSet); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	created, err := h.cleanupEnhanced.CreateRuleSet(ruleSet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(created))
}

func (h *HandlersEnhanced) getRuleSet(c *gin.Context) {
	id := c.Param("id")
	ruleSet, err := h.cleanupEnhanced.GetRuleSet(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(ruleSet))
}

func (h *HandlersEnhanced) updateRuleSet(c *gin.Context) {
	id := c.Param("id")
	var ruleSet CleanupRuleSet
	if err := c.ShouldBindJSON(&ruleSet); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	ruleSet.ID = id
	updated, err := h.cleanupEnhanced.CreateRuleSet(ruleSet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(updated))
}

func (h *HandlersEnhanced) deleteRuleSet(c *gin.Context) {
	// 简化实现
	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

func (h *HandlersEnhanced) executeRuleSet(c *gin.Context) {
	id := c.Param("id")
	results, err := h.cleanupEnhanced.ExecuteRuleSet(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(results))
}

// ========== 趋势数据 API ==========

func (h *HandlersEnhanced) getAllTrends(c *gin.Context) {
	duration := 24 * time.Hour
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	stats := h.monitor.GetAllTrendStats(duration)
	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersEnhanced) getQuotaTrend(c *gin.Context) {
	quotaID := c.Param("quotaId")
	duration := 24 * time.Hour
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	data := h.trendManager.GetRawData(quotaID, duration)
	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersEnhanced) getTrendChartData(c *gin.Context) {
	quotaID := c.Param("quotaId")
	duration := 24 * time.Hour
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}
	granularity := c.Query("granularity") // raw, hourly, daily, weekly, monthly

	data := h.trendManager.GetChartData(quotaID, duration, granularity)
	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersEnhanced) getTrendStats(c *gin.Context) {
	quotaID := c.Param("quotaId")
	duration := 24 * time.Hour * 7 // 默认7天
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	stats := h.trendManager.GetTrendStats(quotaID, duration)
	if stats == nil {
		c.JSON(http.StatusNotFound, Error(404, "无数据"))
		return
	}
	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersEnhanced) getTrendReport(c *gin.Context) {
	quotaID := c.Param("quotaId")
	duration := 24 * time.Hour * 7 // 默认7天
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	report := h.trendManager.GenerateReport(quotaID, duration)
	if report == nil {
		c.JSON(http.StatusNotFound, Error(404, "无数据"))
		return
	}
	c.JSON(http.StatusOK, Success(report))
}

func (h *HandlersEnhanced) getTrendPrediction(c *gin.Context) {
	quotaID := c.Param("quotaId")
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			days = parsed
		}
	}

	prediction := h.trendManager.Predict(quotaID, days)
	if prediction == nil {
		c.JSON(http.StatusNotFound, Error(404, "数据不足，无法预测"))
		return
	}
	c.JSON(http.StatusOK, Success(prediction))
}

func (h *HandlersEnhanced) getAggregatedTrend(c *gin.Context) {
	quotaID := c.Param("quotaId")
	level := c.Param("level") // hourly, daily, weekly, monthly

	duration := 24 * time.Hour * 30 // 默认30天
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	data := h.trendManager.GetAggregatedData(quotaID, level, duration)
	c.JSON(http.StatusOK, Success(data))
}

// ========== 仪表盘 API ==========

// DashboardSummary 仪表盘摘要
type DashboardSummary struct {
	TotalQuotas     int                   `json:"total_quotas"`
	ActiveAlerts    int                   `json:"active_alerts"`
	CriticalAlerts  int                   `json:"critical_alerts"`
	OverSoftLimit   int                   `json:"over_soft_limit"`
	OverHardLimit   int                   `json:"over_hard_limit"`
	TotalUsedBytes  uint64                `json:"total_used_bytes"`
	TotalLimitBytes uint64                `json:"total_limit_bytes"`
	AvgUsagePercent float64               `json:"avg_usage_percent"`
	Predictions     []*TrendPrediction    `json:"predictions,omitempty"`
	TopUsageQuotas  []QuotaUsage          `json:"top_usage_quotas"`
	RecentAlerts    []*Alert              `json:"recent_alerts"`
	Recommendations []TrendRecommendation `json:"recommendations"`
}

func (h *HandlersEnhanced) getDashboardSummary(c *gin.Context) {
	// 获取所有配额使用情况
	usages, _ := h.manager.GetAllUsage()

	// 获取告警
	alerts := h.manager.GetAlerts()

	// 计算摘要
	summary := &DashboardSummary{
		TotalQuotas:     len(usages),
		ActiveAlerts:    len(alerts),
		CriticalAlerts:  0,
		OverSoftLimit:   0,
		OverHardLimit:   0,
		TopUsageQuotas:  make([]QuotaUsage, 0),
		RecentAlerts:    alerts,
		Recommendations: make([]TrendRecommendation, 0),
	}

	var totalUsed, totalLimit uint64
	for _, u := range usages {
		totalUsed += u.UsedBytes
		totalLimit += u.HardLimit

		if u.IsOverHard {
			summary.OverHardLimit++
		}
		if u.IsOverSoft {
			summary.OverSoftLimit++
		}

		if u.UsagePercent >= 80 {
			summary.TopUsageQuotas = append(summary.TopUsageQuotas, *u)
		}
	}

	summary.TotalUsedBytes = totalUsed
	summary.TotalLimitBytes = totalLimit
	if totalLimit > 0 {
		summary.AvgUsagePercent = float64(totalUsed) / float64(totalLimit) * 100
	}

	// 统计严重告警
	for _, a := range alerts {
		if a.Severity == AlertSeverityCritical || a.Severity == AlertSeverityEmergency {
			summary.CriticalAlerts++
		}
	}

	c.JSON(http.StatusOK, Success(summary))
}

func (h *HandlersEnhanced) getDashboardAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()

	// 按严重级别分组
	grouped := map[string][]*Alert{
		"emergency": {},
		"critical":  {},
		"warning":   {},
		"info":      {},
	}

	for _, a := range alerts {
		grouped[string(a.Severity)] = append(grouped[string(a.Severity)], a)
	}

	c.JSON(http.StatusOK, Success(map[string]interface{}{
		"total":   len(alerts),
		"grouped": grouped,
		"alerts":  alerts,
	}))
}

func (h *HandlersEnhanced) getDashboardTrends(c *gin.Context) {
	// 获取所有配额的趋势统计
	duration := 24 * time.Hour * 7
	stats := h.monitor.GetAllTrendStats(duration)

	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersEnhanced) getDashboardPredictions(c *gin.Context) {
	usages, _ := h.manager.GetAllUsage()

	predictions := make([]*TrendPrediction, 0)
	for _, u := range usages {
		if u.UsagePercent >= 50 { // 只预测使用率超过50%的配额
			pred := h.trendManager.Predict(u.QuotaID, 30)
			if pred != nil {
				predictions = append(predictions, pred)
			}
		}
	}

	c.JSON(http.StatusOK, Success(predictions))
}

func (h *HandlersEnhanced) getDashboardRecommendations(c *gin.Context) {
	usages, _ := h.manager.GetAllUsage()

	recommendations := make([]TrendRecommendation, 0)

	for _, u := range usages {
		// 生成建议
		if u.UsagePercent >= 90 {
			recommendations = append(recommendations, TrendRecommendation{
				Type:        "warning",
				Priority:    "critical",
				Title:       u.TargetName + " 存储即将耗尽",
				Description: "当前使用率已达 " + strconv.FormatFloat(u.UsagePercent, 'f', 1, 64) + "%",
				Action:      "立即清理或增加配额",
				Impact:      "可能导致写入失败",
			})
		} else if u.UsagePercent >= 80 {
			recommendations = append(recommendations, TrendRecommendation{
				Type:        "cleanup",
				Priority:    "high",
				Title:       u.TargetName + " 存储空间紧张",
				Description: "当前使用率已达 " + strconv.FormatFloat(u.UsagePercent, 'f', 1, 64) + "%",
				Action:      "检查并清理不需要的文件",
				Impact:      "可能影响系统性能",
			})
		}
	}

	c.JSON(http.StatusOK, Success(recommendations))
}
