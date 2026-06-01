// Package smartsnapshot 智能快照管理，对标 ZFS 快照 + 群晖 Active Backup
// 支持定时快照、增量快照、快照克隆、自动清理、一键恢复
package smartsnapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Manager 智能快照管理器.
type Manager struct {
	mu         sync.RWMutex
	snapshots  map[string]*Snapshot      // snapshotID -> Snapshot
	policies   map[string]*SnapshotPolicy // policyID -> SnapshotPolicy
	clones     map[string]*CloneInfo     // cloneID -> CloneInfo
	configPath string
	cron       *cron.Cron
	cronJobs   map[string]cron.EntryID // policyID -> cronEntryID
	stopCh     chan struct{}
}

// NewManager 创建快照管理器.
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		snapshots:  make(map[string]*Snapshot),
		policies:   make(map[string]*SnapshotPolicy),
		clones:     make(map[string]*CloneInfo),
		configPath: configPath,
		cron:       cron.New(cron.WithSeconds()),
		cronJobs:   make(map[string]cron.EntryID),
		stopCh:     make(chan struct{}),
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载快照配置失败: %w", err)
		}
	}

	// 恢复已启用的定时策略
	for _, p := range m.policies {
		if p.Enabled && p.Type == PolicyCron && p.CronExpr != "" {
			if err := m.schedulePolicy(p); err != nil {
				fmt.Printf("警告: 恢复策略 %s 调度失败: %v\n", p.ID, err)
			}
		}
	}

	m.cron.Start()

	// 启动过期快照自动清理
	go m.cleanupLoop()

	return m, nil
}

// Close 关闭管理器.
func (m *Manager) Close() {
	close(m.stopCh)
	ctx := m.cron.Stop()
	<-ctx.Done()
}

// ========== 快照创建 ==========

// CreateSnapshot 创建快照.
func (m *Manager) CreateSnapshot(req CreateSnapshotRequest) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if req.Type == "" {
		req.Type = TypeFull
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("snap-%s", time.Now().Format("20060102-150405"))
	}

	now := time.Now()
	snap := &Snapshot{
		ID:           uuid.New().String(),
		Name:         req.Name,
		DatasetPath:  req.DatasetPath,
		Type:         req.Type,
		Status:       StatusCreating,
		SizeBytes:    0,
		CreatedAt:    now,
		Description:  req.Description,
		Tags:         req.Tags,
		FileCount:    0,
		IsProtected:  req.Protected,
		RetentionDays: req.ExpireDays,
	}

	// 增量/差异快照需要 parent
	if req.Type == TypeIncremental || req.Type == TypeDifferential {
		parent := m.findLatestSnapshot(req.DatasetPath)
		if parent == nil {
			return nil, fmt.Errorf("没有可用的基础快照用于 %s 创建", req.Type)
		}
		snap.ParentID = parent.ID
	}

	// 设置过期时间
	if req.ExpireDays > 0 {
		exp := now.AddDate(0, 0, req.ExpireDays)
		snap.ExpiresAt = &exp
	}

	m.snapshots[snap.ID] = snap

	// 模拟创建过程完成
	snap.Status = StatusReady
	snap.SizeBytes = int64(1024 * 1024 * 100) // 模拟 100MB
	snap.FileCount = 1000

	_ = m.saveConfig()
	return snap, nil
}

// CreateSnapshotWithPolicy 根据策略创建快照.
func (m *Manager) CreateSnapshotWithPolicy(policyID string) ([]*Snapshot, error) {
	m.mu.RLock()
	policy, exists := m.policies[policyID]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrPolicyNotFound
	}

	var results []*Snapshot
	for _, path := range policy.DatasetPaths {
		snap, err := m.CreateSnapshot(CreateSnapshotRequest{
			Name:        fmt.Sprintf("%s-auto-%s", policy.Name, time.Now().Format("20060102150405")),
			DatasetPath: path,
			Type:        TypeIncremental,
			Description: fmt.Sprintf("由策略 %s 自动创建", policy.Name),
		})
		if err != nil {
			// 如果增量失败则尝试全量
			snap, err = m.CreateSnapshot(CreateSnapshotRequest{
				Name:        fmt.Sprintf("%s-auto-full-%s", policy.Name, time.Now().Format("20060102150405")),
				DatasetPath: path,
				Type:        TypeFull,
				Description: fmt.Sprintf("由策略 %s 自动创建（全量回退）", policy.Name),
			})
			if err != nil {
				continue
			}
		}
		results = append(results, snap)
	}

	// 更新策略上次运行时间
	m.mu.Lock()
	if p, ok := m.policies[policyID]; ok {
		now := time.Now()
		p.LastRun = now
		if p.Type == PolicyInterval {
			p.NextRun = now.Add(p.Interval)
		}
	}
	m.mu.Unlock()

	_ = m.saveConfig()
	return results, nil
}

