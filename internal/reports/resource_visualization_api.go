// Package reports 提供报表生成和管理功能
package reports

import (
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ResourceVisualizationHandlers 资源可视化 API 处理器
type ResourceVisualizationHandlers struct {
	reporter          *ResourceReporter
	costCalculator    *StorageCostCalculator
	bandwidthReporter *BandwidthReporter
	capacityPlanner   *CapacityPlanner
}

// NewResourceVisualizationHandlers 创建资源可视化 API 处理器
func NewResourceVisualizationHandlers() *ResourceVisualizationHandlers {
	return &ResourceVisualizationHandlers{
		reporter:          NewResourceReporter(DefaultResourceReportConfig()),
		costCalculator:    NewStorageCostCalculator(DefaultStorageCostConfig()),
		bandwidthReporter: NewBandwidthReporter(DefaultBandwidthReportConfig()),
		capacityPlanner:   NewCapacityPlanner(DefaultCapacityPlanningConfig()),
	}
}

// DefaultStorageCostConfig 默认存储成本配置
func DefaultStorageCostConfig() StorageCostConfig {
	return StorageCostConfig{
		CostPerGBMonthly:        0.5,
		CostPerIOPSMonthly:      0.01,
		CostPerBandwidthMonthly: 1.0,
		ElectricityCostPerKWh:   0.6,
		DevicePowerWatts:        100,
		OpsCostMonthly:          500,
		DepreciationYears:       5,
		HardwareCost:            50000,
	}
}

// DefaultBandwidthReportConfig 默认带宽报告配置
func DefaultBandwidthReportConfig() BandwidthReportConfig {
	return BandwidthReportConfig{
		BandwidthLimitMbps:           1000,
		HighUtilizationThreshold:     70.0,
		CriticalUtilizationThreshold: 90.0,
		ErrorRateThreshold:           1.0,
		DropRateThreshold:            0.5,
		TrendSampleInterval:          5,
	}
}

// DefaultCapacityPlanningConfig 默认容量规划配置
func DefaultCapacityPlanningConfig() CapacityPlanningConfig {
	return CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
		GrowthModel:       GrowthModelLinear,
		ExpansionLeadTime: 30,
		SafetyBuffer:      20.0,
	}
}

// RegisterResourceVisualizationRoutes 注册资源可视化路由
func (h *ResourceVisualizationHandlers) RegisterResourceVisualizationRoutes(apiGroup *gin.RouterGroup) {
	// ========== 资源总览 ==========
	overview := apiGroup.Group("/resource-visualization")
	{
		overview.GET("/overview", h.getResourceOverview)
		overview.GET("/dashboard", h.getDashboard)
		overview.GET("/summary", h.getResourceSummary)
	}

	// ========== 存储可视化 ==========
	storage := apiGroup.Group("/storage-visualization")
	{
		storage.GET("/overview", h.getStorageVisualization)
		storage.GET("/volumes", h.getVolumeVisualization)
		storage.GET("/volumes/:name", h.getVolumeDetailVisualization)
		storage.GET("/distribution", h.getStorageDistributionViz)
		storage.GET("/efficiency", h.getStorageEfficiency)
		storage.GET("/trend", h.getStorageTrendViz)
		storage.GET("/prediction", h.getStoragePrediction)
		storage.GET("/charts", h.getStorageCharts)
	}

	// ========== 带宽可视化 ==========
	bandwidth := apiGroup.Group("/bandwidth-visualization")
	{
		bandwidth.GET("/overview", h.getBandwidthVisualization)
		bandwidth.GET("/interfaces", h.getInterfaceVisualization)
		bandwidth.GET("/interfaces/:name", h.getInterfaceDetailVisualization)
		bandwidth.GET("/trend", h.getBandwidthTrendViz)
		bandwidth.GET("/peak", h.getBandwidthPeak)
		bandwidth.GET("/charts", h.getBandwidthCharts)
	}

	// ========== 用户资源可视化 ==========
	user := apiGroup.Group("/user-resource-visualization")
	{
		user.GET("/overview", h.getUserResourceVisualization)
		user.GET("/top-users", h.getTopUsersVisualization)
		user.GET("/users/:username", h.getUserDetailVisualization)
		user.GET("/quota-status", h.getQuotaStatusVisualization)
		user.GET("/charts", h.getUserResourceCharts)
	}

	// ========== 系统资源可视化 ==========
	system := apiGroup.Group("/system-visualization")
	{
		system.GET("/overview", h.getSystemVisualization)
		system.GET("/cpu", h.getCPUVisualization)
		system.GET("/memory", h.getMemoryVisualization)
		system.GET("/disk-io", h.getDiskIOVisualization)
		system.GET("/load", h.getLoadVisualization)
		system.GET("/charts", h.getSystemCharts)
	}

	// ========== 告警和建议 ==========
	alerts := apiGroup.Group("/resource-alerts")
	{
		alerts.GET("", h.getResourceAlerts)
		alerts.GET("/storage", h.getStorageAlerts)
		alerts.GET("/bandwidth", h.getBandwidthAlertsViz)
		alerts.GET("/user", h.getUserAlerts)
		alerts.GET("/recommendations", h.getResourceRecommendations)
	}

	// ========== 导出 ==========
	export := apiGroup.Group("/resource-export")
	{
		export.POST("/report", h.exportResourceReport)
		export.POST("/dashboard", h.exportDashboard)
	}
}

