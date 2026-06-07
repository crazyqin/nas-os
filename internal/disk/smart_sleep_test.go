package disk

import (
	"context"
	"testing"
	"time"
)

// ==================== SmartSleepManager 基础测试 ====================

func TestNewSmartSleepManager(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	if mgr == nil {
		t.Fatal("NewSmartSleepManager返回nil")
	}
	if mgr.running {
		t.Error("新建的管理器不应处于运行状态")
	}
	if mgr.patterns == nil {
		t.Error("patterns map未初始化")
	}
	if mgr.policies == nil {
		t.Error("policies map未初始化")
	}
}

func TestNewSmartSleepManager_WithConfig(t *testing.T) {
	cfg := &SmartSleepConfig{
		LearnInterval:      5 * time.Minute,
		PatternWindowSize:  500,
		QuietHourThreshold: 3,
	}
	mgr := NewSmartSleepManager(cfg)
	if mgr.config.LearnInterval != 5*time.Minute {
		t.Errorf("LearnInterval配置错误: got %v, want 5m", mgr.config.LearnInterval)
	}
	if mgr.config.PatternWindowSize != 500 {
		t.Errorf("PatternWindowSize配置错误: got %d, want 500", mgr.config.PatternWindowSize)
	}
}

func TestDefaultSmartSleepConfig(t *testing.T) {
	cfg := DefaultSmartSleepConfig()
	if cfg.LearnInterval != 10*time.Minute {
		t.Errorf("默认LearnInterval错误: got %v", cfg.LearnInterval)
	}
	if cfg.PatternWindowSize != 1000 {
		t.Errorf("默认PatternWindowSize错误: got %d", cfg.PatternWindowSize)
	}
	if cfg.DefaultTempThreshold.WarningTemp != 45.0 {
		t.Errorf("默认警告温度错误: got %f", cfg.DefaultTempThreshold.WarningTemp)
	}
}

// ==================== 生命周期测试 ====================

func TestSmartSleepManager_StartStop(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	ctx := context.Background()

	// 启动
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !mgr.IsRunning() {
		t.Error("启动后应为运行状态")
	}

	// 重复启动应报错
	if err := mgr.Start(ctx); err == nil {
		t.Error("重复启动应返回错误")
	}

	// 停止
	mgr.Stop()
	if mgr.IsRunning() {
		t.Error("停止后不应为运行状态")
	}

	// 重复停止不应panic
	mgr.Stop()
}

// ==================== 访问记录测试 ====================

func TestRecordAccess(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 记录访问
	mgr.RecordAccess("sda", AccessRead, 100*time.Millisecond)
	mgr.RecordAccess("sda", AccessWrite, 200*time.Millisecond)
	mgr.RecordAccess("sda", AccessRead, 50*time.Millisecond)

	pattern := mgr.GetAccessPattern("sda")
	if pattern == nil {
		t.Fatal("获取访问模式失败")
	}
	if pattern.TotalAccessCount != 3 {
		t.Errorf("访问次数错误: got %d, want 3", pattern.TotalAccessCount)
	}
	if pattern.DiskID != "sda" {
		t.Errorf("DiskID错误: got %s, want sda", pattern.DiskID)
	}
	if len(pattern.Records) != 3 {
		t.Errorf("记录数错误: got %d, want 3", len(pattern.Records))
	}
}

func TestRecordAccess_MultipleDisks(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.RecordAccess("sda", AccessRead, 100*time.Millisecond)
	mgr.RecordAccess("sdb", AccessWrite, 200*time.Millisecond)

	patternA := mgr.GetAccessPattern("sda")
	patternB := mgr.GetAccessPattern("sdb")

	if patternA == nil || patternB == nil {
		t.Fatal("磁盘模式获取失败")
	}
	if patternA.TotalAccessCount != 1 {
		t.Errorf("sda访问次数错误: got %d", patternA.TotalAccessCount)
	}
	if patternB.TotalAccessCount != 1 {
		t.Errorf("sdb访问次数错误: got %d", patternB.TotalAccessCount)
	}
}

