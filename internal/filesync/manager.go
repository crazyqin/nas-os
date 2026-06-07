// Package filesync 提供文件同步核心管理逻辑
package filesync

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 文件同步管理器
type Manager struct {
	mu         sync.RWMutex
	engine     *SyncEngine
	devices    map[string]*Device
	folders    map[string]*SyncFolder
	tasks      map[string]*SyncTask
	conflicts  map[string]*SyncConflict
	history    []FileHistory
	transfers  map[string]*TransferInfo
	bandwidth  map[string]*BandwidthLimit
	rules      map[string]*SelectiveSyncRule
	startedAt  time.Time
	totalFiles int
	totalSize  int64
}

// NewManager 创建文件同步管理器
func NewManager() *Manager {
	m := &Manager{
		devices:   make(map[string]*Device),
		folders:   make(map[string]*SyncFolder),
		tasks:     make(map[string]*SyncTask),
		conflicts: make(map[string]*SyncConflict),
		history:   make([]FileHistory, 0),
		transfers: make(map[string]*TransferInfo),
		bandwidth: make(map[string]*BandwidthLimit),
		rules:     make(map[string]*SelectiveSyncRule),
		startedAt: time.Now(),
		engine: &SyncEngine{
			Status: SyncStatusIdle,
		},
	}

	// 初始化默认带宽限制
	m.initDefaultBandwidth()
	// 初始化示例数据
	m.initSampleData()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%04x", time.Now().UnixNano(), rand.Intn(0xffff))
}

// checksum 计算文件校验和
func checksum(data string) string {
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:8])
}