// ========== 资源总览 API ==========

func (h *ResourceVisualizationHandlers) getResourceOverview(c *gin.Context) {
	// 收集数据（实际应从存储模块获取）
	storageMetrics := h.collectStorageMetrics()
	bandwidthHistory := h.collectBandwidthHistory()
	userMetrics := h.collectUserMetrics()
	systemMetrics := h.collectSystemMetrics()

	report := h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)
	api.OK(c, report)
}

func (h *ResourceVisualizationHandlers) getDashboard(c *gin.Context) {
	dashboard := map[string]interface{}{
		"generated_at": time.Now(),
		"storage": map[string]interface{}{
			"usage_percent": 0,
			"total_gb":      0,
			"used_gb":       0,
			"status":        "unknown",
		},
		"bandwidth": map[string]interface{}{
			"current_mbps": 0,
			"peak_mbps":    0,
			"utilization":  0,
			"status":       "unknown",
		},
		"users": map[string]interface{}{
			"total":      0,
			"active":     0,
			"over_quota": 0,
		},
		"system": map[string]interface{}{
			"cpu_usage":    0,
			"memory_usage": 0,
			"status":       "unknown",
		},
		"alerts": map[string]interface{}{
			"critical": 0,
			"warning":  0,
			"total":    0,
		},
	}

	// 收集实际数据
	storageMetrics := h.collectStorageMetrics()
	if len(storageMetrics) > 0 {
		var total, used uint64
		for _, m := range storageMetrics {
			total += m.TotalCapacityBytes
			used += m.UsedCapacityBytes
		}
		usagePercent := float64(0)
		if total > 0 {
			usagePercent = float64(used) / float64(total) * 100
		}
		dashboard["storage"] = map[string]interface{}{
			"usage_percent": round(usagePercent, 2),
			"total_gb":      float64(total) / (1024 * 1024 * 1024),
			"used_gb":       float64(used) / (1024 * 1024 * 1024),
			"status":        h.getStorageStatus(usagePercent),
		}
	}

	bandwidthHistory := h.collectBandwidthHistory()
	if len(bandwidthHistory) > 0 {
		report := h.bandwidthReporter.GenerateReport(bandwidthHistory, ReportPeriod{
			StartTime: time.Now().AddDate(0, 0, -1),
			EndTime:   time.Now(),
		})
		dashboard["bandwidth"] = map[string]interface{}{
			"current_mbps": report.Summary.AvgTotalMbps,
			"peak_mbps":    report.Summary.PeakTotalMbps,
			"utilization":  report.Summary.PeakUtilization,
			"status":       h.getBandwidthStatus(report.Summary.PeakUtilization),
		}
	}

	userMetrics := h.collectUserMetrics()
	if len(userMetrics) > 0 {
		var overQuota int
		for _, u := range userMetrics {
			if u.UsagePercent >= 100 {
				overQuota++
			}
		}
		dashboard["users"] = map[string]interface{}{
			"total":      len(userMetrics),
			"active":     len(userMetrics), // 简化
			"over_quota": overQuota,
		}
	}

	systemMetrics := h.collectSystemMetrics()
	if systemMetrics != nil {
		dashboard["system"] = map[string]interface{}{
			"cpu_usage":    systemMetrics.CPUUsage,
			"memory_usage": systemMetrics.MemoryUsage,
			"status":       systemMetrics.SystemStatus,
		}
	}

	// 计算告警数
	report := h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)
	criticalCount := 0
	warningCount := 0
	for _, alert := range report.Alerts {
		switch alert.Severity {
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		}
	}
	dashboard["alerts"] = map[string]interface{}{
		"critical": criticalCount,
		"warning":  warningCount,
		"total":    len(report.Alerts),
	}

	api.OK(c, dashboard)
}

