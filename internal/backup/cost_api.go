// Package backup 提供备份成本分析 API
package backup

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// CostHandlers 成本 API 处理器.
type CostHandlers struct {
	analyzer *CostAnalyzer
	manager  *Manager
}

// NewCostHandlers 创建成本 API 处理器.
func NewCostHandlers(analyzer *CostAnalyzer, manager *Manager) *CostHandlers {
	return &CostHandlers{
		analyzer: analyzer,
		manager:  manager,
	}
}

// RegisterCostRoutes 注册成本相关路由.
func (h *CostHandlers) RegisterCostRoutes(r *gin.RouterGroup) {
	cost := r.Group("/backup/cost")
	{
		// GET /api/backup/cost - 获取备份成本概览
		cost.GET("", h.getCost)

		// GET /api/backup/cost/trend - 获取成本趋势
		cost.GET("/trend", h.getCostTrend)

		// GET /api/backup/cost/report - 获取成本报告
		cost.GET("/report", h.getCostReport)

		// POST /api/backup/cost/optimize - 获取优化建议
		cost.POST("/optimize", h.getOptimizeSuggestions)

		// GET /api/backup/cost/records - 获取成本记录列表
		cost.GET("/records", h.getCostRecords)

		// GET /api/backup/cost/forecast - 获取成本预测
		cost.GET("/forecast", h.getCostForecast)

		// GET /api/backup/cost/alerts - 获取成本告警
		cost.GET("/alerts", h.getCostAlerts)

		// GET /api/backup/cost/providers - 获取存储提供商定价
		cost.GET("/providers", h.getProviderPricing)

		// PUT /api/backup/cost/providers/:provider - 更新提供商定价
		cost.PUT("/providers/:provider", h.updateProviderPricing)

		// GET /api/backup/cost/thresholds - 获取告警阈值
		cost.GET("/thresholds", h.getAlertThresholds)

		// PUT /api/backup/cost/thresholds - 更新告警阈值
		cost.PUT("/thresholds", h.updateAlertThresholds)
	}
}

// CostQueryRequest 成本查询请求.
type CostQueryRequest struct {
	StartTime string `form:"startTime" json:"startTime"`
	EndTime   string `form:"endTime" json:"endTime"`
	ConfigID  string `form:"configId" json:"configId"`
	Provider  string `form:"provider" json:"provider"`
}

// CostTrendRequest 成本趋势请求.
type CostTrendRequest struct {
	Days   int    `form:"days" json:"days"`
	Period string `form:"period" json:"period"`
}

// CostReportRequest 成本报告请求.
type CostReportRequest struct {
	Period string `form:"period" json:"period"`
}

// CostOverviewResponse 成本概览响应.
type CostOverviewResponse struct {
	CurrentPeriodCost    float64               `json:"currentPeriodCost"`
	PreviousPeriodCost   float64               `json:"previousPeriodCost"`
	CostChangeRate       float64               `json:"costChangeRate"`
	TotalStorage         int64                 `json:"totalStorage"`
	TotalStorageHuman    string                `json:"totalStorageHuman"`
	MonthlyCostToDate    float64               `json:"monthlyCostToDate"`
	EstimatedMonthlyCost float64               `json:"estimatedMonthlyCost"`
	AvgCompressionRatio  float64               `json:"avgCompressionRatio"`
	BackupCount          int                   `json:"backupCount"`
	RestoreCount         int                   `json:"restoreCount"`
	CostByProvider       []ProviderCostSummary `json:"costByProvider"`
}

// ProviderCostSummary 提供商成本摘要.
type ProviderCostSummary struct {
	Provider    string  `json:"provider"`
	TotalCost   float64 `json:"totalCost"`
	Storage     int64   `json:"storage"`
	BackupCount int     `json:"backupCount"`
}

// CostRecordResponse 成本记录响应.
type CostRecordResponse struct {
	Records []*CostRecord `json:"records"`
	Total   int           `json:"total"`
}

