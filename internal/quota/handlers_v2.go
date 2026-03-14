// Package quota 提供存储配额管理功能
package quota

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HandlersV2 v2.7.0 新增 API 处理器
type HandlersV2 struct {
	manager         *Manager
	monitor         *Monitor
	historyMgr      *HistoryManager
	trendMgr        *TrendDataManager
	chartMgr        *ChartManager
	notifyMgr       *NotificationManager
	reportGenEnh    *ReportGeneratorEnhanced
	storageStatsMgr *StorageStatsManager
}

// NewHandlersV2 创建 v2.7.0 处理器
func NewHandlersV2(mgr *Manager) *HandlersV2 {
	historyConfig := DefaultHistoryConfig()
	historyMgr := NewHistoryManager(mgr, historyConfig)

	trendConfig := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, trendConfig)

	chartMgr := NewChartManager(mgr, historyMgr, trendMgr)
	notifyMgr := NewNotificationManager()
	reportGenEnh := NewReportGeneratorEnhanced(mgr, historyMgr, trendMgr)
	storageStatsMgr := NewStorageStatsManager(mgr, historyMgr, trendMgr)

	return &HandlersV2{
		manager:         mgr,
		monitor:         NewMonitor(mgr, mgr.alertConfig),
		historyMgr:      historyMgr,
		trendMgr:        trendMgr,
		chartMgr:        chartMgr,
		notifyMgr:       notifyMgr,
		reportGenEnh:    reportGenEnh,
		storageStatsMgr: storageStatsMgr,
	}
}

// Start 启动服务
func (h *HandlersV2) Start() {
	h.historyMgr.Start()
	h.trendMgr.Start()
}

// Stop 停止服务
func (h *HandlersV2) Stop() {
	h.historyMgr.Stop()
	h.trendMgr.Stop()
}

// RegisterRoutesV2 注册 v2.7.0 API 路由
func (h *HandlersV2) RegisterRoutesV2(api *gin.RouterGroup) {
	// ========== 配额历史统计 API ==========
	history := api.Group("/quota-history")
	{
		history.GET("", h.getHistory)
		history.GET("/statistics/:quotaId", h.getHistoryStatistics)
		history.GET("/statistics", h.getAllHistoryStatistics)
		history.POST("/query", h.queryHistory)
	}

	// ========== 配额使用图表 API ==========
	chart := api.Group("/quota-chart")
	{
		chart.POST("/data", h.getChartData)
		chart.GET("/line/:quotaId", h.getLineChart)
		chart.GET("/bar", h.getBarChart)
		chart.GET("/pie", h.getPieChart)
		chart.GET("/gauge/:quotaId", h.getGaugeChart)
		chart.GET("/heatmap/:quotaId", h.getHeatmapChart)
	}

	// ========== 预警通知 API ==========
	notify := api.Group("/quota-notification")
	{
		notify.GET("/channels", h.getNotificationChannels)
		notify.POST("/channels", h.addNotificationChannel)
		notify.PUT("/channels/:id", h.updateNotificationChannel)
		notify.DELETE("/channels/:id", h.removeNotificationChannel)
		notify.POST("/send", h.sendNotification)
		notify.GET("/history", h.getNotificationHistory)
		notify.POST("/test/:channelId", h.testNotificationChannel)
	}

	// ========== 用户资源报告 API ==========
	userReport := api.Group("/quota-user-report")
	{
		userReport.GET("/:username", h.getUserResourceReport)
		userReport.GET("/:username/export", h.exportUserReport)
	}

	// ========== 系统资源报告 API ==========
	systemReport := api.Group("/quota-system-report")
	{
		systemReport.GET("", h.getSystemResourceReport)
		systemReport.GET("/export", h.exportSystemReport)
		systemReport.GET("/summary", h.getSystemSummary)
	}

	// ========== 存储使用统计 API ==========
	storageStats := api.Group("/quota-storage-stats")
	{
		storageStats.GET("", h.getGlobalStorageStats)
		storageStats.GET("/volumes", h.getAllVolumesStats)
		storageStats.GET("/volumes/:volumeName", h.getVolumeStorageStats)
		storageStats.GET("/top-users", h.getTopUsers)
		storageStats.GET("/trend", h.getStorageTrend)
	}
}

// ========== 配额历史统计 API 实现 ==========

func (h *HandlersV2) getHistory(c *gin.Context) {
	quotaID := c.Query("quota_id")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")
	limitStr := c.Query("limit")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			startTime = time.Now().AddDate(0, 0, -7)
		}
	} else {
		startTime = time.Now().AddDate(0, 0, -7)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			endTime = time.Now()
		}
	} else {
		endTime = time.Now()
	}

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	query := HistoryQuery{
		QuotaID:   quotaID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     limit,
	}

	records := h.historyMgr.Query(query)
	c.JSON(http.StatusOK, Success(records))
}

