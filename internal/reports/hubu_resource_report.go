// Package reports 提供资源报告集成功能 (v2.89.0 户部)
package reports

import (
	"time"
)

// ========== 资源报告集成类型 ==========

// HubuReportType 资源报告类型
type HubuReportType string

const (
	HubuReportStorageUsage   HubuReportType = "storage_usage"   // 存储使用报告
	HubuReportBandwidthStats HubuReportType = "bandwidth_stats" // 带宽统计报告
	HubuReportCapacity       HubuReportType = "capacity"        // 容量预测报告
	HubuReportComprehensive  HubuReportType = "comprehensive"   // 综合资源报告
)

// HubuReportRequest 资源报告请求
type HubuReportRequest struct {
	// 报告类型
	Type HubuReportType `json:"type" binding:"required"`

	// 报告周期开始时间
	StartTime *time.Time `json:"start_time"`

	// 报告周期结束时间
	EndTime *time.Time `json:"end_time"`

	// 存储卷名称（可选，用于单卷报告）
	VolumeName string `json:"volume_name,omitempty"`

	// 接口名称（可选，用于单接口带宽报告）
	InterfaceName string `json:"interface_name,omitempty"`

	// 预测天数（容量预测专用）
	ForecastDays int `json:"forecast_days,omitempty"`

	// 是否包含预测
	IncludeForecast bool `json:"include_forecast"`

	// 是否包含建议
	IncludeRecommendations bool `json:"include_recommendations"`

	// 是否包含告警
	IncludeAlerts bool `json:"include_alerts"`

	// 输出格式
	Format string `json:"format,omitempty"` // json, html, pdf, excel
}

// HubuReportResponse 资源报告响应
type HubuReportResponse struct {
	// 报告ID
	ID string `json:"id"`

	// 报告类型
	Type HubuReportType `json:"type"`

	// 报告名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 生成耗时（毫秒）
	GenerationTimeMS int64 `json:"generation_time_ms"`

	// 存储使用报告（type=storage_usage 时填充）
	StorageUsage *StorageUsageReport `json:"storage_usage,omitempty"`

	// 带宽统计报告（type=bandwidth_stats 时填充）
	BandwidthStats *BandwidthReport `json:"bandwidth_stats,omitempty"`

	// 容量预测报告（type=capacity 时填充）
	CapacityForecast *CapacityPlanningReport `json:"capacity_forecast,omitempty"`

	// 综合报告（type=comprehensive 时填充）
	Comprehensive *HubuComprehensiveReport `json:"comprehensive,omitempty"`

	// 导出链接
	ExportLinks map[string]string `json:"export_links,omitempty"`
}

// HubuComprehensiveReport 综合资源报告
type HubuComprehensiveReport struct {
	// 总体健康评分
	OverallHealthScore float64 `json:"overall_health_score"` // 0-100

	// 健康状态
	HealthStatus string `json:"health_status"` // excellent, good, warning, critical

	// 存储摘要
	StorageSummary StorageUsageSummary `json:"storage_summary"`

	// 带宽摘要
	BandwidthSummary BandwidthSummary `json:"bandwidth_summary"`

	// 容量规划摘要
	CapacitySummary CapacityPlanningSummary `json:"capacity_summary"`

	// 关键指标
	KeyMetrics []HubuKeyMetric `json:"key_metrics"`

	// 告警汇总
	AlertSummary HubuAlertSummary `json:"alert_summary"`

	// 优先建议
	TopRecommendations []HubuPrioritizedRecommendation `json:"top_recommendations"`

	// 趋势摘要
	TrendSummary HubuTrendSummary `json:"trend_summary"`

	// 预测摘要
	ForecastSummary HubuForecastSummary `json:"forecast_summary"`
}

// HubuKeyMetric 关键指标
type HubuKeyMetric struct {
	// 指标名称
	Name string `json:"name"`

	// 指标值
	Value float64 `json:"value"`

	// 单位
	Unit string `json:"unit"`

	// 变化趋势
	Trend string `json:"trend"` // up, down, stable

	// 变化百分比
	ChangePercent float64 `json:"change_percent"`

	// 状态
	Status string `json:"status"` // normal, warning, critical

	// 描述
	Description string `json:"description"`
}

// HubuAlertSummary 告警汇总
type HubuAlertSummary struct {
	// 总告警数
	Total int `json:"total"`

	// 严重告警数
	Critical int `json:"critical"`

	// 警告告警数
	Warning int `json:"warning"`

	// 信息告警数
	Info int `json:"info"`

	// 按类型分组
	ByType map[string]int `json:"by_type"`

	// 按资源分组
	ByResource map[string]int `json:"by_resource"`
}

