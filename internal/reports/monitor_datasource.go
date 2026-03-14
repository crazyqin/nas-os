package reports

import (
	"fmt"
	"time"
)

// MonitorDataSource 监控数据源适配器
// 这是一个通用适配器，用于将监控数据集成到报表系统
type MonitorDataSource struct {
	name string
	// 数据获取函数
	getSystemStats    func() (map[string]interface{}, error)
	getDiskStats      func() ([]map[string]interface{}, error)
	getNetworkStats   func() ([]map[string]interface{}, error)
	getHealthScore    func() (map[string]interface{}, error)
	getTrendData      func(period string) ([]map[string]interface{}, error)
	getResourceReport func(period string) (map[string]interface{}, error)
}

// MonitorDataSourceConfig 监控数据源配置
type MonitorDataSourceConfig struct {
	Name              string
	GetSystemStats    func() (map[string]interface{}, error)
	GetDiskStats      func() ([]map[string]interface{}, error)
	GetNetworkStats   func() ([]map[string]interface{}, error)
	GetHealthScore    func() (map[string]interface{}, error)
	GetTrendData      func(period string) ([]map[string]interface{}, error)
	GetResourceReport func(period string) (map[string]interface{}, error)
}

// NewMonitorDataSource 创建监控数据源
func NewMonitorDataSource(config MonitorDataSourceConfig) *MonitorDataSource {
	return &MonitorDataSource{
		name:              config.Name,
		getSystemStats:    config.GetSystemStats,
		getDiskStats:      config.GetDiskStats,
		getNetworkStats:   config.GetNetworkStats,
		getHealthScore:    config.GetHealthScore,
		getTrendData:      config.GetTrendData,
		getResourceReport: config.GetResourceReport,
	}
}

// Name 数据源名称
func (ds *MonitorDataSource) Name() string {
	if ds.name == "" {
		return "monitor"
	}
	return ds.name
}

// Query 查询数据
func (ds *MonitorDataSource) Query(
	query map[string]interface{},
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	aggregations []TemplateAggregation,
	groupBy []string,
	limit, offset int,
) ([]map[string]interface{}, error) {

	// 根据查询类型返回不同的数据
	reportType := "current"
	if t, ok := query["type"].(string); ok {
		reportType = t
	}

	switch reportType {
	case "health":
		return ds.queryHealthScore()
	case "trend":
		return ds.queryTrend(query)
	case "disk":
		return ds.queryDiskStats()
	case "network":
		return ds.queryNetworkStats()
	case "system":
		return ds.querySystemStats()
	default:
		return ds.queryCurrent()
	}
}

// queryCurrent 查询当前状态
func (ds *MonitorDataSource) queryCurrent() ([]map[string]interface{}, error) {
	row := make(map[string]interface{})

	// 获取系统状态
	if ds.getSystemStats != nil {
		stats, err := ds.getSystemStats()
		if err == nil {
			for k, v := range stats {
				row[k] = v
			}
		}
	}

	// 获取健康评分
	if ds.getHealthScore != nil {
		score, err := ds.getHealthScore()
		if err == nil {
			for k, v := range score {
				row["health_"+k] = v
			}
		}
	}

	row["timestamp"] = time.Now()

	return []map[string]interface{}{row}, nil
}

// queryHealthScore 查询健康评分
func (ds *MonitorDataSource) queryHealthScore() ([]map[string]interface{}, error) {
	if ds.getHealthScore == nil {
		return nil, fmt.Errorf("健康评分函数未配置")
	}

	score, err := ds.getHealthScore()
	if err != nil {
		return nil, err
	}

	return []map[string]interface{}{score}, nil
}