func (h *HandlersV2) getHistoryStatistics(c *gin.Context) {
	quotaID := c.Param("quotaId")
	durationStr := c.Query("duration")

	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	startTime := time.Now().Add(-duration)
	endTime := time.Now()

	stats := h.historyMgr.GetStatistics(quotaID, startTime, endTime)
	if stats == nil {
		c.JSON(http.StatusNotFound, Error(404, "无历史数据"))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersV2) getAllHistoryStatistics(c *gin.Context) {
	durationStr := c.Query("duration")

	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	startTime := time.Now().Add(-duration)
	endTime := time.Now()

	stats := h.historyMgr.GetAllStatistics(startTime, endTime)
	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersV2) queryHistory(c *gin.Context) {
	var query HistoryQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if query.StartTime == nil {
		t := time.Now().AddDate(0, 0, -7)
		query.StartTime = &t
	}
	if query.EndTime == nil {
		t := time.Now()
		query.EndTime = &t
	}
	if query.Limit == 0 {
		query.Limit = 100
	}

	records := h.historyMgr.Query(query)
	c.JSON(http.StatusOK, Success(records))
}

// ========== 配额使用图表 API 实现 ==========

func (h *HandlersV2) getChartData(c *gin.Context) {
	var req ChartDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 设置默认时间范围
	if req.StartTime.IsZero() {
		req.StartTime = time.Now().AddDate(0, 0, -7)
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersV2) getLineChart(c *gin.Context) {
	quotaID := c.Param("quotaId")
	durationStr := c.Query("duration")

	duration := 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	req := ChartDataRequest{
		QuotaID:   quotaID,
		ChartType: ChartTypeLine,
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersV2) getBarChart(c *gin.Context) {
	volumeName := c.Query("volume")

	req := ChartDataRequest{
		VolumeName: volumeName,
		ChartType:  ChartTypeBar,
		StartTime:  time.Now().AddDate(0, 0, -7),
		EndTime:    time.Now(),
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersV2) getPieChart(c *gin.Context) {
	volumeName := c.Query("volume")

	req := ChartDataRequest{
		VolumeName: volumeName,
		ChartType:  ChartTypePie,
		StartTime:  time.Now().AddDate(0, 0, -7),
		EndTime:    time.Now(),
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersV2) getGaugeChart(c *gin.Context) {
	quotaID := c.Param("quotaId")

	req := ChartDataRequest{
		QuotaID:   quotaID,
		ChartType: ChartTypeGauge,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

func (h *HandlersV2) getHeatmapChart(c *gin.Context) {
	quotaID := c.Param("quotaId")
	durationStr := c.Query("duration")

	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	req := ChartDataRequest{
		QuotaID:   quotaID,
		ChartType: ChartTypeHeatmap,
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	data, err := h.chartMgr.GetChartData(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(data))
}

// ========== 预警通知 API 实现 ==========

func (h *HandlersV2) getNotificationChannels(c *gin.Context) {
	channels := h.notifyMgr.GetChannels()
	c.JSON(http.StatusOK, Success(channels))
}

func (h *HandlersV2) addNotificationChannel(c *gin.Context) {
	var channel NotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	h.notifyMgr.AddChannel(&channel)
	c.JSON(http.StatusCreated, Success(channel))
}

func (h *HandlersV2) updateNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	var channel NotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	channel.ID = id
	h.notifyMgr.AddChannel(&channel)
	c.JSON(http.StatusOK, Success(channel))
}

func (h *HandlersV2) removeNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	h.notifyMgr.RemoveChannel(id)
	c.JSON(http.StatusOK, Success(map[string]string{"status": "removed"}))
}

func (h *HandlersV2) sendNotification(c *gin.Context) {
	var req struct {
		AlertID   string `json:"alert_id" binding:"required"`
		ChannelID string `json:"channel_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 查找指定告警
	var targetAlert *Alert
	alerts := h.manager.GetAlerts()
	for _, a := range alerts {
		if a.ID == req.AlertID {
			targetAlert = a
			break
		}
	}

	// 如果没有找到告警，创建一个测试告警
	if targetAlert == nil {
		targetAlert = &Alert{
			ID:         req.AlertID,
			QuotaID:    "test",
			Type:       AlertTypeSoftLimit,
			Severity:   AlertSeverityWarning,
			Status:     AlertStatusActive,
			TargetName: "测试目标",
			Message:    "测试通知消息",
			CreatedAt:  time.Now(),
		}
	}

	notification, err := h.notifyMgr.SendNotification(targetAlert, req.ChannelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(notification))
}

func (h *HandlersV2) getNotificationHistory(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	history := h.notifyMgr.GetHistory(limit)
	c.JSON(http.StatusOK, Success(history))
}

func (h *HandlersV2) testNotificationChannel(c *gin.Context) {
	channelID := c.Param("channelId")

	testAlert := &Alert{
		ID:         "test-" + generateID(),
		QuotaID:    "test",
		Type:       AlertTypeSoftLimit,
		Severity:   AlertSeverityInfo,
		Status:     AlertStatusActive,
		TargetName: "测试配额",
		Message:    "这是一条测试通知消息",
		CreatedAt:  time.Now(),
	}

	notification, err := h.notifyMgr.SendNotification(testAlert, channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]interface{}{
		"status":       "sent",
		"notification": notification,
	}))
}

// ========== 用户资源报告 API 实现 ==========

func (h *HandlersV2) getUserResourceReport(c *gin.Context) {
	username := c.Param("username")
	durationStr := c.Query("duration")

	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	period := ReportPeriod{
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	report, err := h.reportGenEnh.GenerateUserReport(username, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(report))
}

func (h *HandlersV2) exportUserReport(c *gin.Context) {
	username := c.Param("username")
	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	durationStr := c.Query("duration")
	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	period := ReportPeriod{
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	report, err := h.reportGenEnh.GenerateUserReport(username, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	// 设置响应头
	filename := "user-report-" + username + "-" + time.Now().Format("2006-01-02")
	if format == "csv" {
		c.Header("Content-Disposition", "attachment; filename="+filename+".csv")
		c.Header("Content-Type", "text/csv")
		// 简化 CSV 输出
		c.String(http.StatusOK, "Username,QuotaID,VolumeName,UsedBytes,UsagePercent\n")
	} else {
		c.Header("Content-Disposition", "attachment; filename="+filename+".json")
		c.JSON(http.StatusOK, report)
	}
}

// ========== 系统资源报告 API 实现 ==========

func (h *HandlersV2) getSystemResourceReport(c *gin.Context) {
	durationStr := c.Query("duration")

	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	period := ReportPeriod{
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	report, err := h.reportGenEnh.GenerateSystemReport(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(report))
}

func (h *HandlersV2) exportSystemReport(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	durationStr := c.Query("duration")
	duration := 7 * 24 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	period := ReportPeriod{
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
	}

	report, err := h.reportGenEnh.GenerateSystemReport(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	filename := "system-report-" + time.Now().Format("2006-01-02")
	if format == "csv" {
		c.Header("Content-Disposition", "attachment; filename="+filename+".csv")
		c.Header("Content-Type", "text/csv")
		c.String(http.StatusOK, "VolumeName,TotalUsed,TotalLimit,UsagePercent\n")
	} else {
		c.Header("Content-Disposition", "attachment; filename="+filename+".json")
		c.JSON(http.StatusOK, report)
	}
}

func (h *HandlersV2) getSystemSummary(c *gin.Context) {
	usages, err := h.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	summary := SystemSummary{}
	userSet := make(map[string]bool)
	groupSet := make(map[string]bool)

	for _, usage := range usages {
		summary.TotalQuotas++
		summary.TotalUsedBytes += usage.UsedBytes
		summary.TotalLimitBytes += usage.HardLimit

		if usage.IsOverSoft {
			summary.OverSoftCount++
		}
		if usage.IsOverHard {
			summary.OverHardCount++
		}

		switch usage.Type {
		case QuotaTypeUser:
			userSet[usage.TargetID] = true
		case QuotaTypeGroup:
			groupSet[usage.TargetID] = true
		}
	}

	summary.TotalUsers = len(userSet)
	summary.TotalGroups = len(groupSet)

	if summary.TotalLimitBytes > 0 {
		summary.AvgUsagePercent = float64(summary.TotalUsedBytes) / float64(summary.TotalLimitBytes) * 100
	}

	c.JSON(http.StatusOK, Success(summary))
}

// ========== 存储使用统计 API 实现 ==========

func (h *HandlersV2) getGlobalStorageStats(c *gin.Context) {
	stats, err := h.storageStatsMgr.GetGlobalStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersV2) getAllVolumesStats(c *gin.Context) {
	stats, err := h.storageStatsMgr.GetAllVolumesStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersV2) getVolumeStorageStats(c *gin.Context) {
	volumeName := c.Param("volumeName")

	stats, err := h.storageStatsMgr.GetStorageStats(volumeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

func (h *HandlersV2) getTopUsers(c *gin.Context) {
	volumeName := c.Query("volume")
	limitStr := c.Query("limit")

	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	usages, err := h.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	// 过滤用户配额
	users := make([]QuotaUsage, 0)
	for _, usage := range usages {
		if usage.Type == QuotaTypeUser {
			if volumeName == "" || usage.VolumeName == volumeName {
				users = append(users, *usage)
			}
		}
	}

	// 排序
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			if users[j].UsedBytes > users[i].UsedBytes {
				users[i], users[j] = users[j], users[i]
			}
		}
	}

	// 限制数量
	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}

	c.JSON(http.StatusOK, Success(users))
}

func (h *HandlersV2) getStorageTrend(c *gin.Context) {
	usages, err := h.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	trend := h.storageStatsMgr.calculateStorageTrend(usages)
	c.JSON(http.StatusOK, Success(trend))
}
