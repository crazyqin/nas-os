// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"runtime"
	"sort"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ResourceMonitorService 资源监控服务
type ResourceMonitorService struct {
	costAnalyzer     *EnhancedCostAnalyzer
	capacityAnalyzer *CapacityPlanningAnalyzer
	trendAnalyzer    *ResourceTrendAnalyzer
	reporter         *ResourceReporter
	config           ResourceMonitorConfig
}

// ResourceMonitorConfig 资源监控配置
type ResourceMonitorConfig struct {
	// 采集间隔（秒）
	CollectInterval int `json:"collect_interval"`

	// 历史数据保留天数
	HistoryRetentionDays int `json:"history_retention_days"`

	// 告警阈值
	StorageWarningThreshold    float64 `json:"storage_warning_threshold"`
	StorageCriticalThreshold   float64 `json:"storage_critical_threshold"`
	CPUWarningThreshold        float64 `json:"cpu_warning_threshold"`
	CPUCriticalThreshold       float64 `json:"cpu_critical_threshold"`
	MemoryWarningThreshold     float64 `json:"memory_warning_threshold"`
	MemoryCriticalThreshold    float64 `json:"memory_critical_threshold"`
	BandwidthWarningThreshold  float64 `json:"bandwidth_warning_threshold"`
	BandwidthCriticalThreshold float64 `json:"bandwidth_critical_threshold"`

	// 成本配置
	CostPerGBMonthly float64 `json:"cost_per_gb_monthly"`
}

// DefaultResourceMonitorConfig 默认配置
func DefaultResourceMonitorConfig() ResourceMonitorConfig {
	return ResourceMonitorConfig{
		CollectInterval:            60,
		HistoryRetentionDays:       30,
		StorageWarningThreshold:    70,
		StorageCriticalThreshold:   85,
		CPUWarningThreshold:        70,
		CPUCriticalThreshold:       90,
		MemoryWarningThreshold:     75,
		MemoryCriticalThreshold:    90,
		BandwidthWarningThreshold:  70,
		BandwidthCriticalThreshold: 85,
		CostPerGBMonthly:           0.5,
	}
}

// NewResourceMonitorService 创建资源监控服务
func NewResourceMonitorService(config ResourceMonitorConfig) *ResourceMonitorService {
	costConfig := StorageCostConfig{
		CostPerGBMonthly:      config.CostPerGBMonthly,
		ElectricityCostPerKWh: 0.6,
		DevicePowerWatts:      100,
		OpsCostMonthly:        500,
		DepreciationYears:     5,
		HardwareCost:          50000,
	}

	return &ResourceMonitorService{
		costAnalyzer:     NewEnhancedCostAnalyzer(costConfig),
		capacityAnalyzer: NewCapacityPlanningAnalyzer(costConfig),
		trendAnalyzer:    NewResourceTrendAnalyzer(),
		reporter:         NewResourceReporter(DefaultResourceReportConfig()),
		config:           config,
	}
}

// ResourceMonitorAPIHandlers 资源监控 API 处理器
type ResourceMonitorAPIHandlers struct {
	service *ResourceMonitorService
	// 数据获取函数
	getSystemMetrics    func() (*SystemMetricsData, error)
	getStorageMetrics   func() ([]StorageMetricsData, error)
	getNetworkMetrics   func() ([]NetworkMetricsData, error)
	getUserMetrics      func() ([]UserMetricsData, error)
	getProcessMetrics   func() ([]ProcessMetricsData, error)
	getBandwidthHistory func(iface string, duration time.Duration) ([]BandwidthHistoryPoint, error)
	getCapacityHistory  func(volume string, duration time.Duration) ([]CapacityHistory, error)
}

// SystemMetricsData 系统指标数据
type SystemMetricsData struct {
	// CPU
	CPUUsage  float64 `json:"cpu_usage"`
	CPUCount  int     `json:"cpu_count"`
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`
	CPUSteal  float64 `json:"cpu_steal"`
	CPUWait   float64 `json:"cpu_wait"`

	// 内存
	MemoryTotal   uint64  `json:"memory_total"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryFree    uint64  `json:"memory_free"`
	MemoryCached  uint64  `json:"memory_cached"`
	MemoryBuffers uint64  `json:"memory_buffers"`
	MemoryUsage   float64 `json:"memory_usage"`
	SwapTotal     uint64  `json:"swap_total"`
	SwapUsed      uint64  `json:"swap_used"`
	SwapUsage     float64 `json:"swap_usage"`

	// 系统
	Uptime        int64  `json:"uptime"`
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	KernelVersion string `json:"kernel_version"`
	Arch          string `json:"arch"`

	// 时间
	Timestamp time.Time `json:"timestamp"`
}