// HubuPrioritizedRecommendation 优先建议
type HubuPrioritizedRecommendation struct {
	// 优先级排名
	Rank int `json:"rank"`

	// 类型
	Type string `json:"type"` // storage, bandwidth, capacity, cost

	// 优先级
	Priority string `json:"priority"` // critical, high, medium, low

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 预期影响
	Impact string `json:"impact"`

	// 预计节省/收益
	Savings float64 `json:"savings"`

	// 节省单位
	SavingsUnit string `json:"savings_unit"`

	// 实施难度
	Effort string `json:"effort"` // easy, medium, hard

	// 预计实施时间
	EstimatedTime string `json:"estimated_time"`
}

// HubuTrendSummary 趋势摘要
type HubuTrendSummary struct {
	// 存储增长趋势
	StorageTrend string `json:"storage_trend"` // increasing, stable, decreasing

	// 存储增长率（%/月）
	StorageGrowthRate float64 `json:"storage_growth_rate"`

	// 带宽增长趋势
	BandwidthTrend string `json:"bandwidth_trend"`

	// 带宽增长率（%/月）
	BandwidthGrowthRate float64 `json:"bandwidth_growth_rate"`

	// 预计存储满载天数
	DaysToStorageFull int `json:"days_to_storage_full"`

	// 预计带宽满载天数
	DaysToBandwidthFull int `json:"days_to_bandwidth_full"`
}

// HubuForecastSummary 预测摘要
type HubuForecastSummary struct {
	// 下月存储使用预测（GB）
	NextMonthStorageGB float64 `json:"next_month_storage_gb"`

	// 下季度存储使用预测（GB）
	NextQuarterStorageGB float64 `json:"next_quarter_storage_gb"`

	// 下月带宽使用预测（GB）
	NextMonthBandwidthGB float64 `json:"next_month_bandwidth_gb"`

	// 下季度带宽使用预测（GB）
	NextQuarterBandwidthGB float64 `json:"next_quarter_bandwidth_gb"`

	// 建议扩容量（GB）
	RecommendedExpansionGB float64 `json:"recommended_expansion_gb"`

	// 预测置信度
	Confidence float64 `json:"confidence"` // 0-1
}

// HubuResourceReportGenerator 集成资源报告生成器 (v2.89.0)
type HubuResourceReportGenerator struct {
	storageReporter   *StorageUsageReporter
	bandwidthReporter *BandwidthReporter
	capacityPlanner   *CapacityPlanner
	costCalculator    *StorageCostCalculator
}

// NewHubuResourceReportGenerator 创建集成资源报告生成器
func NewHubuResourceReportGenerator(
	storageConfig StorageReportConfig,
	bandwidthConfig BandwidthReportConfig,
	capacityConfig CapacityPlanningConfig,
	costConfig StorageCostConfig,
) *HubuResourceReportGenerator {
	return &HubuResourceReportGenerator{
		storageReporter:   NewStorageUsageReporter(storageConfig),
		bandwidthReporter: NewBandwidthReporter(bandwidthConfig),
		capacityPlanner:   NewCapacityPlanner(capacityConfig),
		costCalculator:    NewStorageCostCalculator(costConfig),
	}
}

// GenerateReport 生成资源报告
func (g *HubuResourceReportGenerator) GenerateReport(req HubuReportRequest) *HubuReportResponse {
	startTime := time.Now()

	response := &HubuReportResponse{
		ID:          "hubu_" + startTime.Format("20060102150405"),
		Type:        req.Type,
		GeneratedAt: startTime,
	}

	// 设置报告周期
	if req.StartTime != nil && req.EndTime != nil {
		response.Period = ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	} else {
		// 默认最近30天
		response.Period = ReportPeriod{
			StartTime: startTime.AddDate(0, 0, -30),
			EndTime:   startTime,
		}
	}

	// 根据类型生成报告
	switch req.Type {
	case HubuReportStorageUsage:
		response.Name = "存储使用报告"
		response.StorageUsage = g.generateStorageUsageReport(req)

	case HubuReportBandwidthStats:
		response.Name = "带宽统计报告"
		response.BandwidthStats = g.generateBandwidthStatsReport(req)

	case HubuReportCapacity:
		response.Name = "容量预测报告"
		response.CapacityForecast = g.generateCapacityForecastReport(req)

	case HubuReportComprehensive:
		response.Name = "综合资源报告"
		response.Comprehensive = g.generateComprehensiveReport(req)
	}

	// 计算生成耗时
	response.GenerationTimeMS = time.Since(startTime).Milliseconds()

	return response
}

