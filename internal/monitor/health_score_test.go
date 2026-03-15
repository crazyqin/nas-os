package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== HealthScorer 基础测试 ==========

func TestNewHealthScorer(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	assert.NotNil(t, scorer)
	assert.NotNil(t, scorer.history)
	assert.Equal(t, 1000, scorer.maxHistory)
}

func TestHealthScorer_SetWeights(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	weights := HealthWeights{
		CPU:     0.3,
		Memory:  0.3,
		Disk:    0.2,
		Network: 0.1,
		SMART:   0.05,
		Uptime:  0.05,
	}

	scorer.SetWeights(weights)

	assert.Equal(t, 0.3, scorer.weights.CPU)
	assert.Equal(t, 0.3, scorer.weights.Memory)
}

func TestHealthScorer_SetThresholds(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	thresholds := HealthThresholds{
		CPUWarning:     75,
		CPUCritical:    95,
		MemoryWarning:  80,
		MemoryCritical: 95,
		DiskWarning:    85,
		DiskCritical:   98,
	}

	scorer.SetThresholds(thresholds)

	assert.Equal(t, float64(75), scorer.thresholds.CPUWarning)
	assert.Equal(t, float64(95), scorer.thresholds.CPUCritical)
}

// ========== 评分计算测试 ==========

func TestHealthScorer_CalculateScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	score := scorer.CalculateScore()

	assert.NotNil(t, score)
	assert.GreaterOrEqual(t, score.TotalScore, 0.0)
	assert.LessOrEqual(t, score.TotalScore, 100.0)
	assert.NotEmpty(t, score.Grade)
	assert.NotNil(t, score.Components)
	assert.NotNil(t, score.Issues)
	assert.NotNil(t, score.Recommendations)
	assert.False(t, score.Timestamp.IsZero())
}

func TestHealthScorer_CalculateScore_Components(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	score := scorer.CalculateScore()

	// CPU 组件
	assert.GreaterOrEqual(t, score.Components.CPU.Score, 0.0)
	assert.LessOrEqual(t, score.Components.CPU.Score, 100.0)
	assert.NotEmpty(t, score.Components.CPU.Status)

	// 内存组件
	assert.GreaterOrEqual(t, score.Components.Memory.Score, 0.0)
	assert.LessOrEqual(t, score.Components.Memory.Score, 100.0)
	assert.NotEmpty(t, score.Components.Memory.Status)

	// 磁盘组件
	assert.GreaterOrEqual(t, score.Components.Disk.Score, 0.0)
	assert.LessOrEqual(t, score.Components.Disk.Score, 100.0)

	// 网络组件
	assert.GreaterOrEqual(t, score.Components.Network.Score, 0.0)
	assert.LessOrEqual(t, score.Components.Network.Score, 100.0)
}

func TestHealthScorer_CalculateScore_MultipleTimes(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 计算多次，验证历史记录
	for i := 0; i < 3; i++ {
		_ = scorer.CalculateScore()
	}

	history := scorer.GetHistory(10)
	assert.GreaterOrEqual(t, len(history), 3)
}

// ========== 等级转换测试 ==========

func TestHealthScorer_ScoreToGrade(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{90, "A"},
		{85, "B"},
		{80, "B"},
		{75, "C"},
		{70, "C"},
		{65, "D"},
		{60, "D"},
		{55, "F"},
		{50, "F"},
		{0, "F"},
	}

	for _, tt := range tests {
		grade := scorer.scoreToGrade(tt.score)
		assert.Equal(t, tt.grade, grade, "score=%.0f", tt.score)
	}
}

// ========== 历史记录测试 ==========

func TestHealthScorer_GetHistory(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 空历史
	history := scorer.GetHistory(10)
	assert.Len(t, history, 0)

	// 添加历史
	_ = scorer.CalculateScore()
	_ = scorer.CalculateScore()

	history = scorer.GetHistory(10)
	assert.GreaterOrEqual(t, len(history), 2)
}

func TestHealthScorer_GetHistory_Limit(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 添加多个历史记录
	for i := 0; i < 5; i++ {
		_ = scorer.CalculateScore()
	}

	// 限制返回数量
	history := scorer.GetHistory(2)
	assert.LessOrEqual(t, len(history), 2)
}

func TestHealthScorer_GetLastScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 未计算过
	score := scorer.GetLastScore()
	assert.Nil(t, score)

	// 计算后
	_ = scorer.CalculateScore()
	score = scorer.GetLastScore()
	assert.NotNil(t, score)
}

