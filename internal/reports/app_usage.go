// Package reports 提供应用资源统计功能 (v2.90.0 户部)
package reports

import (
	"context"
	"sync"
	"time"
)

// ========== 应用资源统计类型定义 ==========

// AppUsageType 应用资源类型.
type AppUsageType string

const (
	// AppUsageCPU represents CPU usage type.
	AppUsageCPU AppUsageType = "cpu" // CPU使用
	// AppUsageMemory represents memory usage type.
	AppUsageMemory AppUsageType = "memory" // 内存使用
	// AppUsageStorage represents storage usage type.
	AppUsageStorage AppUsageType = "storage" // 存储占用
	// AppUsageNetwork represents network usage type.
	AppUsageNetwork AppUsageType = "network" // 网络流量
)

// AppUsageRecord 单次应用资源使用记录.
type AppUsageRecord struct {
	// 应用ID
	AppID string `json:"app_id"`

	// 应用名称
	AppName string `json:"app_name"`

	// 容器ID（Docker环境）
	ContainerID string `json:"container_id,omitempty"`

	// 资源类型
	ResourceType AppUsageType `json:"resource_type"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// CPU使用率（百分比）
	CPUPercent float64 `json:"cpu_percent"`

	// CPU核心数
	CPUCores float64 `json:"cpu_cores"`

	// 内存使用量（字节）
	MemoryBytes uint64 `json:"memory_bytes"`

	// 内存限制（字节）
	MemoryLimit uint64 `json:"memory_limit,omitempty"`

	// 内存使用率（百分比）
	MemoryPercent float64 `json:"memory_percent"`

	// 存储读取量（字节）
	StorageReadBytes uint64 `json:"storage_read_bytes"`

	// 存储写入量（字节）
	StorageWriteBytes uint64 `json:"storage_write_bytes"`

	// 存储占用总量（字节）
	StorageUsedBytes uint64 `json:"storage_used_bytes"`

	// 网络接收量（字节）
	NetworkRxBytes uint64 `json:"network_rx_bytes"`

	// 网络发送量（字节）
	NetworkTxBytes uint64 `json:"network_tx_bytes"`

	// 网络接收包数
	NetworkRxPackets uint64 `json:"network_rx_packets"`

	// 网络发送包数
	NetworkTxPackets uint64 `json:"network_tx_packets"`

	// 运行状态
	Status string `json:"status"` // running, stopped, error

	// 附加标签
	Labels map[string]string `json:"labels,omitempty"`
}

// AppUsageSummary 应用资源使用汇总.
type AppUsageSummary struct {
	// 应用ID
	AppID string `json:"app_id"`

	// 应用名称
	AppName string `json:"app_name"`

	// 统计周期开始
	PeriodStart time.Time `json:"period_start"`

	// 统计周期结束
	PeriodEnd time.Time `json:"period_end"`

	// CPU统计
	CPU CPUUsageStats `json:"cpu"`

	// 内存统计
	Memory MemoryUsageStats `json:"memory"`

	// 存储统计
	Storage StorageUsageStats `json:"storage"`

	// 网络统计
	Network NetworkUsageStats `json:"network"`

	// 成本估算
	CostEstimate CostEstimate `json:"cost_estimate"`

	// 告警
	Alerts []UsageAlert `json:"alerts,omitempty"`

	// 建议
	Recommendations []UsageRecommendation `json:"recommendations,omitempty"`
}

// CPUUsageStats CPU使用统计.
type CPUUsageStats struct {
	// 平均使用率
	AvgPercent float64 `json:"avg_percent"`

	// 峰值使用率
	PeakPercent float64 `json:"peak_percent"`

	// 最低使用率
	MinPercent float64 `json:"min_percent"`

	// 平均核心数
	AvgCores float64 `json:"avg_cores"`

	// 峰值核心数
	PeakCores float64 `json:"peak_cores"`

	// CPU时间（秒）
	TotalCPUTimeSeconds float64 `json:"total_cpu_time_seconds"`

	// 采样次数
	SampleCount int `json:"sample_count"`
}

// MemoryUsageStats 内存使用统计.
type MemoryUsageStats struct {
	// 平均使用量（字节）
	AvgBytes uint64 `json:"avg_bytes"`

	// 峰值使用量（字节）
	PeakBytes uint64 `json:"peak_bytes"`

	// 最低使用量（字节）
	MinBytes uint64 `json:"min_bytes"`

	// 平均使用率
	AvgPercent float64 `json:"avg_percent"`

	// 峰值使用率
	PeakPercent float64 `json:"peak_percent"`

	// 内存限制（字节）
	LimitBytes uint64 `json:"limit_bytes,omitempty"`

	// OOM次数
	OOMCount int `json:"oom_count"`

	// 采样次数
	SampleCount int `json:"sample_count"`
}

// StorageUsageStats 存储使用统计.
type StorageUsageStats struct {
	// 总读取量（字节）
	TotalReadBytes uint64 `json:"total_read_bytes"`

	// 总写入量（字节）
	TotalWriteBytes uint64 `json:"total_write_bytes"`

	// 平均读取速率（字节/秒）
	AvgReadBPS float64 `json:"avg_read_bps"`

	// 平均写入速率（字节/秒）
	AvgWriteBPS float64 `json:"avg_write_bps"`

	// 峰值读取速率（字节/秒）
	PeakReadBPS float64 `json:"peak_read_bps"`

	// 峰值写入速率（字节/秒）
	PeakWriteBPS float64 `json:"peak_write_bps"`

	// 当前存储占用（字节）
	CurrentUsedBytes uint64 `json:"current_used_bytes"`

	// IOPS读取
	IOPSRead uint64 `json:"iops_read"`

	// IOPS写入
	IOPSWrite uint64 `json:"iops_write"`

	// 采样次数
	SampleCount int `json:"sample_count"`
}

// NetworkUsageStats 网络使用统计.
type NetworkUsageStats struct {
	// 总接收量（字节）
	TotalRxBytes uint64 `json:"total_rx_bytes"`

	// 总发送量（字节）
	TotalTxBytes uint64 `json:"total_tx_bytes"`

	// 平均接收速率（字节/秒）
	AvgRxBPS float64 `json:"avg_rx_bps"`

	// 平均发送速率（字节/秒）
	AvgTxBPS float64 `json:"avg_tx_bps"`

	// 峰值接收速率（字节/秒）
	PeakRxBPS float64 `json:"peak_rx_bps"`

	// 峰值发送速率（字节/秒）
	PeakTxBPS float64 `json:"peak_tx_bps"`

	// 总接收包数
	TotalRxPackets uint64 `json:"total_rx_packets"`

	// 总发送包数
	TotalTxPackets uint64 `json:"total_tx_packets"`

	// 连接数
	ConnectionCount int `json:"connection_count"`

	// 采样次数
	SampleCount int `json:"sample_count"`
}

// CostEstimate 成本估算.
type CostEstimate struct {
	// CPU成本（元）
	CPUCost float64 `json:"cpu_cost"`

	// 内存成本（元）
	MemoryCost float64 `json:"memory_cost"`

	// 存储成本（元）
	StorageCost float64 `json:"storage_cost"`

	// 网络成本（元）
	NetworkCost float64 `json:"network_cost"`

	// 总成本（元）
	TotalCost float64 `json:"total_cost"`

	// 成本货币
	Currency string `json:"currency"`

	// 计费周期
	BillingPeriod string `json:"billing_period"`

	// 定价模型
	PricingModel string `json:"pricing_model"`
}

// UsageAlert 使用告警.
type UsageAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 告警类型
	Type string `json:"type"` // cpu_high, memory_high, storage_high, network_high

	// 告警级别
	Level string `json:"level"` // warning, critical

	// 告警消息
	Message string `json:"message"`

	// 触发值
	Value float64 `json:"value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 是否已处理
	Resolved bool `json:"resolved"`

	// 处理时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// UsageRecommendation 使用建议.
type UsageRecommendation struct {
	// 建议类型
	Type string `json:"type"` // scale_down, scale_up, optimize, migrate

	// 优先级
	Priority int `json:"priority"` // 1-5, 1最高

	// 建议标题
	Title string `json:"title"`

	// 建议描述
	Description string `json:"description"`

	// 预计节省
	EstimatedSavings float64 `json:"estimated_savings,omitempty"`

	// 影响分析
	Impact string `json:"impact"`

	// 实施步骤
	Steps []string `json:"steps,omitempty"`
}

// AppUsageConfig 应用资源统计配置.
type AppUsageConfig struct {
	// 采集间隔（秒）
	CollectInterval int `json:"collect_interval"`

	// 汇总周期（分钟）
	SummaryPeriodMinutes int `json:"summary_period_minutes"`

	// 数据保留天数
	RetentionDays int `json:"retention_days"`

	// CPU告警阈值（百分比）
	CPUAlertThreshold float64 `json:"cpu_alert_threshold"`

	// 内存告警阈值（百分比）
	MemoryAlertThreshold float64 `json:"memory_alert_threshold"`

	// 存储告警阈值（百分比）
	StorageAlertThreshold float64 `json:"storage_alert_threshold"`

	// 网络带宽告警阈值（Mbps）
	NetworkAlertThreshold float64 `json:"network_alert_threshold"`

	// 是否启用成本估算
	EnableCostEstimate bool `json:"enable_cost_estimate"`

	// 定价配置
	PricingConfig PricingConfig `json:"pricing_config"`
}

// PricingConfig 定价配置.
type PricingConfig struct {
	// CPU单价（元/核心/小时）
	CPUCorePerHour float64 `json:"cpu_core_per_hour"`

	// 内存单价（元/GB/小时）
	MemoryGBPerHour float64 `json:"memory_gb_per_hour"`

	// 存储单价（元/GB/月）
	StorageGBPerMonth float64 `json:"storage_gb_per_month"`

	// 网络出流量单价（元/GB）
	NetworkOutGB float64 `json:"network_out_gb"`

	// 网络入流量单价（元/GB）
	NetworkInGB float64 `json:"network_in_gb"`
}

// ========== 应用资源统计服务 ==========

// AppUsageCollector 应用资源采集器.
type AppUsageCollector struct {
	config     AppUsageConfig
	records    []AppUsageRecord
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	collectors map[string]UsageDataSource
}

// UsageDataSource 资源数据源接口.
type UsageDataSource interface {
	// Name 数据源名称
	Name() string

	// Collect 采集资源数据
	Collect(ctx context.Context) ([]AppUsageRecord, error)

	// IsAvailable 检查数据源是否可用
	IsAvailable() bool
}

// NewAppUsageCollector 创建应用资源采集器.
func NewAppUsageCollector(config AppUsageConfig) *AppUsageCollector {
	ctx, cancel := context.WithCancel(context.Background())

	// 默认配置
	if config.CollectInterval <= 0 {
		config.CollectInterval = 60 // 默认60秒采集
	}
	if config.SummaryPeriodMinutes <= 0 {
		config.SummaryPeriodMinutes = 5
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = 30
	}
	if config.CPUAlertThreshold <= 0 {
		config.CPUAlertThreshold = 80
	}
	if config.MemoryAlertThreshold <= 0 {
		config.MemoryAlertThreshold = 85
	}
	if config.StorageAlertThreshold <= 0 {
		config.StorageAlertThreshold = 90
	}
	if config.NetworkAlertThreshold <= 0 {
		config.NetworkAlertThreshold = 1000 // 默认1000Mbps
	}

	return &AppUsageCollector{
		config:     config,
		records:    make([]AppUsageRecord, 0),
		ctx:        ctx,
		cancel:     cancel,
		collectors: make(map[string]UsageDataSource),
	}
}

// RegisterDataSource 注册数据源.
func (c *AppUsageCollector) RegisterDataSource(source UsageDataSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collectors[source.Name()] = source
}

// Start 启动采集.
func (c *AppUsageCollector) Start() {
	go c.collectLoop()
}

// Stop 停止采集.
func (c *AppUsageCollector) Stop() {
	c.cancel()
}

// collectLoop 采集循环.
func (c *AppUsageCollector) collectLoop() {
	ticker := time.NewTicker(time.Duration(c.config.CollectInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

// collect 执行采集.
func (c *AppUsageCollector) collect() {
	c.mu.RLock()
	sources := c.collectors
	c.mu.RUnlock()

	for _, source := range sources {
		if !source.IsAvailable() {
			continue
		}

		records, err := source.Collect(c.ctx)
		if err != nil {
			continue
		}

		c.mu.Lock()
		c.records = append(c.records, records...)
		c.mu.Unlock()
	}
}

// Record 记录单条数据.
func (c *AppUsageCollector) Record(record AppUsageRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, record)
}

// GetRecords 获取记录.
func (c *AppUsageCollector) GetRecords(appID string, start, end time.Time) []AppUsageRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []AppUsageRecord
	for _, r := range c.records {
		if appID != "" && r.AppID != appID {
			continue
		}
		if !start.IsZero() && r.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && r.Timestamp.After(end) {
			continue
		}
		result = append(result, r)
	}
	return result
}

// GetSummary 获取汇总统计.
func (c *AppUsageCollector) GetSummary(appID string, start, end time.Time) (*AppUsageSummary, error) {
	records := c.GetRecords(appID, start, end)
	if len(records) == 0 {
		return nil, nil
	}

	summary := &AppUsageSummary{
		AppID:       appID,
		AppName:     records[0].AppName,
		PeriodStart: start,
		PeriodEnd:   end,
		Alerts:      make([]UsageAlert, 0),
		Recommendations: make([]UsageRecommendation, 0),
	}

	// 计算CPU统计
	summary.CPU = c.calculateCPUStats(records)

	// 计算内存统计
	summary.Memory = c.calculateMemoryStats(records)

	// 计算存储统计
	summary.Storage = c.calculateStorageStats(records)

	// 计算网络统计
	summary.Network = c.calculateNetworkStats(records)

	// 计算成本估算
	if c.config.EnableCostEstimate {
		summary.CostEstimate = c.calculateCostEstimate(&summary.CPU, &summary.Memory, &summary.Storage, &summary.Network)
	}

	// 检查告警
	c.checkAlerts(summary)

	// 生成建议
	c.generateRecommendations(summary)

	return summary, nil
}

// calculateCPUStats 计算CPU统计.
func (c *AppUsageCollector) calculateCPUStats(records []AppUsageRecord) CPUUsageStats {
	stats := CPUUsageStats{
		MinPercent:  100,
		SampleCount: len(records),
	}

	var totalPercent float64
	var totalTime float64

	for _, r := range records {
		totalPercent += r.CPUPercent
		totalTime += r.CPUCores * float64(c.config.CollectInterval)

		if r.CPUPercent > stats.PeakPercent {
			stats.PeakPercent = r.CPUPercent
			stats.PeakCores = r.CPUCores
		}
		if r.CPUPercent < stats.MinPercent {
			stats.MinPercent = r.CPUPercent
		}
	}

	if len(records) > 0 {
		stats.AvgPercent = totalPercent / float64(len(records))
		stats.AvgCores = stats.PeakCores // 简化计算
	}
	stats.TotalCPUTimeSeconds = totalTime

	return stats
}

// calculateMemoryStats 计算内存统计.
func (c *AppUsageCollector) calculateMemoryStats(records []AppUsageRecord) MemoryUsageStats {
	stats := MemoryUsageStats{
		MinBytes:    ^uint64(0), // 最大值
		SampleCount: len(records),
	}

	var totalBytes uint64
	var totalPercent float64

	for _, r := range records {
		totalBytes += r.MemoryBytes
		totalPercent += r.MemoryPercent

		if r.MemoryBytes > stats.PeakBytes {
			stats.PeakBytes = r.MemoryBytes
		}
		if r.MemoryBytes < stats.MinBytes {
			stats.MinBytes = r.MemoryBytes
		}
		if r.MemoryLimit > stats.LimitBytes {
			stats.LimitBytes = r.MemoryLimit
		}
	}

	if len(records) > 0 {
		stats.AvgBytes = totalBytes / uint64(len(records))
		stats.AvgPercent = totalPercent / float64(len(records))
		stats.PeakPercent = float64(stats.PeakBytes) / float64(stats.LimitBytes) * 100
	}

	return stats
}

// calculateStorageStats 计算存储统计.
func (c *AppUsageCollector) calculateStorageStats(records []AppUsageRecord) StorageUsageStats {
	stats := StorageUsageStats{
		SampleCount: len(records),
	}

	var lastRecord AppUsageRecord
	for i, r := range records {
		stats.TotalReadBytes += r.StorageReadBytes
		stats.TotalWriteBytes += r.StorageWriteBytes

		if i > 0 {
			elapsed := r.Timestamp.Sub(lastRecord.Timestamp).Seconds()
			if elapsed > 0 {
				readBPS := float64(r.StorageReadBytes-lastRecord.StorageReadBytes) / elapsed
				writeBPS := float64(r.StorageWriteBytes-lastRecord.StorageWriteBytes) / elapsed
				if readBPS > stats.PeakReadBPS {
					stats.PeakReadBPS = readBPS
				}
				if writeBPS > stats.PeakWriteBPS {
					stats.PeakWriteBPS = writeBPS
				}
			}
		}

		if r.StorageUsedBytes > stats.CurrentUsedBytes {
			stats.CurrentUsedBytes = r.StorageUsedBytes
		}
		lastRecord = r
	}

	// 计算平均速率
	duration := records[len(records)-1].Timestamp.Sub(records[0].Timestamp).Seconds()
	if duration > 0 {
		stats.AvgReadBPS = float64(stats.TotalReadBytes) / duration
		stats.AvgWriteBPS = float64(stats.TotalWriteBytes) / duration
	}

	return stats
}

// calculateNetworkStats 计算网络统计.
func (c *AppUsageCollector) calculateNetworkStats(records []AppUsageRecord) NetworkUsageStats {
	stats := NetworkUsageStats{
		SampleCount: len(records),
	}

	var lastRecord AppUsageRecord
	for i, r := range records {
		stats.TotalRxBytes += r.NetworkRxBytes
		stats.TotalTxBytes += r.NetworkTxBytes
		stats.TotalRxPackets += r.NetworkRxPackets
		stats.TotalTxPackets += r.NetworkTxPackets

		if i > 0 {
			elapsed := r.Timestamp.Sub(lastRecord.Timestamp).Seconds()
			if elapsed > 0 {
				rxBPS := float64(r.NetworkRxBytes-lastRecord.NetworkRxBytes) / elapsed
				txBPS := float64(r.NetworkTxBytes-lastRecord.NetworkTxBytes) / elapsed
				if rxBPS > stats.PeakRxBPS {
					stats.PeakRxBPS = rxBPS
				}
				if txBPS > stats.PeakTxBPS {
					stats.PeakTxBPS = txBPS
				}
			}
		}
		lastRecord = r
	}

	// 计算平均速率
	duration := records[len(records)-1].Timestamp.Sub(records[0].Timestamp).Seconds()
	if duration > 0 {
		stats.AvgRxBPS = float64(stats.TotalRxBytes) / duration
		stats.AvgTxBPS = float64(stats.TotalTxBytes) / duration
	}

	return stats
}

// calculateCostEstimate 计算成本估算.
func (c *AppUsageCollector) calculateCostEstimate(cpu *CPUUsageStats, mem *MemoryUsageStats, storage *StorageUsageStats, network *NetworkUsageStats) CostEstimate {
	pricing := c.config.PricingConfig

	// CPU成本（核心*小时*单价）
	hours := float64(cpu.SampleCount * c.config.CollectInterval) / 3600
	cpuCost := cpu.AvgCores * hours * pricing.CPUCorePerHour

	// 内存成本（GB*小时*单价）
	memGB := float64(mem.AvgBytes) / (1024 * 1024 * 1024)
	memCost := memGB * hours * pricing.MemoryGBPerHour

	// 存储成本（GB*月*单价/月小时数）
	storageGB := float64(storage.CurrentUsedBytes) / (1024 * 1024 * 1024)
	storageCost := storageGB * pricing.StorageGBPerMonth * hours / (30 * 24)

	// 网络成本（出流量*单价 + 入流量*单价）
	networkCost := float64(network.TotalTxBytes)/(1024*1024*1024)*pricing.NetworkOutGB +
		float64(network.TotalRxBytes)/(1024*1024*1024)*pricing.NetworkInGB

	return CostEstimate{
		CPUCost:       cpuCost,
		MemoryCost:    memCost,
		StorageCost:   storageCost,
		NetworkCost:   networkCost,
		TotalCost:     cpuCost + memCost + storageCost + networkCost,
		Currency:      "CNY",
		BillingPeriod: "hourly",
		PricingModel:  "usage-based",
	}
}

// checkAlerts 检查告警.
func (c *AppUsageCollector) checkAlerts(summary *AppUsageSummary) {
	now := time.Now()

	// CPU告警
	if summary.CPU.PeakPercent > c.config.CPUAlertThreshold {
		level := "warning"
		if summary.CPU.PeakPercent > c.config.CPUAlertThreshold+10 {
			level = "critical"
		}
		summary.Alerts = append(summary.Alerts, UsageAlert{
			ID:         "cpu-high-" + summary.AppID,
			Type:       "cpu_high",
			Level:      level,
			Message:    "CPU使用率超过阈值",
			Value:      summary.CPU.PeakPercent,
			Threshold:  c.config.CPUAlertThreshold,
			TriggeredAt: now,
		})
	}

	// 内存告警
	if summary.Memory.PeakPercent > c.config.MemoryAlertThreshold {
		level := "warning"
		if summary.Memory.PeakPercent > c.config.MemoryAlertThreshold+10 {
			level = "critical"
		}
		summary.Alerts = append(summary.Alerts, UsageAlert{
			ID:         "memory-high-" + summary.AppID,
			Type:       "memory_high",
			Level:      level,
			Message:    "内存使用率超过阈值",
			Value:      summary.Memory.PeakPercent,
			Threshold:  c.config.MemoryAlertThreshold,
			TriggeredAt: now,
		})
	}
}

// generateRecommendations 生成建议.
func (c *AppUsageCollector) generateRecommendations(summary *AppUsageSummary) {
	// 低CPU使用建议缩容
	if summary.CPU.AvgPercent < 10 && summary.CPU.PeakPercent < 30 {
		summary.Recommendations = append(summary.Recommendations, UsageRecommendation{
			Type:             "scale_down",
			Priority:         3,
			Title:            "建议降低CPU配置",
			Description:      "应用CPU使用率长期较低，建议降低CPU配置以节省成本",
			EstimatedSavings: summary.CostEstimate.CPUCost * 0.5,
			Impact:           "低风险，可能影响峰值性能",
			Steps: []string{
				"分析近期CPU使用趋势",
				"评估降配方案",
				"实施降配并观察",
			},
		})
	}

	// 高内存使用建议扩容
	if summary.Memory.PeakPercent > 90 {
		summary.Recommendations = append(summary.Recommendations, UsageRecommendation{
			Type:         "scale_up",
			Priority:     1,
			Title:        "建议增加内存配置",
			Description:  "应用内存使用率接近上限，建议增加内存以避免OOM",
			Impact:       "高风险，可能导致服务中断",
			Steps: []string{
				"分析内存使用模式",
				"检查内存泄漏",
				"评估扩容方案",
			},
		})
	}
}

// Cleanup 清理过期数据.
func (c *AppUsageCollector) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -c.config.RetentionDays)
	var validRecords []AppUsageRecord
	for _, r := range c.records {
		if r.Timestamp.After(cutoff) {
			validRecords = append(validRecords, r)
		}
	}
	c.records = validRecords
}

// GetAllAppSummaries 获取所有应用汇总.
func (c *AppUsageCollector) GetAllAppSummaries(start, end time.Time) ([]AppUsageSummary, error) {
	records := c.GetRecords("", start, end)

	// 按应用分组
	appRecords := make(map[string][]AppUsageRecord)
	for _, r := range records {
		appRecords[r.AppID] = append(appRecords[r.AppID], r)
	}

	var summaries []AppUsageSummary
	for appID := range appRecords {
		summary, err := c.GetSummary(appID, start, end)
		if err != nil {
			continue
		}
		if summary != nil {
			summaries = append(summaries, *summary)
		}
	}

	return summaries, nil
}