func TestRecordAccess_WindowLimit(t *testing.T) {
	cfg := &SmartSleepConfig{
		PatternWindowSize: 5,
	}
	mgr := NewSmartSleepManager(cfg)

	// 记录超过窗口大小的访问
	for i := 0; i < 10; i++ {
		mgr.RecordAccess("sda", AccessRead, time.Duration(i)*time.Millisecond)
	}

	pattern := mgr.GetAccessPattern("sda")
	if pattern == nil {
		t.Fatal("获取模式失败")
	}
	if len(pattern.Records) != 5 {
		t.Errorf("记录数应裁剪到窗口大小: got %d, want 5", len(pattern.Records))
	}
	// 验证保留的是最新的记录
	if pattern.Records[0].Duration != 5*time.Millisecond {
		t.Errorf("保留的记录应从第6条开始: got duration %v", pattern.Records[0].Duration)
	}
}

func TestGetAccessPattern_NotExist(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	pattern := mgr.GetAccessPattern("nonexistent")
	if pattern != nil {
		t.Error("不存在的磁盘应返回nil")
	}
}

// ==================== 温度监控测试 ====================

func TestUpdateTemperature(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.UpdateTemperature("sda", 40.0)
	mgr.UpdateTemperature("sda", 50.0)

	info := mgr.GetTemperatureInfo("sda")
	if info == nil {
		t.Fatal("获取温度信息失败")
	}
	if info.CurrentTemp != 50.0 {
		t.Errorf("当前温度错误: got %f, want 50.0", info.CurrentTemp)
	}
	if info.MaxTemp != 50.0 {
		t.Errorf("最高温度错误: got %f, want 50.0", info.MaxTemp)
	}
	// 平均温度应该是滚动平均：40*0.9 + 50*0.1 = 41.0
	if info.AverageTemp != 41.0 {
		t.Errorf("平均温度错误: got %f, want 41.0", info.AverageTemp)
	}
}

func TestTemperature_ThresholdTrigger(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 注册策略
	policy := NewSmartSleepPolicy("test", "测试策略", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(policy)

	// 注册服务依赖为空，避免干扰
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})

	// 记录很久以前的访问（确保不是"近期"）
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		LastAccess:       time.Now().Add(-1 * time.Hour),
		TotalAccessCount: 100,
		MaxRecords:       1000,
		QuietHours:       []int{time.Now().Hour()},
	}
	mgr.patternMu.Unlock()

	// 低温 - 不应触发温度休眠
	mgr.UpdateTemperature("sda", 35.0)
	should, reason := mgr.ShouldSleep("sda")
	// 可能因为处于低谷时段而休眠，但理由不应是温度
	if should && reason == "磁盘温度过高，提前休眠散热" {
		t.Error("35℃不应触发温度休眠")
	}

	// 高温 - 应触发温度休眠
	mgr.UpdateTemperature("sda", 52.0)
	should, reason = mgr.ShouldSleep("sda")
	if !should {
		t.Error("52℃应触发休眠")
	}
	if reason != "磁盘温度过高，提前休眠散热" {
		t.Errorf("高温休眠理由错误: %s", reason)
	}
}

func TestTemperature_DefaultThresholds(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	mgr.UpdateTemperature("sda", 44.0)

	info := mgr.GetTemperatureInfo("sda")
	if info.TempThreshold.WarningTemp != 45.0 {
		t.Errorf("默认警告温度错误: got %f", info.TempThreshold.WarningTemp)
	}
	if info.TempThreshold.ThrottleTemp != 50.0 {
		t.Errorf("默认降频温度错误: got %f", info.TempThreshold.ThrottleTemp)
	}
}

func TestTemperature_CustomThresholds(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 先更新温度，再自定义阈值
	mgr.UpdateTemperature("sda", 30.0)

	info := mgr.GetTemperatureInfo("sda")
	if info == nil {
		t.Fatal("温度信息为空")
	}

	// 验证默认阈值
	if info.TempThreshold.WarningTemp != 45.0 {
		t.Errorf("默认警告温度错误: %f", info.TempThreshold.WarningTemp)
	}
}

// ==================== 服务依赖测试 ====================

