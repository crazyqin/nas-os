// Package globalfilelock 核心管理器实现
package globalfilelock

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// LockManager 分布式文件锁管理器
// 负责全局文件锁的获取、释放、续期、升级/降级等核心操作
type LockManager struct {
	mu sync.RWMutex

	// 策略配置
	policy *LockPolicy

	// 锁存储：锁ID -> FileLock
	locks map[string]*FileLock

	// 文件锁索引：文件路径 -> 锁ID列表
	fileIndex map[string][]string

	// 用户锁索引：用户ID -> 锁ID列表
	userIndex map[string][]string

	// 站点信息
	sites map[string]*SiteInfo

	// 冲突记录
	conflicts []*LockConflict

	// 历史记录
	history []*HistoryEntry

	// 等待队列
	waitingQueue []*WaitEntry

	// 同步通道
	syncChan chan *SyncMessage

	// 停止信号
	stopChan chan struct{}

	// 是否运行中
	running bool
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	ID        string    `json:"id"`
	LockID    string    `json:"lock_id"`
	FilePath  string    `json:"file_path"`
	Action    string    `json:"action"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// WaitEntry 等待队列条目
type WaitEntry struct {
	FilePath  string    `json:"file_path"`
	HolderID  string    `json:"holder_id"`
	LockType  LockType  `json:"lock_type"`
	LockScope LockScope `json:"lock_scope"`
	SiteID    string    `json:"site_id"`
	RequestedAt time.Time `json:"requested_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// NewLockManager 创建分布式文件锁管理器
func NewLockManager(policy *LockPolicy) *LockManager {
	if policy == nil {
		policy = DefaultLockPolicy()
	}

	return &LockManager{
		policy:       policy,
		locks:        make(map[string]*FileLock),
		fileIndex:    make(map[string][]string),
		userIndex:    make(map[string][]string),
		sites:        make(map[string]*SiteInfo),
		conflicts:    make([]*LockConflict, 0),
		history:      make([]*HistoryEntry, 0),
		waitingQueue: make([]*WaitEntry, 0),
		syncChan:     make(chan *SyncMessage, 100),
		stopChan:     make(chan struct{}),
	}
}

// Start 启动管理器
func (m *LockManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("管理器已在运行")
	}

	m.running = true
	go m.cleanupLoop()
	go m.syncLoop()
	return nil
}

// Stop 停止管理器
func (m *LockManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
}

// cleanupLoop 定期清理过期锁
func (m *LockManager) cleanupLoop() {
	interval := time.Duration(m.policy.ReadLockMaxDuration/2) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanupExpired()
		}
	}
}

// syncLoop 同步消息处理循环
func (m *LockManager) syncLoop() {
	for {
		select {
		case <-m.stopChan:
			return
		case msg := <-m.syncChan:
			m.handleSyncMessage(msg)
		}
	}
}

// cleanupExpired 清理过期锁
func (m *LockManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, lock := range m.locks {
		if lock.Status == LockStatusActive && now.After(lock.ExpiresAt) {
			lock.Status = LockStatusExpired
			lock.ReleasedAt = &now
			lock.UpdatedAt = now
			m.removeFromIndexes(lock)
			m.addHistory(id, lock.FilePath, "expired", lock.HolderID, lock.HolderName, "锁已过期自动释放")
		}
	}
}

// handleSyncMessage 处理同步消息
func (m *LockManager) handleSyncMessage(msg *SyncMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch msg.Type {
	case "lock_acquired":
		// 同步其他站点获取的锁
		if msg.Lock != nil {
			m.locks[msg.Lock.ID] = msg.Lock
			m.addToIndexes(msg.Lock)
		}
	case "lock_released":
		// 同步其他站点释放的锁
		if msg.Lock != nil {
			if lock, ok := m.locks[msg.Lock.ID]; ok {
				lock.Status = LockStatusReleased
				lock.ReleasedAt = msg.Lock.ReleasedAt
				lock.UpdatedAt = msg.Timestamp
				m.removeFromIndexes(lock)
			}
		}
	case "lock_renewed":
		// 同步其他站点续期的锁
		if msg.Lock != nil {
			if lock, ok := m.locks[msg.Lock.ID]; ok {
				lock.ExpiresAt = msg.Lock.ExpiresAt
				lock.LastRenewedAt = msg.Lock.LastRenewedAt
				lock.UpdatedAt = msg.Timestamp
			}
		}
	}
}

