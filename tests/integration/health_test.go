// Package integration 提供 NAS-OS 集成测试
// 健康检查集成测试
package integration

import (
	"context"
	"testing"
	"time"

	"nas-os/internal/health"
)

func TestHealth_Integration(t *testing.T) {
	manager := health.NewHealthManager(5 * time.Second)

	t.Run("RegisterAndRunChecks", func(t *testing.T) {
		// 注册多个检查器
		manager.RegisterChecker(health.NewMemoryChecker(80.0))
		manager.RegisterChecker(health.NewDiskSpaceChecker("disk-root", 80.0))

		ctx := context.Background()
		report := manager.RunAllChecks(ctx)

		if report == nil {
			t.Fatal("RunAllChecks returned nil")
		}

		if report.Summary.Total != 2 {
			t.Errorf("Expected 2 checks, got %d", report.Summary.Total)
		}
	})

	t.Run("IsHealthy", func(t *testing.T) {
		// 检查健康状态
		healthy := manager.IsHealthy()
		t.Logf("System healthy: %v", healthy)
	})

	t.Run("GetLastResult", func(t *testing.T) {
		ctx := context.Background()
		manager.RunCheck(ctx, "memory")

		result, exists := manager.GetLastResult("memory")
		if !exists {
			t.Fatal("GetLastResult returned not exists")
		}

		if result.Name != "memory" {
			t.Errorf("Expected name 'memory', got %s", result.Name)
		}
	})
}

func TestHealth_MemoryChecker(t *testing.T) {
	checker := health.NewMemoryChecker(80.0)

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status == "" {
		t.Error("Status should not be empty")
	}

	if result.Message == "" {
		t.Error("Message should not be empty")
	}

	t.Logf("Memory check: %s - %s", result.Status, result.Message)
	t.Logf("Details: %v", result.Details)
}

func TestHealth_DiskSpaceChecker(t *testing.T) {
	checker := health.NewDiskSpaceChecker("disk-test", 80.0)

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status == "" {
		t.Error("Status should not be empty")
	}

	t.Logf("Disk check: %s - %s", result.Status, result.Message)
}

func TestHealth_ConcurrentChecks(t *testing.T) {
	manager := health.NewHealthManager(5 * time.Second)

	// 注册多个检查器
	for i := 0; i < 10; i++ {
		manager.RegisterChecker(health.NewDiskSpaceChecker("disk-"+string(rune('0'+i)), 80.0))
	}

	ctx := context.Background()
	report := manager.RunAllChecks(ctx)

	if report.Summary.Total != 10 {
		t.Errorf("Expected 10 checks, got %d", report.Summary.Total)
	}

	t.Logf("Concurrent check summary: Total=%d, Healthy=%d, Unhealthy=%d, Degraded=%d",
		report.Summary.Total, report.Summary.Healthy, report.Summary.Unhealthy, report.Summary.Degraded)
}

func TestHealth_Timeout(t *testing.T) {
	// 使用很短的超时
	manager := health.NewHealthManager(1 * time.Nanosecond)
	manager.RegisterChecker(health.NewMemoryChecker(80.0))

	ctx := context.Background()
	report := manager.RunAllChecks(ctx)

	// 即使超时，也应该返回报告
	if report == nil {
		t.Error("Should return report even with timeout")
	}
}

// 性能测试
func BenchmarkHealth_RunAllChecks(b *testing.B) {
	manager := health.NewHealthManager(5 * time.Second)
	manager.RegisterChecker(health.NewMemoryChecker(80.0))
	manager.RegisterChecker(health.NewDiskSpaceChecker("disk", 80.0))

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		manager.RunAllChecks(ctx)
	}
}