func (h *ResourceVisualizationHandlers) getResourceSummary(c *gin.Context) {
	summary := map[string]interface{}{
		"generated_at": time.Now(),
		"storage":      h.collectStorageSummary(),
		"bandwidth":    h.collectBandwidthSummary(),
		"users":        h.collectUserSummary(),
		"system":       h.collectSystemSummary(),
	}
	api.OK(c, summary)
}

// ========== 存储可视化 API ==========

func (h *ResourceVisualizationHandlers) getStorageVisualization(c *gin.Context) {
	metrics := h.collectStorageMetrics()
	report := h.reporter.GenerateStorageReport(metrics)
	api.OK(c, report.StorageOverview)
}

func (h *ResourceVisualizationHandlers) getVolumeVisualization(c *gin.Context) {
	metrics := h.collectStorageMetrics()
	volumes := make([]VolumeStorageInfo, 0)

	for _, m := range metrics {
		volume := VolumeStorageInfo{
			Name:              m.VolumeName,
			TotalCapacity:     m.TotalCapacityBytes,
			UsedCapacity:      m.UsedCapacityBytes,
			AvailableCapacity: m.AvailableCapacityBytes,
			IOPS:              m.IOPS,
			ReadBandwidth:     m.ReadBandwidthBytes,
			WriteBandwidth:    m.WriteBandwidthBytes,
		}
		if m.TotalCapacityBytes > 0 {
			volume.UsagePercent = round(float64(m.UsedCapacityBytes)/float64(m.TotalCapacityBytes)*100, 2)
		}
		volumes = append(volumes, volume)
	}

	api.OK(c, volumes)
}

func (h *ResourceVisualizationHandlers) getVolumeDetailVisualization(c *gin.Context) {
	name := c.Param("name")
	metrics := h.collectStorageMetrics()

	for _, m := range metrics {
		if m.VolumeName == name {
			volume := VolumeStorageInfo{
				Name:              m.VolumeName,
				TotalCapacity:     m.TotalCapacityBytes,
				UsedCapacity:      m.UsedCapacityBytes,
				AvailableCapacity: m.AvailableCapacityBytes,
				IOPS:              m.IOPS,
				ReadBandwidth:     m.ReadBandwidthBytes,
				WriteBandwidth:    m.WriteBandwidthBytes,
			}
			if m.TotalCapacityBytes > 0 {
				volume.UsagePercent = round(float64(m.UsedCapacityBytes)/float64(m.TotalCapacityBytes)*100, 2)
			}
			api.OK(c, volume)
			return
		}
	}

	api.NotFound(c, "卷不存在")
}

