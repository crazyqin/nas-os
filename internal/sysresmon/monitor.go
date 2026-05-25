package sysresmon

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// ResourceMonitor 资源监控器
type ResourceMonitor struct {
	mu            sync.RWMutex
	config        *Config
	history       *RingBuffer
	cancel        context.CancelFunc
	stopped       chan struct{}
}

// Config 监控配置
type Config struct {
	// 采集间隔（秒），默认 30
	Interval int `json:"interval"`
	// 环形缓冲区大小，默认 2880（24h * 60min / 0.5min）
	BufferSize int `json:"bufferSize"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Interval:   30,
		BufferSize: 2880,
	}
}

// CPUInfo CPU 信息
type CPUInfo struct {
	// 总体使用率（百分比）
	UsagePercent float64 `json:"usagePercent"`
	// 每核心使用率
	PerCore []float64 `json:"perCore"`
	// CPU 温度（摄氏度，如不可用则为 0）
	Temperature float64 `json:"temperature"`
	// CPU 频率（MHz）
	Frequency float64 `json:"frequency"`
	// 负载平均值（1/5/15 分钟）
	LoadAvg1  float64 `json:"loadAvg1"`
	LoadAvg5  float64 `json:"loadAvg5"`
	LoadAvg15 float64 `json:"loadAvg15"`
	// 核心数
	Cores int `json:"cores"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	// 总内存（字节）
	Total uint64 `json:"total"`
	// 已用内存（字节）
	Used uint64 `json:"used"`
	// 可用内存（字节）
	Available uint64 `json:"available"`
	// 使用率（百分比）
	UsagePercent float64 `json:"usagePercent"`
	// Swap 总量（字节）
	SwapTotal uint64 `json:"swapTotal"`
	// Swap 已用（字节）
	SwapUsed uint64 `json:"swapUsed"`
	// Swap 使用率（百分比）
	SwapUsagePercent float64 `json:"swapUsagePercent"`
	// 内存压力指标（0-100）
	Pressure float64 `json:"pressure"`
}

// DiskIOInfo 磁盘 I/O 信息
type DiskIOInfo struct {
	// 读取速率（字节/秒）
	ReadBytesPerSec uint64 `json:"readBytesPerSec"`
	// 写入速率（字节/秒）
	WriteBytesPerSec uint64 `json:"writeBytesPerSec"`
	// 读取 IOPS
	ReadIOPS uint64 `json:"readIOPS"`
	// 写入 IOPS
	WriteIOPS uint64 `json:"writeIOPS"`
	// 读取延迟（毫秒）
	ReadLatency float64 `json:"readLatency"`
	// 写入延迟（毫秒）
	WriteLatency float64 `json:"writeLatency"`
	// 各磁盘使用情况
	Partitions []DiskPartition `json:"partitions"`
}

// DiskPartition 磁盘分区信息
type DiskPartition struct {
	// 挂载点
	Mountpoint string `json:"mountpoint"`
	// 文件系统类型
	FSType string `json:"fsType"`
	// 总容量（字节）
	Total uint64 `json:"total"`
	// 已用（字节）
	Used uint64 `json:"used"`
	// 使用率（百分比）
	UsagePercent float64 `json:"usagePercent"`
}

// NetworkInfo 网络流量信息
type NetworkInfo struct {
	// 上行速率（字节/秒）
	UploadPerSec uint64 `json:"uploadPerSec"`
	// 下行速率（字节/秒）
	DownloadPerSec uint64 `json:"downloadPerSec"`
	// 总连接数
	Connections int `json:"connections"`
	// 网络错误数
	Errors uint64 `json:"errors"`
	// 各网卡统计
	Interfaces []NetInterface `json:"interfaces"`
}

// NetInterface 网卡统计
type NetInterface struct {
	// 网卡名称
	Name string `json:"name"`
	// 上行字节
	BytesSent uint64 `json:"bytesSent"`
	// 下行字节
	BytesRecv uint64 `json:"bytesRecv"`
	// 是否为物理网卡（排除 lo）
	IsPhysical bool `json:"isPhysical"`
}

// GPUInfo GPU 信息（如不可用则返回空）
type GPUInfo struct {
	// GPU 是否存在
	Available bool `json:"available"`
	// GPU 使用率（百分比）
	UsagePercent float64 `json:"usagePercent"`
	// GPU 温度（摄氏度）
	Temperature float64 `json:"temperature"`
	// 显存总量（字节）
	MemoryTotal uint64 `json:"memoryTotal"`
	// 显存已用（字节）
	MemoryUsed uint64 `json:"memoryUsed"`
	// 显存使用率（百分比）
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
}

// ResourceSnapshot 资源快照
type ResourceSnapshot struct {
	// 采集时间
	Timestamp time.Time `json:"timestamp"`
	// CPU 信息
	CPU CPUInfo `json:"cpu"`
	// 内存信息
	Memory MemoryInfo `json:"memory"`
	// 磁盘 I/O 信息
	DiskIO DiskIOInfo `json:"diskIO"`
	// 网络信息
	Network NetworkInfo `json:"network"`
	// GPU 信息
	GPU GPUInfo `json:"gpu"`
	// 系统启动时间（Unix 时间戳）
	Uptime uint64 `json:"uptime"`
}

