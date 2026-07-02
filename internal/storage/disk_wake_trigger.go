package storage

import (
	"context"
	"sync"
	"time"
)

// WakeRequest 唤醒请求.
type WakeRequest struct {
	ID        string
	DiskPath  string
	Priority  WakePriority
	Type      WakeRequestType
	Reason    string
	Timestamp time.Time
	index     int // heap index
}

// WakeRequestType 唤醒请求类型.
type WakeRequestType int

const (
	WakeRequestImmediate WakeRequestType = 0 // 立即唤醒
	WakeRequestBatched   WakeRequestType = 1 // 批量唤醒
	WakeRequestScheduled WakeRequestType = 2 // 定时唤醒
)

// WakeTrigger 磁盘唤醒触发器.
type WakeTrigger struct {
	sleepManager *DiskSleepManager
	monitors     map[string]*ActivityMonitor
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// ActivityMonitor 磁盘活动监控器.
type ActivityMonitor struct {
	Device     string
	LastRead   time.Time
	LastWrite  time.Time
	ReadCount  int64
	WriteCount int64
	IdleTimer  *IdleTimer
}

// IdleTimer 空闲计时器.
type IdleTimer struct {
	Device      string
	StartTime   time.Time
	IdleMinutes int
	Threshold   int // 空闲阈值（分钟）
	OnThreshold func(device string)
}

// NewWakeTrigger 创建唤醒触发器.
func NewWakeTrigger(sleepManager *DiskSleepManager) *WakeTrigger {
	ctx, cancel := context.WithCancel(context.Background())
	return &WakeTrigger{
		sleepManager: sleepManager,
		monitors:     make(map[string]*ActivityMonitor),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start 启动唤醒触发器.
func (t *WakeTrigger) Start() error {
	go t.monitorLoop()
	return nil
}

// Stop 停止唤醒触发器.
func (t *WakeTrigger) Stop() {
	t.cancel()
}

// OnSMBAccess SMB访问时唤醒磁盘.
func (t *WakeTrigger) OnSMBAccess(device string) {
	t.wakeIfNeeded(device, "smb_access")
}

// OnNFSAccess NFS访问时唤醒磁盘.
func (t *WakeTrigger) OnNFSAccess(device string) {
	t.wakeIfNeeded(device, "nfs_access")
}

// OnScheduledTask 定时任务唤醒.
func (t *WakeTrigger) OnScheduledTask(device string) {
	t.wakeIfNeeded(device, "scheduled_task")
}

// OnWebAccess Web界面访问唤醒.
func (t *WakeTrigger) OnWebAccess(device string) {
	t.wakeIfNeeded(device, "web_access")
}

// wakeIfNeeded 检查并唤醒磁盘.
func (t *WakeTrigger) wakeIfNeeded(device, reason string) {
	status := t.sleepManager.GetDiskSleepStatus(device)
	if status != nil && status.State == "sleeping" {
		// 磁盘处于休眠状态，需要唤醒
		if err := t.sleepManager.ManualWake(device); err == nil {
			t.sleepManager.logger.Log(SleepEvent{
				Device:    device,
				Event:     "wake",
				Timestamp: time.Now(),
				Reason:    reason,
			})
		}
	}

	// 更新活动状态
	t.sleepManager.UpdateActivity(device)
	t.UpdateMonitor(device, true, false)
}

// UpdateMonitor 更新活动监控器.
func (t *WakeTrigger) UpdateMonitor(device string, isRead, isWrite bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	monitor, ok := t.monitors[device]
	if !ok {
		monitor = &ActivityMonitor{
			Device:    device,
			IdleTimer: &IdleTimer{Device: device},
		}
		t.monitors[device] = monitor
	}

	now := time.Now()
	if isRead {
		monitor.LastRead = now
		monitor.ReadCount++
	}
	if isWrite {
		monitor.LastWrite = now
		monitor.WriteCount++
	}

	// 重置空闲计时器
	monitor.IdleTimer.StartTime = now
	monitor.IdleTimer.IdleMinutes = 0
}

// MonitorDiskActivity 监控磁盘活动（从系统统计读取）.
func (t *WakeTrigger) MonitorDiskActivity() map[string]*ActivityMonitor {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*ActivityMonitor)
	for device, monitor := range t.monitors {
		result[device] = monitor
	}
	return result
}

// monitorLoop 监控循环.
func (t *WakeTrigger) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.checkIdleStatus()
		}
	}
}

// checkIdleStatus 检查空闲状态.
func (t *WakeTrigger) checkIdleStatus() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for device, monitor := range t.monitors {
		// 计算空闲时间
		lastActivity := monitor.LastRead
		if monitor.LastWrite.After(lastActivity) {
			lastActivity = monitor.LastWrite
		}

		idleMinutes := int(now.Sub(lastActivity).Minutes())
		monitor.IdleTimer.IdleMinutes = idleMinutes

		// 更新休眠管理器的空闲时间
		t.sleepManager.mu.Lock()
		if status, ok := t.sleepManager.statuses[device]; ok {
			status.IdleTime = idleMinutes
			if idleMinutes > 0 {
				status.State = "idle"
			}
		}
		t.sleepManager.mu.Unlock()
	}
}

// GetIdleTime 获取磁盘空闲时间.
func (t *WakeTrigger) GetIdleTime(device string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if monitor, ok := t.monitors[device]; ok {
		return monitor.IdleTimer.IdleMinutes
	}
	return 0
}

// IsIdle 检查磁盘是否空闲.
func (t *WakeTrigger) IsIdle(device string, thresholdMinutes int) bool {
	return t.GetIdleTime(device) >= thresholdMinutes
}

// GetActivityStats 获取活动统计.
func (t *WakeTrigger) GetActivityStats(device string) (readCount, writeCount int64, lastActivity time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if monitor, ok := t.monitors[device]; ok {
		readCount = monitor.ReadCount
		writeCount = monitor.WriteCount
		lastActivity = monitor.LastRead
		if monitor.LastWrite.After(lastActivity) {
			lastActivity = monitor.LastWrite
		}
	}
	return
}
