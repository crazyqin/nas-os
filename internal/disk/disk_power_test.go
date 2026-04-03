package disk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 电源状态测试 ==========

func TestPowerState_Constants(t *testing.T) {
	states := []PowerState{
		PowerStateActive,
		PowerStateIdle,
		PowerStateStandby,
		PowerStateSleep,
	}

	for _, state := range states {
		assert.NotEmpty(t, string(state))
	}
}

// ========== SleepPolicy测试 ==========

func TestSleepPolicy_Default(t *testing.T) {
	policy := DefaultSleepPolicy()

	assert.Equal(t, "default", policy.ID)
	assert.Equal(t, "默认节能策略", policy.Name)
	assert.Equal(t, 5*time.Minute, policy.IdleThreshold)
	assert.Equal(t, 15*time.Minute, policy.StandbyThreshold)
	assert.Equal(t, 30*time.Minute, policy.SleepThreshold)
	assert.True(t, policy.Enabled)
	assert.Empty(t, policy.ExcludedDisks)
}

func TestSleepPolicy_WithBusinessPeriods(t *testing.T) {
	policy := &SleepPolicy{
		ID:               "custom",
		Name:             "自定义策略",
		IdleThreshold:    10 * time.Minute,
		StandbyThreshold: 30 * time.Minute,
		SleepThreshold:   60 * time.Minute,
		Enabled:          true,
		BusinessPeriods: []BusinessPeriod{
			{StartHour: 9, EndHour: 12, Priority: 8},
			{StartHour: 14, EndHour: 18, Priority: 9},
		},
		AllowSleepInPeak: false,
		MaxWakePerHour:   3,
	}

	assert.Len(t, policy.BusinessPeriods, 2)
	assert.False(t, policy.AllowSleepInPeak)
	assert.Equal(t, 3, policy.MaxWakePerHour)
}

// ========== PowerManager基础测试 ==========

func TestNewPowerManager(t *testing.T) {
	pm := NewPowerManager(nil)
	require.NotNil(t, pm)
	assert.NotNil(t, pm.statuses)
	assert.NotNil(t, pm.policies)
	assert.NotNil(t, pm.activityMon)
	assert.NotNil(t, pm.config)
	assert.True(t, pm.config.EnableMonitoring)
	assert.True(t, pm.config.EnableWakeOnDemand)
	assert.True(t, pm.config.EnableSmartScheduling)
}

func TestNewPowerManager_WithConfig(t *testing.T) {
	cfg := &PowerConfig{
		CheckInterval:          10 * time.Second,
		DefaultPolicy:          "custom",
		EnableMonitoring:       true,
		EnableWakeOnDemand:     false,
		EnableSmartScheduling:  false,
		DefaultDiskPowerWatts:  8.0,
		WakePowerSpikeWatts:    12.0,
		WakeDurationSeconds:    5.0,
	}

	pm := NewPowerManager(cfg)
	assert.Equal(t, 10*time.Second, pm.config.CheckInterval)
	assert.Equal(t, "custom", pm.config.DefaultPolicy)
	assert.False(t, pm.config.EnableWakeOnDemand)
	assert.False(t, pm.config.EnableSmartScheduling)
	assert.Equal(t, 8.0, pm.config.DefaultDiskPowerWatts)
}

func TestPowerManager_RegisterDisk(t *testing.T) {
	pm := NewPowerManager(nil)

	// 添加默认策略
	pm.AddPolicy(DefaultSleepPolicy())

	err := pm.RegisterDisk("/dev/sda", "default")
	require.NoError(t, err)

	status := pm.statuses["/dev/sda"]
	assert.NotNil(t, status)
	assert.Equal(t, "/dev/sda", status.DiskID)
	assert.Equal(t, PowerStateActive, status.State)
}

func TestPowerManager_GetDiskStatus(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	status, err := pm.GetDiskStatus("/dev/sda")
	require.NoError(t, err)
	assert.NotNil(t, status)

	// 不存在的磁盘
	status, err = pm.GetDiskStatus("/dev/nonexistent")
	require.NoError(t, err)
	assert.Nil(t, status)
}

func TestPowerManager_GetAllStatuses(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")
	pm.RegisterDisk("/dev/sdb", "default")

	statuses := pm.GetAllStatuses()
	assert.Len(t, statuses, 2)
}

func TestPowerManager_RecordActivity(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 模拟休眠状态
	pm.statuses["/dev/sda"].State = PowerStateSleep

	// 记录活动应该唤醒磁盘
	err := pm.RecordActivity("/dev/sda")
	require.NoError(t, err)

	status := pm.statuses["/dev/sda"]
	assert.Equal(t, PowerStateActive, status.State)
	assert.Equal(t, 0*time.Second, status.IdleDuration)
}

