// Package tunnel 提供内网穿透服务 - 连接质量测试
package tunnel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewQualityMonitor 测试创建质量监控器
func TestNewQualityMonitor(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	monitor := NewQualityMonitor(config, nil)

	require.NotNil(t, monitor)
	assert.NotNil(t, monitor.quality)
}

// TestConnectionQualityScore 测试连接质量评分
func TestConnectionQualityScore(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	monitor := NewQualityMonitor(config, nil)

	// 获取默认质量
	quality := monitor.GetQuality()
	assert.NotNil(t, quality)
	assert.Equal(t, 100, quality.Score)
}

// TestCalculateScore 测试评分计算
func TestCalculateScore(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	monitor := NewQualityMonitor(config, nil)

	tests := []struct {
		name       string
		latency    int64
		jitter     int64
		packetLoss float64
		minScore   int
		maxScore   int
	}{
		{
			name:       "excellent connection",
			latency:    30,
			jitter:     5,
			packetLoss: 0.05,
			minScore:   90,
			maxScore:   100,
		},
		{
			name:       "good connection",
			latency:    80,
			jitter:     20,
			packetLoss: 0.5,
			minScore:   70,
			maxScore:   89,
		},
		{
			name:       "fair connection",
			latency:    150,
			jitter:     40,
			packetLoss: 2.0,
			minScore:   50,
			maxScore:   69,
		},
		{
			name:       "poor connection",
			latency:    300,
			jitter:     80,
			packetLoss: 10.0,
			minScore:   0,
			maxScore:   49,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := monitor.calculateScore(tt.latency, tt.jitter, tt.packetLoss)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}

// TestDetermineStability 测试稳定性评级
func TestDetermineStability(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	monitor := NewQualityMonitor(config, nil)

	tests := []struct {
		score      int
		stability  string
	}{
		{95, "excellent"},
		{85, "good"},
		{75, "good"},
		{65, "fair"},
		{55, "fair"},
		{45, "poor"},
		{25, "poor"},
	}

	for _, tt := range tests {
		t.Run(tt.stability, func(t *testing.T) {
			result := monitor.determineStability(tt.score)
			assert.Equal(t, tt.stability, result)
		})
	}
}

// TestQualityMonitorStartStop 测试启动和停止
func TestQualityMonitorStartStop(t *testing.T) {
	config := QualityMonitorConfig{
		ProbeInterval: 100 * time.Millisecond,
		ProbeTimeout:  50 * time.Millisecond,
		ProbeCount:    3,
		HistorySize:   10,
	}
	monitor := NewQualityMonitor(config, nil)

	ctx := context.Background()
	err := monitor.Start(ctx)
	require.NoError(t, err)

	// 短暂运行
	time.Sleep(150 * time.Millisecond)

	// 停止
	monitor.Stop()
}

// TestConnectionOptimizer 测试连接优化器
func TestConnectionOptimizer(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	optimizer := NewConnectionOptimizer(config, nil)

	require.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.monitor)
}

// TestConnectionOptimizerShouldSwitch 测试模式切换判断
func TestConnectionOptimizerShouldSwitch(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	optimizer := NewConnectionOptimizer(config, nil)

	// 设置中继可用
	optimizer.SetRelayAvailable(true)

	// P2P 模式下，质量差时应该切换
	optimizer.mode = ModeP2P
	optimizer.monitor.quality.Store(&ConnectionQuality{
		Score: 40,
		Mode:  ModeP2P,
	})
	assert.True(t, optimizer.ShouldSwitchToRelay())

	// P2P 模式下，质量好时不应该切换
	optimizer.monitor.quality.Store(&ConnectionQuality{
		Score: 80,
		Mode:  ModeP2P,
	})
	assert.False(t, optimizer.ShouldSwitchToRelay())

	// Relay 模式下，延迟低时可以尝试 P2P
	optimizer.mode = ModeRelay
	optimizer.monitor.quality.Store(&ConnectionQuality{
		Latency:    30,
		PacketLoss: 0.5,
		Mode:       ModeRelay,
	})
	assert.True(t, optimizer.ShouldSwitchToP2P())
}

// TestConnectionOptimizerStartStop 测试优化器启动停止
func TestConnectionOptimizerStartStop(t *testing.T) {
	config := QualityMonitorConfig{
		ProbeInterval: 100 * time.Millisecond,
	}
	optimizer := NewConnectionOptimizer(config, nil)

	ctx := context.Background()
	err := optimizer.Start(ctx)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	optimizer.Stop()
}

// TestStabilityTestResult 测试稳定性测试结果
func TestStabilityTestResult(t *testing.T) {
	result := &StabilityTestResult{
		TotalProbes:  100,
		SuccessCount: 95,
		FailedCount:  5,
		AvgLatency:   50,
		MaxLatency:   100,
		MinLatency:   20,
		PacketLoss:   5.0,
	}

	assert.Equal(t, 100, result.TotalProbes)
	assert.Equal(t, 95, result.SuccessCount)
	assert.Equal(t, 5, result.FailedCount)
	assert.Equal(t, int64(50), result.AvgLatency)
	assert.Equal(t, 5.0, result.PacketLoss)
}

// TestQualityThresholds 测试质量阈值
func TestQualityThresholds(t *testing.T) {
	thresholds := QualityThresholds{
		ExcellentLatency:    50,
		GoodLatency:         100,
		FairLatency:         200,
		ExcellentPacketLoss: 0.1,
		GoodPacketLoss:      1.0,
		FairPacketLoss:      5.0,
	}

	assert.Equal(t, int64(50), thresholds.ExcellentLatency)
	assert.Equal(t, int64(100), thresholds.GoodLatency)
	assert.Equal(t, int64(200), thresholds.FairLatency)
	assert.Equal(t, 0.1, thresholds.ExcellentPacketLoss)
	assert.Equal(t, 1.0, thresholds.GoodPacketLoss)
	assert.Equal(t, 5.0, thresholds.FairPacketLoss)
}

// TestLatencyRecord 测试延迟记录
func TestLatencyRecord(t *testing.T) {
	now := time.Now()
	record := LatencyRecord{
		Timestamp: now,
		Latency:   50,
		Success:   true,
	}

	assert.Equal(t, now, record.Timestamp)
	assert.Equal(t, int64(50), record.Latency)
	assert.True(t, record.Success)
}

// TestGetLatencyHistory 测试获取延迟历史
func TestGetLatencyHistory(t *testing.T) {
	config := DefaultQualityMonitorConfig()
	config.HistorySize = 5
	monitor := NewQualityMonitor(config, nil)

	// 获取空历史
	history := monitor.GetLatencyHistory()
	assert.NotNil(t, history)
	assert.Len(t, history, 0)
}

// TestConnectionQualityFields 测试连接质量字段
func TestConnectionQualityFields(t *testing.T) {
	quality := &ConnectionQuality{
		Latency:    50,
		Jitter:     10,
		PacketLoss: 0.5,
		Score:      85,
		Mode:       ModeP2P,
		IsRelay:    false,
		Stability:  "good",
	}

	assert.Equal(t, int64(50), quality.Latency)
	assert.Equal(t, int64(10), quality.Jitter)
	assert.Equal(t, 0.5, quality.PacketLoss)
	assert.Equal(t, 85, quality.Score)
	assert.Equal(t, ModeP2P, quality.Mode)
	assert.False(t, quality.IsRelay)
	assert.Equal(t, "good", quality.Stability)
}

// TestQualityMonitorConfigDefaults 测试质量监控配置默认值
func TestQualityMonitorConfigDefaults(t *testing.T) {
	config := DefaultQualityMonitorConfig()

	assert.Equal(t, 5*time.Second, config.ProbeInterval)
	assert.Equal(t, 2*time.Second, config.ProbeTimeout)
	assert.Equal(t, 5, config.ProbeCount)
	assert.Equal(t, 10, config.HistorySize)
	assert.Equal(t, int64(50), config.Thresholds.ExcellentLatency)
	assert.Equal(t, int64(100), config.Thresholds.GoodLatency)
	assert.Equal(t, int64(200), config.Thresholds.FairLatency)
}