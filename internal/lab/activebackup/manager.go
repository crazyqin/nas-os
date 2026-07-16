// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 整机备份管理器.
type Manager struct {
	mu            sync.RWMutex
	agents        map[string]*AgentInfo    // agentID -> AgentInfo
	tasks         map[string]*BackupTask   // taskID -> BackupTask
	restorePoints map[string]*RestorePoint // pointID -> RestorePoint
	restoreJobs   map[string]*RestoreJob   // jobID -> RestoreJob
	storagePools  map[string]*StoragePool  // poolID -> StoragePool
	configPath    string
}

// NewManager 创建备份管理器.
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		agents:        make(map[string]*AgentInfo),
		tasks:         make(map[string]*BackupTask),
		restorePoints: make(map[string]*RestorePoint),
		restoreJobs:   make(map[string]*RestoreJob),
		storagePools:  make(map[string]*StoragePool),
		configPath:    configPath,
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载备份配置失败: %w", err)
		}
	}

	return m, nil
}

// ========== Agent 管理 ==========

// RegisterAgent 注册 Agent.
func (m *Manager) RegisterAgent(req AgentRegistrationRequest) (*AgentInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在（通过指纹或MAC地址）
	for _, a := range m.agents {
		if a.Fingerprint == req.Fingerprint {
			return nil, ErrAgentExists
		}
		if a.MACAddress == req.MACAddress {
			return nil, ErrAgentExists
		}
	}

	// 生成认证 Token
	token := generateToken()

	agent := &AgentInfo{
		ID:           generateID(),
		Name:         req.Name,
		Hostname:     req.Hostname,
		IP:           req.IP,
		Platform:     req.Platform,
		OSVersion:    req.OSVersion,
		AgentVersion: req.AgentVersion,
		MACAddress:   req.MACAddress,
		Fingerprint:  req.Fingerprint,
		Token:        token,
		Status:       AgentStatusOnline,
		LastSeen:     time.Now(),
		RegisteredAt: time.Now(),
		CPU:          req.CPU,
		Memory:       req.Memory,
		Tags:         req.Tags,
	}

	m.agents[agent.ID] = agent
	_ = m.saveConfig()
	return agent, nil
}

// GetAgent 获取 Agent 详情.
func (m *Manager) GetAgent(id string) (*AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

// ListAgents 列出所有 Agent.
func (m *Manager) ListAgents() []*AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AgentInfo, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

// UpdateAgent 更新 Agent 信息.
func (m *Manager) UpdateAgent(id string, req AgentRegistrationRequest) (*AgentInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}

	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Hostname != "" {
		agent.Hostname = req.Hostname
	}
	if req.IP != "" {
		agent.IP = req.IP
	}
	if req.OSVersion != "" {
		agent.OSVersion = req.OSVersion
	}
	if req.AgentVersion != "" {
		agent.AgentVersion = req.AgentVersion
	}
	if req.CPU != "" {
		agent.CPU = req.CPU
	}
	if req.Memory > 0 {
		agent.Memory = req.Memory
	}
	if len(req.Tags) > 0 {
		agent.Tags = req.Tags
	}

	_ = m.saveConfig()
	return agent, nil
}

// DeleteAgent 删除 Agent.
func (m *Manager) DeleteAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[id]; !exists {
		return ErrAgentNotFound
	}

	delete(m.agents, id)
	_ = m.saveConfig()
	return nil
}

// ProcessHeartbeat 处理 Agent 心跳.
func (m *Manager) ProcessHeartbeat(req AgentHeartbeatRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[req.AgentID]
	if !exists {
		return ErrAgentNotFound
	}

	agent.LastSeen = time.Now()
	if req.Status != "" {
		agent.Status = req.Status
	}
	if len(req.Disks) > 0 {
		agent.Disks = req.Disks
	}

	return nil
}

// CheckAgentOnline 检查 Agent 是否在线.
func (m *Manager) CheckAgentOnline(id string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return false, ErrAgentNotFound
	}

	// 超过 5 分钟未心跳视为离线
	return time.Since(agent.LastSeen) < 5*time.Minute, nil
}

