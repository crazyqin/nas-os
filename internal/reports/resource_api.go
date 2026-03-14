// Package reports 提供报表生成和管理功能
package reports

import (
	"time"

	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// ResourceAPIHandlers 资源分析 API 处理器
type ResourceAPIHandlers struct {
	generator         *ReportGenerator
	costCalculator    *StorageCostCalculator
	costOptimizer     *CostOptimizer
	capacityPlanner   *CapacityPlanner
	bandwidthReporter *BandwidthReporter
}

// NewResourceAPIHandlers 创建资源分析 API 处理器
func NewResourceAPIHandlers(gen *ReportGenerator) *ResourceAPIHandlers {
	// 使用默认配置创建各分析器
	costConfig := StorageCostConfig{
		CostPerGBMonthly:        0.5,
		CostPerIOPSMonthly:      0.01,
		CostPerBandwidthMonthly: 1.0,
		ElectricityCostPerKWh:   0.6,
		DevicePowerWatts:        100,
		OpsCostMonthly:          500,
		DepreciationYears:       5,
		HardwareCost:            50000,
	}

	capacityConfig := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
		GrowthModel:       GrowthModelLinear,
		ExpansionLeadTime: 30,
		SafetyBuffer:      20.0,
	}

	bandwidthConfig := BandwidthReportConfig{
		BandwidthLimitMbps:           1000,
		HighUtilizationThreshold:     70.0,
		CriticalUtilizationThreshold: 90.0,
		ErrorRateThreshold:           1.0,
		DropRateThreshold:            0.5,
		TrendSampleInterval:          5,
	}

	return &ResourceAPIHandlers{
		generator:         gen,
		costCalculator:    NewStorageCostCalculator(costConfig),
		costOptimizer:     NewCostOptimizer(costConfig),
		capacityPlanner:   NewCapacityPlanner(capacityConfig),
		bandwidthReporter: NewBandwidthReporter(bandwidthConfig),
	}
}

// RegisterResourceRoutes 注册资源分析路由
func (h *ResourceAPIHandlers) RegisterResourceRoutes(apiGroup *gin.RouterGroup) {
	// ========== 存储成本分析 ==========
	cost := apiGroup.Group("/storage-cost")
	{
		cost.GET("/config", h.getStorageCostConfig)
		cost.PUT("/config", h.updateStorageCostConfig)
		cost.POST("/calculate", h.calculateStorageCost)
		cost.POST("/report", h.generateStorageCostReport)
		cost.POST("/trend", h.analyzeCostTrend)
	}

	// ========== 成本优化分析 ==========
	optimization := apiGroup.Group("/cost-optimization")
	{
		optimization.POST("/analyze", h.analyzeCostOptimization)
		optimization.POST("/waste", h.analyzeWaste)
		optimization.POST("/opportunities", h.identifyOptimizationOpportunities)
		optimization.POST("/report", h.generateOptimizationReport)
		optimization.GET("/recommendations", h.getOptimizationRecommendations)
	}

	// ========== 容量规划 ==========
	capacity := apiGroup.Group("/capacity-planning")
	{
		capacity.GET("/config", h.getCapacityConfig)
		capacity.PUT("/config", h.updateCapacityConfig)
		capacity.POST("/analyze", h.analyzeCapacity)
		capacity.POST("/predict", h.predictCapacity)
		capacity.POST("/forecast", h.generateCapacityForecast)
		capacity.GET("/milestones", h.getCapacityMilestones)
		capacity.GET("/recommendations", h.getCapacityRecommendations)
	}

	// ========== 带宽使用报告 ==========
	bandwidth := apiGroup.Group("/bandwidth")
	{
		bandwidth.GET("/config", h.getBandwidthConfig)
		bandwidth.PUT("/config", h.updateBandwidthConfig)
		bandwidth.POST("/stats", h.calculateBandwidthStats)
		bandwidth.POST("/report", h.generateBandwidthReport)
		bandwidth.POST("/trend", h.analyzeBandwidthTrend)
		bandwidth.GET("/alerts", h.getBandwidthAlerts)
		bandwidth.GET("/recommendations", h.getBandwidthRecommendations)
	}
}

// ========== 存储成本分析 API ==========

func (h *ResourceAPIHandlers) getStorageCostConfig(c *gin.Context) {
	config := h.costCalculator.GetConfig()
	api.OK(c, config)
}

func (h *ResourceAPIHandlers) updateStorageCostConfig(c *gin.Context) {
	var config StorageCostConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.costCalculator.UpdateConfig(config)
	h.costOptimizer.UpdateConfig(config)

	api.OK(c, gin.H{"message": "配置已更新"})
}

