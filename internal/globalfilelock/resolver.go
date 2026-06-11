// Package globalfilelock 冲突解决逻辑
// 提供多种冲突检测和解决策略，包括最后写入胜出、手动解决、自动合并等
package globalfilelock

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ConflictResolver 冲突解决器
// 负责检测文件锁冲突并执行相应的解决策略
type ConflictResolver struct {
	mu      sync.RWMutex
	manager *LockManager

	// 合并规则配置
	mergeRules map[string]*MergeRule

	// 解决历史
	resolutionHistory []*ResolutionRecord
}

// MergeRule 合并规则
type MergeRule struct {
	// 文件路径模式（支持通配符）
	FilePathPattern string `json:"file_path_pattern"`
	// 合并策略
	Strategy ConflictResolution `json:"strategy"`
	// 是否允许自动合并
	AllowAutoMerge bool `json:"allow_auto_merge"`
	// 合并优先级（站点ID -> 优先级，数值越小优先级越高）
	SitePriority map[string]int `json:"site_priority"`
	// 用户优先级（用户ID -> 优先级）
	UserPriority map[string]int `json:"user_priority"`
}

// ResolutionRecord 解决记录
type ResolutionRecord struct {
	// 记录ID
	ID string `json:"id"`
	// 冲突ID
	ConflictID string `json:"conflict_id"`
	// 文件路径
	FilePath string `json:"file_path"`
	// 解决策略
	Resolution ConflictResolution `json:"resolution"`
	// 解决结果
	Result string `json:"result"`
	// 解决者ID
	ResolvedBy string `json:"resolved_by"`
	// 解决时间
	ResolvedAt time.Time `json:"resolved_at"`
	// 保留的锁ID
	PreferredLockID string `json:"preferred_lock_id,omitempty"`
	// 详情
	Detail string `json:"detail"`
}

// NewConflictResolver 创建冲突解决器
func NewConflictResolver(manager *LockManager) *ConflictResolver {
	resolver := &ConflictResolver{
		manager:          manager,
		mergeRules:       make(map[string]*MergeRule),
		resolutionHistory: make([]*ResolutionRecord, 0),
	}

	// 初始化默认合并规则
	resolver.initDefaultRules()

	return resolver
}

// initDefaultRules 初始化默认合并规则
func (cr *ConflictResolver) initDefaultRules() {
	// 默认规则：最后写入胜出
	cr.mergeRules["*"] = &MergeRule{
		FilePathPattern: "*",
		Strategy:        ResolutionLastWriteWins,
		AllowAutoMerge:  false,
		SitePriority:    make(map[string]int),
		UserPriority:    make(map[string]int),
	}
}

// ResolveConflict 解决冲突
func (cr *ConflictResolver) ResolveConflict(req *ResolveConflictRequest) (*ResolutionRecord, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// 查找冲突记录
	var conflict *LockConflict
	cr.manager.mu.RLock()
	for _, c := range cr.manager.conflicts {
		if c.ID == req.ConflictID {
			conflict = c
			break
		}
	}
	cr.manager.mu.RUnlock()

	if conflict == nil {
		return nil, fmt.Errorf("冲突不存在: %s", req.ConflictID)
	}

	if conflict.Resolved {
		return nil, fmt.Errorf("冲突已解决: %s", req.ConflictID)
	}

	// 根据解决策略执行
	var record *ResolutionRecord
	var err error

	switch req.Resolution {
	case ResolutionLastWriteWins:
		record, err = cr.resolveLastWriteWins(conflict, req)
	case ResolutionManual:
		record, err = cr.resolveManual(conflict, req)
	case ResolutionAutoMerge:
		record, err = cr.resolveAutoMerge(conflict, req)
	default:
		return nil, fmt.Errorf("不支持的解决策略: %s", req.Resolution)
	}

	if err != nil {
		return nil, err
	}

	// 标记冲突已解决
	cr.manager.mu.Lock()
	conflict.Resolved = true
	conflict.ResolvedAt = &record.ResolvedAt
	conflict.ResolvedBy = req.ResolvedBy
	conflict.Resolution = req.Resolution
	conflict.ResolutionDetail = req.Detail
	cr.manager.mu.Unlock()

	// 记录解决历史
	cr.resolutionHistory = append(cr.resolutionHistory, record)

	return record, nil
}