// StorageMetricsData 存储指标数据
type StorageMetricsData struct {
	VolumeName   string    `json:"volume_name"`
	Device       string    `json:"device"`
	MountPoint   string    `json:"mount_point"`
	TotalBytes   uint64    `json:"total_bytes"`
	UsedBytes    uint64    `json:"used_bytes"`
	FreeBytes    uint64    `json:"free_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	InodeTotal   uint64    `json:"inode_total"`
	InodeUsed    uint64    `json:"inode_used"`
	InodeFree    uint64    `json:"inode_free"`
	InodeUsage   float64   `json:"inode_usage"`
	ReadIOPS     uint64    `json:"read_iops"`
	WriteIOPS    uint64    `json:"write_iops"`
	ReadBytes    uint64    `json:"read_bytes"`
	WriteBytes   uint64    `json:"write_bytes"`
	ReadLatency  float64   `json:"read_latency_ms"`
	WriteLatency float64   `json:"write_latency_ms"`
	FileCount    uint64    `json:"file_count"`
	DirCount     uint64    `json:"dir_count"`
	Timestamp    time.Time `json:"timestamp"`
}

// NetworkMetricsData 网络指标数据
type NetworkMetricsData struct {
	Interface      string    `json:"interface"`
	RxBytes        uint64    `json:"rx_bytes"`
	TxBytes        uint64    `json:"tx_bytes"`
	RxPackets      uint64    `json:"rx_packets"`
	TxPackets      uint64    `json:"tx_packets"`
	RxErrors       uint64    `json:"rx_errors"`
	TxErrors       uint64    `json:"tx_errors"`
	RxDropped      uint64    `json:"rx_dropped"`
	TxDropped      uint64    `json:"tx_dropped"`
	RxBytesPerSec  uint64    `json:"rx_bytes_per_sec"`
	TxBytesPerSec  uint64    `json:"tx_bytes_per_sec"`
	BandwidthLimit uint64    `json:"bandwidth_limit"`
	Timestamp      time.Time `json:"timestamp"`
}

// UserMetricsData 用户指标数据
type UserMetricsData struct {
	Username     string     `json:"username"`
	UserID       string     `json:"user_id"`
	QuotaBytes   uint64     `json:"quota_bytes"`
	UsedBytes    uint64     `json:"used_bytes"`
	UsagePercent float64    `json:"usage_percent"`
	FileCount    uint64     `json:"file_count"`
	DirCount     uint64     `json:"dir_count"`
	LastAccess   *time.Time `json:"last_access,omitempty"`
	Timestamp    time.Time  `json:"timestamp"`
}

// ProcessMetricsData 进程指标数据
type ProcessMetricsData struct {
	PID        int       `json:"pid"`
	Name       string    `json:"name"`
	User       string    `json:"user"`
	CPUUsage   float64   `json:"cpu_usage"`
	MemoryRSS  uint64    `json:"memory_rss"`
	MemoryVMS  uint64    `json:"memory_vms"`
	Status     string    `json:"status"`
	CreateTime time.Time `json:"create_time"`
	Threads    int       `json:"threads"`
	FDCount    int       `json:"fd_count"`
}

// NewResourceMonitorAPIHandlers 创建资源监控 API 处理器
func NewResourceMonitorAPIHandlers(config ResourceMonitorConfig) *ResourceMonitorAPIHandlers {
	return &ResourceMonitorAPIHandlers{
		service: NewResourceMonitorService(config),
	}
}

// SetDataProviders 设置数据提供者
func (h *ResourceMonitorAPIHandlers) SetDataProviders(
	systemProvider func() (*SystemMetricsData, error),
	storageProvider func() ([]StorageMetricsData, error),
	networkProvider func() ([]NetworkMetricsData, error),
	userProvider func() ([]UserMetricsData, error),
	processProvider func() ([]ProcessMetricsData, error),
	bandwidthHistoryProvider func(iface string, duration time.Duration) ([]BandwidthHistoryPoint, error),
	capacityHistoryProvider func(volume string, duration time.Duration) ([]CapacityHistory, error),
) {
	h.getSystemMetrics = systemProvider
	h.getStorageMetrics = storageProvider
	h.getNetworkMetrics = networkProvider
	h.getUserMetrics = userProvider
	h.getProcessMetrics = processProvider
	h.getBandwidthHistory = bandwidthHistoryProvider
	h.getCapacityHistory = capacityHistoryProvider
}

// RegisterResourceMonitorRoutes 注册资源监控路由
func (h *ResourceMonitorAPIHandlers) RegisterResourceMonitorRoutes(apiGroup *gin.RouterGroup) {
	monitor := apiGroup.Group("/resource-monitor")
	{
		// 实时监控
		monitor.GET("/realtime", h.getRealtimeMetrics)
		monitor.GET("/realtime/system", h.getRealtimeSystemMetrics)
		monitor.GET("/realtime/storage", h.getRealtimeStorageMetrics)
		monitor.GET("/realtime/network", h.getRealtimeNetworkMetrics)
		monitor.GET("/realtime/processes", h.getRealtimeProcessMetrics)

		// 资源统计汇总
		monitor.GET("/summary", h.getResourceSummary)
		monitor.GET("/summary/storage", h.getStorageSummary)
		monitor.GET("/summary/users", h.getUserSummary)
		monitor.GET("/summary/cost", h.getCostSummary)

		// 容量分析
		monitor.GET("/capacity/:volume", h.getVolumeCapacityAnalysis)
		monitor.GET("/capacity/prediction", h.getCapacityPrediction)

		// 资源趋势
		monitor.GET("/trend", h.getResourceTrend)
		monitor.GET("/trend/storage", h.getStorageTrend)
		monitor.GET("/trend/bandwidth", h.getBandwidthTrend)

		// 资源告警
		monitor.GET("/alerts", h.getResourceAlerts)
		monitor.GET("/alerts/active", h.getActiveAlerts)
		monitor.GET("/alerts/history", h.getAlertHistory)

		// 成本分析
		monitor.POST("/cost/analyze", h.analyzeCost)
		monitor.POST("/cost/forecast", h.forecastCost)
		monitor.GET("/cost/optimization", h.getCostOptimization)

		// 资源使用排行
		monitor.GET("/ranking/users", h.getUserRanking)
		monitor.GET("/ranking/volumes", h.getVolumeRanking)
		monitor.GET("/ranking/processes", h.getProcessRanking)

		// 健康评分
		monitor.GET("/health", h.getResourceHealth)
		monitor.GET("/health/score", h.getHealthScore)

		// 配置
		monitor.GET("/config", h.getMonitorConfig)
		monitor.PUT("/config", h.updateMonitorConfig)
	}
}

// ========== 实时监控 API ==========

// getRealtimeMetrics 获取实时资源指标
func (h *ResourceMonitorAPIHandlers) getRealtimeMetrics(c *gin.Context) {
	response := gin.H{
		"generated_at": time.Now(),
		"system":       nil,
		"storage":      nil,
		"network":      nil,
		"health_score": 0,
	}

	// 获取系统指标
	if h.getSystemMetrics != nil {
		if system, err := h.getSystemMetrics(); err == nil {
			response["system"] = system
		}
	}

	// 获取存储指标
	if h.getStorageMetrics != nil {
		if storage, err := h.getStorageMetrics(); err == nil {
			response["storage"] = storage
		}
	}

	// 获取网络指标
	if h.getNetworkMetrics != nil {
		if network, err := h.getNetworkMetrics(); err == nil {
			response["network"] = network
		}
	}

	// 计算健康评分
	response["health_score"] = h.calculateHealthScore(response)

	api.OK(c, response)
}

// getRealtimeSystemMetrics 获取实时系统指标
func (h *ResourceMonitorAPIHandlers) getRealtimeSystemMetrics(c *gin.Context) {
	if h.getSystemMetrics == nil {
		// 返回默认/模拟数据
		api.OK(c, h.getDefaultSystemMetrics())
		return
	}

	metrics, err := h.getSystemMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, metrics)
}

// getRealtimeStorageMetrics 获取实时存储指标
func (h *ResourceMonitorAPIHandlers) getRealtimeStorageMetrics(c *gin.Context) {
	if h.getStorageMetrics == nil {
		api.OK(c, []StorageMetricsData{})
		return
	}

	metrics, err := h.getStorageMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, metrics)
}

// getRealtimeNetworkMetrics 获取实时网络指标
func (h *ResourceMonitorAPIHandlers) getRealtimeNetworkMetrics(c *gin.Context) {
	if h.getNetworkMetrics == nil {
		api.OK(c, []NetworkMetricsData{})
		return
	}

	metrics, err := h.getNetworkMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, metrics)
}

// getRealtimeProcessMetrics 获取实时进程指标
func (h *ResourceMonitorAPIHandlers) getRealtimeProcessMetrics(c *gin.Context) {
	if h.getProcessMetrics == nil {
		api.OK(c, []ProcessMetricsData{})
		return
	}

	metrics, err := h.getProcessMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 支持排序
	sortBy := c.Query("sort")
	if sortBy == "cpu" {
		// 按 CPU 排序已在外部处理
	}

	// 支持限制返回数量
	limit := 20
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
		if limit > len(metrics) {
			limit = len(metrics)
		}
	}

	api.OK(c, metrics[:limit])
}

// ========== 资源统计汇总 API ==========

// getResourceSummary 获取资源统计汇总
func (h *ResourceMonitorAPIHandlers) getResourceSummary(c *gin.Context) {
	summary := gin.H{
		"generated_at": time.Now(),
		"system":       h.getSystemSummary(),
		"storage":      h.getStorageSummaryData(),
		"users":        h.getUserSummaryData(),
		"alerts":       h.getActiveAlertsData(),
		"cost":         nil,
	}

	// 成本汇总
	if costSummary, err := h.getCostSummaryData(); err == nil {
		summary["cost"] = costSummary
	}

	api.OK(c, summary)
}

// getStorageSummary 获取存储汇总
func (h *ResourceMonitorAPIHandlers) getStorageSummary(c *gin.Context) {
	api.OK(c, h.getStorageSummaryData())
}

// getUserSummary 获取用户汇总
func (h *ResourceMonitorAPIHandlers) getUserSummary(c *gin.Context) {
	api.OK(c, h.getUserSummaryData())
}

// getCostSummary 获取成本汇总
func (h *ResourceMonitorAPIHandlers) getCostSummary(c *gin.Context) {
	summary, err := h.getCostSummaryData()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OK(c, summary)
}

// ========== 容量分析 API ==========

// getVolumeCapacityAnalysis 获取卷容量分析
func (h *ResourceMonitorAPIHandlers) getVolumeCapacityAnalysis(c *gin.Context) {
	volumeName := c.Param("volume")

	if h.getCapacityHistory == nil {
		api.BadRequest(c, "容量历史数据源未配置")
		return
	}

	// 获取过去 90 天的历史数据
	history, err := h.getCapacityHistory(volumeName, 90*24*time.Hour)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	analysis := h.service.capacityAnalyzer.AnalyzeCapacityEnhanced(history, volumeName, 6)
	if analysis == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, analysis)
}

// getCapacityPrediction 获取容量预测
func (h *ResourceMonitorAPIHandlers) getCapacityPrediction(c *gin.Context) {
	volume := c.Query("volume")
	months := 6
	if m := c.Query("months"); m != "" {
		_, _ = fmt.Sscanf(m, "%d", &months)
	}

	if h.getCapacityHistory == nil {
		api.BadRequest(c, "容量历史数据源未配置")
		return
	}

	history, err := h.getCapacityHistory(volume, 180*24*time.Hour)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	analysis := h.service.capacityAnalyzer.AnalyzeCapacityEnhanced(history, volume, months)
	if analysis == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	prediction := gin.H{
		"volume_name":       volume,
		"prediction_months": months,
		"current_usage":     analysis.Current.UsagePercent,
		"scenarios":         analysis.Scenarios,
		"timeline":          analysis.ExpansionTimeline,
		"risks":             analysis.Risks,
		"cost_impact":       analysis.CostImpact,
		"generated_at":      time.Now(),
	}

	api.OK(c, prediction)
}

// ========== 资源趋势 API ==========

// getResourceTrend 获取资源趋势
func (h *ResourceMonitorAPIHandlers) getResourceTrend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	trend := gin.H{
		"generated_at": time.Now(),
		"period_days":  days,
		"storage":      nil,
		"bandwidth":    nil,
	}

	// 存储趋势
	if storageTrend, err := h.getStorageTrendData(days); err == nil {
		trend["storage"] = storageTrend
	}

	// 带宽趋势
	if bandwidthTrend, err := h.getBandwidthTrendData(days); err == nil {
		trend["bandwidth"] = bandwidthTrend
	}

	api.OK(c, trend)
}

// getStorageTrend 获取存储趋势
func (h *ResourceMonitorAPIHandlers) getStorageTrend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	trend, err := h.getStorageTrendData(days)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, trend)
}

// getBandwidthTrend 获取带宽趋势
func (h *ResourceMonitorAPIHandlers) getBandwidthTrend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	trend, err := h.getBandwidthTrendData(days)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, trend)
}

// ========== 资源告警 API ==========

// getResourceAlerts 获取资源告警
func (h *ResourceMonitorAPIHandlers) getResourceAlerts(c *gin.Context) {
	alerts := h.getActiveAlertsData()
	api.OK(c, alerts)
}

// getActiveAlerts 获取活跃告警
func (h *ResourceMonitorAPIHandlers) getActiveAlerts(c *gin.Context) {
	alerts := h.getActiveAlertsData()

	// 过滤只返回活跃告警
	active := make([]ResourceAlert, 0)
	for _, alert := range alerts {
		if alert.Status == "active" || alert.Status == "" {
			active = append(active, alert)
		}
	}

	api.OK(c, active)
}

// getAlertHistory 获取告警历史
func (h *ResourceMonitorAPIHandlers) getAlertHistory(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	// 返回历史告警（简化实现）
	history := gin.H{
		"period_days": days,
		"alerts":      []ResourceAlert{},
		"summary": gin.H{
			"total":    0,
			"critical": 0,
			"warning":  0,
			"resolved": 0,
		},
	}

	api.OK(c, history)
}

// ========== 成本分析 API ==========

// analyzeCost 分析成本
func (h *ResourceMonitorAPIHandlers) analyzeCost(c *gin.Context) {
	var req struct {
		VolumeMetrics []StorageMetrics     `json:"volume_metrics"`
		UserUsages    []UserStorageUsage   `json:"user_usages"`
		History       []CostTrendDataPoint `json:"history"`
		StartTime     *time.Time           `json:"start_time"`
		EndTime       *time.Time           `json:"end_time"`
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

	report := h.service.costAnalyzer.Analyze(
		req.VolumeMetrics,
		req.UserUsages,
		req.History,
		period,
	)

	api.OK(c, report)
}

// forecastCost 成本预测
func (h *ResourceMonitorAPIHandlers) forecastCost(c *gin.Context) {
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

	forecast := h.service.costAnalyzer.ForecastEnhanced(req.History, months)
	if forecast == nil {
		api.BadRequest(c, "历史数据不足")
		return
	}

	api.OK(c, forecast)
}

// getCostOptimization 获取成本优化建议
func (h *ResourceMonitorAPIHandlers) getCostOptimization(c *gin.Context) {
	// 返回通用优化建议
	recommendations := []gin.H{
		{
			"type":                      "cleanup",
			"priority":                  "high",
			"title":                     "清理重复文件",
			"description":               "扫描并删除重复文件，释放存储空间",
			"potential_savings_percent": 15,
			"effort":                    "low",
		},
		{
			"type":                      "cleanup",
			"priority":                  "high",
			"title":                     "清理过期数据",
			"description":               "根据保留策略清理过期备份和临时文件",
			"potential_savings_percent": 20,
			"effort":                    "low",
		},
		{
			"type":                      "compression",
			"priority":                  "medium",
			"title":                     "启用数据压缩",
			"description":               "对适合的数据类型启用压缩",
			"potential_savings_percent": 30,
			"effort":                    "medium",
		},
		{
			"type":                      "deduplication",
			"priority":                  "medium",
			"title":                     "启用数据去重",
			"description":               "启用块级或文件级去重",
			"potential_savings_percent": 25,
			"effort":                    "medium",
		},
		{
			"type":                      "tiering",
			"priority":                  "low",
			"title":                     "实施分层存储",
			"description":               "将冷数据迁移到低成本存储",
			"potential_savings_percent": 50,
			"effort":                    "high",
		},
	}

	api.OK(c, recommendations)
}

// ========== 资源排行 API ==========

// getUserRanking 获取用户排行
func (h *ResourceMonitorAPIHandlers) getUserRanking(c *gin.Context) {
	if h.getUserMetrics == nil {
		api.OK(c, []UserMetricsData{})
		return
	}

	metrics, err := h.getUserMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 已按使用量排序
	limit := 10
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	if limit > len(metrics) {
		limit = len(metrics)
	}

	// 转换为排行格式
	ranking := make([]gin.H, 0, limit)
	for i, m := range metrics[:limit] {
		ranking = append(ranking, gin.H{
			"rank":          i + 1,
			"username":      m.Username,
			"used_bytes":    m.UsedBytes,
			"used_gb":       float64(m.UsedBytes) / (1024 * 1024 * 1024),
			"quota_bytes":   m.QuotaBytes,
			"usage_percent": m.UsagePercent,
			"file_count":    m.FileCount,
		})
	}

	api.OK(c, ranking)
}

// getVolumeRanking 获取卷排行
func (h *ResourceMonitorAPIHandlers) getVolumeRanking(c *gin.Context) {
	if h.getStorageMetrics == nil {
		api.OK(c, []StorageMetricsData{})
		return
	}

	metrics, err := h.getStorageMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	if limit > len(metrics) {
		limit = len(metrics)
	}

	// 按使用率排序
	ranking := make([]gin.H, 0, limit)
	for i, m := range metrics[:limit] {
		ranking = append(ranking, gin.H{
			"rank":          i + 1,
			"volume_name":   m.VolumeName,
			"total_bytes":   m.TotalBytes,
			"total_gb":      float64(m.TotalBytes) / (1024 * 1024 * 1024),
			"used_bytes":    m.UsedBytes,
			"used_gb":       float64(m.UsedBytes) / (1024 * 1024 * 1024),
			"usage_percent": m.UsagePercent,
			"iops":          m.ReadIOPS + m.WriteIOPS,
		})
	}

	api.OK(c, ranking)
}

// getProcessRanking 获取进程排行
func (h *ResourceMonitorAPIHandlers) getProcessRanking(c *gin.Context) {
	if h.getProcessMetrics == nil {
		api.OK(c, []ProcessMetricsData{})
		return
	}

	metrics, err := h.getProcessMetrics()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 按指定字段排序
	sortBy := c.Query("sort")
	if sortBy == "" {
		sortBy = "cpu"
	}

	// 根据排序字段进行排序
	sort.Slice(metrics, func(i, j int) bool {
		switch sortBy {
		case "pid":
			return metrics[i].PID < metrics[j].PID
		case "name":
			return metrics[i].Name < metrics[j].Name
		case "user":
			return metrics[i].User < metrics[j].User
		case "memory", "mem":
			return metrics[i].MemoryRSS > metrics[j].MemoryRSS
		case "memory_vms":
			return metrics[i].MemoryVMS > metrics[j].MemoryVMS
		case "threads":
			return metrics[i].Threads > metrics[j].Threads
		case "fd":
			return metrics[i].FDCount > metrics[j].FDCount
		case "cpu", "cpu_usage":
			fallthrough
		default:
			return metrics[i].CPUUsage > metrics[j].CPUUsage
		}
	})

	limit := 20
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	if limit > len(metrics) {
		limit = len(metrics)
	}

	ranking := make([]gin.H, 0, limit)
	for i, p := range metrics[:limit] {
		ranking = append(ranking, gin.H{
			"rank":          i + 1,
			"pid":           p.PID,
			"name":          p.Name,
			"user":          p.User,
			"cpu_usage":     p.CPUUsage,
			"memory_rss":    p.MemoryRSS,
			"memory_rss_mb": float64(p.MemoryRSS) / (1024 * 1024),
			"threads":       p.Threads,
		})
	}

	api.OK(c, ranking)
}

// ========== 健康评分 API ==========

// getResourceHealth 获取资源健康状态
func (h *ResourceMonitorAPIHandlers) getResourceHealth(c *gin.Context) {
	health := gin.H{
		"generated_at": time.Now(),
		"score":        0,
		"grade":        "unknown",
		"components": gin.H{
			"storage": gin.H{"score": 0, "status": "unknown"},
			"memory":  gin.H{"score": 0, "status": "unknown"},
			"cpu":     gin.H{"score": 0, "status": "unknown"},
			"network": gin.H{"score": 0, "status": "unknown"},
		},
		"issues": []string{},
	}

	score := 100
	issues := make([]string, 0)

	// 检查存储
	if h.getStorageMetrics != nil {
		if storage, err := h.getStorageMetrics(); err == nil {
			storageScore, storageIssues := h.evaluateStorageHealth(storage)
			if components, ok := health["components"].(gin.H); ok {
				components["storage"] = gin.H{
					"score":  storageScore,
					"status": h.scoreToStatus(storageScore),
				}
			}
			score = score - (100 - storageScore)
			issues = append(issues, storageIssues...)
		}
	}

	// 检查系统（CPU/内存）
	if h.getSystemMetrics != nil {
		if system, err := h.getSystemMetrics(); err == nil {
			cpuScore, memScore, systemIssues := h.evaluateSystemHealth(system)
			if components, ok := health["components"].(gin.H); ok {
				components["cpu"] = gin.H{
					"score":  cpuScore,
					"status": h.scoreToStatus(cpuScore),
				}
				components["memory"] = gin.H{
					"score":  memScore,
					"status": h.scoreToStatus(memScore),
				}
			}
			issues = append(issues, systemIssues...)
		}
	}

	if score < 0 {
		score = 0
	}

	health["score"] = score
	health["grade"] = h.scoreToGrade(score)
	health["issues"] = issues

	api.OK(c, health)
}

// getHealthScore 获取健康评分
func (h *ResourceMonitorAPIHandlers) getHealthScore(c *gin.Context) {
	data := h.getRealtimeMetricsData()
	score := h.calculateHealthScore(data)

	api.OK(c, gin.H{
		"score":        score,
		"grade":        h.scoreToGrade(score),
		"generated_at": time.Now(),
	})
}

// ========== 配置 API ==========

// getMonitorConfig 获取监控配置
func (h *ResourceMonitorAPIHandlers) getMonitorConfig(c *gin.Context) {
	api.OK(c, h.service.config)
}

// updateMonitorConfig 更新监控配置
func (h *ResourceMonitorAPIHandlers) updateMonitorConfig(c *gin.Context) {
	var config ResourceMonitorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.service.config = config
	api.OK(c, gin.H{"message": "配置已更新"})
}

// ========== 辅助方法 ==========

// getDefaultSystemMetrics 获取默认系统指标
func (h *ResourceMonitorAPIHandlers) getDefaultSystemMetrics() *SystemMetricsData {
	return &SystemMetricsData{
		CPUCount:    runtime.NumCPU(),
		CPUUsage:    0,
		MemoryTotal: 0,
		MemoryUsed:  0,
		MemoryUsage: 0,
		Timestamp:   time.Now(),
	}
}

// getRealtimeMetricsData 获取实时指标数据
func (h *ResourceMonitorAPIHandlers) getRealtimeMetricsData() gin.H {
	data := gin.H{
		"system":  nil,
		"storage": nil,
		"network": nil,
	}

	if h.getSystemMetrics != nil {
		if system, err := h.getSystemMetrics(); err == nil {
			data["system"] = system
		}
	}

	if h.getStorageMetrics != nil {
		if storage, err := h.getStorageMetrics(); err == nil {
			data["storage"] = storage
		}
	}

	if h.getNetworkMetrics != nil {
		if network, err := h.getNetworkMetrics(); err == nil {
			data["network"] = network
		}
	}

	return data
}

// calculateHealthScore 计算健康评分
func (h *ResourceMonitorAPIHandlers) calculateHealthScore(data gin.H) int {
	score := 100

	// CPU 评分
	if system, ok := data["system"].(*SystemMetricsData); ok {
		if system.CPUUsage > h.service.config.CPUCriticalThreshold {
			score -= 30
		} else if system.CPUUsage > h.service.config.CPUWarningThreshold {
			score -= 15
		}

		if system.MemoryUsage > h.service.config.MemoryCriticalThreshold {
			score -= 30
		} else if system.MemoryUsage > h.service.config.MemoryWarningThreshold {
			score -= 15
		}
	}

	// 存储评分
	if storage, ok := data["storage"].([]StorageMetricsData); ok {
		for _, s := range storage {
			if s.UsagePercent > h.service.config.StorageCriticalThreshold {
				score -= 20
			} else if s.UsagePercent > h.service.config.StorageWarningThreshold {
				score -= 10
			}
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// scoreToGrade 评分转等级
func (h *ResourceMonitorAPIHandlers) scoreToGrade(score int) string {
	if score >= 90 {
		return "A"
	} else if score >= 80 {
		return "B"
	} else if score >= 70 {
		return "C"
	} else if score >= 60 {
		return "D"
	}
	return "F"
}

// scoreToStatus 评分转状态
func (h *ResourceMonitorAPIHandlers) scoreToStatus(score int) string {
	if score >= 80 {
		return "healthy"
	} else if score >= 60 {
		return "warning"
	}
	return "critical"
}

// getSystemSummary 获取系统汇总
func (h *ResourceMonitorAPIHandlers) getSystemSummary() gin.H {
	summary := gin.H{
		"cpu_usage":    0,
		"memory_usage": 0,
		"load_avg":     []float64{0, 0, 0},
		"uptime":       0,
	}

	if h.getSystemMetrics != nil {
		if system, err := h.getSystemMetrics(); err == nil {
			summary["cpu_usage"] = system.CPUUsage
			summary["memory_usage"] = system.MemoryUsage
			summary["load_avg"] = []float64{system.LoadAvg1, system.LoadAvg5, system.LoadAvg15}
			summary["uptime"] = system.Uptime
		}
	}

	return summary
}

// getStorageSummaryData 获取存储汇总数据
func (h *ResourceMonitorAPIHandlers) getStorageSummaryData() gin.H {
	summary := gin.H{
		"total_volumes": 0,
		"total_bytes":   uint64(0),
		"used_bytes":    uint64(0),
		"usage_percent": float64(0),
		"volumes":       []gin.H{},
	}

	if h.getStorageMetrics == nil {
		return summary
	}

	metrics, err := h.getStorageMetrics()
	if err != nil {
		return summary
	}

	var totalBytes, usedBytes uint64
	volumes := make([]gin.H, 0)

	for _, m := range metrics {
		totalBytes += m.TotalBytes
		usedBytes += m.UsedBytes

		volumes = append(volumes, gin.H{
			"name":          m.VolumeName,
			"total_bytes":   m.TotalBytes,
			"used_bytes":    m.UsedBytes,
			"usage_percent": m.UsagePercent,
		})
	}

	summary["total_volumes"] = len(metrics)
	summary["total_bytes"] = totalBytes
	summary["used_bytes"] = usedBytes
	summary["volumes"] = volumes

	if totalBytes > 0 {
		summary["usage_percent"] = float64(usedBytes) / float64(totalBytes) * 100
	}

	return summary
}

// getUserSummaryData 获取用户汇总数据
func (h *ResourceMonitorAPIHandlers) getUserSummaryData() gin.H {
	summary := gin.H{
		"total_users": 0,
		"total_used":  uint64(0),
		"total_quota": uint64(0),
		"over_soft":   0,
		"over_hard":   0,
		"top_users":   []gin.H{},
	}

	if h.getUserMetrics == nil {
		return summary
	}

	metrics, err := h.getUserMetrics()
	if err != nil {
		return summary
	}

	var totalUsed, totalQuota uint64
	var overSoft, overHard int
	topUsers := make([]gin.H, 0)

	for i, m := range metrics {
		totalUsed += m.UsedBytes
		totalQuota += m.QuotaBytes

		if m.UsagePercent >= 100 {
			overHard++
		} else if m.UsagePercent >= 80 {
			overSoft++
		}

		if i < 10 {
			topUsers = append(topUsers, gin.H{
				"username":      m.Username,
				"used_bytes":    m.UsedBytes,
				"usage_percent": m.UsagePercent,
			})
		}
	}

	summary["total_users"] = len(metrics)
	summary["total_used"] = totalUsed
	summary["total_quota"] = totalQuota
	summary["over_soft"] = overSoft
	summary["over_hard"] = overHard
	summary["top_users"] = topUsers

	return summary
}

// getCostSummaryData 获取成本汇总数据
func (h *ResourceMonitorAPIHandlers) getCostSummaryData() (gin.H, error) {
	if h.getStorageMetrics == nil {
		return gin.H{
			"monthly_cost":      0,
			"cost_per_gb":       h.service.config.CostPerGBMonthly,
			"potential_savings": 0,
		}, nil
	}

	metrics, err := h.getStorageMetrics()
	if err != nil {
		return nil, err
	}

	var totalGB float64
	for _, m := range metrics {
		totalGB += float64(m.TotalBytes) / (1024 * 1024 * 1024)
	}

	monthlyCost := totalGB * h.service.config.CostPerGBMonthly

	return gin.H{
		"monthly_cost":      round(monthlyCost, 2),
		"total_gb":          round(totalGB, 2),
		"cost_per_gb":       h.service.config.CostPerGBMonthly,
		"potential_savings": round(monthlyCost*0.2, 2), // 假设 20% 可优化
	}, nil
}

// getActiveAlertsData 获取活跃告警数据
func (h *ResourceMonitorAPIHandlers) getActiveAlertsData() []ResourceAlert {
	alerts := make([]ResourceAlert, 0)
	now := time.Now()

	// 检查存储告警
	if h.getStorageMetrics != nil {
		if metrics, err := h.getStorageMetrics(); err == nil {
			for _, m := range metrics {
				if m.UsagePercent >= h.service.config.StorageCriticalThreshold {
					alerts = append(alerts, ResourceAlert{
						ID:           fmt.Sprintf("storage_critical_%s", m.VolumeName),
						Type:         "storage",
						Severity:     "critical",
						Title:        fmt.Sprintf("卷 %s 存储空间严重告警", m.VolumeName),
						Message:      fmt.Sprintf("卷 %s 使用率达到 %.1f%%", m.VolumeName, m.UsagePercent),
						CurrentValue: m.UsagePercent,
						Threshold:    h.service.config.StorageCriticalThreshold,
						Unit:         "%",
						TriggeredAt:  now,
						Resource:     m.VolumeName,
						Status:       "active",
					})
				} else if m.UsagePercent >= h.service.config.StorageWarningThreshold {
					alerts = append(alerts, ResourceAlert{
						ID:           fmt.Sprintf("storage_warning_%s", m.VolumeName),
						Type:         "storage",
						Severity:     "warning",
						Title:        fmt.Sprintf("卷 %s 存储空间警告", m.VolumeName),
						Message:      fmt.Sprintf("卷 %s 使用率达到 %.1f%%", m.VolumeName, m.UsagePercent),
						CurrentValue: m.UsagePercent,
						Threshold:    h.service.config.StorageWarningThreshold,
						Unit:         "%",
						TriggeredAt:  now,
						Resource:     m.VolumeName,
						Status:       "active",
					})
				}
			}
		}
	}

	// 检查系统告警
	if h.getSystemMetrics != nil {
		if system, err := h.getSystemMetrics(); err == nil {
			if system.CPUUsage >= h.service.config.CPUCriticalThreshold {
				alerts = append(alerts, ResourceAlert{
					ID:           "cpu_critical",
					Type:         "cpu",
					Severity:     "critical",
					Title:        "CPU 使用率严重告警",
					Message:      fmt.Sprintf("CPU 使用率达到 %.1f%%", system.CPUUsage),
					CurrentValue: system.CPUUsage,
					Threshold:    h.service.config.CPUCriticalThreshold,
					Unit:         "%",
					TriggeredAt:  now,
					Resource:     "system",
					Status:       "active",
				})
			}

			if system.MemoryUsage >= h.service.config.MemoryCriticalThreshold {
				alerts = append(alerts, ResourceAlert{
					ID:           "memory_critical",
					Type:         "memory",
					Severity:     "critical",
					Title:        "内存使用率严重告警",
					Message:      fmt.Sprintf("内存使用率达到 %.1f%%", system.MemoryUsage),
					CurrentValue: system.MemoryUsage,
					Threshold:    h.service.config.MemoryCriticalThreshold,
					Unit:         "%",
					TriggeredAt:  now,
					Resource:     "system",
					Status:       "active",
				})
			}
		}
	}

	return alerts
}

// getStorageTrendData 获取存储趋势数据
func (h *ResourceMonitorAPIHandlers) getStorageTrendData(days int) (gin.H, error) {
	if h.getCapacityHistory == nil {
		return gin.H{"error": "数据源未配置"}, nil
	}

	// 获取历史数据
	history, err := h.getCapacityHistory("", time.Duration(days)*24*time.Hour)
	if err != nil {
		return nil, err
	}

	metrics := h.service.trendAnalyzer.analyzeStorageTrend(history)

	return gin.H{
		"trend_direction":     metrics.TrendDirection,
		"growth_rate":         metrics.GrowthRate,
		"days_to_full":        metrics.DaysToFull,
		"projected_full_date": metrics.ProjectedFullDate,
		"volatility":          metrics.Volatility,
	}, nil
}

// getBandwidthTrendData 获取带宽趋势数据
func (h *ResourceMonitorAPIHandlers) getBandwidthTrendData(days int) (gin.H, error) {
	if h.getBandwidthHistory == nil {
		return gin.H{"error": "数据源未配置"}, nil
	}

	// 获取历史数据
	history, err := h.getBandwidthHistory("", time.Duration(days)*24*time.Hour)
	if err != nil {
		return nil, err
	}

	metrics := h.service.trendAnalyzer.analyzeBandwidthTrend(history)

	return gin.H{
		"avg_read_mbps":   metrics.AvgReadMbps,
		"avg_write_mbps":  metrics.AvgWriteMbps,
		"peak_read_mbps":  metrics.PeakReadMbps,
		"peak_write_mbps": metrics.PeakWriteMbps,
		"saturation_risk": metrics.SaturationRisk,
	}, nil
}

// evaluateStorageHealth 评估存储健康
func (h *ResourceMonitorAPIHandlers) evaluateStorageHealth(metrics []StorageMetricsData) (int, []string) {
	score := 100
	issues := make([]string, 0)

	for _, m := range metrics {
		if m.UsagePercent >= h.service.config.StorageCriticalThreshold {
			score -= 20
			issues = append(issues, fmt.Sprintf("卷 %s 使用率过高 (%.1f%%)", m.VolumeName, m.UsagePercent))
		} else if m.UsagePercent >= h.service.config.StorageWarningThreshold {
			score -= 10
			issues = append(issues, fmt.Sprintf("卷 %s 使用率较高 (%.1f%%)", m.VolumeName, m.UsagePercent))
		}

		if m.InodeUsage >= 90 {
			score -= 10
			issues = append(issues, fmt.Sprintf("卷 %s inode 使用率过高", m.VolumeName))
		}
	}

	if score < 0 {
		score = 0
	}

	return score, issues
}

// evaluateSystemHealth 评估系统健康
func (h *ResourceMonitorAPIHandlers) evaluateSystemHealth(system *SystemMetricsData) (int, int, []string) {
	cpuScore := 100
	memScore := 100
	issues := make([]string, 0)

	// CPU 评估
	if system.CPUUsage >= h.service.config.CPUCriticalThreshold {
		cpuScore = 30
		issues = append(issues, fmt.Sprintf("CPU 使用率过高 (%.1f%%)", system.CPUUsage))
	} else if system.CPUUsage >= h.service.config.CPUWarningThreshold {
		cpuScore = 70
		issues = append(issues, fmt.Sprintf("CPU 使用率较高 (%.1f%%)", system.CPUUsage))
	}

	// 内存评估
	if system.MemoryUsage >= h.service.config.MemoryCriticalThreshold {
		memScore = 30
		issues = append(issues, fmt.Sprintf("内存使用率过高 (%.1f%%)", system.MemoryUsage))
	} else if system.MemoryUsage >= h.service.config.MemoryWarningThreshold {
		memScore = 70
		issues = append(issues, fmt.Sprintf("内存使用率较高 (%.1f%%)", system.MemoryUsage))
	}

	// 负载评估
	if system.LoadAvg1 > float64(runtime.NumCPU()) {
		cpuScore -= 10
		issues = append(issues, "系统负载较高")
	}

	return cpuScore, memScore, issues
}
