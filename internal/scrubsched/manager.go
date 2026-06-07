// Package scrubsched 提供智能Scrub调度功能
package scrubsched

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 接口定义 ==========

// PoolProvider 存储池接口.
type PoolProvider interface {
	// GetPool 获取存储池信息.
	GetPool(poolID string) (*PoolInfo, error)
	// ListPools 列出所有存储池.
	ListPools() ([]*PoolInfo, error)
}

// PoolInfo 存储池信息.
type PoolInfo struct {
	ID     string `json:"id"`     // 池ID
	Name   string `json:"name"`   // 池名称
	Status string `json:"status"` // 池状态
	Size   uint64 `json:"size"`   // 池大小
}

// ScrubExecutor Scrub执行器接口.
type ScrubExecutor interface {
	// StartScrub 启动Scrub.
	StartScrub(poolID string) error
	// StopScrub 停止Scrub.
	StopScrub(poolID string) error
	// PauseScrub 暂停Scrub.
	PauseScrub(poolID string) error
	// ResumeScrub 恢复Scrub.
	ResumeScrub(poolID string) error
	// GetScrubProgress 获取Scrub进度.
	GetScrubProgress(poolID string) (float64, error)
	// IsScrubRunning 判断Scrub是否在运行.
	IsScrubRunning(poolID string) (bool, error)
}

// IOCollector IO采集器接口.
type IOCollector interface {
	// CollectIOLoad 采集指定池的IO负载.
	CollectIOLoad(poolID string) (*IOLoad, error)
	// CollectAllIOLoad 采集所有池的IO负载.
	CollectAllIOLoad() (map[string]*IOLoad, error)
}

// HealthProvider 健康数据接口.
type HealthProvider interface {
	// GetPoolHealth 获取存储池健康汇总.
	GetPoolHealth(poolID string) (*PoolHealthSummary, error)
}

// AlertSender 告警发送接口.
type AlertSender interface {
	// SendAlert 发送告警.
	SendAlert(level, title, message string) error
}

// ========== 核心管理器 ==========

// Manager Scrub调度管理器.
type Manager struct {
	mu           sync.RWMutex
	policies     map[string]*Policy      // policyID -> Policy
	statuses     map[string]*ScrubStatus // poolID -> ScrubStatus
	history      []*ScrubRecord          // 执行历史
	ioHistory    map[string][]*IOLoad    // poolID -> IO负载历史
	scheduler    *Scheduler              // 调度引擎
	analyzer     *IOAnalyzer             // IO分析器
	persister    *Persister              // 持久化管理器
	poolProvider PoolProvider            // 存储池接口
	scrubExec    ScrubExecutor           // Scrub执行器
	ioCollector  IOCollector             // IO采集器
	healthProv   HealthProvider          // 健康数据接口
	alertSender  AlertSender             // 告警发送接口
	configPath   string                  // 配置存储路径
	stopCh       chan struct{}           // 停止信号
	running      bool                    // 是否在运行
}

// NewManager 创建Scrub调度管理器.
func NewManager(configPath string, poolProv PoolProvider, exec ScrubExecutor, collector IOCollector, health HealthProvider, alert AlertSender) *Manager {
	m := &Manager{
		policies:     make(map[string]*Policy),
		statuses:     make(map[string]*ScrubStatus),
		history:      make([]*ScrubRecord, 0),
		ioHistory:    make(map[string][]*IOLoad),
		poolProvider: poolProv,
		scrubExec:    exec,
		ioCollector:  collector,
		healthProv:   health,
		alertSender:  alert,
		configPath:   configPath,
		stopCh:       make(chan struct{}),
	}
	m.scheduler = NewScheduler(m)
	m.analyzer = NewIOAnalyzer(m)
	return m
}

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	// 启动持久化（会自动加载历史数据）
	if m.persister != nil {
		m.persister.Start()
	}

	// 启动调度引擎
	go m.scheduler.Start()
	// 启动IO分析器
	go m.analyzer.Start()

	log.Println("[scrubsched] 智能Scrub调度管理器已启动")
}