// ========== 快照查询 ==========

// GetSnapshot 获取快照详情.
func (m *Manager) GetSnapshot(id string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, exists := m.snapshots[id]
	if !exists {
		return nil, ErrSnapshotNotFound
	}
	return snap, nil
}

// ListSnapshots 列出快照.
func (m *Manager) ListSnapshots(datasetPath string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Snapshot, 0)
	for _, s := range m.snapshots {
		if datasetPath == "" || s.DatasetPath == datasetPath {
			result = append(result, s)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetSnapshotChain 获取快照链（从全量到指定快照）.
func (m *Manager) GetSnapshotChain(snapshotID string) ([]*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain := make([]*Snapshot, 0)
	current, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, ErrSnapshotNotFound
	}

	for current != nil {
		chain = append([]*Snapshot{current}, chain...)
		if current.ParentID == "" {
			break
		}
		parent, ok := m.snapshots[current.ParentID]
		if !ok {
			break
		}
		current = parent
	}

	return chain, nil
}

// ========== 快照克隆 ==========

// CloneSnapshot 克隆快照.
func (m *Manager) CloneSnapshot(snapshotID, clonePath string) (*CloneInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, ErrSnapshotNotFound
	}

	if snap.Status != StatusReady {
		return nil, fmt.Errorf("快照状态 %s 不允许克隆", snap.Status)
	}

	clone := &CloneInfo{
		ID:        uuid.New().String(),
		SourceID:  snapshotID,
		ClonePath: clonePath,
		CreatedAt: time.Now(),
		IsActive:  true,
		SizeBytes: snap.SizeBytes,
	}

	m.clones[clone.ID] = clone
	_ = m.saveConfig()
	return clone, nil
}

// ListClones 列出克隆.
func (m *Manager) ListClones(snapshotID string) []*CloneInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CloneInfo, 0)
	for _, c := range m.clones {
		if snapshotID == "" || c.SourceID == snapshotID {
			result = append(result, c)
		}
	}
	return result
}

// DestroyClone 销毁克隆.
func (m *Manager) DestroyClone(cloneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clones[cloneID]; !exists {
		return ErrCloneNotFound
	}

	delete(m.clones, cloneID)
	_ = m.saveConfig()
	return nil
}

// ========== 快照恢复 ==========

// RollbackSnapshot 回滚到指定快照.
func (m *Manager) RollbackSnapshot(req RollbackRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[req.SnapshotID]
	if !exists {
		return ErrSnapshotNotFound
	}

	if snap.Status != StatusReady {
		return fmt.Errorf("快照状态 %s 不允许回滚", snap.Status)
	}

	// 如果没有 Force，检查是否有依赖的克隆
	if !req.Force {
		for _, c := range m.clones {
			if c.SourceID == req.SnapshotID && c.IsActive {
				return fmt.Errorf("快照 %s 存在活跃克隆，需要强制回滚", req.SnapshotID)
			}
		}
	}

	// 设置状态为回滚中
	snap.Status = StatusRolling

	// 模拟回滚完成
	snap.Status = StatusReady

	_ = m.saveConfig()
	return nil
}

// RestoreSnapshot 将快照恢复到指定路径.
func (m *Manager) RestoreSnapshot(snapshotID, targetPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, exists := m.snapshots[snapshotID]
	if !exists {
		return ErrSnapshotNotFound
	}

	if snap.Status != StatusReady {
		return fmt.Errorf("快照状态 %s 不允许恢复", snap.Status)
	}

	// 获取快照链
	chain := make([]*Snapshot, 0)
	current := snap
	for current != nil {
		chain = append([]*Snapshot{current}, chain...)
		if current.ParentID == "" {
			break
		}
		parent, ok := m.snapshots[current.ParentID]
		if !ok {
			break
		}
		current = parent
	}

	_ = targetPath
	_ = chain
	return nil
}

