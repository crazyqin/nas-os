// Package tiering 提供存储分层管理功能
// 支持热数据自动迁移到SSD，冷数据自动迁移到HDD
// 参考：TrueNAS Electric Eel, 群晖DSM 7.3, 飞牛fnOS
package tiering

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TierType 存储层级类型.
type TierType string

const (
	// TierTypeSSD SSD缓存层（热数据）.
	TierTypeSSD TierType = "ssd"
	// TierTypeHDD HDD存储层（冷数据）.
	TierTypeHDD TierType = "hdd"
	// TierTypeCloud 云存储归档层.
	TierTypeCloud TierType = "cloud"
)

// AccessFrequency 访问频率级别.
type AccessFrequency string

const (
	// AccessFrequencyHot 热数据：频繁访问.
	AccessFrequencyHot AccessFrequency = "hot"
	// AccessFrequencyWarm 温数据：偶尔访问.
	AccessFrequencyWarm AccessFrequency = "warm"
	// AccessFrequencyCold 冷数据：很少访问.
	AccessFrequencyCold AccessFrequency = "cold"
)

// TierConfig 存储层配置.
type TierConfig struct {
	Type        TierType `json:"type"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`        // 存储路径
	Capacity    int64    `json:"capacity"`    // 容量（字节）
	Used        int64    `json:"used"`        // 已使用（字节）
	Threshold   int      `json:"threshold"`   // 使用阈值（%）
	Priority    int      `json:"priority"`    // 优先级（越大越高）
	Enabled     bool     `json:"enabled"`     // 是否启用
	AutoPromote bool     `json:"autoPromote"` // 自动提升热数据
	AutoDemote  bool     `json:"autoDemote"`  // 自动降级冷数据
}

// PolicyConfig 分层策略配置.
type PolicyConfig struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Enabled         bool          `json:"enabled"`
	SourceTier      TierType      `json:"sourceTier"`
	TargetTier      TierType      `json:"targetTier"`
	MinAccessCount  int64         `json:"minAccessCount"` // 热数据阈值
	MaxAccessAge    time.Duration `json:"maxAccessAge"`   // 冷数据判断时长
	MinFileSize     int64         `json:"minFileSize"`
	MaxFileSize     int64         `json:"maxFileSize"`
	FilePatterns    []string      `json:"filePatterns"`
	ExcludePatterns []string      `json:"excludePatterns"`
	Schedule        string        `json:"schedule"` // cron表达式
	LastRun         time.Time     `json:"lastRun"`
	NextRun         time.Time     `json:"nextRun"`
}

// FileAccessRecord 文件访问记录.
type FileAccessRecord struct {
	Path        string          `json:"path"`
	Size        int64           `json:"size"`
	ModTime     time.Time       `json:"modTime"`
	AccessTime  time.Time       `json:"accessTime"`
	AccessCount int64           `json:"accessCount"`
	ReadBytes   int64           `json:"readBytes"`
	WriteBytes  int64           `json:"writeBytes"`
	CurrentTier TierType        `json:"currentTier"`
	Frequency   AccessFrequency `json:"frequency"`
}

// MigrateTask 迁移任务.
type MigrateTask struct {
	ID             string         `json:"id"`
	PolicyID       string         `json:"policyId,omitempty"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
	StartedAt      time.Time      `json:"startedAt,omitempty"`
	CompletedAt    time.Time      `json:"completedAt,omitempty"`
	SourceTier     TierType       `json:"sourceTier"`
	TargetTier     TierType       `json:"targetTier"`
	TotalFiles     int64          `json:"totalFiles"`
	TotalBytes     int64          `json:"totalBytes"`
	ProcessedFiles int64          `json:"processedFiles"`
	ProcessedBytes int64          `json:"processedBytes"`
	FailedFiles    int64          `json:"failedFiles"`
	Files          []MigrateFile  `json:"files,omitempty"`
	Errors         []MigrateError `json:"errors,omitempty"`
}

// MigrateFile 迁移文件.
type MigrateFile struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Status  string    `json:"status"`
	Error   string    `json:"error,omitempty"`
}

// MigrateError 迁移错误.
type MigrateError struct {
	Path    string    `json:"path"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	CheckInterval  time.Duration `json:"checkInterval"`  // 检查间隔
	HotThreshold   int64         `json:"hotThreshold"`   // 热数据访问次数阈值
	WarmThreshold  int64         `json:"warmThreshold"`  // 温数据访问次数阈值
	ColdAgeHours   int           `json:"coldAgeHours"`   // 冷数据判断时长（小时）
	MaxConcurrent  int           `json:"maxConcurrent"`  // 最大并发迁移数
	EnableAutoTier bool          `json:"enableAutoTier"` // 启用自动分层
	ConfigPath     string        `json:"configPath"`     // 配置文件路径
}

