package smarttiering

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 智能数据分层引擎 - 对标群晖DSM 7.3 Synology Tiering
// 支持热/温/冷数据自动迁移、访问模式分析、存储优化

// 错误定义
var (
	ErrInvalidTierID    = errors.New("invalid tier ID")
	ErrTierNotFound     = errors.New("tier not found")
	ErrInvalidPolicyID  = errors.New("invalid policy ID")
	ErrPolicyNotFound   = errors.New("policy not found")
	ErrInvalidFileID    = errors.New("invalid file ID")
	ErrFileNotFound     = errors.New("file not found")
	ErrInsufficientSpace = errors.New("insufficient space")
	ErrMigrationFailed  = errors.New("migration failed")
	ErrTierLocked       = errors.New("tier is locked")
)

// TierConfig 分层配置
type TierConfig struct {
	EnableAutoTiering   bool          `json:"enable_auto_tiering"`
	AnalysisInterval    time.Duration `json:"analysis_interval"`
	MigrationBatchSize  int           `json:"migration_batch_size"`
	HeatThresholdDays   int           `json:"heat_threshold_days"`
	WarmThresholdDays   int           `json:"warm_threshold_days"`
	ColdThresholdDays   int           `json:"cold_threshold_days"`
	MinAccessCount      int           `json:"min_access_count"`
	EnableCompression   bool          `json:"enable_compression"`
	EnableDeduplication bool          `json:"enable_deduplication"`
}

// DefaultTierConfig 默认配置
func DefaultTierConfig() *TierConfig {
	return &TierConfig{
		EnableAutoTiering:   true,
		AnalysisInterval:    24 * time.Hour,
		MigrationBatchSize:  100,
		HeatThresholdDays:   7,
		WarmThresholdDays:   30,
		ColdThresholdDays:   90,
		MinAccessCount:      3,
		EnableCompression:   true,
		EnableDeduplication: true,
	}
}

// StorageTier 存储层
type StorageTier struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Type            TierType  `json:"type"`
	Performance     string    `json:"performance"` // high, medium, low
	PerformanceLevel int      `json:"performance_level"` // 3=hot, 2=warm, 1=cold, 0=archive
	StoragePath     string    `json:"storage_path"`
	TotalCapacity   int64     `json:"total_capacity"`
	UsedCapacity    int64     `json:"used_capacity"`
	AvailableSpace  int64     `json:"available_space"`
	CostPerGB       float64   `json:"cost_per_gb"` // 每GB月成本
	IsEncrypted     bool      `json:"is_encrypted"`
	IsCompressed    bool      `json:"is_compressed"`
	Status          TierStatus `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	FileCount       int       `json:"file_count"`
}

// TierType 存储层类型
type TierType string

const (
	TierTypeHot      TierType = "hot"
	TierTypeWarm     TierType = "warm"
	TierTypeCold     TierType = "cold"
	TierTypeArchive  TierType = "archive"
)

// TierStatus 存储层状态
type TierStatus string

const (
	TierStatusActive    TierStatus = "active"
	TierStatusMigrating TierStatus = "migrating"
	TierStatusLocked    TierStatus = "locked"
	TierStatusOffline   TierStatus = "offline"
)

// TieringPolicy 分层策略
type TieringPolicy struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	SourceTierID    string    `json:"source_tier_id"`
	TargetTierID    string    `json:"target_tier_id"`
	Conditions      []Condition `json:"conditions"`
	Actions         []Action    `json:"actions"`
	IsActive        bool      `json:"is_active"`
	Priority        int       `json:"priority"`
	Schedule        string    `json:"schedule,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastRunAt       time.Time `json:"last_run_at"`
	NextRunAt       time.Time `json:"next_run_at"`
	FilesMigrated   int64     `json:"files_migrated"`
	BytesMigrated   int64     `json:"bytes_migrated"`
}

// Condition 迁移条件
type Condition struct {
	Type      ConditionType `json:"type"`
	Operator  string        `json:"operator"`
	Value     interface{}   `json:"value"`
}

// ConditionType 条件类型
type ConditionType string

const (
	ConditionLastAccess    ConditionType = "last_access"
	ConditionLastModified  ConditionType = "last_modified"
	ConditionFileSize      ConditionType = "file_size"
	ConditionFileExtension ConditionType = "file_extension"
	ConditionAccessCount   ConditionType = "access_count"
	ConditionFileType      ConditionType = "file_type"
)