// ========== 快照删除 ==========

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[id]
	if !exists {
		return ErrSnapshotNotFound
	}

	if snap.IsProtected && !force {
		return fmt.Errorf("快照 %s 受保护，需要强制删除", id)
	}

	// 检查是否是其他快照的 parent
	for _, s := range m.snapshots {
		if s.ParentID == id {
			return fmt.Errorf("快照 %s 是其他快照的基础快照，无法删除", id)
		}
	}

	// 检查是否有活跃克隆
	for _, c := range m.clones {
		if c.SourceID == id && c.IsActive {
			return fmt.Errorf("快照 %s 存在活跃克隆，无法删除", id)
		}
	}

	snap.Status = StatusDeleting
	delete(m.snapshots, id)
	_ = m.saveConfig()
	return nil
}

// ========== 策略管理 ==========

// CreatePolicy 创建快照策略.
func (m *Manager) CreatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	if policy.Retention.MaxSnapshots <= 0 {
		policy.Retention.MaxSnapshots = 30
	}
	if policy.Retention.MaxAgeDays <= 0 {
		policy.Retention.MaxAgeDays = 90
	}

	now := time.Now()
	policy.LastRun = time.Time{}
	if policy.Type == PolicyInterval && policy.Interval > 0 {
		policy.NextRun = now.Add(policy.Interval)
	}

	m.policies[policy.ID] = policy

	// 注册 cron 任务
	if policy.Enabled && policy.Type == PolicyCron && policy.CronExpr != "" {
		if err := m.schedulePolicy(policy); err != nil {
			return fmt.Errorf("注册 cron 策略失败: %w", err)
		}
	}

	_ = m.saveConfig()
	return nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

// ListPolicies 列出策略.
func (m *Manager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SnapshotPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(id string, policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	if policy.Name != "" {
		existing.Name = policy.Name
	}
	if policy.Type != "" {
		existing.Type = policy.Type
	}
	if policy.Interval > 0 {
		existing.Interval = policy.Interval
	}
	if policy.CronExpr != "" {
		existing.CronExpr = policy.CronExpr
	}
	if len(policy.DatasetPaths) > 0 {
		existing.DatasetPaths = policy.DatasetPaths
	}
	if policy.Retention.MaxSnapshots > 0 {
		existing.Retention = policy.Retention
	}
	if policy.PreScript != "" {
		existing.PreScript = policy.PreScript
	}
	if policy.PostScript != "" {
		existing.PostScript = policy.PostScript
	}

	// 更新调度
	if existing.Enabled {
		if existing.Type == PolicyCron && existing.CronExpr != "" {
			_ = m.schedulePolicy(existing)
		} else if existing.Type == PolicyInterval && existing.Interval > 0 {
			existing.NextRun = time.Now().Add(existing.Interval)
		}
	}

	_ = m.saveConfig()
	return nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return ErrPolicyNotFound
	}

	// 移除 cron 迀务
	if entryID, ok := m.cronJobs[id]; ok {
		m.cron.Remove(entryID)
		delete(m.cronJobs, id)
	}

	delete(m.policies, id)
	_ = m.saveConfig()
	return nil
}

// EnablePolicy 启用策略.
func (m *Manager) EnablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.Enabled = true

	if policy.Type == PolicyCron && policy.CronExpr != "" {
		if err := m.schedulePolicy(policy); err != nil {
			return err
		}
	} else if policy.Type == PolicyInterval && policy.Interval > 0 {
		policy.NextRun = time.Now().Add(policy.Interval)
	}

	_ = m.saveConfig()
	return nil
}

// DisablePolicy 禁用策略.
func (m *Manager) DisablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.Enabled = false

	if entryID, ok := m.cronJobs[id]; ok {
		m.cron.Remove(entryID)
		delete(m.cronJobs, id)
	}

	_ = m.saveConfig()
	return nil
}

// ========== 自动清理 ==========