// ========== 备份任务管理 ==========

// CreateTask 创建备份任务.
func (m *Manager) CreateTask(req CreateTaskRequest) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 Agent 存在
	if _, exists := m.agents[req.AgentID]; !exists {
		return nil, ErrAgentNotFound
	}

	// 设置默认值
	if req.ScheduleType == "" {
		req.ScheduleType = ScheduleTypeManual
	}
	if req.Compression == "" {
		req.Compression = CompressionLZ4
	}
	if req.Encryption == "" {
		req.Encryption = EncryptionNone
	}
	if req.RetentionDays <= 0 {
		req.RetentionDays = 30
	}
	if req.MaxVersions <= 0 {
		req.MaxVersions = 10
	}

	now := time.Now()
	task := &BackupTask{
		ID:              generateID(),
		Name:            req.Name,
		Description:     req.Description,
		AgentID:         req.AgentID,
		BackupType:      req.BackupType,
		Status:          TaskStatusIdle,
		ScheduleType:    req.ScheduleType,
		Schedule:        req.Schedule,
		StoragePoolID:   req.StoragePoolID,
		Compression:     req.Compression,
		Encryption:      req.Encryption,
		EncryptionKey:   req.EncryptionKey,
		RetentionDays:   req.RetentionDays,
		MaxVersions:     req.MaxVersions,
		Enabled:         req.Enabled,
		IncludeVolumes:  req.IncludeVolumes,
		ExcludePatterns: req.ExcludePatterns,
		BandwidthLimit:  req.BandwidthLimit,
		PreScript:       req.PreScript,
		PostScript:      req.PostScript,
		NotifyOnSuccess: req.NotifyOnSuccess,
		NotifyOnFailure: req.NotifyOnFailure,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	m.tasks[task.ID] = task
	_ = m.saveConfig()
	return task, nil
}

// GetTask 获取备份任务.
func (m *Manager) GetTask(id string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有备份任务.
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BackupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// UpdateTask 更新备份任务.
func (m *Manager) UpdateTask(id string, req UpdateTaskRequest) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.BackupType != nil {
		task.BackupType = *req.BackupType
	}
	if req.ScheduleType != nil {
		task.ScheduleType = *req.ScheduleType
	}
	if req.Schedule != nil {
		task.Schedule = *req.Schedule
	}
	if req.StoragePoolID != nil {
		task.StoragePoolID = *req.StoragePoolID
	}
	if req.Compression != nil {
		task.Compression = *req.Compression
	}
	if req.Encryption != nil {
		task.Encryption = *req.Encryption
	}
	if req.EncryptionKey != nil {
		task.EncryptionKey = *req.EncryptionKey
	}
	if req.RetentionDays != nil {
		task.RetentionDays = *req.RetentionDays
	}
	if req.MaxVersions != nil {
		task.MaxVersions = *req.MaxVersions
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	if req.IncludeVolumes != nil {
		task.IncludeVolumes = req.IncludeVolumes
	}
	if req.ExcludePatterns != nil {
		task.ExcludePatterns = req.ExcludePatterns
	}
	if req.BandwidthLimit != nil {
		task.BandwidthLimit = *req.BandwidthLimit
	}
	if req.PreScript != nil {
		task.PreScript = *req.PreScript
	}
	if req.PostScript != nil {
		task.PostScript = *req.PostScript
	}
	if req.NotifyOnSuccess != nil {
		task.NotifyOnSuccess = *req.NotifyOnSuccess
	}
	if req.NotifyOnFailure != nil {
		task.NotifyOnFailure = *req.NotifyOnFailure
	}

	task.UpdatedAt = time.Now()
	_ = m.saveConfig()
	return task, nil
}

// DeleteTask 删除备份任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return ErrTaskNotFound
	}

	// 不允许删除运行中的任务
	if m.tasks[id].Status == TaskStatusRunning {
		return ErrTaskRunning
	}

	delete(m.tasks, id)
	_ = m.saveConfig()
	return nil
}

