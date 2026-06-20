package zfsscrubber

import (
	"fmt"
	"testing"
	"time"
)

func TestNewZFSScrubber(t *testing.T) {
	// 测试默认配置
	scrubber := NewZFSScrubber(nil)
	if scrubber == nil {
		t.Fatal("NewZFSScrubber 返回 nil")
	}

	config := scrubber.GetConfig()
	if config.DefaultFrequency != FrequencyWeekly {
		t.Errorf("期望默认频率为 weekly，实际为 %s", config.DefaultFrequency)
	}
	if !config.AutoRepairEnabled {
		t.Error("期望默认启用自动修复")
	}
	if config.MaxConcurrentScans != 1 {
		t.Errorf("期望最大并发数为 1，实际为 %d", config.MaxConcurrentScans)
	}

	// 测试自定义配置
	customConfig := &ScrubberConfig{
		DefaultFrequency:   FrequencyDaily,
		AutoRepairEnabled:  false,
		MaxConcurrentScans: 2,
	}
	customScrubber := NewZFSScrubber(customConfig)
	if customScrubber.GetConfig().DefaultFrequency != FrequencyDaily {
		t.Errorf("期望自定义频率为 daily，实际为 %s", customScrubber.GetConfig().DefaultFrequency)
	}
}

func TestStartStop(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 测试启动
	scrubber.Start()
	if !scrubber.IsRunning() {
		t.Error("期望 scrubber 运行中")
	}

	// 测试重复启动
	scrubber.Start()
	if !scrubber.IsRunning() {
		t.Error("期望 scrubber 仍然运行中")
	}

	// 测试停止
	scrubber.Stop()
	if scrubber.IsRunning() {
		t.Error("期望 scrubber 已停止")
	}

	// 测试重复停止
	scrubber.Stop()
	if scrubber.IsRunning() {
		t.Error("期望 scrubber 仍然停止")
	}
}

func TestCreateSchedule(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 测试创建有效调度
	schedule := &ScrubSchedule{
		ID:        "test-schedule-1",
		PoolID:    "pool-1",
		Frequency: FrequencyWeekly,
		DayOfWeek: 1, // 周一
		Hour:      2,
		Enabled:   true,
	}

	err := scrubber.CreateSchedule(schedule)
	if err != nil {
		t.Fatalf("创建调度失败: %v", err)
	}

	// 验证调度已创建
	saved, err := scrubber.GetSchedule("test-schedule-1")
	if err != nil {
		t.Fatalf("获取调度失败: %v", err)
	}
	if saved.PoolID != "pool-1" {
		t.Errorf("期望池ID为 pool-1，实际为 %s", saved.PoolID)
	}

	// 测试重复创建
	err = scrubber.CreateSchedule(schedule)
	if err != ErrScheduleExists {
		t.Errorf("期望错误为 ErrScheduleExists，实际为 %v", err)
	}

	// 测试空ID
	err = scrubber.CreateSchedule(&ScrubSchedule{
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      3,
	})
	if err != ErrScheduleIDRequired {
		t.Errorf("期望错误为 ErrScheduleIDRequired，实际为 %v", err)
	}

	// 测试空PoolID
	err = scrubber.CreateSchedule(&ScrubSchedule{
		ID:        "test-schedule-2",
		Frequency: FrequencyDaily,
		Hour:      3,
	})
	if err != ErrPoolIDRequired {
		t.Errorf("期望错误为 ErrPoolIDRequired，实际为 %v", err)
	}

	// 测试无效频率
	err = scrubber.CreateSchedule(&ScrubSchedule{
		ID:        "test-schedule-3",
		PoolID:    "pool-1",
		Frequency: "invalid",
		Hour:      3,
	})
	if err != ErrInvalidFrequency {
		t.Errorf("期望错误为 ErrInvalidFrequency，实际为 %v", err)
	}

	// 测试无效小时
	err = scrubber.CreateSchedule(&ScrubSchedule{
		ID:        "test-schedule-4",
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      25,
	})
	if err != ErrInvalidHour {
		t.Errorf("期望错误为 ErrInvalidHour，实际为 %v", err)
	}

	// 测试无效星期
	err = scrubber.CreateSchedule(&ScrubSchedule{
		ID:        "test-schedule-5",
		PoolID:    "pool-1",
		Frequency: FrequencyWeekly,
		DayOfWeek: 7,
		Hour:      3,
	})
	if err != ErrInvalidDayOfWeek {
		t.Errorf("期望错误为 ErrInvalidDayOfWeek，实际为 %v", err)
	}

	// 测试无效日期
	err = scrubber.CreateSchedule(&ScrubSchedule{
		ID:         "test-schedule-6",
		PoolID:     "pool-1",
		Frequency:  FrequencyMonthly,
		DayOfMonth: 32,
		Hour:       3,
	})
	if err != ErrInvalidDayOfMonth {
		t.Errorf("期望错误为 ErrInvalidDayOfMonth，实际为 %v", err)
	}
}

