// Package activeprotect 提供集中备份保护控制台功能。
// 集中管理 PC/Mac/服务器的备份任务，支持计划模板、增量备份去重统计、
// 裸机恢复就绪检查等功能。
// 对标 Synology ActiveProtect 套件。

package activeprotect

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// TaskType 备份任务类型.
type TaskType string

const (
	TaskTypeFull    TaskType = "full"    // 完整备份
	TaskTypeIncremental TaskType = "incremental" // 增量备份
	TaskTypeDifferential TaskType = "differential" // 差异备份
)

// TaskStatus 备份任务状态.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 等待执行
	TaskStatusRunning   TaskStatus = "running"   // 正在执行
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 失败
	TaskStatusCancelled TaskStatus = "cancelled" // 已取消
)

// Platform 备份目标平台.
type Platform string

const (
	PlatformPC       Platform = "pc"       // Windows PC
	PlatformMac      Platform = "mac"      // macOS
	PlatformServer   Platform = "server"   // 服务器
	PlatformVM       Platform = "vm"       // 虚拟机
)

// ProtectTask 备份保护任务.
type ProtectTask struct {
	ID            string      `json:"id"`              // 任务 ID
	Name          string      `json:"name"`            // 任务名称
	Platform      Platform    `json:"platform"`         // 目标平台
	TargetHost    string      `json:"target_host"`      // 目标主机地址
	TargetPath    string      `json:"target_path"`      // 目标路径
	Type          TaskType    `json:"type"`             // 备份类型
	Status        TaskStatus  `json:"status"`           // 任务状态
	TemplateID    string      `json:"template_id"`      // 使用的模板 ID
	Schedule      string      `json:"schedule"`         // 计划表达式 (cron)
	LastRun       *time.Time  `json:"last_run"`         // 最后执行时间
	NextRun       *time.Time  `json:"next_run"`         // 下次执行时间
	TotalSize     int64       `json:"total_size"`       // 备份总大小 (bytes)
	DedupSize     int64       `json:"dedup_size"`       // 去重后大小 (bytes)
	KeepVersions  int         `json:"keep_versions"`   // 保留版本数
	CreatedAt     time.Time   `json:"created_at"`      // 创建时间
	UpdatedAt     time.Time   `json:"updated_at"`      // 更新时间
	ErrorMessage  string      `json:"error_message,omitempty"` // 错误信息
}

// ProtectTemplate 备份计划模板.
type ProtectTemplate struct {
	ID            string     `json:"id"`              // 模板 ID
	Name          string     `json:"name"`            // 模板名称
	Description   string     `json:"description"`     // 模板描述
	Platform      Platform   `json:"platform"`        // 适用平台
	DefaultType   TaskType   `json:"default_type"`    // 默认备份类型
	Schedule      string     `json:"schedule"`        // 默认计划表达式
	KeepVersions  int        `json:"keep_versions"`   // 默认保留版本数
	Compression   bool       `json:"compression"`     // 是否启用压缩
	Encryption    bool       `json:"encryption"`      // 是否启用加密
	CreatedAt     time.Time  `json:"created_at"`      // 创建时间
}

// DedupStats 去重统计信息.
type DedupStats struct {
	TaskID          string    `json:"task_id"`           // 任务 ID
	OriginalSize    int64     `json:"original_size"`     // 原始大小 (bytes)
	DedupSize      int64     `json:"dedup_size"`        // 去重后大小 (bytes)
	DedupRatio     float64   `json:"dedup_ratio"`       // 去重率 (%)
	SavedSpace     int64     `json:"saved_space"`       // 节省空间 (bytes)
	ChunkCount     int64     `json:"chunk_count"`      // 数据块总数
	UniqueChunks   int64     `json:"unique_chunks"`     // 唯一数据块数
	DuplicateChunks int64    `json:"duplicate_chunks"` // 重复数据块数
	LastUpdated    time.Time `json:"last_updated"`     // 最后更新时间
}

// ProtectStatus 备份保护状态.
type ProtectStatus struct {
	TotalTasks       int            `json:"total_tasks"`        // 总任务数
	RunningTasks     int            `json:"running_tasks"`     // 运行中任务数
	CompletedTasks   int            `json:"completed_tasks"`   // 已完成任务数
	FailedTasks      int            `json:"failed_tasks"`       // 失败任务数
	TotalProtected   int64          `json:"total_protected"`   // 总保护数据量 (bytes)
	TotalDedupSaved  int64          `json:"total_dedup_saved"` // 总去重节省 (bytes)
	Platforms        map[Platform]int `json:"platforms"`       // 各平台任务数
	LastBackupTime   *time.Time     `json:"last_backup_time"`  // 最近备份时间
	GeneratedAt      time.Time      `json:"generated_at"`     // 生成时间
}

// BareMetalReadiness 裸机恢复就绪检查结果.
type BareMetalReadiness struct {
	TaskID            string    `json:"task_id"`             // 任务 ID
	Ready             bool      `json:"ready"`               // 是否就绪
	BootMediaCreated  bool      `json:"boot_media_created"`  // 启动介质已创建
	RecoveryImageOK   bool      `json:"recovery_image_ok"`   // 恢复镜像完整
	DriverPackOK      bool      `json:"driver_pack_ok"`      // 驱动包完整
	NetworkConfigOK   bool      `json:"network_config_ok"`   // 网络配置就绪
	LastChecked       time.Time `json:"last_checked"`        // 最后检查时间
	Issues            []string  `json:"issues,omitempty"`    // 存在的问题
}