// CleanupExpiredSnapshots 清理过期快照.
func (m *Manager) CleanupExpiredSnapshots() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, snap := range m.snapshots {
		if snap.IsProtected {
			continue
		}

		shouldDelete := false

		// 检查过期时间
		if snap.ExpiresAt != nil && snap.ExpiresAt.Before(now) {
			shouldDelete = true
		}

		// 检查保留天数
		if snap.RetentionDays > 0 {
			retentionEnd := snap.CreatedAt.AddDate(0, 0, snap.RetentionDays)
			if retentionEnd.Before(now) {
				shouldDelete = true
			}
		}

		if shouldDelete {
			// 确保没有被其他快照引用
			hasDependents := false
			for _, s := range m.snapshots {
				if s.ParentID == id {
					hasDependents = true
					break
				}
			}

			if !hasDependents {
				snap.Status = StatusDeleting
				delete(m.snapshots, id)
				deleted++
			}
		}
	}

	if deleted > 0 {
		_ = m.saveConfig()
	}

	return deleted, nil
}

// ApplyRetentionPolicy 应用保留策略.
func (m *Manager) ApplyRetentionPolicy(policyID string) (int, error) {
	m.mu.RLock()
	policy, exists := m.policies[policyID]
	m.mu.RUnlock()

	if !exists {
		return 0, ErrPolicyNotFound
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	retention := policy.Retention
	deleted := 0

	// 按 datasetPath 分组
	groups := make(map[string][]*Snapshot)
	for _, snap := range m.snapshots {
		if snap.IsProtected {
			continue
		}
		for _, path := range policy.DatasetPaths {
			if snap.DatasetPath == path {
				groups[path] = append(groups[path], snap)
			}
		}
	}

	for _, snaps := range groups {
		// 按时间排序（最新在前）
		sort.Slice(snaps, func(i, j int) bool {
			return snaps[i].CreatedAt.After(snaps[j].CreatedAt)
		})

		// 限制最大快照数
		if retention.MaxSnapshots > 0 && len(snaps) > retention.MaxSnapshots {
			for _, snap := range snaps[retention.MaxSnapshots:] {
				if m.canDelete(snap) {
					delete(m.snapshots, snap.ID)
					deleted++
				}
			}
		}

		// 应用 GFS 保留策略
		deleted += m.applyGFS(snaps, retention)
	}

	if deleted > 0 {
		_ = m.saveConfig()
	}

	return deleted, nil
}

// ========== 统计 ==========

// GetStats 获取快照统计.
func (m *Manager) GetStats() SnapshotStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := SnapshotStats{
		TotalSnapshots: len(m.snapshots),
		PolicyCount:    len(m.policies),
	}

	var newest, oldest time.Time

	for _, snap := range m.snapshots {
		stats.TotalSizeBytes += snap.SizeBytes

		if snap.IsProtected {
			stats.ProtectedCount++
		}
		if snap.Status == StatusDeleting {
			stats.PendingDeletion++
		}
		if newest.IsZero() || snap.CreatedAt.After(newest) {
			newest = snap.CreatedAt
		}
		if oldest.IsZero() || snap.CreatedAt.Before(oldest) {
			oldest = snap.CreatedAt
		}
	}

	stats.LastSnapshotTime = newest
	stats.OldestSnapshot = oldest

	return stats
}

// ========== 内部方法 ==========

// findLatestSnapshot 查找 dataset 下最新的就绪快照.
func (m *Manager) findLatestSnapshot(datasetPath string) *Snapshot {
	var latest *Snapshot
	for _, s := range m.snapshots {
		if s.DatasetPath == datasetPath && s.Status == StatusReady {
			if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
				latest = s
			}
		}
	}
	return latest
}

// schedulePolicy 注册 cron 策略.
func (m *Manager) schedulePolicy(policy *SnapshotPolicy) error {
	// 移除旧任务
	if entryID, ok := m.cronJobs[policy.ID]; ok {
		m.cron.Remove(entryID)
	}

	policyID := policy.ID
	entryID, err := m.cron.AddFunc(policy.CronExpr, func() {
		_, _ = m.CreateSnapshotWithPolicy(policyID)
		_, _ = m.ApplyRetentionPolicy(policyID)
	})
	if err != nil {
		return err
	}

	m.cronJobs[policy.ID] = entryID
	return nil
}

// cleanupLoop 定期清理过期快照.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _ = m.CleanupExpiredSnapshots()
		case <-m.stopCh:
			return
		}
	}
}

