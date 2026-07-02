package sysresmon

import (
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DashboardData 仪表盘数据聚合.
type DashboardData struct {
	// 当前快照
	Current *ResourceSnapshot `json:"current"`
	// CPU 统计
	CPU CPUStats `json:"cpu"`
	// 内存统计
	Memory MemoryStats `json:"memory"`
	// 磁盘 I/O 统计
	DiskIO DiskIOStats `json:"diskIO"`
	// 网络统计
	Network NetworkStats `json:"network"`
	// GPU 统计
	GPU GPUStats `json:"gpu"`
	// 性能瓶颈分析
	Bottleneck BottleneckAnalysis `json:"bottleneck"`
	// 趋势预测
	Prediction ResourcePrediction `json:"prediction"`
}

// CPUStats CPU 统计数据.
type CPUStats struct {
	// 平均使用率
	AvgUsage float64 `json:"avgUsage"`
	// 最大使用率
	MaxUsage float64 `json:"maxUsage"`
	// 最小使用率
	MinUsage float64 `json:"minUsage"`
	// 使用率标准差
	StdDev float64 `json:"stdDev"`
	// 平均温度
	AvgTemp float64 `json:"avgTemp"`
	// 最高温度
	MaxTemp float64 `json:"maxTemp"`
	// 平均负载
	AvgLoad float64 `json:"avgLoad"`
}

// MemoryStats 内存统计数据.
type MemoryStats struct {
	// 平均使用率
	AvgUsage float64 `json:"avgUsage"`
	// 最大使用率
	MaxUsage float64 `json:"maxUsage"`
	// 最小使用率
	MinUsage float64 `json:"minUsage"`
	// 平均压力
	AvgPressure float64 `json:"avgPressure"`
	// Swap 平均使用率
	AvgSwapUsage float64 `json:"avgSwapUsage"`
}

// DiskIOStats 磁盘 I/O 统计.
type DiskIOStats struct {
	// 平均读取速率（字节/秒）
	AvgReadBytesPerSec uint64 `json:"avgReadBytesPerSec"`
	// 平均写入速率（字节/秒）
	AvgWriteBytesPerSec uint64 `json:"avgWriteBytesPerSec"`
	// 最大读取速率
	MaxReadBytesPerSec uint64 `json:"maxReadBytesPerSec"`
	// 最大写入速率
	MaxWriteBytesPerSec uint64 `json:"maxWriteBytesPerSec"`
	// 平均读 IOPS
	AvgReadIOPS uint64 `json:"avgReadIOPS"`
	// 平均写 IOPS
	AvgWriteIOPS uint64 `json:"avgWriteIOPS"`
}

// NetworkStats 网络统计数据.
type NetworkStats struct {
	// 平均上行速率
	AvgUploadPerSec uint64 `json:"avgUploadPerSec"`
	// 平均下行速率
	AvgDownloadPerSec uint64 `json:"avgDownloadPerSec"`
	// 最大上行速率
	MaxUploadPerSec uint64 `json:"maxUploadPerSec"`
	// 最大下行速率
	MaxDownloadPerSec uint64 `json:"maxDownloadPerSec"`
	// 平均连接数
	AvgConnections int `json:"avgConnections"`
	// 累计错误数
	TotalErrors uint64 `json:"totalErrors"`
}

// GPUStats GPU 统计.
type GPUStats struct {
	// 是否可用
	Available bool `json:"available"`
	// 平均使用率
	AvgUsage float64 `json:"avgUsage"`
	// 最大使用率
	MaxUsage float64 `json:"maxUsage"`
	// 平均温度
	AvgTemp float64 `json:"avgTemp"`
	// 最高温度
	MaxTemp float64 `json:"maxTemp"`
}

// BottleneckAnalysis 性能瓶颈分析.
type BottleneckAnalysis struct {
	// 瓶颈类型：cpu/memory/disk/network/none
	Type string `json:"type"`
	// 瓶颈严重程度（0-100）
	Severity int `json:"severity"`
	// 描述
	Description string `json:"description"`
	// 建议
	Suggestions []string `json:"suggestions"`
}

// ResourcePrediction 资源使用预测.
type ResourcePrediction struct {
	// 预测时间
	PredictedAt time.Time `json:"predictedAt"`
	// 预测时长
	Duration string `json:"duration"`
	// CPU 预测
	CPU PredictionItem `json:"cpu"`
	// 内存预测
	Memory PredictionItem `json:"memory"`
	// 磁盘预测
	Disk PredictionItem `json:"disk"`
	// 网络预测
	Network PredictionItem `json:"network"`
}

// PredictionItem 预测项.
type PredictionItem struct {
	// 预测值
	Value float64 `json:"value"`
	// 置信度（0-100）
	Confidence float64 `json:"confidence"`
	// 趋势：rising/stable/falling
	Trend string `json:"trend"`
}

// TimeRange 时间范围枚举.
type TimeRange string

const (
	Range1H  TimeRange = "1h"
	Range6H  TimeRange = "6h"
	Range24H TimeRange = "24h"
	Range7D  TimeRange = "7d"
)

// Dashboard 仪表盘.
type Dashboard struct {
	monitor *ResourceMonitor
}

// NewDashboard 创建仪表盘.
func NewDashboard(monitor *ResourceMonitor) *Dashboard {
	return &Dashboard{
		monitor: monitor,
	}
}

// GetDashboardData 获取仪表盘数据.
func (d *Dashboard) GetDashboardData(rangeType TimeRange) *DashboardData {
	// 获取历史数据
	duration := d.parseRange(rangeType)
	history := d.monitor.GetHistory(duration)

	data := &DashboardData{
		Current: d.monitor.GetLatest(),
	}

	// 计算各项统计
	if len(history) > 0 {
		data.CPU = d.calcCPUStats(history)
		data.Memory = d.calcMemoryStats(history)
		data.DiskIO = d.calcDiskIOStats(history)
		data.Network = d.calcNetworkStats(history)
		data.GPU = d.calcGPUStats(history)
		data.Bottleneck = d.analyzeBottleneck(history)
		data.Prediction = d.predict(history, rangeType)
	}

	return data
}

// parseRange 解析时间范围.
func (d *Dashboard) parseRange(rangeType TimeRange) time.Duration {
	switch rangeType {
	case Range1H:
		return 1 * time.Hour
	case Range6H:
		return 6 * time.Hour
	case Range24H:
		return 24 * time.Hour
	case Range7D:
		return 7 * 24 * time.Hour
	default:
		return 1 * time.Hour
	}
}

// calcCPUStats 计算 CPU 统计.
func (d *Dashboard) calcCPUStats(history []ResourceSnapshot) CPUStats {
	stats := CPUStats{}
	if len(history) == 0 {
		return stats
	}

	var usages, temps []float64
	for _, snap := range history {
		usages = append(usages, snap.CPU.UsagePercent)
		if snap.CPU.Temperature > 0 {
			temps = append(temps, snap.CPU.Temperature)
		}
	}

	stats.AvgUsage = calcMean(usages)
	stats.MaxUsage = calcMax(usages)
	stats.MinUsage = calcMin(usages)
	stats.StdDev = calcStdDev(usages)

	if len(temps) > 0 {
		stats.AvgTemp = calcMean(temps)
		stats.MaxTemp = calcMax(temps)
	}

	// 计算平均负载
	var loads []float64
	for _, snap := range history {
		loads = append(loads, snap.CPU.LoadAvg1)
	}
	stats.AvgLoad = calcMean(loads)

	return stats
}

// calcMemoryStats 计算内存统计.
func (d *Dashboard) calcMemoryStats(history []ResourceSnapshot) MemoryStats {
	stats := MemoryStats{}
	if len(history) == 0 {
		return stats
	}

	var usages, pressures, swapUsages []float64
	for _, snap := range history {
		usages = append(usages, snap.Memory.UsagePercent)
		pressures = append(pressures, snap.Memory.Pressure)
		swapUsages = append(swapUsages, snap.Memory.SwapUsagePercent)
	}

	stats.AvgUsage = calcMean(usages)
	stats.MaxUsage = calcMax(usages)
	stats.MinUsage = calcMin(usages)
	stats.AvgPressure = calcMean(pressures)
	stats.AvgSwapUsage = calcMean(swapUsages)

	return stats
}

// calcDiskIOStats 计算磁盘 I/O 统计.
func (d *Dashboard) calcDiskIOStats(history []ResourceSnapshot) DiskIOStats {
	stats := DiskIOStats{}
	if len(history) == 0 {
		return stats
	}

	var readBytes, writeBytes []float64
	var readIOPS, writeIOPS []float64

	for _, snap := range history {
		readBytes = append(readBytes, float64(snap.DiskIO.ReadBytesPerSec))
		writeBytes = append(writeBytes, float64(snap.DiskIO.WriteBytesPerSec))
		readIOPS = append(readIOPS, float64(snap.DiskIO.ReadIOPS))
		writeIOPS = append(writeIOPS, float64(snap.DiskIO.WriteIOPS))
	}

	stats.AvgReadBytesPerSec = uint64(calcMean(readBytes))
	stats.AvgWriteBytesPerSec = uint64(calcMean(writeBytes))
	stats.MaxReadBytesPerSec = uint64(calcMax(readBytes))
	stats.MaxWriteBytesPerSec = uint64(calcMax(writeBytes))
	stats.AvgReadIOPS = uint64(calcMean(readIOPS))
	stats.AvgWriteIOPS = uint64(calcMean(writeIOPS))

	return stats
}

// calcNetworkStats 计算网络统计.
func (d *Dashboard) calcNetworkStats(history []ResourceSnapshot) NetworkStats {
	stats := NetworkStats{}
	if len(history) == 0 {
		return stats
	}

	var upBytes, downBytes []float64
	var conns []float64
	var totalErrors uint64

	for _, snap := range history {
		upBytes = append(upBytes, float64(snap.Network.UploadPerSec))
		downBytes = append(downBytes, float64(snap.Network.DownloadPerSec))
		conns = append(conns, float64(snap.Network.Connections))
		totalErrors += snap.Network.Errors
	}

	stats.AvgUploadPerSec = uint64(calcMean(upBytes))
	stats.AvgDownloadPerSec = uint64(calcMean(downBytes))
	stats.MaxUploadPerSec = uint64(calcMax(upBytes))
	stats.MaxDownloadPerSec = uint64(calcMax(downBytes))
	stats.AvgConnections = int(calcMean(conns))
	stats.TotalErrors = totalErrors

	return stats
}

// calcGPUStats 计算 GPU 统计.
func (d *Dashboard) calcGPUStats(history []ResourceSnapshot) GPUStats {
	stats := GPUStats{}
	if len(history) == 0 {
		return stats
	}

	var usages, temps []float64
	for _, snap := range history {
		if snap.GPU.Available {
			stats.Available = true
			usages = append(usages, snap.GPU.UsagePercent)
			if snap.GPU.Temperature > 0 {
				temps = append(temps, snap.GPU.Temperature)
			}
		}
	}

	if len(usages) > 0 {
		stats.AvgUsage = calcMean(usages)
		stats.MaxUsage = calcMax(usages)
	}

	if len(temps) > 0 {
		stats.AvgTemp = calcMean(temps)
		stats.MaxTemp = calcMax(temps)
	}

	return stats
}

// analyzeBottleneck 分析性能瓶颈.
func (d *Dashboard) analyzeBottleneck(history []ResourceSnapshot) BottleneckAnalysis {
	if len(history) < 10 {
		return BottleneckAnalysis{
			Type:        "none",
			Severity:    0,
			Description: "数据不足，无法分析",
		}
	}

	// 计算各项资源平均使用率
	var cpuAvg, memAvg, diskAvg, netAvg float64

	for _, snap := range history {
		cpuAvg += snap.CPU.UsagePercent
		memAvg += snap.Memory.UsagePercent
		diskAvg += float64(snap.DiskIO.ReadBytesPerSec+snap.DiskIO.WriteBytesPerSec) / 1e9
		netAvg += float64(snap.Network.UploadPerSec+snap.Network.DownloadPerSec) / 1e9
	}

	n := float64(len(history))
	cpuAvg /= n
	memAvg /= n
	diskAvg /= n
	netAvg /= n

	// 确定瓶颈类型
	analysis := BottleneckAnalysis{}

	// 阈值判定
	if cpuAvg > 80 {
		analysis.Type = "cpu"
		analysis.Severity = int(math.Min(100, cpuAvg))
		analysis.Description = "CPU 使用率过高"
		analysis.Suggestions = []string{
			"检查 CPU 密集型进程",
			"考虑升级 CPU 或增加核心数",
			"优化代码减少 CPU 占用",
		}
	} else if memAvg > 85 {
		analysis.Type = "memory"
		analysis.Severity = int(math.Min(100, memAvg))
		analysis.Description = "内存使用率过高"
		analysis.Suggestions = []string{
			"检查内存泄漏",
			"增加物理内存",
			"优化应用内存占用",
		}
	} else if diskAvg > 0.5 { // 超过 500MB/s
		analysis.Type = "disk"
		analysis.Severity = int(math.Min(100, diskAvg*20))
		analysis.Description = "磁盘 I/O 压力大"
		analysis.Suggestions = []string{
			"升级到 SSD",
			"优化磁盘读写策略",
			"增加缓存",
		}
	} else if netAvg > 0.1 { // 超过 100MB/s
		analysis.Type = "network"
		analysis.Severity = int(math.Min(100, netAvg*100))
		analysis.Description = "网络带宽使用率高"
		analysis.Suggestions = []string{
			"升级网络带宽",
			"优化网络传输",
			"使用流量控制",
		}
	} else {
		analysis.Type = "none"
		analysis.Severity = 0
		analysis.Description = "系统运行正常，无明显瓶颈"
	}

	return analysis
}

// predict 资源使用预测（基于移动平均）.
func (d *Dashboard) predict(history []ResourceSnapshot, rangeType TimeRange) ResourcePrediction {
	prediction := ResourcePrediction{
		PredictedAt: time.Now(),
		Duration:    d.predictionDuration(rangeType),
	}

	if len(history) < 20 {
		return prediction
	}

	// 使用最近的数据点
	windowSize := 30
	if windowSize > len(history) {
		windowSize = len(history)
	}
	recent := history[len(history)-windowSize:]

	// 计算各项资源的移动平均和趋势
	prediction.CPU = d.predictItem(recent, func(s ResourceSnapshot) float64 {
		return s.CPU.UsagePercent
	})
	prediction.Memory = d.predictItem(recent, func(s ResourceSnapshot) float64 {
		return s.Memory.UsagePercent
	})
	prediction.Disk = d.predictItem(recent, func(s ResourceSnapshot) float64 {
		return float64(s.DiskIO.ReadBytesPerSec+s.DiskIO.WriteBytesPerSec) / 1e9
	})
	prediction.Network = d.predictItem(recent, func(s ResourceSnapshot) float64 {
		return float64(s.Network.UploadPerSec+s.Network.DownloadPerSec) / 1e9
	})

	return prediction
}

// predictionDuration 预测时长.
func (d *Dashboard) predictionDuration(rangeType TimeRange) string {
	switch rangeType {
	case Range1H:
		return "1小时"
	case Range6H:
		return "6小时"
	case Range24H:
		return "24小时"
	case Range7D:
		return "7天"
	default:
		return "1小时"
	}
}

// predictItem 预测单项资源.
func (d *Dashboard) predictItem(history []ResourceSnapshot, extractor func(ResourceSnapshot) float64) PredictionItem {
	if len(history) < 5 {
		return PredictionItem{}
	}

	// 提取数据点
	points := make([]float64, len(history))
	for i, snap := range history {
		points[i] = extractor(snap)
	}

	// 计算移动平均
	ma := calcMovingAverage(points, 5)

	// 线性回归预测趋势
	slope := calcSlope(ma)

	// 预测值（基于趋势外推）
	current := points[len(points)-1]
	predicted := current + slope*float64(len(points))*0.1

	// 限制在合理范围
	predicted = math.Max(0, math.Min(100, predicted))

	// 计算置信度
	confidence := calcConfidence(points, ma)

	// 判断趋势
	trend := "stable"
	if slope > 0.5 {
		trend = "rising"
	} else if slope < -0.5 {
		trend = "falling"
	}

	return PredictionItem{
		Value:      math.Round(predicted*100) / 100,
		Confidence: confidence,
		Trend:      trend,
	}
}

// calcMean 计算平均值.
func calcMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return math.Round(sum/float64(len(data))*100) / 100
}