// DefaultManagerConfig 默认配置.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		CheckInterval:  1 * time.Hour,
		HotThreshold:   100,
		WarmThreshold:  10,
		ColdAgeHours:   720, // 30天
		MaxConcurrent:  5,
		EnableAutoTier: true,
		ConfigPath:     "/etc/nas-os/tiering.json",
	}
}

// Manager 存储分层管理器.
type Manager struct {
	mu sync.RWMutex

	config ManagerConfig

	// 存储层
	tiers map[TierType]*TierConfig

	// 策略
	policies map[string]*PolicyConfig

	// 文件访问记录
	records map[string]*FileAccessRecord

	// 按存储层索引
	recordsByTier map[TierType]map[string]*FileAccessRecord

	// 迁移任务
	tasks map[string]*MigrateTask

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 日志
	logger *slog.Logger

	// 回调
	onMigrationComplete func(task *MigrateTask)
}

// NewManager 创建存储分层管理器.
func NewManager(config ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:        config,
		tiers:         make(map[TierType]*TierConfig),
		policies:      make(map[string]*PolicyConfig),
		records:       make(map[string]*FileAccessRecord),
		recordsByTier: make(map[TierType]map[string]*FileAccessRecord),
		tasks:         make(map[string]*MigrateTask),
		ctx:           ctx,
		cancel:        cancel,
		logger:        slog.Default(),
	}
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 加载配置
	if err := m.loadConfig(); err != nil {
		m.logger.Info("使用默认配置", "error", err)
		m.initDefaultTiers()
		m.initDefaultPolicies()
	}

	// 初始化存储层索引
	for tier := range m.tiers {
		if m.recordsByTier[tier] == nil {
			m.recordsByTier[tier] = make(map[string]*FileAccessRecord)
		}
	}

	return nil
}

// initDefaultTiers 初始化默认存储层.
func (m *Manager) initDefaultTiers() {
	m.tiers = map[TierType]*TierConfig{
		TierTypeSSD: {
			Type:        TierTypeSSD,
			Name:        "SSD缓存层",
			Path:        "/mnt/ssd",
			Priority:    100,
			Enabled:     true,
			AutoPromote: true,
			Threshold:   80,
		},
		TierTypeHDD: {
			Type:       TierTypeHDD,
			Name:       "HDD存储层",
			Path:       "/mnt/hdd",
			Priority:   50,
			Enabled:    true,
			AutoDemote: true,
			Threshold:  90,
		},
		TierTypeCloud: {
			Type:      TierTypeCloud,
			Name:      "云存储归档层",
			Path:      "/mnt/cloud",
			Priority:  10,
			Enabled:   false,
			Threshold: 95,
		},
	}
}

