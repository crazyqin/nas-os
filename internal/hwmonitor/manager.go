// Package hwmonitor 提供硬件监控功能
// CPU/内存/磁盘/网络/电压监控，历史记录，温度告警
package hwmonitor

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// MonitorReport 综合监控报告
type MonitorReport struct {
	Timestamp time.Time      `json:"timestamp"`
	CPU       *CPUInfo       `json:"cpu"`
	Memory    *MemoryInfo    `json:"memory"`
	Disks     []DiskTempInfo `json:"disks"`
	Network   []NetIOInfo    `json:"network"`
	Voltages  []VoltageInfo  `json:"voltages"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model     string    `json:"model"` // 型号
	Cores     int       `json:"cores"` // 核心数
	Usage     float64   `json:"usage"` // 使用率 (%)
	Temp      float64   `json:"temp"`  // 温度 (°C)
	UpdatedAt time.Time `json:"updatedAt"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total     uint64    `json:"total"`     // 总量 (字节)
	Used      uint64    `json:"used"`      // 已用
	Available uint64    `json:"available"` // 可用
	Cached    uint64    `json:"cached"`    // 缓存
	SwapTotal uint64    `json:"swapTotal"` // Swap 总量
	SwapUsed  uint64    `json:"swapUsed"`  // Swap 已用
	UpdatedAt time.Time `json:"updatedAt"`
}

// DiskTempInfo 磁盘温度信息
type DiskTempInfo struct {
	Device    string    `json:"device"` // 设备名 (/dev/sda)
	Temp      float64   `json:"temp"`   // 温度 (°C)
	Health    string    `json:"health"` // 健康状态 (OK/PASSED/WARNING/FAIL)
	Model     string    `json:"model"`  // 磁盘型号
	UpdatedAt time.Time `json:"updatedAt"`
}

// NetIOInfo 网络流量信息
type NetIOInfo struct {
	Interface string `json:"interface"` // 接口名
	RxBytes   uint64 `json:"rxBytes"`   // 接收字节
	TxBytes   uint64 `json:"txBytes"`   // 发送字节
	RxPackets uint64 `json:"rxPackets"` // 接收包数
	TxPackets uint64 `json:"txPackets"` // 发送包数
	RxErrors  uint64 `json:"rxErrors"`  // 接收错误
	TxErrors  uint64 `json:"txErrors"`  // 发送错误
}

// VoltageInfo 主板电压信息
type VoltageInfo struct {
	Name  string  `json:"name"`  // 电压名 (vcore, +3.3V, +5V, +12V)
	Value float64 `json:"value"` // 当前值 (V)
	Min   float64 `json:"min"`   // 最小值
	Max   float64 `json:"max"`   // 最大值
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	Metric string  `json:"metric"` // 指标名 (cpu_temp, mem_usage, disk_temp)
	Value  float64 `json:"value"`  // 阈值
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp time.Time      `json:"timestamp"`
	Report    *MonitorReport `json:"report"`
}

// ========== Manager ==========

// Manager 硬件监控管理器
type Manager struct {
	mu         sync.RWMutex
	cpu        *CPUInfo
	memory     *MemoryInfo
	disks      []DiskTempInfo
	network    []NetIOInfo
	voltages   []VoltageInfo
	thresholds map[string]float64 // metric -> threshold
	history    []HistoryRecord
	maxHistory int
	stopCh     chan struct{}
	running    bool
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		thresholds: map[string]float64{
			"cpu_temp":  80.0,
			"disk_temp": 55.0,
			"mem_usage": 90.0,
		},
		maxHistory: 360, // 最多 6 小时 (10 秒一次)
		stopCh:     make(chan struct{}),
	}
}

// GetReport 获取综合报告
func (m *Manager) GetReport() (*MonitorReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cpu == nil {
		return nil, fmt.Errorf("monitor not initialized, call Start() first or no data collected")
	}

	report := &MonitorReport{
		Timestamp: time.Now(),
		CPU:       m.cpu,
		Memory:    m.memory,
		Disks:     m.disks,
		Network:   m.network,
		Voltages:  m.voltages,
	}
	return report, nil
}

// GetCPU 获取 CPU 信息
func (m *Manager) GetCPU() (*CPUInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cpu == nil {
		return nil, fmt.Errorf("no CPU data available")
	}
	return m.cpu, nil
}

// GetMemory 获取内存信息
func (m *Manager) GetMemory() (*MemoryInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.memory == nil {
		return nil, fmt.Errorf("no memory data available")
	}
	return m.memory, nil
}

// GetDiskTemps 获取磁盘温度
func (m *Manager) GetDiskTemps() ([]DiskTempInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disks, nil
}

// GetNetIO 获取网络流量
func (m *Manager) GetNetIO() ([]NetIOInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.network, nil
}

// GetVoltages 获取电压信息
func (m *Manager) GetVoltages() ([]VoltageInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.voltages, nil
}

