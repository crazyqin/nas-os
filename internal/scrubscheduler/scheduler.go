package scrubscheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ZFSExecutor 定义 ZFS scrub 执行的接口，便于测试时注入 mock
type ZFSExecutor interface {
	// StartScrub 启动指定池的 scrub
	StartScrub(ctx context.Context, pool string) error
	// StopScrub 停止指定池的 scrub
	StopScrub(ctx context.Context, pool string) error
	// GetScrubProgress 获取 scrub 进度 (0-100)
	GetScrubProgress(ctx context.Context, pool string) (float64, ScrubState, error)
}

// ScrubScheduler 是 ZFS Scrub 定时调度器
type ScrubScheduler struct {
	mu          sync.RWMutex
	logger      *slog.Logger
	config      *ScrubSchedulerConfig
	executor    ZFSExecutor
	schedules   map[string]*ScrubSchedule
	statuses    map[string]*ScrubStatus // keyed by pool name
	history     []*ScrubHistory
	cronParser  cron.Parser
	cronEntries map[string]cron.EntryID // schedule ID -> cron entry
	cronRunner  *cron.Cron
	stopCh      chan struct{}
	running     bool
}

// NewScheduler 创建调度器
func NewScheduler(logger *slog.Logger, config *ScrubSchedulerConfig, executor ZFSExecutor) *ScrubScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	return &ScrubScheduler{
		logger:      logger,
		config:      config,
		executor:    executor,
		schedules:   make(map[string]*ScrubSchedule),
		statuses:    make(map[string]*ScrubStatus),
		history:     make([]*ScrubHistory, 0, 64),
		cronParser:  cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		cronEntries: make(map[string]cron.EntryID),
		cronRunner: cron.New(cron.WithParser(cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
		))),
		stopCh: make(chan struct{}),
	}
}

// Start 启动调度器
func (s *ScrubScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})

	// 注册所有已启用的调度的 cron 任务
	for _, sch := range s.schedules {
		if sch.Enabled {
			s.registerCronJob(sch)
		}
	}
	s.mu.Unlock()

	s.cronRunner.Start()

	// 启动状态监控协程
	go s.monitorLoop()

	s.logger.Info("scrub scheduler started")
}

// Stop 停止调度器
func (s *ScrubScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	ctx := s.cronRunner.Stop()
	<-ctx.Done()

	s.logger.Info("scrub scheduler stopped")
}

// monitorLoop 定期检查所有运行中的 scrub 状态
func (s *ScrubScheduler) monitorLoop() {
	interval := time.Duration(s.config.PollIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pollAllStatuses()
		}
	}
}

// pollAllStatuses 轮询所有运行中的 scrub 状态
func (s *ScrubScheduler) pollAllStatuses() {
	s.mu.RLock()
	runningPools := make([]string, 0)
	for pool, st := range s.statuses {
		if st.State == ScrubStateRunning {
			runningPools = append(runningPools, pool)
		}
	}
	s.mu.RUnlock()

	for _, pool := range runningPools {
		s.pollPoolStatus(pool)
	}
}

// pollPoolStatus 查询单个池的 scrub 状态
func (s *ScrubScheduler) pollPoolStatus(pool string) {
	ctx := context.Background()
	progress, state, err := s.executor.GetScrubProgress(ctx, pool)

	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.statuses[pool]
	if !ok {
		return
	}

	if err != nil {
		st.Errors = append(st.Errors, err.Error())
		s.logger.Error("failed to get scrub progress", "pool", pool, "error", err)
		return
	}

	st.Progress = progress
	st.State = state

	if state == ScrubStateCompleted || state == ScrubStateFailed {
		st.EndTime = time.Now()
		s.recordHistory(st, pool)

		// 如果失败且还有重试次数，则安排重试
		if state == ScrubStateFailed {
			s.handleRetry(pool)
		}
	}
}

// handleRetry 处理失败重试逻辑
func (s *ScrubScheduler) handleRetry(pool string) {
	st, ok := s.statuses[pool]
	if !ok {
		return
	}

	// 查找对应的调度配置获取最大重试次数
	var maxRetry int
	for _, sch := range s.schedules {
		if sch.PoolName == pool {
			maxRetry = sch.RetryCount
			break
		}
	}
	if maxRetry == 0 {
		maxRetry = s.config.DefaultRetryCount
	}

	if st.RetryAttempt < maxRetry {
		st.RetryAttempt++
		s.logger.Warn("retrying scrub",
			"pool", pool,
			"attempt", st.RetryAttempt,
			"max", maxRetry)

		// 延迟 30 秒后重试
		go func(attempt int) {
			time.Sleep(30 * time.Second)
			s.startScrubForPool(context.Background(), pool)
		}(st.RetryAttempt)
	}
}

