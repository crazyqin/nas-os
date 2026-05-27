package analytics

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Collector 系统指标采集器
type Collector struct {
	mu            sync.RWMutex
	config        CollectorConfig
	history       []SystemMetrics
	lastCollect   time.Time
	stopChan      chan struct{}
	running       bool
	subscribers   []chan SystemMetrics
}

// NewCollector 创建采集器
func NewCollector(cfg CollectorConfig) *Collector {
	return &Collector{
		config:  cfg,
		history: make([]SystemMetrics, 0, cfg.HistorySize),
		stopChan: make(chan struct{}),
		subscribers: make([]chan SystemMetrics, 0),
	}
}

// Start 启动采集器
func (c *Collector) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go c.collectLoop()
}

// Stop 停止采集器
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		close(c.stopChan)
		c.running = false
	}
}

// collectLoop 采集循环
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()

	// 立即采集一次
	c.collect()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

// collect 执行一次采集
func (c *Collector) collect() {
	metrics, err := c.CollectNow()
	if err != nil {
		return
	}

	c.mu.Lock()
	// 维护历史大小
	if len(c.history) >= c.config.HistorySize {
		c.history = c.history[1:]
	}
	c.history = append(c.history, *metrics)
	c.lastCollect = metrics.Timestamp
	c.mu.Unlock()

	// 通知订阅者
	c.notifySubscribers(*metrics)
}

// CollectNow 立即采集当前指标
func (c *Collector) CollectNow() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var cpuErr, memErr, diskErr, netErr error

	if c.config.EnableCPU {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.CPU, cpuErr = c.collectCPU()
		}()
	}

	if c.config.EnableMemory {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.Memory, memErr = c.collectMemory()
		}()
	}

	if c.config.EnableDisk {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.Disk, diskErr = c.collectDisk()
		}()
	}

	if c.config.EnableNetwork {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.Network, netErr = c.collectNetwork()
		}()
	}

	wg.Wait()

	// 记录错误但不阻止返回
	if cpuErr != nil {
		fmt.Printf("采集CPU指标失败: %v\n", cpuErr)
	}
	if memErr != nil {
		fmt.Printf("采集内存指标失败: %v\n", memErr)
	}
	if diskErr != nil {
		fmt.Printf("采集磁盘指标失败: %v\n", diskErr)
	}
	if netErr != nil {
		fmt.Printf("采集网络指标失败: %v\n", netErr)
	}

	return metrics, nil
}

// collectCPU 采集CPU指标
func (c *Collector) collectCPU() (CPUMetrics, error) {
	metrics := CPUMetrics{}

	// 总体使用率
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return metrics, fmt.Errorf("获取CPU使用率失败: %w", err)
	}
	if len(percent) > 0 {
		metrics.UsagePercent = percent[0]
	}

	// 每核使用率
	perCore, err := cpu.Percent(time.Second, true)
	if err == nil {
		metrics.PerCore = perCore
	}

	// 负载
	avg, err := load.Avg()
	if err == nil {
		metrics.LoadAvg1 = avg.Load1
		metrics.LoadAvg5 = avg.Load5
		metrics.LoadAvg15 = avg.Load15
	}

	// 进程数
	metrics.ProcessCount = runtime.NumGoroutine()

	return metrics, nil
}

// collectMemory 采集内存指标
func (c *Collector) collectMemory() (MemoryMetrics, error) {
	metrics := MemoryMetrics{}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return metrics, fmt.Errorf("获取内存信息失败: %w", err)
	}

	metrics.TotalBytes = vm.Total
	metrics.UsedBytes = vm.Used
	metrics.FreeBytes = vm.Free
	metrics.AvailableBytes = vm.Available
	metrics.UsagePercent = vm.UsedPercent
	metrics.CachedBytes = vm.Cached
	metrics.BuffersBytes = vm.Buffers

	// Swap
	swap, err := mem.SwapMemory()
	if err == nil {
		metrics.SwapTotalBytes = swap.Total
		metrics.SwapUsedBytes = swap.Used
		metrics.SwapUsagePct = swap.UsedPercent
	}

	return metrics, nil
}

// collectDisk 采集磁盘指标
func (c *Collector) collectDisk() (DiskMetrics, error) {
	metrics := DiskMetrics{}

	partitions, err := disk.Partitions(false)
	if err != nil {
		return metrics, fmt.Errorf("获取磁盘分区失败: %w", err)
	}

	var totalUsed, totalFree, totalAll uint64

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		device := DiskDeviceMetrics{
			Device:       p.Device,
			MountPoint:   p.Mountpoint,
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			FreeBytes:    usage.Free,
			UsagePercent: usage.UsedPercent,
			FSType:       p.Fstype,
		}

		metrics.Devices = append(metrics.Devices, device)
		totalUsed += usage.Used
		totalFree += usage.Free
		totalAll += usage.Total
	}

	if totalAll > 0 {
		metrics.Total = DiskSummaryMetrics{
			TotalBytes:   totalAll,
			UsedBytes:    totalUsed,
			FreeBytes:    totalFree,
			UsagePercent: float64(totalUsed) / float64(totalAll) * 100,
		}
	}

	return metrics, nil
}

