package predictivefailure

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// 测试默认配置
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager(nil) returned nil")
	}
	if m.config.ScanIntervalMinutes != 60 {
		t.Errorf("expected default ScanIntervalMinutes=60, got %d", m.config.ScanIntervalMinutes)
	}
	if m.config.AlertThreshold != 60 {
		t.Errorf("expected default AlertThreshold=60, got %f", m.config.AlertThreshold)
	}
	if m.running {
		t.Error("new manager should not be running")
	}

	// 测试自定义配置
	cfg := &Config{
		Enabled:             true,
		ScanIntervalMinutes: 30,
		AlertThreshold:      70,
	}
	m2 := NewManager(cfg)
	if m2.config.ScanIntervalMinutes != 30 {
		t.Errorf("expected ScanIntervalMinutes=30, got %d", m2.config.ScanIntervalMinutes)
	}
	if m2.config.AlertThreshold != 70 {
		t.Errorf("expected AlertThreshold=70, got %f", m2.config.AlertThreshold)
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		AlertThreshold:               60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	// 测试启动
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 测试重复启动
	if err := m.Start(); err != ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	// 测试停止
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// 测试重复停止
	if err := m.Stop(); err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// 测试禁用配置启动
	disabledCfg := &Config{Enabled: false}
	m2 := NewManager(disabledCfg)
	if err := m2.Start(); err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig for disabled config, got %v", err)
	}
}

func TestScanDisk(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
	}
	m := NewManager(cfg)

	// 未启动时扫描应返回错误
	_, err := m.ScanDisk("/dev/sda")
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning when not started, got %v", err)
	}

	// 启动后扫描
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	health, err := m.ScanDisk("/dev/sda")
	if err != nil {
		t.Fatalf("ScanDisk failed: %v", err)
	}
	if health == nil {
		t.Fatal("ScanDisk returned nil health")
	}
	if health.Device != "/dev/sda" {
		t.Errorf("expected device=/dev/sda, got %s", health.Device)
	}
	if health.CollectedAt.IsZero() {
		t.Error("CollectedAt should not be zero")
	}

	// 验证磁盘数据已存储
	m.mu.RLock()
	disk, ok := m.disks["/dev/sda"]
	m.mu.RUnlock()
	if !ok {
		t.Error("disk data not stored in manager")
	}
	if disk != nil && disk.Device != "/dev/sda" {
		t.Errorf("stored disk device mismatch: %s", disk.Device)
	}
}

func TestScanMemory(t *testing.T) {
	cfg := &Config{
		Enabled:                    true,
		ScanIntervalMinutes:        60,
		MemoryPercentWarnThreshold: 85,
	}
	m := NewManager(cfg)

	// 未启动时应返回错误
	_, err := m.ScanMemory()
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// 启动后扫描
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	resources, err := m.ScanMemory()
	if err != nil {
		t.Fatalf("ScanMemory failed: %v", err)
	}
	if resources == nil {
		t.Fatal("ScanMemory returned nil")
	}
	if resources.MemoryTotalMB <= 0 {
		t.Errorf("MemoryTotalMB should be positive, got %f", resources.MemoryTotalMB)
	}
	if resources.MemoryUsagePercent < 0 || resources.MemoryUsagePercent > 100 {
		t.Errorf("MemoryUsagePercent out of range: %f", resources.MemoryUsagePercent)
	}
}

func TestScanCPU(t *testing.T) {
	cfg := &Config{
		Enabled:                  true,
		ScanIntervalMinutes:      60,
		CPUPercentWarnThreshold:  80,
		TemperatureWarnThreshold: 50,
	}
	m := NewManager(cfg)

	// 未启动时应返回错误
	_, err := m.ScanCPU()
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// 启动后扫描
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	resources, err := m.ScanCPU()
	if err != nil {
		t.Fatalf("ScanCPU failed: %v", err)
	}
	if resources == nil {
		t.Fatal("ScanCPU returned nil")
	}
	if resources.CPUUsagePercent < 0 {
		t.Errorf("CPUUsagePercent should be non-negative, got %f", resources.CPUUsagePercent)
	}
	if resources.CPUTemperature < 0 {
		t.Errorf("CPUTemperature should be non-negative, got %f", resources.CPUTemperature)
	}
}

func TestPredictFailure(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		AlertThreshold:               60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	// 未启动时应返回错误
	_, err := m.PredictFailure("/dev/sda")
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// 启动并扫描
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 先扫描磁盘以获取数据
	_, err = m.ScanDisk("/dev/sda")
	if err != nil {
		t.Fatalf("ScanDisk failed: %v", err)
	}

	// 预测磁盘故障
	pred, err := m.PredictFailure("/dev/sda")
	if err != nil {
		t.Fatalf("PredictFailure for disk failed: %v", err)
	}
	if pred == nil {
		t.Fatal("PredictFailure returned nil")
	}
	if pred.ComponentType != ComponentDisk {
		t.Errorf("expected ComponentDisk, got %s", pred.ComponentType)
	}
	if pred.ComponentID != "/dev/sda" {
		t.Errorf("expected componentID=/dev/sda, got %s", pred.ComponentID)
	}
	if pred.RiskScore < 0 || pred.RiskScore > 100 {
		t.Errorf("RiskScore out of range: %f", pred.RiskScore)
	}
	if pred.PredictedAt.IsZero() {
		t.Error("PredictedAt should not be zero")
	}

	// 预测内存故障
	_, err = m.ScanMemory()
	if err != nil {
		t.Fatalf("ScanMemory failed: %v", err)
	}

	memPred, err := m.PredictFailure("memory")
	if err != nil {
		t.Fatalf("PredictFailure for memory failed: %v", err)
	}
	if memPred.ComponentType != ComponentMemory {
		t.Errorf("expected ComponentMemory, got %s", memPred.ComponentType)
	}

	// 预测 CPU 故障
	cpuPred, err := m.PredictFailure("cpu")
	if err != nil {
		t.Fatalf("PredictFailure for cpu failed: %v", err)
	}
	if cpuPred.ComponentType != ComponentCPU {
		t.Errorf("expected ComponentCPU, got %s", cpuPred.ComponentType)
	}

	// 不存在的组件
	_, err = m.PredictFailure("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent component")
	}
}

