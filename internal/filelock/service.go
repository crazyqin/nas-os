// Package filelock 提供文件锁定功能。
// 服务层：管理锁的获取、释放、过期清理及列表查询。
package filelock

import (
	"fmt"
	"sync"
	"time"
)

// Service 文件锁服务
type Service struct {
	mu     sync.RWMutex
	config *Config
	locks  map[string]*LockInfo // lockID -> 锁信息
	// fileLocks 文件路径 -> 锁 ID 列表（快速查找文件上的锁）
	fileLocks map[string][]string
	// userLocks 用户 ID -> 锁 ID 列表（快速查找用户的锁）
	userLocks map[string][]string
	// stopChan 停止清理协程
	stopChan chan struct{}
	// running 是否正在运行
	running bool
}

// NewService 创建文件锁服务
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		config:    cfg,
		locks:     make(map[string]*LockInfo),
		fileLocks: make(map[string][]string),
		userLocks: make(map[string][]string),
		stopChan:  make(chan struct{}),
	}
}

// Start 启动服务，开始自动清理过期锁
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.cleanupLoop()
}

// Stop 停止服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
}

// cleanupLoop 定期清理过期锁
func (s *Service) cleanupLoop() {
	interval := time.Duration(s.config.CleanupIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanupExpired()
		}
	}
}

// cleanupExpired 清理过期锁
func (s *Service) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, lock := range s.locks {
		if lock.Status == LockStatusActive && now.After(lock.ExpiresAt) {
			lock.Status = LockStatusExpired
			lock.UpdatedAt = now
			s.removeFromFileIndex(lock.FilePath, id)
			s.removeFromUserIndex(lock.UserID, id)
		}
	}
}

// Lock 获取文件锁
func (s *Service) Lock(req *LockRequest) (*LockInfo, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("文件锁定功能未启用")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查用户锁数量限制
	if len(s.userLocks[req.UserID]) >= s.config.MaxLocksPerUser {
		return nil, fmt.Errorf("用户 %s 的锁数量已达上限 %d", req.UserID, s.config.MaxLocksPerUser)
	}

	// 检查文件是否已被锁定（同一文件不允许并发锁）
	for _, lockID := range s.fileLocks[req.FilePath] {
		lock, exists := s.locks[lockID]
		if exists && lock.Status == LockStatusActive {
			// 同一用户可以重新锁定（返回已有锁信息）
			if lock.UserID == req.UserID {
				return nil, fmt.Errorf("您已锁定该文件")
			}
			return nil, fmt.Errorf("文件已被用户 %s 锁定", lock.UserName)
		}
	}

	// 创建新锁
	lock := newLockInfo(req, s.config)
	s.locks[lock.ID] = lock
	s.fileLocks[req.FilePath] = append(s.fileLocks[req.FilePath], lock.ID)
	s.userLocks[req.UserID] = append(s.userLocks[req.UserID], lock.ID)

	return lock, nil
}

// Unlock 释放文件锁
// 支持 by lockID 或 by filePath + userID
func (s *Service) Unlock(req *UnlockRequest) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	releasedCount := 0

	if req.LockID != "" {
		// 按 lockID 解锁
		lock, ok := s.locks[req.LockID]
		if !ok {
			return 0, fmt.Errorf("锁不存在: %s", req.LockID)
		}
		if lock.UserID != req.UserID {
			return 0, fmt.Errorf("无权释放此锁：锁属于用户 %s", lock.UserName)
		}
		if lock.Status != LockStatusActive {
			return 0, fmt.Errorf("锁已非活跃状态: %s", lock.Status)
		}
		now := time.Now()
		lock.Status = LockStatusReleased
		lock.ReleasedAt = &now
		lock.UpdatedAt = now
		s.removeFromFileIndex(lock.FilePath, req.LockID)
		s.removeFromUserIndex(lock.UserID, req.LockID)
		releasedCount = 1
	} else if req.FilePath != "" {
		// 按 filePath + userID 解锁该文件上该用户的所有锁
		for _, lockID := range s.fileLocks[req.FilePath] {
			lock, exists := s.locks[lockID]
			if !exists || lock.Status != LockStatusActive {
				continue
			}
			if lock.UserID != req.UserID {
				continue
			}
			now := time.Now()
			lock.Status = LockStatusReleased
			lock.ReleasedAt = &now
			lock.UpdatedAt = now
			s.removeFromUserIndex(lock.UserID, lockID)
			releasedCount++
		}
		// 清理文件索引中的已释放锁
		s.compactFileIndex(req.FilePath)
	} else {
		return 0, fmt.Errorf("需要提供 lock_id 或 file_path")
	}

	if releasedCount == 0 {
		return 0, fmt.Errorf("未找到匹配的活跃锁")
	}

	return releasedCount, nil
}

