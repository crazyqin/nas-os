// Package monitor 提供分布式监控功能
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DistributedMonitor 分布式监控管理器
type DistributedMonitor struct {
	mu              sync.RWMutex
	localManager    *Manager
	localCollector  *MetricsCollector
	alertManager    *AlertingManager
	nodes           map[string]*NodeMetrics
	clusterNodes    []ClusterNodeInfo
	reporters       map[string]*MetricReporter
	aggregationRule AggregationRule
	alertRules      []StoragePoolAlertRule
	httpClient      *http.Client
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// ClusterNodeInfo 集群节点信息
type ClusterNodeInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	IsActive bool   `json:"is_active"`
	IsLeader bool   `json:"is_leader"`
}

// NodeMetrics 节点指标
type NodeMetrics struct {
	NodeID         string              `json:"node_id"`
	NodeName       string              `json:"node_name"`
	Timestamp      time.Time           `json:"timestamp"`
	SystemMetrics  *SystemMetricData   `json:"system_metrics"`
	DiskMetrics    []DiskMetricData    `json:"disk_metrics"`
	StorageMetrics []StoragePoolMetric `json:"storage_metrics"`
	NetworkMetrics *NetworkMetricData  `json:"network_metrics"`
	HealthScore    float64             `json:"health_score"`
	Status         string              `json:"status"`
}

// SystemMetricData 系统指标数据
type SystemMetricData struct {
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
	MemoryTotal  uint64  `json:"memory_total"`
	MemoryUsed   uint64  `json:"memory_used"`
	SwapUsage    float64 `json:"swap_usage"`
	LoadAvg1     float64 `json:"load_avg_1"`
	LoadAvg5     float64 `json:"load_avg_5"`
	LoadAvg15    float64 `json:"load_avg_15"`
	UptimeSecs   uint64  `json:"uptime_secs"`
	ProcessCount int     `json:"process_count"`
}

// DiskMetricData 磁盘指标数据
type DiskMetricData struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	ReadBytes    uint64  `json:"read_bytes"`
	WriteBytes   uint64  `json:"write_bytes"`
	ReadOps      uint64  `json:"read_ops"`
	WriteOps     uint64  `json:"write_ops"`
	IOLatency    float64 `json:"io_latency_ms"`
	Temperature  int     `json:"temperature"`
	HealthStatus string  `json:"health_status"`
}

// StoragePoolMetric 存储池指标
type StoragePoolMetric struct {
	PoolName       string           `json:"pool_name"`
	PoolType       string           `json:"pool_type"` // btrfs, zfs, mdadm, etc.
	TotalBytes     uint64           `json:"total_bytes"`
	UsedBytes      uint64           `json:"used_bytes"`
	FreeBytes      uint64           `json:"free_bytes"`
	UsagePercent   float64          `json:"usage_percent"`
	HealthStatus   string           `json:"health_status"`
	RAIDLevel      string           `json:"raid_level"`
	DeviceCount    int              `json:"device_count"`
	HealthyDevices int              `json:"healthy_devices"`
	FailedDevices  int              `json:"failed_devices"`
	IOStats        *StorageIOStats  `json:"io_stats"`
	RebuildStatus  *RebuildStatus   `json:"rebuild_status,omitempty"`
	Alerts         []PoolAlertState `json:"alerts,omitempty"`
}