// resolveLastWriteWins 最后写入胜出策略
// 保留最后获取的锁，释放其他冲突锁
func (cr *ConflictResolver) resolveLastWriteWins(conflict *LockConflict, req *ResolveConflictRequest) (*ResolutionRecord, error) {
	if len(conflict.ConflictingLocks) == 0 {
		return nil, fmt.Errorf("没有冲突的锁")
	}

	// 按获取时间排序，选择最后获取的锁
	sortedLocks := make([]*FileLock, len(conflict.ConflictingLocks))
	copy(sortedLocks, conflict.ConflictingLocks)
	sort.Slice(sortedLocks, func(i, j int) bool {
		return sortedLocks[i].AcquiredAt.After(sortedLocks[j].AcquiredAt)
	})

	winner := sortedLocks[0]

	// 释放其他锁
	for _, lock := range sortedLocks[1:] {
		if lock.Status == LockStatusActive {
			cr.manager.mu.Lock()
			lock.Status = LockStatusForceReleased
			now := time.Now()
			lock.ReleasedAt = &now
			lock.ReleasedBy = "conflict_resolver"
			lock.UpdatedAt = now
			cr.manager.removeFromIndexes(lock)
			cr.manager.mu.Unlock()
		}
	}

	return &ResolutionRecord{
		ID:              generateID(),
		ConflictID:      conflict.ID,
		FilePath:        conflict.FilePath,
		Resolution:      ResolutionLastWriteWins,
		Result:          "resolved",
		ResolvedBy:      req.ResolvedBy,
		ResolvedAt:      time.Now(),
		PreferredLockID: winner.ID,
		Detail:          fmt.Sprintf("最后写入胜出，保留锁 %s (持有者: %s)", winner.ID, winner.HolderName),
	}, nil
}

// resolveManual 手动解决策略
// 由管理员选择保留哪个锁
func (cr *ConflictResolver) resolveManual(conflict *LockConflict, req *ResolveConflictRequest) (*ResolutionRecord, error) {
	if req.PreferredLockID == "" {
		return nil, fmt.Errorf("手动解决必须指定保留的锁ID")
	}

	// 验证指定的锁是否在冲突列表中
	var preferredLock *FileLock
	for _, lock := range conflict.ConflictingLocks {
		if lock.ID == req.PreferredLockID {
			preferredLock = lock
			break
		}
	}

	if preferredLock == nil {
		return nil, fmt.Errorf("指定的锁 %s 不在冲突列表中", req.PreferredLockID)
	}

	// 释放其他锁
	for _, lock := range conflict.ConflictingLocks {
		if lock.ID != req.PreferredLockID && lock.Status == LockStatusActive {
			cr.manager.mu.Lock()
			lock.Status = LockStatusForceReleased
			now := time.Now()
			lock.ReleasedAt = &now
			lock.ReleasedBy = req.ResolvedBy
			lock.UpdatedAt = now
			cr.manager.removeFromIndexes(lock)
			cr.manager.mu.Unlock()
		}
	}

	return &ResolutionRecord{
		ID:              generateID(),
		ConflictID:      conflict.ID,
		FilePath:        conflict.FilePath,
		Resolution:      ResolutionManual,
		Result:          "resolved",
		ResolvedBy:      req.ResolvedBy,
		ResolvedAt:      time.Now(),
		PreferredLockID: req.PreferredLockID,
		Detail:          fmt.Sprintf("手动解决，保留锁 %s (持有者: %s)", preferredLock.ID, preferredLock.HolderName),
	}, nil
}

// resolveAutoMerge 自动合并策略
// 尝试自动合并冲突，如果无法合并则回退到手动解决
func (cr *ConflictResolver) resolveAutoMerge(conflict *LockConflict, req *ResolveConflictRequest) (*ResolutionRecord, error) {
	if len(conflict.ConflictingLocks) < 2 {
		return nil, fmt.Errorf("自动合并需要至少2个冲突的锁")
	}

	// 检查是否可以自动合并
	// 规则：只有读锁可以合并，写锁必须手动解决
	allRead := true
	for _, lock := range conflict.ConflictingLocks {
		if lock.LockType == LockTypeWrite {
			allRead = false
			break
		}
	}

	if allRead {
		// 所有都是读锁，可以合并（允许同时存在）
		return &ResolutionRecord{
			ID:         generateID(),
			ConflictID: conflict.ID,
			FilePath:   conflict.FilePath,
			Resolution: ResolutionAutoMerge,
			Result:     "merged",
			ResolvedBy: req.ResolvedBy,
			ResolvedAt: time.Now(),
			Detail:     fmt.Sprintf("自动合并成功，保留所有 %d 个读锁", len(conflict.ConflictingLocks)),
		}, nil
	}

	// 存在写锁，尝试基于优先级选择
	selectedLock := cr.selectByPriority(conflict.ConflictingLocks)

	// 释放其他锁
	for _, lock := range conflict.ConflictingLocks {
		if lock.ID != selectedLock.ID && lock.Status == LockStatusActive {
			cr.manager.mu.Lock()
			lock.Status = LockStatusForceReleased
			now := time.Now()
			lock.ReleasedAt = &now
			lock.ReleasedBy = "conflict_resolver"
			lock.UpdatedAt = now
			cr.manager.removeFromIndexes(lock)
			cr.manager.mu.Unlock()
		}
	}

	return &ResolutionRecord{
		ID:              generateID(),
		ConflictID:      conflict.ID,
		FilePath:        conflict.FilePath,
		Resolution:      ResolutionAutoMerge,
		Result:          "resolved",
		ResolvedBy:      req.ResolvedBy,
		ResolvedAt:      time.Now(),
		PreferredLockID: selectedLock.ID,
		Detail:          fmt.Sprintf("自动合并（基于优先级），保留锁 %s (持有者: %s)", selectedLock.ID, selectedLock.HolderName),
	}, nil
}

