package smartpower

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// ========== NewPowerManager 测试 ==========

func TestNewPowerManager_DefaultConfig(t *testing.T) {
	pm := NewPowerManager(nil)
	if pm == nil {
		t.Fatal("NewPowerManager(nil) returned nil")
	}

	config := pm.GetConfig()
	if config.DiskSpindownSec != 1800 {
		t.Errorf("expected DiskSpindownSec=1800, got %d", config.DiskSpindownSec)
	}
	if config.CPUGovernor != "ondemand" {
		t.Errorf("expected CPUGovernor=ondemand, got %s", config.CPUGovernor)
	}
	if config.FanSpeed != 50 {
		t.Errorf("expected FanSpeed=50, got %d", config.FanSpeed)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestNewPowerManager_CustomConfig(t *testing.T) {
	config := &PowerConfig{
		DiskSpindownSec:      600,
		CPUGovernor:          "powersave",
		TempCheckIntervalSec: 60,
		FanSpeed:             30,
		Enabled:              false,
	}
	pm := NewPowerManager(config)

	got := pm.GetConfig()
	if got.DiskSpindownSec != 600 {
		t.Errorf("expected 600, got %d", got.DiskSpindownSec)
	}
	if got.CPUGovernor != "powersave" {
		t.Errorf("expected powersave, got %s", got.CPUGovernor)
	}
}

// ========== 磁盘管理测试 ==========

func TestAddDisk(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 35.5)

	states := pm.GetDiskStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(states))
	}

	disk := states[0]
	if disk.Name != "sda" {
		t.Errorf("expected name=sda, got %s", disk.Name)
	}
	if !disk.IsSpinning {
		t.Error("expected new disk to be spinning")
	}
	if disk.Temperature != 35.5 {
		t.Errorf("expected temp=35.5, got %f", disk.Temperature)
	}
	if disk.SpindownTimer != 1800 {
		t.Errorf("expected SpindownTimer=1800, got %d", disk.SpindownTimer)
	}
}

func TestRemoveDisk(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)
	pm.AddDisk("sdb", 32)
	pm.RemoveDisk("sda")

	states := pm.GetDiskStates()
	if len(states) != 1 {
		t.Fatalf("expected 1 disk after remove, got %d", len(states))
	}
	if states[0].Name != "sdb" {
		t.Errorf("expected remaining disk=sdb, got %s", states[0].Name)
	}
}

func TestRemoveDisk_NotExists(t *testing.T) {
	pm := NewPowerManager(nil)
	// 不应 panic
	pm.RemoveDisk("nonexistent")
}

func TestSpindownDisk(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)

	err := pm.SpindownDisk("sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	states := pm.GetDiskStates()
	disk := states[0]
	if disk.IsSpinning {
		t.Error("expected disk to be spun down")
	}
	if disk.SpindownTimer != 0 {
		t.Errorf("expected SpindownTimer=0, got %d", disk.SpindownTimer)
	}
}

func TestSpindownDisk_NotFound(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.SpindownDisk("sda")
	if err == nil {
		t.Fatal("expected error for missing disk")
	}
	if !errors.Is(err, ErrDiskNotFound) {
		t.Errorf("expected ErrDiskNotFound, got %v", err)
	}
}

func TestWakeupDisk(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)
	_ = pm.SpindownDisk("sda")

	err := pm.WakeupDisk("sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	states := pm.GetDiskStates()
	disk := states[0]
	if !disk.IsSpinning {
		t.Error("expected disk to be spinning after wakeup")
	}
	if disk.SpindownTimer != 1800 {
		t.Errorf("expected SpindownTimer=1800, got %d", disk.SpindownTimer)
	}
}

func TestWakeupDisk_NotFound(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.WakeupDisk("sda")
	if err == nil {
		t.Fatal("expected error for missing disk")
	}
	if !errors.Is(err, ErrDiskNotFound) {
		t.Errorf("expected ErrDiskNotFound, got %v", err)
	}
}

func TestGetDiskStates_Empty(t *testing.T) {
	pm := NewPowerManager(nil)
	states := pm.GetDiskStates()
	if len(states) != 0 {
		t.Errorf("expected 0 disks, got %d", len(states))
	}
}