func (h *ResourceVisualizationHandlers) getStorageDistributionViz(c *gin.Context) {
	// 返回存储分布可视化数据
	distribution := map[string]interface{}{
		"by_type": []map[string]interface{}{
			{"type": "文档", "count": 1000, "size_gb": 10.5, "percent": 15.0},
			{"type": "图片", "count": 5000, "size_gb": 25.3, "percent": 35.0},
			{"type": "视频", "count": 200, "size_gb": 30.0, "percent": 42.0},
			{"type": "音频", "count": 800, "size_gb": 5.2, "percent": 8.0},
		},
		"by_user":      []interface{}{},
		"by_directory": []interface{}{},
	}
	api.OK(c, distribution)
}

func (h *ResourceVisualizationHandlers) getStorageEfficiency(c *gin.Context) {
	efficiency := StorageEfficiency{
		CompressionRatio: 1.5,
		DedupRatio:       1.2,
		ActualDataSize:   100 * 1024 * 1024 * 1024, // 100 GB
		PhysicalSize:     60 * 1024 * 1024 * 1024,  // 60 GB
		SavedSpace:       40 * 1024 * 1024 * 1024,  // 40 GB saved
	}
	api.OK(c, efficiency)
}

func (h *ResourceVisualizationHandlers) getStorageTrendViz(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	now := time.Now()
	trend := make([]StorageTrendPoint, 0)

	// 生成模拟趋势数据
	for i := 0; i < days; i++ {
		t := now.AddDate(0, 0, -days+i)
		trend = append(trend, StorageTrendPoint{
			Timestamp:    t,
			UsedCapacity: uint64(50+i) * 1024 * 1024 * 1024, // 模拟增长
			UsagePercent: float64(50 + i),
		})
	}

	api.OK(c, trend)
}

func (h *ResourceVisualizationHandlers) getStoragePrediction(c *gin.Context) {
	prediction := map[string]interface{}{
		"generated_at":            time.Now(),
		"current_usage_percent":   0,
		"predicted_usage_percent": 0,
		"growth_rate_per_day":     0,
		"days_to_capacity":        0,
		"confidence":              0.7,
		"warning":                 "",
	}

	metrics := h.collectStorageMetrics()
	if len(metrics) > 0 {
		var total, used uint64
		for _, m := range metrics {
			total += m.TotalCapacityBytes
			used += m.UsedCapacityBytes
		}

		if total > 0 {
			currentUsage := float64(used) / float64(total) * 100
			prediction["current_usage_percent"] = round(currentUsage, 2)

			// 简单预测：假设每天增长 0.5%
			growthRate := 0.5
			prediction["growth_rate_per_day"] = growthRate
			prediction["predicted_usage_percent"] = round(currentUsage+growthRate*30, 2)

			if growthRate > 0 {
				daysToCapacity := int((100.0 - currentUsage) / growthRate)
				prediction["days_to_capacity"] = daysToCapacity

				if daysToCapacity <= 30 {
					prediction["warning"] = "按当前增长趋势，预计30天内将达到容量上限"
				}
			}
		}
	}

	api.OK(c, prediction)
}

func (h *ResourceVisualizationHandlers) getStorageCharts(c *gin.Context) {
	metrics := h.collectStorageMetrics()
	report := h.reporter.GenerateStorageReport(metrics)
	api.OK(c, report.Charts)
}

// ========== 带宽可视化 API ==========

func (h *ResourceVisualizationHandlers) getBandwidthVisualization(c *gin.Context) {
	history := h.collectBandwidthHistory()
	report := h.reporter.GenerateBandwidthReport(history)
	api.OK(c, report.BandwidthOverview)
}

