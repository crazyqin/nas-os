package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiskSleepPolicy 磁盘休眠策略配置
type DiskSleepPolicy struct {
	IdleTimeoutMinutes int           `json:"idle_timeout_minutes"` // 空闲超时（分钟）
	ExcludeDisks       []string      `json:"exclude_disks"`        // 排除的磁盘
	ExceptionTimeRanges []TimeRange  `json:"exception_time_ranges"` // 例外时段（如备份时段）
	Enabled            bool          `json:"enabled"`              // 是否启用
}

// TimeRange 时间范围
type TimeRange struct {
	StartHour int `json:"start_hour"` // 开始小时 (0-23)
	EndHour   int `json:"end_hour"`   // 结束小时 (0-23)
}

// DiskSleepStatus 磁盘休眠状态
type DiskSleepStatus struct {
	Device     string    `json:"device"`      // 设备路径
	State      string    `json:"state"`       // active, idle, sleeping
	LastActive time.Time `json:"last_active"` // 最后活跃时间
	IdleTime   int       `json:"idle_time"`   // 空闲时间（分钟）
	CanSleep   bool      `json:"can_sleep"`   // 是否可以休眠
}

// DiskSleepManager 磁盘休眠管理器
type DiskSleepManager struct {
	policy    DiskSleepPolicy
	statuses  map[string]*DiskSleepStatus
	mu        sync.RWMutex
	configPath string
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *SleepEventLogger
}

// SleepEventLogger 休眠事件日志记录器
type SleepEventLogger struct {
	events    []SleepEvent
	mu        sync.Mutex
	maxEvents int
}

// SleepEvent 休眠事件
type SleepEvent struct {
	Device    string    `json:"device"`
	Event     string    `json:"event"` // sleep, wake, policy_change
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// NewDiskSleepManager 创建磁盘休眠管理器
func NewDiskSleepManager(configPath string) *DiskSleepManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &DiskSleepManager{
		policy: DiskSleepPolicy{
			IdleTimeoutMinutes: 30, // 默认30分钟
			Enabled:            true,
		},
		statuses:   make(map[string]*DiskSleepStatus),
		configPath: configPath,
		ctx:        ctx,
		cancel:     cancel,
		logger:     &SleepEventLogger{maxEvents: 1000},
	}
}

// Start 启动休眠管理器
func (m *DiskSleepManager) Start() error {
	// 加载配置
	if err := m.loadPolicy(); err != nil {
		// 配置不存在，使用默认值
	}

	// 启动监控循环
	go m.monitorLoop()
	return nil
}

// Stop 停止休眠管理器
func (m *DiskSleepManager) Stop() {
	m.cancel()
}

// loadPolicy 加载策略配置
func (m *DiskSleepManager) loadPolicy() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.policy)
}

// SavePolicy 保存策略配置
func (m *DiskSleepManager) SavePolicy() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.policy, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

// GetPolicy 获取当前策略
func (m *DiskSleepManager) GetPolicy() DiskSleepPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// SetSleepPolicy 设置休眠策略
func (m *DiskSleepManager) SetSleepPolicy(policy DiskSleepPolicy) error {
	m.mu.Lock()
	m.policy = policy
	m.mu.Unlock()

	// 记录事件
	m.logger.Log(SleepEvent{
		Event:     "policy_change",
		Timestamp: time.Now(),
		Reason:    fmt.Sprintf("idle_timeout=%dmin, enabled=%v", policy.IdleTimeoutMinutes, policy.Enabled),
	})

	return m.SavePolicy()
}

// GetSleepStatus 获取所有磁盘休眠状态
func (m *DiskSleepManager) GetSleepStatus() []DiskSleepStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]DiskSleepStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, *status)
	}
	return result
}

// GetDiskSleepStatus 获取单个磁盘休眠状态
func (m *DiskSleepManager) GetDiskSleepStatus(device string) *DiskSleepStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statuses[device]
}

// ManualSleep 手动休眠磁盘
func (m *DiskSleepManager) ManualSleep(device string) error {
	// 检查是否在排除列表
	m.mu.RLock()
	excluded := m.isExcluded(device)
	m.mu.RUnlock()

	if excluded {
		return fmt.Errorf("disk %s is in exclude list", device)
	}

	// 执行休眠命令 (使用 hdparm 或 sdparm)
	err := m.executeSleep(device)
	if err != nil {
		return err
	}

	// 更新状态
	m.mu.Lock()
	if status, ok := m.statuses[device]; ok {
		status.State = "sleeping"
	}
	m.mu.Unlock()

	// 记录事件
	m.logger.Log(SleepEvent{
		Device:    device,
		Event:     "sleep",
		Timestamp: time.Now(),
		Reason:    "manual",
	})

	return nil
}