func TestGetDiskStates_Multiple(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)
	pm.AddDisk("sdb", 32)
	pm.AddDisk("sdc", 28)

	states := pm.GetDiskStates()
	if len(states) != 3 {
		t.Errorf("expected 3 disks, got %d", len(states))
	}
}

// ========== 电源方案测试 ==========

func TestApplyProfile(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)

	profile := &PowerProfile{
		Name:            "节能模式",
		CPUGovernor:     "powersave",
		DiskSpindownSec: 300,
		FanSpeed:        30,
		WakeSchedule: []*WakeSchedule{
			{ID: "sched1", Name: "早上开机", Action: "wake", CronExpr: "0 8 * * *"},
		},
	}

	err := pm.ApplyProfile(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证配置已更新
	config := pm.GetConfig()
	if config.CPUGovernor != "powersave" {
		t.Errorf("expected powersave, got %s", config.CPUGovernor)
	}
	if config.DiskSpindownSec != 300 {
		t.Errorf("expected 300, got %d", config.DiskSpindownSec)
	}
	if config.FanSpeed != 30 {
		t.Errorf("expected 30, got %d", config.FanSpeed)
	}

	// 验证磁盘休眠时间已更新
	states := pm.GetDiskStates()
	if states[0].SpindownTimer != 300 {
		t.Errorf("expected disk SpindownTimer=300, got %d", states[0].SpindownTimer)
	}

	// 验证唤醒调度已添加
	schedule, err := pm.GetSchedule("sched1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schedule.Name != "早上开机" {
		t.Errorf("expected schedule name=早上开机, got %s", schedule.Name)
	}
}

