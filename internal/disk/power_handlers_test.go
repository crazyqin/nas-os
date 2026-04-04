// Package disk 提供磁盘电源管理测试
// power_handlers_test.go - 测试电源管理API扩展功能
package disk

import (
	"context"
	"testing"
	"time"
)

func TestPowerManagerV2_NewPowerManager(t *testing.T) {
	pm := NewPowerManager(nil)
	if pm == nil {
		t.Fatal("PowerManager should not be nil")
	}

	if pm.statuses == nil {
		t.Error("statuses map should be initialized")
	}

	if pm.policies == nil {
		t.Error("policies map should be initialized")
	}

	if pm.config.CheckInterval != 30*time.Second {
		t.Errorf("default check interval should be 30s, got %v", pm.config.CheckInterval)
	}
}

func TestPowerManagerV2_RegisterDisk(t *testing.T) {
	pm := NewPowerManager(nil)

	// 添加默认策略
	pm.AddPolicy(DefaultSleepPolicy())

	// 注册磁盘
	err := pm.RegisterDisk("/dev/sda", "default")
	if err != nil {
		t.Fatalf("failed to register disk: %v", err)
	}

	// 验证状态
	status, err := pm.GetDiskStatus("/dev/sda")
	if err != nil {
		t.Fatalf("failed to get disk status: %v", err)
	}

	if status == nil {
		t.Fatal("status should not be nil")
	}

	if status.State != PowerStateActive {
		t.Errorf("initial state should be active, got %s", status.State)
	}
}

func TestPowerManagerV2_RecordActivity(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 模拟休眠状态
	pm.ForceSleep("/dev/sda")

	status, _ := pm.GetDiskStatus("/dev/sda")
	if status.State != PowerStateSleep {
		t.Errorf("state should be sleep, got %s", status.State)
	}

	// 记录活动唤醒磁盘
	err := pm.RecordActivity("/dev/sda")
	if err != nil {
		t.Fatalf("failed to record activity: %v", err)
	}

	status, _ = pm.GetDiskStatus("/dev/sda")
	if status.State != PowerStateActive {
		t.Errorf("state should be active after activity, got %s", status.State)
	}
}

func TestPowerManagerV2_AddPolicy(t *testing.T) {
	pm := NewPowerManager(nil)

	policy := &SleepPolicy{
		ID:               "custom",
		Name:             "自定义策略",
		IdleThreshold:    10 * time.Minute,
		StandbyThreshold: 20 * time.Minute,
		SleepThreshold:   30 * time.Minute,
		Enabled:          true,
		MaxWakePerHour:   3,
	}

	err := pm.AddPolicy(policy)
	if err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	retrieved, err := pm.GetPolicy("custom")
	if err != nil {
		t.Fatalf("failed to get policy: %v", err)
	}

	if retrieved.Name != "自定义策略" {
		t.Errorf("policy name mismatch: %s", retrieved.Name)
	}
}

func TestPowerManagerV2_DeletePolicy(t *testing.T) {
	pm := NewPowerManager(nil)

	policy := &SleepPolicy{
		ID:      "test-policy",
		Name:    "测试策略",
		Enabled: true,
	}
	pm.AddPolicy(policy)

	// 删除策略
	err := pm.DeletePolicy("test-policy")
	if err != nil {
		t.Fatalf("failed to delete policy: %v", err)
	}

	// 验证已删除
	retrieved, _ := pm.GetPolicy("test-policy")
	if retrieved != nil {
		t.Error("policy should be deleted")
	}
}

func TestPowerManagerV2_WakeQueue(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 添加唤醒请求
	req := WakeRequest{
		DiskID:      "/dev/sda",
		Reason:      "用户访问",
		Priority:    5,
		Timestamp:   time.Now(),
		RequestedBy: "user",
	}

	err := pm.AddWakeRequest(req)
	if err != nil {
		t.Fatalf("failed to add wake request: %v", err)
	}

	queue := pm.GetWakeQueue()
	if len(queue["/dev/sda"]) != 1 {
		t.Errorf("wake queue should have 1 request, got %d", len(queue["/dev/sda"]))
	}

	// 清除队列
	pm.ClearWakeQueue("/dev/sda")
	queue = pm.GetWakeQueue()
	if len(queue["/dev/sda"]) != 0 {
		t.Error("wake queue should be cleared")
	}
}

func TestPowerManagerV2_EnergyStatistics(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())

	// 注册多个磁盘
	pm.RegisterDisk("/dev/sda", "default")
	pm.RegisterDisk("/dev/sdb", "default")

	// 获取能耗统计
	stats := pm.GetEnergyStatistics()
	if stats == nil {
		t.Fatal("energy stats should not be nil")
	}

	// 获取能耗报告
	report := pm.GetEnergyReport()
	if report == nil {
		t.Fatal("energy report should not be nil")
	}
}

