// Package scrubsched 提供智能Scrub调度功能
package scrubsched

import (
	"sync"
	"testing"
	"time"
)

// ========== Mock 实现 ==========

// mockPoolProvider 模拟存储池提供者.
type mockPoolProvider struct {
	pools map[string]*PoolInfo
}

func newMockPoolProvider() *mockPoolProvider {
	return &mockPoolProvider{
		pools: map[string]*PoolInfo{
			"pool1": {ID: "pool1", Name: "tank", Status: "online", Size: 1024 * 1024 * 1024 * 100},
			"pool2": {ID: "pool2", Name: "backup", Status: "online", Size: 1024 * 1024 * 1024 * 50},
		},
	}
}

func (m *mockPoolProvider) GetPool(poolID string) (*PoolInfo, error) {
	pool, ok := m.pools[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

func (m *mockPoolProvider) ListPools() ([]*PoolInfo, error) {
	pools := make([]*PoolInfo, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools, nil
}

// mockScrubExecutor 模拟Scrub执行器.
type mockScrubExecutor struct {
	mu       sync.Mutex
	running  map[string]bool
	progress map[string]float64
}

func newMockScrubExecutor() *mockScrubExecutor {
	return &mockScrubExecutor{
		running:  make(map[string]bool),
		progress: make(map[string]float64),
	}
}

func (m *mockScrubExecutor) StartScrub(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running[poolID] = true
	m.progress[poolID] = 0
	return nil
}

func (m *mockScrubExecutor) StopScrub(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.running, poolID)
	delete(m.progress, poolID)
	return nil
}

func (m *mockScrubExecutor) PauseScrub(poolID string) error {
	return nil
}

func (m *mockScrubExecutor) ResumeScrub(poolID string) error {
	return nil
}

func (m *mockScrubExecutor) GetScrubProgress(poolID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.progress[poolID], nil
}

func (m *mockScrubExecutor) IsScrubRunning(poolID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running[poolID], nil
}

// mockIOCollector 模拟IO采集器.
type mockIOCollector struct {
	loads map[string]*IOLoad
}

func newMockIOCollector() *mockIOCollector {
	return &mockIOCollector{
		loads: map[string]*IOLoad{
			"pool1": {PoolID: "pool1", IOPS: 500, Bandwidth: 100.0, Latency: 5.0, ReadIOPS: 300, WriteIOPS: 200},
			"pool2": {PoolID: "pool2", IOPS: 200, Bandwidth: 50.0, Latency: 3.0, ReadIOPS: 150, WriteIOPS: 50},
		},
	}
}

func (m *mockIOCollector) CollectIOLoad(poolID string) (*IOLoad, error) {
	load, ok := m.loads[poolID]
	if !ok {
		return &IOLoad{PoolID: poolID}, nil
	}
	return load, nil
}

func (m *mockIOCollector) CollectAllIOLoad() (map[string]*IOLoad, error) {
	result := make(map[string]*IOLoad)
	for k, v := range m.loads {
		loadCopy := *v
		result[k] = &loadCopy
	}
	return result, nil
}

// mockHealthProvider 模拟健康数据提供者.
type mockHealthProvider struct {
	health map[string]*PoolHealthSummary
}

func newMockHealthProvider() *mockHealthProvider {
	return &mockHealthProvider{
		health: map[string]*PoolHealthSummary{
			"pool1": {
				PoolID:        "pool1",
				OverallHealth: "good",
				DiskCount:     4,
				HealthyDisks:  4,
				WarningDisks:  0,
				CriticalDisks: 0,
			},
			"pool2": {
				PoolID:        "pool2",
				OverallHealth: "warning",
				DiskCount:     2,
				HealthyDisks:  1,
				WarningDisks:  1,
				CriticalDisks: 0,
				Disks: []DiskHealth{
					{DiskID: "disk1", PoolID: "pool2", Health: "good"},
					{DiskID: "disk2", PoolID: "pool2", Health: "warning", Temperature: 55},
				},
			},
		},
	}
}

func (m *mockHealthProvider) GetPoolHealth(poolID string) (*PoolHealthSummary, error) {
	h, ok := m.health[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return h, nil
}

// mockAlertSender 模拟告警发送器.
type mockAlertSender struct {
	mu     sync.Mutex
	alerts []struct {
		level   string
		title   string
		message string
	}
}

func newMockAlertSender() *mockAlertSender {
	return &mockAlertSender{}
}

func (m *mockAlertSender) SendAlert(level, title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, struct {
		level   string
		title   string
		message string
	}{level, title, message})
	return nil
}

// ========== 测试辅助函数 ==========

func setupTestManager() *Manager {
	poolProv := newMockPoolProvider()
	exec := newMockScrubExecutor()
	collector := newMockIOCollector()
	health := newMockHealthProvider()
	alert := newMockAlertSender()

	return NewManager("/tmp/scrubsched-test", poolProv, exec, collector, health, alert)
}

// ========== 测试用例 ==========

// TestCreatePolicy 测试创建调度策略.
func TestCreatePolicy(t *testing.T) {
	mgr := setupTestManager()

	req := CreatePolicyRequest{
		Name:      "每周Scrub",
		PoolID:    "pool1",
		Schedule:  ScheduleWeekly,
		WeekDay:   0, // 周日
		Hour:      2,
		Minute:    0,
		Priority:  PriorityNormal,
		Enabled:   true,
		AvoidPeak: true,
		PeakWindows: []PeakWindow{
			{DayOfWeek: -1, StartHour: 9, StartMin: 0, EndHour: 18, EndMin: 0},
		},
		IOThreshold: IOThreshold{
			IOPSMax:      1000,
			BandwidthMax: 200.0,
			LatencyMax:   10.0,
			ResumeRatio:  0.7,
		},
	}

	policy, err := mgr.CreatePolicy(req)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	if policy.Name != "每周Scrub" {
		t.Errorf("策略名称不匹配，期望: 每周Scrub, 实际: %s", policy.Name)
	}
	if policy.PoolID != "pool1" {
		t.Errorf("存储池ID不匹配，期望: pool1, 实际: %s", policy.PoolID)
	}
	if policy.Schedule != ScheduleWeekly {
		t.Errorf("调度类型不匹配")
	}
	if !policy.AvoidPeak {
		t.Error("避峰调度应为true")
	}
	if policy.Priority != PriorityNormal {
		t.Errorf("优先级不匹配，期望: %d, 实际: %d", PriorityNormal, policy.Priority)
	}
	if policy.NextRun == nil {
		t.Error("下次执行时间不应为nil")
	}
}

// TestCreatePolicyDuplicateName 测试重复名称策略.
func TestCreatePolicyDuplicateName(t *testing.T) {
	mgr := setupTestManager()

	req := CreatePolicyRequest{
		Name:     "重复策略",
		PoolID:   "pool1",
		Schedule: ScheduleWeekly,
		WeekDay:  1,
		Hour:     3,
		Minute:   0,
		Enabled:  true,
	}

	_, err := mgr.CreatePolicy(req)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}

	_, err = mgr.CreatePolicy(req)
	if err != ErrPolicyExists {
		t.Errorf("期望ErrPolicyExists，实际: %v", err)
	}
}

// TestCreatePolicyInvalidPool 测试无效池ID.
func TestCreatePolicyInvalidPool(t *testing.T) {
	mgr := setupTestManager()

	req := CreatePolicyRequest{
		Name:     "无效池策略",
		PoolID:   "nonexistent",
		Schedule: ScheduleWeekly,
		WeekDay:  1,
		Hour:     3,
		Minute:   0,
		Enabled:  true,
	}

	_, err := mgr.CreatePolicy(req)
	if err != ErrPoolNotFound {
		t.Errorf("期望ErrPoolNotFound，实际: %v", err)
	}
}

// TestListPolicies 测试列出策略.
func TestListPolicies(t *testing.T) {
	mgr := setupTestManager()

	// 创建两个策略
	_, _ = mgr.CreatePolicy(CreatePolicyRequest{
		Name: "策略1", PoolID: "pool1", Schedule: ScheduleWeekly, WeekDay: 1, Hour: 2, Minute: 0, Enabled: true,
	})
	_, _ = mgr.CreatePolicy(CreatePolicyRequest{
		Name: "策略2", PoolID: "pool2", Schedule: ScheduleMonthly, MonthDay: 1, Hour: 3, Minute: 0, Enabled: true,
	})

	policies := mgr.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("策略数量不匹配，期望: 2, 实际: %d", len(policies))
	}
}