// initDefaultPolicies 初始化默认策略.
func (m *Manager) initDefaultPolicies() {
	m.policies = map[string]*PolicyConfig{
		"hot-to-ssd": {
			ID:             "hot-to-ssd",
			Name:           "热数据自动提升到SSD",
			Enabled:        true,
			SourceTier:     TierTypeHDD,
			TargetTier:     TierTypeSSD,
			MinAccessCount: m.config.HotThreshold,
			MaxAccessAge:   24 * time.Hour,
			Schedule:       "0 * * * *", // 每小时执行
		},
		"cold-to-hdd": {
			ID:             "cold-to-hdd",
			Name:           "冷数据自动降级到HDD",
			Enabled:        true,
			SourceTier:     TierTypeSSD,
			TargetTier:     TierTypeHDD,
			MaxAccessAge:   time.Duration(m.config.ColdAgeHours) * time.Hour,
			MinAccessCount: 0,
			Schedule:       "0 3 * * *", // 每天凌晨3点执行
		},
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	// 启动策略引擎
	if m.config.EnableAutoTier {
		go m.runPolicyEngine()
	}

	m.logger.Info("分层存储管理器已启动")
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	m.cancel()

	// 保存配置
	_ = m.saveConfig()

	m.logger.Info("分层存储管理器已停止")
}

// ==================== 存储层管理 ====================

// CreateTier 创建存储层.
func (m *Manager) CreateTier(config TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tiers[config.Type]; exists {
		return fmt.Errorf("存储层已存在: %s", config.Type)
	}

	m.tiers[config.Type] = &config
	m.recordsByTier[config.Type] = make(map[string]*FileAccessRecord)

	return m.saveConfigLocked()
}

// GetTier 获取存储层配置.
func (m *Manager) GetTier(tierType TierType) (*TierConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tier, exists := m.tiers[tierType]
	if !exists {
		return nil, fmt.Errorf("存储层不存在: %s", tierType)
	}

	return tier, nil
}

// UpdateTier 更新存储层配置.
func (m *Manager) UpdateTier(config TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tiers[config.Type]; !exists {
		return fmt.Errorf("存储层不存在: %s", config.Type)
	}

	m.tiers[config.Type] = &config
	return m.saveConfigLocked()
}

// DeleteTier 删除存储层.
func (m *Manager) DeleteTier(tierType TierType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tiers[tierType]; !exists {
		return fmt.Errorf("存储层不存在: %s", tierType)
	}

	delete(m.tiers, tierType)
	delete(m.recordsByTier, tierType)

	return m.saveConfigLocked()
}

// ListTiers 列出所有存储层.
func (m *Manager) ListTiers() []*TierConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TierConfig, 0, len(m.tiers))
	for _, tier := range m.tiers {
		result = append(result, tier)
	}
	return result
}

// ==================== 策略管理 ====================

// CreatePolicy 创建分层策略.
func (m *Manager) CreatePolicy(config PolicyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = "policy_" + uuid.New().String()[:8]
	}

	if _, exists := m.policies[config.ID]; exists {
		return fmt.Errorf("策略已存在: %s", config.ID)
	}

	// 验证存储层
	if _, exists := m.tiers[config.SourceTier]; !exists {
		return fmt.Errorf("源存储层不存在: %s", config.SourceTier)
	}
	if _, exists := m.tiers[config.TargetTier]; !exists {
		return fmt.Errorf("目标存储层不存在: %s", config.TargetTier)
	}

	m.policies[config.ID] = &config
	return m.saveConfigLocked()
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*PolicyConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	return policy, nil
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(config PolicyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[config.ID]; !exists {
		return fmt.Errorf("策略不存在: %s", config.ID)
	}

	m.policies[config.ID] = &config
	return m.saveConfigLocked()
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)
	return m.saveConfigLocked()
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*PolicyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PolicyConfig, 0, len(m.policies))
	for _, policy := range m.policies {
		result = append(result, policy)
	}
	return result
}

// ==================== 文件访问追踪 ====================

// RecordAccess 记录文件访问.
func (m *Manager) RecordAccess(path string, tier TierType, readBytes, writeBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[path]
	if !exists {
		record = &FileAccessRecord{
			Path:        path,
			CurrentTier: tier,
			AccessTime:  time.Now(),
		}

		// 获取文件信息
		if info, err := os.Stat(path); err == nil {
			record.Size = info.Size()
			record.ModTime = info.ModTime()
		}

		m.records[path] = record

		// 更新存储层索引
		if m.recordsByTier[tier] == nil {
			m.recordsByTier[tier] = make(map[string]*FileAccessRecord)
		}
		m.recordsByTier[tier][path] = record
	}

	// 更新访问统计
	record.AccessCount++
	record.ReadBytes += readBytes
	record.WriteBytes += writeBytes
	record.AccessTime = time.Now()

	// 更新访问频率
	record.Frequency = m.calculateFrequency(record)

	return nil
}

// GetRecord 获取文件访问记录.
func (m *Manager) GetRecord(path string) (*FileAccessRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[path]
	if !exists {
		return nil, fmt.Errorf("文件访问记录不存在: %s", path)
	}

	return record, nil
}

