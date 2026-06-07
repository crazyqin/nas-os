// Package diskheat 提供磁盘热力图功能
// 读写热度可视化、IO 分布分析、性能瓶颈定位
package diskheat

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	Device       string    `json:"device"`       // 设备名 (sda, nvme0n1 etc.)
	Model        string    `json:"model"`        // 型号
	MountPoint   string    `json:"mountPoint"`   // 挂载点
	TotalBytes   int64     `json:"totalBytes"`   // 总容量
	UsedBytes    int64     `json:"usedBytes"`    // 已用
	FreeBytes    int64     `json:"freeBytes"`    // 可用
	ReadOPS      int64     `json:"readOps"`      // 读操作数
	WriteOPS     int64     `json:"writeOps"`     // 写操作数
	ReadBytes    int64     `json:"readBytes"`    // 读字节数
	WriteBytes   int64     `json:"writeBytes"`   // 写字节数
	ReadLatency  float64   `json:"readLatency"`  // 读延迟 (ms)
	WriteLatency float64   `json:"writeLatency"` // 写延迟 (ms)
	Temperature  float64   `json:"temperature"`  // 温度
	Health       string    `json:"health"`       // 健康状态
	HeatScore    float64   `json:"heatScore"`    // 热度评分 0-100
	Timestamp    time.Time `json:"timestamp"`
}

// HeatLevel 热度等级
type HeatLevel string

const (
	HeatLevelCold HeatLevel = "cold" // 冷 (0-20)
	HeatLevelCool HeatLevel = "cool" // 凉 (20-40)
	HeatLevelWarm HeatLevel = "warm" // 温 (40-60)
	HeatLevelHot  HeatLevel = "hot"  // 热 (60-80)
	HeatLevelFire HeatLevel = "fire" // 极热 (80-100)
)

