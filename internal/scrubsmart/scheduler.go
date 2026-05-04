package scrubsmart

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ScrubBackend Scrub 执行后端接口，用于对接实际的 zpool scrub 命令。
type ScrubBackend interface {
	// Start 启动 Scrub.
	Start(ctx context.Context, pool string) error
	// Stop 暂停/取消 Scrub.
	Stop(pool string) error
	// GetProgress 获取 Scrub 进度.
	GetProgress(pool string) (*ScrubProgress, error)
}

// Scheduler 智能避峰调度器.
type Scheduler struct {
	mu          sync.RWMutex
	config      *Config
	state       ScrubState
	backend     ScrubBackend
	logger      *zap.Logger
	progress    *ScrubProgress
	ioLoad      *IOLoad
	lastError   string
	cancelCtx   context.CancelFunc
	wg          sync.WaitGroup
	done        chan struct{}
	manualPause bool // 手动暂停标记
	forceResume bool // 强制恢复标记（忽略避峰窗口）
	startTime   time.Time
}

// NewScheduler 创建调度器.
func NewScheduler(backend ScrubBackend, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	cfg := DefaultConfig()
	return &Scheduler{
		config:  cfg,
		state:   StateIdle,
		backend: backend,
		logger:  logger,
		done:    make(chan struct{}),
	}
}

// SetConfig 更新配置.
func (s *Scheduler) SetConfig(cfg *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	s.logger.Info("配置已更新",
		zap.Bool("enabled", cfg.Enabled),
		zap.Int("avoidance_windows", len(cfg.AvoidanceWindows)),
	)
}

// GetConfig 获取当前配置.
func (s *Scheduler) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.config
}

// Start 启动调度循环.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.cancelCtx != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelCtx = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runLoop(ctx)
	}()

	s.logger.Info("智能避峰调度器已启动")
}

// Stop 停止调度循环.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.cancelCtx != nil {
		s.cancelCtx()
		s.cancelCtx = nil
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.logger.Info("智能避峰调度器已停止")
}

// GetStatus 获取当前状态.
func (s *Scheduler) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := Status{
		State:                s.state,
		Pool:                 s.config.TargetPool,
		Progress:             s.progress,
		IOLoad:               s.ioLoad,
		InAvoidanceWindow:    s.isInAvoidanceWindowLocked(time.Now()),
		LastError:            s.lastError,
		Config:               *s.config,
	}

	// 计算预计恢复时间
	if s.state == StatePaused || s.state == StateManual {
		nextResume := s.calcNextResumeLocked(time.Now())
		status.NextResume = &nextResume
	}

	return status
}

// Pause 手动暂停 Scrub.
func (s *Scheduler) Pause(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning {
		return ErrScrubNotRunning
	}

	s.logger.Info("手动暂停 Scrub", zap.String("reason", reason))
	if err := s.backend.Stop(s.config.TargetPool); err != nil {
		s.lastError = err.Error()
		return fmt.Errorf("暂停 Scrub 失败: %w", err)
	}

	s.state = StateManual
	s.manualPause = true
	s.forceResume = false
	return nil
}

// Resume 手动恢复 Scrub.
func (s *Scheduler) Resume(force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StatePaused && s.state != StateManual {
		return ErrScrubNotRunning
	}

	if !force && s.isInAvoidanceWindowLocked(time.Now()) {
		s.logger.Warn("当前在避峰窗口内，使用 force=true 强制恢复")
		return fmt.Errorf("当前在避峰窗口内，请使用 force=true 强制恢复")
	}

	s.logger.Info("恢复 Scrub", zap.Bool("force", force))
	if err := s.startScrubLocked(); err != nil {
		return err
	}

	s.manualPause = false
	s.forceResume = force
	return nil
}

// runLoop 主调度循环.
func (s *Scheduler) runLoop(ctx context.Context) {
	s.logger.Info("调度循环启动")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("调度循环收到退出信号")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick 单次调度检查.
func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	if !cfg.Enabled {
		return
	}

	now := time.Now()
	inWindow := s.isInAvoidanceWindowLocked(now)

	// 更新 IO 负载
	s.updateIOLoad()

	switch s.state {
	case StateIdle:
		// 空闲状态，检查是否应该启动 Scrub
		if !inWindow && cfg.TargetPool != "" {
			s.logger.Info("非避峰时段，启动 Scrub", zap.String("pool", cfg.TargetPool))
			_ = s.startScrubLocked()
		}

	case StateRunning:
		// 运行中，检查是否需要暂停
		if s.manualPause {
			return // 手动暂停，不自动恢复
		}

		// 检查避峰窗口
		if inWindow {
			s.logger.Info("进入避峰窗口，暂停 Scrub")
			if err := s.backend.Stop(cfg.TargetPool); err != nil {
				s.logger.Error("暂停 Scrub 失败", zap.Error(err))
				s.lastError = err.Error()
			} else {
				s.state = StatePaused
			}
			return
		}

		// 检查 IO 负载
		if s.ioLoad != nil {
			if s.ioLoad.WriteMBps > cfg.IOWriteThresholdMBps ||
				s.ioLoad.ReadMBps > cfg.IOReadThresholdMBps {
				s.logger.Warn("IO 负载过高，自动暂停 Scrub",
					zap.Float64("read_mbps", s.ioLoad.ReadMBps),
					zap.Float64("write_mbps", s.ioLoad.WriteMBps),
				)
				if err := s.backend.Stop(cfg.TargetPool); err != nil {
					s.logger.Error("IO 避峰暂停失败", zap.Error(err))
					s.lastError = err.Error()
				} else {
					s.state = StatePaused
				}
				return
			}
		}

		// 更新进度
		s.updateProgressLocked()

	case StatePaused:
		// 避峰暂停中，检查是否应该恢复
		if !inWindow {
			// IO 负载也需检查
			if s.ioLoad != nil {
				if s.ioLoad.WriteMBps > cfg.IOWriteThresholdMBps ||
					s.ioLoad.ReadMBps > cfg.IOReadThresholdMBps {
					return // IO 仍然高，继续等待
				}
			}
			s.logger.Info("避峰窗口结束，恢复 Scrub")
			_ = s.startScrubLocked()
		}

	case StateManual:
		// 手动暂停，不做自动恢复
		return

	case StateComplete, StateError:
		// 已完成或出错，不做处理
		return
	}
}