// GetHotFiles 获取热数据文件.
func (m *Manager) GetHotFiles(tier TierType, limit int) []*FileAccessRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []*FileAccessRecord
	records, exists := m.recordsByTier[tier]
	if !exists {
		return files
	}

	for _, record := range records {
		if record.Frequency == AccessFrequencyHot {
			files = append(files, record)
			if limit > 0 && len(files) >= limit {
				break
			}
		}
	}

	return files
}

// GetColdFiles 获取冷数据文件.
func (m *Manager) GetColdFiles(tier TierType, limit int) []*FileAccessRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []*FileAccessRecord
	records, exists := m.recordsByTier[tier]
	if !exists {
		return files
	}

	for _, record := range records {
		if record.Frequency == AccessFrequencyCold {
			files = append(files, record)
			if limit > 0 && len(files) >= limit {
				break
			}
		}
	}

	return files
}

// calculateFrequency 计算访问频率.
func (m *Manager) calculateFrequency(record *FileAccessRecord) AccessFrequency {
	accessCount := record.AccessCount
	age := time.Since(record.AccessTime)

	// 热数据：访问次数高且近期有访问
	if accessCount >= m.config.HotThreshold && age < 24*time.Hour {
		return AccessFrequencyHot
	}

	// 冷数据：长时间未访问
	coldDuration := time.Duration(m.config.ColdAgeHours) * time.Hour
	if age > coldDuration {
		return AccessFrequencyCold
	}

	// 温数据
	return AccessFrequencyWarm
}

// ==================== 迁移操作 ====================

