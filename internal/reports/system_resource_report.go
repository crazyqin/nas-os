// Package reports 提供报表生成和管理功能
package reports

import (
	"sort"
	"time"
)

// ========== 系统资源报表 v2.56.0 ==========

// SystemResourceReport 系统资源报表
type SystemResourceReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 系统概览
	Summary SystemResourceSummary `json:"summary"`

	// CPU 信息
	CPU CPUResourceInfo `json:"cpu"`

	// 内存信息
	Memory MemoryResourceInfo `json:"memory"`

	// 磁盘信息
	Disk DiskResourceInfo `json:"disk"`

	// 网络信息
	Network NetworkResourceInfo `json:"network"`

	// 进程信息
	Processes ProcessInfo `json:"processes"`

	// 趋势数据
	Trends ResourceTrends `json:"trends"`

	// 告警信息
	Alerts []ResourceAlertItem `json:"alerts"`

	// 建议
	Recommendations []SystemRecommendation `json:"recommendations"`

	// 系统健康评分
	HealthScore SystemHealthScore `json:"health_score"`
}

// SystemResourceSummary 系统资源摘要
type SystemResourceSummary struct {
	// 主机名
	Hostname string `json:"hostname"`

	// 操作系统
	OS string `json:"os"`

	// 内核版本
	Kernel string `json:"kernel"`

	// 架构
	Arch string `json:"arch"`

	// 运行时间（秒）
	Uptime int64 `json:"uptime"`

	// 运行时间（人类可读）
	UptimeHR string `json:"uptime_hr"`

	// 系统负载
	LoadAvg LoadAverage `json:"load_avg"`

	// 总进程数
	TotalProcesses int `json:"total_processes"`

	// 运行中进程数
	RunningProcesses int `json:"running_processes"`

	// 总体状态
	Status string `json:"status"` // healthy, warning, critical

	// 状态消息
	StatusMessage string `json:"status_message"`

	// 资源利用率
	ResourceUtilization ResourceUtilizationSummary `json:"resource_utilization"`

	// 性能评分
	PerformanceScore float64 `json:"performance_score"` // 0-100
}

// LoadAverage 负载均值
type LoadAverage struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

// ResourceUtilizationSummary 资源利用率摘要
type ResourceUtilizationSummary struct {
	CPU     float64 `json:"cpu"`     // %
	Memory  float64 `json:"memory"`  // %
	Disk    float64 `json:"disk"`    // %
	Network float64 `json:"network"` // Mbps
}

// CPUResourceInfo CPU 资源信息
type CPUResourceInfo struct {
	// CPU 核心数
	Cores int `json:"cores"`

	// 逻辑处理器数
	LogicalProcessors int `json:"logical_processors"`

	// CPU 型号
	Model string `json:"model"`

	// CPU 频率（MHz）
	Frequency float64 `json:"frequency"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 用户态使用率
	UserPercent float64 `json:"user_percent"`

	// 系统态使用率
	SystemPercent float64 `json:"system_percent"`

	// 空闲率
	IdlePercent float64 `json:"idle_percent"`

	// I/O 等待率
	IOWaitPercent float64 `json:"io_wait_percent"`

	// 温度（摄氏度）
	Temperature float64 `json:"temperature,omitempty"`

	// 历史趋势
	History []CPUMetricPoint `json:"history"`

	// 每核使用率
	PerCoreUsage []float64 `json:"per_core_usage,omitempty"`

	// 上下文切换数
	ContextSwitches uint64 `json:"context_switches"`

	// 中断数
	Interrupts uint64 `json:"interrupts"`
}

// CPUMetricPoint CPU 指标数据点
type CPUMetricPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	UsagePercent  float64   `json:"usage_percent"`
	UserPercent   float64   `json:"user_percent"`
	SystemPercent float64   `json:"system_percent"`
	IOWaitPercent float64   `json:"io_wait_percent"`
}

// MemoryResourceInfo 内存资源信息
type MemoryResourceInfo struct {
	// 总内存（字节）
	Total uint64 `json:"total"`

	// 已使用（字节）
	Used uint64 `json:"used"`

	// 可用（字节）
	Available uint64 `json:"available"`

	// 缓存（字节）
	Cached uint64 `json:"cached"`

	// 缓冲区（字节）
	Buffers uint64 `json:"buffers"`

	// 共享内存（字节）
	Shared uint64 `json:"shared"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 交换区总量
	SwapTotal uint64 `json:"swap_total"`

	// 交换区使用
	SwapUsed uint64 `json:"swap_used"`

	// 交换区使用率
	SwapUsagePercent float64 `json:"swap_usage_percent"`

	// 人类可读格式
	TotalHR     string `json:"total_hr"`
	UsedHR      string `json:"used_hr"`
	AvailableHR string `json:"available_hr"`

	// 历史趋势
	History []MemoryMetricPoint `json:"history"`

	// 内存压力
	MemoryPressure string `json:"memory_pressure"` // low, medium, high

	// OOM 风险
	OOMRisk float64 `json:"oom_risk"` // 0-1
}