func TestPowerManagerV2_BusinessHours(t *testing.T) {
	pm := NewPowerManager(nil)

	// 获取默认业务时段
	periods := pm.GetBusinessHours()
	if len(periods) == 0 {
		t.Error("default business periods should not be empty")
	}

	// 设置自定义业务时段
	customPeriods := []BusinessPeriod{
		{StartHour: 8, EndHour: 12, Priority: 9},
		{StartHour: 13, EndHour: 18, Priority: 8},
	}
	pm.SetBusinessHours(customPeriods)

	retrieved := pm.GetBusinessHours()
	if len(retrieved) != 2 {
		t.Errorf("should have 2 business periods, got %d", len(retrieved))
	}
}

func TestPowerManagerV2_SmartScheduleConfig(t *testing.T) {
	pm := NewPowerManager(nil)

	// 获取智能调度配置
	config := pm.GetSmartScheduleConfig()
	if !config.EnableWakeOnDemand {
		t.Error("wake on demand should be enabled by default")
	}

	// 更新配置
	pm.UpdateSmartScheduleConfig(true, true, 12.0, 20.0, 15.0)

	updated := pm.GetSmartScheduleConfig()
	if updated.DefaultDiskPowerWatts != 12.0 {
		t.Errorf("disk power watts should be 12.0, got %f", updated.DefaultDiskPowerWatts)
	}
}

func TestPowerManagerV2_TransitionStates(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())
	pm.RegisterDisk("/dev/sda", "default")

	// 测试强制休眠
	err := pm.ForceSleep("/dev/sda")
	if err != nil {
		t.Fatalf("failed to force sleep: %v", err)
	}

	status, _ := pm.GetDiskStatus("/dev/sda")
	if status.State != PowerStateSleep {
		t.Errorf("state should be sleep, got %s", status.State)
	}

	// 测试强制待机
	err = pm.ForceStandby("/dev/sda")
	if err != nil {
		t.Fatalf("failed to force standby: %v", err)
	}

	status, _ = pm.GetDiskStatus("/dev/sda")
	if status.State != PowerStateStandby {
		t.Errorf("state should be standby, got %s", status.State)
	}
}

func TestPowerManagerV2_IsBusinessPeakHour(t *testing.T) {
	pm := NewPowerManager(nil)

	// 测试不同时段
	// 默认配置: 9-12, 14-18 为高峰
	testCases := []struct {
		hour     int
		expected bool
	}{
		{9, true},  // 上午工作时段开始
		{10, true}, // 上午工作时段
		{12, false}, // 上午工作时段结束
		{14, true}, // 下午工作时段开始
		{16, true}, // 下午工作时段
		{18, false}, // 下午工作时段结束
		{22, false}, // 晚间使用时段
		{3, false},  // 凌晨
	}

	for _, tc := range testCases {
		result := pm.isBusinessPeakHour(tc.hour)
		if result != tc.expected {
			t.Errorf("hour %d: expected %v, got %v", tc.hour, tc.expected, result)
		}
	}
}

func TestPowerManagerV2_StartStop(t *testing.T) {
	pm := NewPowerManager(&PowerConfig{
		CheckInterval:          1 * time.Second,
		EnableMonitoring:       true,
		EnableWakeOnDemand:     true,
		EnableSmartScheduling:  true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pm.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start power manager: %v", err)
	}

	// 短暂运行
	time.Sleep(100 * time.Millisecond)

	// 取消上下文停止监控
	cancel()
}

func TestPowerManagerV2_GetAllStatuses(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddPolicy(DefaultSleepPolicy())

	// 注册多个磁盘
	pm.RegisterDisk("/dev/sda", "default")
	pm.RegisterDisk("/dev/sdb", "default")
	pm.RegisterDisk("/dev/sdc", "default")

	// 设置不同状态
	pm.ForceSleep("/dev/sda")
	pm.ForceStandby("/dev/sdb")

	statuses := pm.GetAllStatuses()
	if len(statuses) != 3 {
		t.Errorf("should have 3 statuses, got %d", len(statuses))
	}

	// 验证状态
	counts := make(map[PowerState]int)
	for _, status := range statuses {
		counts[status.State]++
	}

	if counts[PowerStateSleep] != 1 {
		t.Errorf("should have 1 sleeping disk, got %d", counts[PowerStateSleep])
	}
	if counts[PowerStateStandby] != 1 {
		t.Errorf("should have 1 standby disk, got %d", counts[PowerStateStandby])
	}
}