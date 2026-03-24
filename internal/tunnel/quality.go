// Package tunnel 提供内网穿透服务 - 连接质量检测与优化
package tunnel

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ConnectionQuality 连接质量指标
type ConnectionQuality struct {
	// 延迟（毫秒）
	Latency int64 `json:"latency"`
	// 抖动（毫秒）
	Jitter int64 `json:"jitter"`
	// 丢包率（百分比）
	PacketLoss float64 `json:"packet_loss"`
	// 连接质量评分（0-100）
	Score int `json:"score"`
	// 连接模式
	Mode TunnelMode `json:"mode"`
	// 是否使用中继
	IsRelay bool `json:"is_relay"`
	// 上次检测时间
	LastCheck time.Time `json:"last_check"`
	// 稳定性评级（excellent/good/fair/poor）
	Stability string `json:"stability"`
}

// QualityMonitorConfig 质量监控配置
type QualityMonitorConfig struct {
	// 探测间隔
	ProbeInterval time.Duration `json:"probe_interval"`
	// 探测超时
	ProbeTimeout time.Duration `json:"probe_timeout"`
	// 探测包数量
	ProbeCount int `json:"probe_count"`
	// 历史记录大小
	HistorySize int `json:"history_size"`
	// 质量阈值配置
	Thresholds QualityThresholds `json:"thresholds"`
}

// QualityThresholds 质量阈值配置
type QualityThresholds struct {
	// 优秀延迟阈值（毫秒）
	ExcellentLatency int64 `json:"excellent_latency"`
	// 良好延迟阈值（毫秒）
	GoodLatency int64 `json:"good_latency"`
	// 可接受延迟阈值（毫秒）
	FairLatency int64 `json:"fair_latency"`
	// 优秀丢包率阈值（百分比）
	ExcellentPacketLoss float64 `json:"excellent_packet_loss"`
	// 良好丢包率阈值（百分比）
	GoodPacketLoss float64 `json:"good_packet_loss"`
	// 可接受丢包率阈值（百分比）
	FairPacketLoss float64 `json:"fair_packet_loss"`
}

// DefaultQualityMonitorConfig 默认质量监控配置
func DefaultQualityMonitorConfig() QualityMonitorConfig {
	return QualityMonitorConfig{
		ProbeInterval: 5 * time.Second,
		ProbeTimeout:  2 * time.Second,
		ProbeCount:    5,
		HistorySize:   10,
		Thresholds: QualityThresholds{
			ExcellentLatency:    50,
			GoodLatency:         100,
			FairLatency:         200,
			ExcellentPacketLoss: 0.1,
			GoodPacketLoss:      1.0,
			FairPacketLoss:      5.0,
		},
	}
}

// LatencyRecord 延迟记录
type LatencyRecord struct {
	Timestamp time.Time
	Latency   int64
	Success   bool
}

// QualityMonitor 连接质量监控器
type QualityMonitor struct {
	config       QualityMonitorConfig
	logger       *zap.Logger
	conn         net.Conn
	remoteAddr   net.Addr

	// 当前质量
	quality atomic.Value // *ConnectionQuality

	// 延迟历史
	latencyHistory []LatencyRecord
	mu             sync.RWMutex

	// 统计
	totalProbes    int64
	successProbes  int64
	totalLatency   int64
	lastLatency    int64
	lastJitter     int64
	consecutiveLoss int

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewQualityMonitor 创建质量监控器
func NewQualityMonitor(config QualityMonitorConfig, logger *zap.Logger) *QualityMonitor {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &QualityMonitor{
		config:        config,
		logger:        logger,
		latencyHistory: make([]LatencyRecord, 0, config.HistorySize),
	}

	// 初始化默认质量
	m.quality.Store(&ConnectionQuality{
		Score:     100,
		Stability: "unknown",
	})

	return m
}

// SetConnection 设置连接
func (m *QualityMonitor) SetConnection(conn net.Conn, remoteAddr net.Addr) {
	m.mu.Lock()
	m.conn = conn
	m.remoteAddr = remoteAddr
	m.mu.Unlock()
}

// Start 启动监控
func (m *QualityMonitor) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	m.wg.Add(1)
	go m.probeLoop()

	return nil
}