// initDefaultBandwidth 初始化默认带宽限制
func (m *Manager) initDefaultBandwidth() {
	defaults := []BandwidthLimit{
		{
			ID: "bw-default", Name: "默认限制",
			UploadLimit: 10 * 1024 * 1024, DownloadLimit: 20 * 1024 * 1024,
			Enabled: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "bw-scheduled", Name: "工作时间限制",
			UploadLimit: 2 * 1024 * 1024, DownloadLimit: 5 * 1024 * 1024,
			ScheduleStart: "09:00", ScheduleEnd: "18:00",
			Enabled: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}

	for i := range defaults {
		bw := &defaults[i]
		m.bandwidth[bw.ID] = bw
	}
}

// initSampleData 初始化示例数据
func (m *Manager) initSampleData() {
	// 注册本地设备
	localDevice := &Device{
		ID: "dev-local", Name: "NAS-Server", Platform: "linux",
		IP: "127.0.0.1", Status: DeviceOnline, Version: "1.0.0",
		LastSeen: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.devices[localDevice.ID] = localDevice

	// 创建默认同步文件夹
	now := time.Now()
	folder := &SyncFolder{
		ID: "folder-default", Name: "我的文件",
		LocalPath: "/data/sync", RemotePath: "/sync",
		Direction: DirectionBoth, Enabled: true,
		ConflictPolicy: ConflictNewerWins,
		FileCount:      42, TotalSize: 1024 * 1024 * 500,
		LastSyncAt: &now, SyncedCount: 40,
		DeviceIDs: []string{"dev-local"},
		CreatedAt: now, UpdatedAt: now,
	}
	m.folders[folder.ID] = folder
	m.totalFiles = folder.FileCount
	m.totalSize = folder.TotalSize

	// 添加一些历史记录
	actions := []struct {
		path   string
		action FileAction
		size   int64
	}{
		{"documents/report.pdf", ActionCreate, 2048000},
		{"photos/vacation.jpg", ActionCreate, 5120000},
		{"documents/report.pdf", ActionModify, 2100000},
		{"backup/old-data.tar.gz", ActionDelete, 102400000},
		{"documents/notes.txt", ActionCreate, 15000},
	}

	for i, a := range actions {
		m.history = append(m.history, FileHistory{
			ID: generateID(), FolderID: "folder-default",
			FilePath: a.path, Version: i + 1, Size: a.size,
			Checksum: checksum(a.path), Action: a.action,
			DeviceID: "dev-local", CreatedAt: now.Add(-time.Duration(len(actions)-i) * time.Hour),
		})
	}

	// 模拟一个待解决的冲突
	m.conflicts["conflict-1"] = &SyncConflict{
		ID:       "conflict-1",
		FolderID: "folder-default",
		DeviceID: "dev-local",
		FilePath: "documents/shared-doc.docx",
		LocalVersion: FileVersion{
			Version: 3, Size: 50000, Checksum: checksum("local-v3"),
			Modified: now.Add(-10 * time.Minute), DeviceID: "dev-local", Action: ActionModify,
		},
		RemoteVersion: FileVersion{
			Version: 3, Size: 52000, Checksum: checksum("remote-v3"),
			Modified: now.Add(-5 * time.Minute), DeviceID: "dev-remote", Action: ActionModify,
		},
		Resolved: false, CreatedAt: now.Add(-5 * time.Minute),
	}
}

// GetSyncStatus 获取同步引擎状态
func (m *Manager) GetSyncStatus() *SyncEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.engine.Uptime = int64(time.Since(m.startedAt).Seconds())
	return m.engine
}

// StartSync 启动同步
func (m *Manager) StartSync(req *SyncStartRequest) (*SyncEngine, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.engine.Status == SyncStatusSyncing {
		return m.engine, fmt.Errorf("sync already in progress")
	}

	// 如果指定了文件夹，检查是否存在
	if req.FolderID != "" {
		if _, ok := m.folders[req.FolderID]; !ok {
			return nil, fmt.Errorf("folder not found: %s", req.FolderID)
		}
	}

	// 如果指定了设备，检查是否存在
	if req.DeviceID != "" {
		if _, ok := m.devices[req.DeviceID]; !ok {
			return nil, fmt.Errorf("device not found: %s", req.DeviceID)
		}
	}

	// 模拟创建同步任务
	taskCount := 3 + rand.Intn(5)
	for i := 0; i < taskCount; i++ {
		folderID := req.FolderID
		if folderID == "" {
			folderID = "folder-default"
		}
		deviceID := req.DeviceID
		if deviceID == "" {
			deviceID = "dev-local"
		}

		direction := DirectionBoth
		if rand.Float64() > 0.5 {
			direction = DirectionUpload
		}

		now := time.Now()
		task := &SyncTask{
			ID:        generateID(),
			FolderID:  folderID,
			DeviceID:  deviceID,
			Status:    TransferActive,
			Direction: direction,
			FilePath:  fmt.Sprintf("file-%d.dat", i),
			FileSize:  int64(1024 * (100 + rand.Intn(9900))),
			Speed:     int64(1024 * 1024 * (1 + rand.Intn(9))),
			StartedAt: &now,
			CreatedAt: now,
		}
		task.Transferred = task.FileSize * int64(rand.Intn(80)+10) / 100
		m.tasks[task.ID] = task
	}

	m.engine.Status = SyncStatusSyncing
	m.engine.ActiveTasks = len(m.tasks)
	now := time.Now()
	m.engine.LastSyncAt = &now

	return m.engine, nil
}

// StopSync 停止同步
func (m *Manager) StopSync(req *SyncStopRequest) (*SyncEngine, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.engine.Status != SyncStatusSyncing && m.engine.Status != SyncStatusPaused {
		return m.engine, fmt.Errorf("sync is not running")
	}

	// 暂停相关任务
	for _, task := range m.tasks {
		if req.FolderID != "" && task.FolderID != req.FolderID {
			continue
		}
		if req.DeviceID != "" && task.DeviceID != req.DeviceID {
			continue
		}
		task.Status = TransferPaused
	}

	m.engine.Status = SyncStatusPaused
	activeCount := 0
	for _, t := range m.tasks {
		if t.Status == TransferActive {
			activeCount++
		}
	}
	m.engine.ActiveTasks = activeCount

	return m.engine, nil
}

// GetConflicts 获取冲突列表
func (m *Manager) GetConflicts(folderID string) []SyncConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SyncConflict, 0)
	for _, c := range m.conflicts {
		if folderID != "" && c.FolderID != folderID {
			continue
		}
		result = append(result, *c)
	}
	return result
}