// List 列出所有活跃锁
func (s *Service) List() *ListLocksResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LockInfo, 0)
	for _, lock := range s.locks {
		if lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}

	return &ListLocksResponse{
		Locks: result,
		Total: len(result),
	}
}

// ListByUser 列出指定用户的活跃锁
func (s *Service) ListByUser(userID string) []*LockInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LockInfo, 0)
	for _, lockID := range s.userLocks[userID] {
		if lock, exists := s.locks[lockID]; exists && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// ListByFile 列出指定文件的活跃锁
func (s *Service) ListByFile(filePath string) []*LockInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LockInfo, 0)
	for _, lockID := range s.fileLocks[filePath] {
		if lock, exists := s.locks[lockID]; exists && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// GetLock 获取锁详情
func (s *Service) GetLock(lockID string) (*LockInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lock, ok := s.locks[lockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", lockID)
	}
	return lock, nil
}

// IsFileLocked 检查文件是否被锁定
func (s *Service) IsFileLocked(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, lockID := range s.fileLocks[filePath] {
		if lock, exists := s.locks[lockID]; exists && lock.Status == LockStatusActive {
			return true
		}
	}
	return false
}

// GetConfig 获取配置
func (s *Service) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := *s.config
	return &cfg
}

// UpdateConfig 更新配置
func (s *Service) UpdateConfig(cfg *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// Stats 统计信息
type Stats struct {
	ActiveLocks    int `json:"active_locks"`
	ReleasedLocks  int `json:"released_locks"`
	ExpiredLocks   int `json:"expired_locks"`
	TotalLocks     int `json:"total_locks"`
	ActiveUsers    int `json:"active_users"`
	LockedFiles    int `json:"locked_files"`
}

// GetStats 获取统计信息
func (s *Service) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{}
	activeUsers := make(map[string]bool)
	lockedFiles := make(map[string]bool)

	for _, lock := range s.locks {
		stats.TotalLocks++
		switch lock.Status {
		case LockStatusActive:
			stats.ActiveLocks++
			activeUsers[lock.UserID] = true
			lockedFiles[lock.FilePath] = true
		case LockStatusReleased:
			stats.ReleasedLocks++
		case LockStatusExpired:
			stats.ExpiredLocks++
		}
	}

	stats.ActiveUsers = len(activeUsers)
	stats.LockedFiles = len(lockedFiles)
	return stats
}

// ===== 内部辅助方法 =====

// removeFromFileIndex 从文件索引中移除锁 ID
func (s *Service) removeFromFileIndex(filePath, lockID string) {
	ids := s.fileLocks[filePath]
	for i, id := range ids {
		if id == lockID {
			s.fileLocks[filePath] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(s.fileLocks[filePath]) == 0 {
		delete(s.fileLocks, filePath)
	}
}

// removeFromUserIndex 从用户索引中移除锁 ID
func (s *Service) removeFromUserIndex(userID, lockID string) {
	ids := s.userLocks[userID]
	for i, id := range ids {
		if id == lockID {
			s.userLocks[userID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(s.userLocks[userID]) == 0 {
		delete(s.userLocks, userID)
	}
}

// compactFileIndex 清理文件索引中已释放/过期的锁
func (s *Service) compactFileIndex(filePath string) {
	ids := s.fileLocks[filePath]
	if len(ids) == 0 {
		delete(s.fileLocks, filePath)
		return
	}

	active := make([]string, 0, len(ids))
	for _, id := range ids {
		if lock, exists := s.locks[id]; exists && lock.Status == LockStatusActive {
			active = append(active, id)
		}
	}
	if len(active) == 0 {
		delete(s.fileLocks, filePath)
	} else {
		s.fileLocks[filePath] = active
	}
}