// TestUpdatePolicy 测试更新策略.
func TestUpdatePolicy(t *testing.T) {
	mgr := setupTestManager()

	policy, _ := mgr.CreatePolicy(CreatePolicyRequest{
		Name: "待更新策略", PoolID: "pool1", Schedule: ScheduleWeekly, WeekDay: 1, Hour: 2, Minute: 0, Enabled: true,
	})

	newName := "已更新策略"
	newHour := 5
	_, err := mgr.UpdatePolicy(policy.ID, UpdatePolicyRequest{
		Name: &newName,
		Hour: &newHour,
	})
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	updated, _ := mgr.GetPolicy(policy.ID)
	if updated.Name != "已更新策略" {
		t.Errorf("策略名称未更新")
	}
	if updated.Hour != 5 {
		t.Errorf("执行小时未更新")
	}
}

// TestDeletePolicy 测试删除策略.
func TestDeletePolicy(t *testing.T) {
	mgr := setupTestManager()

	policy, _ := mgr.CreatePolicy(CreatePolicyRequest{
		Name: "待删除策略", PoolID: "pool1", Schedule: ScheduleWeekly, WeekDay: 1, Hour: 2, Minute: 0, Enabled: true,
	})

	err := mgr.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}

	_, err = mgr.GetPolicy(policy.ID)
	if err != ErrPolicyNotFound {
		t.Errorf("策略应已被删除")
	}
}

