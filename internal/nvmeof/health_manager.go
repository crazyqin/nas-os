// Package nvmeof - NVMe Health Monitoring Manager
// 温度监控、寿命预测、性能基准测试管理器
// 参考TrueNAS 25.10 NVMe Optimizations
package nvmeof

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthManager NVMe健康监控管理器
type HealthManager struct {
	mu sync.RWMutex

	// 温度监控
	tempConfig     TemperatureConfig
	tempHistory    map[string][]TemperatureReading // device -> readings
	tempAlerts     []TemperatureAlert
	deviceStatuses map[string]*DeviceTemperatureStatus // device -> status

	// 寿命预测
	lifeConfig      LifePredictionConfig
	lifePredictions map[string]*DeviceLifePrediction // device -> prediction
	writePatterns   map[string]*WritePattern         // device -> pattern

	// 性能基准测试
	benchResults map[string]*BenchmarkResult // id -> result
	benchRunning map[string]bool             // id -> running
	benchWg      sync.WaitGroup              // 等待所有benchmark完成

	// 依赖
	nvmeManager *Manager
	logger      *zap.Logger
	configPath  string
}

// NewHealthManager 创建健康监控管理器
func NewHealthManager(nvmeManager *Manager, configPath string, logger *zap.Logger) *HealthManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HealthManager{
		tempConfig:      DefaultTemperatureConfig(),
		tempHistory:     make(map[string][]TemperatureReading),
		tempAlerts:      make([]TemperatureAlert, 0),
		deviceStatuses:  make(map[string]*DeviceTemperatureStatus),
		lifeConfig:      DefaultLifePredictionConfig(),
		lifePredictions: make(map[string]*DeviceLifePrediction),
		writePatterns:   make(map[string]*WritePattern),
		benchResults:    make(map[string]*BenchmarkResult),
		benchRunning:    make(map[string]bool),
		nvmeManager:     nvmeManager,
		logger:          logger,
		configPath:      configPath,
	}
}

// ============================================================
// 温度监控
// ============================================================

// RecordTemperature 记录设备温度读数
func (hm *HealthManager) RecordTemperature(ctx context.Context, device string, subsystemNQN string, temp float64) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	reading := TemperatureReading{
		Device:       device,
		SubsystemNQN: subsystemNQN,
		Temperature:  temp,
		Timestamp:    time.Now(),
	}

	// 添加到历史
	hm.tempHistory[device] = append(hm.tempHistory[device], reading)

	// 裁剪历史到最大长度
	if len(hm.tempHistory[device]) > hm.tempConfig.MaxHistoryLen {
		hm.tempHistory[device] = hm.tempHistory[device][len(hm.tempHistory[device])-hm.tempConfig.MaxHistoryLen:]
	}

	// 更新设备状态
	status, exists := hm.deviceStatuses[device]
	if !exists {
		status = &DeviceTemperatureStatus{
			Device:       device,
			SubsystemNQN: subsystemNQN,
			MinTemp:      temp,
			MaxTemp:      temp,
		}
		hm.deviceStatuses[device] = status
	}

	status.CurrentTemp = temp
	status.LastUpdated = reading.Timestamp
	if temp < status.MinTemp {
		status.MinTemp = temp
	}
	if temp > status.MaxTemp {
		status.MaxTemp = temp
	}

	// 计算平均温度
	var total float64
	for _, r := range hm.tempHistory[device] {
		total += r.Temperature
	}
	status.AvgTemp = total / float64(len(hm.tempHistory[device]))

	// 检查告警
	if temp >= hm.tempConfig.CriticalThreshold {
		status.Status = "critical"
		alert := TemperatureAlert{
			Device:       device,
			SubsystemNQN: subsystemNQN,
			Temperature:  temp,
			Threshold:    hm.tempConfig.CriticalThreshold,
			Level:        "critical",
			Timestamp:    reading.Timestamp,
		}
		hm.tempAlerts = append(hm.tempAlerts, alert)
		status.AlertCount++
		status.LastAlert = &alert
		hm.logger.Warn("NVMe temperature critical",
			zap.String("device", device), zap.Float64("temp", temp))
	} else if temp >= hm.tempConfig.WarningThreshold {
		if status.Status != "critical" {
			status.Status = "warning"
		}
		alert := TemperatureAlert{
			Device:       device,
			SubsystemNQN: subsystemNQN,
			Temperature:  temp,
			Threshold:    hm.tempConfig.WarningThreshold,
			Level:        "warning",
			Timestamp:    reading.Timestamp,
		}
		hm.tempAlerts = append(hm.tempAlerts, alert)
		status.AlertCount++
		status.LastAlert = &alert
		hm.logger.Warn("NVMe temperature warning",
			zap.String("device", device), zap.Float64("temp", temp))
	} else {
		if status.Status != "critical" {
			status.Status = "normal"
		}
	}

	return nil
}