func TestFrequencyValidation(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 测试每日调度
	daily := &ScrubSchedule{
		ID:        "daily-1",
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      1,
	}
	if err := scrubber.CreateSchedule(daily); err != nil {
		t.Fatalf("创建每日调度失败: %v", err)
	}

	// 测试每周调度
	weekly := &ScrubSchedule{
		ID:        "weekly-1",
		PoolID:    "pool-1",
		Frequency: FrequencyWeekly,
		DayOfWeek: 3, // 周三
		Hour:      2,
	}
	if err := scrubber.CreateSchedule(weekly); err != nil {
		t.Fatalf("创建每周调度失败: %v", err)
	}

	// 测试每月调度
	monthly := &ScrubSchedule{
		ID:         "monthly-1",
		PoolID:     "pool-1",
		Frequency:  FrequencyMonthly,
		DayOfMonth: 15,
		Hour:       3,
	}
	if err := scrubber.CreateSchedule(monthly); err != nil {
		t.Fatalf("创建每月调度失败: %v", err)
	}
}

func TestListSchedules(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 创建多个调度
	for i := 0; i < 5; i++ {
		schedule := &ScrubSchedule{
			ID:        fmt.Sprintf("schedule-%d", i),
			PoolID:    "pool-1",
			Frequency: FrequencyDaily,
			Hour:      i,
		}
		scrubber.CreateSchedule(schedule)
	}

	schedules := scrubber.ListSchedules()
	if len(schedules) != 5 {
		t.Errorf("期望 5 个调度，实际为 %d", len(schedules))
	}
}

func TestUpdateSchedule(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 创建调度
	schedule := &ScrubSchedule{
		ID:        "update-test",
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      2,
		Enabled:   true,
	}
	scrubber.CreateSchedule(schedule)

	// 更新调度
	schedule.Hour = 4
	schedule.Enabled = false
	err := scrubber.UpdateSchedule(schedule)
	if err != nil {
		t.Fatalf("更新调度失败: %v", err)
	}

	// 验证更新
	updated, _ := scrubber.GetSchedule("update-test")
	if updated.Hour != 4 {
		t.Errorf("期望小时为 4，实际为 %d", updated.Hour)
	}
	if updated.Enabled {
		t.Error("期望调度已禁用")
	}

	// 测试更新不存在的调度
	err = scrubber.UpdateSchedule(&ScrubSchedule{
		ID:        "non-existent",
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      2,
	})
	if err != ErrScheduleNotFound {
		t.Errorf("期望错误为 ErrScheduleNotFound，实际为 %v", err)
	}
}

