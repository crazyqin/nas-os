package snapshotmgr

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RetentionPolicy 高级快照保留策略
// 参考飞牛 fnOS 的快照保留策略高级设置和群晖 Snapshot Replication。
// 支持按不同时间粒度分别设置保留份数，实现精细化的快照生命周期管理。
type RetentionPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	// TargetScope 策略作用范围：global / pool / dataset / team
	TargetScope string `json:"target_scope"`
	// TargetRef 目标引用（pool名/dataset路径/team ID，取决于 TargetScope）
	TargetRef string `json:"target_ref"`

	// 各时间粒度的保留份数，0 表示不保留该粒度的快照
	Minutely int `json:"minutely"` // 每分钟快照保留份数
	Hourly   int `json:"hourly"`   // 每小时快照保留份数
	Daily    int `json:"daily"`    // 每天保留份数
	Weekly   int `json:"weekly"`   // 每周保留份数
	Monthly  int `json:"monthly"`  // 每月保留份数
	Yearly   int `json:"yearly"`   // 每年保留份数

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MinutelyStrategy 每分钟快照策略（高频场景，如实时写入保护）.
type MinutelyStrategy struct {
	Retain int `json:"retain"` // 保留份数
}

// HourlyStrategy 每小时快照策略.
type HourlyStrategy struct {
	Retain int `json:"retain"`
}

// DailyStrategy 每天快照策略.
type DailyStrategy struct {
	Retain   int `json:"retain"`
	KeepHour int `json:"keep_hour"` // 优先保留该小时的快照（0-23）
}

// WeeklyStrategy 每周快照策略.
type WeeklyStrategy struct {
	Retain      int    `json:"retain"`
	KeepWeekday string `json:"keep_weekday"` // 优先保留该星期几的快照 (Monday..Sunday)
}

// MonthlyStrategy 每月快照策略.
type MonthlyStrategy struct {
	RetainDay int `json:"retain_day"` // 优先保留该日期的快照（1-31）
	Retain    int `json:"retain"`
}

// YearlyStrategy 每年快照策略.
type YearlyStrategy struct {
	RetainMonth int `json:"retain_month"` // 优先保留该月的快照（1-12）
	Retain      int `json:"retain"`
}

// classifyPeriod 将快照创建时间归类到时间周期.
type periodBucket struct {
	period    string // "minutely:2006-01-02T15:04" / "hourly:2006-01-02T15" / ...
	timestamp time.Time
}

func classifyPeriod(t time.Time) map[string]periodBucket {
	return map[string]periodBucket{
		"minutely": {period: t.Format("2006-01-02T15:04"), timestamp: t},
		"hourly":   {period: t.Format("2006-01-02T15"), timestamp: t},
		"daily":    {period: t.Format("2006-01-02"), timestamp: t},
		"weekly":   {period: fmt.Sprintf("%d-W%02d", t.Year(), isoWeek(t)), timestamp: t},
		"monthly":  {period: t.Format("2006-01"), timestamp: t},
		"yearly":   {period: t.Format("2006"), timestamp: t},
	}
}

func isoWeek(t time.Time) int {
	_, w := t.ISOWeek()
	return w
}

// PolicyScheduler 根据保留策略自动清理过期快照.
type PolicyScheduler struct {
	mu      sync.Mutex
	logger  *zap.Logger
	manager *Manager
	policy  *RetentionPolicy
	stopCh  chan struct{}
	running bool
}

// NewPolicyScheduler 创建策略调度器.
func NewPolicyScheduler(logger *zap.Logger, manager *Manager, policy *RetentionPolicy) *PolicyScheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PolicyScheduler{
		logger:  logger,
		manager: manager,
		policy:  policy,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动定时清理（每小时检查一次）.
func (ps *PolicyScheduler) Start() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.running {
		return
	}
	ps.running = true

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		// 启动时立即执行一次
		ps.Enforce()

		for {
			select {
			case <-ticker.C:
				if !ps.policy.Enabled {
					continue
				}
				ps.Enforce()
			case <-ps.stopCh:
				return
			}
		}
	}()

	ps.logger.Info("policy scheduler started",
		zap.String("policy_id", ps.policy.ID),
		zap.String("policy_name", ps.policy.Name),
	)
}

// Stop 停止调度器.
func (ps *PolicyScheduler) Stop() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if !ps.running {
		return
	}
	ps.running = false
	close(ps.stopCh)
	ps.logger.Info("policy scheduler stopped", zap.String("policy_id", ps.policy.ID))
}

// Enforce 执行一次策略清理.
func (ps *PolicyScheduler) Enforce() {
	ps.logger.Info("enforcing retention policy", zap.String("policy_id", ps.policy.ID))

	snapshots := ps.manager.ListSnapshots()

	// 过滤该策略作用范围的快照
	var targets []Snapshot
	for _, s := range snapshots {
		if ps.matchScope(s) {
			targets = append(targets, s)
		}
	}

	toDelete := ps.selectForDeletion(targets)

	for _, id := range toDelete {
		if err := ps.manager.DeleteSnapshot(id); err != nil {
			ps.logger.Error("failed to delete expired snapshot",
				zap.String("snapshot_id", id),
				zap.Error(err),
			)
		} else {
			ps.logger.Info("deleted expired snapshot per policy",
				zap.String("snapshot_id", id),
				zap.String("policy_id", ps.policy.ID),
			)
		}
	}
}

// matchScope 判断快照是否在该策略作用范围内.
func (ps *PolicyScheduler) matchScope(s Snapshot) bool {
	switch ps.policy.TargetScope {
	case "global":
		return true
	default:
		// 可根据实际需求扩展 dataset/pool/team 的匹配逻辑
		return false
	}
}

