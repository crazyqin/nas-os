package scrubsmart

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockBackend 模拟 Scrub 后端.
type mockBackend struct {
	mu           sync.Mutex
	startCalled  int
	stopCalled   int
	startErr     error
	stopErr      error
	progress     *ScrubProgress
}

func (m *mockBackend) Start(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled++
	return m.startErr
}

func (m *mockBackend) Stop(_ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled++
	return m.stopErr
}

func (m *mockBackend) GetProgress(_ string) (*ScrubProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.progress != nil {
		return m.progress, nil
	}
	return &ScrubProgress{
		Percentage:   0,
		BytesScanned: 0,
		BytesTotal:   1024 * 1024 * 1024, // 1GB
		Errors:       0,
	}, nil
}

// ========== AvoidanceWindow 测试 ==========

func TestAvoidanceWindowContains_WeekdayInRange(t *testing.T) {
	w := AvoidanceWindow{
		Name:        "工作日白天",
		Weekdays:    []Weekday{WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday, WeekdayFriday},
		StartHour:   9,
		StartMinute: 0,
		EndHour:     18,
		EndMinute:   0,
	}

	// 周三 10:30 — 应该在窗口内
	wed1030 := time.Date(2026, 5, 6, 10, 30, 0, 0, time.Local) // 2026-05-06 is Wednesday
	if !w.Contains(wed1030) {
		t.Errorf("周三 10:30 应该在避峰窗口内")
	}

	// 周三 08:00 — 不在窗口内
	wed0800 := time.Date(2026, 5, 6, 8, 0, 0, 0, time.Local)
	if w.Contains(wed0800) {
		t.Errorf("周三 08:00 不应在避峰窗口内")
	}
}

func TestAvoidanceWindowContains_Weekend(t *testing.T) {
	w := AvoidanceWindow{
		Name:        "工作日白天",
		Weekdays:    []Weekday{WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday, WeekdayFriday},
		StartHour:   9,
		StartMinute: 0,
		EndHour:     18,
		EndMinute:   0,
	}

	// 周六 10:30 — 不在窗口内（周末）
	sat1030 := time.Date(2026, 5, 2, 10, 30, 0, 0, time.Local) // 2026-05-02 is Saturday
	if w.Contains(sat1030) {
		t.Errorf("周六 10:30 不应在避峰窗口内")
	}
}

func TestAvoidanceWindowContains_MidnightSpan(t *testing.T) {
	w := AvoidanceWindow{
		Name:        "深夜维护",
		Weekdays:    []Weekday{WeekdaySunday},
		StartHour:   22,
		StartMinute: 0,
		EndHour:     6,
		EndMinute:   0,
	}

	// 周日 23:00 — 在跨午夜窗口内
	sun2300 := time.Date(2026, 5, 3, 23, 0, 0, 0, time.Local)
	if !w.Contains(sun2300) {
		t.Errorf("周日 23:00 应该在跨午夜窗口内")
	}

	// 周日 03:00 — 在跨午夜窗口内
	sun0300 := time.Date(2026, 5, 3, 3, 0, 0, 0, time.Local)
	if !w.Contains(sun0300) {
		t.Errorf("周日 03:00 应该在跨午夜窗口内")
	}

	// 周日 12:00 — 不在窗口内
	sun1200 := time.Date(2026, 5, 3, 12, 0, 0, 0, time.Local)
	if w.Contains(sun1200) {
		t.Errorf("周日 12:00 不应在跨午夜窗口内")
	}
}

func TestAvoidanceWindowValidate(t *testing.T) {
	valid := AvoidanceWindow{
		Weekdays:  []Weekday{WeekdayMonday},
		StartHour: 9, StartMinute: 0,
		EndHour: 18, EndMinute: 0,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("有效配置不应报错: %v", err)
	}

	invalid := AvoidanceWindow{
		Weekdays:  []Weekday{WeekdayMonday},
		StartHour: 25, StartMinute: 0,
		EndHour: 18, EndMinute: 0,
	}
	if err := invalid.Validate(); err == nil {
		t.Errorf("无效配置应该报错")
	}

	noDays := AvoidanceWindow{
		Weekdays:  []Weekday{},
		StartHour: 9, StartMinute: 0,
		EndHour: 18, EndMinute: 0,
	}
	if err := noDays.Validate(); err == nil {
		t.Errorf("无星期配置应该报错")
	}
}

// ========== Scheduler 测试 ==========

func newTestScheduler(backend *mockBackend) *Scheduler {
	cfg := DefaultConfig()
	cfg.TargetPool = "tank"
	cfg.IOCheckIntervalSeconds = 1
	cfg.IOWriteThresholdMBps = 50.0
	cfg.IOReadThresholdMBps = 100.0
	s := NewBackendScheduler(backend, nil)
	s.SetConfig(cfg)
	return s
}