// GetDeviceTemperatureStatus 获取设备温度状态
func (hm *HealthManager) GetDeviceTemperatureStatus(device string) (*DeviceTemperatureStatus, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status, exists := hm.deviceStatuses[device]
	if !exists {
		return nil, fmt.Errorf("device %s not found", device)
	}
	return status, nil
}

// GetTemperatureHistory 获取设备温度历史
func (hm *HealthManager) GetTemperatureHistory(device string, limit int) []TemperatureReading {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	history, exists := hm.tempHistory[device]
	if !exists {
		return []TemperatureReading{}
	}

	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	start := len(history) - limit
	result := make([]TemperatureReading, limit)
	copy(result, history[start:])
	return result
}

// GetRecentAlerts 获取最近的温度告警
func (hm *HealthManager) GetRecentAlerts(limit int) []TemperatureAlert {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if limit <= 0 || limit > len(hm.tempAlerts) {
		limit = len(hm.tempAlerts)
	}

	start := len(hm.tempAlerts) - limit
	result := make([]TemperatureAlert, limit)
	copy(result, hm.tempAlerts[start:])
	return result
}

// GetAllDeviceStatuses 获取所有设备温度状态
func (hm *HealthManager) GetAllDeviceStatuses() []*DeviceTemperatureStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	statuses := make([]*DeviceTemperatureStatus, 0, len(hm.deviceStatuses))
	for _, s := range hm.deviceStatuses {
		statuses = append(statuses, s)
	}
	return statuses
}

// UpdateTemperatureConfig 更新温度监控配置
func (hm *HealthManager) UpdateTemperatureConfig(cfg TemperatureConfig) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.tempConfig = cfg
}

// GetTemperatureConfig 获取温度监控配置
func (hm *HealthManager) GetTemperatureConfig() TemperatureConfig {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.tempConfig
}

// ============================================================
// 寿命预测
// ============================================================