func TestGetDashboard(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		AlertThreshold:               60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	// 未扫描时的仪表盘
	dash := m.GetDashboard()
	if dash == nil {
		t.Fatal("GetDashboard returned nil")
	}
	if dash.ScansTotal != 0 {
		t.Errorf("expected 0 scans, got %d", dash.ScansTotal)
	}

	// 启动并执行扫描
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	_, err := m.ScanDisk("/dev/sda")
	if err != nil {
		t.Fatalf("ScanDisk failed: %v", err)
	}

	// 执行完整扫描
	result, err := m.RunFullScan()
	if err != nil {
		t.Fatalf("RunFullScan failed: %v", err)
	}
	if result == nil {
		t.Fatal("RunFullScan returned nil")
	}

	// 检查仪表盘
	dash = m.GetDashboard()
	if dash.ScansTotal < 1 {
		t.Errorf("expected at least 1 scan, got %d", dash.ScansTotal)
	}
	if dash.LastScanTime.IsZero() {
		t.Error("LastScanTime should not be zero after scan")
	}
	if dash.TotalPredictions < 1 {
		t.Errorf("expected at least 1 prediction, got %d", dash.TotalPredictions)
	}
}

func TestListPredictions(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		AlertThreshold:               60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 初始应为空
	preds := m.ListPredictions()
	if len(preds) != 0 {
		t.Errorf("expected 0 predictions initially, got %d", len(preds))
	}

	// 执行扫描后应有预测
	_, _ = m.ScanDisk("/dev/sda")
	_, _ = m.ScanMemory()
	result, err := m.RunFullScan()
	if err != nil {
		t.Fatalf("RunFullScan failed: %v", err)
	}

	preds = m.ListPredictions()
	// RunFullScan 会产生磁盘 + 内存 + CPU 预测
	expectedMin := len(result.DiskPredictions)
	if mem := result.MemoryPrediction; mem != nil {
		expectedMin++
	}
	if cpu := result.CPUPrediction; cpu != nil {
		expectedMin++
	}
	if len(preds) < expectedMin {
		t.Errorf("expected at least %d predictions, got %d", expectedMin, len(preds))
	}
}

func TestGetMaintenanceSuggestions(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		TemperatureWarnThreshold:     50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 无数据时建议应为空
	suggestions := m.GetMaintenanceSuggestions()
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions with no data, got %d", len(suggestions))
	}

	// 扫描后获取建议
	_, _ = m.ScanDisk("/dev/sda")
	_, _ = m.ScanMemory()
	suggestions = m.GetMaintenanceSuggestions()
	// 建议数量取决于模拟数据，但不应 panic
	t.Logf("Got %d maintenance suggestions", len(suggestions))
	for _, s := range suggestions {
		if s.Title == "" {
			t.Error("suggestion should have non-empty title")
		}
		if s.Priority < 1 || s.Priority > 5 {
			t.Errorf("suggestion priority out of range: %d", s.Priority)
		}
	}
}

func TestRunFullScan(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ScanIntervalMinutes:          60,
		AlertThreshold:               30,
		TemperatureWarnThreshold:     35,
		TemperatureCriticalThreshold: 45,
		CPUPercentWarnThreshold:      80,
		MemoryPercentWarnThreshold:   85,
	}
	m := NewManager(cfg)

	// 未启动时应返回错误
	_, err := m.RunFullScan()
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 第一次完整扫描
	result, err := m.RunFullScan()
	if err != nil {
		t.Fatalf("RunFullScan failed: %v", err)
	}
	if result.ID == "" {
		t.Error("scan result should have ID")
	}
	if result.ScanTime.IsZero() {
		t.Error("ScanTime should not be zero")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if len(result.DiskPredictions) == 0 {
		t.Error("should have at least one disk prediction")
	}
	if result.MemoryPrediction == nil {
		t.Error("should have memory prediction")
	}
	if result.CPUPrediction == nil {
		t.Error("should have CPU prediction")
	}
	// OverallRiskLevel 应该是有效的
	switch result.OverallRiskLevel {
	case RiskCritical, RiskHigh, RiskMedium, RiskLow:
		// valid
	default:
		t.Errorf("invalid OverallRiskLevel: %s", result.OverallRiskLevel)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cfg := &Config{
		Enabled:                  true,
		ScanIntervalMinutes:      60,
		TemperatureWarnThreshold: 50,
	}
	m := NewManager(cfg)

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 并发扫描和查询
	done := make(chan bool, 10)
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			device := "/dev/sd" + string(rune('a'+id))
			_, _ = m.ScanDisk(device)
			_, _ = m.PredictFailure(device)
		}(i)
	}
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			_ = m.GetDashboard()
			_ = m.ListPredictions()
			_ = m.GetMaintenanceSuggestions()
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