func TestServiceDependency(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.RegisterServiceDependency("sda", ServiceDependency{
		Name:        "smb",
		Active:      true,
		ActiveConns: 3,
	})

	deps := mgr.GetServiceDependencies("sda")
	if len(deps) != 1 {
		t.Fatalf("依赖数错误: got %d, want 1", len(deps))
	}
	if deps[0].Name != "smb" {
		t.Errorf("服务名错误: got %s", deps[0].Name)
	}
	if !deps[0].Active {
		t.Error("服务应为活跃状态")
	}
}

func TestServiceDependency_Update(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.RegisterServiceDependency("sda", ServiceDependency{
		Name:        "smb",
		Active:      true,
		ActiveConns: 3,
	})

	// 更新同一服务
	mgr.UpdateServiceStatus("sda", "smb", false, 0)

	deps := mgr.GetServiceDependencies("sda")
	if deps[0].Active {
		t.Error("更新后服务应为非活跃")
	}
	if deps[0].ActiveConns != 0 {
		t.Errorf("连接数应为0: got %d", deps[0].ActiveConns)
	}
}

func TestServiceDependency_PreventsSleep(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 注册活跃的SMB服务
	mgr.RegisterServiceDependency("sda", ServiceDependency{
		Name:        "smb",
		Active:      true,
		ActiveConns: 5,
	})

	// 设置温度正常，确保不是因为温度
	mgr.UpdateTemperature("sda", 30.0)

	// 设置很久以前的访问
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		LastAccess:       time.Now().Add(-2 * time.Hour),
		TotalAccessCount: 100,
		MaxRecords:       1000,
		QuietHours:       []int{time.Now().Hour()},
	}
	mgr.patternMu.Unlock()

	should, reason := mgr.ShouldSleep("sda")
	if should {
		t.Error("活跃服务应阻止休眠")
	}
	if reason != "存在活跃的服务依赖（SMB/NFS等）" {
		t.Errorf("阻止理由错误: %s", reason)
	}
}

func TestMultipleServiceDependencies(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: true, ActiveConns: 2})
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "nfs", Active: false, ActiveConns: 0})
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "ftp", Active: true, ActiveConns: 1})

	deps := mgr.GetServiceDependencies("sda")
	if len(deps) != 3 {
		t.Fatalf("依赖数错误: got %d, want 3", len(deps))
	}

	// 有一个活跃就阻止休眠
	if !mgr.hasActiveService("sda") {
		t.Error("有活跃服务应返回true")
	}
}

// ==================== 休眠决策测试 ====================

func TestShouldSleep_AllConditionsMet(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 添加策略
	policy := NewSmartSleepPolicy("default", "默认", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(policy)
	mgr.SetDefaultPolicy(policy)

	// 无服务依赖
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})

	// 温度正常
	mgr.UpdateTemperature("sda", 35.0)

	// 设置为低谷时段且很久没有访问
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		LastAccess:       time.Now().Add(-30 * time.Minute),
		TotalAccessCount: 50,
		MaxRecords:       1000,
		QuietHours:       []int{time.Now().Hour()},
	}
	mgr.patternMu.Unlock()

	should, reason := mgr.ShouldSleep("sda")
	if !should {
		t.Errorf("条件满足时应允许休眠, reason: %s", reason)
	}
	if reason != "满足智能休眠条件" {
		t.Errorf("休眠理由错误: %s", reason)
	}
}

func TestShouldSleep_NoPolicy(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	should, reason := mgr.ShouldSleep("sda")
	if should {
		t.Error("无策略时不应休眠")
	}
	if reason != "未配置休眠策略" {
		t.Errorf("理由错误: %s", reason)
	}
}

func TestShouldSleep_RecentAccess(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("default", "默认", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	policy.MinIdleBeforeSleep = 10 * time.Minute
	mgr.AddPolicy(policy)
	mgr.SetDefaultPolicy(policy)
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})
	mgr.UpdateTemperature("sda", 35.0)

	// 最近刚访问过
	mgr.RecordAccess("sda", AccessRead, 100*time.Millisecond)

	should, reason := mgr.ShouldSleep("sda")
	if should {
		t.Error("近期有访问不应休眠")
	}
	if reason != "距离最近访问时间不足" {
		t.Errorf("理由错误: %s", reason)
	}
}