// PredictDeviceLife 预测设备寿命
func (hm *HealthManager) PredictDeviceLife(ctx context.Context, device string, subsystemNQN string, model string, serial string,
	totalWriteCapacityTB float64, totalWrittenTB float64, percentageUsed int, availableSpare int,
	powerOnHours uint64, unsafeShutdowns uint64, mediaErrors uint64) (*DeviceLifePrediction, error) {

	hm.mu.Lock()
	defer hm.mu.Unlock()

	prediction := &DeviceLifePrediction{
		Device:               device,
		SubsystemNQN:         subsystemNQN,
		Model:                model,
		Serial:               serial,
		TotalWriteCapacityTB: totalWriteCapacityTB,
		TotalWrittenTB:       totalWrittenTB,
		PercentageUsed:       percentageUsed,
		AvailableSpare:       availableSpare,
		PowerOnHours:         powerOnHours,
		UnsafeShutdowns:      unsafeShutdowns,
		MediaErrors:          mediaErrors,
		PredictedAt:          time.Now(),
	}

	// 计算每日写入率
	var dailyWriteRate float64
	if pattern, exists := hm.writePatterns[device]; exists && pattern.DailyWriteAvgGB > 0 {
		dailyWriteRate = pattern.DailyWriteAvgGB
		prediction.WriteAmplification = pattern.WriteAmplification
	} else if powerOnHours > 0 {
		// 从SMART数据估算
		dailyWriteRate = totalWrittenTB * 1024 / float64(powerOnHours/24)
		prediction.WriteAmplification = 1.0
	}

	prediction.DailyWriteRateGB = dailyWriteRate

	// 温度退化因子
	tempDegradation := hm.calculateTempDegradation(device)
	prediction.TempDegradation = tempDegradation

	// 写入退化因子
	writeDegradation := hm.calculateWriteDegradation(totalWrittenTB, totalWriteCapacityTB)
	prediction.WriteDegradation = writeDegradation

	// 剩余寿命计算
	// 基于TBW容量的剩余百分比
	var tbwRemaining float64
	if totalWriteCapacityTB > 0 {
		tbwRemaining = (totalWriteCapacityTB - totalWrittenTB) / totalWriteCapacityTB * 100
	}

	// 综合因子调整
	combinedDegradation := 1.0 - (tempDegradation+writeDegradation)/2
	if combinedDegradation < 0.01 {
		combinedDegradation = 0.01
	}

	remainingPercent := tbwRemaining * combinedDegradation
	if remainingPercent > 100 {
		remainingPercent = 100
	}
	if remainingPercent < 0 {
		remainingPercent = 0
	}
	prediction.RemainingLifePercent = remainingPercent

	// 估算剩余天数
	if dailyWriteRate > 0 && totalWriteCapacityTB > 0 {
		remainingWriteGB := (totalWriteCapacityTB - totalWrittenTB) * 1024
		if remainingWriteGB > 0 {
			estDays := remainingWriteGB / dailyWriteRate
			prediction.EstimatedDaysLeft = int(estDays)
			endDate := time.Now().AddDate(0, 0, int(estDays))
			prediction.EstimatedEndDate = endDate
		}
	}

	// 置信度评估
	prediction.ConfidenceLevel = hm.assessConfidence(
		powerOnHours, len(hm.tempHistory[device]), hm.writePatterns[device])

	// 磨损等级
	prediction.WearLevel = hm.classifyWearLevel(remainingPercent)

	hm.lifePredictions[device] = prediction

	hm.logger.Info("NVMe life prediction updated",
		zap.String("device", device),
		zap.Float64("remaining", remainingPercent),
		zap.Int("days_left", prediction.EstimatedDaysLeft),
		zap.String("confidence", prediction.ConfidenceLevel))

	return prediction, nil
}