// SetPersister 设置持久化管理器.
func (m *Manager) SetPersister(p *Persister) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.persister = p
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)

	// 停止持久化（会自动保存一次）
	if m.persister != nil {
		m.persister.Stop()
	}

	log.Println("[scrubsched] 智能Scrub调度管理器已停止")
}

// ========== 策略管理 ==========

// CreatePolicy 创建调度策略.
func (m *Manager) CreatePolicy(req CreatePolicyRequest) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证存储池存在
	if m.poolProvider != nil {
		if _, err := m.poolProvider.GetPool(req.PoolID); err != nil {
			return nil, ErrPoolNotFound
		}
	}

	// 检查同名策略
	for _, p := range m.policies {
		if p.Name == req.Name {
			return nil, ErrPolicyExists
		}
	}

	// 验证调度配置
	if err := m.validateScheduleConfig(req.Schedule, req.CronExpr); err != nil {
		return nil, err
	}

	now := time.Now()
	policy := &Policy{
		ID:           generateID(),
		Name:         req.Name,
		PoolID:       req.PoolID,
		Schedule:     req.Schedule,
		CronExpr:     req.CronExpr,
		WeekDay:      req.WeekDay,
		MonthDay:     req.MonthDay,
		Hour:         req.Hour,
		Minute:       req.Minute,
		Priority:     req.Priority,
		Enabled:      req.Enabled,
		AvoidPeak:    req.AvoidPeak,
		PeakWindows:  req.PeakWindows,
		IOThreshold:  req.IOThreshold,
		MaxDuration:  time.Duration(req.MaxDuration) * time.Hour,
		ThrottleRate: req.ThrottleRate,
		HealthAdjust: req.HealthAdjust,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 设置默认值
	if policy.Priority == 0 {
		policy.Priority = PriorityNormal
	}
	if policy.MaxDuration == 0 {
		policy.MaxDuration = 48 * time.Hour
	}
	if policy.ThrottleRate == 0 {
		policy.ThrottleRate = 0.5
	}
	if policy.IOThreshold.ResumeRatio == 0 {
		policy.IOThreshold.ResumeRatio = 0.7
	}

	// 计算下次执行时间
	nextRun := m.calculateNextRun(policy)
	policy.NextRun = &nextRun

	m.policies[policy.ID] = policy
	return policy, nil
}

// GetPolicy 获取策略详情.
func (m *Manager) GetPolicy(id string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(id string, req UpdatePolicyRequest) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}

	// 更新各字段
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Schedule != nil {
		if err := m.validateScheduleConfig(*req.Schedule, p.CronExpr); err != nil {
			return nil, err
		}
		p.Schedule = *req.Schedule
	}
	if req.CronExpr != nil {
		if err := m.validateScheduleConfig(p.Schedule, *req.CronExpr); err != nil {
			return nil, err
		}
		p.CronExpr = *req.CronExpr
	}
	if req.WeekDay != nil {
		p.WeekDay = *req.WeekDay
	}
	if req.MonthDay != nil {
		p.MonthDay = *req.MonthDay
	}
	if req.Hour != nil {
		p.Hour = *req.Hour
	}
	if req.Minute != nil {
		p.Minute = *req.Minute
	}
	if req.Priority != nil {
		p.Priority = *req.Priority
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.AvoidPeak != nil {
		p.AvoidPeak = *req.AvoidPeak
	}
	if req.PeakWindows != nil {
		p.PeakWindows = req.PeakWindows
	}
	if req.IOThreshold != nil {
		p.IOThreshold = *req.IOThreshold
	}
	if req.MaxDuration != nil {
		p.MaxDuration = time.Duration(*req.MaxDuration) * time.Hour
	}
	if req.ThrottleRate != nil {
		p.ThrottleRate = *req.ThrottleRate
	}
	if req.HealthAdjust != nil {
		p.HealthAdjust = *req.HealthAdjust
	}

	p.UpdatedAt = time.Now()

	// 重新计算下次执行时间
	nextRun := m.calculateNextRun(p)
	p.NextRun = &nextRun

	return p, nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return ErrPolicyNotFound
	}
	delete(m.policies, id)
	return nil
}