// queryTrend 查询趋势数据
func (ds *MonitorDataSource) queryTrend(query map[string]interface{}) ([]map[string]interface{}, error) {
	if ds.getTrendData == nil {
		return nil, fmt.Errorf("趋势数据函数未配置")
	}

	period := "daily"
	if p, ok := query["period"].(string); ok {
		period = p
	}

	data, err := ds.getTrendData(period)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// queryDiskStats 查询磁盘统计
func (ds *MonitorDataSource) queryDiskStats() ([]map[string]interface{}, error) {
	if ds.getDiskStats == nil {
		return nil, fmt.Errorf("磁盘统计函数未配置")
	}

	stats, err := ds.getDiskStats()
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// queryNetworkStats 查询网络统计
func (ds *MonitorDataSource) queryNetworkStats() ([]map[string]interface{}, error) {
	if ds.getNetworkStats == nil {
		return nil, fmt.Errorf("网络统计函数未配置")
	}

	stats, err := ds.getNetworkStats()
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// querySystemStats 查询系统统计
func (ds *MonitorDataSource) querySystemStats() ([]map[string]interface{}, error) {
	if ds.getSystemStats == nil {
		return nil, fmt.Errorf("系统统计函数未配置")
	}

	stats, err := ds.getSystemStats()
	if err != nil {
		return nil, err
	}

	return []map[string]interface{}{stats}, nil
}

// GetSummary 获取摘要
func (ds *MonitorDataSource) GetSummary(query map[string]interface{}) (map[string]interface{}, error) {
	summary := make(map[string]interface{})

	// 获取当前状态
	current, err := ds.queryCurrent()
	if err == nil && len(current) > 0 {
		summary["current"] = current[0]
	}

	// 获取健康评分
	if ds.getHealthScore != nil {
		score, err := ds.getHealthScore()
		if err == nil {
			summary["health_score"] = score["total_score"]
			summary["health_grade"] = score["grade"]
		}
	}

	return summary, nil
}

// GetAvailableFields 获取可用字段
func (ds *MonitorDataSource) GetAvailableFields() []TemplateField {
	return []TemplateField{
		// 系统字段
		{Name: "cpu_usage", Label: "CPU 使用率", Type: FieldTypePercent, Source: "system.cpu_usage", Sortable: true},
		{Name: "memory_usage", Label: "内存使用率", Type: FieldTypePercent, Source: "system.memory_usage", Sortable: true},
		{Name: "memory_total", Label: "内存总量", Type: FieldTypeBytes, Source: "system.memory_total", Sortable: true},
		{Name: "memory_used", Label: "内存使用", Type: FieldTypeBytes, Source: "system.memory_used", Sortable: true},
		{Name: "swap_usage", Label: "Swap 使用率", Type: FieldTypePercent, Source: "system.swap_usage", Sortable: true},
		{Name: "uptime", Label: "运行时间", Type: FieldTypeDuration, Source: "system.uptime"},
		{Name: "uptime_seconds", Label: "运行秒数", Type: FieldTypeNumber, Source: "system.uptime_seconds", Sortable: true},
		{Name: "load_avg", Label: "负载均衡", Type: FieldTypeList, Source: "system.load_avg"},

		// 磁盘字段
		{Name: "disk_device", Label: "磁盘设备", Type: FieldTypeString, Source: "disk.device", Sortable: true, Filterable: true},
		{Name: "disk_mount", Label: "挂载点", Type: FieldTypeString, Source: "disk.mount_point", Sortable: true, Filterable: true},
		{Name: "disk_total", Label: "磁盘总容量", Type: FieldTypeBytes, Source: "disk.total", Sortable: true},
		{Name: "disk_used", Label: "磁盘使用", Type: FieldTypeBytes, Source: "disk.used", Sortable: true},
		{Name: "disk_free", Label: "磁盘可用", Type: FieldTypeBytes, Source: "disk.free", Sortable: true},
		{Name: "disk_usage_percent", Label: "磁盘使用率", Type: FieldTypePercent, Source: "disk.usage_percent", Sortable: true},

		// 网络字段
		{Name: "net_interface", Label: "网络接口", Type: FieldTypeString, Source: "network.interface", Sortable: true, Filterable: true},
		{Name: "net_rx_bytes", Label: "接收字节", Type: FieldTypeBytes, Source: "network.rx_bytes", Sortable: true},
		{Name: "net_tx_bytes", Label: "发送字节", Type: FieldTypeBytes, Source: "network.tx_bytes", Sortable: true},
		{Name: "net_rx_packets", Label: "接收包数", Type: FieldTypeNumber, Source: "network.rx_packets", Sortable: true},
		{Name: "net_tx_packets", Label: "发送包数", Type: FieldTypeNumber, Source: "network.tx_packets", Sortable: true},

		// 健康评分字段
		{Name: "health_score", Label: "健康评分", Type: FieldTypeNumber, Source: "health.total_score", Sortable: true},
		{Name: "health_grade", Label: "健康等级", Type: FieldTypeString, Source: "health.grade", Sortable: true, Filterable: true},
		{Name: "health_trend", Label: "评分趋势", Type: FieldTypeString, Source: "health.trend.direction", Filterable: true},

		// 时间字段
		{Name: "timestamp", Label: "时间戳", Type: FieldTypeDateTime, Source: "timestamp", Sortable: true},
	}
}

// ResourceReportGenerator 资源报告生成器
type ResourceReportGenerator struct {
	getResourceReport func(period string) (map[string]interface{}, error)
}

// NewResourceReportGenerator 创建资源报告生成器
func NewResourceReportGenerator(getReportFunc func(period string) (map[string]interface{}, error)) *ResourceReportGenerator {
	return &ResourceReportGenerator{
		getResourceReport: getReportFunc,
	}
}

// GenerateDailyReport 生成日报
func (rg *ResourceReportGenerator) GenerateDailyReport() (*GeneratedReport, error) {
	return rg.generateReport("daily")
}

// GenerateWeeklyReport 生成周报
func (rg *ResourceReportGenerator) GenerateWeeklyReport() (*GeneratedReport, error) {
	return rg.generateReport("weekly")
}

// GenerateHourlyReport 生成时报
func (rg *ResourceReportGenerator) GenerateHourlyReport() (*GeneratedReport, error) {
	return rg.generateReport("hourly")
}

// generateReport 生成报告
func (rg *ResourceReportGenerator) generateReport(period string) (*GeneratedReport, error) {
	if rg.getResourceReport == nil {
		return nil, fmt.Errorf("报告生成函数未配置")
	}

	reportData, err := rg.getResourceReport(period)
	if err != nil {
		return nil, err
	}

	// 转换为 GeneratedReport
	var name string
	switch period {
	case "hourly":
		name = "资源使用时报"
	case "daily":
		name = "资源使用日报"
	case "weekly":
		name = "资源使用周报"
	default:
		name = "资源使用报告"
	}

	now := time.Now()
	report := &GeneratedReport{
		ID:          fmt.Sprintf("resource-%s-%d", period, now.Unix()),
		Name:        name,
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.Add(-getPeriodDuration(period)),
			EndTime:   now,
		},
		TotalRecords: 1,
	}

	// 提取摘要
	if usage, ok := reportData["resource_usage"].(map[string]interface{}); ok {
		report.Summary = usage
	}

	// 设置数据
	report.Data = []map[string]interface{}{reportData}

	return report, nil
}

// getPeriodDuration 获取周期时长
func getPeriodDuration(period string) time.Duration {
	switch period {
	case "hourly":
		return time.Hour
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
