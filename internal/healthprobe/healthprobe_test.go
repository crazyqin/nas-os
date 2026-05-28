package healthprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultConfig()

	manager := NewManager(logger, config)
	assert.NotNil(t, manager)
	assert.False(t, manager.IsRunning())
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Second, config.Interval)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 1440, config.HistorySize)
	assert.Equal(t, 5*time.Minute, config.AlertCooldown)
	assert.True(t, config.EnableTrend)
	assert.Equal(t, 10, config.TrendWindow)
	assert.False(t, config.AutoStart)
}

// TestRegisterProbe 测试注册探针
func TestRegisterProbe(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})

	manager.RegisterProbe(probe)
	probes := manager.GetProbes()
	assert.Len(t, probes, 1)
	assert.Contains(t, probes, "test-probe")
}

// TestUnregisterProbe 测试注销探针
func TestUnregisterProbe(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})

	manager.RegisterProbe(probe)
	assert.Len(t, manager.GetProbes(), 1)

	manager.UnregisterProbe("test-probe")
	assert.Len(t, manager.GetProbes(), 0)
}

// TestAddRule 测试添加规则
func TestAddRule(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	rule := &Rule{
		Name:      "cpu-high",
		Type:      MetricCPU,
		Threshold: 90,
		Level:     LevelCritical,
		Operator:  "gt",
		Weight:    1.0,
		Message:   "CPU 使用率过高",
		Enabled:   true,
	}

	manager.AddRule(rule)
	rules := manager.GetRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "cpu-high", rules[0].Name)
}

// TestCheck 测试健康检查
func TestCheck(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册健康探针
	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "CPU 使用率正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 执行检查
	status := manager.Check(context.Background())
	require.NotNil(t, status)
	assert.Equal(t, LevelHealthy, status.Level)
	assert.Equal(t, 100.0, status.Score)
	assert.Len(t, status.Probes, 1)
	assert.Equal(t, 1, status.Summary.Total)
	assert.Equal(t, 1, status.Summary.Healthy)
}

