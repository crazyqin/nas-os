// Package filesyncclient 提供文件同步客户端核心管理逻辑
package filesyncclient

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager 文件同步管理器.
type Manager struct {
	mu        sync.RWMutex
	clients   map[string]*SyncClient
	folders   map[string]*SyncFolder
	conflicts map[string]*SyncConflict
	files     map[string]*SyncFile
	events    []*SyncEvent
	fileCount int
	totalSize int64
}

// NewManager 创建文件同步管理器.
func NewManager() *Manager {
	return &Manager{
		clients:   make(map[string]*SyncClient),
		folders:   make(map[string]*SyncFolder),
		conflicts: make(map[string]*SyncConflict),
		files:     make(map[string]*SyncFile),
		events:    make([]*SyncEvent, 0),
	}
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterClient 注册客户端.
func (m *Manager) RegisterClient(req *RegisterClientRequest) (*SyncClient, error) {
	if !IsValidDeviceType(req.DeviceType) {
		return nil, fmt.Errorf("invalid device type: %s", req.DeviceType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	client := &SyncClient{
		ID:         generateID(),
		Name:       req.Name,
		DeviceType: req.DeviceType,
		OS:         req.OS,
		LastSeen:   time.Now(),
		Status:     ClientOnline,
		PairedAt:   time.Now(),
	}

	m.clients[client.ID] = client

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  client.ID,
		EventType: EventCreate,
		FilePath:  "",
		Timestamp: time.Now(),
	})

	return client, nil
}

// ListClients 列出所有客户端.
func (m *Manager) ListClients() []*SyncClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]*SyncClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	return clients
}

// RemoveClient 移除客户端.
func (m *Manager) RemoveClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[id]
	if !ok {
		return fmt.Errorf("client not found: %s", id)
	}

	// 删除客户端的所有文件夹
	for folderID, folder := range m.folders {
		if folder.ClientID == id {
			delete(m.folders, folderID)
		}
	}

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  id,
		EventType: EventDelete,
		FilePath:  "",
		Timestamp: time.Now(),
	})

	delete(m.clients, id)
	_ = client // 防止编译器警告
	return nil
}

// CreateSyncFolder 创建同步文件夹.
func (m *Manager) CreateSyncFolder(req *CreateFolderRequest) (*SyncFolder, error) {
	if !IsValidSyncMode(req.SyncMode) {
		req.SyncMode = SyncTwoWay
	}
	if !IsValidConflictPolicy(req.ConflictPolicy) {
		req.ConflictPolicy = ConflictKeepLocal
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证客户端存在
	if _, ok := m.clients[req.ClientID]; !ok {
		return nil, fmt.Errorf("client not found: %s", req.ClientID)
	}

	folder := &SyncFolder{
		ID:             generateID(),
		ClientID:       req.ClientID,
		LocalPath:      req.LocalPath,
		RemotePath:     req.RemotePath,
		SyncMode:       req.SyncMode,
		Status:         FolderActive,
		LastSync:       time.Time{},
		FileCount:      0,
		SizeBytes:      0,
		ConflictPolicy: req.ConflictPolicy,
	}

	m.folders[folder.ID] = folder

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  req.ClientID,
		FolderID:  folder.ID,
		EventType: EventCreate,
		FilePath:  "",
		Timestamp: time.Now(),
	})

	return folder, nil
}

// ListFolders 列出所有同步文件夹.
func (m *Manager) ListFolders() []*SyncFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folders := make([]*SyncFolder, 0, len(m.folders))
	for _, f := range m.folders {
		folders = append(folders, f)
	}
	return folders
}

// UpdateFolder 更新同步文件夹设置.
func (m *Manager) UpdateFolder(id string, req *UpdateFolderRequest) (*SyncFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[id]
	if !ok {
		return nil, fmt.Errorf("folder not found: %s", id)
	}

	if req.SyncMode != "" && IsValidSyncMode(req.SyncMode) {
		folder.SyncMode = req.SyncMode
	}
	if req.Status != "" {
		folder.Status = req.Status
	}
	if req.ConflictPolicy != "" && IsValidConflictPolicy(req.ConflictPolicy) {
		folder.ConflictPolicy = req.ConflictPolicy
	}

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  folder.ClientID,
		FolderID:  id,
		EventType: EventUpdate,
		FilePath:  "",
		Timestamp: time.Now(),
	})

	return folder, nil
}

// TriggerSync 触发同步（模拟同步过程）.
func (m *Manager) TriggerSync(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[folderID]
	if !ok {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// 模拟同步过程
	folder.Status = FolderSyncing

	// 模拟添加文件
	fileCount := 5
	for i := 0; i < fileCount; i++ {
		file := &SyncFile{
			ID:           generateID(),
			FolderID:     folderID,
			Path:         fmt.Sprintf("%s/file_%d.txt", folder.LocalPath, i),
			SizeBytes:    int64(1024 * (i + 1)),
			Checksum:     generateID()[:8],
			Version:      1,
			LastModified: time.Now(),
			SyncStatus:   FileSynced,
		}
		m.files[file.ID] = file
	}

	folder.FileCount += fileCount
	folder.SizeBytes += int64(1024 * fileCount * (fileCount + 1) / 2)
	folder.Status = FolderActive
	folder.LastSync = time.Now()

	m.fileCount += fileCount
	m.totalSize += int64(1024 * fileCount * (fileCount + 1) / 2)

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  folder.ClientID,
		FolderID:  folderID,
		EventType: EventUpdate,
		FilePath:  "",
		Timestamp: time.Now(),
	})

	return nil
}

// ListConflicts 列出所有冲突.
func (m *Manager) ListConflicts() []*SyncConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conflicts := make([]*SyncConflict, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		conflicts = append(conflicts, c)
	}
	return conflicts
}

// ResolveConflict 解决冲突.
func (m *Manager) ResolveConflict(conflictID string, req *ResolveConflictRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, ok := m.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("conflict not found: %s", conflictID)
	}

	conflict.Resolution = req.Resolution
	conflict.ResolvedAt = time.Now()

	// 记录事件
	m.addEvent(&SyncEvent{
		ID:        generateID(),
		ClientID:  "",
		FolderID:  conflict.FolderID,
		EventType: EventConflict,
		FilePath:  conflict.FilePath,
		Timestamp: time.Now(),
	})

	return nil
}

// GetStats 获取同步统计.
func (m *Manager) GetStats() *SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeSyncs := 0
	for _, f := range m.folders {
		if f.Status == FolderSyncing {
			activeSyncs++
		}
	}

	conflictCount := 0
	for _, c := range m.conflicts {
		if c.ResolvedAt.IsZero() {
			conflictCount++
		}
	}

	return &SyncStats{
		TotalClients:  len(m.clients),
		TotalFolders:  len(m.folders),
		TotalFiles:    m.fileCount,
		TotalSize:     m.totalSize,
		ActiveSyncs:   activeSyncs,
		ConflictCount: conflictCount,
		UploadSpeed:   0,
		DownloadSpeed: 0,
	}
}

// GetEvents 获取同步事件.
func (m *Manager) GetEvents(clientID string) []*SyncEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if clientID == "" {
		return m.events
	}

	events := make([]*SyncEvent, 0)
	for _, e := range m.events {
		if e.ClientID == clientID {
			events = append(events, e)
		}
	}
	return events
}

// addEvent 添加事件（内部方法，调用时需持有锁）.
func (m *Manager) addEvent(event *SyncEvent) {
	m.events = append(m.events, event)

	// 限制事件数量
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}
}