// ========== Scrub控制 ==========

// TriggerScrub 手动触发Scrub.
func (m *Manager) TriggerScrub(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证存储池
	if m.poolProvider != nil {
		if _, err := m.poolProvider.GetPool(poolID); err != nil {
			return ErrPoolNotFound
		}
	}

	// 检查是否已有Scrub在运行
	if status, ok := m.statuses[poolID]; ok && status.State == StateRunning {
		return ErrScrubAlreadyRunning
	}

	// 启动Scrub
	if m.scrubExec != nil {
		if err := m.scrubExec.StartScrub(poolID); err != nil {
			return fmt.Errorf("启动Scrub失败: %w", err)
		}
	}

	now := time.Now()
	m.statuses[poolID] = &ScrubStatus{
		PoolID:    poolID,
		State:     StateRunning,
		StartTime: &now,
		IsManual:  true,
	}

	return nil
}

// PauseScrub 暂停Scrub.
func (m *Manager) PauseScrub(poolID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[poolID]
	if !ok || status.State != StateRunning {
		return ErrScrubNotRunning
	}

	if m.scrubExec != nil {
		if err := m.scrubExec.PauseScrub(poolID); err != nil {
			return fmt.Errorf("暂停Scrub失败: %w", err)
		}
	}

	now := time.Now()
	status.State = StatePaused
	status.PauseTime = &now
	status.PauseCount++
	status.PauseReason = reason

	return nil
}

// ResumeScrub 恢复Scrub.
func (m *Manager) ResumeScrub(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[poolID]
	if !ok || status.State != StatePaused {
		return ErrScrubNotRunning
	}

	if m.scrubExec != nil {
		if err := m.scrubExec.ResumeScrub(poolID); err != nil {
			return fmt.Errorf("恢复Scrub失败: %w", err)
		}
	}

	now := time.Now()
	status.State = StateRunning
	status.ResumeTime = &now
	status.PauseReason = ""

	return nil
}

// CancelScrub 取消Scrub.
func (m *Manager) CancelScrub(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[poolID]
	if !ok || (status.State != StateRunning && status.State != StatePaused) {
		return ErrScrubNotRunning
	}

	if m.scrubExec != nil {
		if err := m.scrubExec.StopScrub(poolID); err != nil {
			return fmt.Errorf("取消Scrub失败: %w", err)
		}
	}

	// 记录历史
	endTime := time.Now()
	var duration int64
	if status.StartTime != nil {
		duration = int64(endTime.Sub(*status.StartTime).Seconds())
	}

	record := &ScrubRecord{
		ID:         generateID(),
		PoolID:     poolID,
		PolicyID:   status.PolicyID,
		State:      StateCancelled,
		StartTime:  *status.StartTime,
		EndTime:    endTime,
		Duration:   duration,
		PauseCount: status.PauseCount,
		IsManual:   status.IsManual,
	}
	m.history = append(m.history, record)

	// 清除状态
	delete(m.statuses, poolID)

	return nil
}

// ========== 状态查询 ==========

// GetScrubStatus 获取所有池的Scrub状态.
func (m *Manager) GetScrubStatus() map[string]*ScrubStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]*ScrubStatus)
	for k, v := range m.statuses {
		statuses[k] = v
	}
	return statuses
}

// GetPoolScrubStatus 获取指定池的Scrub状态.
func (m *Manager) GetPoolScrubStatus(poolID string) (*ScrubStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.statuses[poolID]
	if !ok {
		return &ScrubStatus{
			PoolID: poolID,
			State:  StateIdle,
		}, nil
	}
	return status, nil
}

// GetHistory 获取Scrub历史记录.
func (m *Manager) GetHistory(poolID string) []*ScrubRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if poolID == "" {
		records := make([]*ScrubRecord, len(m.history))
		copy(records, m.history)
		return records
	}

	var result []*ScrubRecord
	for _, r := range m.history {
		if r.PoolID == poolID {
			result = append(result, r)
		}
	}
	return result
}