// calcMax 计算最大值.
func calcMax(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return math.Round(max*100) / 100
}

// calcMin 计算最小值.
func calcMin(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
	}
	return math.Round(min*100) / 100
}

// calcStdDev 计算标准差.
func calcStdDev(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	mean := calcMean(data)
	sumSquares := 0.0
	for _, v := range data {
		diff := v - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(data))
	return math.Round(math.Sqrt(variance)*100) / 100
}

// calcMovingAverage 计算移动平均.
func calcMovingAverage(data []float64, window int) []float64 {
	if len(data) < window {
		return data
	}

	result := make([]float64, len(data)-window+1)
	for i := 0; i <= len(data)-window; i++ {
		sum := 0.0
		for j := i; j < i+window; j++ {
			sum += data[j]
		}
		result[i] = sum / float64(window)
	}
	return result
}

// calcSlope 计算线性回归斜率.
func calcSlope(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}

	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}

// calcConfidence 计算预测置信度.
func calcConfidence(actual, smoothed []float64) float64 {
	if len(actual) < 2 || len(smoothed) < 2 {
		return 50
	}

	// 计算 R²
	var ssRes, ssTot float64
	mean := calcMean(actual)

	for i := 0; i < len(smoothed); i++ {
		diff := actual[i] - smoothed[i]
		ssRes += diff * diff
		diffMean := actual[i] - mean
		ssTot += diffMean * diffMean
	}

	if ssTot == 0 {
		return 50
	}

	r2 := 1 - ssRes/ssTot
	confidence := math.Round(r2 * 100)

	// 限制在 0-100
	if confidence < 0 {
		confidence = 0
	} else if confidence > 100 {
		confidence = 100
	}

	return confidence
}