// generateStorageUsageReport 生成存储使用报告
func (g *HubuResourceReportGenerator) generateStorageUsageReport(req HubuReportRequest) *StorageUsageReport {
	now := time.Now()
	return &StorageUsageReport{
		ID:          "storage_" + now.Format("20060102150405"),
		Name:        "存储使用报表",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -30),
			EndTime:   now,
		},
		Summary: StorageUsageSummary{
			HealthStatus: "healthy",
		},
		Volumes:              []VolumeUsageDetail{},
		TopUsers:             []UserStorageUsage{},
		FileTypeDistribution: []FileTypeStats{},
		Alerts:               []StorageAlert{},
		Recommendations:      []StorageRecommendation{},
	}
}

// generateBandwidthStatsReport 生成带宽统计报告
func (g *HubuResourceReportGenerator) generateBandwidthStatsReport(req HubuReportRequest) *BandwidthReport {
	now := time.Now()
	return &BandwidthReport{
		ID:          "bw_" + now.Format("20060102150405"),
		Name:        "带宽使用报告",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -30),
			EndTime:   now,
		},
		InterfaceStats:  []BandwidthUsageStats{},
		Trends:          []BandwidthTrend{},
		Alerts:          []BandwidthAlert{},
		Recommendations: []BandwidthRecommendation{},
	}
}

// generateCapacityForecastReport 生成容量预测报告
func (g *HubuResourceReportGenerator) generateCapacityForecastReport(req HubuReportRequest) *CapacityPlanningReport {
	now := time.Now()
	return &CapacityPlanningReport{
		ID:              "cap_" + now.Format("20060102150405"),
		Name:            "容量规划报告",
		GeneratedAt:     now,
		Forecasts:       []CapacityForecast{},
		Milestones:      []CapacityMilestone{},
		Recommendations: []CapacityRecommendation{},
	}
}

// generateComprehensiveReport 生成综合报告
func (g *HubuResourceReportGenerator) generateComprehensiveReport(req HubuReportRequest) *HubuComprehensiveReport {
	report := &HubuComprehensiveReport{
		OverallHealthScore: 85.0,
		HealthStatus:       "good",
		KeyMetrics:         []HubuKeyMetric{},
		TopRecommendations: []HubuPrioritizedRecommendation{},
	}

	// 初始化告警摘要
	report.AlertSummary = HubuAlertSummary{
		Total:      0,
		Critical:   0,
		Warning:    0,
		Info:       0,
		ByType:     make(map[string]int),
		ByResource: make(map[string]int),
	}

	// 初始化趋势摘要
	report.TrendSummary = HubuTrendSummary{
		StorageTrend:   "stable",
		BandwidthTrend: "stable",
	}

	// 初始化预测摘要
	report.ForecastSummary = HubuForecastSummary{
		Confidence: 0.75,
	}

	return report
}