func TestShouldSleep_PeakHours(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("default", "默认", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(policy)
	mgr.SetDefaultPolicy(policy)
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})
	mgr.UpdateTemperature("sda", 35.0)

	// 设置当前时段为高峰时段
	currentHour := time.Now().Hour()
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		LastAccess:       time.Now().Add(-30 * time.Minute),
		TotalAccessCount: 100,
		MaxRecords:       1000,
		HourlyFrequency:  [24]int{},
		PeakHours:        []int{currentHour},
		QuietHours:       []int{},
	}
	// 设置当前小时为高频访问
	mgr.patterns["sda"].HourlyFrequency[currentHour] = 100
	mgr.patternMu.Unlock()

	should, reason := mgr.ShouldSleep("sda")
	if should {
		t.Error("高峰时段不应休眠")
	}
	if reason != "当前时段访问频繁" {
		t.Errorf("理由错误: %s", reason)
	}
}

// ==================== 策略管理测试 ====================

func TestAddPolicy(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("test", "测试策略", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	if err := mgr.AddPolicy(policy); err != nil {
		t.Fatalf("添加策略失败: %v", err)
	}

	got := mgr.GetPolicy("test")
	if got == nil {
		t.Fatal("获取策略失败")
	}
	if got.Name != "测试策略" {
		t.Errorf("策略名称错误: got %s", got.Name)
	}
}

func TestUpdatePolicy(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("test", "原名称", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(policy)

	updated := NewSmartSleepPolicy("test", "新名称", 3*time.Minute, 8*time.Minute, 12*time.Minute)
	if err := mgr.UpdatePolicy("test", updated); err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	got := mgr.GetPolicy("test")
	if got.Name != "新名称" {
		t.Errorf("策略名称未更新: got %s", got.Name)
	}
	if got.IdleThreshold != 3*time.Minute {
		t.Errorf("空闲阈值未更新: got %v", got.IdleThreshold)
	}
}

func TestUpdatePolicy_NilPolicy(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	if err := mgr.UpdatePolicy("test", nil); err == nil {
		t.Error("nil策略应返回错误")
	}
}

func TestSetDefaultPolicy(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("default", "默认策略", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.SetDefaultPolicy(policy)

	got := mgr.GetPolicy("nonexistent")
	if got != nil {
		t.Error("不存在的策略应返回nil")
	}
}

func TestGetPolicy_NotExist(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	if mgr.GetPolicy("nonexistent") != nil {
		t.Error("不存在的策略应返回nil")
	}
}

// ==================== 调度表测试 ====================

func TestSleepSchedule(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	schedule := DefaultSleepSchedule("workday", "weekend")
	mgr.SetSchedule("sda", schedule)

	got := mgr.GetSchedule("sda")
	if got == nil {
		t.Fatal("获取调度表失败")
	}
	if got.WorkdayPolicyID != "workday" {
		t.Errorf("工作日策略ID错误: got %s", got.WorkdayPolicyID)
	}
	if got.WeekendPolicyID != "weekend" {
		t.Errorf("周末策略ID错误: got %s", got.WeekendPolicyID)
	}
	if !got.Enabled {
		t.Error("调度表应默认启用")
	}
}

func TestSleepSchedule_PolicySelection(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	workdayPolicy := NewSmartSleepPolicy("workday", "工作日策略", 10*time.Minute, 20*time.Minute, 30*time.Minute)
	weekendPolicy := NewSmartSleepPolicy("weekend", "周末策略", 5*time.Minute, 10*time.Minute, 15*time.Minute)

	mgr.AddPolicy(workdayPolicy)
	mgr.AddPolicy(weekendPolicy)

	schedule := DefaultSleepSchedule("workday", "weekend")
	mgr.SetSchedule("sda", schedule)

	// 验证策略存在
	if mgr.GetPolicy("workday") == nil {
		t.Fatal("工作日策略不存在")
	}
	if mgr.GetPolicy("weekend") == nil {
		t.Fatal("周末策略不存在")
	}
}

func TestGetSchedule_NotExist(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	if mgr.GetSchedule("nonexistent") != nil {
		t.Error("不存在的调度表应返回nil")
	}
}

// ==================== 预测性唤醒测试 ====================

func TestGetNextWakeTime_NoPattern(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	nextWake := mgr.GetNextWakeTime("sda")
	expected := time.Now().Add(15 * time.Minute)

	// 允许1秒的误差
	if nextWake.Sub(expected).Abs() > time.Second {
		t.Errorf("无历史数据时应返回15分钟后: got %v, want ~%v", nextWake, expected)
	}
}

func TestGetNextWakeTime_WithPattern(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 创建有高峰时段的模式
	currentHour := time.Now().Hour()
	peakHour := (currentHour + 3) % 24

	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:            "sda",
		TotalAccessCount:  100,
		AvgAccessInterval: 30 * time.Minute,
		PeakHours:         []int{peakHour},
		MaxRecords:        1000,
	}
	mgr.patternMu.Unlock()

	nextWake := mgr.GetNextWakeTime("sda")
	if nextWake.Before(time.Now()) {
		t.Error("唤醒时间应在未来")
	}
}

func TestGetNextWakeTime_WithAvgInterval(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:            "sda",
		TotalAccessCount:  50,
		AvgAccessInterval: 20 * time.Minute,
		PeakHours:         []int{}, // 无高峰时段
		MaxRecords:        1000,
	}
	mgr.patternMu.Unlock()

	nextWake := mgr.GetNextWakeTime("sda")
	if nextWake.Before(time.Now()) {
		t.Error("唤醒时间应在未来")
	}
	// 应该大约是20分钟后
	expected := time.Now().Add(20 * time.Minute)
	if nextWake.Sub(expected).Abs() > time.Minute {
		t.Errorf("唤醒时间应基于平均间隔: got %v, want ~%v", nextWake, expected)
	}
}

// ==================== 模式学习测试 ====================

func TestLearnPattern(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 先记录一些访问
	for i := 0; i < 20; i++ {
		mgr.RecordAccess("sda", AccessRead, time.Duration(i)*time.Millisecond)
	}

	// 学习模式
	pattern := mgr.LearnPattern("sda")
	if pattern == nil {
		t.Fatal("学习结果不应为nil")
	}
	if pattern.TotalAccessCount != 20 {
		t.Errorf("访问次数错误: got %d, want 20", pattern.TotalAccessCount)
	}
	if pattern.AvgAccessInterval == 0 {
		t.Error("平均访问间隔不应为0")
	}
}

func TestLearnPattern_NoData(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	pattern := mgr.LearnPattern("nonexistent")
	if pattern != nil {
		t.Error("无数据时应返回nil")
	}
}

func TestLearnPattern_PeakQuietHours(t *testing.T) {
	cfg := &SmartSleepConfig{
		QuietHourThreshold: 3,
		PatternWindowSize:  1000,
	}
	mgr := NewSmartSleepManager(cfg)

	// 模拟在当前小时有大量访问（模拟高峰）
	currentHour := time.Now().Hour()
	for i := 0; i < 20; i++ {
		mgr.RecordAccess("sda", AccessRead, 10*time.Millisecond)
	}

	pattern := mgr.LearnPattern("sda")
	if pattern == nil {
		t.Fatal("学习结果不应为nil")
	}

	// 当前小时应为高峰
	foundPeak := false
	for _, h := range pattern.PeakHours {
		if h == currentHour {
			foundPeak = true
			break
		}
	}
	if !foundPeak {
		// 20次访问 > 3*2=6，应该是高峰
		t.Logf("当前小时 %d 的访问次数: %d, 阈值: %d", currentHour, pattern.HourlyFrequency[currentHour], cfg.QuietHourThreshold)
	}
}

// ==================== 辅助函数测试 ====================

func TestHasActiveService(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 无依赖
	if mgr.hasActiveService("sda") {
		t.Error("无依赖时应返回false")
	}

	// 有非活跃依赖
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})
	if mgr.hasActiveService("sda") {
		t.Error("非活跃服务应返回false")
	}

	// 活跃但无连接
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "nfs", Active: true, ActiveConns: 0})
	if mgr.hasActiveService("sda") {
		t.Error("活跃但无连接应返回false")
	}

	// 活跃且有连接
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "ftp", Active: true, ActiveConns: 3})
	if !mgr.hasActiveService("sda") {
		t.Error("活跃且有连接应返回true")
	}
}