// calculateTempDegradation 计算温度对寿命的影响
func (hm *HealthManager) calculateTempDegradation(device string) float64 {
	history, exists := hm.tempHistory[device]
	if !exists || len(history) == 0 {
		return 0.0
	}

	warningThreshold := hm.tempConfig.WarningThreshold
	totalDegradation := 0.0
	count := 0.0

	for _, reading := range history {
		if reading.Temperature > warningThreshold {
			excess := reading.Temperature - warningThreshold
			totalDegradation += excess * hm.lifeConfig.TempDegradationRate
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	avgDegradation := totalDegradation / float64(len(history))
	if avgDegradation > 1.0 {
		avgDegradation = 1.0
	}
	return avgDegradation
}

// calculateWriteDegradation 计算写入对寿命的影响
func (hm *HealthManager) calculateWriteDegradation(totalWrittenTB, totalCapacityTB float64) float64 {
	if totalCapacityTB <= 0 {
		return 0.0
	}

	tbwPercent := totalWrittenTB / totalCapacityTB * 100
	degradation := tbwPercent / 100.0 * hm.lifeConfig.WriteDegradationRate
	if degradation > 1.0 {
		degradation = 1.0
	}
	return degradation
}

// assessConfidence 评估预测置信度
func (hm *HealthManager) assessConfidence(powerOnHours uint64, tempSamples int, pattern *WritePattern) string {
	score := 0

	// 基于运行时间
	if powerOnHours > 8760 { // > 1年
		score += 3
	} else if powerOnHours > 4380 { // > 6个月
		score += 2
	} else if powerOnHours > 720 { // > 1个月
		score += 1
	}

	// 基于温度采样数
	if tempSamples > 100 {
		score += 2
	} else if tempSamples > 10 {
		score += 1
	}

	// 基于写入模式数据
	if pattern != nil && pattern.SamplePeriodDays > 30 {
		score += 2
	} else if pattern != nil {
		score += 1
	}

	if score >= 5 {
		return "high"
	} else if score >= 3 {
		return "medium"
	}
	return "low"
}

// classifyWearLevel 分类磨损等级
func (hm *HealthManager) classifyWearLevel(remainingPercent float64) string {
	switch {
	case remainingPercent > 70:
		return "low"
	case remainingPercent > 40:
		return "medium"
	case remainingPercent > 15:
		return "high"
	default:
		return "critical"
	}
}

// UpdateWritePattern 更新设备写入模式
func (hm *HealthManager) UpdateWritePattern(device string, subsystemNQN string, totalWriteTB, totalReadTB float64,
	dailyWriteAvgGB, weeklyWriteAvgGB, peakWriteRateGBps, writeAmplification float64, sampleDays int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.writePatterns[device] = &WritePattern{
		Device:             device,
		SubsystemNQN:       subsystemNQN,
		DailyWriteAvgGB:    dailyWriteAvgGB,
		WeeklyWriteAvgGB:   weeklyWriteAvgGB,
		PeakWriteRateGBps:  peakWriteRateGBps,
		WriteAmplification: writeAmplification,
		TotalWriteTB:       totalWriteTB,
		TotalReadTB:        totalReadTB,
		SamplePeriodDays:   sampleDays,
		UpdatedAt:          time.Now(),
	}
}

// GetLifePrediction 获取设备寿命预测
func (hm *HealthManager) GetLifePrediction(device string) (*DeviceLifePrediction, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	pred, exists := hm.lifePredictions[device]
	if !exists {
		return nil, fmt.Errorf("no life prediction for device %s", device)
	}
	return pred, nil
}

// GetAllLifePredictions 获取所有设备寿命预测
func (hm *HealthManager) GetAllLifePredictions() []*DeviceLifePrediction {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	predictions := make([]*DeviceLifePrediction, 0, len(hm.lifePredictions))
	for _, p := range hm.lifePredictions {
		predictions = append(predictions, p)
	}
	return predictions
}

// ============================================================
// 性能基准测试
// ============================================================

// StartBenchmark 启动性能基准测试
func (hm *HealthManager) StartBenchmark(ctx context.Context, cfg BenchmarkConfig) (*BenchmarkResult, error) {
	// 参数校验和默认值
	if cfg.BlockSizeKB <= 0 {
		cfg.BlockSizeKB = 64
	}
	if cfg.FileSizeMB <= 0 {
		cfg.FileSizeMB = 256
	}
	if cfg.FileSizeMB > 4096 {
		return nil, fmt.Errorf("file size %d MB exceeds maximum 4096 MB", cfg.FileSizeMB)
	}
	if cfg.DurationSec <= 0 {
		cfg.DurationSec = 30
	}
	if cfg.NumThreads <= 0 {
		cfg.NumThreads = 1
	}
	if len(cfg.TestTypes) == 0 {
		cfg.TestTypes = []string{"seq_read", "seq_write", "rand_read", "rand_write"}
	}

	resultID := fmt.Sprintf("bench-%d", time.Now().UnixNano())
	result := &BenchmarkResult{
		ID:        resultID,
		Config:    cfg,
		Status:    "pending",
		StartedAt: time.Now(),
	}

	hm.mu.Lock()
	if hm.benchRunning[cfg.DevicePath] {
		hm.mu.Unlock()
		return nil, fmt.Errorf("benchmark already running for device %s", cfg.DevicePath)
	}
	hm.benchRunning[cfg.DevicePath] = true
	hm.benchResults[resultID] = result
	hm.mu.Unlock()

	// 异步执行基准测试
	hm.benchWg.Add(1)
	go hm.runBenchmark(result)

	return result, nil
}

// runBenchmark 执行基准测试
func (hm *HealthManager) runBenchmark(result *BenchmarkResult) {
	defer hm.benchWg.Done()
	result.Status = "running"
	cfg := result.Config

	metrics := &BenchmarkMetrics{}
	testFile := filepath.Join(cfg.DevicePath, fmt.Sprintf(".nvme-bench-%d.tmp", time.Now().UnixNano()))
	// 确保临时文件在任何退出路径都会被清理
	defer os.Remove(testFile)

	var err error

	// 顺序写测试
	if contains(cfg.TestTypes, "seq_write") {
		metrics.SeqWriteMBps, err = hm.benchSequentialWrite(testFile, cfg.FileSizeMB, cfg.BlockSizeKB)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("sequential write failed: %v", err)
			hm.finishBenchmark(result)
			return
		}
	}

	// 顺序读测试
	if contains(cfg.TestTypes, "seq_read") {
		if metrics.SeqWriteMBps > 0 {
			metrics.SeqReadMBps, err = hm.benchSequentialRead(testFile, cfg.BlockSizeKB)
		} else {
			metrics.SeqReadMBps, err = hm.benchSequentialRead(testFile, cfg.BlockSizeKB)
		}
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("sequential read failed: %v", err)
			hm.finishBenchmark(result)
			return
		}
	}

	// 随机IO测试
	if contains(cfg.TestTypes, "rand_read") || contains(cfg.TestTypes, "rand_write") {
		readIOPS, writeIOPS, latAvg, latP50, latP95, latP99 := hm.benchRandomIO(testFile, cfg.BlockSizeKB*1024)
		metrics.RandomReadIOPS = readIOPS
		metrics.RandomWriteIOPS = writeIOPS
		metrics.LatencyAvgMs = latAvg
		metrics.LatencyP50Ms = latP50
		metrics.LatencyP95Ms = latP95
		metrics.LatencyP99Ms = latP99
	}

	// 计算综合评分
	metrics.OverallScore = hm.calculateOverallScore(metrics)
	metrics.TotalThroughputMBps = (metrics.SeqReadMBps + metrics.SeqWriteMBps) / 2

	now := time.Now()
	result.Results = metrics
	result.CompletedAt = &now
	result.Status = "completed"
	result.Duration = now.Sub(result.StartedAt)

	hm.finishBenchmark(result)

	hm.logger.Info("NVMe benchmark completed",
		zap.String("id", result.ID),
		zap.String("device", cfg.DevicePath),
		zap.Float64("score", metrics.OverallScore))
}