// RunTask 手动执行备份任务.
func (m *Manager) RunTask(id string) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	if task.Status == TaskStatusRunning {
		return nil, ErrTaskRunning
	}

	// 检查 Agent 在线
	agent, agentExists := m.agents[task.AgentID]
	if !agentExists {
		return nil, ErrAgentNotFound
	}
	if time.Since(agent.LastSeen) > 5*time.Minute {
		return nil, ErrAgentOffline
	}

	now := time.Now()
	task.Status = TaskStatusRunning
	task.LastRunAt = &now
	task.Progress = 0
	task.Transferred = 0
	task.ErrorMsg = ""
	task.UpdatedAt = now
	task.TotalRuns++

	agent.Status = AgentStatusBackuping

	_ = m.saveConfig()
	return task, nil
}

// CancelTask 取消执行中的任务.
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusRunning {
		return ErrTaskNotRunning
	}

	task.Status = TaskStatusCancelled
	task.UpdatedAt = time.Now()
	task.ErrorMsg = "用户取消"

	// 恢复 Agent 状态
	if agent, ok := m.agents[task.AgentID]; ok {
		agent.Status = AgentStatusOnline
	}

	_ = m.saveConfig()
	return nil
}

// CompleteTask 完成备份任务（供 Agent 上报）.
func (m *Manager) CompleteTask(taskID string, success bool, restorePointID string, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return
	}

	if success {
		task.Status = TaskStatusSuccess
		task.SuccessRuns++
		task.RestorePoint = restorePointID
	} else {
		task.Status = TaskStatusFailed
		task.FailRuns++
		task.ErrorMsg = errorMsg
	}

	task.Progress = 100
	task.UpdatedAt = time.Now()

	now := time.Now()
	task.LastStatus = task.Status
	_ = now
	_ = task

	// 恢复 Agent 状态
	if agent, ok := m.agents[task.AgentID]; ok {
		agent.Status = AgentStatusOnline
	}

	_ = m.saveConfig()
}

// ========== 恢复点管理 ==========

// CreateRestorePoint 创建恢复点.
func (m *Manager) CreateRestorePoint(point *RestorePoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.restorePoints[point.ID] = point
	_ = m.saveConfig()
}

// GetRestorePoint 获取恢复点.
func (m *Manager) GetRestorePoint(id string) (*RestorePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	point, exists := m.restorePoints[id]
	if !exists {
		return nil, ErrRestorePointNotFound
	}
	return point, nil
}

// ListRestorePoints 列出恢复点.
func (m *Manager) ListRestorePoints(taskID string) []*RestorePoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RestorePoint, 0)
	for _, p := range m.restorePoints {
		if taskID == "" || p.TaskID == taskID {
			result = append(result, p)
		}
	}
	return result
}

// DeleteRestorePoint 删除恢复点.
func (m *Manager) DeleteRestorePoint(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.restorePoints[id]; !exists {
		return ErrRestorePointNotFound
	}

	delete(m.restorePoints, id)
	_ = m.saveConfig()
	return nil
}

// GetRestorePointChain 获取恢复点链.
func (m *Manager) GetRestorePointChain(pointID string) []*RestorePoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain := make([]*RestorePoint, 0)
	current, exists := m.restorePoints[pointID]
	if !exists {
		return chain
	}

	// 向上追溯链
	for current != nil {
		chain = append([]*RestorePoint{current}, chain...)
		if current.ParentID == "" {
			break
		}
		current = m.restorePoints[current.ParentID]
	}

	return chain
}