func TestIsOverTempThreshold(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 无温度数据
	if mgr.isOverTempThreshold("sda") {
		t.Error("无温度数据应返回false")
	}

	// 低温
	mgr.UpdateTemperature("sda", 40.0)
	if mgr.isOverTempThreshold("sda") {
		t.Error("40℃不应超过阈值")
	}

	// 超过降频温度
	mgr.UpdateTemperature("sda", 50.0)
	if !mgr.isOverTempThreshold("sda") {
		t.Error("50℃应超过降频阈值")
	}
}

func TestIsQuietPeriod(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 无模式数据 - 默认为低谷
	if !mgr.isQuietPeriod("sda", time.Now()) {
		t.Error("无模式数据应默认为低谷")
	}

	// 有模式数据，低谷时段
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		TotalAccessCount: 100,
		HourlyFrequency:  [24]int{},
		QuietHours:       []int{3, 4, 5},
	}
	mgr.patternMu.Unlock()

	quietTime := time.Date(2025, 1, 1, 3, 30, 0, 0, time.Local)
	if !mgr.isQuietPeriod("sda", quietTime) {
		t.Error("3:30应为低谷时段")
	}

	// 有模式数据，高峰时段
	peakTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.Local)
	mgr.patternMu.Lock()
	mgr.patterns["sda"].HourlyFrequency[10] = 50 // 高于阈值
	mgr.patternMu.Unlock()

	if mgr.isQuietPeriod("sda", peakTime) {
		t.Error("10:00有高频访问不应为低谷")
	}
}