func TestApplyProfile_Nil(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.ApplyProfile(nil)
	if err == nil {
		t.Fatal("expected error for nil profile")
	}
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestApplyProfile_PartialUpdate(t *testing.T) {
	pm := NewPowerManager(nil)

	// 只更新部分字段
	profile := &PowerProfile{
		Name:        "部分更新",
		CPUGovernor: "performance",
		FanSpeed:    101, // 超出范围，应被忽略
	}

	err := pm.ApplyProfile(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := pm.GetConfig()
	if config.CPUGovernor != "performance" {
		t.Errorf("expected performance, got %s", config.CPUGovernor)
	}
	// FanSpeed 应保持默认值
	if config.FanSpeed != 50 {
		t.Errorf("expected FanSpeed=50 (default preserved), got %d", config.FanSpeed)
	}
}

func TestGetProfile(t *testing.T) {
	pm := NewPowerManager(nil)

	// 初始无方案
	if pm.GetProfile() != nil {
		t.Error("expected nil profile initially")
	}

	profile := &PowerProfile{Name: "测试方案"}
	_ = pm.ApplyProfile(profile)

	got := pm.GetProfile()
	if got.Name != "测试方案" {
		t.Errorf("expected 测试方案, got %s", got.Name)
	}
}

// ========== 温度监控测试 ==========

func TestAddThermalZone(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddThermalZone("CPU", 45, 70, 85)

	zones := pm.GetThermalStatus()
	if len(zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(zones))
	}

	zone := zones[0]
	if zone.Name != "CPU" {
		t.Errorf("expected name=CPU, got %s", zone.Name)
	}
	if zone.Temperature != 45 {
		t.Errorf("expected temp=45, got %f", zone.Temperature)
	}
	if zone.Threshold != 70 {
		t.Errorf("expected threshold=70, got %f", zone.Threshold)
	}
	if zone.Critical != 85 {
		t.Errorf("expected critical=85, got %f", zone.Critical)
	}
}

func TestUpdateThermalZone(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddThermalZone("CPU", 45, 70, 85)

	err := pm.UpdateThermalZone("CPU", 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	zones := pm.GetThermalStatus()
	if zones[0].Temperature != 60 {
		t.Errorf("expected temp=60, got %f", zones[0].Temperature)
	}
}

func TestUpdateThermalZone_NotFound(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.UpdateThermalZone("CPU", 60)
	if err == nil {
		t.Fatal("expected error for missing zone")
	}
	if !errors.Is(err, ErrThermalZoneNotFound) {
		t.Errorf("expected ErrThermalZoneNotFound, got %v", err)
	}
}

func TestCheckThermalAlerts(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddThermalZone("CPU", 45, 70, 85)    // 未超阈值
	pm.AddThermalZone("HDD", 75, 70, 85)    // 超阈值
	pm.AddThermalZone("System", 80, 70, 85) // 超阈值

	alerts := pm.CheckThermalAlerts()
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestCheckThermalAlerts_None(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddThermalZone("CPU", 45, 70, 85)

	alerts := pm.CheckThermalAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestCheckCriticalThermal(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddThermalZone("CPU", 45, 70, 85) // 正常
	pm.AddThermalZone("GPU", 90, 70, 85) // 超临界

	critical := pm.CheckCriticalThermal()
	if len(critical) != 1 {
		t.Fatalf("expected 1 critical, got %d", len(critical))
	}
	if critical[0].Name != "GPU" {
		t.Errorf("expected critical zone=GPU, got %s", critical[0].Name)
	}
}

func TestGetThermalStatus_Empty(t *testing.T) {
	pm := NewPowerManager(nil)
	zones := pm.GetThermalStatus()
	if len(zones) != 0 {
		t.Errorf("expected 0 zones, got %d", len(zones))
	}
}

// ========== 功耗统计测试 ==========

func TestGetPowerStats_Initial(t *testing.T) {
	pm := NewPowerManager(nil)

	stats := pm.GetPowerStats()
	if stats.CurrentWatts != 0 {
		t.Errorf("expected 0 watts, got %f", stats.CurrentWatts)
	}
	if stats.DailyKWh != 0 {
		t.Errorf("expected 0 daily, got %f", stats.DailyKWh)
	}
}

func TestUpdatePowerStats(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.UpdatePowerStats(65.5, 1.2, 36.5, 25.0)

	stats := pm.GetPowerStats()
	if stats.CurrentWatts != 65.5 {
		t.Errorf("expected 65.5, got %f", stats.CurrentWatts)
	}
	if stats.DailyKWh != 1.2 {
		t.Errorf("expected 1.2, got %f", stats.DailyKWh)
	}
	if stats.MonthlyKWh != 36.5 {
		t.Errorf("expected 36.5, got %f", stats.MonthlyKWh)
	}
	if stats.CostEstimate != 25.0 {
		t.Errorf("expected 25.0, got %f", stats.CostEstimate)
	}
}

func TestGetPowerStats_ReturnsCopy(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.UpdatePowerStats(100, 2, 60, 40)

	stats1 := pm.GetPowerStats()
	stats1.CurrentWatts = 999 // 修改副本

	stats2 := pm.GetPowerStats()
	if stats2.CurrentWatts == 999 {
		t.Error("GetPowerStats should return a copy, not a reference")
	}
}

// ========== 调度管理测试 ==========

func TestSetSchedule(t *testing.T) {
	pm := NewPowerManager(nil)

	schedule := &WakeSchedule{
		ID:       "wake1",
		Name:     "早上开机",
		Action:   "wake",
		CronExpr: "0 8 * * *",
		Enabled:  true,
	}

	err := pm.SetSchedule(schedule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := pm.GetSchedule("wake1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "早上开机" {
		t.Errorf("expected 早上开机, got %s", got.Name)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSetSchedule_Nil(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.SetSchedule(nil)
	if err == nil {
		t.Fatal("expected error for nil schedule")
	}
	if !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestSetSchedule_EmptyID(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.SetSchedule(&WakeSchedule{Name: "test"})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestGetSchedule_NotFound(t *testing.T) {
	pm := NewPowerManager(nil)

	_, err := pm.GetSchedule("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
	if !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestListSchedules(t *testing.T) {
	pm := NewPowerManager(nil)

	pm.SetSchedule(&WakeSchedule{ID: "s1", Name: "Schedule 1"})
	pm.SetSchedule(&WakeSchedule{ID: "s2", Name: "Schedule 2"})

	schedules := pm.ListSchedules()
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestRemoveSchedule(t *testing.T) {
	pm := NewPowerManager(nil)

	pm.SetSchedule(&WakeSchedule{ID: "s1", Name: "Schedule 1"})
	pm.SetSchedule(&WakeSchedule{ID: "s2", Name: "Schedule 2"})

	err := pm.RemoveSchedule("s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schedules := pm.ListSchedules()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule after remove, got %d", len(schedules))
	}

	_, err = pm.GetSchedule("s1")
	if err == nil {
		t.Error("expected error for removed schedule")
	}
}

func TestRemoveSchedule_NotFound(t *testing.T) {
	pm := NewPowerManager(nil)

	err := pm.RemoveSchedule("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
	if !errors.Is(err, ErrScheduleNotFound) {
		t.Errorf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestListSchedules_Empty(t *testing.T) {
	pm := NewPowerManager(nil)

	schedules := pm.ListSchedules()
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(schedules))
	}
}

// ========== 配置测试 ==========

func TestGetConfig_ReturnsCopy(t *testing.T) {
	pm := NewPowerManager(nil)

	config1 := pm.GetConfig()
	config1.DiskSpindownSec = 999 // 修改副本

	config2 := pm.GetConfig()
	if config2.DiskSpindownSec == 999 {
		t.Error("GetConfig should return a copy, not a reference")
	}
}

// ========== 并发安全测试 ==========

func TestConcurrency(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)
	pm.AddDisk("sdb", 32)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 并发读
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pm.GetDiskStates()
			_ = pm.GetThermalStatus()
			_ = pm.GetPowerStats()
			_ = pm.GetConfig()
			_ = pm.GetProfile()
			_ = pm.ListSchedules()
		}()
	}

	// 并发写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := pm.SpindownDisk("sda"); err != nil {
				errChan <- err
			}
			if err := pm.WakeupDisk("sda"); err != nil {
				errChan <- err
			}
			pm.UpdatePowerStats(float64(idx)*10, 1, 30, 20)
		}(i)
	}

	// 并发添加/删除
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("disk_%d", idx)
			pm.AddDisk(name, 30+float64(idx))
			pm.RemoveDisk(name)
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent error: %v", err)
	}
}

// ========== DefaultPowerConfig 测试 ==========

func TestDefaultPowerConfig(t *testing.T) {
	config := DefaultPowerConfig()
	if config == nil {
		t.Fatal("DefaultPowerConfig() returned nil")
	}
	if config.DiskSpindownSec != 1800 {
		t.Errorf("expected 1800, got %d", config.DiskSpindownSec)
	}
	if config.CPUGovernor != "ondemand" {
		t.Errorf("expected ondemand, got %s", config.CPUGovernor)
	}
	if config.TempCheckIntervalSec != 30 {
		t.Errorf("expected 30, got %d", config.TempCheckIntervalSec)
	}
	if config.FanSpeed != 50 {
		t.Errorf("expected 50, got %d", config.FanSpeed)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true")
	}
}

// ========== 边界情况测试 ==========

func TestSpindownAlreadySpundown(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30)

	_ = pm.SpindownDisk("sda")
	err := pm.SpindownDisk("sda") // 再次休眠
	if err != nil {
		t.Errorf("spindown already spundown disk should not error: %v", err)
	}
}

func TestWakeupAlreadyRunning(t *testing.T) {
	pm := NewPowerManager(nil)
	pm.AddDisk("sda", 30) // 默认已旋转

	err := pm.WakeupDisk("sda")
	if err != nil {
		t.Errorf("wakeup already running disk should not error: %v", err)
	}
}

func TestApplyProfile_InvalidFanSpeed(t *testing.T) {
	pm := NewPowerManager(nil)

	profile := &PowerProfile{
		Name:     "测试",
		FanSpeed: -10, // 负数应被忽略
	}

	_ = pm.ApplyProfile(profile)
	config := pm.GetConfig()
	if config.FanSpeed != 50 {
		t.Errorf("expected default FanSpeed=50, got %d", config.FanSpeed)
	}
}

func TestApplyProfile_FanSpeedOver100(t *testing.T) {
	pm := NewPowerManager(nil)

	profile := &PowerProfile{
		Name:     "测试",
		FanSpeed: 150, // 超过100应被忽略
	}

	_ = pm.ApplyProfile(profile)
	config := pm.GetConfig()
	if config.FanSpeed != 50 {
		t.Errorf("expected default FanSpeed=50, got %d", config.FanSpeed)
	}
}
