package performance

import (
	"bufio"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StorageMetrics 存储性能指标
type StorageMetrics struct {
	// IOPS 指标
	IOPS struct {
		Read     uint64 `json:"read"`
		Write    uint64 `json:"write"`
		Total    uint64 `json:"total"`
		ReadAvg  uint64 `json:"read_avg"`
		WriteAvg uint64 `json:"write_avg"`
		ReadMax  uint64 `json:"read_max"`
		WriteMax uint64 `json:"write_max"`
	} `json:"iops"`

	// 延迟指标
	Latency struct {
		ReadAvgMs  float64 `json:"read_avg_ms"`
		WriteAvgMs float64 `json:"write_avg_ms"`
		ReadP95Ms  float64 `json:"read_p95_ms"`
		WriteP95Ms float64 `json:"write_p95_ms"`
		ReadP99Ms  float64 `json:"read_p99_ms"`
		WriteP99Ms float64 `json:"write_p99_ms"`
		ReadMaxMs  float64 `json:"read_max_ms"`
		WriteMaxMs float64 `json:"write_max_ms"`
	} `json:"latency"`

	// 吞吐量指标
	Throughput struct {
		ReadBytesPerSec  uint64  `json:"read_bytes_per_sec"`
		WriteBytesPerSec uint64  `json:"write_bytes_per_sec"`
		TotalBytesPerSec uint64  `json:"total_bytes_per_sec"`
		ReadMBPerSec     float64 `json:"read_mb_per_sec"`
		WriteMBPerSec    float64 `json:"write_mb_per_sec"`
		TotalMBPerSec    float64 `json:"total_mb_per_sec"`
	} `json:"throughput"`

	// 队列深度
	QueueDepth struct {
		Current float64 `json:"current"`
		Avg     float64 `json:"avg"`
		Max     float64 `json:"max"`
	} `json:"queue_depth"`

	// 缓存指标
	Cache struct {
		ReadHits    uint64  `json:"read_hits"`
		ReadMisses  uint64  `json:"read_misses"`
		WriteHits   uint64  `json:"write_hits"`
		WriteMisses uint64  `json:"write_misses"`
		HitRate     float64 `json:"hit_rate"`
		DirtyPages  uint64  `json:"dirty_pages"`
	} `json:"cache"`

	// 设备列表
	Devices []StorageDeviceMetrics `json:"devices"`

	Timestamp time.Time `json:"timestamp"`
}

// StorageDeviceMetrics 单个存储设备指标
type StorageDeviceMetrics struct {
	Device      string `json:"device"`
	Type        string `json:"type"` // ssd, hdd, nvme
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	SizeBytes   uint64 `json:"size_bytes"`
	Temperature int    `json:"temperature,omitempty"`
	Health      int    `json:"health,omitempty"` // 0-100

	// IOPS
	ReadIOPS  uint64 `json:"read_iops"`
	WriteIOPS uint64 `json:"write_iops"`

	// 延迟
	ReadLatencyMs  float64 `json:"read_latency_ms"`
	WriteLatencyMs float64 `json:"write_latency_ms"`

	// 吞吐量
	ReadSpeed  uint64 `json:"read_speed"`  // bytes/s
	WriteSpeed uint64 `json:"write_speed"` // bytes/s

	// SMART 状态
	SMARTStatus    string `json:"smart_status,omitempty"`
	Reallocates    uint64 `json:"reallocates,omitempty"`
	PendingSectors uint64 `json:"pending_sectors,omitempty"`
}

// StorageCollector 存储性能收集器
type StorageCollector struct {
	logger      *zap.Logger
	mu          sync.RWMutex
	history     []StorageMetrics
	historySize int
	collector   *SystemCollector
}

// NewStorageCollector 创建存储性能收集器
func NewStorageCollector(logger *zap.Logger, collector *SystemCollector, historySize int) *StorageCollector {
	return &StorageCollector{
		logger:      logger,
		history:     make([]StorageMetrics, 0, historySize),
		historySize: historySize,
		collector:   collector,
	}
}

// Collect 收集存储性能指标
func (sc *StorageCollector) Collect() *StorageMetrics {
	metrics := &StorageMetrics{
		Timestamp: time.Now(),
	}

	// 从系统收集器获取磁盘 I/O
	diskIO := sc.collector.collectDiskIO()

	for _, dio := range diskIO {
		device := StorageDeviceMetrics{
			Device:         dio.Device,
			ReadIOPS:       dio.ReadOps,
			WriteIOPS:      dio.WriteOps,
			ReadLatencyMs:  dio.ReadLatency,
			WriteLatencyMs: dio.WriteLatency,
			ReadSpeed:      dio.ReadSpeed,
			WriteSpeed:     dio.WriteSpeed,
		}

		// 获取设备类型和信息
		device.Type = sc.getDeviceType(dio.Device)
		device.SizeBytes = sc.getDeviceSize(dio.Device)
		device.Temperature = sc.getDeviceTemperature(dio.Device)

		metrics.Devices = append(metrics.Devices, device)

		// 汇总
		metrics.IOPS.Read += dio.ReadOps
		metrics.IOPS.Write += dio.WriteOps
		metrics.Throughput.ReadBytesPerSec += dio.ReadSpeed
		metrics.Throughput.WriteBytesPerSec += dio.WriteSpeed
	}

	metrics.IOPS.Total = metrics.IOPS.Read + metrics.IOPS.Write
	metrics.Throughput.TotalBytesPerSec = metrics.Throughput.ReadBytesPerSec + metrics.Throughput.WriteBytesPerSec
	metrics.Throughput.ReadMBPerSec = float64(metrics.Throughput.ReadBytesPerSec) / 1024 / 1024
	metrics.Throughput.WriteMBPerSec = float64(metrics.Throughput.WriteBytesPerSec) / 1024 / 1024
	metrics.Throughput.TotalMBPerSec = metrics.Throughput.ReadMBPerSec + metrics.Throughput.WriteMBPerSec

	// 计算平均延迟
	if len(metrics.Devices) > 0 {
		var readSum, writeSum float64
		for _, d := range metrics.Devices {
			readSum += d.ReadLatencyMs
			writeSum += d.WriteLatencyMs
		}
		metrics.Latency.ReadAvgMs = readSum / float64(len(metrics.Devices))
		metrics.Latency.WriteAvgMs = writeSum / float64(len(metrics.Devices))
	}

	// 获取系统缓存信息
	sc.collectCacheMetrics(metrics)

	// 保存历史
	sc.mu.Lock()
	sc.history = append(sc.history, *metrics)
	if len(sc.history) > sc.historySize {
		sc.history = sc.history[1:]
	}
	sc.mu.Unlock()

	return metrics
}

// collectCacheMetrics 收集缓存指标
func (sc *StorageCollector) collectCacheMetrics(metrics *StorageMetrics) {
	// 读取 /proc/meminfo 获取页面缓存信息
	if file, err := os.Open("/proc/meminfo"); err == nil {
		defer func() { _ = file.Close() }()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Dirty:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					value, err := strconv.ParseUint(fields[1], 10, 64)
					if err == nil {
						metrics.Cache.DirtyPages = value * 1024 // KB to bytes
					}
				}
			}
		}
	}

	// 从 /proc/vmstat 获取缓存命中/未命中
	if file, err := os.Open("/proc/vmstat"); err == nil {
		defer func() { _ = file.Close() }()

		var pswpin, pswpout uint64
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "pswpin ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					val, err := strconv.ParseUint(fields[1], 10, 64)
					if err == nil {
						pswpin = val
					}
				}
			} else if strings.HasPrefix(line, "pswpout ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					val, err := strconv.ParseUint(fields[1], 10, 64)
					if err == nil {
						pswpout = val
					}
				}
			}
		}
		// 使用 swap 活动作为代理指标
		_ = pswpin
		_ = pswpout
	}
}