// Stop 停止监控
func (m *QualityMonitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// probeLoop 探测循环
func (m *QualityMonitor) probeLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.probe()
		}
	}
}

// probe 执行探测
func (m *QualityMonitor) probe() {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	if conn == nil {
		return
	}

	var totalLatency int64
	var successCount int

	for i := 0; i < m.config.ProbeCount; i++ {
		latency, err := m.sendProbe(conn)
		if err == nil {
			totalLatency += latency
			successCount++
		}

		// 短暂间隔避免拥塞
		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalProbes += int64(m.config.ProbeCount)
	m.successProbes += int64(successCount)

	// 更新统计
	if successCount > 0 {
		avgLatency := totalLatency / int64(successCount)
		m.totalLatency += avgLatency

		// 计算抖动
		if m.lastLatency > 0 {
			diff := avgLatency - m.lastLatency
			if diff < 0 {
				diff = -diff
			}
			m.lastJitter = (m.lastJitter + diff) / 2
		}
		m.lastLatency = avgLatency

		// 添加历史记录
		m.latencyHistory = append(m.latencyHistory, LatencyRecord{
			Timestamp: time.Now(),
			Latency:   avgLatency,
			Success:   true,
		})

		if len(m.latencyHistory) > m.config.HistorySize {
			m.latencyHistory = m.latencyHistory[1:]
		}

		m.consecutiveLoss = 0
	} else {
		m.consecutiveLoss++
		m.latencyHistory = append(m.latencyHistory, LatencyRecord{
			Timestamp: time.Now(),
			Success:   false,
		})

		if len(m.latencyHistory) > m.config.HistorySize {
			m.latencyHistory = m.latencyHistory[1:]
		}
	}

	// 更新质量评估
	m.updateQuality()
}

// sendProbe 发送探测包
func (m *QualityMonitor) sendProbe(conn net.Conn) (int64, error) {
	// 设置超时
	deadline := time.Now().Add(m.config.ProbeTimeout)
	_ = conn.SetDeadline(deadline)

	// 发送探测消息（PING）
	start := time.Now()
	probeData := []byte("PING")

	_, err := conn.Write(probeData)
	if err != nil {
		return 0, err
	}

	// 读取响应
	resp := make([]byte, 4)
	n, err := conn.Read(resp)
	if err != nil {
		return 0, err
	}

	latency := time.Since(start).Milliseconds()

	if n != 4 || string(resp) != "PONG" {
		return 0, errors.New("invalid probe response")
	}

	return latency, nil
}

// updateQuality 更新质量评估
func (m *QualityMonitor) updateQuality() {
	// 计算丢包率
	var packetLoss float64
	if m.totalProbes > 0 {
		lostProbes := m.totalProbes - m.successProbes
		packetLoss = float64(lostProbes) / float64(m.totalProbes) * 100
	}

	// 计算平均延迟
	var avgLatency int64
	if m.successProbes > 0 {
		avgLatency = m.totalLatency / m.successProbes
	}

	// 计算质量分数
	score := m.calculateScore(avgLatency, m.lastJitter, packetLoss)

	// 确定稳定性评级
	stability := m.determineStability(score)

	quality := &ConnectionQuality{
		Latency:    avgLatency,
		Jitter:     m.lastJitter,
		PacketLoss: packetLoss,
		Score:      score,
		Stability:  stability,
		LastCheck:  time.Now(),
	}

	m.quality.Store(quality)
}

// calculateScore 计算质量分数
func (m *QualityMonitor) calculateScore(latency, jitter int64, packetLoss float64) int {
	thresholds := m.config.Thresholds

	// 延迟得分（权重 40%）
	latencyScore := 0
	switch {
	case latency <= thresholds.ExcellentLatency:
		latencyScore = 100
	case latency <= thresholds.GoodLatency:
		latencyScore = 80
	case latency <= thresholds.FairLatency:
		latencyScore = 60
	default:
		latencyScore = 40
	}

	// 丢包得分（权重 40%）
	lossScore := 0
	switch {
	case packetLoss <= thresholds.ExcellentPacketLoss:
		lossScore = 100
	case packetLoss <= thresholds.GoodPacketLoss:
		lossScore = 80
	case packetLoss <= thresholds.FairPacketLoss:
		lossScore = 60
	default:
		lossScore = 20
	}

	// 抖动得分（权重 20%）
	jitterScore := 100
	if jitter > 50 {
		jitterScore = 60
	} else if jitter > 100 {
		jitterScore = 40
	}

	// 综合得分
	totalScore := (latencyScore*40 + lossScore*40 + jitterScore*20) / 100

	return totalScore
}

// determineStability 确定稳定性评级
func (m *QualityMonitor) determineStability(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 70:
		return "good"
	case score >= 50:
		return "fair"
	default:
		return "poor"
	}
}