// NewResourceMonitor 创建资源监控器
func NewResourceMonitor(cfg *Config) *ResourceMonitor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 2880
	}

	return &ResourceMonitor{
		config:  cfg,
		history: NewRingBuffer(cfg.BufferSize),
		stopped: make(chan struct{}),
	}
}

// Start 启动监控采集
func (rm *ResourceMonitor) Start(ctx context.Context) error {
	rm.mu.Lock()
	if rm.cancel != nil {
		rm.mu.Unlock()
		return fmt.Errorf("monitor already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	rm.cancel = cancel
	rm.mu.Unlock()

	// 立即采集一次
	snapshot, err := rm.collect()
	if err == nil {
		rm.history.Push(snapshot)
	}

	// 启动定时采集协程
	go rm.loop(ctx)

	return nil
}

// Stop 停止监控
func (rm *ResourceMonitor) Stop() {
	rm.mu.Lock()
	if rm.cancel != nil {
		rm.cancel()
		rm.cancel = nil
	}
	rm.mu.Unlock()

	// 等待 loop 退出（不在锁内等待，避免死锁）
	<-rm.stopped
}

// GetLatest 获取最新快照
func (rm *ResourceMonitor) GetLatest() *ResourceSnapshot {
	if snap := rm.history.Latest(); snap != nil {
		return snap
	}
	// 如果历史为空，实时采集一次
	snapshot, err := rm.collect()
	if err != nil {
		return nil
	}
	return &snapshot
}

// GetHistory 获取历史数据
func (rm *ResourceMonitor) GetHistory(duration time.Duration) []ResourceSnapshot {
	since := time.Now().Add(-duration)
	return rm.history.Since(since)
}

// GetConfig 获取配置
func (rm *ResourceMonitor) GetConfig() *Config {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.config
}

// loop 采集循环
func (rm *ResourceMonitor) loop(ctx context.Context) {
	defer func() {
		rm.mu.Lock()
		close(rm.stopped)
		rm.mu.Unlock()
	}()

	ticker := time.NewTicker(time.Duration(rm.config.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot, err := rm.collect()
			if err != nil {
				// 采集失败时跳过本次
				continue
			}
			rm.history.Push(snapshot)
		}
	}
}

// collect 采集一次系统资源数据
func (rm *ResourceMonitor) collect() (ResourceSnapshot, error) {
	snap := ResourceSnapshot{
		Timestamp: time.Now(),
	}

	// 并发采集各项数据
	var wg sync.WaitGroup
	var cpuErr, memErr, diskErr, netErr error

	wg.Add(4)

	go func() {
		defer wg.Done()
		snap.CPU, cpuErr = rm.collectCPU()
	}()

	go func() {
		defer wg.Done()
		snap.Memory, memErr = rm.collectMemory()
	}()

	go func() {
		defer wg.Done()
		snap.DiskIO, diskErr = rm.collectDiskIO()
	}()

	go func() {
		defer wg.Done()
		snap.Network, netErr = rm.collectNetwork()
	}()

	// GPU 和 Uptime 单独采集
	snap.GPU = rm.collectGPU()

	if hostInfo, err := host.Uptime(); err == nil {
		snap.Uptime = hostInfo
	}

	wg.Wait()

	// 如果核心采集全部失败则报错
	if cpuErr != nil && memErr != nil && diskErr != nil && netErr != nil {
		return snap, fmt.Errorf("all collectors failed: cpu=%v, mem=%v, disk=%v, net=%v",
			cpuErr, memErr, diskErr, netErr)
	}

	return snap, nil
}

// collectCPU 采集 CPU 信息
func (rm *ResourceMonitor) collectCPU() (CPUInfo, error) {
	info := CPUInfo{}

	// 获取每核心使用率
	perCore, err := cpu.Percent(0, true)
	if err != nil {
		return info, err
	}
	info.PerCore = perCore
	info.Cores = len(perCore)

	// 计算总体使用率
	total := 0.0
	for _, v := range perCore {
		total += v
	}
	if info.Cores > 0 {
		info.UsagePercent = math.Round(total/float64(info.Cores)*100) / 100
	}

	// 获取负载平均值
	if avg, err := load.Avg(); err == nil {
		info.LoadAvg1 = avg.Load1
		info.LoadAvg5 = avg.Load5
		info.LoadAvg15 = avg.Load15
	}

	// 获取 CPU 频率
	if freq, err := cpu.Info(); err == nil && len(freq) > 0 {
		info.Frequency = freq[0].Mhz
	}

	// 尝试获取温度（部分系统不支持）
	info.Temperature = rm.getCPUTemperature()

	return info, nil
}

// getCPUTemperature 获取 CPU 温度
func (rm *ResourceMonitor) getCPUTemperature() float64 {
	// 使用 host.SensorsTemperatures() 获取温度
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0
	}

	// 查找 CPU 相关温度传感器
	for _, t := range temps {
		if t.Temperature > 0 {
			return t.Temperature
		}
	}
	return 0
}

// collectMemory 采集内存信息
func (rm *ResourceMonitor) collectMemory() (MemoryInfo, error) {
	info := MemoryInfo{}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return info, err
	}

	info.Total = vm.Total
	info.Used = vm.Used
	info.Available = vm.Available
	info.UsagePercent = math.Round(vm.UsedPercent*100) / 100

	// Swap 信息
	if sw, err := mem.SwapMemory(); err == nil {
		info.SwapTotal = sw.Total
		info.SwapUsed = sw.Used
		info.SwapUsagePercent = math.Round(sw.UsedPercent*100) / 100
	}

	// 计算内存压力指标
	// 基于使用率和 Swap 使用率综合计算
	swapPressure := info.SwapUsagePercent * 0.3
	memPressure := info.UsagePercent * 0.7
	info.Pressure = math.Round((memPressure+swapPressure)*100) / 100

	return info, nil
}