// HeatPoint 热力图数据点
type HeatPoint struct {
	DiskID    string    `json:"diskId"`
	Device    string    `json:"device"`
	ReadRate  float64   `json:"readRate"`  // MB/s
	WriteRate float64   `json:"writeRate"` // MB/s
	IOPS      int64     `json:"iops"`
	Latency   float64   `json:"latency"` // ms
	Heat      float64   `json:"heat"`    // 0-100
	Level     HeatLevel `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// IOBottleneck IO 瓶颈
type IOBottleneck struct {
	DiskID         string    `json:"diskId"`
	Device         string    `json:"device"`
	Type           string    `json:"type"`     // high_iops, high_latency, low_throughput
	Severity       string    `json:"severity"` // warning, critical
	Message        string    `json:"message"`
	Value          float64   `json:"value"`
	Threshold      float64   `json:"threshold"`
	Timestamp      time.Time `json:"timestamp"`
	Recommendation string    `json:"recommendation"`
}

// DiskHealthReport 磁盘健康报告
type DiskHealthReport struct {
	DiskID          string    `json:"diskId"`
	Device          string    `json:"device"`
	Model           string    `json:"model"`
	HealthScore     float64   `json:"healthScore"` // 0-100
	Temperature     float64   `json:"temperature"`
	PowerOnHours    int64     `json:"powerOnHours"`
	ReallocSects    int64     `json:"reallocSects"`
	PendingSects    int64     `json:"pendingSects"`
	PredictFail     bool      `json:"predictFail"`
	Recommendations []string  `json:"recommendations,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

// HeatmapData 热力图数据
type HeatmapData struct {
	Disks       []HeatPoint     `json:"disks"`
	Timeline    []TimelinePoint `json:"timeline"`
	Bottlenecks []IOBottleneck  `json:"bottlenecks"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

// TimelinePoint 时间线数据点
type TimelinePoint struct {
	Timestamp time.Time `json:"timestamp"`
	AvgHeat   float64   `json:"avgHeat"`
	MaxHeat   float64   `json:"maxHeat"`
	TotalIOPS int64     `json:"totalIops"`
}

// IOStat IO 统计
type IOStat struct {
	Device       string  `json:"device"`
	ReadMBps     float64 `json:"readMBps"`
	WriteMBps    float64 `json:"writeMBps"`
	ReadIOPS     int64   `json:"readIops"`
	WriteIOPS    int64   `json:"writeIops"`
	AvgQueueSize float64 `json:"avgQueueSize"`
	Utilization  float64 `json:"utilization"` // 0-100%
}

// ========== Manager ==========

// Manager 磁盘热力图管理器
type Manager struct {
	mu          sync.RWMutex
	disks       map[string]*DiskMetrics
	history     []HeatPoint
	bottlenecks []IOBottleneck
	historyMax  int
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		disks:      make(map[string]*DiskMetrics),
		historyMax: 1000,
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认数据
func (m *Manager) initDefaults() {
	m.disks["sda"] = &DiskMetrics{
		Device: "sda", Model: "WD Red Plus 4TB", MountPoint: "/data",
		TotalBytes: 4000000000000, UsedBytes: 2800000000000, FreeBytes: 1200000000000,
		ReadOPS: 1500, WriteOPS: 800, ReadBytes: 50000000, WriteBytes: 30000000,
		ReadLatency: 0.5, WriteLatency: 0.8, Temperature: 38, Health: "good",
		HeatScore: 45, Timestamp: time.Now(),
	}
	m.disks["sdb"] = &DiskMetrics{
		Device: "sdb", Model: "Seagate IronWolf 4TB", MountPoint: "/data2",
		TotalBytes: 4000000000000, UsedBytes: 3200000000000, FreeBytes: 800000000000,
		ReadOPS: 2000, WriteOPS: 1200, ReadBytes: 80000000, WriteBytes: 50000000,
		ReadLatency: 0.6, WriteLatency: 1.0, Temperature: 42, Health: "good",
		HeatScore: 62, Timestamp: time.Now(),
	}
	m.disks["nvme0n1"] = &DiskMetrics{
		Device: "nvme0n1", Model: "Samsung 980 PRO 1TB", MountPoint: "/",
		TotalBytes: 1000000000000, UsedBytes: 600000000000, FreeBytes: 400000000000,
		ReadOPS: 50000, WriteOPS: 30000, ReadBytes: 500000000, WriteBytes: 300000000,
		ReadLatency: 0.02, WriteLatency: 0.03, Temperature: 45, Health: "excellent",
		HeatScore: 75, Timestamp: time.Now(),
	}
}

// ========== 数据采集 ==========

// UpdateMetrics 更新磁盘指标
func (m *Manager) UpdateMetrics(device string, metrics DiskMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics.Timestamp = time.Now()
	metrics.HeatScore = m.calculateHeatScore(&metrics)
	m.disks[device] = &metrics

	// 记录历史
	point := HeatPoint{
		DiskID:    device,
		Device:    device,
		ReadRate:  float64(metrics.ReadBytes) / 1024 / 1024,
		WriteRate: float64(metrics.WriteBytes) / 1024 / 1024,
		IOPS:      metrics.ReadOPS + metrics.WriteOPS,
		Latency:   (metrics.ReadLatency + metrics.WriteLatency) / 2,
		Heat:      metrics.HeatScore,
		Level:     m.getHeatLevel(metrics.HeatScore),
		Timestamp: time.Now(),
	}
	m.history = append(m.history, point)

	// 限制历史大小
	if len(m.history) > m.historyMax {
		m.history = m.history[len(m.history)-m.historyMax:]
	}

	// 检测瓶颈
	m.detectBottlenecks(device, &metrics)

	return nil
}

// calculateHeatScore 计算热度评分
func (m *Manager) calculateHeatScore(metrics *DiskMetrics) float64 {
	// 基于多个因素计算热度
	iopsScore := math.Min(float64(metrics.ReadOPS+metrics.WriteOPS)/1000*10, 40)
	latencyScore := math.Min((metrics.ReadLatency+metrics.WriteLatency)/2*10, 30)
	tempScore := math.Min(metrics.Temperature/80*20, 20)
	usageScore := float64(metrics.UsedBytes) / float64(metrics.TotalBytes) * 10

	return math.Min(iopsScore+latencyScore+tempScore+usageScore, 100)
}

// getHeatLevel 获取热度等级
func (m *Manager) getHeatLevel(heat float64) HeatLevel {
	switch {
	case heat >= 80:
		return HeatLevelFire
	case heat >= 60:
		return HeatLevelHot
	case heat >= 40:
		return HeatLevelWarm
	case heat >= 20:
		return HeatLevelCool
	default:
		return HeatLevelCold
	}
}

// detectBottlenecks 检测瓶颈
func (m *Manager) detectBottlenecks(device string, metrics *DiskMetrics) {
	// 高延迟
	if metrics.ReadLatency > 10 || metrics.WriteLatency > 10 {
		m.bottlenecks = append(m.bottlenecks, IOBottleneck{
			DiskID: device, Device: device, Type: "high_latency",
			Severity:  "warning",
			Message:   fmt.Sprintf("%s 延迟过高: 读 %.1fms, 写 %.1fms", device, metrics.ReadLatency, metrics.WriteLatency),
			Value:     (metrics.ReadLatency + metrics.WriteLatency) / 2,
			Threshold: 10, Timestamp: time.Now(),
			Recommendation: "检查磁盘健康状态，考虑升级到 SSD",
		})
	}

	// 高 IOPS
	totalIOPS := metrics.ReadOPS + metrics.WriteOPS
	if totalIOPS > 100000 {
		m.bottlenecks = append(m.bottlenecks, IOBottleneck{
			DiskID: device, Device: device, Type: "high_iops",
			Severity:  "warning",
			Message:   fmt.Sprintf("%s IOPS 过高: %d", device, totalIOPS),
			Value:     float64(totalIOPS),
			Threshold: 100000, Timestamp: time.Now(),
			Recommendation: "考虑启用缓存或升级到 NVMe SSD",
		})
	}

	// 容量使用率高
	usagePercent := float64(metrics.UsedBytes) / float64(metrics.TotalBytes) * 100
	if usagePercent > 90 {
		m.bottlenecks = append(m.bottlenecks, IOBottleneck{
			DiskID: device, Device: device, Type: "high_usage",
			Severity:  "critical",
			Message:   fmt.Sprintf("%s 容量使用率 %.1f%%", device, usagePercent),
			Value:     usagePercent,
			Threshold: 90, Timestamp: time.Now(),
			Recommendation: "清理无用文件或扩容",
		})
	}

	// 温度过高
	if metrics.Temperature > 60 {
		m.bottlenecks = append(m.bottlenecks, IOBottleneck{
			DiskID: device, Device: device, Type: "high_temp",
			Severity:  "warning",
			Message:   fmt.Sprintf("%s 温度过高: %.1f°C", device, metrics.Temperature),
			Value:     metrics.Temperature,
			Threshold: 60, Timestamp: time.Now(),
			Recommendation: "检查散热，增加风扇转速",
		})
	}
}

// ========== 查询 ==========

// GetDiskMetrics 获取磁盘指标
func (m *Manager) GetDiskMetrics(device string) (*DiskMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[device]
	if !ok {
		return nil, fmt.Errorf("disk %s not found", device)
	}
	return disk, nil
}

// ListDisks 列出所有磁盘
func (m *Manager) ListDisks() []*DiskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskMetrics, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].HeatScore > disks[j].HeatScore
	})

	return disks
}

// GenerateHeatmap 生成热力图数据
func (m *Manager) GenerateHeatmap() HeatmapData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var points []HeatPoint
	for _, disk := range m.disks {
		points = append(points, HeatPoint{
			DiskID:    disk.Device,
			Device:    disk.Device,
			ReadRate:  float64(disk.ReadBytes) / 1024 / 1024,
			WriteRate: float64(disk.WriteBytes) / 1024 / 1024,
			IOPS:      disk.ReadOPS + disk.WriteOPS,
			Latency:   (disk.ReadLatency + disk.WriteLatency) / 2,
			Heat:      disk.HeatScore,
			Level:     m.getHeatLevel(disk.HeatScore),
			Timestamp: disk.Timestamp,
		})
	}

	// 生成时间线
	timeline := m.generateTimeline()

	return HeatmapData{
		Disks:       points,
		Timeline:    timeline,
		Bottlenecks: m.bottlenecks,
		GeneratedAt: time.Now(),
	}
}

// generateTimeline 生成时间线
func (m *Manager) generateTimeline() []TimelinePoint {
	if len(m.history) == 0 {
		return nil
	}

	// 按分钟聚合
	buckets := make(map[int64]*TimelinePoint)
	for _, h := range m.history {
		minKey := h.Timestamp.Unix() / 60
		bucket, ok := buckets[minKey]
		if !ok {
			bucket = &TimelinePoint{Timestamp: h.Timestamp.Truncate(time.Minute)}
			buckets[minKey] = bucket
		}
		bucket.AvgHeat += h.Heat
		if h.Heat > bucket.MaxHeat {
			bucket.MaxHeat = h.Heat
		}
		bucket.TotalIOPS += h.IOPS
	}

	timeline := make([]TimelinePoint, 0, len(buckets))
	for _, bucket := range buckets {
		timeline = append(timeline, *bucket)
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	return timeline
}

// GetBottlenecks 获取瓶颈列表
func (m *Manager) GetBottlenecks() []IOBottleneck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回最近的瓶颈
	recent := make([]IOBottleneck, 0)
	cutoff := time.Now().Add(-1 * time.Hour)
	for _, b := range m.bottlenecks {
		if b.Timestamp.After(cutoff) {
			recent = append(recent, b)
		}
	}
	return recent
}

// GetIOStats 获取 IO 统计
func (m *Manager) GetIOStats() []IOStat {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats []IOStat
	for _, disk := range m.disks {
		stats = append(stats, IOStat{
			Device:      disk.Device,
			ReadMBps:    float64(disk.ReadBytes) / 1024 / 1024,
			WriteMBps:   float64(disk.WriteBytes) / 1024 / 1024,
			ReadIOPS:    disk.ReadOPS,
			WriteIOPS:   disk.WriteOPS,
			Utilization: float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Utilization > stats[j].Utilization
	})

	return stats
}

// ========== 健康检查 ==========

// GetDiskHealth 获取磁盘健康报告
func (m *Manager) GetDiskHealth(device string) (*DiskHealthReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, ok := m.disks[device]
	if !ok {
		return nil, fmt.Errorf("disk %s not found", device)
	}

	report := &DiskHealthReport{
		DiskID:      device,
		Device:      device,
		Model:       disk.Model,
		HealthScore: 100 - disk.HeatScore/2,
		Temperature: disk.Temperature,
		Timestamp:   time.Now(),
	}

	// 温度建议
	if disk.Temperature > 50 {
		report.Recommendations = append(report.Recommendations, "温度偏高，建议改善散热")
	}

	// 容量建议
	usagePercent := float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100
	if usagePercent > 80 {
		report.Recommendations = append(report.Recommendations, "容量使用率超过 80%，建议清理或扩容")
	}

	// 延迟建议
	if disk.ReadLatency > 5 || disk.WriteLatency > 5 {
		report.Recommendations = append(report.Recommendations, "IO 延迟较高，建议检查磁盘健康或升级存储")
	}

	return report, nil
}

// ========== 统计 ==========

// GetOverallStats 获取总体统计
func (m *Manager) GetOverallStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCapacity := int64(0)
	totalUsed := int64(0)
	totalIOPS := int64(0)
	avgHeat := 0.0
	maxHeat := 0.0
	bottleneckCount := 0

	for _, disk := range m.disks {
		totalCapacity += disk.TotalBytes
		totalUsed += disk.UsedBytes
		totalIOPS += disk.ReadOPS + disk.WriteOPS
		avgHeat += disk.HeatScore
		if disk.HeatScore > maxHeat {
			maxHeat = disk.HeatScore
		}
	}

	if len(m.disks) > 0 {
		avgHeat /= float64(len(m.disks))
	}

	recentCutoff := time.Now().Add(-1 * time.Hour)
	for _, b := range m.bottlenecks {
		if b.Timestamp.After(recentCutoff) {
			bottleneckCount++
		}
	}

	return map[string]interface{}{
		"totalDisks":      len(m.disks),
		"totalCapacity":   totalCapacity,
		"totalUsed":       totalUsed,
		"usagePercent":    float64(totalUsed) / float64(totalCapacity) * 100,
		"totalIOPS":       totalIOPS,
		"avgHeat":         avgHeat,
		"maxHeat":         maxHeat,
		"bottleneckCount": bottleneckCount,
	}
}