// finishBenchmark 完成基准测试
func (hm *HealthManager) finishBenchmark(result *BenchmarkResult) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.benchRunning, result.Config.DevicePath)
}

// benchSequentialWrite 顺序写基准测试
func (hm *HealthManager) benchSequentialWrite(path string, sizeMB, blockKB int) (float64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	rand.Read(buf)
	totalBytes := int64(sizeMB) * 1024 * 1024

	start := time.Now()
	written := int64(0)
	for written < totalBytes {
		toWrite := blockSize
		if int64(toWrite) > totalBytes-written {
			toWrite = int(totalBytes - written)
		}
		n, err := f.Write(buf[:toWrite])
		if err != nil {
			return 0, fmt.Errorf("write: %w", err)
		}
		written += int64(n)
	}
	f.Sync()
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0, nil
	}
	return float64(written) / 1024 / 1024 / elapsed, nil
}

// benchSequentialRead 顺序读基准测试
func (hm *HealthManager) benchSequentialRead(path string, blockKB int) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	start := time.Now()
	totalRead := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			totalRead += int64(n)
		}
		if err != nil {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0, nil
	}
	return float64(totalRead) / 1024 / 1024 / elapsed, nil
}

// benchRandomIO 随机IO基准测试
func (hm *HealthManager) benchRandomIO(path string, blockSize int) (readIOPS, writeIOPS, latAvgMs, latP50Ms, latP95Ms, latP99Ms float64) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, 0, 0, 0
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return 0, 0, 0, 0, 0, 0
	}
	fileSize := info.Size()

	if blockSize <= 0 {
		blockSize = 4096
	}
	buf := make([]byte, blockSize)
	rand.Read(buf)
	ops := 1000
	latencies := make([]time.Duration, 0, ops*2)

	// 随机写
	writeStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		f.WriteAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	f.Sync()
	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed > 0 {
		writeIOPS = float64(ops) / writeElapsed
	}

	// 随机读
	readStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		f.ReadAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	readElapsed := time.Since(readStart).Seconds()
	if readElapsed > 0 {
		readIOPS = float64(ops) / readElapsed
	}

	// 延迟统计
	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		avgNs := total.Nanoseconds() / int64(len(latencies))
		latAvgMs = float64(avgNs) / 1e6

		// 排序计算百分位
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		p50Idx := int(float64(len(sorted)) * 0.50)
		p95Idx := int(float64(len(sorted)) * 0.95)
		p99Idx := int(float64(len(sorted)) * 0.99)

		if p50Idx >= len(sorted) {
			p50Idx = len(sorted) - 1
		}
		if p95Idx >= len(sorted) {
			p95Idx = len(sorted) - 1
		}
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}

		latP50Ms = float64(sorted[p50Idx].Nanoseconds()) / 1e6
		latP95Ms = float64(sorted[p95Idx].Nanoseconds()) / 1e6
		latP99Ms = float64(sorted[p99Idx].Nanoseconds()) / 1e6
	}

	return
}

