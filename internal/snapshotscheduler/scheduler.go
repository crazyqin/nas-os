package snapshotscheduler

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Scheduler 快照调度器
type Scheduler struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot   // id -> snapshot
	schedules map[string]*Schedule   // id -> schedule
	volumes   map[string]FileSystemType // volume -> fs type
	stopChan  chan struct{}
	running   bool
	ticker    *time.Ticker
}

// NewScheduler 创建快照调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		snapshots: make(map[string]*Snapshot),
		schedules: make(map[string]*Schedule),
		volumes:   make(map[string]FileSystemType),
		stopChan:  make(chan struct{}),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ticker = time.NewTicker(30 * time.Second)
	s.mu.Unlock()

	go s.run()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopChan)
}

// run 调度主循环
func (s *Scheduler) run() {
	for {
		select {
		case <-s.stopChan:
			return
		case <-s.ticker.C:
			s.checkAndExecuteSchedules()
		}
	}
}

// checkAndExecuteSchedules 检查并执行到期的调度
func (s *Scheduler) checkAndExecuteSchedules() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, sched := range s.schedules {
		if !sched.Enabled {
			continue
		}
		if sched.NextRunAt != nil && now.After(*sched.NextRunAt) {
			go s.executeSchedule(sched)
		}
	}
}

// executeSchedule 执行快照调度
func (s *Scheduler) executeSchedule(sched *Schedule) {
	_, err := s.CreateSnapshot(sched.VolumePath, sched.Name+"-"+time.Now().Format("20060102-150405"), sched.Tags)
	if err != nil {
		s.mu.Lock()
		sched.FailCount++
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	sched.RunCount++
	now := time.Now()
	sched.LastRunAt = &now
	sched.NextRunAt = s.calculateNextRun(sched)
	s.mu.Unlock()

	// 应用保留策略
	s.applyRetentionPolicy(sched)
}

// CreateSnapshot 创建快照
func (s *Scheduler) CreateSnapshot(volumePath, name string, tags []string) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if volumePath == "" {
		return nil, fmt.Errorf("volume path is required")
	}

	// 检测文件系统类型
	fsType, ok := s.volumes[volumePath]
	if !ok {
		fsType = FSSimulated
		s.volumes[volumePath] = fsType
	}

	snapshot := &Snapshot{
		ID:         uuid.New().String(),
		Name:       name,
		VolumePath: volumePath,
		FilePath:   fmt.Sprintf("%s/.snapshots/%s", volumePath, name),
		Size:       0,
		Status:     StatusActive,
		FSType:     fsType,
		Tags:       tags,
		CreatedAt:  time.Now(),
	}

	s.snapshots[snapshot.ID] = snapshot
	return snapshot, nil
}

// DeleteSnapshot 删除快照
func (s *Scheduler) DeleteSnapshot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	if snapshot.Status == StatusDeleting {
		return fmt.Errorf("snapshot is already being deleted")
	}

	snapshot.Status = StatusDeleting

	// 模拟删除过程
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
		delete(s.snapshots, id)
		s.mu.Unlock()
	}()

	return nil
}

// CloneSnapshot 克隆快照
func (s *Scheduler) CloneSnapshot(id, targetPath string) (*CloneResult, error) {
	s.mu.RLock()
	snapshot, ok := s.snapshots[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}

	if snapshot.Status != StatusActive {
		return nil, fmt.Errorf("snapshot is not active, status: %s", snapshot.Status)
	}

	if targetPath == "" {
		targetPath = fmt.Sprintf("%s/clone-%s", snapshot.VolumePath, time.Now().Format("20060102150405"))
	}

	clone := &Snapshot{
		ID:         uuid.New().String(),
		Name:       fmt.Sprintf("clone-%s", snapshot.Name),
		VolumePath: targetPath,
		FilePath:   targetPath,
		Size:       snapshot.Size,
		Status:     StatusActive,
		FSType:     snapshot.FSType,
		ParentID:   id,
		Tags:       snapshot.Tags,
		CreatedAt:  time.Now(),
	}

	s.mu.Lock()
	s.snapshots[clone.ID] = clone
	s.mu.Unlock()

	return &CloneResult{
		CloneID:    clone.ID,
		SourceID:   id,
		TargetPath: targetPath,
		Size:       clone.Size,
	}, nil
}

// Rollback 回滚到快照
func (s *Scheduler) Rollback(req *RollbackRequest) error {
	s.mu.RLock()
	snapshot, ok := s.snapshots[req.SnapshotID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("snapshot not found: %s", req.SnapshotID)
	}

	if snapshot.Status != StatusActive {
		return fmt.Errorf("snapshot is not active, status: %s", snapshot.Status)
	}

	// 实际实现中会执行文件系统回滚操作
	return nil
}

// GetSnapshot 获取快照信息
func (s *Scheduler) GetSnapshot(id string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}

	result := *snapshot
	return &result, nil
}

// ListSnapshots 列出快照
func (s *Scheduler) ListSnapshots(volumePath string, limit int) []*Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []*Snapshot
	for _, snap := range s.snapshots {
		if volumePath != "" && snap.VolumePath != volumePath {
			continue
		}
		snapshots = append(snapshots, snap)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	if limit > 0 && limit < len(snapshots) {
		snapshots = snapshots[:limit]
	}

	return snapshots
}