// StorageIOStats 存储 IO 统计
type StorageIOStats struct {
	ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
	WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
	ReadIOPS         float64 `json:"read_iops"`
	WriteIOPS        float64 `json:"write_iops"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// RebuildStatus 重建状态
type RebuildStatus struct {
	IsActive   bool    `json:"is_active"`
	Progress   float64 `json:"progress"`
	ETASeconds int     `json:"eta_seconds"`
	StartTime  string  `json:"start_time,omitempty"`
}

// PoolAlertState 存储池告警状态
type PoolAlertState struct {
	AlertType   string    `json:"alert_type"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// NetworkMetricData 网络指标数据
type NetworkMetricData struct {
	RXBytes      uint64  `json:"rx_bytes"`
	TXBytes      uint64  `json:"tx_bytes"`
	RXPackets    uint64  `json:"rx_packets"`
	TXPackets    uint64  `json:"tx_packets"`
	RXErrors     uint64  `json:"rx_errors"`
	TXErrors     uint64  `json:"tx_errors"`
	RXDrop       uint64  `json:"rx_drop"`
	TXDrop       uint64  `json:"tx_drop"`
	BandwidthIn  float64 `json:"bandwidth_in_mbps"`
	BandwidthOut float64 `json:"bandwidth_out_mbps"`
}

// AggregationRule 指标聚合规则
type AggregationRule struct {
	Interval        time.Duration       `json:"interval"`
	RetentionPeriod time.Duration       `json:"retention_period"`
	MaxNodes        int                 `json:"max_nodes"`
	Aggregations    []MetricAggregation `json:"aggregations"`
}

// MetricAggregation 指标聚合配置
type MetricAggregation struct {
	MetricName string `json:"metric_name"`
	Method     string `json:"method"` // avg, max, min, sum, percentile
	WindowSize string `json:"window_size"`
}

// MetricReporter 指标上报器
type MetricReporter struct {
	NodeID     string
	Endpoint   string
	Interval   time.Duration
	Enabled    bool
	LastReport time.Time
	LastError  error
}

// StoragePoolAlertRule 存储池告警规则
type StoragePoolAlertRule struct {
	Name            string        `json:"name"`
	PoolPattern     string        `json:"pool_pattern"` // 支持通配符匹配
	MetricType      string        `json:"metric_type"`  // usage, health, io_latency, device_failure
	Threshold       float64       `json:"threshold"`
	Duration        time.Duration `json:"duration"` // 持续时间阈值
	Level           string        `json:"level"`    // warning, critical
	Enabled         bool          `json:"enabled"`
	MessageTemplate string        `json:"message_template"`
}

// AggregatedMetrics 聚合后的指标
type AggregatedMetrics struct {
	Timestamp     time.Time              `json:"timestamp"`
	NodeCount     int                    `json:"node_count"`
	ActiveNodes   int                    `json:"active_nodes"`
	ClusterHealth float64                `json:"cluster_health"`
	TotalCPU      float64                `json:"total_cpu_avg"`
	TotalMemory   uint64                 `json:"total_memory"`
	UsedMemory    uint64                 `json:"used_memory"`
	TotalDisk     uint64                 `json:"total_disk"`
	UsedDisk      uint64                 `json:"used_disk"`
	StoragePools  []AggregatedPoolMetric `json:"storage_pools"`
	NetworkTotal  NetworkMetricData      `json:"network_total"`
	NodeMetrics   []*NodeMetrics         `json:"node_metrics"`
	Alerts        []ClusterAlert         `json:"alerts"`
}

// AggregatedPoolMetric 聚合的存储池指标
type AggregatedPoolMetric struct {
	PoolName       string  `json:"pool_name"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
	HealthStatus   string  `json:"health_status"`
	FaultTolerance int     `json:"fault_tolerance"`
	AlertCount     int     `json:"alert_count"`
}

// ClusterAlert 集群告警
type ClusterAlert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // node_down, pool_degraded, high_usage, io_latency
	Level        string    `json:"level"`
	Message      string    `json:"message"`
	NodeID       string    `json:"node_id,omitempty"`
	PoolName     string    `json:"pool_name,omitempty"`
	CurrentValue float64   `json:"current_value,omitempty"`
	Threshold    float64   `json:"threshold,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewDistributedMonitor 创建分布式监控管理器
func NewDistributedMonitor(localManager *Manager, collector *MetricsCollector, alertManager *AlertingManager) *DistributedMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	dm := &DistributedMonitor{
		localManager:   localManager,
		localCollector: collector,
		alertManager:   alertManager,
		nodes:          make(map[string]*NodeMetrics),
		clusterNodes:   make([]ClusterNodeInfo, 0),
		reporters:      make(map[string]*MetricReporter),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		ctx:            ctx,
		cancel:         cancel,
		aggregationRule: AggregationRule{
			Interval:        time.Minute,
			RetentionPeriod: 24 * time.Hour,
			MaxNodes:        64,
			Aggregations: []MetricAggregation{
				{MetricName: "cpu_usage", Method: "avg", WindowSize: "5m"},
				{MetricName: "memory_usage", Method: "avg", WindowSize: "5m"},
				{MetricName: "disk_usage", Method: "max", WindowSize: "5m"},
			},
		},
		alertRules: DefaultStoragePoolAlertRules(),
	}

	return dm
}

// DefaultStoragePoolAlertRules 默认存储池告警规则
func DefaultStoragePoolAlertRules() []StoragePoolAlertRule {
	return []StoragePoolAlertRule{
		{
			Name:            "pool_usage_warning",
			PoolPattern:     "*",
			MetricType:      "usage",
			Threshold:       80,
			Duration:        5 * time.Minute,
			Level:           "warning",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} 使用率达到 {{.Value}}%，超过阈值 80%",
		},
		{
			Name:            "pool_usage_critical",
			PoolPattern:     "*",
			MetricType:      "usage",
			Threshold:       95,
			Duration:        1 * time.Minute,
			Level:           "critical",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} 使用率达到 {{.Value}}%，严重不足！",
		},
		{
			Name:            "pool_health_degraded",
			PoolPattern:     "*",
			MetricType:      "health",
			Threshold:       1, // 1 表示降级
			Duration:        0, // 立即告警
			Level:           "critical",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} 健康状态降级，请立即检查！",
		},
		{
			Name:            "pool_device_failure",
			PoolPattern:     "*",
			MetricType:      "device_failure",
			Threshold:       1,
			Duration:        0,
			Level:           "critical",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} 检测到 {{.Value}} 个设备故障",
		},
		{
			Name:            "pool_io_latency_high",
			PoolPattern:     "*",
			MetricType:      "io_latency",
			Threshold:       50, // 50ms
			Duration:        3 * time.Minute,
			Level:           "warning",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} IO 延迟过高: {{.Value}}ms",
		},
		{
			Name:            "pool_io_latency_critical",
			PoolPattern:     "*",
			MetricType:      "io_latency",
			Threshold:       100, // 100ms
			Duration:        1 * time.Minute,
			Level:           "critical",
			Enabled:         true,
			MessageTemplate: "存储池 {{.PoolName}} IO 延迟严重: {{.Value}}ms，可能影响服务",
		},
	}
}