// ResolveConflict 解决冲突
func (m *Manager) ResolveConflict(req *ConflictResolveRequest) (*SyncConflict, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, ok := m.conflicts[req.ConflictID]
	if !ok {
		return nil, fmt.Errorf("conflict not found: %s", req.ConflictID)
	}

	if conflict.Resolved {
		return conflict, fmt.Errorf("conflict already resolved")
	}

	now := time.Now()
	conflict.Resolution = req.Resolution
	conflict.Resolved = true
	conflict.ResolvedBy = "user"
	conflict.ResolvedAt = &now

	// 根据解决策略处理
	switch req.Resolution {
	case ConflictKeepLocal:
		// 保留本地版本
		m.addHistory(conflict.FolderID, conflict.FilePath, conflict.LocalVersion.Size,
			ActionModify, conflict.LocalVersion.DeviceID, "冲突解决：保留本地版本")
	case ConflictKeepRemote:
		// 保留远程版本
		m.addHistory(conflict.FolderID, conflict.FilePath, conflict.RemoteVersion.Size,
			ActionModify, conflict.RemoteVersion.DeviceID, "冲突解决：保留远程版本")
	case ConflictKeepBoth:
		// 保留两个版本
		ext := filepath.Ext(conflict.FilePath)
		base := strings.TrimSuffix(conflict.FilePath, ext)
		m.addHistory(conflict.FolderID, fmt.Sprintf("%s (冲突副本)%s", base, ext),
			conflict.RemoteVersion.Size, ActionCreate, conflict.RemoteVersion.DeviceID, "冲突解决：保留两个版本")
	}

	// 更新同步引擎状态
	if m.engine.Status == SyncStatusConflict {
		hasUnresolved := false
		for _, c := range m.conflicts {
			if !c.Resolved {
				hasUnresolved = true
				break
			}
		}
		if !hasUnresolved {
			m.engine.Status = SyncStatusSyncing
		}
	}

	return conflict, nil
}

// addHistory 添加文件历史记录
func (m *Manager) addHistory(folderID, filePath string, size int64, action FileAction, deviceID, message string) {
	m.history = append(m.history, FileHistory{
		ID: generateID(), FolderID: folderID,
		FilePath: filePath, Version: len(m.history) + 1,
		Size: size, Checksum: checksum(filePath),
		Action: action, DeviceID: deviceID,
		Message: message, CreatedAt: time.Now(),
	})
}

// GetSyncHistory 获取同步历史
func (m *Manager) GetSyncHistory(req *HistoryRequest) []FileHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FileHistory, 0)
	for _, h := range m.history {
		if req.FolderID != "" && h.FolderID != req.FolderID {
			continue
		}
		if req.FilePath != "" && !strings.Contains(h.FilePath, req.FilePath) {
			continue
		}
		result = append(result, h)
	}

	// 分页
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := req.Offset
	if offset >= len(result) {
		return []FileHistory{}
	}
	if offset+limit > len(result) {
		limit = len(result) - offset
	}

	return result[offset : offset+limit]
}

// RestoreVersion 恢复文件版本
func (m *Manager) RestoreVersion(historyID string) (*FileHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.history {
		if h.ID == historyID {
			m.history[i].Restored = true
			m.addHistory(h.FolderID, h.FilePath, h.Size, ActionModify, "dev-local",
				fmt.Sprintf("恢复到版本 %d", h.Version))
			return &m.history[i], nil
		}
	}
	return nil, fmt.Errorf("history entry not found: %s", historyID)
}

// ListDevices 列出设备
func (m *Manager) ListDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result
}

// GetDevice 获取设备信息
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(req *DeviceRegisterRequest) *Device {
	m.mu.Lock()
	defer m.mu.Unlock()

	device := &Device{
		ID: generateID(), Name: req.Name, Platform: req.Platform,
		IP: req.IP, Status: DeviceOnline, Version: req.Version,
		LastSeen: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.devices[device.ID] = device
	return device
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[id]; !ok {
		return fmt.Errorf("device not found: %s", id)
	}
	delete(m.devices, id)

	// 移除关联的同步任务
	for taskID, task := range m.tasks {
		if task.DeviceID == id {
			delete(m.tasks, taskID)
		}
	}
	return nil
}

// ListFolders 列出同步文件夹
func (m *Manager) ListFolders() []SyncFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SyncFolder, 0, len(m.folders))
	for _, f := range m.folders {
		result = append(result, *f)
	}
	return result
}

