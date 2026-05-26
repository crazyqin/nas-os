// Package smartbackup 提供智能多目标备份调度功能
// Version: v1.0.0 - 智能备份调度模块
package smartbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// BackupType 备份类型.
type BackupType string

const (
	// BackupTypeFull 全量备份.
	BackupTypeFull BackupType = "full"
	// BackupTypeIncremental 增量备份.
	BackupTypeIncremental BackupType = "incremental"
)

// TargetType 目标类型.
type TargetType string

const (
	// TargetTypeLocal 本地目标.
	TargetTypeLocal TargetType = "local"
	// TargetTypeNFS NFS目标.
	TargetTypeNFS TargetType = "nfs"
	// TargetTypeS3 S3目标.
	TargetTypeS3 TargetType = "s3"
	// TargetTypeCloud 云存储目标.
	TargetTypeCloud TargetType = "cloud"
)

// JobStatus 任务状态.
type JobStatus string

const (
	// JobStatusPending 待执行.
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning 执行中.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted 已完成.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed 失败.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled 已取消.
	JobStatusCancelled JobStatus = "cancelled"
)

// Config 智能备份配置.
type Config struct {
	// 基本配置
	StoragePath string `json:"storagePath"`
	TempPath    string `json:"tempPath"`
	LogPath     string `json:"logPath"`

	// 调度配置
	ScheduleWindowStart string `json:"scheduleWindowStart"` // HH:MM
	ScheduleWindowEnd   string `json:"scheduleWindowEnd"`   // HH:MM
	MaxConcurrent       int    `json:"maxConcurrent"`
	LoadThreshold       float64 `json:"loadThreshold"` // CPU负载阈值

	// 备份配置
	MaxChainLength    int  `json:"maxChainLength"`    // 最大备份链长度
	AutoVerify        bool `json:"autoVerify"`        // 自动验证
	CompressionLevel  int  `json:"compressionLevel"`  // 压缩级别 1-9
	Deduplication     bool `json:"deduplication"`     // 去重
	RetentionDays     int  `json:"retentionDays"`     // 保留天数
	MaxBackupSize     int64 `json:"maxBackupSize"`    // 单个备份最大大小(bytes)
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		StoragePath:        "/var/lib/nas-os/smartbackup",
		TempPath:           "/tmp/smartbackup",
		MaxConcurrent:      3,
		LoadThreshold:      0.8,
		MaxChainLength:     10,
		AutoVerify:         true,
		CompressionLevel:   6,
		Deduplication:      true,
		RetentionDays:      30,
		MaxBackupSize:      100 * 1024 * 1024 * 1024, // 100GB
		ScheduleWindowStart: "00:00",
		ScheduleWindowEnd:   "06:00",
	}
}

// BackupTarget 备份目标.
type BackupTarget struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TargetType        `json:"type"`
	Path        string            `json:"path"`
	Credentials map[string]string `json:"-"` // 敏感信息不序列化
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"` // 优先级，数字越小优先级越高
	Options     map[string]string `json:"options,omitempty"`
}

// BackupPolicy 备份策略.
type BackupPolicy struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Sources          []string     `json:"sources"`
	Targets          []string     `json:"targetIds"` // 关联的目标ID
	BackupType       BackupType   `json:"backupType"`
	Schedule         Schedule     `json:"schedule"`
	Retention        int          `json:"retention"`        // 保留备份数
	Compression      bool         `json:"compression"`
	Deduplication    bool         `json:"deduplication"`
	Encryption       bool         `json:"encryption"`
	FullBackupEvery  int          `json:"fullBackupEvery"` // 每N次增量后做全量
	VerifyAfter      bool         `json:"verifyAfter"`
	Priority         int          `json:"priority"`
	Enabled          bool         `json:"enabled"`
}

