// Package health 提供系统健康检查功能
package health

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(5 * time.Second)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", manager.timeout)
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		Timeout:       3 * time.Second,
		CheckInterval: 30 * time.Second,
		Version:       "1.0.0",
	}
	manager := NewManagerWithConfig(config)
	if manager == nil {
		t.Fatal("NewManagerWithConfig returned nil")
	}

	if manager.timeout != 3*time.Second {
		t.Errorf("Expected timeout 3s, got %v", manager.timeout)
	}
}

func TestManager_RegisterChecker(t *testing.T) {
	manager := NewManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)

	manager.RegisterChecker(checker)

	if len(manager.checkers) != 1 {
		t.Errorf("Expected 1 checker, got %d", len(manager.checkers))
	}
}

func TestManager_RemoveChecker(t *testing.T) {
	manager := NewManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)

	manager.RegisterChecker(checker)
	manager.RemoveChecker("memory")

	if len(manager.checkers) != 0 {
		t.Errorf("Expected 0 checkers, got %d", len(manager.checkers))
	}
}

func TestManager_GetChecker(t *testing.T) {
	manager := NewManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)
	manager.RegisterChecker(checker)

	got, exists := manager.GetChecker("memory")
	if !exists {
		t.Error("Expected checker to exist")
	}
	if got == nil {
		t.Error("Expected non-nil checker")
	}
}

func TestManager_ListCheckers(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))
	manager.RegisterChecker(NewDiskSpaceChecker("disk", 80.0))

	list := manager.ListCheckers()
	if len(list) != 2 {
		t.Errorf("Expected 2 checkers, got %d", len(list))
	}
}

func TestManager_RunCheck(t *testing.T) {
	manager := NewManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)
	manager.RegisterChecker(checker)

	ctx := context.Background()
	result, err := manager.RunCheck(ctx, "memory")

	if err != nil {
		t.Fatalf("RunCheck failed: %v", err)
	}

	if result.Name != "memory" {
		t.Errorf("Expected name 'memory', got %s", result.Name)
	}

	if result.Status == "" {
		t.Error("Status should not be empty")
	}

	if result.Duration == 0 {
		t.Error("Duration should be recorded")
	}
}

func TestManager_RunCheck_NotFound(t *testing.T) {
	manager := NewManager(5 * time.Second)

	ctx := context.Background()
	_, err := manager.RunCheck(ctx, "nonexistent")

	if err == nil {
		t.Error("Expected error for nonexistent checker")
	}
}

