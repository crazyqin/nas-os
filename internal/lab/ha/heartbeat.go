// Package ha 心跳管理器
// 实现 Phi Accrual 故障检测算法，参考 Cassandra/Akka 的心跳机制
package ha

import (
	"context"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HeartbeatManager 心跳管理器.
type HeartbeatManager struct {
	config   *HAConfig
	samples  map[string]*HeartbeatSample
	detector *PhiAccrualDetector
	senders  map[string]*HeartbeatSender
	mu       sync.RWMutex
	ctx      context.Context
	logger   *zap.Logger
}

// HeartbeatSample 心跳样本.
type HeartbeatSample struct {
	NodeID        string
	Intervals     []time.Duration
	LastHeartbeat time.Time
	Mean          float64
	Variance      float64
	MissCount     int
	TotalSamples  int
}

// HeartbeatSender 心跳发送器.
type HeartbeatSender struct {
	NodeID      string
	Address     string
	Port        int
	LastSend    time.Time
	SuccessRate float64
	FailCount   int
}

// PhiAccrualDetector Phi 累积故障检测器
// 参考: Hayashi, et al. "Phi Accrual Failure Detector".
type PhiAccrualDetector struct {
	threshold    float64
	minStdDev    time.Duration
	acceptableHB time.Duration
	sampleWindow int
}

// NewHeartbeatManager 创建心跳管理器.
func NewHeartbeatManager(config *HAConfig, logger *zap.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		config:   config,
		samples:  make(map[string]*HeartbeatSample),
		senders:  make(map[string]*HeartbeatSender),
		detector: NewPhiAccrualDetector(config.HeartbeatTimeout, config.HeartbeatMissMax),
		logger:   logger,
	}
}

// NewPhiAccrualDetector 创建 Phi 累积检测器.
func NewPhiAccrualDetector(timeout time.Duration, threshold int) *PhiAccrualDetector {
	return &PhiAccrualDetector{
		threshold:    float64(threshold),
		minStdDev:    500 * time.Millisecond,
		acceptableHB: timeout,
		sampleWindow: 1000,
	}
}

// Start 启动心跳管理.
func (hm *HeartbeatManager) Start(ctx context.Context) error {
	hm.ctx = ctx

	// 为每个节点创建样本和发送器
	for _, peer := range hm.config.Peers {
		hm.samples[peer.ID] = &HeartbeatSample{
			NodeID:        peer.ID,
			Intervals:     make([]time.Duration, 0, hm.detector.sampleWindow),
			LastHeartbeat: time.Now(),
		}

		hm.senders[peer.ID] = &HeartbeatSender{
			NodeID:      peer.ID,
			Address:     peer.Address,
			Port:        peer.Port,
			SuccessRate: 1.0,
		}
	}

	hm.logger.Info("Heartbeat manager started",
		zap.Int("peers", len(hm.config.Peers)),
	)

	return nil
}

// Stop 停止心跳管理.
func (hm *HeartbeatManager) Stop() {
	hm.logger.Info("Heartbeat manager stopped")
}

// RecordHeartbeat 记录心跳.
func (hm *HeartbeatManager) RecordHeartbeat(nodeID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	sample, exists := hm.samples[nodeID]
	if !exists {
		sample = &HeartbeatSample{
			NodeID:        nodeID,
			Intervals:     make([]time.Duration, 0, hm.detector.sampleWindow),
			LastHeartbeat: time.Now(),
			MissCount:     0,
		}
		hm.samples[nodeID] = sample
		return
	}

	now := time.Now()

	// 计算间隔
	if sample.LastHeartbeat.IsZero() {
		sample.LastHeartbeat = now
		return
	}

	interval := now.Sub(sample.LastHeartbeat)
	sample.LastHeartbeat = now
	sample.MissCount = 0 // 重置丢失计数

	// 添加到样本窗口
	sample.Intervals = append(sample.Intervals, interval)
	if len(sample.Intervals) > hm.detector.sampleWindow {
		sample.Intervals = sample.Intervals[1:]
	}
	sample.TotalSamples++

	// 更新统计
	hm.updateSampleStats(sample)
}

// updateSampleStats 更新样本统计.
func (hm *HeartbeatManager) updateSampleStats(sample *HeartbeatSample) {
	if len(sample.Intervals) == 0 {
		return
	}

	// 计算均值
	var sum float64
	for _, i := range sample.Intervals {
		sum += float64(i)
	}
	sample.Mean = sum / float64(len(sample.Intervals))

	// 计算方差
	var varianceSum float64
	for _, i := range sample.Intervals {
		diff := float64(i) - sample.Mean
		varianceSum += diff * diff
	}
	sample.Variance = varianceSum / float64(len(sample.Intervals))
}

// Phi 计算 Phi 值
// Phi 表示节点故障的可能性，值越大可能性越高.
func (hm *HeartbeatManager) Phi(nodeID string) float64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	sample, exists := hm.samples[nodeID]
	if !exists {
		return hm.detector.threshold
	}

	elapsed := time.Since(sample.LastHeartbeat)
	return hm.detector.ComputePhi(sample, elapsed)
}

