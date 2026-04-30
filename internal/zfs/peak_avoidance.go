// Package zfs - Scrub智能避峰调度增强
// 对标 TrueNAS 26 Scheduled Scrub with peak avoidance
// 在业务高峰时段自动暂停或推迟Scrub，避免性能影响
package zfs

import (
	"log"
	"sync"
	"time"
)

// PeakWindow 业务高峰窗口
type PeakWindow struct {
	Name      string `json:"name"`      // 窗口名称，如"上午高峰"
	StartHour int    `json:"startHour"` // 开始小时 (0-23)
	EndHour   int    `json:"endHour"`   // 结束小时 (0-23)
	Weekdays  []int  `json:"weekdays"`  // 适用星期 (0=Sunday, 1=Monday...)
	Priority  int    `json:"priority"`  // 优先级，数字越小优先级越高
}

// PeakAvoidanceConfig 避峰配置
type PeakAvoidanceConfig struct {
	Enabled         bool          `json:"enabled"`         // 启用避峰
	PeakWindows     []PeakWindow  `json:"peakWindows"`     // 高峰窗口列表
	IOPSThreshold   int           `json:"iopsThreshold"`   // IO负载阈值
	CPUThreshold    float64       `json:"cpuThreshold"`    // CPU使用率阈值 (0-100)
	CheckInterval   time.Duration `json:"checkInterval"`   // 检查间隔
	AutoPause       bool          `json:"autoPause"`       // 高峰自动暂停
	AutoResume      bool          `json:"autoResume"`      // 高峰过后自动恢复
	MaxDelayHours   int           `json:"maxDelayHours"`   // 最大延迟小时数
	QuietHoursStart int           `json:"quietHoursStart"` // 安静时段开始 (默认凌晨1点)
	QuietHoursEnd   int           `json:"quietHoursEnd"`   // 安静时段结束 (默认早上6点)
}

// DefaultPeakAvoidanceConfig 默认避峰配置
func DefaultPeakAvoidanceConfig() PeakAvoidanceConfig {
	return PeakAvoidanceConfig{
		Enabled: true,
		PeakWindows: []PeakWindow{
			{Name: "上午工作高峰", StartHour: 9, EndHour: 12, Weekdays: []int{1, 2, 3, 4, 5}, Priority: 1},
			{Name: "下午工作高峰", StartHour: 13, EndHour: 18, Weekdays: []int{1, 2, 3, 4, 5}, Priority: 2},
			{Name: "晚间使用高峰", StartHour: 19, EndHour: 23, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, Priority: 3},
		},
		IOPSThreshold:   500,
		CPUThreshold:    80.0,
		CheckInterval:   5 * time.Minute,
		AutoPause:       true,
		AutoResume:      true,
		MaxDelayHours:   24,
		QuietHoursStart: 1,
		QuietHoursEnd:   6,
	}
}

// PeakAvoidanceManager 避峰管理器
type PeakAvoidanceManager struct {
	config      PeakAvoidanceConfig
	schedulers  map[string]*ScrubScheduler // poolName -> scheduler
	mu          sync.RWMutex
	pausedPools map[string]bool
	delayCount  map[string]int // 连续延迟次数
	lastCheck   time.Time
}

// NewPeakAvoidanceManager 创建避峰管理器
func NewPeakAvoidanceManager(config PeakAvoidanceConfig) *PeakAvoidanceManager {
	return &PeakAvoidanceManager{
		config:      config,
		schedulers:  make(map[string]*ScrubScheduler),
		pausedPools: make(map[string]bool),
		delayCount:  make(map[string]int),
	}
}

// RegisterScheduler 注册Scrub调度器
func (m *PeakAvoidanceManager) RegisterScheduler(poolName string, scheduler *ScrubScheduler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedulers[poolName] = scheduler
}

// UnregisterScheduler 注销调度器
func (m *PeakAvoidanceManager) UnregisterScheduler(poolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.schedulers, poolName)
	delete(m.pausedPools, poolName)
	delete(m.delayCount, poolName)
}