// AcquireLock 获取文件锁
func (m *LockManager) AcquireLock(req *AcquireLockRequest) (*FileLock, error) {
	if !m.policy.Enabled {
		return nil, fmt.Errorf("全局文件锁功能未启用")
	}

	// 验证请求
	if err := ValidateAcquireRequest(req); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	lockType := req.LockType
	if lockType == "" {
		lockType = m.policy.DefaultLockType
	}
	lockScope := req.LockScope
	if lockScope == "" {
		lockScope = m.policy.DefaultLockScope
	}

	// 设置持续时间
	duration := req.Duration
	if duration <= 0 {
		if lockType == LockTypeRead {
			duration = m.policy.ReadLockMaxDuration
		} else {
			duration = m.policy.WriteLockMaxDuration
		}
	}

	// 验证持续时间
	maxDuration := m.policy.WriteLockMaxDuration
	if lockType == LockTypeRead {
		maxDuration = m.policy.ReadLockMaxDuration
	}
	if duration > maxDuration {
		return nil, fmt.Errorf("锁定时长超过最大限制: %d分钟", maxDuration)
	}

	// 检查用户锁数量限制
	userLocks := m.getUserActiveLocks(req.HolderID)
	if len(userLocks) >= m.policy.MaxLocksPerUser {
		return nil, fmt.Errorf("用户锁数量已达上限: %d", m.policy.MaxLocksPerUser)
	}

	// 检查锁冲突
	existingLocks := m.getFileActiveLocks(req.FilePath)
	for _, existing := range existingLocks {
		// 同一用户重复加锁检查
		if existing.HolderID == req.HolderID {
			if existing.LockType == lockType {
				return nil, fmt.Errorf("您已持有该文件的%s锁", lockType.String())
			}
			// 升级/降级由专门的方法处理
			return nil, fmt.Errorf("您已持有该文件的%s锁，请使用升级/降级功能", existing.LockType.String())
		}

		// 写锁冲突：任何写锁都阻止新的锁
		if existing.LockType == LockTypeWrite || lockType == LockTypeWrite {
			// 检测到冲突
			m.detectConflict(req.FilePath, existing, lockType)
			return nil, fmt.Errorf("文件 %s 存在冲突锁，持有者: %s", req.FilePath, existing.HolderName)
		}

		// 到这里两个都是读锁，检查范围冲突
		// 只有站点锁与站点锁在相同站点才冲突（读锁跨站点可以共存）
		if existing.LockScope == LockScopeSite && lockScope == LockScopeSite && existing.SiteID == req.SiteID {
			return nil, fmt.Errorf("文件 %s 在站点 %s 已存在读锁", req.FilePath, req.SiteID)
		}
	}

	// 创建锁
	now := time.Now()
	lockID := generateID()

	lock := &FileLock{
		ID:            lockID,
		FilePath:      req.FilePath,
		HolderID:      req.HolderID,
		HolderName:    req.HolderName,
		LockType:      lockType,
		LockScope:     lockScope,
		SiteID:        req.SiteID,
		AcquiredAt:    now,
		ExpiresAt:     now.Add(time.Duration(duration) * time.Minute),
		LastRenewedAt: now,
		Status:        LockStatusActive,
		Comment:       req.Comment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 保存锁
	m.locks[lockID] = lock
	m.addToIndexes(lock)

	// 添加历史记录
	m.addHistory(lockID, req.FilePath, "acquired", req.HolderID, req.HolderName,
		fmt.Sprintf("获取%s锁，范围: %s", lockType.String(), lockScope.String()))

	// 发送同步消息（全局锁和站点锁）
	if lockScope != LockScopeLocal {
		m.sendSyncMessage("lock_acquired", lock)
	}

	return lock, nil
}

// ReleaseLock 释放文件锁
func (m *LockManager) ReleaseLock(req *ReleaseLockRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.HolderID != req.HolderID {
		return fmt.Errorf("无权释放此锁：锁属于用户 %s", lock.HolderName)
	}

	if lock.Status != LockStatusActive {
		return fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	now := time.Now()
	lock.Status = LockStatusReleased
	lock.ReleasedAt = &now
	lock.ReleasedBy = req.HolderID
	lock.UpdatedAt = now

	m.removeFromIndexes(lock)
	m.addHistory(req.LockID, lock.FilePath, "released", req.HolderID, lock.HolderName, "用户主动释放")

	// 发送同步消息
	if lock.LockScope != LockScopeLocal {
		m.sendSyncMessage("lock_released", lock)
	}

	// 检查等待队列
	m.processWaitingQueue(lock.FilePath)

	return nil
}

// RenewLock 续期文件锁
func (m *LockManager) RenewLock(req *RenewLockRequest) (*FileLock, error) {
	if !m.policy.AllowRenewal {
		return nil, fmt.Errorf("续期功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.HolderID != req.HolderID {
		return nil, fmt.Errorf("无权续期此锁：锁属于用户 %s", lock.HolderName)
	}

	if lock.Status != LockStatusActive {
		return nil, fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	// 设置续期时长
	duration := req.Duration
	if duration <= 0 {
		if lock.LockType == LockTypeRead {
			duration = m.policy.ReadLockMaxDuration
		} else {
			duration = m.policy.WriteLockMaxDuration
		}
	}

	// 验证续期时长
	maxDuration := m.policy.WriteLockMaxDuration
	if lock.LockType == LockTypeRead {
		maxDuration = m.policy.ReadLockMaxDuration
	}
	if duration > maxDuration {
		return nil, fmt.Errorf("续期时长超过最大限制: %d分钟", maxDuration)
	}

	now := time.Now()
	lock.LastRenewedAt = now
	lock.ExpiresAt = now.Add(time.Duration(duration) * time.Minute)
	lock.UpdatedAt = now

	m.addHistory(req.LockID, lock.FilePath, "renewed", req.HolderID, lock.HolderName,
		fmt.Sprintf("续期%d分钟", duration))

	// 发送同步消息
	if lock.LockScope != LockScopeLocal {
		m.sendSyncMessage("lock_renewed", lock)
	}

	return lock, nil
}

// UpgradeLock 升级锁（读锁升级为写锁）
func (m *LockManager) UpgradeLock(req *UpgradeLockRequest) (*FileLock, error) {
	if !m.policy.AllowUpgradeDowngrade {
		return nil, fmt.Errorf("升级/降级功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.HolderID != req.HolderID {
		return nil, fmt.Errorf("无权升级此锁：锁属于用户 %s", lock.HolderName)
	}

	if lock.Status != LockStatusActive {
		return nil, fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	if lock.LockType != LockTypeRead {
		return nil, fmt.Errorf("只有读锁可以升级为写锁")
	}

	// 检查是否有其他读锁
	otherLocks := m.getFileActiveLocks(lock.FilePath)
	for _, other := range otherLocks {
		if other.ID != lock.ID {
			return nil, fmt.Errorf("文件存在其他锁，无法升级")
		}
	}

	lock.LockType = LockTypeWrite
	lock.UpdatedAt = time.Now()

	m.addHistory(req.LockID, lock.FilePath, "upgraded", req.HolderID, lock.HolderName, "读锁升级为写锁")

	return lock, nil
}

// DowngradeLock 降级锁（写锁降级为读锁）
func (m *LockManager) DowngradeLock(req *DowngradeLockRequest) (*FileLock, error) {
	if !m.policy.AllowUpgradeDowngrade {
		return nil, fmt.Errorf("升级/降级功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.HolderID != req.HolderID {
		return nil, fmt.Errorf("无权降级此锁：锁属于用户 %s", lock.HolderName)
	}

	if lock.Status != LockStatusActive {
		return nil, fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	if lock.LockType != LockTypeWrite {
		return nil, fmt.Errorf("只有写锁可以降级为读锁")
	}

	lock.LockType = LockTypeRead
	lock.UpdatedAt = time.Now()

	m.addHistory(req.LockID, lock.FilePath, "downgraded", req.HolderID, lock.HolderName, "写锁降级为读锁")

	return lock, nil
}

// GetLock 获取锁详情
func (m *LockManager) GetLock(lockID string) (*FileLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, ok := m.locks[lockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", lockID)
	}
	return lock, nil
}

// GetFileLocks 获取文件的所有活跃锁
func (m *LockManager) GetFileLocks(filePath string) []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getFileActiveLocks(filePath)
}

// GetUserLocks 获取用户的所有活跃锁
func (m *LockManager) GetUserLocks(userID string) []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getUserActiveLocks(userID)
}

// ListConflicts 列出冲突记录
func (m *LockManager) ListConflicts(resolved *bool) []*LockConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LockConflict, 0)
	for _, c := range m.conflicts {
		if resolved != nil && c.Resolved != *resolved {
			continue
		}
		result = append(result, c)
	}
	return result
}

// GetStatistics 获取锁统计信息
func (m *LockManager) GetStatistics() *LockStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LockStatistics{}
	today := time.Now().Truncate(24 * time.Hour)
	siteSet := make(map[string]bool)
	var totalHoldTime time.Duration
	holdCount := 0

	for _, lock := range m.locks {
		if lock.Status == LockStatusActive {
			stats.ActiveLocks++

			// 按类型统计
			if lock.LockType == LockTypeRead {
				stats.ReadLocks++
			} else {
				stats.WriteLocks++
			}

			// 按范围统计
			switch lock.LockScope {
			case LockScopeLocal:
				stats.LocalLocks++
			case LockScopeGlobal:
				stats.GlobalLocks++
			case LockScopeSite:
				stats.SiteLocks++
				siteSet[lock.SiteID] = true
			}

			// 计算持有时间
			totalHoldTime += time.Since(lock.AcquiredAt)
			holdCount++
		}
	}

	// 统计历史记录
	for _, entry := range m.history {
		if entry.Timestamp.Before(today) {
			continue
		}
		switch entry.Action {
		case "acquired":
			stats.TodayAcquisitions++
		case "released":
			stats.TodayReleases++
		}
	}

	// 统计冲突
	for _, c := range m.conflicts {
		stats.TotalConflicts++
		if c.Resolved {
			stats.ResolvedConflicts++
		} else {
			stats.UnresolvedConflicts++
		}
	}

	// 计算平均持有时间
	if holdCount > 0 {
		stats.AverageHoldTime = totalHoldTime.Seconds() / float64(holdCount)
	}

	stats.WaitingCount = len(m.waitingQueue)
	stats.ActiveSites = len(siteSet)

	return stats
}

// RegisterSite 注册站点
func (m *LockManager) RegisterSite(site *SiteInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sites[site.ID] = site
}

// Heartbeat 更新站点心跳
func (m *LockManager) Heartbeat(siteID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	site, ok := m.sites[siteID]
	if !ok {
		return fmt.Errorf("站点未注册: %s", siteID)
	}

	site.LastHeartbeat = time.Now()
	site.Online = true
	return nil
}

// GetSites 获取所有站点信息
func (m *LockManager) GetSites() []*SiteInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SiteInfo, 0, len(m.sites))
	for _, site := range m.sites {
		result = append(result, site)
	}
	return result
}

// GetHistory 获取历史记录
func (m *LockManager) GetHistory(limit int) []*HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*HistoryEntry, limit)
	copy(result, m.history[start:])
	return result
}