func TestPowerManager_AddPolicy(t *testing.T) {
	pm := NewPowerManager(nil)

	policy := &SleepPolicy{
		ID:               "aggressive",
		Name:             "激进节能",
		IdleThreshold:    2 * time.Minute,
		StandbyThreshold: 5 * time.Minute,
		SleepThreshold:   10 * time.Minute,
		Enabled:          true,
	}

	err := pm.AddPolicy(policy)
	require.NoError(t, err)

	retrieved, err := pm.GetPolicy("aggressive")
	require.NoError(t, err)
	assert.Equal(t, policy, retrieved)
}

// ========== 能耗统计测试 ==========

func TestEnergyStatistics_New(t *testing.T) {
	stats := NewEnergyStatistics()
	require.NotNil(t, stats)
	assert.NotNil(t, stats.DiskStats)
	assert.NotZero(t, stats.StartTime)
}

func TestPowerManager_GetEnergyReport(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")
	pm.RegisterDisk("/dev/sdb", "default")

	// 设置一些能耗数据
	pm.statuses["/dev/sda"].EnergySaved = 0.5
	pm.statuses["/dev/sda"].SleepCount = 10
	pm.statuses["/dev/sdb"].EnergySaved = 0.3

	report := pm.GetEnergyReport()
	require.NotNil(t, report)
	assert.Len(t, report.Disks, 2)
	assert.GreaterOrEqual(t, report.TotalEnergySaved, 0.8)
}

func TestPowerManager_updateEnergyStatistics(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 模拟活跃状态
	pm.statuses["/dev/sda"].State = PowerStateActive

	pm.updateEnergyStatistics()

	stats := pm.energyStats.DiskStats["/dev/sda"]
	require.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.ActiveHours, 0.0)
	assert.GreaterOrEqual(t, stats.EnergyConsumed, 0.0)
}

// ========== 业务时段测试 ==========

func TestDefaultBusinessPeriods(t *testing.T) {
	periods := DefaultBusinessPeriods()
	require.Len(t, periods, 3)

	// 上午时段
	assert.Equal(t, 9, periods[0].StartHour)
	assert.Equal(t, 12, periods[0].EndHour)
	assert.Equal(t, 8, periods[0].Priority)

	// 下午时段
	assert.Equal(t, 14, periods[1].StartHour)
	assert.Equal(t, 18, periods[1].EndHour)
	assert.Equal(t, 9, periods[1].Priority)

	// 晚间时段
	assert.Equal(t, 20, periods[2].StartHour)
	assert.Equal(t, 22, periods[2].EndHour)
	assert.Equal(t, 5, periods[2].Priority)
}

func TestPowerManager_isBusinessPeakHour(t *testing.T) {
	pm := NewPowerManager(nil)

	tests := []struct {
		hour     int
		expected bool
	}{
		{8, false},  // 早于工作时间
		{9, true},   // 上午工作时段开始
		{10, true},  // 上午工作时段
		{12, false}, // 午休
		{14, true},  // 下午工作时段开始
		{16, true},  // 下午工作时段
		{18, false}, // 下班后
		{20, false}, // 晚间低优先级时段
		{22, false}, // 晚间结束
		{23, false}, // 夜间
		{0, false},  // 深夜
		{3, false},  // 深夜
	}

	for _, tt := range tests {
		result := pm.isBusinessPeakHour(tt.hour)
		assert.Equal(t, tt.expected, result, "hour=%d", tt.hour)
	}
}

// ========== 智能调度测试 ==========

func TestPowerManager_checkDiskStates_BusinessPeak(t *testing.T) {
	pm := NewPowerManager(nil)

	// 配置不允许高峰时段休眠
	policy := &SleepPolicy{
		ID:               "test",
		IdleThreshold:    1 * time.Minute,
		StandbyThreshold: 2 * time.Minute,
		SleepThreshold:   3 * time.Minute,
		Enabled:          true,
		AllowSleepInPeak: false,
	}
	pm.AddPolicy(policy)
	pm.RegisterDisk("/dev/sda", "test")

	// 模拟高峰时段 (设置businessHours为当前时段)
	now := time.Now()
	pm.businessHours = []BusinessPeriod{
		{StartHour: now.Hour(), EndHour: now.Hour() + 1, Priority: 9},
	}

	// 模拟长时间空闲
	pm.statuses["/dev/sda"].LastActivity = now.Add(-10 * time.Minute)

	pm.checkDiskStates()

	// 高峰时段不应休眠
	status := pm.statuses["/dev/sda"]
	assert.Equal(t, PowerStateActive, status.State)
}