// selectForDeletion 根据保留策略选出应删除的快照 ID
// 核心算法：各粒度独立分组，每组只保留最新的 N 个，其余标记为待删除。
// 高粒度（yearly → minutely）优先保留：先标记低粒度要保留的，这些快照在更高粒度也算保留。
func (ps *PolicyScheduler) selectForDeletion(snapshots []Snapshot) []string {
	if len(snapshots) == 0 {
		return nil
	}

	type granularity struct {
		name   string
		retain int
	}

	granularities := []granularity{
		{name: "yearly", retain: ps.policy.Yearly},
		{name: "monthly", retain: ps.policy.Monthly},
		{name: "weekly", retain: ps.policy.Weekly},
		{name: "daily", retain: ps.policy.Daily},
		{name: "hourly", retain: ps.policy.Hourly},
		{name: "minutely", retain: ps.policy.Minutely},
	}

	// 按创建时间升序排列
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	keepSet := make(map[string]bool)

	for _, g := range granularities {
		if g.retain <= 0 {
			continue
		}

		// 按该粒度分桶
		buckets := make(map[string][]Snapshot)
		for _, s := range snapshots {
			buckets[classifyPeriod(s.CreatedAt)[g.name].period] = append(
				buckets[classifyPeriod(s.CreatedAt)[g.name].period], s,
			)
		}

		// 每个桶按时间排序后取最新 N 个
		for _, bucket := range buckets {
			sort.Slice(bucket, func(i, j int) bool {
				return bucket[i].CreatedAt.After(bucket[j].CreatedAt)
			})
			for i := 0; i < g.retain && i < len(bucket); i++ {
				keepSet[bucket[i].ID] = true
			}
		}
	}

	var toDelete []string
	for _, s := range snapshots {
		if !keepSet[s.ID] {
			toDelete = append(toDelete, s.ID)
		}
	}

	return toDelete
}

// QuotaManager 快照配额管理.
type QuotaManager struct {
	mu         sync.Mutex
	logger     *zap.Logger
	maxPercent float64 // 快照最多占存储空间百分比（0-100）
	totalBytes int64   // 存储总容量
	usedBytes  int64   // 快照已用容量
	manager    *Manager
}

// NewQuotaManager 创建配额管理器.
func NewQuotaManager(logger *zap.Logger, manager *Manager, maxPercent float64, totalBytes int64) *QuotaManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if maxPercent <= 0 {
		maxPercent = 20 // 默认 20%
	}
	if maxPercent > 100 {
		maxPercent = 100
	}
	return &QuotaManager{
		logger:     logger,
		maxPercent: maxPercent,
		totalBytes: totalBytes,
		manager:    manager,
	}
}

// SetQuota 设置配额百分比.
func (qm *QuotaManager) SetQuota(percent float64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if percent <= 0 {
		percent = 20
	}
	if percent > 100 {
		percent = 100
	}
	qm.maxPercent = percent
}

// SetTotalBytes 设置存储总容量.
func (qm *QuotaManager) SetTotalBytes(total int64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.totalBytes = total
}

// CheckQuota 检查创建新快照是否超出配额，返回是否允许.
func (qm *QuotaManager) CheckQuota(additionalBytes int64) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if qm.totalBytes <= 0 {
		return true // 未知总容量时不限制
	}
	projected := qm.usedBytes + additionalBytes
	limit := int64(float64(qm.totalBytes) * qm.maxPercent / 100)
	return projected <= limit
}

// RefreshUsage 刷新快照使用量.
func (qm *QuotaManager) RefreshUsage() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	stats := qm.manager.GetStats()
	if size, ok := stats["total_size_bytes"].(int64); ok {
		qm.usedBytes = size
	}
}

// GetStatus 获取配额状态.
func (qm *QuotaManager) GetStatus() map[string]interface{} {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	var percentUsed float64
	if qm.totalBytes > 0 {
		percentUsed = float64(qm.usedBytes) / float64(qm.totalBytes) * 100
	}

	return map[string]interface{}{
		"total_bytes":       qm.totalBytes,
		"used_bytes":        qm.usedBytes,
		"max_percent":       qm.maxPercent,
		"percent_used":      percentUsed,
		"quota_limit_bytes": int64(float64(qm.totalBytes) * qm.maxPercent / 100),
	}
}

// EnforceQuota 当配额超限时自动删除最老的快照.
func (qm *QuotaManager) EnforceQuota() {
	qm.RefreshUsage()

	qm.mu.Lock()
	totalBytes := qm.totalBytes
	maxPercent := qm.maxPercent
	qm.mu.Unlock()

	if totalBytes <= 0 {
		return
	}

	limit := int64(float64(totalBytes) * maxPercent / 100)

	snapshots := qm.manager.ListSnapshots()
	// 按时间升序（最老的在前）
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	var currentUsage int64
	for _, s := range snapshots {
		currentUsage += s.SizeBytes
	}

	for _, s := range snapshots {
		if currentUsage <= limit {
			break
		}
		if err := qm.manager.DeleteSnapshot(s.ID); err != nil {
			qm.logger.Error("failed to delete snapshot for quota enforcement",
				zap.String("snapshot_id", s.ID),
				zap.Error(err),
			)
			continue
		}
		currentUsage -= s.SizeBytes
		qm.logger.Info("deleted snapshot to enforce quota",
			zap.String("snapshot_id", s.ID),
			zap.Int64("size", s.SizeBytes),
		)
	}
}