// MemoryMetricPoint 内存指标数据点
type MemoryMetricPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	Used         uint64    `json:"used"`
	UsagePercent float64   `json:"usage_percent"`
	SwapUsed     uint64    `json:"swap_used"`
	Available    uint64    `json:"available"`
}

// DiskResourceInfo 磁盘资源信息
type DiskResourceInfo struct {
	// 磁盘数量
	DiskCount int `json:"disk_count"`

	// 总容量（字节）
	TotalCapacity uint64 `json:"total_capacity"`

	// 已使用（字节）
	UsedCapacity uint64 `json:"used_capacity"`

	// 可用空间（字节）
	AvailableCapacity uint64 `json:"available_capacity"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 读 IOPS
	ReadIOPS uint64 `json:"read_iops"`

	// 写 IOPS
	WriteIOPS uint64 `json:"write_iops"`

	// 读吞吐量（字节/秒）
	ReadThroughput uint64 `json:"read_throughput"`

	// 写吞吐量（字节/秒）
	WriteThroughput uint64 `json:"write_throughput"`

	// 平均 I/O 延迟（毫秒）
	AvgIOLatency float64 `json:"avg_io_latency"`

	// I/O 等待时间（%）
	IOWaitPercent float64 `json:"io_wait_percent"`

	// 磁盘详情
	Disks []DiskDetail `json:"disks"`

	// 历史趋势
	History []DiskMetricPoint `json:"history"`
}

// DiskDetail 磁盘详情
type DiskDetail struct {
	// 设备名
	Name string `json:"name"`

	// 型号
	Model string `json:"model"`

	// 序列号
	Serial string `json:"serial"`

	// 类型
	Type string `json:"type"` // hdd, ssd, nvme

	// 容量（字节）
	Capacity uint64 `json:"capacity"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 温度（摄氏度）
	Temperature float64 `json:"temperature,omitempty"`

	// 健康状态
	HealthStatus string `json:"health_status"` // healthy, warning, critical

	// SMART 状态
	SMARTStatus string `json:"smart_status"`

	// 读 IOPS
	ReadIOPS uint64 `json:"read_iops"`

	// 写 IOPS
	WriteIOPS uint64 `json:"write_iops"`

	// 剩余寿命（%）（SSD）
	LifeRemaining int `json:"life_remaining,omitempty"`
}

// DiskMetricPoint 磁盘指标数据点
type DiskMetricPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	UsedCapacity    uint64    `json:"used_capacity"`
	UsagePercent    float64   `json:"usage_percent"`
	ReadIOPS        uint64    `json:"read_iops"`
	WriteIOPS       uint64    `json:"write_iops"`
	ReadThroughput  uint64    `json:"read_throughput"`
	WriteThroughput uint64    `json:"write_throughput"`
	AvgIOLatency    float64   `json:"avg_io_latency"`
}

// NetworkResourceInfo 网络资源信息
type NetworkResourceInfo struct {
	// 接口数量
	InterfaceCount int `json:"interface_count"`

	// 接收速率（字节/秒）
	RxBytesPerSec uint64 `json:"rx_bytes_per_sec"`

	// 发送速率（字节/秒）
	TxBytesPerSec uint64 `json:"tx_bytes_per_sec"`

	// 总带宽（Mbps）
	TotalBandwidthMbps float64 `json:"total_bandwidth_mbps"`

	// 接口详情
	Interfaces []NetworkInterfaceInfo `json:"interfaces"`

	// 连接数
	Connections int `json:"connections"`

	// TCP 连接状态
	TCPStates TCPConnectionStates `json:"tcp_states"`

	// 历史趋势
	History []NetworkMetricPoint `json:"history"`
}