// BrowseRestorePoint 浏览恢复点内容.
func (m *Manager) BrowseRestorePoint(pointID string, path string) ([]BrowseItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.restorePoints[pointID]; !exists {
		return nil, ErrRestorePointNotFound
	}

	// 模拟返回文件列表（实际实现需要读取备份文件）
	items := []BrowseItem{
		{
			Path:    "/",
			Name:    "C:",
			IsDir:   true,
			Size:    0,
			ModTime: time.Now().Add(-24 * time.Hour),
			Mode:    "drwxr-xr-x",
		},
	}

	if path != "" {
		items = []BrowseItem{
			{
				Path:    filepath.Join(path, "Users"),
				Name:    "Users",
				IsDir:   true,
				Size:    0,
				ModTime: time.Now().Add(-24 * time.Hour),
				Mode:    "drwxr-xr-x",
			},
			{
				Path:    filepath.Join(path, "Windows"),
				Name:    "Windows",
				IsDir:   true,
				Size:    0,
				ModTime: time.Now().Add(-24 * time.Hour),
				Mode:    "drwxr-xr-x",
			},
			{
				Path:    filepath.Join(path, "boot.ini"),
				Name:    "boot.ini",
				IsDir:   false,
				Size:    512,
				ModTime: time.Now().Add(-48 * time.Hour),
				Mode:    "-rw-r--r--",
			},
		}
	}

	return items, nil
}

// ========== 恢复任务管理 ==========

// CreateRestoreJob 创建恢复任务.
func (m *Manager) CreateRestoreJob(req RestoreRequest) (*RestoreJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证恢复点
	restorePoint, exists := m.restorePoints[req.RestorePointID]
	if !exists {
		return nil, ErrRestorePointNotFound
	}

	// 确定目标 Agent
	targetAgentID := req.TargetAgentID
	if targetAgentID == "" {
		targetAgentID = restorePoint.AgentID
	}

	// 验证目标 Agent 在线
	agent, agentExists := m.agents[targetAgentID]
	if !agentExists {
		return nil, ErrAgentNotFound
	}
	if time.Since(agent.LastSeen) > 5*time.Minute {
		return nil, ErrAgentOffline
	}

	// 确定恢复类型
	restoreType := req.RestoreType
	if restoreType == "" {
		restoreType = RestoreTypeFull
	}

	job := &RestoreJob{
		ID:             generateID(),
		RestorePointID: req.RestorePointID,
		AgentID:        targetAgentID,
		RestoreType:    restoreType,
		Status:         TaskStatusRunning,
		Progress:       0,
		TotalBytes:     restorePoint.Size,
		StartedAt:      time.Now(),
	}

	m.restoreJobs[job.ID] = job
	agent.Status = AgentStatusRestoring

	_ = m.saveConfig()
	return job, nil
}

// GetRestoreJob 获取恢复任务.
func (m *Manager) GetRestoreJob(id string) (*RestoreJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.restoreJobs[id]
	if !exists {
		return nil, fmt.Errorf("恢复任务不存在: %s", id)
	}
	return job, nil
}

// ListRestoreJobs 列出恢复任务.
func (m *Manager) ListRestoreJobs() []*RestoreJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RestoreJob, 0, len(m.restoreJobs))
	for _, j := range m.restoreJobs {
		result = append(result, j)
	}
	return result
}

// ========== 存储管理 ==========

// CreateStoragePool 创建存储池.
func (m *Manager) CreateStoragePool(name, path string, totalBytes uint64) *StoragePool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool := &StoragePool{
		ID:         generateID(),
		Name:       name,
		Path:       path,
		TotalBytes: totalBytes,
		FreeBytes:  totalBytes,
		CreatedAt:  time.Now(),
	}

	m.storagePools[pool.ID] = pool
	_ = m.saveConfig()
	return pool
}

// GetStoragePool 获取存储池.
func (m *Manager) GetStoragePool(id string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.storagePools[id]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", id)
	}
	return pool, nil
}

// ListStoragePools 列出存储池.
func (m *Manager) ListStoragePools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*StoragePool, 0, len(m.storagePools))
	for _, p := range m.storagePools {
		result = append(result, p)
	}
	return result
}

// ========== 统计 ==========