// GetPolicy 获取当前策略
func (m *LockManager) GetPolicy() *LockPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy := *m.policy
	return &policy
}

// UpdatePolicy 更新策略
func (m *LockManager) UpdatePolicy(policy *LockPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policy = policy
}

// ============================================================
// 内部辅助方法
// ============================================================

// getFileActiveLocks 获取文件的所有活跃锁（需持有锁）
func (m *LockManager) getFileActiveLocks(filePath string) []*FileLock {
	lockIDs, ok := m.fileIndex[filePath]
	if !ok {
		return nil
	}

	result := make([]*FileLock, 0)
	for _, id := range lockIDs {
		if lock, exists := m.locks[id]; exists && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// getUserActiveLocks 获取用户的所有活跃锁（需持有锁）
func (m *LockManager) getUserActiveLocks(userID string) []*FileLock {
	lockIDs, ok := m.userIndex[userID]
	if !ok {
		return nil
	}

	result := make([]*FileLock, 0)
	for _, id := range lockIDs {
		if lock, exists := m.locks[id]; exists && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// addToIndexes 添加到索引（需持有锁）
func (m *LockManager) addToIndexes(lock *FileLock) {
	m.fileIndex[lock.FilePath] = append(m.fileIndex[lock.FilePath], lock.ID)
	m.userIndex[lock.HolderID] = append(m.userIndex[lock.HolderID], lock.ID)
}

// removeFromIndexes 从索引移除（需持有锁）
func (m *LockManager) removeFromIndexes(lock *FileLock) {
	// 从文件索引移除
	fileLocks := m.fileIndex[lock.FilePath]
	for i, id := range fileLocks {
		if id == lock.ID {
			m.fileIndex[lock.FilePath] = append(fileLocks[:i], fileLocks[i+1:]...)
			break
		}
	}
	if len(m.fileIndex[lock.FilePath]) == 0 {
		delete(m.fileIndex, lock.FilePath)
	}

	// 从用户索引移除
	userLocks := m.userIndex[lock.HolderID]
	for i, id := range userLocks {
		if id == lock.ID {
			m.userIndex[lock.HolderID] = append(userLocks[:i], userLocks[i+1:]...)
			break
		}
	}
	if len(m.userIndex[lock.HolderID]) == 0 {
		delete(m.userIndex, lock.HolderID)
	}
}

// detectConflict 检测冲突
func (m *LockManager) detectConflict(filePath string, existing *FileLock, newLockType LockType) {
	conflict := &LockConflict{
		ID:               generateID(),
		FilePath:         filePath,
		ConflictingLocks: []*FileLock{existing},
		DetectedAt:       time.Now(),
		Resolved:         false,
		Resolution:       m.policy.DefaultResolution,
	}
	m.conflicts = append(m.conflicts, conflict)
}

// isScopeConflict 检查锁范围是否冲突
func (m *LockManager) isScopeConflict(existingScope, newScope LockScope, existingSiteID, newSiteID string) bool {
	// 全局锁与任何锁冲突
	if existingScope == LockScopeGlobal || newScope == LockScopeGlobal {
		return true
	}

	// 站点锁在同一站点内冲突
	if existingScope == LockScopeSite && newScope == LockScopeSite {
		return existingSiteID == newSiteID
	}

	// 本地锁不跨站点冲突
	return false
}

// processWaitingQueue 处理等待队列
func (m *LockManager) processWaitingQueue(filePath string) {
	// 简化实现：通知等待者文件锁已释放
	// 实际生产中可以通过回调或消息队列实现
	for i, entry := range m.waitingQueue {
		if entry.FilePath == filePath {
			// 从等待队列移除
			m.waitingQueue = append(m.waitingQueue[:i], m.waitingQueue[i+1:]...)
			break
		}
	}
}

// sendSyncMessage 发送同步消息
func (m *LockManager) sendSyncMessage(msgType string, lock *FileLock) {
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      msgType,
		Lock:      lock,
		Timestamp: time.Now(),
	}

	select {
	case m.syncChan <- msg:
	default:
		// 同步通道满，丢弃消息
	}
}

// addHistory 添加历史记录
func (m *LockManager) addHistory(lockID, filePath, action, userID, userName, detail string) {
	entry := &HistoryEntry{
		ID:        generateID(),
		LockID:    lockID,
		FilePath:  filePath,
		Action:    action,
		UserID:    userID,
		UserName:  userName,
		Detail:    detail,
		Timestamp: time.Now(),
	}
	m.history = append(m.history, entry)
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ============================================================
// 辅助函数
// ============================================================

// ContainsPath 检查路径是否匹配（支持前缀匹配）
func ContainsPath(path, prefix string) bool {
	return strings.HasPrefix(path, prefix)
}