func TestManager_RunAllChecks(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	ctx := context.Background()
	report := manager.RunAllChecks(ctx)

	if report == nil {
		t.Fatal("RunAllChecks returned nil")
	}

	if report.Summary.Total != 1 {
		t.Errorf("Expected 1 total check, got %d", report.Summary.Total)
	}

	if report.Status == "" {
		t.Error("Status should not be empty")
	}

	if report.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestManager_GetLastResult(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	ctx := context.Background()
	manager.RunCheck(ctx, "memory")

	result, exists := manager.GetLastResult("memory")
	if !exists {
		t.Fatal("Expected result to exist")
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Name != "memory" {
		t.Errorf("Expected name 'memory', got %s", result.Name)
	}
}

func TestManager_GetAllLastResults(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))
	manager.RegisterChecker(NewDiskSpaceChecker("disk", 80.0))

	ctx := context.Background()
	manager.RunAllChecks(ctx)

	results := manager.GetAllLastResults()
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestMemoryChecker(t *testing.T) {
	checker := NewMemoryChecker(80.0)

	if checker.Name() != "memory" {
		t.Errorf("Expected name 'memory', got %s", checker.Name())
	}

	if checker.Type() != CheckTypeMemory {
		t.Errorf("Expected type 'memory', got %s", checker.Type())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status == "" {
		t.Error("Status should not be empty")
	}

	if _, exists := result.Details["alloc_mb"]; !exists {
		t.Error("Expected alloc_mb in details")
	}

	if _, exists := result.Details["heap_alloc_mb"]; !exists {
		t.Error("Expected heap_alloc_mb in details")
	}
}

func TestMemoryChecker_WithName(t *testing.T) {
	checker := NewMemoryCheckerWithName("custom-memory", 90.0)

	if checker.Name() != "custom-memory" {
		t.Errorf("Expected name 'custom-memory', got %s", checker.Name())
	}
}

func TestDiskSpaceChecker(t *testing.T) {
	checker := NewDiskSpaceChecker("disk-root", 80.0)

	if checker.Name() != "disk-root" {
		t.Errorf("Expected name 'disk-root', got %s", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	// 磁盘检查应该返回状态
	if result.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestDiskSpaceChecker_WithPath(t *testing.T) {
	checker := NewDiskSpaceCheckerWithPath("disk-tmp", "/tmp", 80.0)

	if checker.Name() != "disk-tmp" {
		t.Errorf("Expected name 'disk-tmp', got %s", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Details["path"] != "/tmp" {
		t.Errorf("Expected path '/tmp', got %v", result.Details["path"])
	}
}

func TestHTTPChecker(t *testing.T) {
	// 使用一个不太可能存在的地址测试失败情况
	checker := NewHTTPChecker("http-test", "http://127.0.0.1:9999/health", 1*time.Second, 200)

	ctx := context.Background()
	result := checker.Check(ctx)

	// 应该失败
	if result.Status != StatusUnhealthy {
		t.Logf("HTTP check to non-existent endpoint returned: %s", result.Status)
	}
}

func TestHTTPChecker_SetHeaders(t *testing.T) {
	checker := NewHTTPChecker("http-with-headers", "http://example.com", 1*time.Second, 200)
	checker.SetHeaders(map[string]string{
		"X-Custom-Header": "test-value",
	})

	if len(checker.headers) != 1 {
		t.Errorf("Expected 1 header, got %d", len(checker.headers))
	}
}

func TestDatabaseChecker(t *testing.T) {
	// 无数据库连接测试
	checker := NewDatabaseChecker("db-test", nil, 5*time.Second)

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusUnhealthy {
		t.Error("Expected unhealthy status for nil database")
	}
	if result.Message != "Database connection is nil" {
		t.Errorf("Unexpected message: %s", result.Message)
	}
}

func TestDatabaseChecker_SetQuery(t *testing.T) {
	checker := NewDatabaseChecker("db-test", nil, 5*time.Second)
	checker.SetQuery("SELECT 1 FROM dual")

	if checker.query != "SELECT 1 FROM dual" {
		t.Errorf("Query not set correctly")
	}
}

func TestNetworkChecker(t *testing.T) {
	// 测试连接到一个不太可能开放的端口
	checker := NewNetworkChecker("network-test", "127.0.0.1:9999", 1*time.Second)

	ctx := context.Background()
	result := checker.Check(ctx)

	// 应该失败
	if result.Status != StatusUnhealthy {
		t.Logf("Network check to non-existent port returned: %s", result.Status)
	}
}

func TestDNSChecker(t *testing.T) {
	checker := NewDNSChecker("dns-test", "localhost", 5*time.Second)

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Logf("DNS check returned: %s, message: %s", result.Status, result.Message)
	}
}

func TestTCPChecker(t *testing.T) {
	checker := NewTCPChecker("tcp-test", "127.0.0.1:9999", 1*time.Second)

	ctx := context.Background()
	result := checker.Check(ctx)

	// 应该失败（端口不存在）
	if result.Status != StatusUnhealthy {
		t.Logf("TCP check to non-existent port returned: %s", result.Status)
	}
}

func TestFileChecker(t *testing.T) {
	// 测试已存在的文件
	checker := NewFileChecker("file-test", "/etc/hosts")

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status for /etc/hosts, got %s", result.Status)
	}
}

func TestFileChecker_NotExist(t *testing.T) {
	checker := NewFileChecker("file-notexist", "/nonexistent/path/to/file")

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nonexistent file, got %s", result.Status)
	}
}

func TestDirChecker(t *testing.T) {
	// 测试已存在的目录
	checker := NewDirChecker("dir-test", "/tmp")

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status for /tmp, got %s", result.Status)
	}
}

func TestProcessChecker(t *testing.T) {
	// 测试当前进程 (PID 1 在容器中通常存在)
	checker := NewProcessChecker("process-test", 1)

	ctx := context.Background()
	result := checker.Check(ctx)

	// 结果取决于是否有权限检查进程
	t.Logf("Process check for PID 1: %s - %s", result.Status, result.Message)
}

func TestCustomChecker(t *testing.T) {
	checker := NewCustomChecker("custom-test", func(ctx context.Context) (Status, string, map[string]interface{}) {
		return StatusHealthy, "custom check passed", map[string]interface{}{
			"custom_value": 123,
		}
	})

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", result.Status)
	}
	if result.Message != "custom check passed" {
		t.Errorf("Unexpected message: %s", result.Message)
	}
	if result.Details["custom_value"] != 123 {
		t.Errorf("Expected custom_value 123, got %v", result.Details["custom_value"])
	}
}

func TestCustomChecker_Nil(t *testing.T) {
	checker := NewCustomChecker("custom-nil", nil)

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy for nil check function, got %s", result.Status)
	}
}

func TestManager_IsHealthy(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	healthy := manager.IsHealthy()
	t.Logf("IsHealthy: %v", healthy)
}

func TestReport_Summary(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))
	manager.RegisterChecker(NewDiskSpaceChecker("disk", 80.0))

	ctx := context.Background()
	report := manager.RunAllChecks(ctx)

	if report.Summary.Total != 2 {
		t.Errorf("Expected 2 total checks, got %d", report.Summary.Total)
	}

	// 验证摘要计算
	if report.Summary.Healthy+report.Summary.Unhealthy+report.Summary.Degraded != report.Summary.Total {
		t.Error("Summary counts should add up to total")
	}
}