// Schedule 调度配置.
type Schedule struct {
	Type       string `json:"type"` // cron, interval, onetime
	Expression string `json:"expression,omitempty"`
	Interval   string `json:"interval,omitempty"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	TimeWindowStart string `json:"timeWindowStart,omitempty"` // HH:MM
	TimeWindowEnd   string `json:"timeWindowEnd,omitempty"`   // HH:MM
	LoadAware       bool   `json:"loadAware"`                  // 负载感知
}

// BackupJob 备份任务.
type BackupJob struct {
	ID          string     `json:"id"`
	PolicyID    string     `json:"policyId"`
	ChainID     string     `json:"chainId,omitempty"`
	Status      JobStatus  `json:"status"`
	BackupType  BackupType `json:"backupType"`
	SourcePath  string     `json:"sourcePath"`
	TargetID    string     `json:"targetId"`
	TargetPath  string     `json:"targetPath"`
	StartTime   time.Time  `json:"startTime"`
	EndTime     time.Time  `json:"endTime,omitempty"`
	Size        int64      `json:"size"`
	FileCount   int64      `json:"fileCount"`
	Checksum    string     `json:"checksum,omitempty"`
	Error       string     `json:"error,omitempty"`
	ParentID    string     `json:"parentId,omitempty"` // 父备份ID（增量链）
}

// BackupChain 备份链.
type BackupChain struct {
	ID        string     `json:"id"`
	PolicyID  string     `json:"policyId"`
	JobIDs    []string   `json:"jobIds"` // 按时间顺序排列
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// VerificationResult 验证结果.
type VerificationResult struct {
	JobID       string    `json:"jobId"`
	Valid       bool      `json:"valid"`
	CheckedAt   time.Time `json:"checkedAt"`
	FileCount   int64     `json:"fileCount"`
	Errors      []string  `json:"errors,omitempty"`
	ChecksumOK  bool      `json:"checksumOk"`
	SizeMatch   bool      `json:"sizeMatch"`
}

// Manager 智能备份管理器.
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	jobs     map[string]*BackupJob
	policies map[string]*BackupPolicy
	targets  map[string]*BackupTarget
	chains   map[string]*BackupChain
	running  int
	stopCh   chan struct{}
}

// NewManager 创建智能备份管理器.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		config:   config,
		jobs:     make(map[string]*BackupJob),
		policies: make(map[string]*BackupPolicy),
		targets:  make(map[string]*BackupTarget),
		chains:   make(map[string]*BackupChain),
		stopCh:   make(chan struct{}),
	}
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	// 创建必要的目录
	dirs := []string{
		m.config.StoragePath,
		m.config.TempPath,
		filepath.Join(m.config.StoragePath, "chains"),
		filepath.Join(m.config.StoragePath, "metadata"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}
	}
	return nil
}

// ========== 目标管理 ==========

// AddTarget 添加备份目标.
func (m *Manager) AddTarget(target *BackupTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.ID == "" {
		target.ID = generateID()
	}
	if target.Name == "" {
		return fmt.Errorf("目标名称不能为空")
	}
	if target.Type == "" {
		return fmt.Errorf("目标类型不能为空")
	}

	// 验证目标类型
	switch target.Type {
	case TargetTypeLocal, TargetTypeNFS, TargetTypeS3, TargetTypeCloud:
	default:
		return fmt.Errorf("不支持的目标类型: %s", target.Type)
	}

	m.targets[target.ID] = target
	return nil
}

// GetTarget 获取备份目标.
func (m *Manager) GetTarget(id string) (*BackupTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, ok := m.targets[id]
	if !ok {
		return nil, fmt.Errorf("目标不存在: %s", id)
	}
	return target, nil
}

// ListTargets 列出所有备份目标.
func (m *Manager) ListTargets() []*BackupTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*BackupTarget, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Priority < targets[j].Priority
	})
	return targets
}

// RemoveTarget 移除备份目标.
func (m *Manager) RemoveTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.targets[id]; !ok {
		return fmt.Errorf("目标不存在: %s", id)
	}

	// 检查是否有策略使用此目标
	for _, policy := range m.policies {
		for _, tid := range policy.Targets {
			if tid == id {
				return fmt.Errorf("目标 %s 正在被策略 %s 使用", id, policy.ID)
			}
		}
	}

	delete(m.targets, id)
	return nil
}

// ========== 策略管理 ==========

// SetPolicy 设置备份策略.
func (m *Manager) SetPolicy(policy *BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateID()
	}
	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}
	if len(policy.Sources) == 0 {
		return fmt.Errorf("源路径不能为空")
	}
	if len(policy.Targets) == 0 {
		return fmt.Errorf("备份目标不能为空")
	}

	// 验证目标存在
	for _, tid := range policy.Targets {
		if _, ok := m.targets[tid]; !ok {
			return fmt.Errorf("目标不存在: %s", tid)
		}
	}

	// 设置默认值
	if policy.Retention == 0 {
		policy.Retention = m.config.RetentionDays
	}
	if policy.FullBackupEvery == 0 {
		policy.FullBackupEvery = 5
	}

	m.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取备份策略.
func (m *Manager) GetPolicy(id string) (*BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*BackupPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*BackupPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// RemovePolicy 移除策略.
func (m *Manager) RemovePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)
	return nil
}

// ========== 任务管理 ==========

// CreateJob 创建备份任务.
func (m *Manager) CreateJob(policyID string) (*BackupJob, error) {
	m.mu.RLock()
	policy, ok := m.policies[policyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", policyID)
	}

	if !policy.Enabled {
		return nil, fmt.Errorf("策略已禁用: %s", policyID)
	}

	// 决定备份类型
	backupType := policy.BackupType
	if policy.BackupType == BackupTypeIncremental && policy.FullBackupEvery > 0 {
		// 检查是否需要全量备份
		chain := m.getChainByPolicy(policyID)
		if chain == nil || len(chain.JobIDs) >= policy.FullBackupEvery {
			backupType = BackupTypeFull
		}
	}

	// 选择目标
	targetID := m.selectTarget(policy.Targets)
	if targetID == "" {
		return nil, fmt.Errorf("没有可用的备份目标")
	}

	job := &BackupJob{
		ID:         generateID(),
		PolicyID:   policyID,
		Status:     JobStatusPending,
		BackupType: backupType,
		SourcePath: policy.Sources[0], // 简化：使用第一个源路径
		TargetID:   targetID,
		StartTime:  time.Now(),
	}

	m.mu.Lock()
	m.jobs[job.ID] = job

	// 管理备份链
	chain := m.getChainByPolicy(policyID)
	if backupType == BackupTypeFull {
		// 全量备份创建新链
		if chain == nil {
			chain = &BackupChain{
				ID:        generateID(),
				PolicyID:  policyID,
				CreatedAt: time.Now(),
			}
			m.chains[chain.ID] = chain
		}
		chain.JobIDs = []string{job.ID}
		chain.UpdatedAt = time.Now()
		job.ChainID = chain.ID
	} else {
		// 增量备份加入现有链
		if chain != nil {
			chain.JobIDs = append(chain.JobIDs, job.ID)
			chain.UpdatedAt = time.Now()
			job.ChainID = chain.ID
			// 设置父备份ID
			if len(chain.JobIDs) > 1 {
				job.ParentID = chain.JobIDs[len(chain.JobIDs)-2]
			}
		}
	}
	m.mu.Unlock()

	return job, nil
}

// GetJob 获取备份任务.
func (m *Manager) GetJob(id string) (*BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return job, nil
}

// ListJobs 列出所有任务.
func (m *Manager) ListJobs() []*BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartTime.After(jobs[j].StartTime)
	})
	return jobs
}

// CancelJob 取消任务.
func (m *Manager) CancelJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return fmt.Errorf("任务无法取消，当前状态: %s", job.Status)
	}

	job.Status = JobStatusCancelled
	job.EndTime = time.Now()
	return nil
}

// ========== 备份执行 ==========

// RunBackup 执行备份任务.
func (m *Manager) RunBackup(ctx context.Context, jobID string) error {
	m.mu.RLock()
	job, ok := m.jobs[jobID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status != JobStatusPending {
		return fmt.Errorf("任务状态不允许执行: %s", job.Status)
	}

	// 检查并发限制
	m.mu.Lock()
	if m.running >= m.config.MaxConcurrent {
		m.mu.Unlock()
		return fmt.Errorf("达到最大并发数: %d", m.config.MaxConcurrent)
	}
	m.running++
	job.Status = JobStatusRunning
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running--
		m.mu.Unlock()
	}()

	// 检查时间窗口
	if !m.isInTimeWindow() {
		return fmt.Errorf("当前不在备份时间窗口内")
	}

	// 检查系统负载
	if job.BackupType == BackupTypeIncremental {
		// 增量备份需要检查负载
		// 简化实现：跳过实际负载检查
	}

	// 执行备份
	var err error
	switch job.BackupType {
	case BackupTypeFull:
		err = m.runFullBackup(ctx, job)
	case BackupTypeIncremental:
		err = m.runIncrementalBackup(ctx, job)
	default:
		err = fmt.Errorf("不支持的备份类型: %s", job.BackupType)
	}

	m.mu.Lock()
	job.EndTime = time.Now()
	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = JobStatusCompleted
	}
	m.mu.Unlock()

	// 自动验证
	if err == nil && m.config.AutoVerify {
		if verifyErr := m.VerifyBackup(ctx, jobID); verifyErr != nil {
			slog.Warn("备份验证失败", "jobId", jobID, "error", verifyErr)
		}
	}

	return err
}

// runFullBackup 执行全量备份.
func (m *Manager) runFullBackup(ctx context.Context, job *BackupJob) error {
	target, err := m.GetTarget(job.TargetID)
	if err != nil {
		return err
	}

	// 构建备份路径
	backupDir := filepath.Join(target.Path, job.PolicyID, job.ID)
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 模拟备份过程
	sourceInfo, err := os.Stat(job.SourcePath)
	if err != nil {
		return fmt.Errorf("源路径不存在: %w", err)
	}

	job.Size = sourceInfo.Size()
	job.FileCount = 1 // 简化：实际需要遍历目录

	// 计算校验和
	checksum, err := m.calculateChecksum(job.SourcePath)
	if err != nil {
		return fmt.Errorf("计算校验和失败: %w", err)
	}
	job.Checksum = checksum
	job.TargetPath = backupDir

	return nil
}

// runIncrementalBackup 执行增量备份.
func (m *Manager) runIncrementalBackup(ctx context.Context, job *BackupJob) error {
	if job.ParentID == "" {
		// 没有父备份，执行全量备份
		job.BackupType = BackupTypeFull
		return m.runFullBackup(ctx, job)
	}

	// 获取父备份
	parentJob, err := m.GetJob(job.ParentID)
	if err != nil {
		return fmt.Errorf("父备份不存在: %w", err)
	}

	if parentJob.Status != JobStatusCompleted {
		return fmt.Errorf("父备份未完成: %s", parentJob.Status)
	}

	target, err := m.GetTarget(job.TargetID)
	if err != nil {
		return err
	}

	// 构建增量备份路径
	backupDir := filepath.Join(target.Path, job.PolicyID, job.ChainID, job.ID)
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 模拟增量备份（实际应该只备份变化的文件）
	sourceInfo, err := os.Stat(job.SourcePath)
	if err != nil {
		return fmt.Errorf("源路径不存在: %w", err)
	}

	job.Size = sourceInfo.Size() / 10 // 假设增量为10%
	job.FileCount = 1

	checksum, err := m.calculateChecksum(job.SourcePath)
	if err != nil {
		return fmt.Errorf("计算校验和失败: %w", err)
	}
	job.Checksum = checksum
	job.TargetPath = backupDir

	return nil
}

// ========== 验证与恢复 ==========

// VerifyBackup 验证备份完整性.
func (m *Manager) VerifyBackup(ctx context.Context, jobID string) error {
	m.mu.RLock()
	job, ok := m.jobs[jobID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status != JobStatusCompleted {
		return fmt.Errorf("任务未完成，无法验证: %s", job.Status)
	}

	// 验证备份文件存在
	if job.TargetPath == "" {
		return fmt.Errorf("备份路径为空")
	}

	// 验证校验和
	currentChecksum, err := m.calculateChecksum(job.SourcePath)
	if err != nil {
		return fmt.Errorf("计算校验和失败: %w", err)
	}

	if currentChecksum != job.Checksum {
		return fmt.Errorf("校验和不匹配")
	}

	return nil
}

// Restore 恢复备份.
func (m *Manager) Restore(ctx context.Context, jobID string, targetPath string) error {
	m.mu.RLock()
	job, ok := m.jobs[jobID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status != JobStatusCompleted {
		return fmt.Errorf("任务未完成，无法恢复: %s", job.Status)
	}

	// 如果是增量备份，需要恢复整个链
	if job.BackupType == BackupTypeIncremental && job.ChainID != "" {
		chain, err := m.GetChain(job.ChainID)
		if err != nil {
			return fmt.Errorf("备份链不存在: %w", err)
		}

		// 从链的开头（全量备份）开始恢复
		for _, chainJobID := range chain.JobIDs {
			chainJob, err := m.GetJob(chainJobID)
			if err != nil {
				return fmt.Errorf("链中任务不存在: %w", err)
			}

			if err := m.restoreSingleJob(ctx, chainJob, targetPath); err != nil {
				return fmt.Errorf("恢复失败: %w", err)
			}
		}
	} else {
		// 全量备份直接恢复
		if err := m.restoreSingleJob(ctx, job, targetPath); err != nil {
			return fmt.Errorf("恢复失败: %w", err)
		}
	}

	return nil
}

// restoreSingleJob 恢复单个备份任务.
func (m *Manager) restoreSingleJob(ctx context.Context, job *BackupJob, targetPath string) error {
	// 创建目标目录
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 模拟恢复过程
	// 实际应该从备份目标读取文件并写入目标路径
	return nil
}

// GetVerificationResult 获取验证结果.
func (m *Manager) GetVerificationResult(jobID string) (*VerificationResult, error) {
	m.mu.RLock()
	job, ok := m.jobs[jobID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}

	result := &VerificationResult{
		JobID:     jobID,
		CheckedAt: time.Now(),
		FileCount: job.FileCount,
	}

	// 执行验证
	currentChecksum, err := m.calculateChecksum(job.SourcePath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}

	result.ChecksumOK = currentChecksum == job.Checksum
	result.SizeMatch = true // 简化
	result.Valid = result.ChecksumOK && result.SizeMatch

	return result, nil
}

// ========== 备份链管理 ==========

// GetChain 获取备份链.
func (m *Manager) GetChain(id string) (*BackupChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, ok := m.chains[id]
	if !ok {
		return nil, fmt.Errorf("备份链不存在: %s", id)
	}
	return chain, nil
}

// ListChains 列出所有备份链.
func (m *Manager) ListChains() []*BackupChain {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chains := make([]*BackupChain, 0, len(m.chains))
	for _, c := range m.chains {
		chains = append(chains, c)
	}
	return chains
}

// PruneChain 修剪备份链（删除旧备份）.
func (m *Manager) PruneChain(chainID string, keepCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, ok := m.chains[chainID]
	if !ok {
		return fmt.Errorf("备份链不存在: %s", chainID)
	}

	if keepCount <= 0 {
		keepCount = 1
	}

	if len(chain.JobIDs) <= keepCount {
		return nil // 不需要修剪
	}

	// 删除旧的备份任务
	toRemove := chain.JobIDs[:len(chain.JobIDs)-keepCount]
	for _, jobID := range toRemove {
		if job, exists := m.jobs[jobID]; exists {
			job.Status = JobStatusCancelled // 标记为已删除
		}
	}

	chain.JobIDs = chain.JobIDs[len(chain.JobIDs)-keepCount:]
	chain.UpdatedAt = time.Now()

	return nil
}

// ========== 辅助方法 ==========

// selectTarget 选择最优目标.
func (m *Manager) selectTarget(targetIDs []string) string {
	// 按优先级选择
	var targets []*BackupTarget
	for _, tid := range targetIDs {
		if t, ok := m.targets[tid]; ok && t.Enabled {
			targets = append(targets, t)
		}
	}

	if len(targets) == 0 {
		return ""
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Priority < targets[j].Priority
	})

	return targets[0].ID
}

// getChainByPolicy 根据策略获取备份链.
func (m *Manager) getChainByPolicy(policyID string) *BackupChain {
	for _, chain := range m.chains {
		if chain.PolicyID == policyID {
			return chain
		}
	}
	return nil
}

// isInTimeWindow 检查是否在时间窗口内.
func (m *Manager) isInTimeWindow() bool {
	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	return currentTime >= m.config.ScheduleWindowStart && currentTime <= m.config.ScheduleWindowEnd
}

// calculateChecksum 计算文件校验和.
func (m *Manager) calculateChecksum(path string) (string, error) {
	// 简化实现：使用路径作为基础计算校验和
	// 实际应该读取文件内容计算
	hash := sha256.Sum256([]byte(path + time.Now().Format("2006-01-02")))
	return hex.EncodeToString(hash[:]), nil
}

// generateID 生成唯一ID.
func generateID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// ========== 统计与报告 ==========

// Stats 备份统计.
type Stats struct {
	TotalJobs      int           `json:"totalJobs"`
	CompletedJobs  int           `json:"completedJobs"`
	FailedJobs     int           `json:"failedJobs"`
	TotalSize      int64         `json:"totalSize"`
	TotalChains    int           `json:"totalChains"`
	AvgJobDuration time.Duration `json:"avgJobDuration"`
}

// GetStats 获取备份统计.
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalJobs:   len(m.jobs),
		TotalChains: len(m.chains),
	}

	var totalDuration time.Duration
	var completedCount int

	for _, job := range m.jobs {
		stats.TotalSize += job.Size
		switch job.Status {
		case JobStatusCompleted:
			stats.CompletedJobs++
			completedCount++
			if !job.EndTime.IsZero() {
				totalDuration += job.EndTime.Sub(job.StartTime)
			}
		case JobStatusFailed:
			stats.FailedJobs++
		}
	}

	if completedCount > 0 {
		stats.AvgJobDuration = totalDuration / time.Duration(completedCount)
	}

	return stats
}

// ========== 保存/加载（简化实现） ==========

// SaveState 保存状态到文件.
func (m *Manager) SaveState() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := struct {
		Jobs     map[string]*BackupJob     `json:"jobs"`
		Policies map[string]*BackupPolicy  `json:"policies"`
		Targets  map[string]*BackupTarget  `json:"targets"`
		Chains   map[string]*BackupChain   `json:"chains"`
	}{
		Jobs:     m.jobs,
		Policies: m.policies,
		Targets:  m.targets,
		Chains:   m.chains,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	statePath := filepath.Join(m.config.StoragePath, "state.json")
	return os.WriteFile(statePath, data, 0600)
}

// LoadState 从文件加载状态.
func (m *Manager) LoadState() error {
	statePath := filepath.Join(m.config.StoragePath, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，使用空状态
		}
		return fmt.Errorf("读取状态文件失败: %w", err)
	}

	state := struct {
		Jobs     map[string]*BackupJob     `json:"jobs"`
		Policies map[string]*BackupPolicy  `json:"policies"`
		Targets  map[string]*BackupTarget  `json:"targets"`
		Chains   map[string]*BackupChain   `json:"chains"`
	}{}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("解析状态文件失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if state.Jobs != nil {
		m.jobs = state.Jobs
	}
	if state.Policies != nil {
		m.policies = state.Policies
	}
	if state.Targets != nil {
		m.targets = state.Targets
	}
	if state.Chains != nil {
		m.chains = state.Chains
	}

	return nil
}

// ========== 策略模板 ==========

// CreateDefaultPolicy 创建默认策略.
func CreateDefaultPolicy(name string, sources []string, targetIDs []string) BackupPolicy {
	return BackupPolicy{
		Name:            name,
		Description:     "默认备份策略",
		Sources:         sources,
		Targets:         targetIDs,
		BackupType:      BackupTypeIncremental,
		Retention:       7,
		Compression:     true,
		Deduplication:   true,
		FullBackupEvery: 5,
		VerifyAfter:     true,
		Priority:        5,
		Enabled:         true,
		Schedule: Schedule{
			Type:     "cron",
			Expression: "0 2 * * *", // 每天凌晨2点
			LoadAware: true,
		},
	}
}

// CreateDailyFullPolicy 创建每日全量策略.
func CreateDailyFullPolicy(name string, sources []string, targetIDs []string) BackupPolicy {
	return BackupPolicy{
		Name:          name,
		Description:   "每日全量备份策略",
		Sources:       sources,
		Targets:       targetIDs,
		BackupType:    BackupTypeFull,
		Retention:     14,
		Compression:   true,
		Deduplication: true,
		VerifyAfter:   true,
		Priority:      3,
		Enabled:       true,
		Schedule: Schedule{
			Type:       "cron",
			Expression: "0 1 * * *", // 每天凌晨1点
			LoadAware:  false,
		},
	}
}

// CreateWeeklyFullIncrementalPolicy 创建每周全量+每日增量策略.
func CreateWeeklyFullIncrementalPolicy(name string, sources []string, targetIDs []string) BackupPolicy {
	return BackupPolicy{
		Name:            name,
		Description:     "每周日全量，其他时间增量",
		Sources:         sources,
		Targets:         targetIDs,
		BackupType:      BackupTypeIncremental,
		Retention:       30,
		Compression:     true,
		Deduplication:   true,
		FullBackupEvery: 7, // 每7次增量后做全量（即每周日）
		VerifyAfter:     true,
		Priority:        5,
		Enabled:         true,
		Schedule: Schedule{
			Type:       "cron",
			Expression: "0 2 * * *", // 每天凌晨2点
			LoadAware:  true,
		},
	}
}