func TestPowerManager_checkDiskStates_WakeFrequencyLimit(t *testing.T) {
	pm := NewPowerManager(nil)

	policy := &SleepPolicy{
		ID:              "test",
		IdleThreshold:   1 * time.Minute,
		StandbyThreshold: 2 * time.Minute,
		SleepThreshold:  3 * time.Minute,
		Enabled:         true,
		MaxWakePerHour:  2,
	}
	pm.AddPolicy(policy)
	pm.RegisterDisk("/dev/sda", "test")

	// 模拟当前小时已达到唤醒限制
	now := time.Now()
	pm.statuses["/dev/sda"].LastActivity = now.Add(-10 * time.Minute)
	pm.statuses["/dev/sda"].WakeCountHour = 2
	pm.statuses["/dev/sda"].LastWakeHour = now.Hour()

	pm.checkDiskStates()

	// 由于达到唤醒限制，不应休眠
	status := pm.statuses["/dev/sda"]
	assert.Equal(t, PowerStateActive, status.State)
}

// ========== 按需唤醒测试 ==========

func TestPowerManager_WakeOnDemand(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 模拟休眠状态
	pm.statuses["/dev/sda"].State = PowerStateSleep

	// 添加唤醒请求
	request := WakeRequest{
		DiskID:      "/dev/sda",
		Reason:      "用户访问文件",
		Priority:    8,
		Timestamp:   time.Now(),
		RequestedBy: "api",
	}

	pm.wakeQueueMu.Lock()
	pm.wakeQueue["/dev/sda"] = []WakeRequest{request}
	pm.wakeQueueMu.Unlock()

	// 检查磁盘状态应该处理唤醒请求
	pm.checkDiskStates()

	status := pm.statuses["/dev/sda"]
	assert.Equal(t, PowerStateActive, status.State)
	assert.Equal(t, "用户访问文件", status.LastWakeReason)
}

// ========== 状态转换测试 ==========

func TestPowerManager_transitionDisk(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// Active -> Sleep
	pm.transitionDisk("/dev/sda", PowerStateSleep)
	assert.Equal(t, PowerStateSleep, pm.statuses["/dev/sda"].State)
	assert.Equal(t, 1, pm.statuses["/dev/sda"].SleepCount)

	// Sleep -> Active (wake)
	pm.transitionDisk("/dev/sda", PowerStateActive)
	assert.Equal(t, PowerStateActive, pm.statuses["/dev/sda"].State)
	assert.Equal(t, 1, pm.statuses["/dev/sda"].WakeCount)

	// 再次休眠
	pm.transitionDisk("/dev/sda", PowerStateSleep)
	assert.Equal(t, 2, pm.statuses["/dev/sda"].SleepCount)
}

// ========== Start/Stop测试 ==========

func TestPowerManager_Start(t *testing.T) {
	pm := NewPowerManager(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pm.Start(ctx)
	require.NoError(t, err)

	// 等待一小段时间确保监控启动
	time.Sleep(100 * time.Millisecond)
}

func TestPowerManager_Start_Disabled(t *testing.T) {
	cfg := &PowerConfig{
		EnableMonitoring: false,
	}
	pm := NewPowerManager(cfg)

	ctx := context.Background()
	err := pm.Start(ctx)
	require.NoError(t, err)

	// 监控禁用时不应启动监控循环
}

// ========== 并发测试 ==========

func TestPowerManager_Concurrent(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())

	// 并发注册磁盘
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			pm.RegisterDisk("/dev/sd"+string(rune('a'+i)), "default")
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}

	statuses := pm.GetAllStatuses()
	assert.Len(t, statuses, 10)
}

func TestPowerManager_ConcurrentActivity(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 并发记录活动
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			pm.RecordActivity("/dev/sda")
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 100; i++ {
		<-done
	}

	status := pm.statuses["/dev/sda"]
	assert.Equal(t, PowerStateActive, status.State)
}

// ========== 性能测试 ==========

func BenchmarkPowerManager_RecordActivity(b *testing.B) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordActivity("/dev/sda")
	}
}

func BenchmarkPowerManager_checkDiskStates(b *testing.B) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())

	// 注册10个磁盘
	for i := 0; i < 10; i++ {
		pm.RegisterDisk("/dev/sd"+string(rune('a'+i)), "default")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.checkDiskStates()
	}
}

func BenchmarkPowerManager_GetEnergyReport(b *testing.B) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())

	// 注册10个磁盘
	for i := 0; i < 10; i++ {
		pm.RegisterDisk("/dev/sd"+string(rune('a'+i)), "default")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.GetEnergyReport()
	}
}