// CreateSchedule 创建调度计划
func (s *Scheduler) CreateSchedule(schedule *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if schedule.Name == "" {
		return fmt.Errorf("schedule name is required")
	}
	if schedule.VolumePath == "" {
		return fmt.Errorf("volume path is required")
	}

	schedule.ID = uuid.New().String()
	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()
	schedule.NextRunAt = s.calculateNextRun(schedule)

	s.schedules[schedule.ID] = schedule
	return nil
}

// UpdateSchedule 更新调度计划
func (s *Scheduler) UpdateSchedule(id string, update *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	if update.Name != "" {
		schedule.Name = update.Name
	}
	if update.Frequency != "" {
		schedule.Frequency = update.Frequency
	}
	if update.CronExpr != "" {
		schedule.CronExpr = update.CronExpr
	}
	if update.Hour >= 0 {
		schedule.Hour = update.Hour
	}
	if update.Minute >= 0 {
		schedule.Minute = update.Minute
	}
	schedule.Enabled = update.Enabled
	if update.Retention.Unit != "" {
		schedule.Retention = update.Retention
	}
	if update.Tags != nil {
		schedule.Tags = update.Tags
	}
	schedule.UpdatedAt = time.Now()
	schedule.NextRunAt = s.calculateNextRun(schedule)

	return nil
}

// DeleteSchedule 删除调度计划
func (s *Scheduler) DeleteSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	delete(s.schedules, id)
	return nil
}

// GetSchedule 获取调度计划
func (s *Scheduler) GetSchedule(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	result := *schedule
	return &result, nil
}

// ListSchedules 列出调度计划
func (s *Scheduler) ListSchedules(enabledOnly bool) []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var schedules []*Schedule
	for _, sched := range s.schedules {
		if enabledOnly && !sched.Enabled {
			continue
		}
		schedules = append(schedules, sched)
	}

	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].CreatedAt.After(schedules[j].CreatedAt)
	})

	return schedules
}

// GetStats 获取统计信息
func (s *Scheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SchedulerStats{
		TotalSnapshots: len(s.snapshots),
		TotalSchedules: len(s.schedules),
		ByStatus:       make(map[SnapshotStatus]int),
		ByVolume:       make(map[string]int),
	}

	for _, snap := range s.snapshots {
		stats.ByStatus[snap.Status]++
		stats.ByVolume[snap.VolumePath]++
		stats.TotalSizeBytes += snap.Size
		if stats.LastSnapshotAt == nil || snap.CreatedAt.After(*stats.LastSnapshotAt) {
			stats.LastSnapshotAt = &snap.CreatedAt
		}
	}

	for _, sched := range s.schedules {
		if sched.Enabled {
			stats.ActiveSchedules++
		}
		if sched.NextRunAt != nil {
			if stats.NextSnapshotAt == nil || sched.NextRunAt.Before(*stats.NextSnapshotAt) {
				stats.NextSnapshotAt = sched.NextRunAt
			}
		}
	}

	return stats
}

// calculateNextRun 计算下次运行时间
func (s *Scheduler) calculateNextRun(sched *Schedule) *time.Time {
	now := time.Now()
	var next time.Time

	switch sched.Frequency {
	case FreqMinutely:
		next = now.Add(time.Minute)
	case FreqHourly:
		next = time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, sched.Minute, 0, 0, now.Location())
	case FreqDaily:
		next = time.Date(now.Year(), now.Month(), now.Day()+1, sched.Hour, sched.Minute, 0, 0, now.Location())
	case FreqWeekly:
		daysUntilWeekday := (sched.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntilWeekday == 0 {
			daysUntilWeekday = 7
		}
		next = time.Date(now.Year(), now.Month(), now.Day()+daysUntilWeekday, sched.Hour, sched.Minute, 0, 0, now.Location())
	case FreqMonthly:
		next = time.Date(now.Year(), now.Month()+1, sched.DayOfMonth, sched.Hour, sched.Minute, 0, 0, now.Location())
	default:
		next = now.Add(time.Hour)
	}

	return &next
}

// applyRetentionPolicy 应用保留策略
func (s *Scheduler) applyRetentionPolicy(sched *Schedule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshots := make([]*Snapshot, 0)
	for _, snap := range s.snapshots {
		if snap.VolumePath == sched.VolumePath {
			snapshots = append(snapshots, snap)
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	policy := sched.Retention
	switch policy.Unit {
	case RetainByCount:
		if policy.MaxCount > 0 && len(snapshots) > policy.MaxCount {
			toDelete := snapshots[policy.MaxCount:]
			for _, snap := range toDelete {
				if policy.MinKeepCount > 0 && len(snapshots)-len(toDelete) <= policy.MinKeepCount {
					break
				}
				snap.Status = StatusDeleted
				delete(s.snapshots, snap.ID)
			}
		}
	case RetainByAge:
		if policy.MaxAgeDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -policy.MaxAgeDays)
			for _, snap := range snapshots {
				if snap.CreatedAt.Before(cutoff) {
					snap.Status = StatusDeleted
					delete(s.snapshots, snap.ID)
				}
			}
		}
	}
}

// RegisterVolume 注册卷
func (s *Scheduler) RegisterVolume(path string, fsType FileSystemType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volumes[path] = fsType
}

// IsRunning 检查调度器是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