// Start 启动分布式监控
func (dm *DistributedMonitor) Start() error {
	dm.wg.Add(1)
	go dm.monitorLoop()

	dm.wg.Add(1)
	go dm.reportLoop()

	dm.wg.Add(1)
	go dm.alertCheckLoop()

	return nil
}

// Stop 停止分布式监控
func (dm *DistributedMonitor) Stop() {
	dm.cancel()
	dm.wg.Wait()
}

// monitorLoop 监控循环
func (dm *DistributedMonitor) monitorLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.aggregationRule.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.collectLocalMetrics()
			dm.aggregateMetrics()
		}
	}
}

// reportLoop 上报循环
func (dm *DistributedMonitor) reportLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.reportMetrics()
		}
	}
}

// alertCheckLoop 告警检查循环
func (dm *DistributedMonitor) alertCheckLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.checkStoragePoolAlerts()
		}
	}
}

// collectLocalMetrics 收集本地指标
func (dm *DistributedMonitor) collectLocalMetrics() {
	if dm.localManager == nil {
		return
	}

	metrics := &NodeMetrics{
		NodeID:    dm.localManager.GetHostname(),
		NodeName:  dm.localManager.GetHostname(),
		Timestamp: time.Now(),
		Status:    "healthy",
	}

	// 收集系统指标
	stats, err := dm.localManager.GetSystemStats()
	if err == nil {
		metrics.SystemMetrics = &SystemMetricData{
			CPUUsage:     stats.CPUUsage,
			MemoryUsage:  stats.MemoryUsage,
			MemoryTotal:  stats.MemoryTotal,
			MemoryUsed:   stats.MemoryUsed,
			SwapUsage:    stats.SwapUsage,
			UptimeSecs:   stats.UptimeSeconds,
			ProcessCount: stats.Processes,
		}
		if len(stats.LoadAvg) >= 3 {
			metrics.SystemMetrics.LoadAvg1 = stats.LoadAvg[0]
			metrics.SystemMetrics.LoadAvg5 = stats.LoadAvg[1]
			metrics.SystemMetrics.LoadAvg15 = stats.LoadAvg[2]
		}
	}

	// 收集磁盘指标
	diskStats, err := dm.localManager.GetDiskStats()
	if err == nil {
		metrics.DiskMetrics = make([]DiskMetricData, 0)
		for _, d := range diskStats {
			if d.FSType == "tmpfs" || d.FSType == "devtmpfs" {
				continue
			}
			metrics.DiskMetrics = append(metrics.DiskMetrics, DiskMetricData{
				Device:       d.Device,
				MountPoint:   d.MountPoint,
				TotalBytes:   d.Total,
				UsedBytes:    d.Used,
				FreeBytes:    d.Free,
				UsagePercent: d.UsagePercent,
				HealthStatus: "healthy",
			})
		}
	}

	// 收集网络指标
	netStats, err := dm.localManager.GetNetworkStats()
	if err == nil {
		metrics.NetworkMetrics = &NetworkMetricData{}
		for _, n := range netStats {
			metrics.NetworkMetrics.RXBytes += n.RXBytes
			metrics.NetworkMetrics.TXBytes += n.TXBytes
			metrics.NetworkMetrics.RXPackets += n.RXPackets
			metrics.NetworkMetrics.TXPackets += n.TXPackets
			metrics.NetworkMetrics.RXErrors += n.RXErrors
			metrics.NetworkMetrics.TXErrors += n.TXErrors
		}
	}

	// 获取健康评分
	if dm.localCollector != nil {
		if latest := dm.localCollector.GetLatestMetrics(); latest != nil {
			metrics.HealthScore = latest.HealthScore
		}
	}

	// 保存本地指标
	dm.mu.Lock()
	dm.nodes[metrics.NodeID] = metrics
	dm.mu.Unlock()
}