func (h *ResourceVisualizationHandlers) getInterfaceVisualization(c *gin.Context) {
	history := h.collectBandwidthHistory()
	interfaces := make([]InterfaceBandwidthInfo, 0)

	for iface, points := range history {
		if len(points) == 0 {
			continue
		}

		var totalRx, totalTx uint64
		var rxRate, txRate uint64

		for _, p := range points {
			totalRx += p.RxBytes
			totalTx += p.TxBytes
			rxRate += p.RxRate
			txRate += p.TxRate
		}

		n := uint64(len(points))
		if n > 0 {
			rxRate /= n
			txRate /= n
		}

		interfaces = append(interfaces, InterfaceBandwidthInfo{
			Name: iface,
			Rate: BandwidthRate{
				RxBytesPerSec:    rxRate,
				TxBytesPerSec:    txRate,
				TotalBytesPerSec: rxRate + txRate,
				RxMbps:           float64(rxRate) * 8 / (1024 * 1024),
				TxMbps:           float64(txRate) * 8 / (1024 * 1024),
				TotalMbps:        float64(rxRate+txRate) * 8 / (1024 * 1024),
			},
			TotalRx: totalRx,
			TotalTx: totalTx,
		})
	}

	api.OK(c, interfaces)
}

func (h *ResourceVisualizationHandlers) getInterfaceDetailVisualization(c *gin.Context) {
	name := c.Param("name")
	history := h.collectBandwidthHistory()

	if points, exists := history[name]; exists {
		var totalRx, totalTx uint64
		var rxRate, txRate uint64

		for _, p := range points {
			totalRx += p.RxBytes
			totalTx += p.TxBytes
			rxRate += p.RxRate
			txRate += p.TxRate
		}

		n := uint64(len(points))
		if n > 0 {
			rxRate /= n
			txRate /= n
		}

		iface := InterfaceBandwidthInfo{
			Name: name,
			Rate: BandwidthRate{
				RxBytesPerSec:    rxRate,
				TxBytesPerSec:    txRate,
				TotalBytesPerSec: rxRate + txRate,
				RxMbps:           float64(rxRate) * 8 / (1024 * 1024),
				TxMbps:           float64(txRate) * 8 / (1024 * 1024),
				TotalMbps:        float64(rxRate+txRate) * 8 / (1024 * 1024),
			},
			TotalRx: totalRx,
			TotalTx: totalTx,
		}

		api.OK(c, iface)
		return
	}

	api.NotFound(c, "接口不存在")
}

func (h *ResourceVisualizationHandlers) getBandwidthTrendViz(c *gin.Context) {
	history := h.collectBandwidthHistory()
	trend := make([]BandwidthTrendPoint, 0)

	for _, points := range history {
		for _, p := range points {
			trend = append(trend, BandwidthTrendPoint{
				Timestamp: p.Timestamp,
				RxMbps:    float64(p.RxRate) * 8 / (1024 * 1024),
				TxMbps:    float64(p.TxRate) * 8 / (1024 * 1024),
				TotalMbps: float64(p.RxRate+p.TxRate) * 8 / (1024 * 1024),
			})
		}
		break // 只取第一个接口的趋势
	}

	api.OK(c, trend)
}

func (h *ResourceVisualizationHandlers) getBandwidthPeak(c *gin.Context) {
	history := h.collectBandwidthHistory()

	var peakMbps float64
	var peakTime *time.Time
	var peakInterface string

	for iface, points := range history {
		for _, p := range points {
			totalMbps := float64(p.RxRate+p.TxRate) * 8 / (1024 * 1024)
			if totalMbps > peakMbps {
				peakMbps = totalMbps
				peakTime = &p.Timestamp
				peakInterface = iface
			}
		}
	}

	api.OK(c, BandwidthPeakInfo{
		PeakMbps:      round(peakMbps, 2),
		PeakTime:      peakTime,
		PeakInterface: peakInterface,
	})
}

func (h *ResourceVisualizationHandlers) getBandwidthCharts(c *gin.Context) {
	history := h.collectBandwidthHistory()
	report := h.reporter.GenerateBandwidthReport(history)
	api.OK(c, report.Charts)
}

// ========== 用户资源可视化 API ==========