// ========== 统计测试 ==========

func TestHealthScorer_GetScoreStats(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 无历史数据
	stats := scorer.GetScoreStats(time.Hour)
	assert.Equal(t, 0, stats["count"])

	// 添加历史
	for i := 0; i < 3; i++ {
		_ = scorer.CalculateScore()
	}

	stats = scorer.GetScoreStats(time.Hour)
	assert.GreaterOrEqual(t, stats["count"], 3)
	assert.NotNil(t, stats["average"])
	assert.NotNil(t, stats["min"])
	assert.NotNil(t, stats["max"])
}

// ========== 趋势测试 ==========

func TestHealthScorer_CalculateTrend(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 首次计算
	score := scorer.CalculateScore()
	assert.Equal(t, "stable", score.Trend.Direction)

	// 再次计算
	score = scorer.CalculateScore()
	assert.NotEmpty(t, score.Trend.Direction)
}

// ========== 建议生成测试 ==========

func TestHealthScorer_GenerateRecommendations(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	score := &HealthScore{
		TotalScore: 50,
		Components: ComponentScores{
			CPU:    ComponentHealth{Score: 60},
			Memory: ComponentHealth{Score: 50},
			Disk:   ComponentHealth{Score: 40},
			SMART:  ComponentHealth{Score: 70},
		},
		Recommendations: make([]string, 0),
	}

	scorer.generateRecommendations(score)

	// 低分应该生成建议
	assert.Greater(t, len(score.Recommendations), 0)
}

// ========== 组件评分测试 ==========

func TestHealthScorer_CalculateCPUScore_Healthy(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		CPUUsage: 30.0,
		LoadAvg:  []float64{0.5, 0.4, 0.3},
	}

	health := scorer.calculateCPUScore(stats, scorer.thresholds)

	assert.GreaterOrEqual(t, health.Score, 70.0)
	assert.Equal(t, "healthy", health.Status)
}

func TestHealthScorer_CalculateCPUScore_Warning(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		CPUUsage: 80.0,
		LoadAvg:  []float64{2.0, 1.5, 1.0},
	}

	health := scorer.calculateCPUScore(stats, scorer.thresholds)

	assert.Equal(t, "warning", health.Status)
}

func TestHealthScorer_CalculateCPUScore_Critical(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		CPUUsage: 95.0,
		LoadAvg:  []float64{5.0, 4.0, 3.0},
	}

	health := scorer.calculateCPUScore(stats, scorer.thresholds)

	assert.Equal(t, "critical", health.Status)
	assert.Less(t, health.Score, 50.0)
}

func TestHealthScorer_CalculateMemoryScore_Healthy(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		MemoryUsage: 50.0,
		SwapUsage:   10.0,
	}

	health := scorer.calculateMemoryScore(stats, scorer.thresholds)

	assert.Equal(t, "healthy", health.Status)
	assert.GreaterOrEqual(t, health.Score, 70.0)
}

func TestHealthScorer_CalculateMemoryScore_Warning(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		MemoryUsage: 80.0,
		SwapUsage:   20.0,
	}

	health := scorer.calculateMemoryScore(stats, scorer.thresholds)

	assert.Equal(t, "warning", health.Status)
}

func TestHealthScorer_CalculateMemoryScore_Critical(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	stats := &SystemStats{
		MemoryUsage: 95.0,
		SwapUsage:   60.0,
	}

	health := scorer.calculateMemoryScore(stats, scorer.thresholds)

	assert.Equal(t, "critical", health.Status)
}

func TestHealthScorer_CalculateDiskScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 正常磁盘
	disks := []*DiskStats{
		{MountPoint: "/", UsagePercent: 50.0, FSType: "ext4"},
	}

	health := scorer.calculateDiskScore(disks, scorer.thresholds)
	assert.Equal(t, "healthy", health.Status)

	// 警告磁盘
	disks = []*DiskStats{
		{MountPoint: "/", UsagePercent: 85.0, FSType: "ext4"},
	}

	health = scorer.calculateDiskScore(disks, scorer.thresholds)
	assert.Equal(t, "warning", health.Status)
}