// RegisterNode 注册集群节点
func (dm *DistributedMonitor) RegisterNode(node ClusterNodeInfo) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查是否已存在
	for i, n := range dm.clusterNodes {
		if n.ID == node.ID {
			dm.clusterNodes[i] = node
			return
		}
	}

	dm.clusterNodes = append(dm.clusterNodes, node)
}

// UnregisterNode 注销集群节点
func (dm *DistributedMonitor) UnregisterNode(nodeID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 从节点列表移除
	for i, n := range dm.clusterNodes {
		if n.ID == nodeID {
			dm.clusterNodes = append(dm.clusterNodes[:i], dm.clusterNodes[i+1:]...)
			break
		}
	}

	// 移除节点指标
	delete(dm.nodes, nodeID)
}

// UpdateNodeMetrics 更新节点指标（用于接收远程节点上报）
func (dm *DistributedMonitor) UpdateNodeMetrics(metrics *NodeMetrics) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	metrics.Timestamp = time.Now()
	dm.nodes[metrics.NodeID] = metrics
}

// aggregateMetrics 聚合指标
func (dm *DistributedMonitor) aggregateMetrics() *AggregatedMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	agg := &AggregatedMetrics{
		Timestamp:    time.Now(),
		NodeCount:    len(dm.clusterNodes),
		ActiveNodes:  0,
		NodeMetrics:  make([]*NodeMetrics, 0),
		StoragePools: make([]AggregatedPoolMetric, 0),
		Alerts:       make([]ClusterAlert, 0),
	}

	// 计算活跃节点
	for _, node := range dm.clusterNodes {
		if node.IsActive {
			agg.ActiveNodes++
		}
	}

	// 聚合各节点指标
	var totalCPU, totalHealth float64
	var cpuCount, healthCount int
	poolMap := make(map[string]*AggregatedPoolMetric)

	for nodeID, metrics := range dm.nodes {
		agg.NodeMetrics = append(agg.NodeMetrics, metrics)

		// 聚合 CPU
		if metrics.SystemMetrics != nil {
			totalCPU += metrics.SystemMetrics.CPUUsage
			cpuCount++
			agg.TotalMemory += metrics.SystemMetrics.MemoryTotal
			agg.UsedMemory += metrics.SystemMetrics.MemoryUsed
		}

		// 聚合健康评分
		if metrics.HealthScore > 0 {
			totalHealth += metrics.HealthScore
			healthCount++
		}

		// 聚合磁盘
		for _, d := range metrics.DiskMetrics {
			agg.TotalDisk += d.TotalBytes
			agg.UsedDisk += d.UsedBytes
		}

		// 聚合网络
		if metrics.NetworkMetrics != nil {
			agg.NetworkTotal.RXBytes += metrics.NetworkMetrics.RXBytes
			agg.NetworkTotal.TXBytes += metrics.NetworkMetrics.TXBytes
			agg.NetworkTotal.RXPackets += metrics.NetworkMetrics.RXPackets
			agg.NetworkTotal.TXPackets += metrics.NetworkMetrics.TXPackets
		}

		// 聚合存储池
		for _, pool := range metrics.StorageMetrics {
			if existing, ok := poolMap[pool.PoolName]; ok {
				existing.TotalBytes += pool.TotalBytes
				existing.UsedBytes += pool.UsedBytes
				if pool.UsagePercent > existing.UsagePercent {
					existing.UsagePercent = pool.UsagePercent
				}
			} else {
				poolMap[pool.PoolName] = &AggregatedPoolMetric{
					PoolName:     pool.PoolName,
					TotalBytes:   pool.TotalBytes,
					UsedBytes:    pool.UsedBytes,
					UsagePercent: pool.UsagePercent,
					HealthStatus: pool.HealthStatus,
					AlertCount:   len(pool.Alerts),
				}
			}
		}

		// 检查节点状态
		if time.Since(metrics.Timestamp) > 2*time.Minute {
			agg.Alerts = append(agg.Alerts, ClusterAlert{
				ID:        fmt.Sprintf("node-down-%s", nodeID),
				Type:      "node_down",
				Level:     "critical",
				Message:   fmt.Sprintf("节点 %s 已离线超过 2 分钟", nodeID),
				NodeID:    nodeID,
				Timestamp: time.Now(),
			})
		}
	}

	// 计算平均值
	if cpuCount > 0 {
		agg.TotalCPU = totalCPU / float64(cpuCount)
	}
	if healthCount > 0 {
		agg.ClusterHealth = totalHealth / float64(healthCount)
	}

	// 转换存储池 map 为 slice
	for _, pool := range poolMap {
		agg.StoragePools = append(agg.StoragePools, *pool)
	}

	return agg
}