// NetworkInterfaceInfo 网络接口信息
type NetworkInterfaceInfo struct {
	// 接口名
	Name string `json:"name"`

	// MAC 地址
	MAC string `json:"mac"`

	// IP 地址
	IPAddresses []string `json:"ip_addresses"`

	// 状态
	Status string `json:"status"` // up, down

	// 速率（Mbps）
	Speed int `json:"speed"`

	// 接收速率（字节/秒）
	RxBytesPerSec uint64 `json:"rx_bytes_per_sec"`

	// 发送速率（字节/秒）
	TxBytesPerSec uint64 `json:"tx_bytes_per_sec"`

	// 接收错误数
	RxErrors uint64 `json:"rx_errors"`

	// 发送错误数
	TxErrors uint64 `json:"tx_errors"`

	// 接收丢包数
	RxDropped uint64 `json:"rx_dropped"`

	// 发送丢包数
	TxDropped uint64 `json:"tx_dropped"`

	// 利用率（%）
	UtilizationPercent float64 `json:"utilization_percent"`
}

// TCPConnectionStates TCP 连接状态
type TCPConnectionStates struct {
	Established int `json:"established"`
	SynSent     int `json:"syn_sent"`
	SynRecv     int `json:"syn_recv"`
	FinWait1    int `json:"fin_wait1"`
	FinWait2    int `json:"fin_wait2"`
	TimeWait    int `json:"time_wait"`
	Close       int `json:"close"`
	CloseWait   int `json:"close_wait"`
	LastAck     int `json:"last_ack"`
	Listen      int `json:"listen"`
	Closing     int `json:"closing"`
}

// NetworkMetricPoint 网络指标数据点
type NetworkMetricPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	RxBytesPerSec uint64    `json:"rx_bytes_per_sec"`
	TxBytesPerSec uint64    `json:"tx_bytes_per_sec"`
	Connections   int       `json:"connections"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	// 总进程数
	Total int `json:"total"`

	// 运行中
	Running int `json:"running"`

	// 睡眠中
	Sleeping int `json:"sleeping"`

	// 停止
	Stopped int `json:"stopped"`

	// 僵尸进程
	Zombie int `json:"zombie"`

	// Top 进程（按 CPU）
	TopByCPU []ProcessDetail `json:"top_by_cpu"`

	// Top 进程（按内存）
	TopByMemory []ProcessDetail `json:"top_by_memory"`

	// Top 进程（按 I/O）
	TopByIO []ProcessDetail `json:"top_by_io"`
}

// ProcessDetail 进程详情
type ProcessDetail struct {
	// PID
	PID int `json:"pid"`

	// 进程名
	Name string `json:"name"`

	// 用户
	User string `json:"user"`

	// CPU 使用率
	CPUPercent float64 `json:"cpu_percent"`

	// 内存使用率
	MemoryPercent float64 `json:"memory_percent"`

	// 内存使用（字节）
	MemoryBytes uint64 `json:"memory_bytes"`

	// 状态
	Status string `json:"status"`

	// 运行时间
	Uptime int64 `json:"uptime"`

	// 命令行
	Cmdline string `json:"cmdline,omitempty"`
}

// ResourceTrends 资源趋势
type ResourceTrends struct {
	// CPU 趋势
	CPU TrendAnalysis `json:"cpu"`

	// 内存趋势
	Memory TrendAnalysis `json:"memory"`

	// 磁盘趋势
	Disk TrendAnalysis `json:"disk"`

	// 网络趋势
	Network TrendAnalysis `json:"network"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	// 数据点
	DataPoints []MetricDataPoint `json:"data_points"`

	// 趋势方向
	Direction string `json:"direction"` // up, down, stable

	// 平均值
	Average float64 `json:"average"`

	// 最大值
	Max float64 `json:"max"`

	// 最小值
	Min float64 `json:"min"`

	// 峰值时间
	PeakTime *time.Time `json:"peak_time,omitempty"`

	// 波动率
	Volatility float64 `json:"volatility"`
}