// GetTrendData 获取趋势数据（用于绘图）.
func (d *Dashboard) GetTrendData(rangeType TimeRange) []TrendPoint {
	duration := d.parseRange(rangeType)
	history := d.monitor.GetHistory(duration)

	if len(history) == 0 {
		return nil
	}

	// 降采样，最多保留 200 个点
	maxPoints := 200
	if len(history) <= maxPoints {
		result := make([]TrendPoint, len(history))
		for i, snap := range history {
			result[i] = TrendPoint{
				Timestamp:   snap.Timestamp,
				CPU:         snap.CPU.UsagePercent,
				Memory:      snap.Memory.UsagePercent,
				DiskRead:    float64(snap.DiskIO.ReadBytesPerSec) / 1e9,
				DiskWrite:   float64(snap.DiskIO.WriteBytesPerSec) / 1e9,
				NetUpload:   float64(snap.Network.UploadPerSec) / 1e9,
				NetDownload: float64(snap.Network.DownloadPerSec) / 1e9,
			}
		}
		return result
	}

	// 均匀降采样
	step := len(history) / maxPoints
	result := make([]TrendPoint, 0, maxPoints)

	for i := 0; i < len(history); i += step {
		snap := history[i]
		result = append(result, TrendPoint{
			Timestamp:   snap.Timestamp,
			CPU:         snap.CPU.UsagePercent,
			Memory:      snap.Memory.UsagePercent,
			DiskRead:    float64(snap.DiskIO.ReadBytesPerSec) / 1e9,
			DiskWrite:   float64(snap.DiskIO.WriteBytesPerSec) / 1e9,
			NetUpload:   float64(snap.Network.UploadPerSec) / 1e9,
			NetDownload: float64(snap.Network.DownloadPerSec) / 1e9,
		})
	}

	return result
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	CPU         float64   `json:"cpu"`
	Memory      float64   `json:"memory"`
	DiskRead    float64   `json:"diskRead"`
	DiskWrite   float64   `json:"diskWrite"`
	NetUpload   float64   `json:"netUpload"`
	NetDownload float64   `json:"netDownload"`
}

// GetTopProcesses 获取资源占用最高的进程。
func (d *Dashboard) GetTopProcesses(limit int) []ProcessInfo {
	if limit <= 0 {
		limit = 10
	}
	out, err := exec.Command("ps", "-eo", "pid=,comm=,pcpu=,pmem=,stat=", "--sort=-pcpu").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := make([]ProcessInfo, 0, limit)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, _ := strconv.ParseInt(fields[0], 10, 32)
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 32)
		result = append(result, ProcessInfo{PID: int32(pid), Name: fields[1], CPUPercent: cpu, MemPercent: float32(mem), Status: fields[4]})
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ProcessInfo 进程信息.
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemPercent float32 `json:"memPercent"`
	Status     string  `json:"status"`
}

// percentile 计算百分位数.
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