// getCost 获取备份成本概览
// @Summary 获取备份成本概览
// @Tags backup-cost
// @Router /backup/cost [get].
func (h *CostHandlers) getCost(c *gin.Context) {
	var req CostQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	report, err := h.analyzer.GenerateCostReport(PeriodMonthly)
	if err != nil {
		api.InternalError(c, "生成成本报告失败: "+err.Error())
		return
	}

	response := &CostOverviewResponse{
		CurrentPeriodCost:    report.Summary.TotalCost,
		TotalStorage:         report.Summary.TotalStorage,
		TotalStorageHuman:    report.Summary.TotalStorageHuman,
		MonthlyCostToDate:    report.Summary.TotalCost,
		EstimatedMonthlyCost: report.Summary.EstimatedMonthlyCost,
		AvgCompressionRatio:  report.Summary.AvgCompressionRatio,
		BackupCount:          report.Summary.BackupCount,
		RestoreCount:         report.Summary.RestoreCount,
	}

	response.CostByProvider = make([]ProviderCostSummary, 0, len(report.CostByProvider))
	for _, pc := range report.CostByProvider {
		response.CostByProvider = append(response.CostByProvider, ProviderCostSummary{
			Provider:    string(pc.Provider),
			TotalCost:   pc.TotalCost,
			Storage:     pc.Storage,
			BackupCount: pc.BackupCount,
		})
	}

	previousReport, _ := h.analyzer.GenerateCostReport(PeriodMonthly)
	if previousReport != nil && previousReport.Summary.TotalCost > 0 {
		response.PreviousPeriodCost = previousReport.Summary.TotalCost
		response.CostChangeRate = (report.Summary.TotalCost - previousReport.Summary.TotalCost) / previousReport.Summary.TotalCost * 100
	}

	if req.Provider != "" {
		filtered := make([]ProviderCostSummary, 0)
		for _, pc := range response.CostByProvider {
			if pc.Provider == req.Provider {
				filtered = append(filtered, pc)
			}
		}
		response.CostByProvider = filtered
	}

	api.OK(c, response)
}

// getCostTrend 获取成本趋势
// @Summary 获取成本趋势
// @Tags backup-cost
// @Router /backup/cost/trend [get].
func (h *CostHandlers) getCostTrend(c *gin.Context) {
	var req CostTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	if req.Days <= 0 {
		req.Days = 30
	}
	if req.Period == "" {
		req.Period = "daily"
	}

	period := ReportPeriod(req.Period)
	switch period {
	case PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodYearly:
	default:
		api.BadRequest(c, "无效的聚合周期: "+req.Period)
		return
	}

	trend, err := h.analyzer.GetCostTrend(req.Days, period)
	if err != nil {
		api.InternalError(c, "获取趋势数据失败: "+err.Error())
		return
	}

	api.OK(c, trend)
}

// getCostReport 获取成本报告
// @Summary 获取成本报告
// @Tags backup-cost
// @Router /backup/cost/report [get].
func (h *CostHandlers) getCostReport(c *gin.Context) {
	var req CostReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	if req.Period == "" {
		req.Period = "monthly"
	}

	period := ReportPeriod(req.Period)
	switch period {
	case PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodYearly:
	default:
		api.BadRequest(c, "无效的报告周期: "+req.Period)
		return
	}

	report, err := h.analyzer.GenerateCostReport(period)
	if err != nil {
		api.InternalError(c, "生成报告失败: "+err.Error())
		return
	}

	api.OK(c, report)
}