// NewBackendScheduler 创建使用指定后端的调度器（测试用）.
func NewBackendScheduler(backend ScrubBackend, logger *interface{}) *Scheduler {
	s := NewScheduler(backend, nil)
	return s
}

func TestSchedulerInitialState(t *testing.T) {
	backend := &mockBackend{}
	s := newTestScheduler(backend)

	status := s.GetStatus()
	if status.State != StateIdle {
		t.Errorf("初始状态应为 idle，实际为 %s", status.State)
	}
	if status.Pool != "tank" {
		t.Errorf("存储池应为 tank，实际为 %s", status.Pool)
	}
}

func TestSchedulerPauseNotRunning(t *testing.T) {
	backend := &mockBackend{}
	s := newTestScheduler(backend)

	err := s.Pause("测试")
	if err != ErrScrubNotRunning {
		t.Errorf("未运行时暂停应返回 ErrScrubNotRunning，实际为 %v", err)
	}
}

func TestSchedulerSetConfig(t *testing.T) {
	backend := &mockBackend{}
	s := newTestScheduler(backend)

	cfg := DefaultConfig()
	cfg.TargetPool = "data"
	cfg.AvoidanceWindows = []AvoidanceWindow{
		{
			Name:        "测试窗口",
			Weekdays:    []Weekday{WeekdaySunday},
			StartHour:   1,
			StartMinute: 0,
			EndHour:     5,
			EndMinute:   0,
		},
	}
	s.SetConfig(cfg)

	got := s.GetConfig()
	if got.TargetPool != "data" {
		t.Errorf("目标池应为 data，实际为 %s", got.TargetPool)
	}
	if len(got.AvoidanceWindows) != 1 {
		t.Errorf("避峰窗口数应为 1，实际为 %d", len(got.AvoidanceWindows))
	}
}

func TestSchedulerCalcNextResume(t *testing.T) {
	backend := &mockBackend{}
	s := newTestScheduler(backend)

	// 设置状态为 paused，计算恢复时间
	s.mu.Lock()
	s.state = StatePaused
	now := time.Date(2026, 5, 1, 14, 0, 0, 0, time.Local) // 周五 14:00
	nextResume := s.calcNextResumeLocked(now)
	s.mu.Unlock()

	// 当前在窗口内（周五 14:00，窗口 9-18），恢复时间应为 18:00
	expected := time.Date(2026, 5, 1, 18, 0, 0, 0, time.Local)
	if !nextResume.Equal(expected) {
		t.Errorf("预计恢复时间应为 %v，实际为 %v", expected, nextResume)
	}
}

func TestSchedulerIOThreshold(t *testing.T) {
	backend := &mockBackend{
		progress: &ScrubProgress{
			Percentage: 50.0,
			BytesTotal: 1024 * 1024 * 1024,
		},
	}
	s := newTestScheduler(backend)

	// 手动设置状态为 Running
	s.mu.Lock()
	s.state = StateRunning
	// 设置 IO 负载超过阈值
	s.ioLoad = &IOLoad{
		ReadMBps:  200.0, // 超过 100 阈值
		WriteMBps: 10.0,
		Timestamp: time.Now(),
	}
	s.mu.Unlock()

	// 调用 tick 触发 IO 检查（tick 内部自行加锁）
	s.tick(context.Background())

	status := s.GetStatus()
	if status.State != StatePaused {
		t.Errorf("IO 超限时状态应为 paused，实际为 %s", status.State)
	}

	if backend.stopCalled != 1 {
		t.Errorf("应该调用 Stop 1 次，实际调用 %d 次", backend.stopCalled)
	}
}

// ========== Handler 测试 ==========

func TestHandlerStatus(t *testing.T) {
	backend := &mockBackend{}
	s := newTestScheduler(backend)
	handler := NewHandler(s, nil)

	status := handler.scheduler.GetStatus()
	if status.State != StateIdle {
		t.Errorf("Handler 返回状态应为 idle，实际为 %s", status.State)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("默认配置应启用")
	}
	if len(cfg.AvoidanceWindows) != 1 {
		t.Errorf("默认应有 1 个避峰窗口，实际 %d", len(cfg.AvoidanceWindows))
	}
	if cfg.AvoidanceWindows[0].StartHour != 9 {
		t.Errorf("默认窗口开始小时应为 9，实际 %d", cfg.AvoidanceWindows[0].StartHour)
	}
	if cfg.AvoidanceWindows[0].EndHour != 18 {
		t.Errorf("默认窗口结束小时应为 18，实际 %d", cfg.AvoidanceWindows[0].EndHour)
	}
}
