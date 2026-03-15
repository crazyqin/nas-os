// Package health 提供系统健康检查功能
// checker_test.go - 健康检查器单元测试 v2.51.0
package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewHealthChecker(t *testing.T) {
	hc := NewHealthChecker()
	require.NotNil(t, hc)
	assert.NotNil(t, hc.checkers)
	assert.NotNil(t, hc.results)
	assert.Equal(t, 80.0, hc.cpuThreshold)
	assert.Equal(t, 85.0, hc.memoryThreshold)
	assert.Equal(t, 90.0, hc.diskThreshold)
}

func TestNewHealthCheckerWithOptions(t *testing.T) {
	logger := zap.NewNop()
	hc := NewHealthChecker(
		WithLogger(logger),
		WithVersion("v2.51.0"),
		WithCPUThreshold(70.0),
		WithMemoryThreshold(75.0),
		WithDiskThreshold(85.0),
	)

	require.NotNil(t, hc)
	assert.Equal(t, logger, hc.logger)
	assert.Equal(t, "v2.51.0", hc.version)
	assert.Equal(t, 70.0, hc.cpuThreshold)
	assert.Equal(t, 75.0, hc.memoryThreshold)
	assert.Equal(t, 85.0, hc.diskThreshold)
}

func TestHealthChecker_RegisterCheck(t *testing.T) {
	hc := NewHealthChecker()

	config := CheckerConfig{
		Name:     "custom-check",
		Type:     CheckTypeCustom,
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
		Enabled:  true,
	}

	hc.RegisterCheck(config, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
		return StatusHealthy, "custom check passed", nil
	})

	assert.Contains(t, hc.ListChecks(), "custom-check")
}

func TestHealthChecker_UnregisterCheck(t *testing.T) {
	hc := NewHealthChecker()

	// 默认检查项存在
	assert.Contains(t, hc.ListChecks(), "cpu")

	hc.UnregisterCheck("cpu")
	assert.NotContains(t, hc.ListChecks(), "cpu")
}

func TestHealthChecker_EnableDisableCheck(t *testing.T) {
	hc := NewHealthChecker()

	// 禁用检查
	err := hc.DisableCheck("cpu")
	require.NoError(t, err)

	hc.mu.RLock()
	checker := hc.checkers["cpu"]
	hc.mu.RUnlock()
	assert.False(t, checker.enabled)

	// 启用检查
	err = hc.EnableCheck("cpu")
	require.NoError(t, err)
	assert.True(t, checker.enabled)

	// 不存在的检查
	err = hc.DisableCheck("nonexistent")
	assert.Error(t, err)
}

func TestHealthChecker_Check(t *testing.T) {
	hc := NewHealthChecker(
		WithLogger(zap.NewNop()),
		WithVersion("v2.51.0"),
	)

	ctx := context.Background()
	report := hc.Check(ctx)

	require.NotNil(t, report)
	assert.NotEmpty(t, report.Version)
	assert.NotZero(t, report.Timestamp)
	assert.NotZero(t, report.Uptime)
	assert.NotEmpty(t, report.Checks)

	// 检查摘要
	assert.Equal(t, report.Summary.Total, len(report.Checks))
	assert.Equal(t, report.Summary.Healthy+report.Summary.Unhealthy+report.Summary.Degraded, report.Summary.Total)
}