// GetQuality 获取当前质量
func (m *QualityMonitor) GetQuality() *ConnectionQuality {
	return m.quality.Load().(*ConnectionQuality)
}

// GetLatencyHistory 获取延迟历史
func (m *QualityMonitor) GetLatencyHistory() []LatencyRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]LatencyRecord, len(m.latencyHistory))
	copy(history, m.latencyHistory)
	return history
}

// ConnectionOptimizer 连接优化器
type ConnectionOptimizer struct {
	config       QualityMonitorConfig
	logger       *zap.Logger
	monitor      *QualityMonitor
	mode         TunnelMode
	relayAvailable bool

	// 自动切换阈值
	switchThreshold int // 质量分数低于此值时考虑切换
}

// NewConnectionOptimizer 创建连接优化器
func NewConnectionOptimizer(config QualityMonitorConfig, logger *zap.Logger) *ConnectionOptimizer {
	return &ConnectionOptimizer{
		config:          config,
		logger:          logger,
		monitor:         NewQualityMonitor(config, logger),
		switchThreshold: 50,
	}
}

// Start 启动优化器
func (o *ConnectionOptimizer) Start(ctx context.Context) error {
	return o.monitor.Start(ctx)
}

// Stop 停止优化器
func (o *ConnectionOptimizer) Stop() {
	o.monitor.Stop()
}

// SetConnection 设置连接
func (o *ConnectionOptimizer) SetConnection(conn net.Conn, remoteAddr net.Addr, mode TunnelMode) {
	o.mode = mode
	o.monitor.SetConnection(conn, remoteAddr)
}

// ShouldSwitchToRelay 判断是否应该切换到中继
func (o *ConnectionOptimizer) ShouldSwitchToRelay() bool {
	quality := o.monitor.GetQuality()

	// P2P 模式下，如果质量太差且中继可用，建议切换
	if o.mode == ModeP2P && o.relayAvailable {
		return quality.Score < o.switchThreshold
	}

	return false
}

// ShouldSwitchToP2P 判断是否应该切换到 P2P
func (o *ConnectionOptimizer) ShouldSwitchToP2P() bool {
	quality := o.monitor.GetQuality()

	// 中继模式下，如果延迟很低，可以尝试 P2P
	if o.mode == ModeRelay {
		return quality.Latency < 50 && quality.PacketLoss < 1.0
	}

	return false
}

// SetRelayAvailable 设置中继是否可用
func (o *ConnectionOptimizer) SetRelayAvailable(available bool) {
	o.relayAvailable = available
}