// ComputePhi 计算 Phi 值.
func (pd *PhiAccrualDetector) ComputePhi(sample *HeartbeatSample, elapsed time.Duration) float64 {
	if len(sample.Intervals) == 0 {
		return float64(elapsed) / float64(pd.acceptableHB)
	}

	// 使用正态分布计算概率
	mean := sample.Mean
	stdDev := math.Sqrt(sample.Variance)

	// 确保方差有最小值
	if stdDev < float64(pd.minStdDev) {
		stdDev = float64(pd.minStdDev)
	}

	// 计算 Phi = -log10(1 - F(t))
	elapsedF := float64(elapsed)

	// 标准化
	y := (elapsedF - mean) / stdDev

	// 使用累积分布函数
	probability := 1 - normalCDF(y)

	// 避免 log(0)
	if probability < 1e-10 {
		probability = 1e-10
	}

	phi := -math.Log10(probability)
	return phi
}

// normalCDF 正态分布累积函数近似.
func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// IsNodeHealthy 判断节点是否健康.
func (hm *HeartbeatManager) IsNodeHealthy(nodeID string) bool {
	phi := hm.Phi(nodeID)
	return phi < hm.detector.threshold
}

// GetPhiLevel 获取 Phi 级别.
func (hm *HeartbeatManager) GetPhiLevel(nodeID string) PhiLevel {
	phi := hm.Phi(nodeID)

	switch {
	case phi < hm.detector.threshold*0.5:
		return PhiLevelHealthy
	case phi < hm.detector.threshold*0.75:
		return PhiLevelSuspect
	case phi < hm.detector.threshold:
		return PhiLevelWarning
	default:
		return PhiLevelCritical
	}
}

// PhiLevel Phi 级别.
type PhiLevel int

const (
	PhiLevelHealthy  PhiLevel = iota // 健康
	PhiLevelSuspect                  // 可疑
	PhiLevelWarning                  // 警告
	PhiLevelCritical                 // 严重
)

// GetNodeStats 获取节点统计.
func (hm *HeartbeatManager) GetNodeStats(nodeID string) *HeartbeatStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	sample, exists := hm.samples[nodeID]
	if !exists {
		return &HeartbeatStats{
			NodeID: nodeID,
			Status: "unknown",
		}
	}

	sender := hm.senders[nodeID]

	phi := hm.Phi(nodeID)
	level := hm.GetPhiLevel(nodeID)

	stdDev := math.Sqrt(sample.Variance)

	var status string
	switch level {
	case PhiLevelHealthy:
		status = "healthy"
	case PhiLevelSuspect:
		status = "suspect"
	case PhiLevelWarning:
		status = "warning"
	case PhiLevelCritical:
		status = "critical"
	}

	var successRate float64
	if sender != nil {
		successRate = sender.SuccessRate
	}

	return &HeartbeatStats{
		NodeID:        nodeID,
		Status:        status,
		Phi:           phi,
		Threshold:     hm.detector.threshold,
		MeanInterval:  time.Duration(sample.Mean).String(),
		StdDev:        time.Duration(stdDev).String(),
		LastHeartbeat: sample.LastHeartbeat,
		MissCount:     sample.MissCount,
		TotalSamples:  sample.TotalSamples,
		SuccessRate:   successRate,
	}
}

// HeartbeatStats 心跳统计.
type HeartbeatStats struct {
	NodeID        string    `json:"node_id"`
	Status        string    `json:"status"`
	Phi           float64   `json:"phi"`
	Threshold     float64   `json:"threshold"`
	MeanInterval  string    `json:"mean_interval"`
	StdDev        string    `json:"std_dev"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	MissCount     int       `json:"miss_count"`
	TotalSamples  int       `json:"total_samples"`
	SuccessRate   float64   `json:"success_rate"`
}

// RecordMiss 记录心跳丢失.
func (hm *HeartbeatManager) RecordMiss(nodeID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	sample, exists := hm.samples[nodeID]
	if !exists {
		return
	}

	sample.MissCount++

	hm.logger.Debug("Heartbeat miss recorded",
		zap.String("node_id", nodeID),
		zap.Int("miss_count", sample.MissCount),
	)

	// 更新发送器统计
	if sender := hm.senders[nodeID]; sender != nil {
		sender.FailCount++
		total := sender.FailCount + sample.TotalSamples
		if total > 0 {
			sender.SuccessRate = float64(sample.TotalSamples) / float64(total)
		}
	}
}

// GetThreshold 获取阈值.
func (hm *HeartbeatManager) GetThreshold() float64 {
	return hm.detector.threshold
}

// RemoveNode 移除节点.
func (hm *HeartbeatManager) RemoveNode(nodeID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.samples, nodeID)
	delete(hm.senders, nodeID)
}

// ResetNode 重置节点状态.
func (hm *HeartbeatManager) ResetNode(nodeID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.samples, nodeID)

	hm.samples[nodeID] = &HeartbeatSample{
		NodeID:        nodeID,
		Intervals:     make([]time.Duration, 0, hm.detector.sampleWindow),
		LastHeartbeat: time.Now(),
	}
}