// GetFolder 获取同步文件夹
func (m *Manager) GetFolder(id string) (*SyncFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folder, ok := m.folders[id]
	if !ok {
		return nil, fmt.Errorf("folder not found: %s", id)
	}
	return folder, nil
}

// CreateFolder 创建同步文件夹
func (m *Manager) CreateFolder(req *FolderCreateRequest) *SyncFolder {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Direction == "" {
		req.Direction = DirectionBoth
	}
	if req.ConflictPolicy == "" {
		req.ConflictPolicy = ConflictNewerWins
	}

	folder := &SyncFolder{
		ID: generateID(), Name: req.Name,
		LocalPath: req.LocalPath, RemotePath: req.RemotePath,
		Direction: req.Direction, Enabled: true,
		ConflictPolicy:  req.ConflictPolicy,
		FilterPatterns:  req.FilterPatterns,
		ExcludePatterns: req.ExcludePatterns,
		DeviceIDs:       req.DeviceIDs,
		CreatedAt:       time.Now(), UpdatedAt: time.Now(),
	}
	m.folders[folder.ID] = folder
	return folder
}

// DeleteFolder 删除同步文件夹
func (m *Manager) DeleteFolder(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.folders[id]; !ok {
		return fmt.Errorf("folder not found: %s", id)
	}
	delete(m.folders, id)

	// 清理关联任务
	for taskID, task := range m.tasks {
		if task.FolderID == id {
			delete(m.tasks, taskID)
		}
	}
	return nil
}

// GetSyncTasks 获取同步任务列表
func (m *Manager) GetSyncTasks(folderID, deviceID string) []SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SyncTask, 0)
	for _, t := range m.tasks {
		if folderID != "" && t.FolderID != folderID {
			continue
		}
		if deviceID != "" && t.DeviceID != deviceID {
			continue
		}
		result = append(result, *t)
	}
	return result
}

// GetTransferInfo 获取断点续传信息
func (m *Manager) GetTransferInfo(taskID string) (*TransferInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	chunkSize := int64(1024 * 1024) // 1MB chunks
	totalChunks := int((task.FileSize + chunkSize - 1) / chunkSize)
	doneChunks := int((task.Transferred + chunkSize - 1) / chunkSize)

	return &TransferInfo{
		TaskID:      task.ID,
		FilePath:    task.FilePath,
		FileSize:    task.FileSize,
		Offset:      task.Transferred,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		DoneChunks:  doneChunks,
		Resumable:   task.Status == TransferPaused || task.Status == TransferFailed,
		ETag:        checksum(task.FilePath + fmt.Sprintf("%d", task.FileSize)),
	}, nil
}

// ResumeTransfer 断点续传
func (m *Manager) ResumeTransfer(taskID string) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != TransferPaused && task.Status != TransferFailed {
		return task, fmt.Errorf("task is not paused or failed, cannot resume")
	}

	now := time.Now()
	task.Status = TransferActive
	task.StartedAt = &now
	task.Error = ""

	return task, nil
}

// GetBandwidthLimits 获取带宽限制
func (m *Manager) GetBandwidthLimits() []BandwidthLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BandwidthLimit, 0, len(m.bandwidth))
	for _, bw := range m.bandwidth {
		result = append(result, *bw)
	}
	return result
}

// UpdateBandwidthLimit 更新带宽限制
func (m *Manager) UpdateBandwidthLimit(id string, upload, download int64, scheduleStart, scheduleEnd string) (*BandwidthLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bw, ok := m.bandwidth[id]
	if !ok {
		return nil, fmt.Errorf("bandwidth limit not found: %s", id)
	}

	bw.UploadLimit = upload
	bw.DownloadLimit = download
	bw.ScheduleStart = scheduleStart
	bw.ScheduleEnd = scheduleEnd
	bw.UpdatedAt = time.Now()

	return bw, nil
}

