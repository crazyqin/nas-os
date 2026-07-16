// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// SyncService 数据同步服务.
type SyncService struct {
	mu      sync.RWMutex
	config  *SyncConfig
	stats   *SyncStats
	items   map[string]*SyncItem // itemID -> syncItem
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewSyncService 创建同步服务.
func NewSyncService(config *SyncConfig) *SyncService {
	if config == nil {
		config = DefaultSyncConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SyncService{
		config: config,
		stats:  &SyncStats{},
		items:  make(map[string]*SyncItem),
		ctx:    ctx,
		cancel: cancel,
	}
}

// DefaultSyncConfig 返回默认同步配置.
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Enabled:          true,
		AutoSyncPhotos:   true,
		AutoSyncVideos:   true,
		AutoSyncContacts: false,
		SyncOnWifiOnly:   true,
		MaxFileSize:      1024 * 1024 * 1024, // 1GB
		RemoteBasePath:   "/mobile-sync",
	}
}

// StartSync 开始同步.
func (s *SyncService) StartSync(userID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("sync is already running")
	}

	if !s.config.Enabled {
		return fmt.Errorf("sync is disabled")
	}

	s.running = true
	s.stats.IsSyncing = true

	// 启动同步协程
	go s.runSync(userID, deviceID)

	return nil
}

// StopSync 停止同步.
func (s *SyncService) StopSync() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
	s.stats.IsSyncing = false

	// 重置context
	s.ctx, s.cancel = context.WithCancel(context.Background())
}

// runSync 执行同步.
func (s *SyncService) runSync(userID, deviceID string) {
	startTime := time.Now()

	// 获取待同步项
	items := s.getPendingItems(userID, deviceID)

	s.mu.Lock()
	s.stats.TotalItems = int64(len(items))
	s.stats.PendingItems = int64(len(items))
	s.mu.Unlock()

	for _, item := range items {
		// 检查是否已取消
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 同步单个项
		s.syncItem(item)
	}

	// 更新统计
	s.mu.Lock()
	s.stats.IsSyncing = false
	s.stats.LastSyncTime = time.Now()
	s.stats.TotalSyncTime += time.Since(startTime)
	s.running = false
	s.mu.Unlock()
}

// getPendingItems 获取待同步项.
func (s *SyncService) getPendingItems(userID, deviceID string) []*SyncItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*SyncItem
	for _, item := range s.items {
		if item.UserID == userID && item.DeviceID == deviceID && item.Status == SyncPending {
			items = append(items, item)
		}
	}
	return items
}

// syncItem 同步单个项.
func (s *SyncService) syncItem(item *SyncItem) {
	// 更新状态为同步中
	s.mu.Lock()
	item.Status = SyncInProgress
	item.UpdatedAt = time.Now()
	s.mu.Unlock()

	if item.LocalPath != "" {
		if info, err := os.Stat(item.LocalPath); err == nil {
			item.FileSize = info.Size()
			item.FileName = info.Name()
			item.Checksum = checksumFile(item.LocalPath)
		} else {
			s.mu.Lock()
			item.Status = SyncFailed
			item.Error = err.Error()
			item.UpdatedAt = time.Now()
			s.stats.FailedItems++
			s.mu.Unlock()
			return
		}
	}

	now := time.Now()
	s.mu.Lock()
	item.Status = SyncCompleted
	item.Progress = 100
	item.CompletedAt = &now
	item.UpdatedAt = now

	// 更新统计
	s.stats.CompletedItems++
	s.stats.PendingItems--
	if s.stats.PendingItems < 0 {
		s.stats.PendingItems = 0
	}

	// 按类型统计
	switch item.Type {
	case SyncPhoto:
		s.stats.Photos++
	case SyncVideo:
		s.stats.Videos++
	case SyncFile:
		s.stats.Files++
	case SyncContact:
		s.stats.Contacts++
	case SyncDocument:
		s.stats.Documents++
	}

	s.stats.TotalBytes += item.FileSize
	s.stats.SyncedBytes += item.FileSize
	s.mu.Unlock()
}

// AddSyncItem 添加同步项.
func (s *SyncService) AddSyncItem(item *SyncItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.ID == "" {
		item.ID = generateID()
	}

	item.Status = SyncPending
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	s.items[item.ID] = item
	s.stats.PendingItems++

	return nil
}

// GetSyncItem 获取同步项.
func (s *SyncService) GetSyncItem(itemID string) (*SyncItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[itemID]
	return item, ok
}

// ListItems 列出用户同步项.
func (s *SyncService) ListItems(userID string) []*SyncItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*SyncItem
	for _, item := range s.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	return items
}

// ListItemsByDevice 列出设备同步项.
func (s *SyncService) ListItemsByDevice(deviceID string) []*SyncItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*SyncItem
	for _, item := range s.items {
		if item.DeviceID == deviceID {
			items = append(items, item)
		}
	}
	return items
}

// ListItemsByStatus 按状态列出同步项.
func (s *SyncService) ListItemsByStatus(userID string, status SyncStatus) []*SyncItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*SyncItem
	for _, item := range s.items {
		if item.UserID == userID && item.Status == status {
			items = append(items, item)
		}
	}
	return items
}

// RemoveSyncItem 删除同步项.
func (s *SyncService) RemoveSyncItem(itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return fmt.Errorf("sync item not found: %s", itemID)
	}

	// 更新统计
	switch item.Status {
	case SyncPending:
		s.stats.PendingItems--
	case SyncCompleted:
		s.stats.CompletedItems--
	case SyncFailed:
		s.stats.FailedItems--
	}

	delete(s.items, itemID)
	return nil
}

// GetStats 获取同步统计.
func (s *SyncService) GetStats() *SyncStats {
	return s.stats.GetSnapshot()
}

// GetConfig 获取同步配置.
func (s *SyncService) GetConfig() *SyncConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	config := *s.config
	return &config
}

// UpdateConfig 更新同步配置.
func (s *SyncService) UpdateConfig(config *SyncConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// IsRunning 检查是否正在同步.
func (s *SyncService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Stop 停止同步服务.
func (s *SyncService) Stop() {
	s.StopSync()
}

func checksumFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