// ShouldAllowScrub 判断当前是否允许执行Scrub
func (m *PeakAvoidanceManager) ShouldAllowScrub() (bool, string) {
	if !m.config.Enabled {
		return true, "避峰未启用"
	}
	now := time.Now()

	// 检查是否在安静时段
	if m.isInQuietHours(now) {
		return true, "当前为安静时段，适合执行Scrub"
	}

	// 检查是否在高峰窗口
	for _, window := range m.config.PeakWindows {
		if m.isInPeakWindow(now, window) {
			return false, "当前在业务高峰时段: " + window.Name
		}
	}

	return true, "当前非高峰时段"
}

// CheckAndPause 检查并在需要时暂停Scrub
func (m *PeakAvoidanceManager) CheckAndPause() []string {
	if !m.config.Enabled || !m.config.AutoPause {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var paused []string
	allow, reason := m.shouldAllowScrubInternal()
	if allow {
		// 非高峰，尝试恢复
		if m.config.AutoResume {
			for poolName, wasPaused := range m.pausedPools {
				if wasPaused {
					if sched, ok := m.schedulers[poolName]; ok {
						if err := sched.ResumeScrub(); err == nil {
							m.pausedPools[poolName] = false
							log.Printf("[PeakAvoidance] Resumed scrub on pool %s", poolName)
						}
					}
				}
			}
		}
		return nil
	}
	// 高峰时段，暂停正在运行的Scrub
	for poolName, sched := range m.schedulers {
		if m.pausedPools[poolName] {
			continue
		}
		progress := sched.GetProgress()
		if progress.Status == ScrubStatusRunning {
			if err := sched.PauseScrub(); err == nil {
				m.pausedPools[poolName] = true
				paused = append(paused, poolName)
				log.Printf("[PeakAvoidance] Paused scrub on pool %s: %s", poolName, reason)
			}
		}
	}
	return paused
}

// GetStatus 获取避峰状态
func (m *PeakAvoidanceManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	allow, reason := m.shouldAllowScrubInternal()
	return map[string]interface{}{
		"enabled":      m.config.Enabled,
		"allowScrub":   allow,
		"reason":       reason,
		"pausedPools":  m.pausedPools,
		"delayCounts":  m.delayCount,
		"quietHours":   map[string]int{"start": m.config.QuietHoursStart, "end": m.config.QuietHoursEnd},
		"peakWindows":  m.config.PeakWindows,
		"lastCheck":    m.lastCheck,
	}
}

// isInQuietHours 判断是否在安静时段
func (m *PeakAvoidanceManager) isInQuietHours(now time.Time) bool {
	hour := now.Hour()
	start := m.config.QuietHoursStart
	end := m.config.QuietHoursEnd
	if start <= end {
		return hour >= start && hour < end
	}
	// 跨午夜，如 22:00 - 06:00
	return hour >= start || hour < end
}

// isInPeakWindow 判断是否在指定高峰窗口
func (m *PeakAvoidanceManager) isInPeakWindow(now time.Time, window PeakWindow) bool {
	hour := now.Hour()
	weekday := int(now.Weekday())

	// 检查是否在适用星期内
	dayMatch := false
	for _, d := range window.Weekdays {
		if d == weekday {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false
	}

	// 检查小时范围
	if window.StartHour <= window.EndHour {
		return hour >= window.StartHour && hour < window.EndHour
	}
	return hour >= window.StartHour || hour < window.EndHour
}

// shouldAllowScrubInternal 内部判断（不加锁）
func (m *PeakAvoidanceManager) shouldAllowScrubInternal() (bool, string) {
	if !m.config.Enabled {
		return true, "避峰未启用"
	}
	now := time.Now()
	m.lastCheck = now

	if m.isInQuietHours(now) {
		return true, "当前为安静时段"
	}

	for _, window := range m.config.PeakWindows {
		if m.isInPeakWindow(now, window) {
			return false, "当前在业务高峰时段: " + window.Name
		}
	}

	return true, "当前非高峰时段"
}