// MigrateHotToSSD 迁移热数据到SSD.
func (m *Manager) MigrateHotToSSD(ctx context.Context) (*MigrateTask, error) {
	m.mu.RLock()
	ssdTier, ssdExists := m.tiers[TierTypeSSD]
	hddTier, hddExists := m.tiers[TierTypeHDD]
	m.mu.RUnlock()

	if !ssdExists || !ssdTier.Enabled {
		return nil, fmt.Errorf("SSD缓存层未启用")
	}

	if !hddExists || !hddTier.Enabled {
		return nil, fmt.Errorf("HDD存储层未启用")
	}

	// 获取HDD上的热数据
	hotFiles := m.GetHotFiles(TierTypeHDD, 0)
	if len(hotFiles) == 0 {
		return nil, fmt.Errorf("没有需要迁移的热数据")
	}

	// 计算SSD可用空间
	ssdAvailable := ssdTier.Capacity - ssdTier.Used
	if ssdAvailable <= 0 {
		return nil, fmt.Errorf("SSD空间不足")
	}

	// 筛选可迁移的文件
	var files []MigrateFile
	var totalSize int64
	for _, record := range hotFiles {
		// 保留20%空间
		if totalSize+record.Size > ssdAvailable*80/100 {
			break
		}
		files = append(files, MigrateFile{
			Path:    record.Path,
			Size:    record.Size,
			ModTime: record.ModTime,
			Status:  "pending",
		})
		totalSize += record.Size
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("没有可迁移的热数据（空间不足）")
	}

	// 创建迁移任务
	task := &MigrateTask{
		ID:         "task_" + uuid.New().String()[:8],
		Status:     "pending",
		CreatedAt:  time.Now(),
		SourceTier: TierTypeHDD,
		TargetTier: TierTypeSSD,
		TotalFiles: int64(len(files)),
		TotalBytes: totalSize,
		Files:      files,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 异步执行迁移
	go m.executeMigration(task, ssdTier.Path, true)

	return task, nil
}

// MigrateColdToHDD 迁移冷数据到HDD.
func (m *Manager) MigrateColdToHDD(ctx context.Context) (*MigrateTask, error) {
	m.mu.RLock()
	ssdTier, ssdExists := m.tiers[TierTypeSSD]
	hddTier, hddExists := m.tiers[TierTypeHDD]
	m.mu.RUnlock()

	if !ssdExists || !ssdTier.Enabled {
		return nil, fmt.Errorf("SSD缓存层未启用")
	}

	if !hddExists || !hddTier.Enabled {
		return nil, fmt.Errorf("HDD存储层未启用")
	}

	// 获取SSD上的冷数据
	coldFiles := m.GetColdFiles(TierTypeSSD, 0)
	if len(coldFiles) == 0 {
		return nil, fmt.Errorf("没有需要迁移的冷数据")
	}

	// 创建迁移文件列表
	var files []MigrateFile
	var totalSize int64
	for _, record := range coldFiles {
		files = append(files, MigrateFile{
			Path:    record.Path,
			Size:    record.Size,
			ModTime: record.ModTime,
			Status:  "pending",
		})
		totalSize += record.Size
	}

	// 创建迁移任务
	task := &MigrateTask{
		ID:         "task_" + uuid.New().String()[:8],
		Status:     "pending",
		CreatedAt:  time.Now(),
		SourceTier: TierTypeSSD,
		TargetTier: TierTypeHDD,
		TotalFiles: int64(len(files)),
		TotalBytes: totalSize,
		Files:      files,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 异步执行迁移
	go m.executeMigration(task, hddTier.Path, false)

	return task, nil
}

// executeMigration 执行迁移.
func (m *Manager) executeMigration(task *MigrateTask, targetPath string, preserveSource bool) {
	m.mu.Lock()
	task.Status = "running"
	task.StartedAt = time.Now()
	m.mu.Unlock()

	for i := range task.Files {
		file := &task.Files[i]

		// 构建目标路径（简化处理：直接使用文件名）
		targetFilePath := filepath.Join(targetPath, filepath.Base(file.Path))

		// 执行文件复制
		err := m.copyFile(file.Path, targetFilePath)
		if err != nil {
			file.Status = "failed"
			file.Error = err.Error()
			task.FailedFiles++
			task.Errors = append(task.Errors, MigrateError{
				Path:    file.Path,
				Message: err.Error(),
				Time:    time.Now(),
			})
		} else {
			file.Status = "completed"

			// 如果不保留源文件，则删除
			if !preserveSource {
				if err := os.Remove(file.Path); err != nil {
					m.logger.Warn("删除源文件失败", "path", file.Path, "error", err)
				}
			}

			// 更新文件访问记录的存储层
			m.mu.Lock()
			if record, exists := m.records[file.Path]; exists {
				oldTier := record.CurrentTier
				record.CurrentTier = task.TargetTier

				// 更新存储层索引
				if m.recordsByTier[oldTier] != nil {
					delete(m.recordsByTier[oldTier], file.Path)
				}
				if m.recordsByTier[task.TargetTier] == nil {
					m.recordsByTier[task.TargetTier] = make(map[string]*FileAccessRecord)
				}
				m.recordsByTier[task.TargetTier][file.Path] = record
			}
			m.mu.Unlock()
		}

		m.mu.Lock()
		task.ProcessedFiles++
		task.ProcessedBytes += file.Size
		m.mu.Unlock()
	}

	// 更新任务状态
	m.mu.Lock()
	task.CompletedAt = time.Now()
	if task.FailedFiles == task.TotalFiles {
		task.Status = "failed"
	} else if task.FailedFiles > 0 {
		task.Status = "partial"
	} else {
		task.Status = "completed"
	}
	m.mu.Unlock()

	// 回调
	if m.onMigrationComplete != nil {
		m.onMigrationComplete(task)
	}

	m.logger.Info("迁移任务完成",
		"taskId", task.ID,
		"totalFiles", task.TotalFiles,
		"processedFiles", task.ProcessedFiles,
		"failedFiles", task.FailedFiles,
	)
}

// copyFile 复制文件.
func (m *Manager) copyFile(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer func() { _ = srcFile.Close() }() //nolint:errcheck

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer func() { _ = dstFile.Close() }() //nolint:errcheck

	// 复制内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 同步到磁盘
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("同步文件失败: %w", err)
	}

	// 获取源文件信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败: %w", err)
	}

	// 设置目标文件权限
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	return nil
}

// GetTask 获取迁移任务.
func (m *Manager) GetTask(id string) (*MigrateTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}

	return task, nil
}

// ListTasks 列出迁移任务.
func (m *Manager) ListTasks(limit int) []*MigrateTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrateTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		result = append(result, task)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ==================== 策略引擎 ====================

// runPolicyEngine 运行策略引擎.
func (m *Manager) runPolicyEngine() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.executeAutoPolicies()
		}
	}
}