func TestHealthScorer_CalculateDiskScore_SkipPseudoFS(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 伪文件系统应被跳过
	disks := []*DiskStats{
		{MountPoint: "/dev", UsagePercent: 0, FSType: "devtmpfs"},
		{MountPoint: "/run", UsagePercent: 0, FSType: "tmpfs"},
	}

	health := scorer.calculateDiskScore(disks, scorer.thresholds)
	assert.Equal(t, "healthy", health.Status)
}

func TestHealthScorer_CalculateNetworkScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 正常网络
	nets := []*NetworkStats{
		{Interface: "eth0", RXPackets: 1000, TXPackets: 1000, RXErrors: 0, TXErrors: 0},
	}

	health := scorer.calculateNetworkScore(nets)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, 100.0, health.Score)
}

func TestHealthScorer_CalculateNetworkScore_WithErrors(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 有错误的网络
	nets := []*NetworkStats{
		{Interface: "eth0", RXPackets: 1000, TXPackets: 1000, RXErrors: 20, TXErrors: 10},
	}

	health := scorer.calculateNetworkScore(nets)
	assert.Equal(t, "warning", health.Status)
	assert.Less(t, health.Score, 100.0)
}

func TestHealthScorer_CalculateSMARTScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 健康 SMART
	smarts := []*SMARTInfo{
		{Device: "/dev/sda", Health: "PASSED", Temperature: 35},
	}

	health := scorer.calculateSMARTScore(smarts, scorer.thresholds)
	assert.Equal(t, "healthy", health.Status)
}

func TestHealthScorer_CalculateSMARTScore_Failed(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 失败 SMART
	smarts := []*SMARTInfo{
		{Device: "/dev/sda", Health: "FAILED", Temperature: 45},
	}

	health := scorer.calculateSMARTScore(smarts, scorer.thresholds)
	assert.Equal(t, "critical", health.Status)
}

func TestHealthScorer_CalculateSMARTScore_HighTemp(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)
	scorer.thresholds.MaxTemperature = 50

	// 高温 SMART
	smarts := []*SMARTInfo{
		{Device: "/dev/sda", Health: "PASSED", Temperature: 65},
	}

	health := scorer.calculateSMARTScore(smarts, scorer.thresholds)
	assert.Less(t, health.Score, 100.0)
}

func TestHealthScorer_CalculateSystemScore(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	// 正常运行时间
	stats := &SystemStats{
		UptimeSeconds: 86400 * 7, // 7 天
	}

	health := scorer.calculateSystemScore(stats, scorer.thresholds)
	assert.Equal(t, "healthy", health.Status)

	// 刚启动
	stats = &SystemStats{
		UptimeSeconds: 100,
	}

	health = scorer.calculateSystemScore(stats, scorer.thresholds)
	assert.Less(t, health.Score, 100.0)
}

// ========== HealthScore 结构测试 ==========

func TestHealthScore_Validation(t *testing.T) {
	score := &HealthScore{
		TotalScore: 85.5,
		Grade:      "B",
		Components: ComponentScores{
			CPU:    ComponentHealth{Score: 90, Status: "healthy"},
			Memory: ComponentHealth{Score: 80, Status: "healthy"},
			Disk:   ComponentHealth{Score: 85, Status: "healthy"},
		},
		Issues:          []HealthIssue{},
		Recommendations: []string{},
		Timestamp:       time.Now(),
	}

	assert.Equal(t, 85.5, score.TotalScore)
	assert.Equal(t, "B", score.Grade)
	assert.NotEmpty(t, score.Components.CPU.Status)
}

func TestHealthIssue_Structure(t *testing.T) {
	issue := HealthIssue{
		Component: "cpu",
		Severity:  "warning",
		Message:   "CPU usage high",
		Value:     85.0,
		Threshold: 80.0,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "cpu", issue.Component)
	assert.Equal(t, "warning", issue.Severity)
	assert.Greater(t, issue.Value, issue.Threshold)
}

func TestScoreTrend_Structure(t *testing.T) {
	trend := ScoreTrend{
		Direction:    "up",
		Change:       5.5,
		PreviousHour: 80.0,
		PreviousDay:  75.0,
	}

	assert.Equal(t, "up", trend.Direction)
	assert.Equal(t, 5.5, trend.Change)
}

// ========== 并发测试 ==========

func TestHealthScorer_ConcurrentCalculate(t *testing.T) {
	mgr, _ := NewManager()
	scorer := NewHealthScorer(mgr)

	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			_ = scorer.CalculateScore()
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}