// collectNetwork 采集网络指标
func (c *Collector) collectNetwork() (NetworkMetrics, error) {
	metrics := NetworkMetrics{}

	// 第一次采样
	counters1, err := net.IOCounters(true)
	if err != nil {
		return metrics, fmt.Errorf("获取网络统计失败: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 第二次采样
	counters2, err := net.IOCounters(true)
	if err != nil {
		return metrics, fmt.Errorf("获取网络统计失败: %w", err)
	}

	duration := 0.1 // 100ms

	for i, c1 := range counters1 {
		if c1.Name == "lo" {
			continue
		}

		var c2 net.IOCountersStat
		for _, c := range counters2 {
			if c.Name == c1.Name {
				c2 = c
				break
			}
		}

		iface := NetworkInterfaceMetrics{
			Name:        c1.Name,
			RXBytesPS:   uint64(float64(c2.BytesRecv-c1.BytesRecv) / duration),
			TXBytesPS:   uint64(float64(c2.BytesSent-c1.BytesSent) / duration),
			RXPacketsPS: uint64(float64(c2.PacketsRecv-c1.PacketsRecv) / duration),
			TXPacketsPS: uint64(float64(c2.PacketsSent-c1.PacketsSent) / duration),
			RXErrors:    c2.Errin,
			TXErrors:    c2.Errout,
		}

		metrics.Interfaces = append(metrics.Interfaces, iface)
		metrics.Total.TotalRXBytesPS += iface.RXBytesPS
		metrics.Total.TotalTXBytesPS += iface.TXBytesPS
		metrics.Total.TotalRXPackets += c2.PacketsRecv
		metrics.Total.TotalTXPackets += c2.PacketsSent

		_ = i
	}

	return metrics, nil
}

// GetHistory 获取历史指标
func (c *Collector) GetHistory(limit int) []SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.history) {
		limit = len(c.history)
	}

	start := len(c.history) - limit
	result := make([]SystemMetrics, limit)
	copy(result, c.history[start:])
	return result
}

// GetLatest 获取最新指标
func (c *Collector) GetLatest() *SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.history) == 0 {
		return nil
	}

	latest := c.history[len(c.history)-1]
	return &latest
}

// GetHistoryByTimeRange 按时间范围获取历史
func (c *Collector) GetHistoryByTimeRange(start, end time.Time) []SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]SystemMetrics, 0)
	for _, m := range c.history {
		if m.Timestamp.After(start) && m.Timestamp.Before(end) {
			result = append(result, m)
		}
	}
	return result
}

// Subscribe 订阅指标更新
func (c *Collector) Subscribe() chan SystemMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan SystemMetrics, 100)
	c.subscribers = append(c.subscribers, ch)
	return ch
}

// Unsubscribe 取消订阅
func (c *Collector) Unsubscribe(ch chan SystemMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// notifySubscribers 通知订阅者
func (c *Collector) notifySubscribers(metrics SystemMetrics) {
	c.mu.RLock()
	subscribers := make([]chan SystemMetrics, len(c.subscribers))
	copy(subscribers, c.subscribers)
	c.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- metrics:
		default:
			// channel 满，跳过
		}
	}
}

// CalculateCPUAverage 计算CPU平均使用率
func CalculateCPUAverage(metrics []SystemMetrics) float64 {
	if len(metrics) == 0 {
		return 0
	}

	var total float64
	for _, m := range metrics {
		total += m.CPU.UsagePercent
	}
	return total / float64(len(metrics))
}

// CalculateMemoryAverage 计算内存平均使用率
func CalculateMemoryAverage(metrics []SystemMetrics) float64 {
	if len(metrics) == 0 {
		return 0
	}

	var total float64
	for _, m := range metrics {
		total += m.Memory.UsagePercent
	}
	return total / float64(len(metrics))
}

// FindPeakCPUUsage 查找CPU峰值
func FindPeakCPUUsage(metrics []SystemMetrics) (time.Time, float64) {
	if len(metrics) == 0 {
		return time.Time{}, 0
	}

	peakTime := metrics[0].Timestamp
	peakUsage := metrics[0].CPU.UsagePercent

	for _, m := range metrics[1:] {
		if m.CPU.UsagePercent > peakUsage {
			peakUsage = m.CPU.UsagePercent
			peakTime = m.Timestamp
		}
	}

	return peakTime, peakUsage
}

// CalculateStandardDeviation 计算标准差
func CalculateStandardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}