// MetricDataPoint 指标数据点
type MetricDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ResourceAlertItem 资源告警项
type ResourceAlertItem struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // cpu, memory, disk, network, process

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 资源名称
	Resource string `json:"resource"`

	// 消息
	Message string `json:"message"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 单位
	Unit string `json:"unit"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 持续时间（秒）
	Duration int64 `json:"duration"`

	// 是否已确认
	Acknowledged bool `json:"acknowledged"`

	// 建议操作
	SuggestedAction string `json:"suggested_action"`
}

// SystemRecommendation 系统建议
type SystemRecommendation struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // upgrade, optimize, cleanup, configure, monitor

	// 优先级
	Priority string `json:"priority"` // high, medium, low

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 影响
	Impact string `json:"impact"`

	// 实施难度
	Effort string `json:"effort"`

	// 详细步骤
	Steps []string `json:"steps,omitempty"`
}

// SystemHealthScore 系统健康评分
type SystemHealthScore struct {
	// 总分
	Overall int `json:"overall"` // 0-100

	// CPU 评分
	CPU int `json:"cpu"`

	// 内存评分
	Memory int `json:"memory"`

	// 磁盘评分
	Disk int `json:"disk"`

	// 网络评分
	Network int `json:"network"`

	// 状态
	Status string `json:"status"` // excellent, good, fair, poor

	// 主要问题
	MainIssues []string `json:"main_issues"`
}

// SystemResourceReporter 系统资源报告生成器
type SystemResourceReporter struct {
	config SystemReportConfig
}

// SystemReportConfig 系统报告配置
type SystemReportConfig struct {
	// CPU 高使用率阈值
	CPUHighThreshold float64 `json:"cpu_high_threshold"`

	// CPU 严重使用率阈值
	CPUCriticalThreshold float64 `json:"cpu_critical_threshold"`

	// 内存高使用率阈值
	MemoryHighThreshold float64 `json:"memory_high_threshold"`

	// 内存严重使用率阈值
	MemoryCriticalThreshold float64 `json:"memory_critical_threshold"`

	// 磁盘高使用率阈值
	DiskHighThreshold float64 `json:"disk_high_threshold"`

	// 磁盘严重使用率阈值
	DiskCriticalThreshold float64 `json:"disk_critical_threshold"`

	// 趋势历史小时数
	TrendHistoryHours int `json:"trend_history_hours"`

	// Top 进程数量
	TopProcessCount int `json:"top_process_count"`
}

// DefaultSystemReportConfig 默认系统报告配置
func DefaultSystemReportConfig() SystemReportConfig {
	return SystemReportConfig{
		CPUHighThreshold:        70.0,
		CPUCriticalThreshold:    90.0,
		MemoryHighThreshold:     80.0,
		MemoryCriticalThreshold: 95.0,
		DiskHighThreshold:       80.0,
		DiskCriticalThreshold:   90.0,
		TrendHistoryHours:       24,
		TopProcessCount:         10,
	}
}

// NewSystemResourceReporter 创建系统资源报告生成器
func NewSystemResourceReporter(config SystemReportConfig) *SystemResourceReporter {
	return &SystemResourceReporter{config: config}
}

// GenerateReport 生成系统资源报告
func (r *SystemResourceReporter) GenerateReport(
	summary SystemResourceSummary,
	cpu CPUResourceInfo,
	memory MemoryResourceInfo,
	disk DiskResourceInfo,
	network NetworkResourceInfo,
	processes ProcessInfo,
	trends ResourceTrends,
) *SystemResourceReport {
	now := time.Now()
	report := &SystemResourceReport{
		ID:          "system_" + now.Format("20060102150405"),
		Name:        "系统资源报表",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.Add(-time.Duration(r.config.TrendHistoryHours) * time.Hour),
			EndTime:   now,
		},
		Summary:   summary,
		CPU:       cpu,
		Memory:    memory,
		Disk:      disk,
		Network:   network,
		Processes: processes,
		Trends:    trends,
	}

	// 生成告警
	report.Alerts = r.generateAlerts(report)

	// 生成建议
	report.Recommendations = r.generateRecommendations(report)

	// 计算健康评分
	report.HealthScore = r.calculateHealthScore(report)

	return report
}

