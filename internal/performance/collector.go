package performance

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// SystemCollector 系统指标收集器
type SystemCollector struct {
	logger *zap.Logger
	mu     sync.RWMutex

	// 历史数据
	cpuHistory     []CPUMetric
	memoryHistory  []MemoryMetric
	diskHistory    []DiskMetric
	networkHistory []NetworkMetric

	// 上一次采集的数据（用于计算速率）
	prevDiskIO   map[string]DiskIOMetric
	prevNetwork  map[string]NetworkIOMetric
	prevCPUTimes CPUTimes

	// 配置
	historySize int
	interval    time.Duration

	// 回调
	onAlert func(alertType, message string)
}

// CPUMetric CPU 指标
type CPUMetric struct {
	UsagePercent  float64   `json:"usage_percent"`
	UserPercent   float64   `json:"user_percent"`
	SystemPercent float64   `json:"system_percent"`
	IdlePercent   float64   `json:"idle_percent"`
	IOWaitPercent float64   `json:"iowait_percent"`
	Cores         int       `json:"cores"`
	LoadAvg1      float64   `json:"load_avg_1"`
	LoadAvg5      float64   `json:"load_avg_5"`
	LoadAvg15     float64   `json:"load_avg_15"`
	Temperature   int       `json:"temperature,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// CPUTimes CPU 时间片
type CPUTimes struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
	Total   uint64
}

// MemoryMetric 内存指标
type MemoryMetric struct {
	TotalBytes     uint64    `json:"total_bytes"`
	UsedBytes      uint64    `json:"used_bytes"`
	FreeBytes      uint64    `json:"free_bytes"`
	AvailableBytes uint64    `json:"available_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
	SwapTotalBytes uint64    `json:"swap_total_bytes"`
	SwapUsedBytes  uint64    `json:"swap_used_bytes"`
	SwapPercent    float64   `json:"swap_percent"`
	CachedBytes    uint64    `json:"cached_bytes"`
	BuffersBytes   uint64    `json:"buffers_bytes"`
	SharedBytes    uint64    `json:"shared_bytes"`
	Timestamp      time.Time `json:"timestamp"`
}