// recordHistory 记录 scrub 历史
func (s *ScrubScheduler) recordHistory(st *ScrubStatus, pool string) {
	var scheduleID string
	for _, sch := range s.schedules {
		if sch.PoolName == pool {
			scheduleID = sch.ID
			break
		}
	}

	duration := st.EndTime.Sub(st.StartTime).Seconds()
	h := &ScrubHistory{
		ID:              uuid.New().String(),
		ScheduleID:      scheduleID,
		PoolName:        pool,
		State:           st.State,
		Progress:        st.Progress,
		StartTime:       st.StartTime,
		EndTime:         st.EndTime,
		DurationSeconds: duration,
		Errors:          st.Errors,
		RetryAttempt:    st.RetryAttempt,
		CreatedAt:       time.Now(),
	}

	s.history = append(s.history, h)

	// 限制历史记录数量
	if len(s.history) > s.config.MaxHistoryRecords {
		s.history = s.history[len(s.history)-s.config.MaxHistoryRecords:]
	}

	s.logger.Info("scrub history recorded",
		"pool", pool,
		"state", st.State,
		"duration", duration)
}

// inMaintenanceWindow 判断当前时间是否在维护窗口内
func (s *ScrubScheduler) inMaintenanceWindow(window MaintenanceWindow) bool {
	if window.Start == "" || window.End == "" {
		return true // 未配置窗口则随时可以执行
	}

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	startParts := parseHHMM(window.Start)
	endParts := parseHHMM(window.End)
	if startParts < 0 || endParts < 0 {
		return true // 解析失败则放行
	}

	if startParts <= endParts {
		// 同一天范围内，如 00:00 - 06:00
		return currentMinutes >= startParts && currentMinutes < endParts
	}
	// 跨天范围，如 22:00 - 06:00
	return currentMinutes >= startParts || currentMinutes < endParts
}

// parseHHMM 解析 HH:MM 格式为分钟数
func parseHHMM(s string) int {
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// AddSchedule 添加调度
func (s *ScrubScheduler) AddSchedule(sch *ScrubSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证 cron 表达式
	_, err := s.cronParser.Parse(sch.Schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", sch.Schedule, err)
	}

	if sch.ID == "" {
		sch.ID = uuid.New().String()
	}
	sch.CreatedAt = time.Now()
	sch.UpdatedAt = time.Now()

	s.schedules[sch.ID] = sch

	// 如果调度器已运行且已启用，注册 cron 任务
	if s.running && sch.Enabled {
		s.registerCronJob(sch)
	}

	s.logger.Info("scrub schedule added",
		"id", sch.ID,
		"pool", sch.PoolName,
		"schedule", sch.Schedule)

	return nil
}

// registerCronJob 注册 cron 定时任务
func (s *ScrubScheduler) registerCronJob(sch *ScrubSchedule) {
	// 先移除旧的
	if entryID, ok := s.cronEntries[sch.ID]; ok {
		s.cronRunner.Remove(entryID)
	}

	entryID, err := s.cronRunner.AddFunc(sch.Schedule, func() {
		s.onScheduleTrigger(sch.ID)
	})
	if err != nil {
		s.logger.Error("failed to add cron job", "id", sch.ID, "error", err)
		return
	}
	s.cronEntries[sch.ID] = entryID
}

// onScheduleTrigger 定时触发回调
func (s *ScrubScheduler) onScheduleTrigger(scheduleID string) {
	s.mu.RLock()
	sch, ok := s.schedules[scheduleID]
	s.mu.RUnlock()

	if !ok {
		return
	}

	// 检查是否在维护窗口内
	window := sch.MaintenanceWindow
	if window.Start == "" && window.End == "" {
		window = s.config.DefaultMaintenanceWindow
	}

	if !s.inMaintenanceWindow(window) {
		s.logger.Info("scrub skipped: outside maintenance window",
			"pool", sch.PoolName,
			"window", fmt.Sprintf("%s-%s", window.Start, window.End))
		return
	}

	s.startScrubForPool(context.Background(), sch.PoolName)
}

// startScrubForPool 启动指定池的 scrub
func (s *ScrubScheduler) startScrubForPool(ctx context.Context, pool string) {
	// 检查是否已在运行
	s.mu.RLock()
	if st, ok := s.statuses[pool]; ok && st.State == ScrubStateRunning {
		s.mu.RUnlock()
		s.logger.Warn("scrub already running", "pool", pool)
		return
	}
	s.mu.RUnlock()

	s.logger.Info("starting scrub", "pool", pool)

	err := s.executor.StartScrub(ctx, pool)
	now := time.Now()

	s.mu.Lock()
	if err != nil {
		s.statuses[pool] = &ScrubStatus{
			PoolName:  pool,
			State:     ScrubStateFailed,
			StartTime: now,
			EndTime:   now,
			Errors:    []string{err.Error()},
		}
		s.logger.Error("failed to start scrub", "pool", pool, "error", err)
	} else {
		s.statuses[pool] = &ScrubStatus{
			PoolName:  pool,
			State:     ScrubStateRunning,
			Progress:  0,
			StartTime: now,
		}
	}
	s.mu.Unlock()

	// 启动超时监控
	if sch := s.getScheduleForPool(pool); sch != nil && sch.MaxDuration > 0 {
		go s.watchTimeout(pool, sch.MaxDuration)
	}
}

// getScheduleForPool 获取池对应的调度配置
func (s *ScrubScheduler) getScheduleForPool(pool string) *ScrubSchedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sch := range s.schedules {
		if sch.PoolName == pool {
			return sch
		}
	}
	return nil
}

