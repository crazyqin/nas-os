// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ResourceAPIHandlers 资源分析 API 处理器.
type ResourceAPIHandlers struct {
	generator         *ReportGenerator
	costCalculator    *StorageCostCalculator
	costOptimizer     *CostOptimizer
	capacityPlanner   *CapacityPlanner
	bandwidthReporter *BandwidthReporter
}

// NewResourceAPIHandlers 创建资源分析 API 处理器.
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

// RegisterResourceRoutes 注册资源分析路由.
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

// ========== v2.35.0 增强功能：导出报告 API ==========

// EnhancedExportAPIHandlers 增强的导出 API 处理器.
type EnhancedExportAPIHandlers struct {
	exporter         *Exporter
	advancedExporter *AdvancedExcelExporter
}

// NewEnhancedExportAPIHandlers 创建增强导出 API 处理器.
func NewEnhancedExportAPIHandlers(exporter *Exporter) *EnhancedExportAPIHandlers {
	return &EnhancedExportAPIHandlers{
		exporter:         exporter,
		advancedExporter: NewAdvancedExcelExporter(NewExcelExporter("/tmp/reports")),
	}
}

// RegisterEnhancedExportRoutes 注册增强导出路由.
func (h *EnhancedExportAPIHandlers) RegisterEnhancedExportRoutes(apiGroup *gin.RouterGroup) {
	export := apiGroup.Group("/enhanced-export")
	{
		// 带图表导出
		export.POST("/with-charts", h.exportWithCharts)
		// 多工作表导出
		export.POST("/multi-sheet", h.exportMultiSheet)
		// 样式模板列表
		export.GET("/style-templates", h.listStyleTemplates)
		// 自定义样式模板
		export.POST("/style-templates", h.createStyleTemplate)
		// 批量导出
		export.POST("/batch", h.exportBatch)
		// 预览导出
		export.POST("/preview", h.previewExport)
	}
}

// exportWithCharts 带图表导出.
func (h *EnhancedExportAPIHandlers) exportWithCharts(c *gin.Context) {
	var req struct {
		Report        *GeneratedReport `json:"report" binding:"required"`
		Charts        []ChartConfig    `json:"charts"`
		StyleTemplate string           `json:"style_template"`
		OutputPath    string           `json:"output_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/reports/report_%s.xlsx", time.Now().Format("20060102_150405"))
	}

	result, err := h.advancedExporter.ExportWithCharts(req.Report, outputPath, req.Charts, req.StyleTemplate)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// exportMultiSheet 多工作表导出.
func (h *EnhancedExportAPIHandlers) exportMultiSheet(c *gin.Context) {
	var req struct {
		Report     *GeneratedReport  `json:"report" binding:"required"`
		Config     *MultiSheetConfig `json:"config"`
		OutputPath string            `json:"output_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/reports/multi_sheet_%s.xlsx", time.Now().Format("20060102_150405"))
	}

	result, err := h.advancedExporter.ExportMultiSheet(req.Report, req.Config, outputPath)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// listStyleTemplates 列出样式模板.
func (h *EnhancedExportAPIHandlers) listStyleTemplates(c *gin.Context) {
	templates := h.advancedExporter.ListStyleTemplates()
	api.OK(c, templates)
}

// createStyleTemplate 创建样式模板.
func (h *EnhancedExportAPIHandlers) createStyleTemplate(c *gin.Context) {
	var template ExcelStyleTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.advancedExporter.RegisterStyleTemplate(&template)
	api.OK(c, gin.H{"message": "样式模板已注册", "id": template.ID})
}