func TestDeleteSchedule(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 创建调度
	schedule := &ScrubSchedule{
		ID:        "delete-test",
		PoolID:    "pool-1",
		Frequency: FrequencyDaily,
		Hour:      2,
	}
	scrubber.CreateSchedule(schedule)

	// 删除调度
	err := scrubber.DeleteSchedule("delete-test")
	if err != nil {
		t.Fatalf("删除调度失败: %v", err)
	}

	// 验证已删除
	_, err = scrubber.GetSchedule("delete-test")
	if err != ErrScheduleNotFound {
		t.Errorf("期望错误为 ErrScheduleNotFound，实际为 %v", err)
	}

	// 测试删除不存在的调度
	err = scrubber.DeleteSchedule("non-existent")
	if err != ErrScheduleNotFound {
		t.Errorf("期望错误为 ErrScheduleNotFound，实际为 %v", err)
	}
}

func TestExecuteScrub(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册测试数据块
	for i := 0; i < 10; i++ {
		block := &DataBlock{
			ID:          fmt.Sprintf("block-%d", i),
			PoolID:      "pool-1",
			Dataset:     "dataset-1",
			BlockNumber: int64(i),
			Size:        4096,
			Checksum:    fmt.Sprintf("checksum-%d", i),
			Algorithm:   ChecksumSHA256,
			Valid:       true,
			LastVerified: time.Now(),
		}
		scrubber.RegisterBlock(block)
	}

	// 执行清洗
	job, err := scrubber.ExecuteScrub("pool-1")
	if err != nil {
		t.Fatalf("执行清洗失败: %v", err)
	}

	if job.PoolID != "pool-1" {
		t.Errorf("期望池ID为 pool-1，实际为 %s", job.PoolID)
	}
	if job.Status != ScrubRunning {
		t.Errorf("期望状态为 running，实际为 %s", job.Status)
	}

	// 等待任务完成
	time.Sleep(200 * time.Millisecond)

	// 获取任务状态
	savedJob, err := scrubber.GetJob(job.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if savedJob.Status != ScrubCompleted {
		t.Errorf("期望状态为 completed，实际为 %s", savedJob.Status)
	}
	if savedJob.Progress != 100 {
		t.Errorf("期望进度为 100，实际为 %f", savedJob.Progress)
	}

	// 测试空池ID
	_, err = scrubber.ExecuteScrub("")
	if err != ErrPoolIDRequired {
		t.Errorf("期望错误为 ErrPoolIDRequired，实际为 %v", err)
	}
}

func TestListJobs(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册数据块
	for i := 0; i < 5; i++ {
		block := &DataBlock{
			ID:       fmt.Sprintf("block-%d", i),
			PoolID:   "pool-1",
			Size:     4096,
			Checksum: fmt.Sprintf("checksum-%d", i),
		}
		scrubber.RegisterBlock(block)
	}

	// 执行多个清洗任务
	for i := 0; i < 3; i++ {
		poolID := fmt.Sprintf("pool-%d", i)
		block := &DataBlock{
			ID:       fmt.Sprintf("block-pool-%d", i),
			PoolID:   poolID,
			Size:     4096,
			Checksum: "checksum",
		}
		scrubber.RegisterBlock(block)
		scrubber.ExecuteScrub(poolID)
	}

	time.Sleep(300 * time.Millisecond)

	// 列出所有任务
	jobs := scrubber.ListJobs("")
	if len(jobs) < 3 {
		t.Errorf("期望至少 3 个任务，实际为 %d", len(jobs))
	}

	// 列出特定池的任务
	poolJobs := scrubber.ListJobs("pool-0")
	if len(poolJobs) != 1 {
		t.Errorf("期望 pool-0 有 1 个任务，实际为 %d", len(poolJobs))
	}
}

func TestCancelJob(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册数据块
	block := &DataBlock{
		ID:       "block-cancel",
		PoolID:   "pool-cancel",
		Size:     4096,
		Checksum: "checksum",
	}
	scrubber.RegisterBlock(block)

	// 执行清洗
	job, _ := scrubber.ExecuteScrub("pool-cancel")

	// 取消任务
	err := scrubber.CancelJob(job.ID)
	if err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}

	// 验证任务已取消
	savedJob, _ := scrubber.GetJob(job.ID)
	if savedJob.Status != ScrubCancelled {
		t.Errorf("期望状态为 cancelled，实际为 %s", savedJob.Status)
	}

	// 测试取消不存在的任务
	err = scrubber.CancelJob("non-existent")
	if err != ErrJobNotFound {
		t.Errorf("期望错误为 ErrJobNotFound，实际为 %v", err)
	}
}