func (h *ResourceVisualizationHandlers) getUserResourceVisualization(c *gin.Context) {
	metrics := h.collectUserMetrics()
	report := h.reporter.GenerateUserReport(metrics)
	api.OK(c, report.UserOverview)
}

func (h *ResourceVisualizationHandlers) getTopUsersVisualization(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	metrics := h.collectUserMetrics()

	// 按使用量排序
	sorted := make([]UserResourceInfo, len(metrics))
	copy(sorted, metrics)
	// 排序逻辑（简化）

	if limit > len(sorted) {
		limit = len(sorted)
	}

	api.OK(c, sorted[:limit])
}

func (h *ResourceVisualizationHandlers) getUserDetailVisualization(c *gin.Context) {
	username := c.Param("username")
	metrics := h.collectUserMetrics()

	for _, m := range metrics {
		if m.Username == username {
			api.OK(c, m)
			return
		}
	}

	api.NotFound(c, "用户不存在")
}

func (h *ResourceVisualizationHandlers) getQuotaStatusVisualization(c *gin.Context) {
	metrics := h.collectUserMetrics()

	normal := 0
	warning := 0
	critical := 0

	for _, m := range metrics {
		if m.UsagePercent >= 100 {
			critical++
		} else if m.UsagePercent >= 80 {
			warning++
		} else {
			normal++
		}
	}

	api.OK(c, map[string]interface{}{
		"normal":   normal,
		"warning":  warning,
		"critical": critical,
		"total":    len(metrics),
	})
}

func (h *ResourceVisualizationHandlers) getUserResourceCharts(c *gin.Context) {
	metrics := h.collectUserMetrics()
	report := h.reporter.GenerateUserReport(metrics)
	api.OK(c, report.Charts)
}

// ========== 系统资源可视化 API ==========

func (h *ResourceVisualizationHandlers) getSystemVisualization(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	api.OK(c, metrics)
}

func (h *ResourceVisualizationHandlers) getCPUVisualization(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	if metrics != nil {
		api.OK(c, map[string]interface{}{
			"usage":        metrics.CPUUsage,
			"load_average": metrics.LoadAverage,
		})
		return
	}
	api.OK(c, map[string]interface{}{"usage": 0})
}

func (h *ResourceVisualizationHandlers) getMemoryVisualization(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	if metrics != nil {
		api.OK(c, metrics.MemoryInfo)
		return
	}
	api.OK(c, MemoryInfo{})
}

func (h *ResourceVisualizationHandlers) getDiskIOVisualization(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	if metrics != nil {
		api.OK(c, metrics.DiskIO)
		return
	}
	api.OK(c, DiskIOInfo{})
}

func (h *ResourceVisualizationHandlers) getLoadVisualization(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	if metrics != nil {
		api.OK(c, metrics.LoadAverage)
		return
	}
	api.OK(c, LoadAverageInfo{})
}

func (h *ResourceVisualizationHandlers) getSystemCharts(c *gin.Context) {
	metrics := h.collectSystemMetrics()
	storageMetrics := h.collectStorageMetrics()
	bandwidthHistory := h.collectBandwidthHistory()
	userMetrics := h.collectUserMetrics()

	report := h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, metrics)
	api.OK(c, report.Charts)
}

// ========== 告警和建议 API ==========

func (h *ResourceVisualizationHandlers) getResourceAlerts(c *gin.Context) {
	storageMetrics := h.collectStorageMetrics()
	bandwidthHistory := h.collectBandwidthHistory()
	userMetrics := h.collectUserMetrics()
	systemMetrics := h.collectSystemMetrics()

	report := h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)
	api.OK(c, report.Alerts)
}

func (h *ResourceVisualizationHandlers) getStorageAlerts(c *gin.Context) {
	metrics := h.collectStorageMetrics()
	report := h.reporter.GenerateStorageReport(metrics)
	api.OK(c, report.Alerts)
}

func (h *ResourceVisualizationHandlers) getBandwidthAlertsViz(c *gin.Context) {
	history := h.collectBandwidthHistory()
	report := h.reporter.GenerateBandwidthReport(history)
	api.OK(c, report.Alerts)
}

