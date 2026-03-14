// Package monitor 提供系统监控功能
package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr, err := NewManager()
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotEmpty(t, mgr.hostname)
}

func TestManager_GetSystemStats(t *testing.T) {
	mgr, _ := NewManager()

	stats, err := mgr.GetSystemStats()
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// 验证基本字段
	assert.GreaterOrEqual(t, stats.CPUUsage, 0.0)
	assert.LessOrEqual(t, stats.CPUUsage, 100.0)
	assert.GreaterOrEqual(t, stats.MemoryUsage, 0.0)
	assert.LessOrEqual(t, stats.MemoryUsage, 100.0)
	assert.NotZero(t, stats.MemoryTotal)
	assert.NotZero(t, stats.UptimeSeconds)
	assert.Len(t, stats.LoadAvg, 3)
	assert.NotZero(t, stats.Processes)
	assert.False(t, stats.Timestamp.IsZero())
}

func TestManager_GetDiskStats(t *testing.T) {
	mgr, _ := NewManager()

	stats, err := mgr.GetDiskStats()
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// 应该至少有一个挂载点
	if len(stats) > 0 {
		disk := stats[0]
		assert.NotEmpty(t, disk.MountPoint)
		assert.NotZero(t, disk.Total)
		assert.GreaterOrEqual(t, disk.UsagePercent, 0.0)
		assert.LessOrEqual(t, disk.UsagePercent, 100.0)
	}
}

func TestManager_GetNetworkStats(t *testing.T) {
	mgr, _ := NewManager()

	stats, err := mgr.GetNetworkStats()
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// 检查是否有网络接口
	for _, net := range stats {
		assert.NotEmpty(t, net.Interface)
		// lo 接口通常在第一个
		break
	}
}

// ========== SystemStats 结构测试 ==========

func TestSystemStats_Validation(t *testing.T) {
	stats := &SystemStats{
		CPUUsage:      25.5,
		MemoryUsage:   45.0,
		MemoryTotal:   16000000000,
		MemoryUsed:    7200000000,
		MemoryFree:    8800000000,
		SwapUsage:     10.0,
		SwapTotal:     8000000000,
		SwapUsed:      800000000,
		Uptime:        "10 days",
		UptimeSeconds: 864000,
		LoadAvg:       []float64{0.5, 0.3, 0.2},
		Processes:     150,
		Timestamp:     time.Now(),
	}

	// 验证数据一致性
	expectedFree := stats.MemoryTotal - stats.MemoryUsed
	assert.Equal(t, expectedFree, stats.MemoryFree)

	// 负载平均值应该递减
	assert.GreaterOrEqual(t, stats.LoadAvg[0], stats.LoadAvg[1])
	assert.GreaterOrEqual(t, stats.LoadAvg[1], stats.LoadAvg[2])
}

// ========== DiskStats 结构测试 ==========

func TestDiskStats_Validation(t *testing.T) {
	disk := &DiskStats{
		Device:       "/dev/sda1",
		MountPoint:   "/",
		Total:        500000000000,
		Used:         250000000000,
		Free:         250000000000,
		UsagePercent: 50.0,
		FSType:       "ext4",
	}

	// 验证大小一致性
	expectedFree := disk.Total - disk.Used
	assert.Equal(t, expectedFree, disk.Free)

	// 验证使用百分比
	expectedPercent := float64(disk.Used) / float64(disk.Total) * 100
	assert.InDelta(t, expectedPercent, disk.UsagePercent, 0.1)
}

// ========== NetworkStats 结构测试 ==========

func TestNetworkStats_Validation(t *testing.T) {
	net := &NetworkStats{
		Interface: "eth0",
		RXBytes:   1000000000,
		TXBytes:   500000000,
		RXPackets: 1000000,
		TXPackets: 500000,
		RXErrors:  0,
		TXErrors:  0,
	}

	assert.NotEmpty(t, net.Interface)
	assert.GreaterOrEqual(t, net.RXBytes, uint64(0))
	assert.GreaterOrEqual(t, net.TXBytes, uint64(0))
}

// ========== Alert 结构测试 ==========

func TestAlert_Structure(t *testing.T) {
	alert := Alert{
		ID:           "alert-001",
		Type:         "cpu",
		Level:        "warning",
		Message:      "CPU 使用率过高",
		Source:       "system",
		Timestamp:    time.Now(),
		Acknowledged: false,
	}

	assert.NotEmpty(t, alert.ID)
	assert.Contains(t, []string{"cpu", "memory", "disk", "smart"}, alert.Type)
	assert.Contains(t, []string{"warning", "critical"}, alert.Level)
}

// ========== AlertRule 结构测试 ==========

func TestAlertRule_Structure(t *testing.T) {
	rule := AlertRule{
		Name:      "high-cpu",
		Type:      "cpu",
		Threshold: 80.0,
		Level:     "warning",
		Enabled:   true,
	}

	assert.NotEmpty(t, rule.Name)
	assert.Contains(t, []string{"cpu", "memory", "disk", "smart"}, rule.Type)
	assert.GreaterOrEqual(t, rule.Threshold, 0.0)
	assert.LessOrEqual(t, rule.Threshold, 100.0)
}

// ========== 默认告警规则测试 ==========

func TestDefaultAlertRules(t *testing.T) {
	rules := []AlertRule{
		{Name: "cpu-warning", Type: "cpu", Threshold: 80.0, Level: "warning", Enabled: true},
		{Name: "cpu-critical", Type: "cpu", Threshold: 95.0, Level: "critical", Enabled: true},
		{Name: "memory-warning", Type: "memory", Threshold: 80.0, Level: "warning", Enabled: true},
		{Name: "memory-critical", Type: "memory", Threshold: 95.0, Level: "critical", Enabled: true},
		{Name: "disk-warning", Type: "disk", Threshold: 80.0, Level: "warning", Enabled: true},
		{Name: "disk-critical", Type: "disk", Threshold: 95.0, Level: "critical", Enabled: true},
	}

	for _, rule := range rules {
		assert.NotEmpty(t, rule.Name)
		assert.True(t, rule.Enabled)
		assert.GreaterOrEqual(t, rule.Threshold, 0.0)
		assert.LessOrEqual(t, rule.Threshold, 100.0)
	}
}

// ========== 并发测试 ==========

func TestManager_ConcurrentStats(t *testing.T) {
	mgr, _ := NewManager()
	done := make(chan bool, 10)

	// 并发获取系统统计
	for i := 0; i < 5; i++ {
		go func() {
			_, err := mgr.GetSystemStats()
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 并发获取磁盘统计
	for i := 0; i < 5; i++ {
		go func() {
			_, err := mgr.GetDiskStats()
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 性能测试 ==========

func BenchmarkManager_GetSystemStats(b *testing.B) {
	mgr, _ := NewManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetSystemStats()
	}
}

func BenchmarkManager_GetDiskStats(b *testing.B) {
	mgr, _ := NewManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetDiskStats()
	}
}
