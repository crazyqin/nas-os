package powerscheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager 智能功耗调度管理器
type Manager struct {
	mu        sync.RWMutex
	schedules map[string]*Schedule
	state     *PowerState
	throttle  *ThrottleConfig
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
}

// NewManager 创建功耗调度管理器
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		schedules: make(map[string]*Schedule),
		state:     &PowerState{},
		throttle: &ThrottleConfig{
			MaxCPUPercent: 100,
			MaxMemoryMB:   0,
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	go m.scheduleLoop()
	m.logger.Info("智能功耗调度管理器已启动")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	m.running = false
	m.cancel()
	m.logger.Info("智能功耗调度管理器已停止")
	return nil
}

// CreateSchedule 创建调度计划
func (m *Manager) CreateSchedule(s Schedule) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.Name == "" || s.Time == "" {
		return nil, ErrInvalidSchedule
	}
	if _, exists := m.schedules[s.ID]; exists {
		return nil, ErrScheduleExists
	}
	if s.ID == "" {
		s.ID = fmt.Sprintf("sched_%d", time.Now().UnixNano())
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	if !s.Enabled {
		s.Enabled = true
	}
	m.schedules[s.ID] = &s
	m.logger.Info("调度计划已创建", "id", s.ID, "name", s.Name)
	return &s, nil
}

// UpdateSchedule 更新调度计划
func (m *Manager) UpdateSchedule(id string, s Schedule) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.schedules[id]
	if !exists {
		return nil, ErrScheduleNotFound
	}
	existing.Name = s.Name
	existing.Action = s.Action
	existing.Time = s.Time
	existing.Days = s.Days
	existing.Enabled = s.Enabled
	existing.UpdatedAt = time.Now()
	return existing, nil
}

// DeleteSchedule 删除调度计划
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.schedules[id]; !exists {
		return ErrScheduleNotFound
	}
	delete(m.schedules, id)
	return nil
}

// GetSchedule 获取调度计划
func (m *Manager) GetSchedule(id string) (*Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, exists := m.schedules[id]
	if !exists {
		return nil, ErrScheduleNotFound
	}
	return s, nil
}

// ListSchedules 列出所有调度计划
func (m *Manager) ListSchedules() []*Schedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	schedules := make([]*Schedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetPowerState 获取当前功耗状态
func (m *Manager) GetPowerState() *PowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SetThrottle 设置功耗节流
func (m *Manager) SetThrottle(config ThrottleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.throttle = &config
	m.logger.Info("功耗节流已更新", "max_cpu", config.MaxCPUPercent)
}

// ApplyProfile 应用功耗配置
func (m *Manager) ApplyProfile(profile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch profile {
	case "performance":
		m.throttle = &ThrottleConfig{MaxCPUPercent: 100}
	case "balanced":
		m.throttle = &ThrottleConfig{MaxCPUPercent: 70, SpinDownDisks: true}
	case "powersave":
		m.throttle = &ThrottleConfig{MaxCPUPercent: 40, SpinDownDisks: true, ReduceNetworkQoS: true}
	}
	m.logger.Info("功耗配置已应用", "profile", profile)
}

func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkSchedules()
		}
	}
}

func (m *Manager) checkSchedules() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	hour, min := now.Hour(), now.Minute()
	weekday := DayOfWeek(now.Weekday())
	for _, s := range m.schedules {
		if !s.Enabled {
			continue
		}
		var sh, sm int
		fmt.Sscanf(s.Time, "%d:%d", &sh, &sm)
		if sh == hour && sm == min {
			dayMatch := len(s.Days) == 0
			for _, d := range s.Days {
				if d == weekday {
					dayMatch = true
					break
				}
			}
			if dayMatch {
				m.executeAction(s)
			}
		}
	}
}

func (m *Manager) executeAction(s *Schedule) {
	now := time.Now()
	s.LastRun = &now
	m.logger.Info("执行功耗调度", "action", s.Action, "name", s.Name)
}