// selectByPriority 基于优先级选择锁
// 优先级：写锁 > 读锁；同类型按用户/站点优先级排序
func (cr *ConflictResolver) selectByPriority(locks []*FileLock) *FileLock {
	if len(locks) == 0 {
		return nil
	}

	// 复制锁列表
	sorted := make([]*FileLock, len(locks))
	copy(sorted, locks)

	// 排序规则：
	// 1. 写锁优先于读锁
	// 2. 同类型按获取时间（最新的优先）
	sort.Slice(sorted, func(i, j int) bool {
		// 写锁优先
		if sorted[i].LockType != sorted[j].LockType {
			return sorted[i].LockType == LockTypeWrite
		}

		// 同类型按获取时间（最新优先）
		return sorted[i].AcquiredAt.After(sorted[j].AcquiredAt)
	})

	return sorted[0]
}

// DetectConflict 检测文件冲突
// 检查文件是否存在锁冲突，并返回冲突信息
func (cr *ConflictResolver) DetectConflict(filePath string) *LockConflict {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	// 获取文件的所有活跃锁
	locks := cr.manager.GetFileLocks(filePath)
	if len(locks) <= 1 {
		return nil // 没有冲突
	}

	// 检查是否存在冲突
	hasWrite := false
	hasRead := false
	for _, lock := range locks {
		if lock.LockType == LockTypeWrite {
			hasWrite = true
		} else {
			hasRead = true
		}
	}

	// 写锁与任何锁冲突，多个写锁也冲突
	if hasWrite && (hasRead || countWriteLocks(locks) > 1) {
		return &LockConflict{
			ID:               generateID(),
			FilePath:         filePath,
			ConflictingLocks: locks,
			DetectedAt:       time.Now(),
			Resolved:         false,
			Resolution:       cr.manager.GetPolicy().DefaultResolution,
		}
	}

	// 检查范围冲突
	for i := 0; i < len(locks); i++ {
		for j := i + 1; j < len(locks); j++ {
			if cr.isLockConflict(locks[i], locks[j]) {
				return &LockConflict{
					ID:               generateID(),
					FilePath:         filePath,
					ConflictingLocks: locks,
					DetectedAt:       time.Now(),
					Resolved:         false,
					Resolution:       cr.manager.GetPolicy().DefaultResolution,
				}
			}
		}
	}

	return nil
}

// isLockConflict 检查两个锁是否冲突
func (cr *ConflictResolver) isLockConflict(lock1, lock2 *FileLock) bool {
	// 同一持有者的锁不冲突（升级/降级场景）
	if lock1.HolderID == lock2.HolderID {
		return false
	}

	// 写锁与任何锁冲突
	if lock1.LockType == LockTypeWrite || lock2.LockType == LockTypeWrite {
		return true
	}

	// 检查范围冲突
	if lock1.LockScope == LockScopeGlobal || lock2.LockScope == LockScopeGlobal {
		return true
	}

	if lock1.LockScope == LockScopeSite && lock2.LockScope == LockScopeSite {
		return lock1.SiteID == lock2.SiteID
	}

	return false
}

// AddMergeRule 添加合并规则
func (cr *ConflictResolver) AddMergeRule(pattern string, rule *MergeRule) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.mergeRules[pattern] = rule
}

// GetMergeRule 获取合并规则
func (cr *ConflictResolver) GetMergeRule(filePath string) *MergeRule {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	// 查找匹配的规则（精确匹配优先）
	if rule, ok := cr.mergeRules[filePath]; ok {
		return rule
	}

	// 通配符匹配
	if rule, ok := cr.mergeRules["*"]; ok {
		return rule
	}

	return nil
}

