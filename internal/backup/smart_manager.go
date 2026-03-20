// Package backup 智能备份管理器
// Version: v2.50.0
package backup

import (
	"sync"
	"time"
)

// SmartManager 智能备份管理器
type SmartManager struct {
	mu         sync.RWMutex
	configs    map[string]*JobBackupConfig
	tasks      map[string]*ActiveBackupJob
	configPath string
}

// JobBackupConfig 备份作业配置
type JobBackupConfig struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Source      string             `json:"source"`
	Destination string             `json:"destination"`
	Schedule    string             `json:"schedule"`
	Enabled     bool               `json:"enabled"`
	Priority    int                `json:"priority"`
	Compression *CompressionConfig `json:"compression,omitempty"`
	Encryption  *EncryptionConfig  `json:"encryption,omitempty"`
	Incremental *IncrementalConfig `json:"incremental,omitempty"`
	Versioning  *VersioningConfig  `json:"versioning,omitempty"`
	Cleanup     *CleanupConfig     `json:"cleanup,omitempty"`
	Performance *PerformanceConfig `json:"performance,omitempty"`
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	Level     int    `json:"level"`
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId"`
}

// IncrementalConfig 增量备份配置
type IncrementalConfig struct {
	Enabled            bool `json:"enabled"`
	FullBackupInterval int  `json:"fullBackupInterval"`
}

// VersioningConfig 版本控制配置
type VersioningConfig struct {
	Enabled     bool `json:"enabled"`
	MaxVersions int  `json:"maxVersions"`
	KeepDays    int  `json:"keepDays"`
}

// CleanupConfig 清理配置
type CleanupConfig struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retentionDays"`
	MaxBackups    int  `json:"maxBackups"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	ParallelJobs int   `json:"parallelJobs"`
	ChunkSize    int64 `json:"chunkSize"`
	IOBufferSize int   `json:"ioBufferSize"`
}

// ActiveBackupJob 活动的备份任务
type ActiveBackupJob struct {
	ID        string       `json:"id"`
	ConfigID  string       `json:"configId"`
	Status    BackupStatus `json:"status"`
	StartTime time.Time    `json:"startTime"`
	Progress  int          `json:"progress"`
	Error     string       `json:"error,omitempty"`
}

// SmartBackupJob 智能备份任务
type SmartBackupJob struct {
	Config *JobBackupConfig `json:"config"`
}

// NewSmartManager 创建智能备份管理器
func NewSmartManager(configPath string, logger interface{}) (*SmartManager, error) {
	return &SmartManager{
		configs:    make(map[string]*JobBackupConfig),
		tasks:      make(map[string]*ActiveBackupJob),
		configPath: configPath,
	}, nil
}

// GetConfig 获取配置
func (m *SmartManager) GetConfig(id string) (*JobBackupConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[id], nil
}

// ListConfigs 列出所有配置
func (m *SmartManager) ListConfigs() []*JobBackupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*JobBackupConfig, 0, len(m.configs))
	for _, c := range m.configs {
		configs = append(configs, c)
	}
	return configs
}

// CreateConfig 创建配置
func (m *SmartManager) CreateConfig(config *JobBackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[config.ID] = config
	return nil
}

// UpdateConfig 更新配置
func (m *SmartManager) UpdateConfig(config *JobBackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[config.ID] = config
	return nil
}

// DeleteConfig 删除配置
func (m *SmartManager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, id)
	return nil
}

// RunBackup 执行备份
func (m *SmartManager) RunBackup(configID string) (*ActiveBackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &ActiveBackupJob{
		ID:        generateID(),
		ConfigID:  configID,
		Status:    BackupStatusRunning,
		StartTime: time.Now(),
	}

	m.tasks[job.ID] = job
	return job, nil
}

// GetTask 获取任务
func (m *SmartManager) GetTask(id string) (*ActiveBackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id], nil
}

// ListTasks 列出任务
func (m *SmartManager) ListTasks() []*ActiveBackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ActiveBackupJob, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetJob 获取作业配置（兼容调度器）
func (m *SmartManager) GetJob(id string) (*JobBackupConfig, error) {
	return m.GetConfig(id)
}

// ListJobs 列出作业配置（兼容调度器）
func (m *SmartManager) ListJobs() []*JobBackupConfig {
	return m.ListConfigs()
}

// generateUUID 生成 UUID
func generateUUID() string {
	return generateID()
}

// BackupStatus 备份状态类型（兼容别名，指向manager.go中的Status）
type BackupStatus = Status

// 备份状态常量（复用manager.go中的Status类型）
const (
	BackupStatusPending   Status = "pending"
	BackupStatusRunning   Status = "running"
	BackupStatusCompleted Status = "completed"
	BackupStatusFailed    Status = "failed"
)