// ========== IO负载 ==========

// GetCurrentIOLoad 获取当前IO负载.
func (m *Manager) GetCurrentIOLoad() map[string]*IOLoad {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ioCollector == nil {
		return nil
	}
	loads, err := m.ioCollector.CollectAllIOLoad()
	if err != nil {
		log.Printf("[scrubsched] 采集IO负载失败: %v", err)
		return nil
	}
	return loads
}

// GetIOHistory 获取IO历史.
func (m *Manager) GetIOHistory(poolID string) []*IOLoad {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records, ok := m.ioHistory[poolID]
	if !ok {
		return nil
	}
	result := make([]*IOLoad, len(records))
	copy(result, records)
	return result
}

// ========== 建议 ==========

// GetRecommendations 获取Scrub调度建议.
func (m *Manager) GetRecommendations() []ScrubRecommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recs []ScrubRecommendation

	// 检查各池健康状态
	if m.healthProv != nil {
		pools, err := m.poolProvider.ListPools()
		if err == nil {
			for _, pool := range pools {
				health, err := m.healthProv.GetPoolHealth(pool.ID)
				if err != nil {
					continue
				}

				rec := ScrubRecommendation{
					PoolID:   pool.ID,
					PoolName: pool.Name,
				}

				switch health.OverallHealth {
				case "critical":
					rec.Reason = "磁盘健康状态严重，建议立即执行Scrub检查数据完整性"
					rec.Priority = "urgent"
					rec.SuggestedAt = time.Now()
				case "warning":
					rec.Reason = "部分磁盘存在告警，建议尽快执行Scrub"
					rec.Priority = "normal"
					rec.SuggestedAt = time.Now().Add(1 * time.Hour)
				default:
					rec.Reason = "存储池状态正常，建议按计划执行Scrub"
					rec.Priority = "low"
					rec.SuggestedAt = time.Now().Add(24 * time.Hour)
				}

				rec.HealthNote = fmt.Sprintf("健康磁盘: %d/%d, 告警: %d, 严重: %d",
					health.HealthyDisks, health.DiskCount,
					health.WarningDisks, health.CriticalDisks)

				recs = append(recs, rec)
			}
		}
	}

	// 检查是否有池长时间未Scrub
	for _, p := range m.policies {
		if p.LastRun == nil {
			recs = append(recs, ScrubRecommendation{
				PoolID:   p.PoolID,
				Reason:   "该池从未执行过Scrub，建议尽快执行",
				Priority: "normal",
			})
		} else if time.Since(*p.LastRun) > 30*24*time.Hour {
			recs = append(recs, ScrubRecommendation{
				PoolID:   p.PoolID,
				Reason:   fmt.Sprintf("距上次Scrub已超过30天，建议执行一次完整Scrub"),
				Priority: "normal",
			})
		}
	}

	return recs
}

// ========== 内部方法 ==========

// validateScheduleConfig 验证调度配置.
func (m *Manager) validateScheduleConfig(schedule ScheduleType, cronExpr string) error {
	switch schedule {
	case ScheduleWeekly:
		// WeekDay 由调用方校验范围 0-6
	case ScheduleMonthly:
		// MonthDay 由调用方校验范围 1-31
	case ScheduleCustom:
		if cronExpr == "" {
			return ErrInvalidCronExpr
		}
	default:
		return ErrInvalidCronExpr
	}
	return nil
}