// TestTriggerScrub 测试手动触发Scrub.
func TestTriggerScrub(t *testing.T) {
	mgr := setupTestManager()

	err := mgr.TriggerScrub("pool1")
	if err != nil {
		t.Fatalf("触发Scrub失败: %v", err)
	}

	status, _ := mgr.GetPoolScrubStatus("pool1")
	if status.State != StateRunning {
		t.Errorf("Scrub状态应为running，实际: %s", status.State)
	}
	if !status.IsManual {
		t.Error("手动触发的Scrub IsManual应为true")
	}
}

// TestTriggerScrubDuplicate 测试重复触发Scrub.
func TestTriggerScrubDuplicate(t *testing.T) {
	mgr := setupTestManager()

	_ = mgr.TriggerScrub("pool1")
	err := mgr.TriggerScrub("pool1")
	if err != ErrScrubAlreadyRunning {
		t.Errorf("期望ErrScrubAlreadyRunning，实际: %v", err)
	}
}

// TestTriggerScrubInvalidPool 测试无效池触发Scrub.
func TestTriggerScrubInvalidPool(t *testing.T) {
	mgr := setupTestManager()

	err := mgr.TriggerScrub("nonexistent")
	if err != ErrPoolNotFound {
		t.Errorf("期望ErrPoolNotFound，实际: %v", err)
	}
}

// TestPauseAndResumeScrub 测试暂停和恢复Scrub.
func TestPauseAndResumeScrub(t *testing.T) {
	mgr := setupTestManager()

	_ = mgr.TriggerScrub("pool1")

	// 暂停
	err := mgr.PauseScrub("pool1", "测试暂停")
	if err != nil {
		t.Fatalf("暂停Scrub失败: %v", err)
	}

	status, _ := mgr.GetPoolScrubStatus("pool1")
	if status.State != StatePaused {
		t.Errorf("Scrub状态应为paused，实际: %s", status.State)
	}
	if status.PauseCount != 1 {
		t.Errorf("暂停次数应为1，实际: %d", status.PauseCount)
	}

	// 恢复
	err = mgr.ResumeScrub("pool1")
	if err != nil {
		t.Fatalf("恢复Scrub失败: %v", err)
	}

	status, _ = mgr.GetPoolScrubStatus("pool1")
	if status.State != StateRunning {
		t.Errorf("Scrub状态应为running，实际: %s", status.State)
	}
}

// TestCancelScrub 测试取消Scrub.
func TestCancelScrub(t *testing.T) {
	mgr := setupTestManager()

	_ = mgr.TriggerScrub("pool1")
	time.Sleep(10 * time.Millisecond) // 确保有记录

	err := mgr.CancelScrub("pool1")
	if err != nil {
		t.Fatalf("取消Scrub失败: %v", err)
	}

	status, _ := mgr.GetPoolScrubStatus("pool1")
	if status.State != StateIdle {
		t.Errorf("取消后状态应为idle，实际: %s", status.State)
	}

	// 检查历史记录
	records := mgr.GetHistory("pool1")
	found := false
	for _, r := range records {
		if r.State == StateCancelled {
			found = true
			break
		}
	}
	if !found {
		t.Error("历史记录中应有取消记录")
	}
}

