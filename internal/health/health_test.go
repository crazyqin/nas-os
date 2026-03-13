// Package health 提供系统健康检查功能
package health

import (
	"context"
	"testing"
	"time"
)

func TestNewHealthManager(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
	if manager == nil {
		t.Fatal("NewHealthManager returned nil")
	}

	if manager.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", manager.timeout)
	}
}

func TestHealthManager_RegisterChecker(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)

	manager.RegisterChecker(checker)

	if len(manager.checkers) != 1 {
		t.Errorf("Expected 1 checker, got %d", len(manager.checkers))
	}
}

func TestHealthManager_RemoveChecker(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
	checker := NewMemoryChecker(80.0)

	manager.RegisterChecker(checker)
	manager.RemoveChecker("memory")

	if len(manager.checkers) != 0 {
		t.Errorf("Expected 0 checkers, got %d", len(manager.checkers))
	}
}

func TestHealthManager_RunCheck(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
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
}

func TestHealthManager_RunCheck_NotFound(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)

	ctx := context.Background()
	_, err := manager.RunCheck(ctx, "nonexistent")

	if err == nil {
		t.Error("Expected error for nonexistent checker")
	}
}

func TestHealthManager_RunAllChecks(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
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
}

func TestDiskSpaceChecker(t *testing.T) {
	checker := NewDiskSpaceChecker("disk-root", 80.0)

	if checker.Name() != "disk-root" {
		t.Errorf("Expected name 'disk-root', got %s", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}
}

func TestHealthManager_IsHealthy(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	// 内存检查通常会返回健康状态
	healthy := manager.IsHealthy()
	// 不强制要求 true，只验证方法能正常执行
	t.Logf("IsHealthy: %v", healthy)
}

func TestHealthReport_Summary(t *testing.T) {
	manager := NewHealthManager(5 * time.Second)
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

func TestHealthChecker_ContextCancellation(t *testing.T) {
	manager := NewHealthManager(1 * time.Nanosecond)
	manager.RegisterChecker(NewMemoryChecker(80.0))

	// 极短超时可能导致检查失败，但不会 panic
	ctx := context.Background()
	_ = manager.RunAllChecks(ctx)
}