// generateAlerts 生成告警
func (r *SystemResourceReporter) generateAlerts(report *SystemResourceReport) []ResourceAlertItem {
	alerts := make([]ResourceAlertItem, 0)
	now := time.Now()

	// CPU 告警
	if report.CPU.UsagePercent >= r.config.CPUCriticalThreshold {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_cpu_critical",
			Type:            "cpu",
			Severity:        "critical",
			Resource:        "CPU",
			Message:         "CPU 使用率过高",
			CurrentValue:    report.CPU.UsagePercent,
			Threshold:       r.config.CPUCriticalThreshold,
			Unit:            "%",
			TriggeredAt:     now,
			SuggestedAction: "检查高 CPU 占用进程，优化或扩容",
		})
	} else if report.CPU.UsagePercent >= r.config.CPUHighThreshold {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_cpu_high",
			Type:            "cpu",
			Severity:        "warning",
			Resource:        "CPU",
			Message:         "CPU 使用率较高",
			CurrentValue:    report.CPU.UsagePercent,
			Threshold:       r.config.CPUHighThreshold,
			Unit:            "%",
			TriggeredAt:     now,
			SuggestedAction: "监控 CPU 使用趋势，考虑优化",
		})
	}

	// 内存告警
	if report.Memory.UsagePercent >= r.config.MemoryCriticalThreshold {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_memory_critical",
			Type:            "memory",
			Severity:        "critical",
			Resource:        "内存",
			Message:         "内存使用率过高",
			CurrentValue:    report.Memory.UsagePercent,
			Threshold:       r.config.MemoryCriticalThreshold,
			Unit:            "%",
			TriggeredAt:     now,
			SuggestedAction: "释放内存或增加内存容量",
		})
	} else if report.Memory.UsagePercent >= r.config.MemoryHighThreshold {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_memory_high",
			Type:            "memory",
			Severity:        "warning",
			Resource:        "内存",
			Message:         "内存使用率较高",
			CurrentValue:    report.Memory.UsagePercent,
			Threshold:       r.config.MemoryHighThreshold,
			Unit:            "%",
			TriggeredAt:     now,
			SuggestedAction: "监控内存使用，考虑扩容",
		})
	}

	// 磁盘告警
	if report.Disk.UsagePercent >= r.config.DiskCriticalThreshold {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_disk_critical",
			Type:            "disk",
			Severity:        "critical",
			Resource:        "磁盘",
			Message:         "磁盘使用率过高",
			CurrentValue:    report.Disk.UsagePercent,
			Threshold:       r.config.DiskCriticalThreshold,
			Unit:            "%",
			TriggeredAt:     now,
			SuggestedAction: "立即清理数据或扩容磁盘",
		})
	}

	// 磁盘健康告警
	for _, d := range report.Disk.Disks {
		if d.HealthStatus == "critical" {
			alerts = append(alerts, ResourceAlertItem{
				ID:              "alert_disk_health_" + d.Name,
				Type:            "disk",
				Severity:        "critical",
				Resource:        d.Name,
				Message:         "磁盘 " + d.Name + " 健康状态异常",
				CurrentValue:    0,
				Threshold:       0,
				Unit:            "",
				TriggeredAt:     now,
				SuggestedAction: "立即更换磁盘并迁移数据",
			})
		}
	}

	// 僵尸进程告警
	if report.Processes.Zombie > 10 {
		alerts = append(alerts, ResourceAlertItem{
			ID:              "alert_zombie",
			Type:            "process",
			Severity:        "warning",
			Resource:        "进程",
			Message:         "存在大量僵尸进程",
			CurrentValue:    float64(report.Processes.Zombie),
			Threshold:       10,
			Unit:            "个",
			TriggeredAt:     now,
			SuggestedAction: "检查并清理僵尸进程",
		})
	}

	return alerts
}