// TestCheckWithCriticalProbe 测试严重状态探针
func TestCheckWithCriticalProbe(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册严重状态探针
	probe := NewProbeFunc("critical-probe", MetricDisk, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "critical-probe",
			Type:      MetricDisk,
			Category:  CategoryHardware,
			Level:     LevelCritical,
			Value:     95.0,
			Unit:      "%",
			Message:   "磁盘空间严重不足",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 执行检查
	status := manager.Check(context.Background())
	require.NotNil(t, status)
	assert.Equal(t, LevelCritical, status.Level)
	assert.Equal(t, 0.0, status.Score)
	assert.Equal(t, 1, status.Summary.Critical)
}

// TestCheckWithMixedProbes 测试混合状态探针
func TestCheckWithMixedProbes(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册健康探针
	healthyProbe := NewProbeFunc("healthy-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "healthy-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(healthyProbe)

	// 注册降级探针
	degradedProbe := NewProbeFunc("degraded-probe", MetricMemory, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "degraded-probe",
			Type:      MetricMemory,
			Category:  CategoryHardware,
			Level:     LevelDegraded,
			Value:     85.0,
			Unit:      "%",
			Message:   "内存使用率偏高",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(degradedProbe)

	// 执行检查
	status := manager.Check(context.Background())
	require.NotNil(t, status)
	assert.Equal(t, LevelDegraded, status.Level)
	assert.Equal(t, 1, status.Summary.Healthy)
	assert.Equal(t, 1, status.Summary.Degraded)
}

// TestTrendAnalysis 测试趋势分析
func TestTrendAnalysis(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultConfig()
	config.EnableTrend = true
	config.TrendWindow = 5
	manager := NewManager(logger, config)

	// 注册探针
	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 多次执行检查建立历史
	for i := 0; i < 5; i++ {
		manager.Check(context.Background())
		time.Sleep(10 * time.Millisecond)
	}

	// 获取最新状态
	status := manager.GetStatus()
	require.NotNil(t, status)
	require.NotNil(t, status.Trend)
	assert.Equal(t, "stable", status.Trend.Direction)
}

// TestAlertGeneration 测试告警生成
func TestAlertGeneration(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultConfig()
	config.AlertCooldown = 0 // 禁用冷却期以便测试
	manager := NewManager(logger, config)

	// 注册告警通知器
	var receivedAlerts []*Alert
	manager.AddNotifier(NotifierFunc(func(ctx context.Context, alert *Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}))

	// 注册严重探针
	probe := NewProbeFunc("critical-probe", MetricDisk, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "critical-probe",
			Type:      MetricDisk,
			Category:  CategoryHardware,
			Level:     LevelCritical,
			Value:     95.0,
			Unit:      "%",
			Message:   "磁盘空间严重不足",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 执行检查
	manager.Check(context.Background())

	// 等待异步通知
	time.Sleep(100 * time.Millisecond)

	// 验证告警
	alerts := manager.GetAlerts(0, false)
	assert.Len(t, alerts, 1)
	assert.Equal(t, LevelCritical, alerts[0].Level)
	assert.Equal(t, "critical-probe", alerts[0].Probe)
}

// TestResolveAlert 测试解决告警
func TestResolveAlert(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册探针
	probe := NewProbeFunc("test-probe", MetricDisk, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricDisk,
			Category:  CategoryHardware,
			Level:     LevelCritical,
			Value:     95.0,
			Unit:      "%",
			Message:   "磁盘空间严重不足",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 执行检查生成告警
	manager.Check(context.Background())

	// 获取告警
	alerts := manager.GetAlerts(0, false)
	require.Len(t, alerts, 1)

	// 解决告警
	err := manager.ResolveAlert(alerts[0].ID)
	assert.NoError(t, err)

	// 验证告警已解决（默认不显示已解决）
	unresolvedAlerts := manager.GetAlerts(0, false)
	assert.Len(t, unresolvedAlerts, 0)

	// 验证可以显示已解决告警
	allAlerts := manager.GetAlerts(0, true)
	assert.Len(t, allAlerts, 1)
	assert.True(t, allAlerts[0].Resolved)
}

// TestGenerateReport 测试生成报告
func TestGenerateReport(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册探针
	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 生成报告
	report := manager.GenerateReport()
	require.NotNil(t, report)
	assert.Equal(t, LevelHealthy, report.Level)
	assert.Equal(t, 100.0, report.Score)
	assert.NotNil(t, report.Summary)
	assert.Len(t, report.Recommendations, 1)
}

// TestStartStop 测试启动停止
func TestStartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultConfig()
	config.Interval = 100 * time.Millisecond
	manager := NewManager(logger, config)

	// 注册探针
	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 启动
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	assert.True(t, manager.IsRunning())

	// 等待至少一次检查
	time.Sleep(200 * time.Millisecond)

	// 验证有数据
	status := manager.GetStatus()
	assert.NotNil(t, status)

	// 停止
	manager.Stop()
	assert.False(t, manager.IsRunning())
}

// TestGetHistory 测试获取历史
func TestGetHistory(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger, nil)

	// 注册探针
	probe := NewProbeFunc("test-probe", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test-probe",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})
	manager.RegisterProbe(probe)

	// 多次执行检查
	for i := 0; i < 5; i++ {
		manager.Check(context.Background())
	}

	// 获取历史
	history := manager.GetHistory(3)
	assert.Len(t, history, 3)
}

// TestProbeFunc 测试函数式探针
func TestProbeFunc(t *testing.T) {
	probe := NewProbeFunc("test", MetricCPU, CategoryHardware, func(ctx context.Context) (*ProbeResult, error) {
		return &ProbeResult{
			Name:      "test",
			Type:      MetricCPU,
			Category:  CategoryHardware,
			Level:     LevelHealthy,
			Value:     50.0,
			Unit:      "%",
			Message:   "正常",
			Timestamp: time.Now(),
		}, nil
	})

	assert.Equal(t, "test", probe.Name())
	assert.Equal(t, MetricCPU, probe.Type())
	assert.Equal(t, CategoryHardware, probe.Category())

	result, err := probe.Collect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, LevelHealthy, result.Level)
}

// TestNotifierFunc 测试函数式通知器
func TestNotifierFunc(t *testing.T) {
	var received bool
	notifier := NotifierFunc(func(ctx context.Context, alert *Alert) error {
		received = true
		return nil
	})

	alert := &Alert{
		ID:        "test-alert",
		Probe:     "test-probe",
		Severity:  SeverityWarning,
		Level:     LevelDegraded,
		Message:   "测试告警",
		Timestamp: time.Now(),
	}

	err := notifier.Notify(context.Background(), alert)
	assert.NoError(t, err)
	assert.True(t, received)
}