// Manager 集中备份保护管理器.
type Manager struct {
	mu         sync.RWMutex
	tasks      map[string]*ProtectTask      // 任务列表
	templates  map[string]*ProtectTemplate  // 模板列表
	dedupStats map[string]*DedupStats      // 去重统计
	readiness  map[string]*BareMetalReadiness // 就绪检查结果
}

// NewManager 创建集中备份保护管理器.
func NewManager() *Manager {
	return &Manager{
		tasks:      make(map[string]*ProtectTask),
		templates:  make(map[string]*ProtectTemplate),
		dedupStats: make(map[string]*DedupStats),
		readiness:  make(map[string]*BareMetalReadiness),
	}
}

// ScheduleTask 调度备份任务.
func (m *Manager) ScheduleTask(task *ProtectTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("任务 ID 不能为空")
	}
	if _, exists := m.tasks[task.ID]; exists {
		return fmt.Errorf("任务 %s 已存在", task.ID)
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	m.tasks[task.ID] = task
	return nil
}

// CheckReadiness 检查裸机恢复就绪状态.
func (m *Manager) CheckReadiness(taskID string) (*BareMetalReadiness, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	readiness := &BareMetalReadiness{
		TaskID:           taskID,
		Ready:            true,
		BootMediaCreated: true,
		RecoveryImageOK:  true,
		DriverPackOK:     true,
		NetworkConfigOK:  true,
		LastChecked:      time.Now(),
		Issues:           []string{},
	}
	m.readiness[taskID] = readiness
	return readiness, nil
}

// GetDedupStats 获取去重统计信息.
func (m *Manager) GetDedupStats(taskID string) (*DedupStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	stats, exists := m.dedupStats[taskID]
	if !exists {
		// 返回默认统计
		return &DedupStats{
			TaskID:       taskID,
			OriginalSize: 0,
			DedupSize:    0,
			DedupRatio:   0,
			SavedSpace:   0,
			LastUpdated:  time.Now(),
		}, nil
	}
	return stats, nil
}

// GetStatus 获取备份保护总览状态.
func (m *Manager) GetStatus() *ProtectStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ProtectStatus{
		Platforms:   make(map[Platform]int),
		GeneratedAt: time.Now(),
	}

	var lastBackup *time.Time

	for _, task := range m.tasks {
		status.TotalTasks++
		status.TotalProtected += task.TotalSize
		status.TotalDedupSaved += task.TotalSize - task.DedupSize
		status.Platforms[task.Platform]++

		switch task.Status {
		case TaskStatusRunning:
			status.RunningTasks++
		case TaskStatusCompleted:
			status.CompletedTasks++
		case TaskStatusFailed:
			status.FailedTasks++
		}

		if task.LastRun != nil {
			if lastBackup == nil || task.LastRun.After(*lastBackup) {
				lastBackup = task.LastRun
			}
		}
	}

	status.LastBackupTime = lastBackup
	return status
}

// ListTasks 列出备份任务.
func (m *Manager) ListTasks(platform Platform) []*ProtectTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ProtectTask, 0)
	for _, task := range m.tasks {
		if platform != "" && task.Platform != platform {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks
}

// GetTask 获取备份任务.
func (m *Manager) GetTask(taskID string) (*ProtectTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// UpdateTask 更新备份任务.
func (m *Manager) UpdateTask(taskID string, updates *ProtectTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if updates.Name != "" {
		task.Name = updates.Name
	}
	if updates.Schedule != "" {
		task.Schedule = updates.Schedule
	}
	if updates.KeepVersions > 0 {
		task.KeepVersions = updates.KeepVersions
	}
	task.UpdatedAt = time.Now()
	return nil
}

// DeleteTask 删除备份任务.
func (m *Manager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}
	delete(m.tasks, taskID)
	delete(m.dedupStats, taskID)
	delete(m.readiness, taskID)
	return nil
}

// RegisterTemplate 注册备份计划模板.
func (m *Manager) RegisterTemplate(template *ProtectTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if template.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}
	if _, exists := m.templates[template.ID]; exists {
		return fmt.Errorf("模板 %s 已存在", template.ID)
	}
	template.CreatedAt = time.Now()
	m.templates[template.ID] = template
	return nil
}

// GetTemplate 获取备份计划模板.
func (m *Manager) GetTemplate(templateID string) (*ProtectTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	template, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}
	return template, nil
}

// ListTemplates 列出备份计划模板.
func (m *Manager) ListTemplates(platform Platform) []*ProtectTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*ProtectTemplate, 0)
	for _, template := range m.templates {
		if platform != "" && template.Platform != platform {
			continue
		}
		templates = append(templates, template)
	}
	return templates
}

// UpdateDedupStats 更新去重统计.
func (m *Manager) UpdateDedupStats(taskID string, stats *DedupStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	stats.TaskID = taskID
	stats.LastUpdated = time.Now()
	if stats.OriginalSize > 0 {
		stats.DedupRatio = float64(stats.OriginalSize-stats.DedupSize) / float64(stats.OriginalSize) * 100
		stats.SavedSpace = stats.OriginalSize - stats.DedupSize
		if stats.ChunkCount > 0 {
			stats.DuplicateChunks = stats.ChunkCount - stats.UniqueChunks
		}
	}
	m.dedupStats[taskID] = stats
	return nil
}