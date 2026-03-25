package snapshot

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 调度器.
type Scheduler struct {
	mu sync.RWMutex

	// cron 调度器
	cron *cron.Cron

	// jobIDs 任务 ID 映射
	jobIDs map[string]cron.EntryID

	// policyManager 策略管理器
	policyManager *PolicyManager

	// running 是否运行中
	running bool
}

// NewScheduler 创建调度器.
func NewScheduler(pm *PolicyManager) *Scheduler {
	return &Scheduler{
		cron:          cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		jobIDs:        make(map[string]cron.EntryID),
		policyManager: pm,
	}
}

// Start 启动调度器.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		s.cron.Start()
		s.running = true
	}
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.running = false
	}
}

// AddJob 添加定时任务.
func (s *Scheduler) AddJob(policy *Policy) error {
	if policy.Type != PolicyTypeScheduled || policy.Schedule == nil {
		return fmt.Errorf("策略不是定时类型或未配置调度")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先移除
	if entryID, exists := s.jobIDs[policy.ID]; exists {
		s.cron.Remove(entryID)
	}

	// 生成 cron 表达式
	cronExpr := s.generateCronExpression(policy.Schedule)
	if cronExpr == "" {
		return fmt.Errorf("无法生成 cron 表达式")
	}

	// 创建任务
	policyID := policy.ID
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		// 获取最新的策略
		p, err := s.policyManager.GetPolicy(policyID)
		if err != nil {
			return
		}
		if !p.Enabled {
			return
		}
		_, _ = s.policyManager.ExecutePolicy(policyID)
	})

	if err != nil {
		return fmt.Errorf("添加定时任务失败: %w", err)
	}

	s.jobIDs[policy.ID] = entryID

	return nil
}

// RemoveJob 移除定时任务.
func (s *Scheduler) RemoveJob(policyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.jobIDs[policyID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, policyID)
	}
}

// UpdateJob 更新定时任务.
func (s *Scheduler) UpdateJob(policy *Policy) error {
	s.RemoveJob(policy.ID)
	if policy.Enabled {
		return s.AddJob(policy)
	}
	return nil
}

// ValidateCron 验证 cron 表达式.
func (s *Scheduler) ValidateCron(expr string) bool {
	_, err := cron.ParseStandard(expr)
	return err == nil
}

// CalculateNextRun 计算下次执行时间.
func (s *Scheduler) CalculateNextRun(policy *Policy) time.Time {
	s.mu.RLock()
	entryID, exists := s.jobIDs[policy.ID]
	s.mu.RUnlock()

	if !exists {
		return time.Time{}
	}

	entry := s.cron.Entry(entryID)
	return entry.Next
}

// GetNextRuns 获取所有任务的下次执行时间.
func (s *Scheduler) GetNextRuns() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]time.Time)
	for policyID, entryID := range s.jobIDs {
		entry := s.cron.Entry(entryID)
		result[policyID] = entry.Next
	}
	return result
}

// generateCronExpression 生成 cron 表达式.
func (s *Scheduler) generateCronExpression(schedule *ScheduleConfig) string {
	if schedule == nil {
		return ""
	}

	minute := schedule.Minute
	hour := schedule.Hour

	// 确保范围有效
	if minute < 0 || minute > 59 {
		minute = 0
	}
	if hour < 0 || hour > 23 {
		hour = 0
	}

	switch schedule.Type {
	case ScheduleTypeHourly:
		// 每小时执行
		// 格式: "0 M * * * *" -> 每小时的 M 分执行
		interval := schedule.IntervalHours
		if interval <= 0 {
			interval = 1
		}
		if interval == 1 {
			return fmt.Sprintf("0 %d * * * *", minute)
		}
		// 每 N 小时执行
		return fmt.Sprintf("0 %d */%d * * *", minute, interval)

	case ScheduleTypeDaily:
		// 每天执行
		// 格式: "0 M H * * *" -> 每天的 H:M 执行
		return fmt.Sprintf("0 %d %d * * *", minute, hour)

	case ScheduleTypeWeekly:
		// 每周执行
		// 格式: "0 M H * * D" -> 每周 D 的 H:M 执行
		dayOfWeek := schedule.DayOfWeek
		if dayOfWeek < 0 || dayOfWeek > 6 {
			dayOfWeek = 0
		}
		return fmt.Sprintf("0 %d %d * * %d", minute, hour, dayOfWeek)

	case ScheduleTypeMonthly:
		// 每月执行
		// 格式: "0 M H D * *" -> 每月 D 号的 H:M 执行
		dayOfMonth := schedule.DayOfMonth
		if dayOfMonth < 1 || dayOfMonth > 31 {
			dayOfMonth = 1
		}
		return fmt.Sprintf("0 %d %d %d * *", minute, hour, dayOfMonth)

	case ScheduleTypeCustom:
		// 自定义 cron 表达式
		return schedule.CronExpression

	default:
		return ""
	}
}

// ListJobs 列出所有任务.
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []JobInfo
	for policyID, entryID := range s.jobIDs {
		entry := s.cron.Entry(entryID)
		policy, err := s.policyManager.GetPolicy(policyID)
		if err != nil {
			// 策略不存在时跳过此任务
			continue
		}

		jobs = append(jobs, JobInfo{
			PolicyID:   policyID,
			PolicyName: policy.Name,
			NextRun:    entry.Next,
			PrevRun:    entry.Prev,
			Schedule:   fmt.Sprintf("%v", entry.Schedule),
			Enabled:    policy.Enabled,
		})
	}
	return jobs
}

// JobInfo 任务信息.
type JobInfo struct {
	PolicyID   string    `json:"policyId"`
	PolicyName string    `json:"policyName"`
	NextRun    time.Time `json:"nextRun"`
	PrevRun    time.Time `json:"prevRun"`
	Schedule   string    `json:"schedule"`
	Enabled    bool      `json:"enabled"`
}

// GetJobStatus 获取任务状态.
func (s *Scheduler) GetJobStatus(policyID string) (*JobInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entryID, exists := s.jobIDs[policyID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", policyID)
	}

	entry := s.cron.Entry(entryID)
	policy, err := s.policyManager.GetPolicy(policyID)
	if err != nil {
		return nil, err
	}

	return &JobInfo{
		PolicyID:   policyID,
		PolicyName: policy.Name,
		NextRun:    entry.Next,
		PrevRun:    entry.Prev,
		Schedule:   fmt.Sprintf("%v", entry.Schedule),
		Enabled:    policy.Enabled,
	}, nil
}

// IsRunning 检查调度器是否运行中.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