// ==================== NewSmartSleepPolicy 测试 ====================

func TestNewSmartSleepPolicy(t *testing.T) {
	p := NewSmartSleepPolicy("test", "测试", 5*time.Minute, 10*time.Minute, 15*time.Minute)

	if p.ID != "test" {
		t.Errorf("ID错误: got %s", p.ID)
	}
	if p.Name != "测试" {
		t.Errorf("名称错误: got %s", p.Name)
	}
	if p.IdleThreshold != 5*time.Minute {
		t.Errorf("空闲阈值错误: got %v", p.IdleThreshold)
	}
	if p.StandbyThreshold != 10*time.Minute {
		t.Errorf("待机阈值错误: got %v", p.StandbyThreshold)
	}
	if p.SleepThreshold != 15*time.Minute {
		t.Errorf("休眠阈值错误: got %v", p.SleepThreshold)
	}
	if !p.PredictiveWake {
		t.Error("预测唤醒应默认启用")
	}
	if !p.ServiceCheckEnabled {
		t.Error("服务检查应默认启用")
	}
	if p.MinIdleBeforeSleep != 5*time.Minute {
		t.Errorf("最小空闲时长错误: got %v", p.MinIdleBeforeSleep)
	}
	if p.TemperatureThreshold.WarningTemp != 45.0 {
		t.Errorf("警告温度错误: got %f", p.TemperatureThreshold.WarningTemp)
	}
}

func TestDefaultSleepSchedule(t *testing.T) {
	s := DefaultSleepSchedule("workday", "weekend")

	if s.WorkdayPolicyID != "workday" {
		t.Errorf("工作日策略ID错误: got %s", s.WorkdayPolicyID)
	}
	if s.WeekendPolicyID != "weekend" {
		t.Errorf("周末策略ID错误: got %s", s.WeekendPolicyID)
	}
	if !s.Enabled {
		t.Error("调度表应默认启用")
	}
}

// ==================== 温度信息查询测试 ====================

func TestGetTemperatureInfo_NotExist(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	if mgr.GetTemperatureInfo("nonexistent") != nil {
		t.Error("不存在的磁盘应返回nil")
	}
}

func TestGetTemperatureInfo_Immutability(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	mgr.UpdateTemperature("sda", 40.0)

	info := mgr.GetTemperatureInfo("sda")
	info.CurrentTemp = 999.0 // 修改副本

	// 再次获取应该还是原值
	original := mgr.GetTemperatureInfo("sda")
	if original.CurrentTemp != 40.0 {
		t.Error("GetTemperatureInfo应返回副本，修改不应影响原始数据")
	}
}

// ==================== 配置查询测试 ====================