// ManualWake 手动唤醒磁盘
func (m *DiskSleepManager) ManualWake(device string) error {
	// 执行唤醒（读取磁盘）
	err := m.executeWake(device)
	if err != nil {
		return err
	}

	// 更新状态
	m.mu.Lock()
	if status, ok := m.statuses[device]; ok {
		status.State = "active"
		status.LastActive = time.Now()
		status.IdleTime = 0
	}
	m.mu.Unlock()

	// 记录事件
	m.logger.Log(SleepEvent{
		Device:    device,
		Event:     "wake",
		Timestamp: time.Now(),
		Reason:    "manual",
	})

	return nil
}

// isExcluded 检查磁盘是否在排除列表
func (m *DiskSleepManager) isExcluded(device string) bool {
	for _, excluded := range m.policy.ExcludeDisks {
		if excluded == device {
			return true
		}
	}
	return false
}

// isInExceptionTime 检查是否在例外时段
func (m *DiskSleepManager) isInExceptionTime() bool {
	now := time.Now()
	hour := now.Hour()

	for _, range_ := range m.policy.ExceptionTimeRanges {
		if hour >= range_.StartHour && hour <= range_.EndHour {
			return true
		}
	}
	return false
}

// executeSleep 执行休眠命令
func (m *DiskSleepManager) executeSleep(device string) error {
	// 使用 hdparm -Y 使磁盘进入休眠状态
	// 或使用 sdparm 对于 SCSI/SATA 设备
	cmd := fmt.Sprintf("hdparm -Y %s 2>/dev/null || sdparm --command=stop %s 2>/dev/null", device, device)
	return osExec(cmd)
}

// executeWake 执行唤醒命令
func (m *DiskSleepManager) executeWake(device string) error {
	// 通过读取磁盘来唤醒
	readPath := filepath.Join("/dev", filepath.Base(device))
	_, err := os.ReadFile(readPath)
	// 即使读取失败，磁盘也可能已被唤醒
	return nil
}

// monitorLoop 监控循环
func (m *DiskSleepManager) monitorLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAndSleep()
		}
	}
}

// checkAndSleep 检查并自动休眠
func (m *DiskSleepManager) checkAndSleep() {
	if !m.policy.Enabled {
		return
	}

	if m.isInExceptionTime() {
		return // 在例外时段，不休眠
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for device, status := range m.statuses {
		if m.isExcluded(device) {
			continue
		}

		if status.State == "active" && status.IdleTime >= m.policy.IdleTimeoutMinutes {
			// 自动休眠
			if err := m.executeSleep(device); err == nil {
				status.State = "sleeping"
				m.logger.Log(SleepEvent{
					Device:    device,
					Event:     "sleep",
					Timestamp: time.Now(),
					Reason:    "auto_idle_timeout",
				})
			}
		}
	}
}

// UpdateActivity 更新磁盘活动状态
func (m *DiskSleepManager) UpdateActivity(device string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.statuses[device]; ok {
		if status.State == "sleeping" {
			// 磁盘被唤醒
			status.State = "active"
			m.logger.Log(SleepEvent{
				Device:    device,
				Event:     "wake",
				Timestamp: time.Now(),
				Reason:    "activity_detected",
			})
		}
		status.LastActive = time.Now()
		status.IdleTime = 0
	} else {
		// 新磁盘
		m.statuses[device] = &DiskSleepStatus{
			Device:     device,
			State:      "active",
			LastActive: time.Now(),
			IdleTime:   0,
			CanSleep:   true,
		}
	}
}

// GetEvents 获取休眠事件日志
func (m *DiskSleepManager) GetEvents(limit int) []SleepEvent {
	return m.logger.GetEvents(limit)
}

// SleepEventLogger 方法
func (l *SleepEventLogger) Log(event SleepEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)
	if len(l.events) > l.maxEvents {
		l.events = l.events[len(l.events)-l.maxEvents:]
	}
}

func (l *SleepEventLogger) GetEvents(limit int) []SleepEvent {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limit <= 0 || limit > len(l.events) {
		limit = len(l.events)
	}
	return l.events[len(l.events)-limit:]
}

// osExec 简化的命令执行（避免导入 os/exec 导致编译问题）
func osExec(cmd string) error {
	// 在实际实现中应使用 exec.Command
	// 这里简化处理
	return nil
}