// generateRecommendations 生成建议
func (r *SystemResourceReporter) generateRecommendations(report *SystemResourceReport) []SystemRecommendation {
	recs := make([]SystemRecommendation, 0)

	// CPU 建议
	if report.CPU.UsagePercent >= r.config.CPUHighThreshold {
		recs = append(recs, SystemRecommendation{
			ID:          "rec_cpu_optimize",
			Type:        "optimize",
			Priority:    "high",
			Title:       "优化 CPU 使用",
			Description: "当前 CPU 使用率较高，建议检查高 CPU 占用进程",
			Impact:      "提升系统响应速度",
			Effort:      "medium",
			Steps: []string{
				"1. 查看高 CPU 进程列表",
				"2. 分析进程行为",
				"3. 优化或限制资源使用",
			},
		})
	}

	// 内存建议
	if report.Memory.UsagePercent >= r.config.MemoryHighThreshold {
		recs = append(recs, SystemRecommendation{
			ID:          "rec_memory",
			Type:        "upgrade",
			Priority:    "high",
			Title:       "增加内存",
			Description: "当前内存使用率较高，建议增加内存容量",
			Impact:      "提升系统性能，减少 OOM 风险",
			Effort:      "medium",
		})
	}

	// 磁盘建议
	if report.Disk.UsagePercent >= r.config.DiskHighThreshold {
		recs = append(recs, SystemRecommendation{
			ID:          "rec_disk",
			Type:        "cleanup",
			Priority:    "high",
			Title:       "清理磁盘空间",
			Description: "磁盘使用率较高，建议清理不必要的数据",
			Impact:      "释放存储空间，避免磁盘满",
			Effort:      "easy",
			Steps: []string{
				"1. 扫描大文件",
				"2. 清理日志文件",
				"3. 删除过期备份",
				"4. 清理包管理器缓存",
			},
		})
	}

	// 系统监控建议
	recs = append(recs, SystemRecommendation{
		ID:          "rec_monitor",
		Type:        "monitor",
		Priority:    "low",
		Title:       "持续监控",
		Description: "建议持续监控系统资源使用情况",
		Impact:      "及时发现问题，预防性能问题",
		Effort:      "easy",
	})

	// 按优先级排序
	priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(recs, func(i, j int) bool {
		return priorityOrder[recs[i].Priority] < priorityOrder[recs[j].Priority]
	})

	return recs
}

// calculateHealthScore 计算健康评分
func (r *SystemResourceReporter) calculateHealthScore(report *SystemResourceReport) SystemHealthScore {
	score := SystemHealthScore{
		MainIssues: make([]string, 0),
	}

	// CPU 评分
	if report.CPU.UsagePercent < 50 {
		score.CPU = 100
	} else if report.CPU.UsagePercent < 70 {
		score.CPU = 80
	} else if report.CPU.UsagePercent < 90 {
		score.CPU = 60
	} else {
		score.CPU = 30
		score.MainIssues = append(score.MainIssues, "CPU 使用率过高")
	}

	// 内存评分
	if report.Memory.UsagePercent < 60 {
		score.Memory = 100
	} else if report.Memory.UsagePercent < 80 {
		score.Memory = 80
	} else if report.Memory.UsagePercent < 95 {
		score.Memory = 50
		score.MainIssues = append(score.MainIssues, "内存使用率较高")
	} else {
		score.Memory = 20
		score.MainIssues = append(score.MainIssues, "内存不足")
	}

	// 磁盘评分
	if report.Disk.UsagePercent < 60 {
		score.Disk = 100
	} else if report.Disk.UsagePercent < 80 {
		score.Disk = 80
	} else if report.Disk.UsagePercent < 90 {
		score.Disk = 50
		score.MainIssues = append(score.MainIssues, "磁盘空间不足")
	} else {
		score.Disk = 20
		score.MainIssues = append(score.MainIssues, "磁盘空间严重不足")
	}

	// 网络评分（基于错误率）
	score.Network = 90 // 默认良好

	// 总分
	score.Overall = (score.CPU + score.Memory + score.Disk + score.Network) / 4

	// 状态
	if score.Overall >= 90 {
		score.Status = "excellent"
	} else if score.Overall >= 75 {
		score.Status = "good"
	} else if score.Overall >= 50 {
		score.Status = "fair"
	} else {
		score.Status = "poor"
	}

	return score
}