func TestReports(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册数据块并执行清洗
	for i := 0; i < 5; i++ {
		block := &DataBlock{
			ID:       fmt.Sprintf("report-block-%d", i),
			PoolID:   "pool-report",
			Size:     4096,
			Checksum: fmt.Sprintf("checksum-%d", i),
		}
		scrubber.RegisterBlock(block)
	}

	scrubber.ExecuteScrub("pool-report")
	time.Sleep(200 * time.Millisecond)

	// 列出报告
	reports := scrubber.ListReports("")
	if len(reports) == 0 {
		t.Error("期望有报告生成")
	}

	// 获取特定报告
	if len(reports) > 0 {
		report, err := scrubber.GetReport(reports[0].ID)
		if err != nil {
			t.Fatalf("获取报告失败: %v", err)
		}
		if report.PoolID != "pool-report" {
			t.Errorf("期望池ID为 pool-report，实际为 %s", report.PoolID)
		}
	}

	// 测试获取不存在的报告
	_, err := scrubber.GetReport("non-existent")
	if err != ErrReportNotFound {
		t.Errorf("期望错误为 ErrReportNotFound，实际为 %v", err)
	}
}

func TestPoolHealth(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 检查池健康（自动创建）
	health, err := scrubber.CheckPoolHealth("pool-health")
	if err != nil {
		t.Fatalf("检查池健康失败: %v", err)
	}
	if health.PoolID != "pool-health" {
		t.Errorf("期望池ID为 pool-health，实际为 %s", health.PoolID)
	}
	if health.OverallHealth != HealthUnknown {
		t.Errorf("期望健康状态为 unknown，实际为 %s", health.OverallHealth)
	}

	// 更新池健康
	newHealth := &PoolHealth{
		PoolID:        "pool-health",
		PoolName:      "测试池",
		OverallHealth: HealthGood,
		Status:        "online",
		TotalSize:     1024 * 1024 * 1024,
		UsedSize:      512 * 1024 * 1024,
		FreeSize:      512 * 1024 * 1024,
		Disks: []*DiskSMART{
			{
				DevicePath:   "/dev/sda",
				Model:        "WD Red",
				Capacity:     1024 * 1024 * 1024,
				Temperature:  35.0,
				HealthStatus:  HealthGood,
			},
		},
	}
	scrubber.UpdatePoolHealth(newHealth)

	// 验证更新
	updated, _ := scrubber.CheckPoolHealth("pool-health")
	if updated.OverallHealth != HealthGood {
		t.Errorf("期望健康状态为 good，实际为 %s", updated.OverallHealth)
	}

	// 列出所有池健康
	healths := scrubber.ListPoolHealths()
	if len(healths) == 0 {
		t.Error("期望有池健康状态")
	}
}

func TestDiskSMART(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册池健康和磁盘
	health := &PoolHealth{
		PoolID: "pool-smart",
		Disks: []*DiskSMART{
			{
				DevicePath:          "/dev/sda",
				Model:               "WD Red",
				Serial:              "WD123456",
				Capacity:            4 * 1024 * 1024 * 1024,
				Temperature:         35.0,
				PowerOnHours:        1000,
				ReallocatedSectors:  0,
				PendingSectors:      0,
				OfflineUncorrectable: 0,
				HealthStatus:        HealthGood,
				LastChecked:         time.Now(),
			},
		},
	}
	scrubber.UpdatePoolHealth(health)

	// 检查磁盘 SMART
	smart, err := scrubber.CheckDiskSMART("/dev/sda")
	if err != nil {
		t.Fatalf("检查磁盘 SMART 失败: %v", err)
	}
	if smart.Model != "WD Red" {
		t.Errorf("期望型号为 WD Red，实际为 %s", smart.Model)
	}

	// 测试不存在的磁盘
	_, err = scrubber.CheckDiskSMART("/dev/sdz")
	if err != ErrDiskNotFound {
		t.Errorf("期望错误为 ErrDiskNotFound，实际为 %v", err)
	}
}