// getOptimizeSuggestions 获取优化建议
// @Summary 获取成本优化建议
// @Tags backup-cost
// @Router /backup/cost/optimize [post].
func (h *CostHandlers) getOptimizeSuggestions(c *gin.Context) {
	var req OptimizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	if req.OptimizeGoal == "" {
		req.OptimizeGoal = "balance"
	}

	switch req.OptimizeGoal {
	case "cost", "performance", "balance":
	default:
		api.BadRequest(c, "无效的优化目标: "+req.OptimizeGoal)
		return
	}

	response, err := h.analyzer.GetOptimizationSuggestions(&req)
	if err != nil {
		api.InternalError(c, "生成优化建议失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "优化建议生成成功", response)
}

// getCostRecords 获取成本记录列表
// @Summary 获取成本记录列表
// @Tags backup-cost
// @Router /backup/cost/records [get].
func (h *CostHandlers) getCostRecords(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		var err error
		if limit, err = parseIntParam(l, 100); err != nil {
			api.BadRequest(c, "无效的 limit 参数")
			return
		}
	}

	records := h.analyzer.GetRecords(limit)

	response := &CostRecordResponse{
		Records: records,
		Total:   len(records),
	}

	api.OK(c, response)
}

// getCostForecast 获取成本预测
// @Summary 获取成本预测
// @Tags backup-cost
// @Router /backup/cost/forecast [get].
func (h *CostHandlers) getCostForecast(c *gin.Context) {
	report, err := h.analyzer.GenerateCostReport(PeriodMonthly)
	if err != nil {
		api.InternalError(c, "获取预测数据失败: "+err.Error())
		return
	}

	if report.Forecast == nil {
		api.OK(c, map[string]interface{}{
			"available": false,
			"message":   "历史数据不足，无法生成预测",
		})
		return
	}

	api.OK(c, report.Forecast)
}

// getCostAlerts 获取成本告警
// @Summary 获取成本告警
// @Tags backup-cost
// @Router /backup/cost/alerts [get].
func (h *CostHandlers) getCostAlerts(c *gin.Context) {
	report, err := h.analyzer.GenerateCostReport(PeriodMonthly)
	if err != nil {
		api.InternalError(c, "获取告警失败: "+err.Error())
		return
	}

	api.OK(c, report.Alerts)
}

// getProviderPricing 获取存储提供商定价
// @Summary 获取存储提供商定价
// @Tags backup-cost
// @Router /backup/cost/providers [get].
func (h *CostHandlers) getProviderPricing(c *gin.Context) {
	configs := DefaultStorageCostConfigs()

	response := make(map[string]StorageCostConfig)
	for provider, config := range configs {
		response[string(provider)] = *config
	}

	api.OK(c, response)
}

// updateProviderPricingRequest 更新提供商定价请求.
type updateProviderPricingRequest struct {
	StoragePricePerGB  float64 `json:"storagePricePerGB"`
	DownloadPricePerGB float64 `json:"downloadPricePerGB"`
	UploadPricePerGB   float64 `json:"uploadPricePerGB"`
	RequestPricePer10K float64 `json:"requestPricePer10K"`
	MinimumStorageDays int     `json:"minimumStorageDays"`
	AvailabilitySLA    float64 `json:"availabilitySLA"`
}

// updateProviderPricing 更新提供商定价
// @Summary 更新存储提供商定价
// @Tags backup-cost
// @Router /backup/cost/providers/{provider} [put].
func (h *CostHandlers) updateProviderPricing(c *gin.Context) {
	provider := CloudProvider(c.Param("provider"))

	switch provider {
	case "local", CloudProviderS3, CloudProviderAliyun, CloudProviderWebDAV:
	default:
		api.BadRequest(c, "无效的存储提供商: "+string(provider))
		return
	}

	var req updateProviderPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	config := &StorageCostConfig{
		Provider:           provider,
		StoragePricePerGB:  req.StoragePricePerGB,
		DownloadPricePerGB: req.DownloadPricePerGB,
		UploadPricePerGB:   req.UploadPricePerGB,
		RequestPricePer10K: req.RequestPricePer10K,
		MinimumStorageDays: req.MinimumStorageDays,
		AvailabilitySLA:    req.AvailabilitySLA,
	}

	h.analyzer.SetCostConfig(provider, config)

	api.OKWithMessage(c, "定价配置更新成功", nil)
}

// getAlertThresholds 获取告警阈值
// @Summary 获取成本告警阈值
// @Tags backup-cost
// @Router /backup/cost/thresholds [get].
func (h *CostHandlers) getAlertThresholds(c *gin.Context) {
	thresholds := DefaultCostAlertThresholds()
	api.OK(c, thresholds)
}

// updateAlertThresholdsRequest 更新告警阈值请求.
type updateAlertThresholdsRequest struct {
	MonthlyCostWarning   float64 `json:"monthlyCostWarning"`
	MonthlyCostCritical  float64 `json:"monthlyCostCritical"`
	SingleBackupWarning  float64 `json:"singleBackupWarning"`
	SingleBackupCritical float64 `json:"singleBackupCritical"`
	StorageGrowthWarning float64 `json:"storageGrowthWarning"`
	MinCompressionRatio  float64 `json:"minCompressionRatio"`
}

// updateAlertThresholds 更新告警阈值
// @Summary 更新成本告警阈值
// @Tags backup-cost
// @Router /backup/cost/thresholds [put].
func (h *CostHandlers) updateAlertThresholds(c *gin.Context) {
	var req updateAlertThresholdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "参数解析失败: "+err.Error())
		return
	}

	thresholds := &CostAlertThresholds{
		MonthlyCostWarning:   req.MonthlyCostWarning,
		MonthlyCostCritical:  req.MonthlyCostCritical,
		SingleBackupWarning:  req.SingleBackupWarning,
		SingleBackupCritical: req.SingleBackupCritical,
		StorageGrowthWarning: req.StorageGrowthWarning,
		MinCompressionRatio:  req.MinCompressionRatio,
	}

	h.analyzer.SetAlertThresholds(thresholds)

	api.OKWithMessage(c, "告警阈值更新成功", nil)
}

// parseIntParam 解析整数参数.
func parseIntParam(s string, defaultValue int) (int, error) {
	if s == "" {
		return defaultValue, nil
	}

	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return defaultValue, err
	}

	return result, nil
}