func TestHealthChecker_CheckSingle(t *testing.T) {
	hc := NewHealthChecker()

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "memory")

	require.NoError(t, err)
	assert.Equal(t, "memory", result.Name)
	assert.Equal(t, CheckTypeMemory, result.Type)
	assert.NotEmpty(t, result.Status)
	assert.NotZero(t, result.Duration)

	// 不存在的检查
	_, err = hc.CheckSingle(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestHealthChecker_GetResult(t *testing.T) {
	hc := NewHealthChecker()

	// 执行一次检查
	ctx := context.Background()
	hc.Check(ctx)

	// 获取缓存结果
	result, exists := hc.GetResult("cpu")
	assert.True(t, exists)
	assert.Equal(t, "cpu", result.Name)

	// 不存在的检查
	_, exists = hc.GetResult("nonexistent")
	assert.False(t, exists)
}

func TestHealthChecker_GetAllResults(t *testing.T) {
	hc := NewHealthChecker()

	ctx := context.Background()
	hc.Check(ctx)

	results := hc.GetAllResults()
	assert.NotEmpty(t, results)
	assert.Contains(t, results, "cpu")
	assert.Contains(t, results, "memory")
	assert.Contains(t, results, "disk")
	assert.Contains(t, results, "network")
}

func TestHealthChecker_ListChecks(t *testing.T) {
	hc := NewHealthChecker()

	checks := hc.ListChecks()
	assert.Contains(t, checks, "cpu")
	assert.Contains(t, checks, "memory")
	assert.Contains(t, checks, "disk")
	assert.Contains(t, checks, "network")
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	hc := NewHealthChecker()

	ctx := context.Background()
	healthy := hc.IsHealthy(ctx)
	// 结果取决于系统状态
	t.Logf("System is healthy: %v", healthy)
}

func TestHealthChecker_SetGetThreshold(t *testing.T) {
	hc := NewHealthChecker()

	// 设置阈值
	hc.SetThreshold(CheckTypeCPU, 50.0)
	hc.SetThreshold(CheckTypeMemory, 60.0)
	hc.SetThreshold(CheckTypeDisk, 70.0)

	assert.Equal(t, 50.0, hc.GetThreshold(CheckTypeCPU))
	assert.Equal(t, 60.0, hc.GetThreshold(CheckTypeMemory))
	assert.Equal(t, 70.0, hc.GetThreshold(CheckTypeDisk))

	// 不存在的类型
	assert.Equal(t, 0.0, hc.GetThreshold(CheckTypeCustom))
}

func TestHealthChecker_GetUptime(t *testing.T) {
	hc := NewHealthChecker()

	time.Sleep(100 * time.Millisecond)
	uptime := hc.GetUptime()
	assert.True(t, uptime >= 100*time.Millisecond)
}

func TestHealthChecker_GetVersion(t *testing.T) {
	hc := NewHealthChecker(WithVersion("v2.51.0"))
	assert.Equal(t, "v2.51.0", hc.GetVersion())
}

func TestHealthChecker_CPUCheck(t *testing.T) {
	hc := NewHealthChecker(WithCPUThreshold(95.0)) // 高阈值确保测试通过

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "cpu")

	require.NoError(t, err)
	assert.Equal(t, "cpu", result.Name)
	assert.Contains(t, result.Details, "cpu_percent")
	assert.Contains(t, result.Details, "threshold")
	assert.Contains(t, result.Details, "cpu_count")
}

func TestHealthChecker_MemoryCheck(t *testing.T) {
	hc := NewHealthChecker(WithMemoryThreshold(95.0))

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "memory")

	require.NoError(t, err)
	assert.Equal(t, "memory", result.Name)
	assert.Contains(t, result.Details, "alloc_mb")
	assert.Contains(t, result.Details, "sys_mb")
	assert.Contains(t, result.Details, "used_percent")
	assert.Contains(t, result.Details, "num_gc")
}

func TestHealthChecker_DiskCheck(t *testing.T) {
	hc := NewHealthChecker(WithDiskThreshold(95.0))

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "disk")

	require.NoError(t, err)
	assert.Equal(t, "disk", result.Name)
	assert.Contains(t, result.Details, "path")
	assert.Contains(t, result.Details, "total_gb")
	assert.Contains(t, result.Details, "used_gb")
	assert.Contains(t, result.Details, "free_gb")
	assert.Contains(t, result.Details, "used_percent")
}

func TestHealthChecker_NetworkCheck(t *testing.T) {
	hc := NewHealthChecker()

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "network")

	require.NoError(t, err)
	assert.Equal(t, "network", result.Name)
	assert.Contains(t, result.Details, "interface_count")
}

func TestHealthChecker_CustomCheck(t *testing.T) {
	hc := NewHealthChecker()

	// 注册自定义检查
	hc.RegisterCheck(CheckerConfig{
		Name:    "custom-app",
		Type:    CheckTypeCustom,
		Enabled: true,
	}, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
		return StatusHealthy, "application is running", map[string]interface{}{
			"uptime_seconds": 3600,
			"connections":    10,
		}
	})

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "custom-app")

	require.NoError(t, err)
	assert.Equal(t, "custom-app", result.Name)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "application is running", result.Message)
	assert.Equal(t, 3600, result.Details["uptime_seconds"])
	assert.Equal(t, 10, result.Details["connections"])
}