// watchTimeout 监控 scrub 是否超时
func (s *ScrubScheduler) watchTimeout(pool string, maxDuration int) {
	select {
	case <-s.stopCh:
		return
	case <-time.After(time.Duration(maxDuration) * time.Second):
		s.mu.RLock()
		st, ok := s.statuses[pool]
		if !ok || st.State != ScrubStateRunning {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		s.logger.Warn("scrub timed out, stopping", "pool", pool, "max_duration", maxDuration)
		ctx := context.Background()
		if err := s.executor.StopScrub(ctx, pool); err != nil {
			s.logger.Error("failed to stop scrub", "pool", pool, "error", err)
		}

		s.mu.Lock()
		st.EndTime = time.Now()
		st.State = ScrubStateFailed
		st.Errors = append(st.Errors, fmt.Sprintf("timed out after %d seconds", maxDuration))
		s.recordHistory(st, pool)
		s.handleRetry(pool)
		s.mu.Unlock()
	}
}

// UpdateSchedule 更新调度
func (s *ScrubScheduler) UpdateSchedule(id string, req *UpdateScheduleRequest) (*ScrubSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sch, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	if req.Schedule != nil {
		if _, err := s.cronParser.Parse(*req.Schedule); err != nil {
			return nil, fmt.Errorf("invalid cron expression %q: %w", *req.Schedule, err)
		}
		sch.Schedule = *req.Schedule
	}
	if req.MaintenanceWindow != nil {
		sch.MaintenanceWindow = *req.MaintenanceWindow
	}
	if req.MaxDuration != nil {
		sch.MaxDuration = *req.MaxDuration
	}
	if req.RetryCount != nil {
		sch.RetryCount = *req.RetryCount
	}
	if req.Enabled != nil {
		sch.Enabled = *req.Enabled
	}
	sch.UpdatedAt = time.Now()

	// 重新注册 cron 任务
	if s.running {
		if sch.Enabled {
			s.registerCronJob(sch)
		} else if entryID, ok := s.cronEntries[id]; ok {
			s.cronRunner.Remove(entryID)
			delete(s.cronEntries, id)
		}
	}

	s.logger.Info("scrub schedule updated", "id", id, "pool", sch.PoolName)
	return sch, nil
}

// DeleteSchedule 删除调度
func (s *ScrubScheduler) DeleteSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	// 移除 cron 任务
	if entryID, ok := s.cronEntries[id]; ok {
		s.cronRunner.Remove(entryID)
		delete(s.cronEntries, id)
	}

	delete(s.schedules, id)
	s.logger.Info("scrub schedule deleted", "id", id)
	return nil
}

// GetSchedule 获取单个调度
func (s *ScrubScheduler) GetSchedule(id string) (*ScrubSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sch, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	return sch, nil
}

// ListSchedules 列出所有调度
func (s *ScrubScheduler) ListSchedules() []*ScrubSchedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ScrubSchedule, 0, len(s.schedules))
	for _, sch := range s.schedules {
		result = append(result, sch)
	}
	return result
}

// StartScrub 手动触发指定池的 scrub
func (s *ScrubScheduler) StartScrub(ctx context.Context, pool string) error {
	s.startScrubForPool(ctx, pool)
	s.mu.RLock()
	st, ok := s.statuses[pool]
	s.mu.RUnlock()
	if ok && st.State == ScrubStateFailed && len(st.Errors) > 0 {
		return fmt.Errorf("%s", st.Errors[len(st.Errors)-1])
	}
	return nil
}

// GetPoolStatus 获取指定池的 scrub 状态
func (s *ScrubScheduler) GetPoolStatus(pool string) (*ScrubStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.statuses[pool]
	if !ok {
		// 返回默认 idle 状态
		return &ScrubStatus{
			PoolName: pool,
			State:    ScrubStateIdle,
		}, nil
	}
	return st, nil
}

// GetHistory 获取 scrub 历史记录
func (s *ScrubScheduler) GetHistory(limit int) []*ScrubHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ScrubHistory, limit)
	copy(result, s.history[start:])
	return result
}