// isInAvoidanceWindowLocked 检查当前时间是否在避峰窗口内（调用者持有锁）.
func (s *Scheduler) isInAvoidanceWindowLocked(t time.Time) bool {
	for _, w := range s.config.AvoidanceWindows {
		if w.Contains(t) {
			return true
		}
	}
	return false
}

// calcNextResumeLocked 计算下一个恢复时间（调用者持有锁）.
func (s *Scheduler) calcNextResumeLocked(now time.Time) time.Time {
	// 遍历未来 7 天，找到最早的一个窗口结束时间
	for dayOffset := 0; dayOffset <= 7; dayOffset++ {
		t := now.AddDate(0, 0, dayOffset)
		weekday := Weekday(t.Weekday())

		for _, w := range s.config.AvoidanceWindows {
			hasDay := false
			for _, d := range w.Weekdays {
				if d == weekday {
					hasDay = true
					break
				}
			}
			if !hasDay {
				continue
			}

			endTime := time.Date(t.Year(), t.Month(), t.Day(),
				w.EndHour, w.EndMinute, 0, 0, t.Location())

			// 如果是今天且已过结束时间，跳过
			if dayOffset == 0 && !endTime.After(now) {
				continue
			}

			// 如果是今天，窗口还未开始，返回该窗口的结束时间
			if dayOffset == 0 {
				startTime := time.Date(t.Year(), t.Month(), t.Day(),
					w.StartHour, w.StartMinute, 0, 0, t.Location())
				if now.Before(startTime) {
					continue // 还没进入窗口，不会暂停
				}
			}

			return endTime
		}
	}

	// 兜底：24小时后
	return now.Add(24 * time.Hour)
}

// startScrubLocked 启动 Scrub（调用者持有锁）.
func (s *Scheduler) startScrubLocked() error {
	if s.config.TargetPool == "" {
		return fmt.Errorf("未配置目标存储池")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelCtx = cancel

	if err := s.backend.Start(ctx, s.config.TargetPool); err != nil {
		s.lastError = err.Error()
		s.state = StateError
		return fmt.Errorf("启动 Scrub 失败: %w", err)
	}

	s.state = StateRunning
	s.startTime = time.Now()
	s.manualPause = false
	s.logger.Info("Scrub 已启动", zap.String("pool", s.config.TargetPool))
	return nil
}

// updateProgressLocked 更新进度信息（调用者持有锁）.
func (s *Scheduler) updateProgressLocked() {
	if s.state != StateRunning || s.backend == nil {
		return
	}

	progress, err := s.backend.GetProgress(s.config.TargetPool)
	if err != nil {
		s.logger.Warn("获取 Scrub 进度失败", zap.Error(err))
		return
	}

	s.progress = progress

	// 检查是否完成
	if progress != nil && progress.Percentage >= 100.0 {
		s.state = StateComplete
		s.logger.Info("Scrub 已完成",
			zap.Duration("duration", progress.Duration),
			zap.Int64("errors", progress.Errors),
		)
	}
}

// updateIOLoad 更新 IO 负载.
func (s *Scheduler) updateIOLoad() {
	load, err := s.readIOLoad()
	if err != nil {
		s.logger.Debug("读取 IO 负载失败", zap.Error(err))
		return
	}
	s.ioLoad = load
}

// readIOLoad 从 /proc/diskstats 读取 IO 负载（简化实现）.
func (s *Scheduler) readIOLoad() (*IOLoad, error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, err
	}

	var totalReadSectors, totalWriteSectors int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		// 只统计主磁盘（如 sda, nvme0n1）
		name := fields[2]
		if !isMainDisk(name) {
			continue
		}
		// field[5] = sectors read, field[9] = sectors written
		readSectors, _ := strconv.ParseInt(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseInt(fields[9], 10, 64)
		totalReadSectors += readSectors
		totalWriteSectors += writeSectors
	}

	// 简化：返回当前累计值作为快照（实际应用需要差值计算）
	load := &IOLoad{
		ReadMBps:  float64(totalReadSectors) * 512 / 1024 / 1024,
		WriteMBps: float64(totalWriteSectors) * 512 / 1024 / 1024,
		Timestamp: time.Now(),
	}
	return load, nil
}

// isMainDisk 判断是否为主磁盘设备.
func isMainDisk(name string) bool {
	if strings.HasPrefix(name, "sd") && len(name) == 3 {
		return true
	}
	if strings.HasPrefix(name, "nvme") && strings.HasSuffix(name, "n1") {
		return true
	}
	if strings.HasPrefix(name, "vd") && len(name) == 3 {
		return true
	}
	return false
}