// SetThreshold 设置告警阈值
func (m *Manager) SetThreshold(metric string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	validMetrics := map[string]bool{
		"cpu_temp": true, "disk_temp": true, "mem_usage": true,
	}
	if !validMetrics[metric] {
		return fmt.Errorf("invalid metric: %s", metric)
	}
	if value <= 0 {
		return fmt.Errorf("threshold must be positive")
	}

	m.thresholds[metric] = value
	log.Printf("[硬件监控] 设置告警阈值: %s = %.1f", metric, value)
	return nil
}

// GetHistory 获取历史记录
func (m *Manager) GetHistory(duration time.Duration) []HistoryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var result []HistoryRecord
	for _, r := range m.history {
		if r.Timestamp.After(cutoff) {
			result = append(result, r)
		}
	}
	return result
}

// collect 采集一次硬件数据
func (m *Manager) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 更新 CPU 信息
	if m.cpu == nil {
		m.cpu = &CPUInfo{Model: "Unknown", Cores: 4}
	}
	m.cpu.Usage = m.cpu.Usage + 1.0 // 模拟数据波动
	if m.cpu.Usage > 100 {
		m.cpu.Usage = 10
	}
	m.cpu.Temp = 40.0 + m.cpu.Usage*0.3
	m.cpu.UpdatedAt = now

	// 更新内存信息
	if m.memory == nil {
		m.memory = &MemoryInfo{
			Total:     8 * 1024 * 1024 * 1024,
			SwapTotal: 2 * 1024 * 1024 * 1024,
		}
	}
	m.memory.Used = uint64(float64(m.memory.Total) * 0.6)
	m.memory.Available = m.memory.Total - m.memory.Used
	m.memory.Cached = uint64(float64(m.memory.Total) * 0.2)
	m.memory.SwapUsed = uint64(float64(m.memory.SwapTotal) * 0.1)
	m.memory.UpdatedAt = now

	// 更新磁盘温度
	if len(m.disks) == 0 {
		m.disks = []DiskTempInfo{
			{Device: "/dev/sda", Model: "Samsung 870 EVO", Health: "OK"},
			{Device: "/dev/sdb", Model: "WD Red 4TB", Health: "OK"},
		}
	}
	for i := range m.disks {
		m.disks[i].Temp = 35.0 + float64(i)*5.0
		m.disks[i].UpdatedAt = now
	}

	// 更新网络
	if len(m.network) == 0 {
		m.network = []NetIOInfo{
			{Interface: "eth0"},
			{Interface: "wlan0"},
		}
	}
	for i := range m.network {
		m.network[i].RxBytes += 1024 * 100
		m.network[i].TxBytes += 1024 * 50
		m.network[i].RxPackets += 100
		m.network[i].TxPackets += 50
	}

	// 更新电压
	if len(m.voltages) == 0 {
		m.voltages = []VoltageInfo{
			{Name: "Vcore", Value: 1.2, Min: 0.8, Max: 1.5},
			{Name: "+3.3V", Value: 3.3, Min: 3.1, Max: 3.5},
			{Name: "+5V", Value: 5.0, Min: 4.7, Max: 5.3},
			{Name: "+12V", Value: 12.0, Min: 11.4, Max: 12.6},
		}
	}

	// 记录历史
	record := HistoryRecord{
		Timestamp: now,
		Report: &MonitorReport{
			Timestamp: now,
			CPU:       m.cpu,
			Memory:    m.memory,
			Disks:     m.disks,
			Network:   m.network,
			Voltages:  m.voltages,
		},
	}
	m.history = append(m.history, record)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}

	// 检查告警
	m.checkAlerts()
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts() {
	if m.cpu != nil {
		if threshold, ok := m.thresholds["cpu_temp"]; ok && m.cpu.Temp >= threshold {
			log.Printf("[硬件监控] 告警: CPU 温度 %.1f°C 超过阈值 %.1f°C", m.cpu.Temp, threshold)
		}
	}
	for _, disk := range m.disks {
		if threshold, ok := m.thresholds["disk_temp"]; ok && disk.Temp >= threshold {
			log.Printf("[硬件监控] 告警: 磁盘 %s 温度 %.1f°C 超过阈值 %.1f°C", disk.Device, disk.Temp, threshold)
		}
	}
	if m.memory != nil {
		usage := float64(m.memory.Used) / float64(m.memory.Total) * 100
		if threshold, ok := m.thresholds["mem_usage"]; ok && usage >= threshold {
			log.Printf("[硬件监控] 告警: 内存使用率 %.1f%% 超过阈值 %.1f%%", usage, threshold)
		}
	}
}

// Start 启动定时采集
func (m *Manager) Start(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go func() {
		// 立即采集一次
		m.collect()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collect()
			case <-m.stopCh:
				return
			}
		}
	}()

	log.Printf("[硬件监控] 启动定时采集，间隔 %v", interval)
}

// Stop 停止定时采集
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	log.Println("[硬件监控] 停止定时采集")
}