// exportBatch 批量导出.
func (h *EnhancedExportAPIHandlers) exportBatch(c *gin.Context) {
	var req struct {
		Reports   []*GeneratedReport `json:"reports" binding:"required"`
		Formats   []ExportFormat     `json:"formats" binding:"required"`
		OutputDir string             `json:"output_dir"`
		Options   ExportOptions      `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = "/tmp/reports/outputs"
	}

	results := make([]*ExportResult, 0)
	for _, report := range req.Reports {
		for _, format := range req.Formats {
			filename := fmt.Sprintf("%s_%s.%s", report.Name, time.Now().Format("20060102"), format)
			outputPath := fmt.Sprintf("%s/%s", outputDir, filename)

			result, err := h.exporter.Export(report, format, outputPath, req.Options)
			if err != nil {
				continue
			}
			results = append(results, result)
		}
	}

	api.OK(c, gin.H{
		"total":     len(req.Reports) * len(req.Formats),
		"succeeded": len(results),
		"results":   results,
	})
}

// previewExport 预览导出.
func (h *EnhancedExportAPIHandlers) previewExport(c *gin.Context) {
	var req struct {
		Report *GeneratedReport `json:"report" binding:"required"`
		Format ExportFormat     `json:"format"`
		Limit  int              `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	format := req.Format
	if format == "" {
		format = ExportJSON
	}

	limit := req.Limit
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	// 创建预览数据
	preview := map[string]interface{}{
		"report_name":   req.Report.Name,
		"total_records": req.Report.TotalRecords,
		"format":        format,
		"preview_data":  req.Report.Data,
	}

	if previewData, ok := preview["preview_data"].([]map[string]interface{}); ok && len(previewData) > limit {
		preview["preview_data"] = req.Report.Data[:limit]
	}

	api.OK(c, preview)
}

// ========== v2.35.0 增强功能：成本分析 API ==========

// CostAnalysisAPIHandlers 成本分析 API 处理器.
type CostAnalysisAPIHandlers struct {
	enhancedAnalyzer *EnhancedCostAnalyzer
	capacityAnalyzer *CapacityPlanningAnalyzer
	trendAnalyzer    *ResourceTrendAnalyzer
}

// NewCostAnalysisAPIHandlers 创建成本分析 API 处理器.
func NewCostAnalysisAPIHandlers(config StorageCostConfig) *CostAnalysisAPIHandlers {
	return &CostAnalysisAPIHandlers{
		enhancedAnalyzer: NewEnhancedCostAnalyzer(config),
		capacityAnalyzer: NewCapacityPlanningAnalyzer(config),
		trendAnalyzer:    NewResourceTrendAnalyzer(),
	}
}

// RegisterCostAnalysisRoutes 注册成本分析路由.
func (h *CostAnalysisAPIHandlers) RegisterCostAnalysisRoutes(apiGroup *gin.RouterGroup) {
	cost := apiGroup.Group("/cost-analysis")
	{
		// 增强成本预测
		cost.POST("/forecast", h.forecastCost)
		// 季节性分析
		cost.POST("/seasonality", h.analyzeSeasonality)
		// 异常检测
		cost.POST("/anomalies", h.detectAnomalies)
		// 多模型预测对比
		cost.POST("/multi-model", h.multiModelForecast)
		// 成本健康评分
		cost.POST("/health-score", h.calculateHealthScore)
	}

	// 增强容量规划
	capacity := apiGroup.Group("/capacity-planning-enhanced")
	{
		capacity.POST("/analyze", h.analyzeCapacityEnhanced)
		capacity.POST("/scenarios", h.generateScenarios)
		capacity.POST("/timeline", h.generateTimeline)
		capacity.POST("/risks", h.assessRisks)
		capacity.POST("/optimization-paths", h.getOptimizationPaths)
	}

	// 资源趋势分析
	trend := apiGroup.Group("/resource-trend")
	{
		trend.POST("/analyze", h.analyzeResourceTrend)
		trend.POST("/storage", h.analyzeStorageTrend)
		trend.POST("/io", h.analyzeIOTrend)
		trend.POST("/bandwidth", h.analyzeBandwidthTrendEnhanced)
		trend.POST("/correlations", h.analyzeCorrelations)
		trend.POST("/alerts", h.generateTrendAlerts)
	}
}

// forecastCost 成本预测.
func (h *CostAnalysisAPIHandlers) forecastCost(c *gin.Context) {
	var req struct {
		History []CostTrendDataPoint `json:"history" binding:"required"`
		Months  int                  `json:"months"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	months := req.Months
	if months <= 0 {
		months = 12
	}

	forecast := h.enhancedAnalyzer.ForecastEnhanced(req.History, months)
	if forecast == nil {
		api.BadRequest(c, "历史数据不足，至少需要3个数据点")
		return
	}

	api.OK(c, forecast)
}

// analyzeSeasonality 季节性分析.
func (h *CostAnalysisAPIHandlers) analyzeSeasonality(c *gin.Context) {
	var req struct {
		History []CostTrendDataPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	seasonality := h.enhancedAnalyzer.AnalyzeSeasonality(req.History)
	api.OK(c, seasonality)
}

// detectAnomalies 异常检测.
func (h *CostAnalysisAPIHandlers) detectAnomalies(c *gin.Context) {
	var req struct {
		History []CostTrendDataPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	anomalies := h.enhancedAnalyzer.DetectAnomalies(req.History)
	api.OK(c, anomalies)
}

// multiModelForecast 多模型预测对比.
func (h *CostAnalysisAPIHandlers) multiModelForecast(c *gin.Context) {
	var req struct {
		History []CostTrendDataPoint `json:"history" binding:"required"`
		Months  int                  `json:"months"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	months := req.Months
	if months <= 0 {
		months = 12
	}

	forecast := h.enhancedAnalyzer.ForecastEnhanced(req.History, months)
	if forecast == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, gin.H{
		"linear":       forecast.MultiModelForecasts["linear"],
		"exponential":  forecast.MultiModelForecasts["exponential"],
		"holt_winters": forecast.MultiModelForecasts["holt_winters"],
		"best_model":   forecast.Model,
		"accuracy":     forecast.AccuracyMetrics,
	})
}

// calculateHealthScore 计算成本健康评分.
func (h *CostAnalysisAPIHandlers) calculateHealthScore(c *gin.Context) {
	var req struct {
		VolumeCosts []VolumeCostAnalysis `json:"volume_costs"`
		UserCosts   []UserCostAnalysis   `json:"user_costs"`
		Trend       CostTrendAnalysis    `json:"trend"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 计算综合健康评分
	score := 100.0

	// 使用率评估
	var avgUsage float64
	for _, vc := range req.VolumeCosts {
		avgUsage += vc.UsagePercent
	}
	if len(req.VolumeCosts) > 0 {
		avgUsage /= float64(len(req.VolumeCosts))
	}

	if avgUsage < 30 {
		score -= 20
	} else if avgUsage > 90 {
		score -= 30
	} else if avgUsage > 80 {
		score -= 15
	}

	// 趋势评估
	if req.Trend.MonthlyGrowthRate > 20 {
		score -= 20
	} else if req.Trend.MonthlyGrowthRate > 10 {
		score -= 10
	}

	// 波动评估
	if req.Trend.Volatility > 30 {
		score -= 15
	}

	if score < 0 {
		score = 0
	}

	// 确定状态
	status := "healthy"
	if score < 60 {
		status = "critical"
	} else if score < 80 {
		status = "warning"
	}

	api.OK(c, gin.H{
		"health_score": int(score),
		"status":       status,
		"avg_usage":    avgUsage,
		"factors": map[string]interface{}{
			"usage_factor":  avgUsage,
			"growth_factor": req.Trend.MonthlyGrowthRate,
			"volatility":    req.Trend.Volatility,
		},
	})
}

// analyzeCapacityEnhanced 增强容量分析.
func (h *CostAnalysisAPIHandlers) analyzeCapacityEnhanced(c *gin.Context) {
	var req struct {
		History         []CapacityHistory `json:"history" binding:"required"`
		VolumeName      string            `json:"volume_name"`
		MonthsToProject int               `json:"months_to_project"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	months := req.MonthsToProject
	if months <= 0 {
		months = 12
	}

	plan := h.capacityAnalyzer.AnalyzeCapacityEnhanced(req.History, req.VolumeName, months)
	if plan == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, plan)
}

// generateScenarios 生成场景分析.
func (h *CostAnalysisAPIHandlers) generateScenarios(c *gin.Context) {
	var req struct {
		History []CapacityHistory `json:"history" binding:"required"`
		Months  int               `json:"months"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	months := req.Months
	if months <= 0 {
		months = 12
	}

	scenarios := h.capacityAnalyzer.generateScenarios(req.History, months)
	api.OK(c, scenarios)
}

// generateTimeline 生成扩容时间线.
func (h *CostAnalysisAPIHandlers) generateTimeline(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	planner := NewCapacityPlanner(CapacityPlanningConfig{ForecastDays: 365})
	report := planner.Analyze(req.History, req.VolumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	timeline := h.capacityAnalyzer.generateExpansionTimeline(report)
	api.OK(c, timeline)
}

// assessRisks 风险评估.
func (h *CostAnalysisAPIHandlers) assessRisks(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	planner := NewCapacityPlanner(CapacityPlanningConfig{ForecastDays: 365})
	report := planner.Analyze(req.History, req.VolumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	risks := h.capacityAnalyzer.assessRisks(report)
	api.OK(c, risks)
}

// getOptimizationPaths 获取优化路径.
func (h *CostAnalysisAPIHandlers) getOptimizationPaths(c *gin.Context) {
	var req struct {
		History    []CapacityHistory `json:"history" binding:"required"`
		VolumeName string            `json:"volume_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	planner := NewCapacityPlanner(CapacityPlanningConfig{ForecastDays: 365})
	report := planner.Analyze(req.History, req.VolumeName)
	if report == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	paths := h.capacityAnalyzer.generateOptimizationPaths(report)
	api.OK(c, paths)
}

// analyzeResourceTrend 资源趋势分析.
func (h *CostAnalysisAPIHandlers) analyzeResourceTrend(c *gin.Context) {
	var req struct {
		StorageHistory   []CapacityHistory       `json:"storage_history"`
		IOHistory        []IOHistoryPoint        `json:"io_history"`
		BandwidthHistory []BandwidthHistoryPoint `json:"bandwidth_history"`
		VolumeName       string                  `json:"volume_name"`
		StartTime        *time.Time              `json:"start_time"`
		EndTime          *time.Time              `json:"end_time"`
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

	analysis := &ResourceTrendAnalysis{
		VolumeName:     req.VolumeName,
		Period:         period,
		StorageTrend:   h.trendAnalyzer.analyzeStorageTrend(req.StorageHistory),
		IOTrend:        h.trendAnalyzer.analyzeIOTrend(req.IOHistory),
		BandwidthTrend: h.trendAnalyzer.analyzeBandwidthTrend(req.BandwidthHistory),
		Correlations:   h.trendAnalyzer.calculateCorrelations(req.StorageHistory, req.IOHistory, req.BandwidthHistory),
		GeneratedAt:    time.Now(),
	}

	api.OK(c, analysis)
}

// analyzeStorageTrend 存储趋势分析.
func (h *CostAnalysisAPIHandlers) analyzeStorageTrend(c *gin.Context) {
	var req struct {
		History []CapacityHistory `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	metrics := h.trendAnalyzer.analyzeStorageTrend(req.History)
	api.OK(c, metrics)
}

// analyzeIOTrend IO 趋势分析.
func (h *CostAnalysisAPIHandlers) analyzeIOTrend(c *gin.Context) {
	var req struct {
		History []IOHistoryPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	metrics := h.trendAnalyzer.analyzeIOTrend(req.History)
	api.OK(c, metrics)
}

// analyzeBandwidthTrendEnhanced 带宽趋势分析增强.
func (h *CostAnalysisAPIHandlers) analyzeBandwidthTrendEnhanced(c *gin.Context) {
	var req struct {
		History []BandwidthHistoryPoint `json:"history" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	metrics := h.trendAnalyzer.analyzeBandwidthTrend(req.History)
	api.OK(c, metrics)
}

// analyzeCorrelations 相关性分析.
func (h *CostAnalysisAPIHandlers) analyzeCorrelations(c *gin.Context) {
	var req struct {
		StorageHistory   []CapacityHistory       `json:"storage_history"`
		IOHistory        []IOHistoryPoint        `json:"io_history"`
		BandwidthHistory []BandwidthHistoryPoint `json:"bandwidth_history"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	correlations := h.trendAnalyzer.calculateCorrelations(
		req.StorageHistory,
		req.IOHistory,
		req.BandwidthHistory,
	)

	api.OK(c, correlations)
}

// generateTrendAlerts 生成趋势预警.
func (h *CostAnalysisAPIHandlers) generateTrendAlerts(c *gin.Context) {
	var req struct {
		StorageHistory []CapacityHistory `json:"storage_history"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	storageTrend := h.trendAnalyzer.analyzeStorageTrend(req.StorageHistory)

	analysis := &ResourceTrendAnalysis{
		StorageTrend: storageTrend,
	}

	alerts := h.trendAnalyzer.generateAlerts(analysis)
	api.OK(c, alerts)
}