// GetResolutionHistory 获取解决历史
func (cr *ConflictResolver) GetResolutionHistory(limit int) []*ResolutionRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if limit <= 0 || limit > len(cr.resolutionHistory) {
		limit = len(cr.resolutionHistory)
	}

	start := len(cr.resolutionHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ResolutionRecord, limit)
	copy(result, cr.resolutionHistory[start:])
	return result
}

// countWriteLocks 统计写锁数量
func countWriteLocks(locks []*FileLock) int {
	count := 0
	for _, lock := range locks {
		if lock.LockType == LockTypeWrite {
			count++
		}
	}
	return count
}

// ============================================================
// SyncCoordinator 跨站点锁同步协调器
// ============================================================

// SyncCoordinator 跨站点锁同步协调器
// 负责在多个站点之间同步锁状态，确保全局一致性
type SyncCoordinator struct {
	mu      sync.RWMutex
	manager *LockManager

	// 本地站点ID
	localSiteID string

	// 同步状态
	syncStates map[string]*SyncState
}

// SyncState 同步状态
type SyncState struct {
	// 站点ID
	SiteID string `json:"site_id"`
	// 最后同步时间
	LastSyncAt time.Time `json:"last_sync_at"`
	// 同步状态
	Status string `json:"status"` // "synced", "syncing", "error"
	// 错误信息
	Error string `json:"error,omitempty"`
	// 待同步消息数
	PendingMessages int `json:"pending_messages"`
}

// NewSyncCoordinator 创建同步协调器
func NewSyncCoordinator(manager *LockManager, localSiteID string) *SyncCoordinator {
	return &SyncCoordinator{
		manager:     manager,
		localSiteID: localSiteID,
		syncStates:  make(map[string]*SyncState),
	}
}

// SyncLock 同步锁到其他站点
func (sc *SyncCoordinator) SyncLock(lock *FileLock, targetSiteID string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// 更新同步状态
	state := sc.getOrCreateSyncState(targetSiteID)
	state.Status = "syncing"

	// 发送同步消息
	msg := &SyncMessage{
		ID:           generateID(),
		Type:         "lock_acquired",
		SourceSiteID: sc.localSiteID,
		TargetSiteID: targetSiteID,
		Lock:         lock,
		Timestamp:    time.Now(),
	}

	// 通过管理器的同步通道发送
	select {
	case sc.manager.syncChan <- msg:
		state.Status = "synced"
		state.LastSyncAt = time.Now()
		state.PendingMessages = 0
	default:
		state.Status = "error"
		state.Error = "同步通道已满"
		state.PendingMessages++
		return fmt.Errorf("同步通道已满")
	}

	return nil
}

// SyncRelease 同步锁释放到其他站点
func (sc *SyncCoordinator) SyncRelease(lock *FileLock, targetSiteID string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	state := sc.getOrCreateSyncState(targetSiteID)
	state.Status = "syncing"

	msg := &SyncMessage{
		ID:           generateID(),
		Type:         "lock_released",
		SourceSiteID: sc.localSiteID,
		TargetSiteID: targetSiteID,
		Lock:         lock,
		Timestamp:    time.Now(),
	}

	select {
	case sc.manager.syncChan <- msg:
		state.Status = "synced"
		state.LastSyncAt = time.Now()
	default:
		state.Status = "error"
		state.Error = "同步通道已满"
		return fmt.Errorf("同步通道已满")
	}

	return nil
}

// BroadcastLock 广播锁到所有站点
func (sc *SyncCoordinator) BroadcastLock(lock *FileLock) error {
	sites := sc.manager.GetSites()

	for _, site := range sites {
		if site.ID == sc.localSiteID {
			continue // 跳过本地站点
		}

		if err := sc.SyncLock(lock, site.ID); err != nil {
			// 记录错误但继续同步其他站点
			continue
		}
	}

	return nil
}

// GetSyncStates 获取所有同步状态
func (sc *SyncCoordinator) GetSyncStates() map[string]*SyncState {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[string]*SyncState)
	for k, v := range sc.syncStates {
		state := *v
		result[k] = &state
	}
	return result
}

// getOrCreateSyncState 获取或创建同步状态
func (sc *SyncCoordinator) getOrCreateSyncState(siteID string) *SyncState {
	state, ok := sc.syncStates[siteID]
	if !ok {
		state = &SyncState{
			SiteID:   siteID,
			Status:   "pending",
		}
		sc.syncStates[siteID] = state
	}
	return state
}