// getDeviceType 获取设备类型
func (sc *StorageCollector) getDeviceType(device string) string {
	if strings.HasPrefix(device, "nvme") {
		return "nvme"
	}

	// 尝试通过 rotational 判断
	rotationalPath := "/sys/block/" + device + "/queue/rotational"
	if data, err := os.ReadFile(rotationalPath); err == nil {
		if strings.TrimSpace(string(data)) == "0" {
			return "ssd"
		}
		return "hdd"
	}

	return "unknown"
}

// getDeviceSize 获取设备大小
func (sc *StorageCollector) getDeviceSize(device string) uint64 {
	sizePath := "/sys/block/" + device + "/size"
	if data, err := os.ReadFile(sizePath); err == nil {
		sectors, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			return sectors * 512
		}
	}
	return 0
}

// getDeviceTemperature 获取设备温度
func (sc *StorageCollector) getDeviceTemperature(device string) int {
	// 尝试通过 smartctl 获取温度
	cmd := exec.Command("smartctl", "-A", "/dev/"+device, "--json")
	if output, err := cmd.Output(); err == nil {
		// 解析 JSON 获取温度
		if strings.Contains(string(output), "temperature") {
			// 简单解析
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Current") && strings.Contains(line, "Temperature") {
					fields := strings.Fields(line)
					for i, f := range fields {
						if f == "Temperature_Celsius" && i+1 < len(fields) {
							temp, err := strconv.Atoi(fields[i+1])
							if err == nil {
								return temp
							}
						}
					}
				}
			}
		}
	}
	return 0
}

// GetHistory 获取历史数据
func (sc *StorageCollector) GetHistory() []StorageMetrics {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]StorageMetrics, len(sc.history))
	copy(result, sc.history)
	return result
}

// GetAverageIOPS 获取平均 IOPS
func (sc *StorageCollector) GetAverageIOPS() (read, write uint64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if len(sc.history) == 0 {
		return 0, 0
	}

	var totalRead, totalWrite uint64
	for _, m := range sc.history {
		totalRead += m.IOPS.Read
		totalWrite += m.IOPS.Write
	}

	return totalRead / uint64(len(sc.history)),
		totalWrite / uint64(len(sc.history))
}

// GetAverageLatency 获取平均延迟
func (sc *StorageCollector) GetAverageLatency() (read, write float64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if len(sc.history) == 0 {
		return 0, 0
	}

	var totalRead, totalWrite float64
	for _, m := range sc.history {
		totalRead += m.Latency.ReadAvgMs
		totalWrite += m.Latency.WriteAvgMs
	}

	return totalRead / float64(len(sc.history)),
		totalWrite / float64(len(sc.history))
}

// CalculatePercentiles 计算延迟百分位
func (sc *StorageCollector) CalculatePercentiles() (p50, p95, p99 float64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if len(sc.history) < 2 {
		return 0, 0, 0
	}

	// 收集所有延迟值
	var latencies []float64
	for _, m := range sc.history {
		for _, d := range m.Devices {
			latencies = append(latencies, d.ReadLatencyMs, d.WriteLatencyMs)
		}
	}

	if len(latencies) == 0 {
		return 0, 0, 0
	}

	// 排序
	sort.Float64s(latencies)

	p50 = latencies[len(latencies)*50/100]
	p95 = latencies[len(latencies)*95/100]
	p99 = latencies[len(latencies)*99/100]

	return
}