// collectDiskIO 采集磁盘 I/O 信息
func (rm *ResourceMonitor) collectDiskIO() (DiskIOInfo, error) {
	info := DiskIOInfo{}

	// 获取磁盘分区信息
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, p := range partitions {
			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}
			info.Partitions = append(info.Partitions, DiskPartition{
				Mountpoint:   p.Mountpoint,
				FSType:       p.Fstype,
				Total:        usage.Total,
				Used:         usage.Used,
				UsagePercent: math.Round(usage.UsedPercent*100) / 100,
			})
		}
	}

	// 获取 I/O 统计（两次采样计算速率）
	ioCounters1, err := disk.IOCounters()
	if err != nil {
		return info, nil
	}

	time.Sleep(100 * time.Millisecond)

	ioCounters2, err := disk.IOCounters()
	if err != nil {
		return info, nil
	}

	// 计算总 I/O
	var totalReadBytes, totalWriteBytes, totalReadCount, totalWriteCount uint64
	for name, io1 := range ioCounters1 {
		if io2, ok := ioCounters2[name]; ok {
			totalReadBytes += io2.ReadBytes - io1.ReadBytes
			totalWriteBytes += io2.WriteBytes - io1.WriteBytes
			totalReadCount += io2.ReadCount - io1.ReadCount
			totalWriteCount += io2.WriteCount - io1.WriteCount
		}
	}

	// 换算为每秒（采样间隔 0.1 秒）
	info.ReadBytesPerSec = totalReadBytes * 10
	info.WriteBytesPerSec = totalWriteBytes * 10
	info.ReadIOPS = totalReadCount * 10
	info.WriteIOPS = totalWriteCount * 10

	// 延迟估算（基于 IOPS 和队列深度，简化处理）
	if info.ReadIOPS > 0 {
		info.ReadLatency = 1000.0 / float64(info.ReadIOPS)
	}
	if info.WriteIOPS > 0 {
		info.WriteLatency = 1000.0 / float64(info.WriteIOPS)
	}

	return info, nil
}

// collectNetwork 采集网络流量信息
func (rm *ResourceMonitor) collectNetwork() (NetworkInfo, error) {
	info := NetworkInfo{}

	// 获取网卡统计
	counters1, err := net.IOCounters(true)
	if err != nil {
		return info, err
	}

	time.Sleep(100 * time.Millisecond)

	counters2, err := net.IOCounters(true)
	if err != nil {
		return info, err
	}

	var totalUp, totalDown, totalErrors uint64
	for i, c1 := range counters1 {
		if i < len(counters2) {
			c2 := counters2[i]
			isPhysical := c1.Name != "lo"

			iface := NetInterface{
				Name:       c1.Name,
				BytesSent:  c2.BytesSent - c1.BytesSent,
				BytesRecv:  c2.BytesRecv - c1.BytesRecv,
				IsPhysical: isPhysical,
			}
			info.Interfaces = append(info.Interfaces, iface)

			if isPhysical {
				totalUp += iface.BytesSent
				totalDown += iface.BytesRecv
				totalErrors += c2.Errin - c1.Errin + c2.Errout - c1.Errout
			}
		}
	}

	// 换算为每秒
	info.UploadPerSec = totalUp * 10
	info.DownloadPerSec = totalDown * 10
	info.Errors = totalErrors * 10

	// 获取连接数
	if conns, err := net.Connections("all"); err == nil {
		info.Connections = len(conns)
	}

	return info, nil
}

// collectGPU 采集 GPU 信息（尝试 nvidia-smi）
func (rm *ResourceMonitor) collectGPU() GPUInfo {
	info := GPUInfo{Available: false}

	// GPU 监控需要 nvidia-smi，此处简化处理
	// 实际生产环境建议使用 NVML 库
	return info
}