// GetQuality 获取连接质量
func (o *ConnectionOptimizer) GetQuality() *ConnectionQuality {
	return o.monitor.GetQuality()
}

// ConnectionStabilityTest 连接稳定性测试
type ConnectionStabilityTest struct {
	config StabilityTestConfig
	logger *zap.Logger
}

// StabilityTestConfig 稳定性测试配置
type StabilityTestConfig struct {
	Duration       time.Duration `json:"duration"`        // 测试持续时间
	ProbeInterval  time.Duration `json:"probe_interval"`  // 探测间隔
	SuccessRate    float64       `json:"success_rate"`    // 成功率阈值
	MaxLatency     int64         `json:"max_latency"`     // 最大延迟阈值
	MaxPacketLoss  float64       `json:"max_packet_loss"` // 最大丢包率阈值
}

// StabilityTestResult 稳定性测试结果
type StabilityTestResult struct {
	Success      bool          `json:"success"`
	TotalProbes  int           `json:"total_probes"`
	SuccessCount int           `json:"success_count"`
	FailedCount  int           `json:"failed_count"`
	AvgLatency   int64         `json:"avg_latency"`
	MaxLatency   int64         `json:"max_latency"`
	MinLatency   int64         `json:"min_latency"`
	PacketLoss   float64       `json:"packet_loss"`
	Duration     time.Duration `json:"duration"`
	StartTime    time.Time     `json:"start_time"`
}

// NewConnectionStabilityTest 创建稳定性测试
func NewConnectionStabilityTest(config StabilityTestConfig, logger *zap.Logger) *ConnectionStabilityTest {
	return &ConnectionStabilityTest{
		config: config,
		logger: logger,
	}
}

// Run 执行稳定性测试
func (t *ConnectionStabilityTest) Run(ctx context.Context, conn net.Conn) (*StabilityTestResult, error) {
	result := &StabilityTestResult{
		StartTime: time.Now(),
		MinLatency: 999999,
	}

	var totalLatency int64
	var maxLatency int64

	timeout := time.NewTimer(t.config.Duration)
	defer timeout.Stop()

	ticker := time.NewTicker(t.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timeout.C:
			// 测试完成
			goto done
		case <-ticker.C:
			result.TotalProbes++

			// 发送探测
			start := time.Now()
			probeData := []byte("PING_TEST")

			err := conn.SetDeadline(time.Now().Add(2 * time.Second))
			if err != nil {
				result.FailedCount++
				continue
			}

			_, err = conn.Write(probeData)
			if err != nil {
				result.FailedCount++
				continue
			}

			resp := make([]byte, 9)
			n, err := conn.Read(resp)
			if err != nil || n != 9 {
				result.FailedCount++
				continue
			}

			latency := time.Since(start).Milliseconds()
			totalLatency += latency
			result.SuccessCount++

			if latency > maxLatency {
				maxLatency = latency
			}
			if latency < result.MinLatency {
				result.MinLatency = latency
			}
		}
	}

done:
	result.Duration = time.Since(result.StartTime)
	result.MaxLatency = maxLatency

	if result.SuccessCount > 0 {
		result.AvgLatency = totalLatency / int64(result.SuccessCount)
	}

	if result.TotalProbes > 0 {
		result.PacketLoss = float64(result.FailedCount) / float64(result.TotalProbes) * 100
	}

	// 判断是否通过测试
	result.Success = t.evaluateResult(result)

	return result, nil
}

// evaluateResult 评估测试结果
func (t *ConnectionStabilityTest) evaluateResult(result *StabilityTestResult) bool {
	// 检查成功率
	successRate := float64(result.SuccessCount) / float64(result.TotalProbes)
	if successRate < t.config.SuccessRate {
		return false
	}

	// 检查最大延迟
	if result.MaxLatency > t.config.MaxLatency {
		return false
	}

	// 检查丢包率
	if result.PacketLoss > t.config.MaxPacketLoss {
		return false
	}

	return true
}