// CalculateHealthScore 计算健康评分
func (g *HubuResourceReportGenerator) CalculateHealthScore(
	storageReport *StorageUsageReport,
	bandwidthReport *BandwidthReport,
	capacityReport *CapacityPlanningReport,
) float64 {
	score := 100.0

	// 存储使用率扣分
	if storageReport != nil {
		usage := storageReport.Summary.UsagePercent
		if usage >= 90 {
			score -= 30
		} else if usage >= 80 {
			score -= 20
		} else if usage >= 70 {
			score -= 10
		}

		// 告警扣分
		for _, alert := range storageReport.Alerts {
			switch alert.Severity {
			case "critical":
				score -= 5
			case "warning":
				score -= 2
			}
		}
	}

	// 带宽利用率扣分
	if bandwidthReport != nil {
		util := bandwidthReport.Summary.PeakUtilization
		if util >= 90 {
			score -= 20
		} else if util >= 80 {
			score -= 10
		} else if util >= 70 {
			score -= 5
		}

		// 错误率扣分
		if bandwidthReport.Summary.AvgErrorRate > 1 {
			score -= 10
		}
	}

	// 容量预测扣分
	if capacityReport != nil {
		switch capacityReport.Summary.Urgency {
		case "critical":
			score -= 25
		case "high":
			score -= 15
		case "medium":
			score -= 5
		}
	}

	// 确保分数在 0-100 范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// GetHealthStatus 获取健康状态描述
func (g *HubuResourceReportGenerator) GetHealthStatus(score float64) string {
	if score >= 90 {
		return "excellent"
	} else if score >= 75 {
		return "good"
	} else if score >= 50 {
		return "warning"
	}
	return "critical"
}

// ========== 存储使用报告增强功能 ==========

// HubuStorageEnhancedReport 存储使用增强报告
type HubuStorageEnhancedReport struct {
	*StorageUsageReport

	// 增强指标
	EnhancedMetrics HubuStorageEnhancedMetrics `json:"enhanced_metrics"`

	// 成本分析
	CostAnalysis HubuStorageCostAnalysis `json:"cost_analysis"`

	// 热点分析
	HotspotAnalysis HubuStorageHotspotAnalysis `json:"hotspot_analysis"`
}

// HubuStorageEnhancedMetrics 存储增强指标
type HubuStorageEnhancedMetrics struct {
	// IOPS统计
	ReadIOPS  uint64 `json:"read_iops"`
	WriteIOPS uint64 `json:"write_iops"`

	// 吞吐量统计
	ReadThroughputMB  float64 `json:"read_throughput_mb"`  // MB/s
	WriteThroughputMB float64 `json:"write_throughput_mb"` // MB/s

	// 延迟统计
	AvgLatencyMs float64 `json:"avg_latency_ms"` // 毫秒
	MaxLatencyMs float64 `json:"max_latency_ms"`

	// 数据效率
	CompressionRatio float64 `json:"compression_ratio"` // 压缩比
	DedupRatio       float64 `json:"dedup_ratio"`       // 去重比

	// 快照统计
	SnapshotCount      int     `json:"snapshot_count"`
	SnapshotSizeGB     float64 `json:"snapshot_size_gb"`
	SnapshotEfficiency float64 `json:"snapshot_efficiency"`
}

// HubuStorageCostAnalysis 存储成本分析
type HubuStorageCostAnalysis struct {
	// 月度成本
	MonthlyCost float64 `json:"monthly_cost"` // 元

	// 成本构成
	CostBreakdown HubuCostBreakdown `json:"cost_breakdown"`

	// 成本趋势
	CostTrend []CostTrendPoint `json:"cost_trend"`

	// 成本预测
	ProjectedCost float64 `json:"projected_cost"` // 预计下月成本

	// 成本效率
	CostEfficiency float64 `json:"cost_efficiency"` // 0-100
}

// HubuCostBreakdown 成本构成
type HubuCostBreakdown struct {
	// 存储成本
	StorageCost float64 `json:"storage_cost"`

	// 电力成本
	ElectricityCost float64 `json:"electricity_cost"`

	// 运维成本
	OperationsCost float64 `json:"operations_cost"`

	// 折旧成本
	DepreciationCost float64 `json:"depreciation_cost"`
}

// HubuStorageHotspotAnalysis 存储热点分析
type HubuStorageHotspotAnalysis struct {
	// 热点目录
	HotDirectories []HubuHotDirectory `json:"hot_directories"`

	// 热点文件
	HotFiles []HubuHotFile `json:"hot_files"`

	// 活跃用户
	ActiveUsers []HubuActiveUser `json:"active_users"`

	// 访问模式
	AccessPattern string `json:"access_pattern"` // read_heavy, write_heavy, balanced
}

// HubuHotDirectory 热点目录
type HubuHotDirectory struct {
	Path        string  `json:"path"`
	AccessCount int64   `json:"access_count"`
	SizeGB      float64 `json:"size_gb"`
	GrowthRate  float64 `json:"growth_rate"` // %/day
}

// HubuHotFile 热点文件
type HubuHotFile struct {
	Path        string    `json:"path"`
	SizeMB      float64   `json:"size_mb"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

// HubuActiveUser 活跃用户
type HubuActiveUser struct {
	Username   string `json:"username"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	FileCount  int    `json:"file_count"`
}

// ========== 带宽统计报告增强功能 ==========

// HubuBandwidthEnhancedReport 带宽统计增强报告
type HubuBandwidthEnhancedReport struct {
	*BandwidthReport

	// 增强指标
	EnhancedMetrics HubuBandwidthEnhancedMetrics `json:"enhanced_metrics"`

	// 流量分析
	TrafficAnalysis HubuTrafficAnalysis `json:"traffic_analysis"`

	// 协议分布
	ProtocolDistribution []HubuProtocolStats `json:"protocol_distribution"`
}

// HubuBandwidthEnhancedMetrics 带宽增强指标
type HubuBandwidthEnhancedMetrics struct {
	// 连接统计
	ActiveConnections int `json:"active_connections"`
	MaxConnections    int `json:"max_connections"`

	// 会话统计
	TotalSessions  int64   `json:"total_sessions"`
	AvgSessionTime float64 `json:"avg_session_time"` // 秒

	// 重传率
	RetransmitRate float64 `json:"retransmit_rate"` // %

	// RTT
	AvgRTTMs float64 `json:"avg_rtt_ms"` // 毫秒
	MaxRTTMs float64 `json:"max_rtt_ms"`

	// TCP窗口
	AvgTCPWindow uint64 `json:"avg_tcp_window"`
}

// HubuTrafficAnalysis 流量分析
type HubuTrafficAnalysis struct {
	// 流量模式
	Pattern string `json:"pattern"` // bursty, steady, periodic

	// 峰值分析
	PeakHours []HubuPeakHour `json:"peak_hours"`

	// 应用分布
	AppDistribution []HubuAppStats `json:"app_distribution"`
}

// HubuPeakHour 峰值时段
type HubuPeakHour struct {
	Hour       int     `json:"hour"` // 0-23
	AvgMbps    float64 `json:"avg_mbps"`
	PeakMbps   float64 `json:"peak_mbps"`
	Percentage float64 `json:"percentage"` // 占总流量比例
}

// HubuAppStats 应用统计
type HubuAppStats struct {
	Application string  `json:"application"`
	Port        int     `json:"port"`
	TrafficGB   float64 `json:"traffic_gb"`
	Percentage  float64 `json:"percentage"`
}

// HubuProtocolStats 协议统计
type HubuProtocolStats struct {
	Protocol   string  `json:"protocol"` // TCP, UDP, HTTP, HTTPS, etc.
	Bytes      uint64  `json:"bytes"`
	Percentage float64 `json:"percentage"`
	Packets    uint64  `json:"packets"`
}

// ========== 容量预测增强功能 ==========

// HubuCapacityEnhancedReport 容量预测增强报告
type HubuCapacityEnhancedReport struct {
	*CapacityPlanningReport

	// 增强预测
	EnhancedForecast HubuEnhancedCapacityForecast `json:"enhanced_forecast"`

	// 场景分析
	ScenarioAnalysis []HubuCapacityScenario `json:"scenario_analysis"`
}

// HubuEnhancedCapacityForecast 增强容量预测
type HubuEnhancedCapacityForecast struct {
	// 置信区间
	ConfidenceIntervals []HubuConfidenceInterval `json:"confidence_intervals"`

	// 预测模型详情
	ModelDetails HubuModelDetails `json:"model_details"`

	// 季节性分析
	Seasonality HubuSeasonalityAnalysis `json:"seasonality"`
}

// HubuConfidenceInterval 置信区间
type HubuConfidenceInterval struct {
	Date      time.Time `json:"date"`
	Lower     uint64    `json:"lower"`     // 下限
	Predicted uint64    `json:"predicted"` // 预测值
	Upper     uint64    `json:"upper"`     // 上限
	Level     float64   `json:"level"`     // 置信水平 (0.95 = 95%)
}

// HubuModelDetails 模型详情
type HubuModelDetails struct {
	Type         string  `json:"type"`          // linear, exponential, arima, lstm
	Accuracy     float64 `json:"accuracy"`      // 模型准确度
	MAPE         float64 `json:"mape"`          // 平均绝对百分比误差
	RMSE         float64 `json:"rmse"`          // 均方根误差
	TrainingDays int     `json:"training_days"` // 训练数据天数
}

// HubuSeasonalityAnalysis 季节性分析
type HubuSeasonalityAnalysis struct {
	HasSeasonality bool    `json:"has_seasonality"`
	CycleDays      int     `json:"cycle_days"` // 周期天数
	PeakDay        int     `json:"peak_day"`   // 周期峰值日
	Variation      float64 `json:"variation"`  // 波动幅度
}

// HubuCapacityScenario 容量场景
type HubuCapacityScenario struct {
	Name        string     `json:"name"` // 乐观, 基准, 悲观
	Description string     `json:"description"`
	GrowthRate  float64    `json:"growth_rate"` // 增长率
	FullDate    *time.Time `json:"full_date"`   // 满载日期
	ActionDate  *time.Time `json:"action_date"` // 建议行动日期
}