// executeAutoPolicies 执行自动策略.
func (m *Manager) executeAutoPolicies() {
	m.mu.RLock()
	policies := make([]*PolicyConfig, 0)
	for _, policy := range m.policies {
		if policy.Enabled && (policy.NextRun.IsZero() || time.Now().After(policy.NextRun)) {
			policies = append(policies, policy)
		}
	}
	m.mu.RUnlock()

	for _, policy := range policies {
		var task *MigrateTask
		var err error

		switch policy.ID {
		case "hot-to-ssd":
			task, err = m.MigrateHotToSSD(m.ctx)
		case "cold-to-hdd":
			task, err = m.MigrateColdToHDD(m.ctx)
		default:
			continue
		}

		if err != nil {
			m.logger.Warn("策略执行失败", "policyId", policy.ID, "error", err)
		} else {
			m.logger.Info("策略已触发", "policyId", policy.ID, "taskId", task.ID)
		}

		// 更新下次执行时间
		m.mu.Lock()
		if p, exists := m.policies[policy.ID]; exists {
			p.LastRun = time.Now()
			p.NextRun = time.Now().Add(m.config.CheckInterval)
		}
		m.mu.Unlock()
	}
}

// ==================== 统计 ====================

// GetStatus 获取分层状态.
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 统计运行中的任务
	var running, pending int
	for _, task := range m.tasks {
		switch task.Status {
		case "running":
			running++
		case "pending":
			pending++
		}
	}

	// 统计活跃策略
	var activePolicy int
	for _, policy := range m.policies {
		if policy.Enabled {
			activePolicy++
		}
	}

	// 统计热/冷数据
	var hotFiles, coldFiles int64
	for _, record := range m.records {
		switch record.Frequency {
		case AccessFrequencyHot:
			hotFiles++
		case AccessFrequencyCold:
			coldFiles++
		}
	}

	return map[string]interface{}{
		"enabled":        m.config.EnableAutoTier,
		"runningTasks":   running,
		"pendingTasks":   pending,
		"totalPolicies":  len(m.policies),
		"activePolicies": activePolicy,
		"hotFiles":       hotFiles,
		"coldFiles":      coldFiles,
		"totalRecords":   len(m.records),
	}
}

// GetTierStats 获取存储层统计.
func (m *Manager) GetTierStats(tierType TierType) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tier, exists := m.tiers[tierType]
	if !exists {
		return nil, fmt.Errorf("存储层不存在: %s", tierType)
	}

	records := m.recordsByTier[tierType]

	var hotFiles, warmFiles, coldFiles int64
	var hotBytes, warmBytes, coldBytes int64

	for _, record := range records {
		switch record.Frequency {
		case AccessFrequencyHot:
			hotFiles++
			hotBytes += record.Size
		case AccessFrequencyWarm:
			warmFiles++
			warmBytes += record.Size
		case AccessFrequencyCold:
			coldFiles++
			coldBytes += record.Size
		}
	}

	return map[string]interface{}{
		"type":       tierType,
		"name":       tier.Name,
		"path":       tier.Path,
		"enabled":    tier.Enabled,
		"capacity":   tier.Capacity,
		"used":       tier.Used,
		"threshold":  tier.Threshold,
		"totalFiles": len(records),
		"hotFiles":   hotFiles,
		"warmFiles":  warmFiles,
		"coldFiles":  coldFiles,
		"hotBytes":   hotBytes,
		"warmBytes":  warmBytes,
		"coldBytes":  coldBytes,
	}, nil
}

// SetMigrationCallback 设置迁移完成回调.
func (m *Manager) SetMigrationCallback(callback func(task *MigrateTask)) {
	m.onMigrationComplete = callback
}

// ==================== 配置持久化 ====================

type configData struct {
	Tiers    map[TierType]*TierConfig `json:"tiers"`
	Policies map[string]*PolicyConfig `json:"policies"`
	Config   ManagerConfig            `json:"config"`
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		return err
	}

	var cfg configData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.tiers = cfg.Tiers
	m.policies = cfg.Policies
	if cfg.Config.CheckInterval > 0 {
		m.config = cfg.Config
	}

	return nil
}

func (m *Manager) saveConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveConfigLocked()
}

func (m *Manager) saveConfigLocked() error {
	cfg := configData{
		Tiers:    m.tiers,
		Policies: m.policies,
		Config:   m.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.config.ConfigPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(m.config.ConfigPath, data, 0640)
}