func TestManager_GenerateReport(t *testing.T) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	ctx := context.Background()
	report := manager.GenerateReport(ctx, "v2.6.0")

	if report.Version != "v2.6.0" {
		t.Errorf("Expected version v2.6.0, got %s", report.Version)
	}

	if report.Uptime == 0 {
		t.Error("Expected uptime to be set")
	}
}

func TestChecker_ContextCancellation(t *testing.T) {
	manager := NewManager(1 * time.Nanosecond)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	// 极短超时可能导致检查失败，但不会 panic
	ctx := context.Background()
	_ = manager.RunAllChecks(ctx)
}

func TestStatus_String(t *testing.T) {
	statuses := []Status{StatusHealthy, StatusUnhealthy, StatusDegraded}
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Status should have string representation")
		}
	}
}

func TestCheckType_String(t *testing.T) {
	types := []CheckType{CheckTypeDatabase, CheckTypeStorage, CheckTypeMemory, CheckTypeNetwork, CheckTypeService, CheckTypeCustom}
	for _, ct := range types {
		if string(ct) == "" {
			t.Errorf("CheckType should have string representation")
		}
	}
}

// 集成测试.
func TestManager_Integration(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// 注册多种检查器
	manager.RegisterChecker(NewMemoryChecker(90.0))
	manager.RegisterChecker(NewDiskSpaceChecker("disk-root", 95.0))
	manager.RegisterChecker(NewDNSChecker("dns-localhost", "localhost", 5*time.Second))
	manager.RegisterChecker(NewFileChecker("file-hosts", "/etc/hosts"))
	manager.RegisterChecker(NewDirChecker("dir-tmp", "/tmp"))

	// 添加自定义检查器
	manager.RegisterChecker(NewCustomChecker("app-health", func(ctx context.Context) (Status, string, map[string]interface{}) {
		return StatusHealthy, "application is running", map[string]interface{}{
			"uptime_seconds": 3600,
		}
	}))

	ctx := context.Background()
	report := manager.RunAllChecks(ctx)

	t.Logf("Health Report:")
	t.Logf("  Status: %s", report.Status)
	t.Logf("  Total: %d", report.Summary.Total)
	t.Logf("  Healthy: %d", report.Summary.Healthy)
	t.Logf("  Unhealthy: %d", report.Summary.Unhealthy)
	t.Logf("  Degraded: %d", report.Summary.Degraded)

	for name, result := range report.Checks {
		t.Logf("  [%s] %s: %s - %s", result.Type, name, result.Status, result.Message)
	}

	// 大多数检查应该通过
	if report.Summary.Unhealthy > report.Summary.Total/2 {
		t.Error("Too many unhealthy checks in integration test")
	}
}

// Benchmark 测试.
func BenchmarkMemoryChecker_Check(b *testing.B) {
	checker := NewMemoryChecker(80.0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.Check(ctx)
	}
}

func BenchmarkManager_RunAllChecks(b *testing.B) {
	manager := NewManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))
	manager.RegisterChecker(NewDiskSpaceChecker("disk", 80.0))
	manager.RegisterChecker(NewDNSChecker("dns", "localhost", 5*time.Second))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.RunAllChecks(ctx)
	}
}
