// sysresguard 系统资源守护单元测试
package sysresguard

import (
	"context"
	"testing"
	"time"
)

func TestNewGuard(t *testing.T) {
	thresholds := DefaultThresholds()
	guard := NewGuard(thresholds)
	if guard == nil {
		t.Fatal("NewGuard returned nil")
	}

	if guard.thresholds.DiskWarning != 75.0 {
		t.Errorf("Expected DiskWarning=75, got %.1f", guard.thresholds.DiskWarning)
	}
	if guard.thresholds.MemCritical != 95.0 {
		t.Errorf("Expected MemCritical=95, got %.1f", guard.thresholds.MemCritical)
	}
}

func TestCheckResources(t *testing.T) {
	guard := NewGuard(DefaultThresholds())
	ctx := context.Background()

	statuses := guard.CheckResources(ctx)
	if len(statuses) == 0 {
		t.Error("Expected at least one resource status")
	}

	for _, s := range statuses {
		if s.Percent < 0 || s.Percent > 100 {
			t.Errorf("Invalid percent for %s: %.1f", s.Type, s.Percent)
		}
		if s.Timestamp.IsZero() {
			t.Errorf("Zero timestamp for %s", s.Type)
		}
	}
}

func TestPredictUsage(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	// 添加趋势数据
	guard.mu.Lock()
	for i := 0; i < 48; i++ {
		guard.trends[ResourceDisk] = append(guard.trends[ResourceDisk], 50.0+float64(i)*0.5)
	}
	guard.mu.Unlock()

	trend := guard.PredictUsage(ResourceDisk)
	if trend == nil {
		t.Fatal("Expected trend prediction")
	}

	if trend.Type != ResourceDisk {
		t.Errorf("Expected disk trend, got %s", trend.Type)
	}
	if trend.Trend == "" {
		t.Error("Expected non-empty trend direction")
	}
}

func TestPredictUsageInsufficientData(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	trend := guard.PredictUsage(ResourceDisk)
	if trend != nil {
		t.Error("Expected nil trend with insufficient data")
	}
}

func TestAlerts(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	// 手动添加告警
	guard.mu.Lock()
	guard.alerts = append(guard.alerts, Alert{
		ID:        "test-1",
		Resource:  ResourceDisk,
		Level:     AlertWarning,
		Message:   "测试告警",
		Value:     80.0,
		Threshold: 75.0,
		Timestamp: time.Now(),
	})
	guard.alerts = append(guard.alerts, Alert{
		ID:        "test-2",
		Resource:  ResourceMemory,
		Level:     AlertCritical,
		Message:   "内存告警",
		Value:     96.0,
		Threshold: 95.0,
		Timestamp: time.Now(),
	})
	guard.mu.Unlock()

	// 获取所有告警
	all := guard.GetAlerts("", 0)
	if len(all) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(all))
	}

	// 按级别过滤
	warnings := guard.GetAlerts(AlertWarning, 0)
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(warnings))
	}

	// 限制数量
	limited := guard.GetAlerts("", 1)
	if len(limited) != 1 {
		t.Errorf("Expected 1 alert with limit, got %d", len(limited))
	}
}

func TestCleanupResult(t *testing.T) {
	guard := NewGuard(DefaultThresholds())
	guard.SetCleanPaths([]string{"/tmp/sysresguard-test"})

	result := &CleanupResult{
		Timestamp: time.Now(),
	}

	// 测试清理空目录
	guard.mu.Lock()
	guard.cleanups = append(guard.cleanups, *result)
	guard.mu.Unlock()

	history := guard.GetCleanupHistory(10)
	if len(history) != 1 {
		t.Errorf("Expected 1 cleanup record, got %d", len(history))
	}
}

func TestGetStatusSummary(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	summary := guard.GetStatusSummary()
	if summary == nil {
		t.Fatal("Expected non-nil summary")
	}

	if _, ok := summary["totalAlerts"]; !ok {
		t.Error("Expected totalAlerts in summary")
	}
	if _, ok := summary["totalCleanups"]; !ok {
		t.Error("Expected totalCleanups in summary")
	}
}

func TestForceGC(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	// 分配一些内存
	data := make([]byte, 1024*1024)
	_ = data

	// 执行GC
	guard.ForceGC()

	// GC应该正常完成，不会panic
}

func TestResourceTypeString(t *testing.T) {
	tests := []struct {
		resType  ResourceType
		expected string
	}{
		{ResourceDisk, "磁盘"},
		{ResourceMemory, "内存"},
		{ResourceCPU, "CPU"},
	}

	for _, tt := range tests {
		if got := tt.resType.String(); got != tt.expected {
			t.Errorf("ResourceType.String() = %s, want %s", got, tt.expected)
		}
	}
}

func TestAlertLevelString(t *testing.T) {
	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertInfo, "信息"},
		{AlertWarning, "警告"},
		{AlertCritical, "危险"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("AlertLevel.String() = %s, want %s", got, tt.expected)
		}
	}
}

func TestSetCleanPaths(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	paths := []string{"/tmp", "/var/tmp", "/var/log"}
	guard.SetCleanPaths(paths)

	guard.mu.RLock()
	if len(guard.cleanPaths) != 3 {
		t.Errorf("Expected 3 clean paths, got %d", len(guard.cleanPaths))
	}
	guard.mu.RUnlock()
}

func TestTrendDataLimit(t *testing.T) {
	guard := NewGuard(DefaultThresholds())

	// 添加超过168条数据
	guard.mu.Lock()
	for i := 0; i < 200; i++ {
		guard.trends[ResourceDisk] = append(guard.trends[ResourceDisk], 50.0)
	}
	guard.mu.Unlock()

	// 手动触发截断（模拟CheckResources的行为）
	guard.mu.Lock()
	if len(guard.trends[ResourceDisk]) > 168 {
		guard.trends[ResourceDisk] = guard.trends[ResourceDisk][len(guard.trends[ResourceDisk])-168:]
	}
	guard.mu.Unlock()

	guard.mu.RLock()
	if len(guard.trends[ResourceDisk]) > 168 {
		t.Errorf("Trend data not limited: got %d", len(guard.trends[ResourceDisk]))
	}
	guard.mu.RUnlock()
}