// GetAggregatedMetrics 获取聚合指标
func (dm *DistributedMonitor) GetAggregatedMetrics() *AggregatedMetrics {
	return dm.aggregateMetrics()
}

// GetNodeMetrics 获取节点指标
func (dm *DistributedMonitor) GetNodeMetrics(nodeID string) *NodeMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.nodes[nodeID]
}

// GetAllNodeMetrics 获取所有节点指标
func (dm *DistributedMonitor) GetAllNodeMetrics() []*NodeMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]*NodeMetrics, 0, len(dm.nodes))
	for _, m := range dm.nodes {
		result = append(result, m)
	}
	return result
}

// UpdateStoragePoolMetrics 更新存储池指标
func (dm *DistributedMonitor) UpdateStoragePoolMetrics(poolMetrics []StoragePoolMetric) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	nodeID := "local"
	if dm.localManager != nil {
		nodeID = dm.localManager.GetHostname()
	}

	if metrics, exists := dm.nodes[nodeID]; exists {
		metrics.StorageMetrics = poolMetrics
	} else {
		dm.nodes[nodeID] = &NodeMetrics{
			NodeID:         nodeID,
			NodeName:       nodeID,
			Timestamp:      time.Now(),
			StorageMetrics: poolMetrics,
			Status:         "healthy",
		}
	}
}