func (h *ResourceAPIHandlers) calculateStorageCost(c *gin.Context) {
	var req struct {
		Metrics []StorageMetrics `json:"metrics" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	results := h.costCalculator.CalculateAll(req.Metrics)
	api.OK(c, results)
}

func (h *ResourceAPIHandlers) generateStorageCostReport(c *gin.Context) {
	var req struct {
		Metrics   []StorageMetrics `json:"metrics" binding:"required"`
		StartTime *time.Time       `json:"start_time"`
		EndTime   *time.Time       `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	}
	if req.StartTime != nil && req.EndTime != nil {
		period = ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report := h.costCalculator.GenerateReport(req.Metrics, period)
	api.OK(c, report)
}

func (h *ResourceAPIHandlers) analyzeCostTrend(c *gin.Context) {
	var req struct {
		History []CostTrendPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	report := h.costCalculator.AnalyzeTrend(req.History)
	if report == nil {
		api.BadRequest(c, "历史数据不足，至少需要2个数据点")
		return
	}

	api.OK(c, report)
}

// ========== 成本优化分析 API ==========

func (h *ResourceAPIHandlers) analyzeCostOptimization(c *gin.Context) {
	var req struct {
		WasteItems    []WasteItem         `json:"waste_items"`
		VolumeMetrics []StorageMetrics    `json:"volume_metrics"`
		CurrentCosts  []StorageCostResult `json:"current_costs"`
		TotalCapacity uint64              `json:"total_capacity"`
		StartTime     *time.Time          `json:"start_time"`
		EndTime       *time.Time          `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	}
	if req.StartTime != nil && req.EndTime != nil {
		period = ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report := h.costOptimizer.GenerateReport(
		req.WasteItems,
		req.VolumeMetrics,
		req.CurrentCosts,
		req.TotalCapacity,
		period,
	)

	api.OK(c, report)
}

func (h *ResourceAPIHandlers) analyzeWaste(c *gin.Context) {
	var req struct {
		WasteItems    []WasteItem `json:"waste_items" binding:"required"`
		TotalCapacity uint64      `json:"total_capacity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	summary := h.costOptimizer.AnalyzeWaste(req.WasteItems, req.TotalCapacity)
	api.OK(c, summary)
}

func (h *ResourceAPIHandlers) identifyOptimizationOpportunities(c *gin.Context) {
	var req struct {
		WasteItems    []WasteItem         `json:"waste_items"`
		VolumeMetrics []StorageMetrics    `json:"volume_metrics"`
		CurrentCosts  []StorageCostResult `json:"current_costs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	opportunities := h.costOptimizer.IdentifyOpportunities(
		req.WasteItems,
		req.VolumeMetrics,
		req.CurrentCosts,
	)

	api.OK(c, opportunities)
}

func (h *ResourceAPIHandlers) generateOptimizationReport(c *gin.Context) {
	var req struct {
		WasteItems    []WasteItem         `json:"waste_items"`
		VolumeMetrics []StorageMetrics    `json:"volume_metrics"`
		CurrentCosts  []StorageCostResult `json:"current_costs"`
		TotalCapacity uint64              `json:"total_capacity"`
		StartTime     *time.Time          `json:"start_time"`
		EndTime       *time.Time          `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	}
	if req.StartTime != nil && req.EndTime != nil {
		period = ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report := h.costOptimizer.GenerateReport(
		req.WasteItems,
		req.VolumeMetrics,
		req.CurrentCosts,
		req.TotalCapacity,
		period,
	)

	api.OK(c, report)
}

func (h *ResourceAPIHandlers) getOptimizationRecommendations(c *gin.Context) {
	// 返回通用的优化建议模板
	recommendations := []map[string]interface{}{
		{
			"type":        "cleanup",
			"priority":    "high",
			"title":       "清理重复文件",
			"description": "扫描并删除重复文件，释放存储空间",
			"savings":     "可回收5-15%存储空间",
		},
		{
			"type":        "cleanup",
			"priority":    "high",
			"title":       "清理过期数据",
			"description": "根据保留策略清理过期备份和临时文件",
			"savings":     "可回收10-30%存储空间",
		},
		{
			"type":        "optimize",
			"priority":    "medium",
			"title":       "启用数据压缩",
			"description": "对适合的数据类型启用压缩",
			"savings":     "可节省20-40%存储空间",
		},
		{
			"type":        "optimize",
			"priority":    "medium",
			"title":       "启用数据去重",
			"description": "启用块级或文件级去重",
			"savings":     "可节省10-30%存储空间",
		},
		{
			"type":        "tiering",
			"priority":    "low",
			"title":       "实施分层存储",
			"description": "将冷数据迁移到低成本存储",
			"savings":     "可降低50%存储成本",
		},
	}

	api.OK(c, recommendations)
}

// ========== 容量规划 API ==========

func (h *ResourceAPIHandlers) getCapacityConfig(c *gin.Context) {
	config := h.capacityPlanner.GetConfig()
	api.OK(c, config)
}

func (h *ResourceAPIHandlers) updateCapacityConfig(c *gin.Context) {
	var config CapacityPlanningConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.capacityPlanner.UpdateConfig(config)
	api.OK(c, gin.H{"message": "配置已更新"})
}

func (h *ResourceAPIHandlers) analyzeCapacity(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	volumeName := req.VolumeName
	if volumeName == "" {
		volumeName = "default"
	}

	report := h.capacityPlanner.Analyze(req.History, volumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, report)
}

func (h *ResourceAPIHandlers) predictCapacity(c *gin.Context) {
	var req struct {
		History      []CapacityHistory `json:"history" binding:"required"`
		TargetMonths int               `json:"target_months"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	targetMonths := req.TargetMonths
	if targetMonths <= 0 {
		targetMonths = 3
	}

	predicted, err := h.capacityPlanner.PredictCapacityNeeds(req.History, targetMonths)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"target_months":   targetMonths,
		"predicted_bytes": predicted,
		"predicted_gb":    float64(predicted) / (1024 * 1024 * 1024),
		"generated_at":    time.Now(),
	})
}

func (h *ResourceAPIHandlers) generateCapacityForecast(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	volumeName := req.VolumeName
	if volumeName == "" {
		volumeName = "default"
	}

	report := h.capacityPlanner.Analyze(req.History, volumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, gin.H{
		"volume_name":  volumeName,
		"forecasts":    report.Forecasts,
		"current":      report.Current,
		"summary":      report.Summary,
		"generated_at": time.Now(),
	})
}

func (h *ResourceAPIHandlers) getCapacityMilestones(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	volumeName := req.VolumeName
	if volumeName == "" {
		volumeName = "default"
	}

	report := h.capacityPlanner.Analyze(req.History, volumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, report.Milestones)
}

func (h *ResourceAPIHandlers) getCapacityRecommendations(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	volumeName := req.VolumeName
	if volumeName == "" {
		volumeName = "default"
	}

	report := h.capacityPlanner.Analyze(req.History, volumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, report.Recommendations)
}

// ========== 带宽使用报告 API ==========

func (h *ResourceAPIHandlers) getBandwidthConfig(c *gin.Context) {
	config := h.bandwidthReporter.GetConfig()
	api.OK(c, config)
}

func (h *ResourceAPIHandlers) updateBandwidthConfig(c *gin.Context) {
	var config BandwidthReportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.bandwidthReporter.UpdateConfig(config)
	api.OK(c, gin.H{"message": "配置已更新"})
}

func (h *ResourceAPIHandlers) calculateBandwidthStats(c *gin.Context) {
	var req struct {
		History   []BandwidthHistoryPoint `json:"history" binding:"required"`
		Interface string                  `json:"interface"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	iface := req.Interface
	if iface == "" {
		iface = "eth0"
	}

	stats := h.bandwidthReporter.CalculateStats(req.History, iface)
	api.OK(c, stats)
}

func (h *ResourceAPIHandlers) generateBandwidthReport(c *gin.Context) {
	var req struct {
		HistoryByInterface map[string][]BandwidthHistoryPoint `json:"history_by_interface" binding:"required"`
		StartTime          *time.Time                         `json:"start_time"`
		EndTime            *time.Time                         `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, 0, -7),
		EndTime:   time.Now(),
	}
	if req.StartTime != nil && req.EndTime != nil {
		period = ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report := h.bandwidthReporter.GenerateReport(req.HistoryByInterface, period)
	api.OK(c, report)
}

func (h *ResourceAPIHandlers) analyzeBandwidthTrend(c *gin.Context) {
	var req struct {
		History []BandwidthHistoryPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	analysis := h.bandwidthReporter.AnalyzeBandwidthTrend(req.History)
	if analysis == nil {
		api.BadRequest(c, "历史数据不足，至少需要2个数据点")
		return
	}

	api.OK(c, analysis)
}

func (h *ResourceAPIHandlers) getBandwidthAlerts(c *gin.Context) {
	var req struct {
		History   []BandwidthHistoryPoint `json:"history" binding:"required"`
		Interface string                  `json:"interface"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	iface := req.Interface
	if iface == "" {
		iface = "eth0"
	}

	alerts := h.bandwidthReporter.DetectAlerts(req.History, iface)
	api.OK(c, alerts)
}

func (h *ResourceAPIHandlers) getBandwidthRecommendations(c *gin.Context) {
	var req struct {
		Stats []BandwidthUsageStats `json:"stats" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	recommendations := h.bandwidthReporter.GenerateRecommendations(req.Stats)
	api.OK(c, recommendations)
}