// calculateOverallScore 计算综合评分 (0-100)
func (hm *HealthManager) calculateOverallScore(m *BenchmarkMetrics) float64 {
	score := 0.0
	count := 0.0

	// 顺序读评分 (参考: 优秀>3000MB/s, 良好>1000MB/s)
	if m.SeqReadMBps > 0 {
		readScore := math.Min(100, m.SeqReadMBps/30)
		score += readScore
		count++
	}

	// 顺序写评分
	if m.SeqWriteMBps > 0 {
		writeScore := math.Min(100, m.SeqWriteMBps/25)
		score += writeScore
		count++
	}

	// 随机读IOPS评分 (参考: 优秀>500K IOPS)
	if m.RandomReadIOPS > 0 {
		readIO := math.Min(100, m.RandomReadIOPS/5000)
		score += readIO
		count++
	}

	// 随机写IOPS评分
	if m.RandomWriteIOPS > 0 {
		writeIO := math.Min(100, m.RandomWriteIOPS/5000)
		score += writeIO
		count++
	}

	// 延迟评分 (越低越好, <0.05ms = 满分)
	if m.LatencyP99Ms > 0 {
		latScore := math.Max(0, 100-m.LatencyP99Ms*1000)
		score += latScore
		count++
	}

	if count == 0 {
		return 0
	}
	return math.Round(score/count*100) / 100
}

// GetBenchmarkResult 获取基准测试结果
func (hm *HealthManager) GetBenchmarkResult(id string) (*BenchmarkResult, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result, exists := hm.benchResults[id]
	if !exists {
		return nil, fmt.Errorf("benchmark result %s not found", id)
	}
	return result, nil
}

// ListBenchmarkResults 列出所有基准测试结果
func (hm *HealthManager) ListBenchmarkResults() []*BenchmarkResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	results := make([]*BenchmarkResult, 0, len(hm.benchResults))
	for _, r := range hm.benchResults {
		results = append(results, r)
	}
	return results
}

// WaitForBenchmarks 等待所有异步基准测试完成（测试用）
func (hm *HealthManager) WaitForBenchmarks() {
	hm.benchWg.Wait()
}

// contains 检查字符串切片是否包含目标字符串
func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