// checkStoragePoolAlerts 检查存储池告警
func (dm *DistributedMonitor) checkStoragePoolAlerts() {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for nodeID, metrics := range dm.nodes {
		for _, pool := range metrics.StorageMetrics {
			for _, rule := range dm.alertRules {
				if !rule.Enabled {
					continue
				}

				// 简单匹配（实际应支持通配符）
				if rule.PoolPattern != "*" && rule.PoolPattern != pool.PoolName {
					continue
				}

				var currentValue float64
				var shouldAlert bool

				switch rule.MetricType {
				case "usage":
					currentValue = pool.UsagePercent
					shouldAlert = currentValue >= rule.Threshold
				case "health":
					if pool.HealthStatus == "degraded" || pool.FailedDevices > 0 {
						currentValue = float64(pool.FailedDevices)
						shouldAlert = currentValue >= rule.Threshold
					}
				case "device_failure":
					currentValue = float64(pool.FailedDevices)
					shouldAlert = currentValue >= rule.Threshold
				case "io_latency":
					if pool.IOStats != nil {
						currentValue = pool.IOStats.AvgLatencyMs
						shouldAlert = currentValue >= rule.Threshold
					}
				}

				if shouldAlert {
					alert := &Alert{
						ID:        fmt.Sprintf("pool-%s-%s-%d", pool.PoolName, rule.MetricType, time.Now().Unix()),
						Type:      fmt.Sprintf("storage_pool_%s", rule.MetricType),
						Level:     rule.Level,
						Message:   formatAlertMessage(rule.MessageTemplate, pool.PoolName, currentValue),
						Source:    fmt.Sprintf("node:%s,pool:%s", nodeID, pool.PoolName),
						Timestamp: time.Now(),
					}

					if dm.alertManager != nil {
						dm.alertManager.triggerAlert(alert.Type, alert.Level, alert.Message, alert.Source, map[string]interface{}{
							"pool_name":     pool.PoolName,
							"node_id":       nodeID,
							"current_value": currentValue,
							"threshold":     rule.Threshold,
							"rule_name":     rule.Name,
						})
					}
				}
			}
		}
	}
}

// formatAlertMessage 格式化告警消息
func formatAlertMessage(template, poolName string, value float64) string {
	// 简单模板替换
	return fmt.Sprintf("存储池 %s 当前值 %.2f", poolName, value)
}

// AddAlertRule 添加告警规则
func (dm *DistributedMonitor) AddAlertRule(rule StoragePoolAlertRule) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.alertRules = append(dm.alertRules, rule)
}

// RemoveAlertRule 移除告警规则
func (dm *DistributedMonitor) RemoveAlertRule(name string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for i, rule := range dm.alertRules {
		if rule.Name == name {
			dm.alertRules = append(dm.alertRules[:i], dm.alertRules[i+1:]...)
			break
		}
	}
}

// GetAlertRules 获取告警规则
func (dm *DistributedMonitor) GetAlertRules() []StoragePoolAlertRule {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]StoragePoolAlertRule, len(dm.alertRules))
	copy(result, dm.alertRules)
	return result
}

// reportMetrics 上报指标
func (dm *DistributedMonitor) reportMetrics() {
	dm.mu.RLock()
	reporters := make(map[string]*MetricReporter)
	for k, v := range dm.reporters {
		reporters[k] = v
	}
	dm.mu.RUnlock()

	for _, reporter := range reporters {
		if !reporter.Enabled {
			continue
		}

		if err := dm.reportToEndpoint(reporter); err != nil {
			reporter.LastError = err
		} else {
			reporter.LastReport = time.Now()
			reporter.LastError = nil
		}
	}
}

// reportToEndpoint 上报到指定端点
func (dm *DistributedMonitor) reportToEndpoint(reporter *MetricReporter) error {
	metrics := dm.GetAggregatedMetrics()
	if metrics == nil {
		return nil
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("序列化指标失败: %w", err)
	}

	req, err := http.NewRequestWithContext(dm.ctx, "POST", reporter.Endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", reporter.NodeID)

	resp, err := dm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("上报失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// RegisterReporter 注册指标上报器
func (dm *DistributedMonitor) RegisterReporter(reporter *MetricReporter) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.reporters[reporter.NodeID] = reporter
}

// UnregisterReporter 注销指标上报器
func (dm *DistributedMonitor) UnregisterReporter(nodeID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.reporters, nodeID)
}

// GetClusterStats 获取集群统计
func (dm *DistributedMonitor) GetClusterStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_nodes":   len(dm.clusterNodes),
		"active_nodes":  0,
		"nodes_online":  0,
		"nodes_offline": 0,
		"metrics_count": len(dm.nodes),
		"alert_rules":   len(dm.alertRules),
		"reporters":     len(dm.reporters),
	}

	// 统计活跃节点
	for _, node := range dm.clusterNodes {
		if node.IsActive {
			stats["active_nodes"] = stats["active_nodes"].(int) + 1
		}
	}

	// 统计在线/离线节点（基于指标时间戳）
	for _, metrics := range dm.nodes {
		if time.Since(metrics.Timestamp) < 2*time.Minute {
			stats["nodes_online"] = stats["nodes_online"].(int) + 1
		} else {
			stats["nodes_offline"] = stats["nodes_offline"].(int) + 1
		}
	}

	return stats
}