// SetBandwidthEnabled 启用/禁用带宽限制
func (m *Manager) SetBandwidthEnabled(id string, enabled bool) (*BandwidthLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bw, ok := m.bandwidth[id]
	if !ok {
		return nil, fmt.Errorf("bandwidth limit not found: %s", id)
	}

	bw.Enabled = enabled
	bw.UpdatedAt = time.Now()
	return bw, nil
}

// ListSelectiveSyncRules 列出选择性同步规则
func (m *Manager) ListSelectiveSyncRules(folderID, deviceID string) []SelectiveSyncRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SelectiveSyncRule, 0)
	for _, r := range m.rules {
		if folderID != "" && r.FolderID != folderID {
			continue
		}
		if deviceID != "" && r.DeviceID != deviceID {
			continue
		}
		result = append(result, *r)
	}
	return result
}

// CreateSelectiveSyncRule 创建选择性同步规则
func (m *Manager) CreateSelectiveSyncRule(folderID, deviceID, pattern, ruleType string, priority int) *SelectiveSyncRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ruleType == "" {
		ruleType = "include"
	}

	rule := &SelectiveSyncRule{
		ID: generateID(), FolderID: folderID, DeviceID: deviceID,
		PathPattern: pattern, Enabled: true, Type: ruleType,
		Priority: priority, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.rules[rule.ID] = rule
	return rule
}

// DeleteSelectiveSyncRule 删除选择性同步规则
func (m *Manager) DeleteSelectiveSyncRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	delete(m.rules, id)
	return nil
}

// GetSyncStats 获取同步统计
func (m *Manager) GetSyncStats() *SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SyncStats{
		TotalFolders:  len(m.folders),
		TotalDevices:  len(m.devices),
		TotalFiles:    m.totalFiles,
		TotalSize:     m.totalSize,
		TotalVersions: len(m.history),
		EngineStatus:  m.engine.Status,
		Uptime:        int64(time.Since(m.startedAt).Seconds()),
	}

	// 统计任务
	for _, t := range m.tasks {
		switch t.Status {
		case TransferActive:
			stats.ActiveTasks++
			stats.SyncSpeed += t.Speed
		case TransferPending:
			stats.PendingTasks++
		case TransferFailed:
			stats.FailedTasks++
		}
	}

	// 统计冲突
	for _, c := range m.conflicts {
		stats.TotalConflicts++
		if !c.Resolved {
			stats.UnresolvedConflicts++
		}
	}

	// 最近冲突
	recentConflictCount := 5
	for _, c := range m.conflicts {
		if len(stats.RecentConflicts) < recentConflictCount {
			stats.RecentConflicts = append(stats.RecentConflicts, *c)
		}
	}

	// 最近历史
	recentHistoryCount := 10
	start := len(m.history) - recentHistoryCount
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.history); i++ {
		stats.RecentHistory = append(stats.RecentHistory, m.history[i])
	}

	return stats
}

// SimulateSync 模拟同步进度推进
func (m *Manager) SimulateSync() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == TransferActive {
			// 推进传输进度
			increment := task.Speed / 10 // 100ms worth
			task.Transferred += increment
			if task.Transferred >= task.FileSize {
				task.Transferred = task.FileSize
				task.Status = TransferCompleted
				now := time.Now()
				task.CompletedAt = &now
				m.engine.TotalSynced++

				// 添加历史记录
				m.history = append(m.history, FileHistory{
					ID: generateID(), FolderID: task.FolderID,
					FilePath: task.FilePath, Version: len(m.history) + 1,
					Size: task.FileSize, Checksum: checksum(task.FilePath),
					Action: ActionModify, DeviceID: task.DeviceID,
					Message: "同步完成", CreatedAt: now,
				})
			}

			// 模拟随机错误
			if rand.Float64() < 0.01 {
				task.Status = TransferFailed
				task.Error = "network timeout"
				m.engine.TotalErrors++
			}
		}
	}

	// 更新引擎状态
	activeCount := 0
	for _, t := range m.tasks {
		if t.Status == TransferActive {
			activeCount++
		}
	}
	m.engine.ActiveTasks = activeCount
}