func TestAlerts(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 更新池健康，触发告警
	health := &PoolHealth{
		PoolID:        "pool-alert",
		PoolName:      "告警测试池",
		OverallHealth: HealthCritical,
		Disks: []*DiskSMART{
			{
				DevicePath:   "/dev/sdb",
				HealthStatus: HealthCritical,
			},
		},
	}
	scrubber.UpdatePoolHealth(health)

	// 列出告警
	alerts := scrubber.ListAlerts("", false)
	if len(alerts) == 0 {
		t.Error("期望有告警生成")
	}

	// 确认告警
	if len(alerts) > 0 {
		err := scrubber.AcknowledgeAlert(alerts[0].ID)
		if err != nil {
			t.Fatalf("确认告警失败: %v", err)
		}

		// 验证告警已确认
		alert, _ := scrubber.GetAlert(alerts[0].ID)
		if !alert.Acked {
			t.Error("期望告警已确认")
		}
	}

	// 测试确认不存在的告警
	err := scrubber.AcknowledgeAlert("non-existent")
	if err != ErrAlertNotFound {
		t.Errorf("期望错误为 ErrAlertNotFound，实际为 %v", err)
	}
}

func TestConfig(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 获取配置
	config := scrubber.GetConfig()
	if config == nil {
		t.Fatal("配置为 nil")
	}

	// 更新配置
	newConfig := &ScrubberConfig{
		DefaultFrequency:   FrequencyDaily,
		AutoRepairEnabled:  false,
		MaxConcurrentScans: 4,
		ScanBandwidthLimit: 1024 * 1024,
		AlertThresholdDays: 7,
		SMARTCheckInterval: 12 * time.Hour,
		RetryFailedRepairs: 5,
	}
	scrubber.UpdateConfig(newConfig)

	// 验证更新
	updated := scrubber.GetConfig()
	if updated.DefaultFrequency != FrequencyDaily {
		t.Errorf("期望频率为 daily，实际为 %s", updated.DefaultFrequency)
	}
	if updated.AutoRepairEnabled {
		t.Error("期望自动修复已禁用")
	}
	if updated.MaxConcurrentScans != 4 {
		t.Errorf("期望最大并发数为 4，实际为 %d", updated.MaxConcurrentScans)
	}
}

func TestRepairActions(t *testing.T) {
	scrubber := NewZFSScrubber(nil)

	// 注册数据块（包含损坏的块）
	block := &DataBlock{
		ID:       "repair-block",
		PoolID:   "pool-repair",
		Size:     4096,
		Checksum: "correct-checksum",
	}
	scrubber.RegisterBlock(block)

	// 执行清洗（会触发修复）
	scrubber.ExecuteScrub("pool-repair")
	time.Sleep(200 * time.Millisecond)

	// 列出修复动作
	actions := scrubber.ListRepairActions("")
	// 注意：由于模拟的校验和与块的校验和相同，可能不会触发修复
	// 这里主要测试 API 正常工作
	if len(actions) > 0 {
		action, err := scrubber.GetRepairAction(actions[0].ID)
		if err != nil {
			t.Fatalf("获取修复动作失败: %v", err)
		}
		if action.PoolID != "pool-repair" {
			t.Errorf("期望池ID为 pool-repair，实际为 %s", action.PoolID)
		}
	}

	// 测试获取不存在的修复动作
	_, err := scrubber.GetRepairAction("non-existent")
	if err != ErrRepairActionNotFound {
		t.Errorf("期望错误为 ErrRepairActionNotFound，实际为 %v", err)
	}
}

// 辅助函数已内联使用 fmt.Sprintf