// DiskMetric 磁盘指标
type DiskMetric struct {
	Device       string    `json:"device"`
	MountPoint   string    `json:"mount_point"`
	TotalBytes   uint64    `json:"total_bytes"`
	UsedBytes    uint64    `json:"used_bytes"`
	FreeBytes    uint64    `json:"free_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	InodeTotal   uint64    `json:"inode_total"`
	InodeUsed    uint64    `json:"inode_used"`
	InodePercent float64   `json:"inode_percent"`
	Timestamp    time.Time `json:"timestamp"`
}

// DiskIOMetric 磁盘 I/O 指标
type DiskIOMetric struct {
	Device       string    `json:"device"`
	ReadBytes    uint64    `json:"read_bytes"`
	WriteBytes   uint64    `json:"write_bytes"`
	ReadOps      uint64    `json:"read_ops"`
	WriteOps     uint64    `json:"write_ops"`
	ReadSpeed    uint64    `json:"read_speed"`    // bytes/s
	WriteSpeed   uint64    `json:"write_speed"`   // bytes/s
	ReadLatency  float64   `json:"read_latency"`  // ms
	WriteLatency float64   `json:"write_latency"` // ms
	IOPS         uint64    `json:"iops"`
	Throughput   uint64    `json:"throughput"` // bytes/s
	Timestamp    time.Time `json:"timestamp"`
	// 内部使用
	ReadMs  uint64 `json:"-"`
	WriteMs uint64 `json:"-"`
}

// NetworkMetric 网络指标
type NetworkMetric struct {
	Interface string    `json:"interface"`
	RXBytes   uint64    `json:"rx_bytes"`
	TXBytes   uint64    `json:"tx_bytes"`
	RXPackets uint64    `json:"rx_packets"`
	TXPackets uint64    `json:"tx_packets"`
	RXErrors  uint64    `json:"rx_errors"`
	TXErrors  uint64    `json:"tx_errors"`
	RXDropped uint64    `json:"rx_dropped"`
	TXDropped uint64    `json:"tx_dropped"`
	RXSpeed   uint64    `json:"rx_speed"` // bytes/s
	TXSpeed   uint64    `json:"tx_speed"` // bytes/s
	Timestamp time.Time `json:"timestamp"`
}

// NetworkIOMetric 网络 I/O 指标
type NetworkIOMetric struct {
	Interface string
	RXBytes   uint64
	TXBytes   uint64
	RXPackets uint64
	TXPackets uint64
	RXErrors  uint64
	TXErrors  uint64
	RXDropped uint64
	TXDropped uint64
	Timestamp time.Time
}

// SystemMetricsSummary 系统指标汇总
type SystemMetricsSummary struct {
	CPU     CPUMetric       `json:"cpu"`
	Memory  MemoryMetric    `json:"memory"`
	Disks   []DiskMetric    `json:"disks"`
	DiskIO  []DiskIOMetric  `json:"disk_io"`
	Network []NetworkMetric `json:"network"`
	Uptime  uint64          `json:"uptime_seconds"`
}

// NewSystemCollector 创建系统指标收集器
func NewSystemCollector(logger *zap.Logger, historySize int) *SystemCollector {
	return &SystemCollector{
		logger:         logger,
		cpuHistory:     make([]CPUMetric, 0, historySize),
		memoryHistory:  make([]MemoryMetric, 0, historySize),
		diskHistory:    make([]DiskMetric, 0, historySize*10),
		networkHistory: make([]NetworkMetric, 0, historySize*5),
		prevDiskIO:     make(map[string]DiskIOMetric),
		prevNetwork:    make(map[string]NetworkIOMetric),
		historySize:    historySize,
		interval:       10 * time.Second,
	}
}

// SetAlertCallback 设置告警回调
func (sc *SystemCollector) SetAlertCallback(callback func(alertType, message string)) {
	sc.onAlert = callback
}

// Collect 收集所有系统指标
func (sc *SystemCollector) Collect() *SystemMetricsSummary {
	summary := &SystemMetricsSummary{
		CPU:     sc.collectCPU(),
		Memory:  sc.collectMemory(),
		Disks:   sc.collectDisks(),
		DiskIO:  sc.collectDiskIO(),
		Network: sc.collectNetwork(),
		Uptime:  sc.getUptime(),
	}

	// 更新历史
	sc.mu.Lock()
	sc.cpuHistory = append(sc.cpuHistory, summary.CPU)
	if len(sc.cpuHistory) > sc.historySize {
		sc.cpuHistory = sc.cpuHistory[1:]
	}
	sc.memoryHistory = append(sc.memoryHistory, summary.Memory)
	if len(sc.memoryHistory) > sc.historySize {
		sc.memoryHistory = sc.memoryHistory[1:]
	}
	sc.mu.Unlock()

	return summary
}

// collectCPU 收集 CPU 指标
func (sc *SystemCollector) collectCPU() CPUMetric {
	metric := CPUMetric{
		Timestamp: time.Now(),
	}

	// 读取 /proc/stat
	if file, err := os.Open("/proc/stat"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 8 && fields[0] == "cpu" {
				user, _ := strconv.ParseUint(fields[1], 10, 64)
				nice, _ := strconv.ParseUint(fields[2], 10, 64)
				system, _ := strconv.ParseUint(fields[3], 10, 64)
				idle, _ := strconv.ParseUint(fields[4], 10, 64)
				iowait, _ := strconv.ParseUint(fields[5], 10, 64)
				irq, _ := strconv.ParseUint(fields[6], 10, 64)
				softirq, _ := strconv.ParseUint(fields[7], 10, 64)

				current := CPUTimes{
					User:    user,
					Nice:    nice,
					System:  system,
					Idle:    idle,
					IOWait:  iowait,
					IRQ:     irq,
					SoftIRQ: softirq,
				}
				current.Total = current.User + current.Nice + current.System + current.Idle +
					current.IOWait + current.IRQ + current.SoftIRQ

				// 计算差值
				if sc.prevCPUTimes.Total > 0 {
					deltaTotal := current.Total - sc.prevCPUTimes.Total
					if deltaTotal > 0 {
						deltaUser := current.User - sc.prevCPUTimes.User
						deltaSystem := current.System - sc.prevCPUTimes.System
						deltaIdle := current.Idle - sc.prevCPUTimes.Idle
						deltaIOWait := current.IOWait - sc.prevCPUTimes.IOWait

						metric.UserPercent = float64(deltaUser) / float64(deltaTotal) * 100
						metric.SystemPercent = float64(deltaSystem) / float64(deltaTotal) * 100
						metric.IdlePercent = float64(deltaIdle) / float64(deltaTotal) * 100
						metric.IOWaitPercent = float64(deltaIOWait) / float64(deltaTotal) * 100
						metric.UsagePercent = 100 - metric.IdlePercent
					}
				}

				sc.prevCPUTimes = current
			}
		}
	}

	// 读取负载
	if file, err := os.Open("/proc/loadavg"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 {
				metric.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
				metric.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
				metric.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
			}
		}
	}

	// CPU 核心数
	metric.Cores = getCPUCores()

	// CPU 温度
	metric.Temperature = sc.getCPUTemperature()

	return metric
}

// collectMemory 收集内存指标
func (sc *SystemCollector) collectMemory() MemoryMetric {
	metric := MemoryMetric{
		Timestamp: time.Now(),
	}

	if file, err := os.Open("/proc/meminfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}

			value, _ := strconv.ParseUint(fields[1], 10, 64)
			value *= 1024 // KB to bytes

			switch fields[0] {
			case "MemTotal:":
				metric.TotalBytes = value
			case "MemFree:":
				metric.FreeBytes = value
			case "MemAvailable:":
				metric.AvailableBytes = value
			case "Buffers:":
				metric.BuffersBytes = value
			case "Cached:":
				metric.CachedBytes = value
			case "Shmem:":
				metric.SharedBytes = value
			case "SwapTotal:":
				metric.SwapTotalBytes = value
			case "SwapFree:":
				swapFree := value
				metric.SwapUsedBytes = metric.SwapTotalBytes - swapFree
			}
		}
	}

	metric.UsedBytes = metric.TotalBytes - metric.AvailableBytes
	if metric.TotalBytes > 0 {
		metric.UsagePercent = float64(metric.UsedBytes) / float64(metric.TotalBytes) * 100
	}
	if metric.SwapTotalBytes > 0 {
		metric.SwapPercent = float64(metric.SwapUsedBytes) / float64(metric.SwapTotalBytes) * 100
	}

	return metric
}

// collectDisks 收集磁盘使用指标
func (sc *SystemCollector) collectDisks() []DiskMetric {
	var metrics []DiskMetric

	if file, err := os.Open("/proc/mounts"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		now := time.Now()

		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}

			device := fields[0]
			mountPoint := fields[1]

			// 只关心实际设备
			if !strings.HasPrefix(device, "/dev/") {
				continue
			}

			// 获取磁盘使用情况
			if stat, err := getDiskUsage(mountPoint); err == nil {
				metric := DiskMetric{
					Device:       device,
					MountPoint:   mountPoint,
					TotalBytes:   stat.TotalBytes,
					UsedBytes:    stat.UsedBytes,
					FreeBytes:    stat.FreeBytes,
					UsagePercent: stat.UsagePercent,
					InodeTotal:   stat.InodeTotal,
					InodeUsed:    stat.InodeUsed,
					InodePercent: stat.InodePercent,
					Timestamp:    now,
				}
				metrics = append(metrics, metric)
			}
		}
	}

	return metrics
}

// collectDiskIO 收集磁盘 I/O 指标
func (sc *SystemCollector) collectDiskIO() []DiskIOMetric {
	var metrics []DiskIOMetric

	if file, err := os.Open("/proc/diskstats"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		now := time.Now()

		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 14 {
				continue
			}

			device := fields[2]
			// 过滤非物理磁盘
			if !isPhysicalDisk(device) {
				continue
			}

			readOps, _ := strconv.ParseUint(fields[3], 10, 64)
			readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
			writeOps, _ := strconv.ParseUint(fields[7], 10, 64)
			writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
			readMs, _ := strconv.ParseUint(fields[6], 10, 64)
			writeMs, _ := strconv.ParseUint(fields[10], 10, 64)

			readBytes := readSectors * 512
			writeBytes := writeSectors * 512

			metric := DiskIOMetric{
				Device:     device,
				ReadBytes:  readBytes,
				WriteBytes: writeBytes,
				ReadOps:    readOps,
				WriteOps:   writeOps,
				Timestamp:  now,
			}

			// 计算速率和延迟
			if prev, ok := sc.prevDiskIO[device]; ok {
				elapsed := now.Sub(prev.Timestamp).Seconds()
				if elapsed > 0 {
					metric.ReadSpeed = uint64(float64(readBytes-prev.ReadBytes) / elapsed)
					metric.WriteSpeed = uint64(float64(writeBytes-prev.WriteBytes) / elapsed)
					metric.IOPS = uint64(float64(readOps+writeOps-prev.ReadOps-prev.WriteOps) / elapsed)
					metric.Throughput = metric.ReadSpeed + metric.WriteSpeed

					// 计算平均延迟
					deltaReadOps := readOps - prev.ReadOps
					deltaWriteOps := writeOps - prev.WriteOps
					deltaReadMs := readMs - prev.ReadMs
					deltaWriteMs := writeMs - prev.WriteMs

					if deltaReadOps > 0 {
						metric.ReadLatency = float64(deltaReadMs) / float64(deltaReadOps)
					}
					if deltaWriteOps > 0 {
						metric.WriteLatency = float64(deltaWriteMs) / float64(deltaWriteOps)
					}
				}
			}

			// 保存当前值（包含延迟计算需要的毫秒数）
			sc.prevDiskIO[device] = DiskIOMetric{
				Device:     device,
				ReadBytes:  readBytes,
				WriteBytes: writeBytes,
				ReadOps:    readOps,
				WriteOps:   writeOps,
				ReadMs:     readMs,
				WriteMs:    writeMs,
				Timestamp:  now,
			}

			metrics = append(metrics, metric)
		}
	}

	return metrics
}

// collectNetwork 收集网络指标
func (sc *SystemCollector) collectNetwork() []NetworkMetric {
	var metrics []NetworkMetric

	if file, err := os.Open("/proc/net/dev"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		now := time.Now()

		// 跳过头部
		scanner.Scan()
		scanner.Scan()

		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			iface := strings.TrimSpace(parts[0])
			// 跳过 loopback
			if iface == "lo" {
				continue
			}

			fields := strings.Fields(parts[1])
			if len(fields) < 16 {
				continue
			}

			rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
			rxPackets, _ := strconv.ParseUint(fields[1], 10, 64)
			rxErrors, _ := strconv.ParseUint(fields[2], 10, 64)
			rxDropped, _ := strconv.ParseUint(fields[3], 10, 64)
			txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
			txPackets, _ := strconv.ParseUint(fields[9], 10, 64)
			txErrors, _ := strconv.ParseUint(fields[10], 10, 64)
			txDropped, _ := strconv.ParseUint(fields[11], 10, 64)

			metric := NetworkMetric{
				Interface: iface,
				RXBytes:   rxBytes,
				TXBytes:   txBytes,
				RXPackets: rxPackets,
				TXPackets: txPackets,
				RXErrors:  rxErrors,
				TXErrors:  txErrors,
				RXDropped: rxDropped,
				TXDropped: txDropped,
				Timestamp: now,
			}

			// 计算速率
			if prev, ok := sc.prevNetwork[iface]; ok {
				elapsed := now.Sub(prev.Timestamp).Seconds()
				if elapsed > 0 {
					metric.RXSpeed = uint64(float64(rxBytes-prev.RXBytes) / elapsed)
					metric.TXSpeed = uint64(float64(txBytes-prev.TXBytes) / elapsed)
				}
			}

			sc.prevNetwork[iface] = NetworkIOMetric{
				Interface: iface,
				RXBytes:   rxBytes,
				TXBytes:   txBytes,
				RXPackets: rxPackets,
				TXPackets: txPackets,
				Timestamp: now,
			}

			metrics = append(metrics, metric)
		}
	}

	return metrics
}

// getUptime 获取系统运行时间
func (sc *SystemCollector) getUptime() uint64 {
	if file, err := os.Open("/proc/uptime"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 1 {
				uptime, _ := strconv.ParseFloat(fields[0], 64)
				return uint64(uptime)
			}
		}
	}
	return 0
}

// getCPUTemperature 获取 CPU 温度
func (sc *SystemCollector) getCPUTemperature() int {
	// 尝试读取 thermal_zone
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return temp / 1000
	}
	// 尝试 hwmon
	if data, err := os.ReadFile("/sys/class/hwmon/hwmon0/temp1_input"); err == nil {
		temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return temp / 1000
	}
	return 0
}

// GetHistory 获取历史数据
func (sc *SystemCollector) GetHistory() (cpu, memory []float64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	cpu = make([]float64, len(sc.cpuHistory))
	for i, m := range sc.cpuHistory {
		cpu[i] = m.UsagePercent
	}

	memory = make([]float64, len(sc.memoryHistory))
	for i, m := range sc.memoryHistory {
		memory[i] = m.UsagePercent
	}

	return
}

// GetCPUHistory 获取 CPU 历史数据
func (sc *SystemCollector) GetCPUHistory() []CPUMetric {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]CPUMetric, len(sc.cpuHistory))
	copy(result, sc.cpuHistory)
	return result
}

// GetMemoryHistory 获取内存历史数据
func (sc *SystemCollector) GetMemoryHistory() []MemoryMetric {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]MemoryMetric, len(sc.memoryHistory))
	copy(result, sc.memoryHistory)
	return result
}

// 辅助函数

func getCPUCores() int {
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		cores := 0
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "processor") {
				cores++
			}
		}
		return cores
	}
	return 1
}

type diskUsageStat struct {
	TotalBytes   uint64
	UsedBytes    uint64
	FreeBytes    uint64
	UsagePercent float64
	InodeTotal   uint64
	InodeUsed    uint64
	InodePercent float64
}

func getDiskUsage(path string) (*diskUsageStat, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	inodeTotal := stat.Files
	inodeFree := stat.Ffree
	inodeUsed := inodeTotal - inodeFree

	result := &diskUsageStat{
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  free,
		InodeTotal: inodeTotal,
		InodeUsed:  inodeUsed,
	}

	if total > 0 {
		result.UsagePercent = float64(used) / float64(total) * 100
	}
	if inodeTotal > 0 {
		result.InodePercent = float64(inodeUsed) / float64(inodeTotal) * 100
	}

	return result, nil
}

func isPhysicalDisk(device string) bool {
	// 检查是否是物理磁盘
	prefixes := []string{"sd", "nvme", "vd", "hd", "xvd"}
	for _, p := range prefixes {
		if strings.HasPrefix(device, p) {
			// 排除分区
			if len(device) > len(p) {
				rest := device[len(p):]
				// 如果后面跟着数字，是分区
				if _, err := strconv.Atoi(rest); err == nil {
					return false
				}
				// nvme 格式: nvme0n1p1 (p 后面是分区)
				if strings.HasPrefix(device, "nvme") && strings.Contains(rest, "p") {
					return false
				}
			}
			return true
		}
	}
	return false
}
