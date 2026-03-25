package cluster

import (
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FailureDetector Phi Accrual 故障检测器
// 使用 Phi Accrual 算法进行更精确的故障检测.
type FailureDetector struct {
	threshold  float64
	samples    map[string]*SampleWindow
	maxSamples int
	mu         sync.RWMutex
	logger     *zap.Logger

	// 快速检测参数
	minStdDev        time.Duration
	acceptableHBTime time.Duration
}

// SampleWindow 心跳样本窗口.
type SampleWindow struct {
	intervals     []time.Duration
	lastHeartbeat time.Time
	mean          float64
	variance      float64
}

// NewFailureDetector 创建故障检测器.
func NewFailureDetector(timeout time.Duration, threshold int, logger *zap.Logger) *FailureDetector {
	return &FailureDetector{
		threshold:        float64(threshold),
		samples:          make(map[string]*SampleWindow),
		maxSamples:       1000,
		minStdDev:        500 * time.Millisecond,
		acceptableHBTime: timeout,
		logger:           logger,
	}
}

// RecordHeartbeat 记录心跳.
func (fd *FailureDetector) RecordHeartbeat(nodeID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	now := time.Now()

	sample, exists := fd.samples[nodeID]
	if !exists {
		sample = &SampleWindow{
			intervals:     make([]time.Duration, 0, fd.maxSamples),
			lastHeartbeat: now,
		}
		fd.samples[nodeID] = sample
		return
	}

	// 计算间隔
	interval := now.Sub(sample.lastHeartbeat)
	sample.lastHeartbeat = now

	// 添加到样本窗口
	sample.intervals = append(sample.intervals, interval)
	if len(sample.intervals) > fd.maxSamples {
		sample.intervals = sample.intervals[1:]
	}

	// 更新统计
	fd.updateStats(sample)
}

// updateStats 更新样本统计.
func (fd *FailureDetector) updateStats(sample *SampleWindow) {
	if len(sample.intervals) == 0 {
		return
	}

	// 计算均值
	var sum float64
	for _, i := range sample.intervals {
		sum += float64(i)
	}
	sample.mean = sum / float64(len(sample.intervals))

	// 计算方差
	var varianceSum float64
	for _, i := range sample.intervals {
		diff := float64(i) - sample.mean
		varianceSum += diff * diff
	}
	sample.variance = varianceSum / float64(len(sample.intervals))
}

// Phi 计算 Phi 值
// Phi 值表示节点故障的可能性，值越大可能性越高.
func (fd *FailureDetector) Phi(nodeID string, elapsed time.Duration) float64 {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	sample, exists := fd.samples[nodeID]
	if !exists {
		// 没有历史数据，使用默认阈值
		return float64(elapsed) / float64(fd.acceptableHBTime)
	}

	if len(sample.intervals) == 0 {
		return float64(elapsed) / float64(fd.acceptableHBTime)
	}

	// 使用正态分布计算概率
	mean := sample.mean
	variance := sample.variance

	// 确保方差有最小值
	stdDev := math.Sqrt(variance)
	if stdDev < float64(fd.minStdDev) {
		stdDev = float64(fd.minStdDev)
	}

	// 计算 Phi
	// Phi(t) = -log(1 - F(t)) 其中 F 是累积分布函数
	elapsedF := float64(elapsed)

	// 使用误差函数近似正态分布的累积分布函数
	y := (elapsedF - mean) / stdDev
	probability := 1 - normalCDF(y)

	// 避免 log(0)
	if probability < 1e-10 {
		probability = 1e-10
	}

	phi := -math.Log10(probability)

	return phi
}

// normalCDF 正态分布累积分布函数近似.
func normalCDF(x float64) float64 {
	// 使用近似公式
	// erfc 是互补误差函数
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// Threshold 获取阈值.
func (fd *FailureDetector) Threshold() float64 {
	return fd.threshold
}

// IsFailed 判断节点是否故障.
func (fd *FailureDetector) IsFailed(nodeID string, elapsed time.Duration) bool {
	return fd.Phi(nodeID, elapsed) > fd.threshold
}

// GetStats 获取检测器统计.
func (fd *FailureDetector) GetStats(nodeID string) map[string]interface{} {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	sample, exists := fd.samples[nodeID]
	if !exists {
		return map[string]interface{}{
			"samples": 0,
			"mean":    0,
			"stddev":  0,
		}
	}

	stdDev := math.Sqrt(sample.variance)

	return map[string]interface{}{
		"samples":        len(sample.intervals),
		"mean":           time.Duration(sample.mean).String(),
		"stddev":         time.Duration(stdDev).String(),
		"last_heartbeat": sample.lastHeartbeat,
	}
}

// Remove 移除节点.
func (fd *FailureDetector) Remove(nodeID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	delete(fd.samples, nodeID)
}

// Reset 重置节点状态.
func (fd *FailureDetector) Reset(nodeID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	delete(fd.samples, nodeID)
}

// AccrualLevel 获取累积级别（用于监控）.
type AccrualLevel int

// 累积级别常量.
const (
	AccrualLevelHealthy  AccrualLevel = iota // 健康
	AccrualLevelSuspect                      // 可疑
	AccrualLevelWarning                      // 警告
	AccrualLevelCritical                     // 严重
)

// GetAccrualLevel 获取累积级别.
func (fd *FailureDetector) GetAccrualLevel(nodeID string, elapsed time.Duration) AccrualLevel {
	phi := fd.Phi(nodeID, elapsed)

	switch {
	case phi < fd.threshold*0.5:
		return AccrualLevelHealthy
	case phi < fd.threshold*0.75:
		return AccrualLevelSuspect
	case phi < fd.threshold:
		return AccrualLevelWarning
	default:
		return AccrualLevelCritical
	}
}

// FailureDetectorStats 故障检测器统计.
type FailureDetectorStats struct {
	NodeID        string    `json:"node_id"`
	Samples       int       `json:"samples"`
	MeanInterval  string    `json:"mean_interval"`
	StdDev        string    `json:"std_dev"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Phi           float64   `json:"phi"`
	AccrualLevel  string    `json:"accrual_level"`
	Threshold     float64   `json:"threshold"`
}

// GetDetailedStats 获取详细统计.
func (fd *FailureDetector) GetDetailedStats(nodeID string, elapsed time.Duration) *FailureDetectorStats {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	sample, exists := fd.samples[nodeID]
	if !exists {
		return &FailureDetectorStats{
			NodeID:       nodeID,
			Phi:          float64(elapsed) / float64(fd.acceptableHBTime),
			AccrualLevel: "unknown",
			Threshold:    fd.threshold,
		}
	}

	stdDev := math.Sqrt(sample.variance)
	phi := fd.Phi(nodeID, elapsed)
	level := fd.GetAccrualLevel(nodeID, elapsed)

	var levelStr string
	switch level {
	case AccrualLevelHealthy:
		levelStr = "healthy"
	case AccrualLevelSuspect:
		levelStr = "suspect"
	case AccrualLevelWarning:
		levelStr = "warning"
	case AccrualLevelCritical:
		levelStr = "critical"
	}

	return &FailureDetectorStats{
		NodeID:        nodeID,
		Samples:       len(sample.intervals),
		MeanInterval:  time.Duration(sample.mean).String(),
		StdDev:        time.Duration(stdDev).String(),
		LastHeartbeat: sample.lastHeartbeat,
		Phi:           phi,
		AccrualLevel:  levelStr,
		Threshold:     fd.threshold,
	}
}