func (h *ResourceVisualizationHandlers) getUserAlerts(c *gin.Context) {
	metrics := h.collectUserMetrics()
	report := h.reporter.GenerateUserReport(metrics)
	api.OK(c, report.Alerts)
}

func (h *ResourceVisualizationHandlers) getResourceRecommendations(c *gin.Context) {
	storageMetrics := h.collectStorageMetrics()
	bandwidthHistory := h.collectBandwidthHistory()
	userMetrics := h.collectUserMetrics()
	systemMetrics := h.collectSystemMetrics()

	report := h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)
	api.OK(c, report.Recommendations)
}

// ========== 导出 API ==========

func (h *ResourceVisualizationHandlers) exportResourceReport(c *gin.Context) {
	var req struct {
		Type     string `json:"type"` // overview, storage, bandwidth, user
		Format   string `json:"format"`
		Language string `json:"language"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	var report *ResourceVisualizationReport

	switch req.Type {
	case "storage":
		metrics := h.collectStorageMetrics()
		report = h.reporter.GenerateStorageReport(metrics)
	case "bandwidth":
		history := h.collectBandwidthHistory()
		report = h.reporter.GenerateBandwidthReport(history)
	case "user":
		metrics := h.collectUserMetrics()
		report = h.reporter.GenerateUserReport(metrics)
	default:
		storageMetrics := h.collectStorageMetrics()
		bandwidthHistory := h.collectBandwidthHistory()
		userMetrics := h.collectUserMetrics()
		systemMetrics := h.collectSystemMetrics()
		report = h.reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)
	}

	api.OK(c, map[string]interface{}{
		"report":  report,
		"format":  req.Format,
		"message": "报告生成成功",
	})
}

func (h *ResourceVisualizationHandlers) exportDashboard(c *gin.Context) {
	// 导出仪表板数据
	h.getDashboard(c)
}

// ========== 辅助方法 ==========

func (h *ResourceVisualizationHandlers) collectStorageMetrics() []StorageMetrics {
	// 实际应从存储模块收集
	return []StorageMetrics{}
}

func (h *ResourceVisualizationHandlers) collectBandwidthHistory() map[string][]BandwidthHistoryPoint {
	// 实际应从监控模块收集
	return make(map[string][]BandwidthHistoryPoint)
}

func (h *ResourceVisualizationHandlers) collectUserMetrics() []UserResourceInfo {
	// 实际应从配额模块收集
	return []UserResourceInfo{}
}

func (h *ResourceVisualizationHandlers) collectSystemMetrics() *SystemResourceOverview {
	// 实际应从监控模块收集
	return nil
}

func (h *ResourceVisualizationHandlers) collectStorageSummary() map[string]interface{} {
	return map[string]interface{}{
		"total_gb":      0,
		"used_gb":       0,
		"usage_percent": 0,
	}
}

func (h *ResourceVisualizationHandlers) collectBandwidthSummary() map[string]interface{} {
	return map[string]interface{}{
		"current_mbps": 0,
		"peak_mbps":    0,
	}
}

func (h *ResourceVisualizationHandlers) collectUserSummary() map[string]interface{} {
	return map[string]interface{}{
		"total_users":  0,
		"active_users": 0,
	}
}

func (h *ResourceVisualizationHandlers) collectSystemSummary() map[string]interface{} {
	return map[string]interface{}{
		"cpu_usage":    0,
		"memory_usage": 0,
	}
}

func (h *ResourceVisualizationHandlers) getStorageStatus(usagePercent float64) string {
	if usagePercent >= 85 {
		return "critical"
	} else if usagePercent >= 70 {
		return "warning"
	}
	return "healthy"
}

func (h *ResourceVisualizationHandlers) getBandwidthStatus(utilization float64) string {
	if utilization >= 90 {
		return "critical"
	} else if utilization >= 70 {
		return "warning"
	}
	return "healthy"
}