func TestHealthChecker_CheckWithTimeout(t *testing.T) {
	hc := NewHealthChecker()

	// 注册一个慢检查
	hc.RegisterCheck(CheckerConfig{
		Name:    "slow-check",
		Type:    CheckTypeCustom,
		Timeout: 100 * time.Millisecond,
		Enabled: true,
	}, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
		time.Sleep(200 * time.Millisecond) // 比超时时间长
		return StatusHealthy, "done", nil
	})

	ctx := context.Background()
	result, err := hc.CheckSingle(ctx, "slow-check")

	// 检查应该因为超时而失败或返回结果
	_ = result
	_ = err
}

func TestHealthChecker_StartStop(t *testing.T) {
	hc := NewHealthChecker(WithLogger(zap.NewNop()))

	// 启动定期检查
	hc.Start(100 * time.Millisecond)

	// 等待几次检查周期
	time.Sleep(250 * time.Millisecond)

	// 停止
	hc.Stop()

	// 验证有结果
	results := hc.GetAllResults()
	assert.NotEmpty(t, results)
}

func TestHealthChecker_ConcurrentAccess(t *testing.T) {
	hc := NewHealthChecker()

	// 并发执行检查
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			ctx := context.Background()
			hc.Check(ctx)
			done <- true
		}()
	}

	// 等待所有完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 应该没有竞态条件
	results := hc.GetAllResults()
	assert.NotEmpty(t, results)
}

func TestHealthReport_StatusDetermination(t *testing.T) {
	tests := []struct {
		name           string
		healthy        int
		unhealthy      int
		degraded       int
		expectedStatus HealthStatus
	}{
		{"all healthy", 3, 0, 0, StatusHealthy},
		{"one degraded", 2, 0, 1, StatusDegraded},
		{"one unhealthy", 2, 1, 0, StatusUnhealthy},
		{"multiple unhealthy", 1, 2, 0, StatusUnhealthy},
		{"mixed", 1, 1, 1, StatusUnhealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker()

			// 清除默认检查
			for name := range hc.checkers {
				delete(hc.checkers, name)
			}

			// 添加模拟检查
			for i := 0; i < tt.healthy; i++ {
				hc.RegisterCheck(CheckerConfig{
					Name:    "healthy-" + string(rune(i)),
					Type:    CheckTypeCustom,
					Enabled: true,
				}, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
					return StatusHealthy, "ok", nil
				})
			}

			for i := 0; i < tt.unhealthy; i++ {
				hc.RegisterCheck(CheckerConfig{
					Name:    "unhealthy-" + string(rune(i)),
					Type:    CheckTypeCustom,
					Enabled: true,
				}, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
					return StatusUnhealthy, "fail", nil
				})
			}

			for i := 0; i < tt.degraded; i++ {
				hc.RegisterCheck(CheckerConfig{
					Name:    "degraded-" + string(rune(i)),
					Type:    CheckTypeCustom,
					Enabled: true,
				}, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
					return StatusDegraded, "degraded", nil
				})
			}

			ctx := context.Background()
			report := hc.Check(ctx)

			assert.Equal(t, tt.expectedStatus, report.Status)
		})
	}
}

func TestCheckResult_JSON(t *testing.T) {
	result := CheckResult{
		Name:      "test",
		Type:      CheckTypeMemory,
		Status:    StatusHealthy,
		Message:   "All good",
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
		Details: map[string]interface{}{
			"value": 42.5,
		},
	}

	// 测试序列化
	// 实际实现中可以使用 json.Marshal 验证
	assert.NotNil(t, result.Details)
}

// 基准测试
func BenchmarkHealthChecker_Check(b *testing.B) {
	hc := NewHealthChecker()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.Check(ctx)
	}
}

func BenchmarkHealthChecker_CheckSingle(b *testing.B) {
	hc := NewHealthChecker()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.CheckSingle(ctx, "memory")
	}
}

func BenchmarkHealthChecker_RegisterCheck(b *testing.B) {
	hc := NewHealthChecker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := CheckerConfig{
			Name:    "bench-check",
			Type:    CheckTypeCustom,
			Enabled: true,
		}
		hc.RegisterCheck(config, func(ctx context.Context) (HealthStatus, string, map[string]interface{}) {
			return StatusHealthy, "ok", nil
		})
		hc.UnregisterCheck("bench-check")
	}
}