// Action 迁移动作
type Action struct {
	Type       ActionType `json:"type"`
	Compress   bool       `json:"compress,omitempty"`
	Encrypt    bool       `json:"encrypt,omitempty"`
	Notify     bool       `json:"notify,omitempty"`
	KeepOriginal bool     `json:"keep_original,omitempty"`
}

// ActionType 动作类型
type ActionType string

const (
	ActionMigrate   ActionType = "migrate"
	ActionArchive   ActionType = "archive"
	ActionDelete    ActionType = "delete"
	ActionNotify    ActionType = "notify"
	ActionCompress  ActionType = "compress"
)

// FileMetadata 文件元数据
type FileMetadata struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	Name            string    `json:"name"`
	Size            int64     `json:"size"`
	Extension       string    `json:"extension"`
	FileType        string    `json:"file_type"`
	CurrentTierID   string    `json:"current_tier_id"`
	OriginalTierID  string    `json:"original_tier_id"`
	LastAccessed    time.Time `json:"last_accessed"`
	LastModified    time.Time `json:"last_modified"`
	AccessCount     int64     `json:"access_count"`
	IsCompressed    bool      `json:"is_compressed"`
	IsEncrypted     bool      `json:"is_encrypted"`
	IsPinned        bool      `json:"is_pinned"`
	HeatScore       float64   `json:"heat_score"`
	Tags            []string  `json:"tags,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID            string    `json:"id"`
	FileID        string    `json:"file_id"`
	SourceTierID  string    `json:"source_tier_id"`
	TargetTierID  string    `json:"target_tier_id"`
	Status        TaskStatus `json:"status"`
	Priority      int       `json:"priority"`
	CreatedAt     time.Time `json:"created_at"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	Error         string    `json:"error,omitempty"`
	BytesTransfer int64     `json:"bytes_transfer"`
	Duration      int64     `json:"duration_ms"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TieringStats 分层统计
type TieringStats struct {
	TotalTiers       int                  `json:"total_tiers"`
	TotalPolicies    int                  `json:"total_policies"`
	TotalFiles       int                  `json:"total_files"`
	TotalMigrations  int64                `json:"total_migrations"`
	TierUsage        map[string]int64     `json:"tier_usage"`
	TierFiles        map[string]int       `json:"tier_files"`
	PolicyStats      map[string]int64     `json:"policy_stats"`
	AvgHeatScore     float64              `json:"avg_heat_score"`
	LastAnalysisAt   time.Time            `json:"last_analysis_at"`
}

// TieringEngine 智能分层引擎
type TieringEngine struct {
	mu           sync.RWMutex
	config       *TierConfig
	tiers        map[string]*StorageTier
	policies     map[string]*TieringPolicy
	files        map[string]*FileMetadata
	accessLog    map[string][]time.Time
	migrationQ   []*MigrationTask
	running      bool
	stopCh       chan struct{}
	stats        *TieringStats
	heatAnalyzer *HeatAnalyzer
}

// HeatAnalyzer 热度分析器
type HeatAnalyzer struct {
	weights map[string]float64
}

// NewTieringEngine 创建分层引擎
func NewTieringEngine(config *TierConfig) *TieringEngine {
	if config == nil {
		config = DefaultTierConfig()
	}
	return &TieringEngine{
		config:     config,
		tiers:      make(map[string]*StorageTier),
		policies:   make(map[string]*TieringPolicy),
		files:      make(map[string]*FileMetadata),
		accessLog:  make(map[string][]time.Time),
		migrationQ: make([]*MigrationTask, 0),
		stats:      &TieringStats{},
		heatAnalyzer: &HeatAnalyzer{
			weights: map[string]float64{
				"access_frequency": 0.4,
				"recency":          0.3,
				"file_size":        0.2,
				"file_type":        0.1,
			},
		},
	}
}

// Start 启动引擎
func (te *TieringEngine) Start() error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if te.running {
		return nil
	}

	te.running = true
	te.stopCh = make(chan struct{})
	return nil
}

// Stop 停止引擎
func (te *TieringEngine) Stop() error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if !te.running {
		return nil
	}

	close(te.stopCh)
	te.running = false
	return nil
}

// RegisterTier 注册存储层
func (te *TieringEngine) RegisterTier(tier *StorageTier) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if tier.ID == "" {
		return ErrInvalidTierID
	}

	tier.CreatedAt = time.Now()
	tier.UpdatedAt = time.Now()
	tier.Status = TierStatusActive
	te.tiers[tier.ID] = tier

	te.stats.TotalTiers++
	return nil
}

// UnregisterTier 注销存储层
func (te *TieringEngine) UnregisterTier(tierID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	tier, exists := te.tiers[tierID]
	if !exists {
		return ErrTierNotFound
	}

	if tier.Status == TierStatusLocked {
		return ErrTierLocked
	}

	delete(te.tiers, tierID)
	te.stats.TotalTiers--
	return nil
}

// GetTier 获取存储层
func (te *TieringEngine) GetTier(tierID string) (*StorageTier, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	tier, exists := te.tiers[tierID]
	if !exists {
		return nil, ErrTierNotFound
	}

	return tier, nil
}

// ListTiers 列出所有存储层
func (te *TieringEngine) ListTiers() []*StorageTier {
	te.mu.RLock()
	defer te.mu.RUnlock()

	tiers := make([]*StorageTier, 0, len(te.tiers))
	for _, tier := range te.tiers {
		tiers = append(tiers, tier)
	}

	return tiers
}

// CreatePolicy 创建分层策略
func (te *TieringEngine) CreatePolicy(policy *TieringPolicy) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if policy.ID == "" {
		return ErrInvalidPolicyID
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	te.policies[policy.ID] = policy

	te.stats.TotalPolicies++
	return nil
}

// UpdatePolicy 更新策略
func (te *TieringEngine) UpdatePolicy(policy *TieringPolicy) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	existing, exists := te.policies[policy.ID]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	policy.FilesMigrated = existing.FilesMigrated
	policy.BytesMigrated = existing.BytesMigrated
	te.policies[policy.ID] = policy

	return nil
}

// DeletePolicy 删除策略
func (te *TieringEngine) DeletePolicy(policyID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	_, exists := te.policies[policyID]
	if !exists {
		return ErrPolicyNotFound
	}

	delete(te.policies, policyID)
	te.stats.TotalPolicies--
	return nil
}

// GetPolicy 获取策略
func (te *TieringEngine) GetPolicy(policyID string) (*TieringPolicy, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	policy, exists := te.policies[policyID]
	if !exists {
		return nil, ErrPolicyNotFound
	}

	return policy, nil
}

// ListPolicies 列出所有策略
func (te *TieringEngine) ListPolicies() []*TieringPolicy {
	te.mu.RLock()
	defer te.mu.RUnlock()

	policies := make([]*TieringPolicy, 0, len(te.policies))
	for _, policy := range te.policies {
		policies = append(policies, policy)
	}

	return policies
}

// RegisterFile 注册文件
func (te *TieringEngine) RegisterFile(file *FileMetadata) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if file.ID == "" {
		return ErrInvalidFileID
	}

	file.CreatedAt = time.Now()
	file.UpdatedAt = time.Now()
	file.HeatScore = te.calculateHeatScore(file)
	te.files[file.ID] = file

	te.stats.TotalFiles++
	return nil
}

// UnregisterFile 注销文件
func (te *TieringEngine) UnregisterFile(fileID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	_, exists := te.files[fileID]
	if !exists {
		return ErrFileNotFound
	}

	delete(te.files, fileID)
	delete(te.accessLog, fileID)
	te.stats.TotalFiles--
	return nil
}

// GetFile 获取文件
func (te *TieringEngine) GetFile(fileID string) (*FileMetadata, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	file, exists := te.files[fileID]
	if !exists {
		return nil, ErrFileNotFound
	}

	return file, nil
}

// RecordAccess 记录文件访问
func (te *TieringEngine) RecordAccess(fileID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	file, exists := te.files[fileID]
	if !exists {
		return ErrFileNotFound
	}

	now := time.Now()
	file.LastAccessed = now
	file.AccessCount++
	file.UpdatedAt = now

	te.accessLog[fileID] = append(te.accessLog[fileID], now)

	// 重新计算热度
	file.HeatScore = te.calculateHeatScore(file)

	return nil
}

// calculateHeatScore 计算热度分数
func (te *TieringEngine) calculateHeatScore(file *FileMetadata) float64 {
	score := 0.0

	// 访问频率权重
	accessFreq := float64(file.AccessCount) / 30.0 // 30天平均
	if accessFreq > 1.0 {
		accessFreq = 1.0
	}
	score += accessFreq * te.heatAnalyzer.weights["access_frequency"]

	// 最近访问权重
	daysSinceAccess := time.Since(file.LastAccessed).Hours() / 24.0
	recency := 1.0 / (1.0 + daysSinceAccess)
	score += recency * te.heatAnalyzer.weights["recency"]

	// 文件大小权重（小文件热度更高）
	sizeMB := float64(file.Size) / (1024 * 1024)
	sizeScore := 1.0 / (1.0 + sizeMB/100.0)
	score += sizeScore * te.heatAnalyzer.weights["file_size"]

	return score
}

// AnalyzeAndMigrate 分析并迁移
func (te *TieringEngine) AnalyzeAndMigrate() ([]*MigrationTask, error) {
	te.mu.Lock()
	defer te.mu.Unlock()

	if !te.config.EnableAutoTiering {
		return nil, nil
	}

	var tasks []*MigrationTask

	for _, policy := range te.policies {
		if !policy.IsActive {
			continue
		}

		_, exists := te.tiers[policy.SourceTierID]
		if !exists {
			continue
		}

		targetTier, exists := te.tiers[policy.TargetTierID]
		if !exists {
			continue
		}

		// 检查目标层空间
		if targetTier.AvailableSpace <= 0 {
			continue
		}

		// 查找符合条件的文件
		for _, file := range te.files {
			if file.CurrentTierID != policy.SourceTierID {
				continue
			}

			if file.IsPinned {
				continue
			}

			// 检查条件
			if te.matchesConditions(file, policy.Conditions) {
				task := &MigrationTask{
					ID:           generateTaskID(),
					FileID:       file.ID,
					SourceTierID: policy.SourceTierID,
					TargetTierID: policy.TargetTierID,
					Status:       TaskStatusPending,
					Priority:     policy.Priority,
					CreatedAt:    time.Now(),
				}

				tasks = append(tasks, task)
				te.migrationQ = append(te.migrationQ, task)

				// 限制批量大小
				if len(tasks) >= te.config.MigrationBatchSize {
					break
				}
			}
		}

		policy.LastRunAt = time.Now()
	}

	te.stats.LastAnalysisAt = time.Now()
	return tasks, nil
}

// matchesConditions 检查是否匹配条件
func (te *TieringEngine) matchesConditions(file *FileMetadata, conditions []Condition) bool {
	for _, cond := range conditions {
		switch cond.Type {
		case ConditionLastAccess:
			days := time.Since(file.LastAccessed).Hours() / 24.0
			threshold := cond.Value.(float64)
			switch cond.Operator {
			case ">":
				if days <= threshold {
					return false
				}
			case "<":
				if days >= threshold {
					return false
				}
			}

		case ConditionLastModified:
			days := time.Since(file.LastModified).Hours() / 24.0
			threshold := cond.Value.(float64)
			switch cond.Operator {
			case ">":
				if days <= threshold {
					return false
				}
			case "<":
				if days >= threshold {
					return false
				}
			}

		case ConditionFileSize:
			threshold := cond.Value.(int64)
			switch cond.Operator {
			case ">":
				if file.Size <= threshold {
					return false
				}
			case "<":
				if file.Size >= threshold {
					return false
				}
			}

		case ConditionAccessCount:
			threshold := cond.Value.(int64)
			switch cond.Operator {
			case "<":
				if file.AccessCount >= threshold {
					return false
				}
			}

		case ConditionFileExtension:
			extensions := cond.Value.([]string)
			found := false
			for _, ext := range extensions {
				if file.Extension == ext {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// ExecuteMigration 执行迁移
func (te *TieringEngine) ExecuteMigration(taskID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	var task *MigrationTask
	for _, t := range te.migrationQ {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		return ErrMigrationFailed
	}

	file, exists := te.files[task.FileID]
	if !exists {
		return ErrFileNotFound
	}

	sourceTier, exists := te.tiers[task.SourceTierID]
	if !exists {
		return ErrTierNotFound
	}

	targetTier, exists := te.tiers[task.TargetTierID]
	if !exists {
		return ErrTierNotFound
	}

	// 检查空间
	if targetTier.AvailableSpace < file.Size {
		return ErrInsufficientSpace
	}

	// 执行迁移
	task.Status = TaskStatusRunning
	task.StartedAt = time.Now()

	// 更新存储层
	sourceTier.UsedCapacity -= file.Size
	sourceTier.AvailableSpace += file.Size
	sourceTier.FileCount--

	targetTier.UsedCapacity += file.Size
	targetTier.AvailableSpace -= file.Size
	targetTier.FileCount++

	// 更新文件
	file.OriginalTierID = file.CurrentTierID
	file.CurrentTierID = task.TargetTierID
	file.UpdatedAt = time.Now()

	// 更新策略统计
	if policy, exists := te.policies[task.ID]; exists {
		policy.FilesMigrated++
		policy.BytesMigrated += file.Size
	}

	task.Status = TaskStatusCompleted
	task.CompletedAt = time.Now()
	task.BytesTransfer = file.Size
	task.Duration = task.CompletedAt.Sub(task.StartedAt).Milliseconds()

	te.stats.TotalMigrations++

	return nil
}

// GetMigrationQueue 获取迁移队列
func (te *TieringEngine) GetMigrationQueue() []*MigrationTask {
	te.mu.RLock()
	defer te.mu.RUnlock()

	return te.migrationQ
}

// GetFilesByTier 获取指定层的文件
func (te *TieringEngine) GetFilesByTier(tierID string) []*FileMetadata {
	te.mu.RLock()
	defer te.mu.RUnlock()

	var files []*FileMetadata
	for _, file := range te.files {
		if file.CurrentTierID == tierID {
			files = append(files, file)
		}
	}

	return files
}

// GetHotFiles 获取热文件
func (te *TieringEngine) GetHotFiles(limit int) []*FileMetadata {
	te.mu.RLock()
	defer te.mu.RUnlock()

	type scoredFile struct {
		file  *FileMetadata
		score float64
	}

	var scored []scoredFile
	for _, file := range te.files {
		scored = append(scored, scoredFile{file: file, score: file.HeatScore})
	}

	// 按热度排序
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 取前N个
	if limit > len(scored) {
		limit = len(scored)
	}

	files := make([]*FileMetadata, limit)
	for i := 0; i < limit; i++ {
		files[i] = scored[i].file
	}

	return files
}

// GetStats 获取统计信息
func (te *TieringEngine) GetStats() *TieringStats {
	te.mu.RLock()
	defer te.mu.RUnlock()

	stats := *te.stats
	stats.TotalTiers = len(te.tiers)
	stats.TotalPolicies = len(te.policies)
	stats.TotalFiles = len(te.files)

	// 计算各层使用情况
	stats.TierUsage = make(map[string]int64)
	stats.TierFiles = make(map[string]int)
	for _, tier := range te.tiers {
		stats.TierUsage[tier.ID] = tier.UsedCapacity
		stats.TierFiles[tier.ID] = tier.FileCount
	}

	// 计算平均热度
	totalHeat := 0.0
	for _, file := range te.files {
		totalHeat += file.HeatScore
	}
	if stats.TotalFiles > 0 {
		stats.AvgHeatScore = totalHeat / float64(stats.TotalFiles)
	}

	// 策略统计
	stats.PolicyStats = make(map[string]int64)
	for _, policy := range te.policies {
		stats.PolicyStats[policy.ID] = policy.FilesMigrated
	}

	return &stats
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return "task-" + time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// CostOptimizationReport 成本优化报告
type CostOptimizationReport struct {
	TotalStorageGB    int64              `json:"total_storage_gb"`
	UsedStorageGB     int64              `json:"used_storage_gb"`
	CurrentMonthlyCost float64           `json:"current_monthly_cost"`
	OptimizedCost     float64            `json:"optimized_cost"`
	PotentialSavings  float64            `json:"potential_savings"`
	SavingsPercent    float64            `json:"savings_percent"`
	TierBreakdown     map[string]*TierCostInfo `json:"tier_breakdown"`
	Recommendations   []string           `json:"recommendations"`
}

// TierCostInfo 层级成本信息
type TierCostInfo struct {
	TierName     string  `json:"tier_name"`
	StorageGB    int64   `json:"storage_gb"`
	CostPerGB    float64 `json:"cost_per_gb"`
	MonthlyCost  float64 `json:"monthly_cost"`
	FilesCount   int     `json:"files_count"`
}

// GenerateCostReport 生成成本优化报告
func (te *TieringEngine) GenerateCostReport() *CostOptimizationReport {
	te.mu.RLock()
	defer te.mu.RUnlock()

	report := &CostOptimizationReport{
		TierBreakdown:   make(map[string]*TierCostInfo),
		Recommendations: make([]string, 0),
	}

	tierFileCounts := make(map[string]int)
	for _, file := range te.files {
		tierFileCounts[file.CurrentTierID]++
	}

	for _, tier := range te.tiers {
		info := &TierCostInfo{
			TierName:    tier.Name,
			StorageGB:   tier.UsedCapacity / (1024 * 1024 * 1024),
			CostPerGB:   tier.CostPerGB,
			MonthlyCost: float64(tier.UsedCapacity) / (1024 * 1024 * 1024) * tier.CostPerGB,
			FilesCount:  tierFileCounts[tier.ID],
		}
		report.TierBreakdown[tier.ID] = info
		report.TotalStorageGB += tier.TotalCapacity / (1024 * 1024 * 1024)
		report.UsedStorageGB += tier.UsedCapacity / (1024 * 1024 * 1024)
		report.CurrentMonthlyCost += info.MonthlyCost
	}

	// 计算优化后成本（将冷数据迁移到更便宜的层级）
	optimizedCost := report.CurrentMonthlyCost
	for _, task := range te.migrationQ {
		if task.Status != TaskStatusPending {
			continue
		}
		currentTier, ok1 := te.tiers[task.SourceTierID]
		if !ok1 {
			continue
		}
		targetTier, ok2 := te.tiers[task.TargetTierID]
		if !ok2 {
			continue
		}
		fileSizeGB := float64(task.BytesTransfer) / (1024 * 1024 * 1024)
		if fileSizeGB == 0 {
			if file, ok := te.files[task.FileID]; ok {
				fileSizeGB = float64(file.Size) / (1024 * 1024 * 1024)
			}
		}
		if targetTier.CostPerGB < currentTier.CostPerGB {
			optimizedCost -= fileSizeGB * (currentTier.CostPerGB - targetTier.CostPerGB)
		}
	}

	report.OptimizedCost = optimizedCost
	report.PotentialSavings = report.CurrentMonthlyCost - optimizedCost
	if report.CurrentMonthlyCost > 0 {
		report.SavingsPercent = (report.PotentialSavings / report.CurrentMonthlyCost) * 100
	}

	// 生成建议
	if report.SavingsPercent > 5 {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("建议将 %.1f%% 的冷数据迁移到归档层，可节省 $%.2f/月", report.SavingsPercent, report.PotentialSavings))
	}

	hotDataGB := float64(0)
	for _, file := range te.files {
		if file.HeatScore > 0.7 {
			hotDataGB += float64(file.Size) / (1024 * 1024 * 1024)
		}
	}
	if hotDataGB > 0 {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("热数据共 %.1f GB，建议使用 NVMe SSD 存储以提升性能", hotDataGB))
	}

	return report
}

// PredictiveTiering 预测性分层 - 基于历史访问模式预测文件热度
func (te *TieringEngine) PredictiveTiering() map[string]string {
	te.mu.RLock()
	defer te.mu.RUnlock()

	result := make(map[string]string)
	now := time.Now()

	for _, file := range te.files {
		if file.LastAccessed.IsZero() {
			result[file.ID] = te.defaultColdTier()
			continue
		}

		daysSinceAccess := now.Sub(file.LastAccessed).Hours() / 24
		accessFrequency := float64(file.AccessCount)

		// 预测未来7天热度
		predictedHeat := accessFrequency / (daysSinceAccess + 1)

		var targetTier string
		switch {
		case predictedHeat > 10:
			targetTier = te.defaultHotTier()
		case predictedHeat > 3:
			targetTier = te.defaultWarmTier()
		case predictedHeat > 0.5:
			targetTier = te.defaultColdTier()
		default:
			targetTier = te.defaultArchiveTier()
		}

		result[file.ID] = targetTier
	}

	return result
}

func (te *TieringEngine) defaultHotTier() string {
	for _, t := range te.tiers {
		if t.PerformanceLevel >= 3 {
			return t.ID
		}
	}
	return ""
}

func (te *TieringEngine) defaultWarmTier() string {
	for _, t := range te.tiers {
		if t.PerformanceLevel == 2 {
			return t.ID
		}
	}
	return te.defaultHotTier()
}

func (te *TieringEngine) defaultColdTier() string {
	for _, t := range te.tiers {
		if t.PerformanceLevel == 1 {
			return t.ID
		}
	}
	return ""
}

func (te *TieringEngine) defaultArchiveTier() string {
	for _, t := range te.tiers {
		if t.PerformanceLevel == 0 {
			return t.ID
		}
	}
	return te.defaultColdTier()
}