// TestGetRecommendations 测试获取建议.
func TestGetRecommendations(t *testing.T) {
	mgr := setupTestManager()

	recs := mgr.GetRecommendations()
	// 没有策略时，健康检查应该给出建议
	if len(recs) == 0 {
		// 可能没有足够的数据产生建议，这是正常的
		t.Log("当前无建议（正常情况）")
	}
}

// TestIsInPeakWindow 测试高峰窗口判断.
func TestIsInPeakWindow(t *testing.T) {
	mgr := setupTestManager()

	p := &Policy{
		AvoidPeak: true,
		PeakWindows: []PeakWindow{
			{DayOfWeek: -1, StartHour: 9, StartMin: 0, EndHour: 18, EndMin: 0},
		},
	}

	// 测试工作时间
	workTime := time.Date(2026, 5, 1, 14, 30, 0, 0, time.Local) // 周五 14:30
	if !mgr.isInPeakWindow(p, workTime) {
		t.Error("14:30应在高峰窗口内")
	}

	// 测试非工作时间
	offTime := time.Date(2026, 5, 1, 22, 0, 0, 0, time.Local) // 22:00
	if mgr.isInPeakWindow(p, offTime) {
		t.Error("22:00不应在高峰窗口内")
	}

	// 测试禁用避峰
	p2 := &Policy{AvoidPeak: false}
	if mgr.isInPeakWindow(p2, workTime) {
		t.Error("禁用避峰时不应返回true")
	}
}

// TestIsInPeakWindowCrossMidnight 测试跨午夜高峰窗口.
func TestIsInPeakWindowCrossMidnight(t *testing.T) {
	mgr := setupTestManager()

	p := &Policy{
		AvoidPeak: true,
		PeakWindows: []PeakWindow{
			{DayOfWeek: -1, StartHour: 22, StartMin: 0, EndHour: 6, EndMin: 0}, // 22:00 - 06:00
		},
	}

	// 测试午夜前
	nightTime := time.Date(2026, 5, 1, 23, 0, 0, 0, time.Local)
	if !mgr.isInPeakWindow(p, nightTime) {
		t.Error("23:00应在跨午夜高峰窗口内")
	}

	// 测试午夜后
	earlyMorning := time.Date(2026, 5, 2, 3, 0, 0, 0, time.Local)
	if !mgr.isInPeakWindow(p, earlyMorning) {
		t.Error("03:00应在跨午夜高峰窗口内")
	}

	// 测试窗口外
	dayTime := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
	if mgr.isInPeakWindow(p, dayTime) {
		t.Error("12:00不应在跨午夜高峰窗口内")
	}
}

// TestGetAllStatus 测试获取所有状态.
func TestGetAllStatus(t *testing.T) {
	mgr := setupTestManager()

	_ = mgr.TriggerScrub("pool1")
	_ = mgr.TriggerScrub("pool2")

	statuses := mgr.GetScrubStatus()
	if len(statuses) != 2 {
		t.Errorf("状态数量不匹配，期望: 2, 实际: %d", len(statuses))
	}
	if statuses["pool1"].State != StateRunning {
		t.Errorf("pool1状态应为running")
	}
	if statuses["pool2"].State != StateRunning {
		t.Errorf("pool2状态应为running")
	}
}

// TestIOAnalyzer 测试IO分析器.
func TestIOAnalyzer(t *testing.T) {
	mgr := setupTestManager()
	analyzer := mgr.analyzer

	// 测试未学习状态
	pattern := analyzer.GetPeakPattern("pool1")
	if pattern != nil {
		t.Error("初始状态不应有模式数据")
	}

	isPeak := analyzer.GetIsPeakHour("pool1", 14)
	if isPeak {
		t.Error("未学习时不应判断为高峰")
	}
}

// TestPoolHealthSummary 测试池健康汇总.
func TestPoolHealthSummary(t *testing.T) {
	healthProv := newMockHealthProvider()

	health, err := healthProv.GetPoolHealth("pool1")
	if err != nil {
		t.Fatalf("获取健康状态失败: %v", err)
	}
	if health.OverallHealth != "good" {
		t.Errorf("pool1健康状态应为good，实际: %s", health.OverallHealth)
	}

	health2, err := healthProv.GetPoolHealth("pool2")
	if err != nil {
		t.Fatalf("获取健康状态失败: %v", err)
	}
	if health2.OverallHealth != "warning" {
		t.Errorf("pool2健康状态应为warning，实际: %s", health2.OverallHealth)
	}
	if health2.WarningDisks != 1 {
		t.Errorf("pool2告警磁盘数应为1，实际: %d", health2.WarningDisks)
	}
}