// GetStats 获取备份统计.
func (m *Manager) GetStats() BackupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := BackupStats{
		TotalAgents: len(m.agents),
		TotalTasks:  len(m.tasks),
	}

	var totalSuccess, totalRuns int
	var lastBackup *time.Time

	for _, a := range m.agents {
		if a.Status == AgentStatusOnline || a.Status == AgentStatusBackuping {
			stats.OnlineAgents++
		}
	}

	for _, t := range m.tasks {
		if t.Status == TaskStatusRunning {
			stats.RunningTasks++
		}
		totalSuccess += t.SuccessRuns
		totalRuns += t.TotalRuns
		if t.LastRunAt != nil {
			if lastBackup == nil || t.LastRunAt.After(*lastBackup) {
				lastBackup = t.LastRunAt
			}
		}
	}

	stats.TotalRestorePoints = len(m.restorePoints)
	stats.LastBackupAt = lastBackup

	// 计算总数据量
	var totalData, totalStorage uint64
	for _, p := range m.restorePoints {
		totalData += p.Size
		totalStorage += p.CompressedSize
	}
	stats.TotalDataBytes = totalData
	stats.TotalStorageBytes = totalStorage

	if totalData > 0 {
		stats.CompressionRatio = float64(totalStorage) / float64(totalData)
	}
	if totalRuns > 0 {
		stats.SuccessRate = float64(totalSuccess) / float64(totalRuns) * 100
	}

	return stats
}

// GetStorageUsage 获取存储使用情况.
func (m *Manager) GetStorageUsage() StorageUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usage := StorageUsage{
		Pools: make([]StoragePool, 0, len(m.storagePools)),
	}

	for _, p := range m.storagePools {
		usage.Pools = append(usage.Pools, *p)
		usage.TotalBytes += p.TotalBytes
		usage.UsedBytes += p.UsedBytes
		usage.FreeBytes += p.FreeBytes
	}

	if usage.TotalBytes > 0 {
		usage.UsagePercent = float64(usage.UsedBytes) / float64(usage.TotalBytes) * 100
	}

	// 计算保留天数
	var oldest, newest *time.Time
	for _, p := range m.restorePoints {
		if oldest == nil || p.CreatedAt.Before(*oldest) {
			t := p.CreatedAt
			oldest = &t
		}
		if newest == nil || p.CreatedAt.After(*newest) {
			t := p.CreatedAt
			newest = &t
		}
	}
	usage.OldestBackup = oldest
	usage.NewestBackup = newest
	if oldest != nil {
		usage.RetainedDays = int(time.Since(*oldest).Hours() / 24)
	}

	return usage
}

// ========== 持久化 ==========

type persistentConfig struct {
	Agents        []*AgentInfo    `json:"agents"`
	Tasks         []*BackupTask   `json:"tasks"`
	RestorePoints []*RestorePoint `json:"restore_points"`
	RestoreJobs   []*RestoreJob   `json:"restore_jobs"`
	StoragePools  []*StoragePool  `json:"storage_pools"`
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
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	for _, a := range pc.Agents {
		m.agents[a.ID] = a
	}
	for _, t := range pc.Tasks {
		m.tasks[t.ID] = t
	}
	for _, p := range pc.RestorePoints {
		m.restorePoints[p.ID] = p
	}
	for _, j := range pc.RestoreJobs {
		m.restoreJobs[j.ID] = j
	}
	for _, p := range pc.StoragePools {
		m.storagePools[p.ID] = p
	}

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentConfig{
		Agents:        make([]*AgentInfo, 0, len(m.agents)),
		Tasks:         make([]*BackupTask, 0, len(m.tasks)),
		RestorePoints: make([]*RestorePoint, 0, len(m.restorePoints)),
		RestoreJobs:   make([]*RestoreJob, 0, len(m.restoreJobs)),
		StoragePools:  make([]*StoragePool, 0, len(m.storagePools)),
	}

	for _, a := range m.agents {
		pc.Agents = append(pc.Agents, a)
	}
	for _, t := range m.tasks {
		pc.Tasks = append(pc.Tasks, t)
	}
	for _, p := range m.restorePoints {
		pc.RestorePoints = append(pc.RestorePoints, p)
	}
	for _, j := range m.restoreJobs {
		pc.RestoreJobs = append(pc.RestoreJobs, j)
	}
	for _, p := range m.storagePools {
		pc.StoragePools = append(pc.StoragePools, p)
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 工具函数 ==========

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}
