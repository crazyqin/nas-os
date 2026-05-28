package drivesync

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncIdle       SyncStatus = "idle"
	SyncSyncing    SyncStatus = "syncing"
	SyncPaused     SyncStatus = "paused"
	SyncError      SyncStatus = "error"
	SyncConflict   SyncStatus = "conflict"
)

// SyncFile 同步文件记录
type SyncFile struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Version      int       `json:"version"`
	IsDeleted    bool      `json:"is_deleted"`
	SyncedAt     time.Time `json:"synced_at"`
}

// SyncFolder 同步文件夹
type SyncFolder struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	LocalPath         string            `json:"local_path"`
	RemotePath        string            `json:"remote_path"`
	DeviceID          string            `json:"device_id"`
	Status            SyncStatus        `json:"status"`
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`
	Enabled           bool              `json:"enabled"`
	SelectiveSync     bool              `json:"selective_sync"`
	SelectivePatterns []string          `json:"selective_patterns"`
	LastSync          time.Time         `json:"last_sync"`
	TotalFiles        int               `json:"total_files"`
	SyncedFiles       int               `json:"synced_files"`
	ConflictFiles     int               `json:"conflict_files"`
	ErrorFiles        int               `json:"error_files"`
	CreatedAt         time.Time         `json:"created_at"`
}

// SyncEvent 同步事件
type SyncEvent struct {
	ID        string    `json:"id"`
	FolderID  string    `json:"folder_id"`
	FilePath  string    `json:"file_path"`
	Action    string    `json:"action"` // create, update, delete, conflict
	Status    string    `json:"status"` // success, failed, pending
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SyncDevice 同步设备
type SyncDevice struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // desktop, mobile, server
	OS          string    `json:"os"`
	LastSeen    time.Time `json:"last_seen"`
	IsOnline    bool      `json:"is_online"`
	ClientVer   string    `json:"client_version"`
	LinkedAt    time.Time `json:"linked_at"`
}

// DriveSyncManager 文件同步管理器 (类似群晖 Drive)
type DriveSyncManager struct {
	mu         sync.RWMutex
	folders    map[string]*SyncFolder
	devices    map[string]*SyncDevice
	files      map[string]map[string]*SyncFile // folderID -> filePath -> SyncFile
	events     []SyncEvent
	maxEvents  int
}

// NewDriveSyncManager 创建文件同步管理器
func NewDriveSyncManager() *DriveSyncManager {
	return &DriveSyncManager{
		folders:   make(map[string]*SyncFolder),
		devices:   make(map[string]*SyncDevice),
		files:     make(map[string]map[string]*SyncFile),
		events:    make([]SyncEvent, 0),
		maxEvents: 10000,
	}
}

// CreateFolder 创建同步文件夹
func (m *DriveSyncManager) CreateFolder(folder *SyncFolder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if folder.ID == "" {
		return fmt.Errorf("文件夹ID不能为空")
	}
	if _, exists := m.folders[folder.ID]; exists {
		return fmt.Errorf("同步文件夹 %s 已存在", folder.ID)
	}

	folder.Status = SyncIdle
	folder.CreatedAt = time.Now()
	m.folders[folder.ID] = folder
	m.files[folder.ID] = make(map[string]*SyncFile)
	return nil
}
func (m *DriveSyncManager) DeleteFolder(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.folders[folderID]; !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	delete(m.folders, folderID)
	delete(m.files, folderID)
	return nil
}

// GetFolder 获取同步文件夹
func (m *DriveSyncManager) GetFolder(folderID string) (*SyncFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return nil, fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}
	return folder, nil
}

// ListFolders 列出所有同步文件夹
func (m *DriveSyncManager) ListFolders(deviceID string) []*SyncFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SyncFolder, 0)
	for _, folder := range m.folders {
		if deviceID == "" || folder.DeviceID == deviceID {
			result = append(result, folder)
		}
	}
	return result
}

// RegisterDevice 注册同步设备
func (m *DriveSyncManager) RegisterDevice(device *SyncDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	device.LastSeen = time.Now()
	device.IsOnline = true
	device.LinkedAt = time.Now()
	m.devices[device.ID] = device
	return nil
}

// UnregisterDevice 注销同步设备
func (m *DriveSyncManager) UnregisterDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	delete(m.devices, deviceID)
	return nil
}

// ListDevices 列出同步设备
func (m *DriveSyncManager) ListDevices(onlineOnly bool) []*SyncDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SyncDevice, 0)
	for _, device := range m.devices {
		if !onlineOnly || device.IsOnline {
			result = append(result, device)
		}
	}
	return result
}

// UpdateFile 更新文件记录
func (m *DriveSyncManager) UpdateFile(folderID string, file *SyncFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	if file.Hash == "" {
		hash := sha256.Sum256([]byte(file.Path + file.ModTime.String()))
		file.Hash = fmt.Sprintf("%x", hash[:8])
	}

	if existing, ok := m.files[folderID][file.Path]; ok {
		if existing.Hash != file.Hash {
			file.Version = existing.Version + 1
			if folder.ConflictPolicy == ConflictKeepBoth {
				m.addEvent(folderID, file.Path, "conflict", "pending")
				folder.ConflictFiles++
			}
		}
	} else {
		file.Version = 1
		folder.TotalFiles++
	}

	file.SyncedAt = time.Now()
	m.files[folderID][file.Path] = file
	folder.SyncedFiles++

	m.addEvent(folderID, file.Path, "update", "success")
	return nil
}

// DeleteFile 删除文件记录
func (m *DriveSyncManager) DeleteFile(folderID, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	if file, ok := m.files[folderID][filePath]; ok {
		file.IsDeleted = true
		file.SyncedAt = time.Now()
		folder.TotalFiles--
		m.addEvent(folderID, filePath, "delete", "success")
	}
	return nil
}

// GetFileVersionHistory 获取文件版本历史
func (m *DriveSyncManager) GetFileVersionHistory(folderID, filePath string) (*SyncFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files, exists := m.files[folderID]
	if !exists {
		return nil, fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	file, exists := files[filePath]
	if !exists {
		return nil, fmt.Errorf("文件 %s 不存在", filePath)
	}
	return file, nil
}

// StartSync 开始同步
func (m *DriveSyncManager) StartSync(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	if folder.Status == SyncSyncing {
		return fmt.Errorf("同步文件夹 %s 正在同步中", folderID)
	}

	folder.Status = SyncSyncing
	folder.LastSync = time.Now()
	m.addEvent(folderID, "", "sync_start", "success")
	return nil
}

// StopSync 停止同步
func (m *DriveSyncManager) StopSync(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	folder.Status = SyncPaused
	m.addEvent(folderID, "", "sync_stop", "success")
	return nil
}

// CompleteSync 完成同步
func (m *DriveSyncManager) CompleteSync(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("同步文件夹 %s 不存在", folderID)
	}

	folder.Status = SyncIdle
	folder.LastSync = time.Now()
	m.addEvent(folderID, "", "sync_complete", "success")
	return nil
}

// GetSyncEvents 获取同步事件
func (m *DriveSyncManager) GetSyncEvents(folderID string, limit int) []SyncEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SyncEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if folderID == "" || m.events[i].FolderID == folderID {
			result = append(result, m.events[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetSyncStats 获取同步统计
func (m *DriveSyncManager) GetSyncStats(folderID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	totalFolders := 0
	totalFiles := 0
	syncedFiles := 0
	conflictFiles := 0

	for _, folder := range m.folders {
		if folderID == "" || folder.ID == folderID {
			totalFolders++
			totalFiles += folder.TotalFiles
			syncedFiles += folder.SyncedFiles
			conflictFiles += folder.ConflictFiles
		}
	}

	stats["total_folders"] = totalFolders
	stats["total_files"] = totalFiles
	stats["synced_files"] = syncedFiles
	stats["conflict_files"] = conflictFiles
	stats["total_devices"] = len(m.devices)

	return stats
}

func (m *DriveSyncManager) addEvent(folderID, filePath, action, status string) {
	event := SyncEvent{
		ID:        fmt.Sprintf("evt_%d", len(m.events)+1),
		FolderID:  folderID,
		FilePath:  filePath,
		Action:    action,
		Status:    status,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[1:]
	}
}