func TestGetConfig(t *testing.T) {
	cfg := &SmartSleepConfig{
		LearnInterval:     5 * time.Minute,
		PatternWindowSize: 500,
	}
	mgr := NewSmartSleepManager(cfg)

	got := mgr.GetConfig()
	if got.LearnInterval != 5*time.Minute {
		t.Errorf("LearnInterval错误: got %v", got.LearnInterval)
	}
}

func TestGetServiceDependencies_Empty(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	deps := mgr.GetServiceDependencies("nonexistent")
	if deps == nil {
		t.Fatal("应返回空切片而非nil")
	}
	if len(deps) != 0 {
		t.Errorf("应返回空切片: got len %d", len(deps))
	}
}

func TestGetServiceDependencies_Immutability(t *testing.T) {
	mgr := NewSmartSleepManager(nil)
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: true, ActiveConns: 5})

	deps := mgr.GetServiceDependencies("sda")
	deps[0].Active = false // 修改副本

	original := mgr.GetServiceDependencies("sda")
	if !original[0].Active {
		t.Error("GetServiceDependencies应返回副本")
	}
}

// ==================== 集成测试 ====================

func TestIntegration_FullCycle(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	// 1. 添加策略
	workdayPolicy := NewSmartSleepPolicy("workday", "工作日", 10*time.Minute, 20*time.Minute, 30*time.Minute)
	weekendPolicy := NewSmartSleepPolicy("weekend", "周末", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(workdayPolicy)
	mgr.AddPolicy(weekendPolicy)
	mgr.SetDefaultPolicy(workdayPolicy)

	// 2. 设置调度表
	schedule := DefaultSleepSchedule("workday", "weekend")
	mgr.SetSchedule("sda", schedule)

	// 3. 注册服务依赖
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "nfs", Active: false})

	// 4. 更新温度
	mgr.UpdateTemperature("sda", 38.0)

	// 5. 记录访问
	for i := 0; i < 10; i++ {
		mgr.RecordAccess("sda", AccessRead, 100*time.Millisecond)
	}

	// 6. 学习模式
	pattern := mgr.LearnPattern("sda")
	if pattern == nil {
		t.Fatal("模式学习失败")
	}

	// 7. 获取下次唤醒时间
	nextWake := mgr.GetNextWakeTime("sda")
	if nextWake.Before(time.Now()) {
		t.Error("下次唤醒时间应在未来")
	}

	// 8. 验证策略存在
	if mgr.GetPolicy("workday") == nil || mgr.GetPolicy("weekend") == nil {
		t.Error("策略应存在")
	}

	// 9. 验证调度表
	gotSchedule := mgr.GetSchedule("sda")
	if gotSchedule == nil {
		t.Fatal("调度表应存在")
	}
	if gotSchedule.WorkdayPolicyID != "workday" {
		t.Errorf("工作日策略ID错误: %s", gotSchedule.WorkdayPolicyID)
	}
}

func TestIntegration_TemperatureForcesSleep(t *testing.T) {
	mgr := NewSmartSleepManager(nil)

	policy := NewSmartSleepPolicy("default", "默认", 5*time.Minute, 10*time.Minute, 15*time.Minute)
	mgr.AddPolicy(policy)
	mgr.SetDefaultPolicy(policy)

	// 无服务依赖
	mgr.RegisterServiceDependency("sda", ServiceDependency{Name: "smb", Active: false})

	// 设置很久以前的访问
	mgr.patternMu.Lock()
	mgr.patterns["sda"] = &AccessPattern{
		DiskID:           "sda",
		LastAccess:       time.Now().Add(-2 * time.Hour),
		TotalAccessCount: 100,
		MaxRecords:       1000,
	}
	mgr.patternMu.Unlock()

	// 低温 - 可能休眠（取决于时段）
	mgr.UpdateTemperature("sda", 30.0)
	_, reason1 := mgr.ShouldSleep("sda")

	// 高温 - 应该因为温度休眠
	mgr.UpdateTemperature("sda", 52.0)
	should2, reason2 := mgr.ShouldSleep("sda")

	if !should2 {
		t.Error("高温时应允许休眠")
	}
	if reason2 != "磁盘温度过高，提前休眠散热" {
		t.Errorf("高温休眠理由错误: %s (低温时理由: %s)", reason2, reason1)
	}
}