// calculateNextRun 计算下次执行时间.
func (m *Manager) calculateNextRun(p *Policy) time.Time {
	now := time.Now()

	switch p.Schedule {
	case ScheduleWeekly:
		// 计算下一个指定星期几
		daysUntil := (p.WeekDay - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 {
			// 如果今天就是目标日，检查时间是否已过
			target := time.Date(now.Year(), now.Month(), now.Day(), p.Hour, p.Minute, 0, 0, now.Location())
			if now.After(target) {
				daysUntil = 7
			}
		}
		next := now.AddDate(0, 0, daysUntil)
		return time.Date(next.Year(), next.Month(), next.Day(), p.Hour, p.Minute, 0, 0, now.Location())

	case ScheduleMonthly:
		// 计算下一个指定日期
		year, month := now.Year(), now.Month()
		day := p.MonthDay
		if day > 28 {
			day = 28 // 简化处理，避免月末日期问题
		}
		target := time.Date(year, month, day, p.Hour, p.Minute, 0, 0, now.Location())
		if now.After(target) {
			target = target.AddDate(0, 1, 0)
		}
		return target

	case ScheduleCustom:
		// 自定义cron表达式的简单解析（简化版）
		// 实际项目中应使用 cron 库
		return now.Add(24 * time.Hour)

	default:
		return now.Add(24 * time.Hour)
	}
}

// isInPeakWindow 判断当前是否在高峰窗口.
func (m *Manager) isInPeakWindow(p *Policy, now time.Time) bool {
	if !p.AvoidPeak {
		return false
	}

	weekday := int(now.Weekday())
	hour := now.Hour()
	minute := now.Minute()
	currentMinutes := hour*60 + minute

	for _, w := range p.PeakWindows {
		// 检查日期匹配
		if w.DayOfWeek != -1 && w.DayOfWeek != weekday {
			continue
		}

		startMinutes := w.StartHour*60 + w.StartMin
		endMinutes := w.EndHour*60 + w.EndMin

		// 处理跨午夜的情况
		if startMinutes <= endMinutes {
			if currentMinutes >= startMinutes && currentMinutes < endMinutes {
				return true
			}
		} else {
			if currentMinutes >= startMinutes || currentMinutes < endMinutes {
				return true
			}
		}
	}

	return false
}

// addIORecord 添加IO记录.
func (m *Manager) addIORecord(poolID string, load *IOLoad) {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, ok := m.ioHistory[poolID]
	if !ok {
		records = make([]*IOLoad, 0)
	}

	records = append(records, load)

	// 保留最近1000条记录
	if len(records) > 1000 {
		records = records[len(records)-1000:]
	}

	m.ioHistory[poolID] = records
}

// completeScrub 完成Scrub.
func (m *Manager) completeScrub(poolID string, state ScrubState, errorsFound, errorsFixed int, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.statuses[poolID]
	if !ok {
		return
	}

	endTime := time.Now()
	var duration int64
	if status.StartTime != nil {
		duration = int64(endTime.Sub(*status.StartTime).Seconds())
	}

	record := &ScrubRecord{
		ID:           generateID(),
		PoolID:       poolID,
		PolicyID:     status.PolicyID,
		State:        state,
		StartTime:    *status.StartTime,
		EndTime:      endTime,
		Duration:     duration,
		PauseCount:   status.PauseCount,
		ErrorsFound:  errorsFound,
		ErrorsFixed:  errorsFixed,
		IsManual:     status.IsManual,
		ErrorMessage: errMsg,
	}

	// 计算速度
	if duration > 0 {
		// 简化：假设每次Scrub的数据量与池大小成正比
		record.SpeedMBps = float64(100*1024) / float64(duration) // 假设100TB池
	}

	m.history = append(m.history, record)

	// 更新策略的最后执行时间
	if status.PolicyID != "" {
		if p, ok := m.policies[status.PolicyID]; ok {
			now := time.Now()
			p.LastRun = &now
			p.RunCount++
			nextRun := m.calculateNextRun(p)
			p.NextRun = &nextRun
		}
	}

	// 清除状态
	delete(m.statuses, poolID)

	// 发送告警
	if state == StateFailed && m.alertSender != nil {
		_ = m.alertSender.SendAlert("error", "Scrub执行失败",
			fmt.Sprintf("存储池 %s 的Scrub执行失败: %s", poolID, errMsg))
	}
	if errorsFound > 0 && m.alertSender != nil {
		_ = m.alertSender.SendAlert("warning", "Scrub发现数据错误",
			fmt.Sprintf("存储池 %s 的Scrub发现 %d 个错误，已修复 %d 个", poolID, errorsFound, errorsFixed))
	}
}

// generateID 生成唯一ID.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