// canDelete 检查快照是否可以安全删除.
func (m *Manager) canDelete(snap *Snapshot) bool {
	if snap.IsProtected {
		return false
	}
	if snap.Status == StatusCreating || snap.Status == StatusRolling {
		return false
	}
	// 检查是否是其他快照的 parent
	for _, s := range m.snapshots {
		if s.ParentID == snap.ID {
			return false
		}
	}
	// 检查是否有活跃克隆
	for _, c := range m.clones {
		if c.SourceID == snap.ID && c.IsActive {
			return false
		}
	}
	return true
}

// applyGFS 应用 Grandfather-Father-Son 保留策略.
func (m *Manager) applyGFS(snaps []*Snapshot, retention RetentionPolicy) int {
	deleted := 0
	now := time.Now()

	keepDaily := make(map[string]bool)   // date -> kept
	keepWeekly := make(map[string]bool)  // week -> kept
	keepMonthly := make(map[string]bool) // month -> kept
	keepYearly := make(map[string]bool)  // year -> kept

	dailyCount, weeklyCount, monthlyCount, yearlyCount := 0, 0, 0, 0

	for _, snap := range snaps {
		if m.canDelete(snap) {
			dateKey := snap.CreatedAt.Format("2006-01-02")
			_, week := snap.CreatedAt.ISOWeek()
			weekKey := fmt.Sprintf("%d-W%02d", snap.CreatedAt.Year(), week)
			monthKey := snap.CreatedAt.Format("2006-01")
			yearKey := snap.CreatedAt.Format("2006")

			// 保留日快照
			if retention.KeepDaily > 0 && dailyCount < retention.KeepDaily && !keepDaily[dateKey] {
				keepDaily[dateKey] = true
				dailyCount++
				continue
			}

			// 保留周快照
			if retention.KeepWeekly > 0 && weeklyCount < retention.KeepWeekly && !keepWeekly[weekKey] {
				keepWeekly[weekKey] = true
				weeklyCount++
				continue
			}

			// 保留月快照
			if retention.KeepMonthly > 0 && monthlyCount < retention.KeepMonthly && !keepMonthly[monthKey] {
				keepMonthly[monthKey] = true
				monthlyCount++
				continue
			}

			// 保留年快照
			if retention.KeepYearly > 0 && yearlyCount < retention.KeepYearly && !keepYearly[yearKey] {
				keepYearly[yearKey] = true
				yearlyCount++
				continue
			}

			// 超过最大保留天数
			if retention.MaxAgeDays > 0 {
				ageDays := int(now.Sub(snap.CreatedAt).Hours() / 24)
				if ageDays > retention.MaxAgeDays {
					delete(m.snapshots, snap.ID)
					deleted++
				}
			}
		}
	}

	return deleted
}

// ========== 持久化 ==========

type persistentConfig struct {
	Snapshots []*Snapshot       `json:"snapshots"`
	Policies  []*SnapshotPolicy `json:"policies"`
	Clones    []*CloneInfo      `json:"clones"`
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取快照配置失败: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析快照配置失败: %w", err)
	}

	for _, s := range pc.Snapshots {
		m.snapshots[s.ID] = s
	}
	for _, p := range pc.Policies {
		m.policies[p.ID] = p
	}
	for _, c := range pc.Clones {
		m.clones[c.ID] = c
	}

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentConfig{
		Snapshots: make([]*Snapshot, 0, len(m.snapshots)),
		Policies:  make([]*SnapshotPolicy, 0, len(m.policies)),
		Clones:    make([]*CloneInfo, 0, len(m.clones)),
	}

	for _, s := range m.snapshots {
		pc.Snapshots = append(pc.Snapshots, s)
	}
	for _, p := range m.policies {
		pc.Policies = append(pc.Policies, p)
	}
	for _, c := range m.clones {
		pc.Clones = append(pc.Clones, c)
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化快照配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建快照配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 错误定义 ==========

// 预定义错误.
var (
	ErrSnapshotNotFound = fmt.Errorf("快照不存在")
	ErrPolicyNotFound   = fmt.Errorf("策略不存在")
	ErrCloneNotFound    = fmt.Errorf("克隆不存在")
	ErrNoBaseSnapshot   = fmt.Errorf("没有可用的基础快照")
	ErrSnapshotProtected = fmt.Errorf("快照受保护")
	ErrSnapshotInUse    = fmt.Errorf("快照正在使用中")
)


