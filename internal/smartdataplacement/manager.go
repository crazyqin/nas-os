// Package smartdataplacement 提供智能数据放置引擎
// 基于访问频率、文件类型、数据温度自动在多层存储间迁移数据
package smartdataplacement

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrFileNotFound      = errors.New("file not found")
	ErrTierNotFound      = errors.New("storage tier not found")
	ErrNoMigrationNeeded = errors.New("no migration needed")
	ErrInvalidPolicy     = errors.New("invalid placement policy")
)

// DataTemperature 数据温度
type DataTemperature string

const (
	TemperatureHot    DataTemperature = "hot"    // 热数据：频繁访问
	TemperatureWarm   DataTemperature = "warm"   // 温数据：偶尔访问
	TemperatureCold   DataTemperature = "cold"   // 冷数据：很少访问
	TemperatureFrozen DataTemperature = "frozen" // 冻结数据：归档
)

// StorageTier 存储层
type StorageTier string

const (
	TierNVMe  StorageTier = "nvme"  // NVMe SSD
	TierSSD   StorageTier = "ssd"   // SATA SSD
	TierHDD   StorageTier = "hdd"   // 机械硬盘
	TierCloud StorageTier = "cloud" // 云存储
	TierTape  StorageTier = "tape"  // 磁带归档
)

// FileRecord 文件记录
type FileRecord struct {
	FileID       string          `json:"fileId"`
	FilePath     string          `json:"filePath"`
	SizeBytes    int64           `json:"sizeBytes"`
	CurrentTier  StorageTier     `json:"currentTier"`
	Temperature  DataTemperature `json:"temperature"`
	AccessCount  int64           `json:"accessCount"`
	LastAccessed time.Time       `json:"lastAccessed"`
	CreatedAt    time.Time       `json:"createdAt"`
	ModifiedAt   time.Time       `json:"modifiedAt"`
	FileType     string          `json:"fileType"`
	AccessScore  float64         `json:"accessScore"`
}

// PlacementPolicy 放置策略
type PlacementPolicy struct {
	Name                  string                          `json:"name"`
	TierMapping           map[DataTemperature]StorageTier `json:"tierMapping"`
	TemperatureThresholds TemperatureThresholds           `json:"temperatureThresholds"`
	MinFileSize           int64                           `json:"minFileSize"`
	MaxMigrationsPerDay   int                             `json:"maxMigrationsPerDay"`
	CooldownPeriod        time.Duration                   `json:"cooldownPeriod"`
	PriorityFileTypes     []string                        `json:"priorityFileTypes"`
}

// TemperatureThresholds 温度阈值
type TemperatureThresholds struct {
	HotAccessPerDay  float64 `json:"hotAccessPerDay"`  // 每天访问次数 >= 此值为热数据
	WarmAccessPerDay float64 `json:"warmAccessPerDay"` // 每天访问次数 >= 此值为温数据
	ColdAccessPerDay float64 `json:"coldAccessPerDay"` // 每天访问次数 < 此值为冷数据
	DaysToFreeze     int     `json:"daysToFreeze"`     // 多少天不访问变为冻结
}

// MigrationTask 迁移任务
type MigrationTask struct {
	TaskID      string          `json:"taskId"`
	FileID      string          `json:"fileId"`
	FilePath    string          `json:"filePath"`
	SourceTier  StorageTier     `json:"sourceTier"`
	TargetTier  StorageTier     `json:"targetTier"`
	SizeBytes   int64           `json:"sizeBytes"`
	Reason      string          `json:"reason"`
	Priority    int             `json:"priority"`
	CreatedAt   time.Time       `json:"createdAt"`
	Status      MigrationStatus `json:"status"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
}

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationPending   MigrationStatus = "pending"
	MigrationRunning   MigrationStatus = "running"
	MigrationCompleted MigrationStatus = "completed"
	MigrationFailed    MigrationStatus = "failed"
)

// PlacementReport 放置报告
type PlacementReport struct {
	GeneratedAt             time.Time                `json:"generatedAt"`
	TotalFiles              int                      `json:"totalFiles"`
	TotalSizeBytes          int64                    `json:"totalSizeBytes"`
	TierDistribution        map[StorageTier]TierInfo `json:"tierDistribution"`
	TemperatureDistribution map[DataTemperature]int  `json:"temperatureDistribution"`
	PendingMigrations       int                      `json:"pendingMigrations"`
	RecommendedActions      int                      `json:"recommendedActions"`
	CostSavingsEstimate     float64                  `json:"costSavingsEstimate"`
}

// TierInfo 层信息
type TierInfo struct {
	FileCount      int     `json:"fileCount"`
	TotalBytes     int64   `json:"totalBytes"`
	UsedPercent    float64 `json:"usedPercent"`
	CostPerTBMonth float64 `json:"costPerTbMonth"`
}

// Manager 智能数据放置管理器
type Manager struct {
	mu               sync.RWMutex
	config           *Config
	files            map[string]*FileRecord
	tiers            map[StorageTier]*TierConfig
	policy           *PlacementPolicy
	migrations       []MigrationTask
	migrationCounter int64
	running          bool
	stopCh           chan struct{}
	nowFunc          func() time.Time
}

// Config 配置
type Config struct {
	Enabled                 bool          `json:"enabled"`
	AnalysisInterval        time.Duration `json:"analysisInterval"`
	TemperatureWindow       time.Duration `json:"temperatureWindow"`
	MinAccessForScore       int64         `json:"minAccessForScore"`
	AutoMigrate             bool          `json:"autoMigrate"`
	MaxConcurrentMigrations int           `json:"maxConcurrentMigrations"`
}

// TierConfig 层配置
type TierConfig struct {
	Name           string  `json:"name"`
	CapacityBytes  int64   `json:"capacityBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	CostPerTBMonth float64 `json:"costPerTbMonth"`
	ReadSpeedMBps  int     `json:"readSpeedMbps"`
	WriteSpeedMBps int     `json:"writeSpeedMbps"`
	Durability     string  `json:"durability"`
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:                 true,
			AnalysisInterval:        time.Hour,
			TemperatureWindow:       time.Hour * 24 * 30, // 30天
			MinAccessForScore:       1,
			AutoMigrate:             false,
			MaxConcurrentMigrations: 5,
		}
	}

	m := &Manager{
		config:     config,
		files:      make(map[string]*FileRecord),
		tiers:      make(map[StorageTier]*TierConfig),
		migrations: make([]MigrationTask, 0),
		stopCh:     make(chan struct{}),
		nowFunc:    time.Now,
	}

	// 默认放置策略
	m.policy = &PlacementPolicy{
		Name: "balanced",
		TierMapping: map[DataTemperature]StorageTier{
			TemperatureHot:    TierNVMe,
			TemperatureWarm:   TierSSD,
			TemperatureCold:   TierHDD,
			TemperatureFrozen: TierCloud,
		},
		TemperatureThresholds: TemperatureThresholds{
			HotAccessPerDay:  10,
			WarmAccessPerDay: 2,
			ColdAccessPerDay: 0.1,
			DaysToFreeze:     90,
		},
		MaxMigrationsPerDay: 100,
		CooldownPeriod:      time.Hour * 24,
	}

	// 默认层配置
	m.tiers[TierNVMe] = &TierConfig{Name: "NVMe SSD", CapacityBytes: 2 * 1024 * 1024 * 1024 * 1024, CostPerTBMonth: 800, ReadSpeedMBps: 3500, WriteSpeedMBps: 3000}
	m.tiers[TierSSD] = &TierConfig{Name: "SATA SSD", CapacityBytes: 8 * 1024 * 1024 * 1024 * 1024, CostPerTBMonth: 400, ReadSpeedMBps: 550, WriteSpeedMBps: 520}
	m.tiers[TierHDD] = &TierConfig{Name: "HDD", CapacityBytes: 20 * 1024 * 1024 * 1024 * 1024, CostPerTBMonth: 100, ReadSpeedMBps: 200, WriteSpeedMBps: 180}
	m.tiers[TierCloud] = &TierConfig{Name: "Cloud", CapacityBytes: 100 * 1024 * 1024 * 1024 * 1024, CostPerTBMonth: 50, ReadSpeedMBps: 100, WriteSpeedMBps: 50}

	return m
}

// RegisterFile 注册文件
func (m *Manager) RegisterFile(fileID, path string, sizeBytes int64, currentTier StorageTier, fileType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fileID == "" {
		return fmt.Errorf("file id is required")
	}

	m.files[fileID] = &FileRecord{
		FileID:       fileID,
		FilePath:     path,
		SizeBytes:    sizeBytes,
		CurrentTier:  currentTier,
		Temperature:  TemperatureWarm, // 默认温数据
		LastAccessed: m.nowFunc(),
		CreatedAt:    m.nowFunc(),
		ModifiedAt:   m.nowFunc(),
		FileType:     fileType,
	}
	return nil
}

// RecordAccess 记录文件访问
func (m *Manager) RecordAccess(fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, ok := m.files[fileID]
	if !ok {
		return ErrFileNotFound
	}

	file.AccessCount++
	file.LastAccessed = m.nowFunc()

	// 更新温度
	file.Temperature = m.calculateTemperature(file)

	// 更新访问分数
	daysSinceCreation := m.nowFunc().Sub(file.CreatedAt).Hours() / 24
	if daysSinceCreation > 0 {
		file.AccessScore = float64(file.AccessCount) / daysSinceCreation
	}

	return nil
}

// AnalyzePlacement 分析数据放置
func (m *Manager) AnalyzePlacement() (*PlacementReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &PlacementReport{
		GeneratedAt:             m.nowFunc(),
		TierDistribution:        make(map[StorageTier]TierInfo),
		TemperatureDistribution: make(map[DataTemperature]int),
	}

	// 统计各层分布
	for _, file := range m.files {
		report.TotalFiles++
		report.TotalSizeBytes += file.SizeBytes

		// 温度分布
		report.TemperatureDistribution[file.Temperature]++

		// 层分布
		tierInfo := report.TierDistribution[file.CurrentTier]
		tierInfo.FileCount++
		tierInfo.TotalBytes += file.SizeBytes
		report.TierDistribution[file.CurrentTier] = tierInfo
	}

	// 计算层使用率
	for tier, info := range report.TierDistribution {
		if tc, ok := m.tiers[tier]; ok && tc.CapacityBytes > 0 {
			info.UsedPercent = float64(info.TotalBytes) / float64(tc.CapacityBytes) * 100
			info.CostPerTBMonth = tc.CostPerTBMonth
			report.TierDistribution[tier] = info
		}
	}

	// 计算推荐迁移
	recommendations := m.generateRecommendations()
	report.RecommendedActions = len(recommendations)

	// 估算成本节约
	report.CostSavingsEstimate = m.estimateCostSavings(recommendations)

	// 统计待处理迁移
	for _, mig := range m.migrations {
		if mig.Status == MigrationPending {
			report.PendingMigrations++
		}
	}

	return report, nil
}

// PlanMigrations 规划迁移
func (m *Manager) PlanMigrations() ([]MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	recommendations := m.generateRecommendations()

	// 按优先级排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Priority > recommendations[j].Priority
	})

	// 限制每日迁移数
	dailyCount := 0
	today := m.nowFunc().Format("2006-01-02")
	for _, mig := range m.migrations {
		if mig.CreatedAt.Format("2006-01-02") == today {
			dailyCount++
		}
	}

	result := make([]MigrationTask, 0)
	for _, rec := range recommendations {
		if dailyCount >= m.policy.MaxMigrationsPerDay {
			break
		}
		m.migrationCounter++
		rec.TaskID = fmt.Sprintf("mig-%d", m.migrationCounter)
		rec.Status = MigrationPending
		rec.CreatedAt = m.nowFunc()
		m.migrations = append(m.migrations, rec)
		result = append(result, rec)
		dailyCount++
	}

	return result, nil
}

// CompleteMigration 完成迁移
func (m *Manager) CompleteMigration(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.migrations {
		if m.migrations[i].TaskID == taskID {
			now := m.nowFunc()
			m.migrations[i].Status = MigrationCompleted
			m.migrations[i].CompletedAt = &now

			// 更新文件的当前层
			if file, ok := m.files[m.migrations[i].FileID]; ok {
				file.CurrentTier = m.migrations[i].TargetTier
			}
			return nil
		}
	}
	return fmt.Errorf("migration task %s not found", taskID)
}

// GetFile 获取文件信息
func (m *Manager) GetFile(fileID string) (*FileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, ok := m.files[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}
	copy := *file
	return &copy, nil
}

// SetPolicy 设置放置策略
func (m *Manager) SetPolicy(policy *PlacementPolicy) error {
	if policy == nil {
		return ErrInvalidPolicy
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	return nil
}

// GetMigrations 获取迁移任务
func (m *Manager) GetMigrations(status MigrationStatus, limit int) []MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MigrationTask, 0)
	for i := len(m.migrations) - 1; i >= 0; i-- {
		if status != "" && m.migrations[i].Status != status {
			continue
		}
		result = append(result, m.migrations[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetDashboard 获取仪表板
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tempCounts := make(map[DataTemperature]int)
	tierCounts := make(map[StorageTier]int)
	totalSize := int64(0)

	for _, f := range m.files {
		tempCounts[f.Temperature]++
		tierCounts[f.CurrentTier]++
		totalSize += f.SizeBytes
	}

	pendingMigrations := 0
	for _, mig := range m.migrations {
		if mig.Status == MigrationPending {
			pendingMigrations++
		}
	}

	return map[string]interface{}{
		"totalFiles":        len(m.files),
		"totalSizeBytes":    totalSize,
		"temperatureCounts": tempCounts,
		"tierCounts":        tierCounts,
		"pendingMigrations": pendingMigrations,
		"policyName":        m.policy.Name,
		"autoMigrate":       m.config.AutoMigrate,
	}
}

// 内部方法

func (m *Manager) calculateTemperature(file *FileRecord) DataTemperature {
	daysSinceAccess := m.nowFunc().Sub(file.LastAccessed).Hours() / 24

	if daysSinceAccess >= float64(m.policy.TemperatureThresholds.DaysToFreeze) {
		return TemperatureFrozen
	}

	// 计算每天访问频率
	daysSinceCreation := m.nowFunc().Sub(file.CreatedAt).Hours() / 24
	if daysSinceCreation < 1 {
		daysSinceCreation = 1
	}
	accessPerDay := float64(file.AccessCount) / daysSinceCreation

	if accessPerDay >= m.policy.TemperatureThresholds.HotAccessPerDay {
		return TemperatureHot
	}
	if accessPerDay >= m.policy.TemperatureThresholds.WarmAccessPerDay {
		return TemperatureWarm
	}
	return TemperatureCold
}

func (m *Manager) generateRecommendations() []MigrationTask {
	recommendations := make([]MigrationTask, 0)

	for _, file := range m.files {
		optimalTier, ok := m.policy.TierMapping[file.Temperature]
		if !ok {
			continue
		}

		if optimalTier == file.CurrentTier {
			continue
		}

		// 检查目标层容量
		if tc, ok := m.tiers[optimalTier]; ok {
			if tc.UsedBytes+file.SizeBytes > tc.CapacityBytes {
				continue // 目标层已满
			}
		}

		priority := m.calculatePriority(file, optimalTier)

		recommendations = append(recommendations, MigrationTask{
			FileID:     file.FileID,
			FilePath:   file.FilePath,
			SourceTier: file.CurrentTier,
			TargetTier: optimalTier,
			SizeBytes:  file.SizeBytes,
			Reason:     fmt.Sprintf("数据温度为 %s，建议放置到 %s 层", file.Temperature, optimalTier),
			Priority:   priority,
		})
	}

	return recommendations
}

func (m *Manager) calculatePriority(file *FileRecord, targetTier StorageTier) int {
	priority := 50 // 基础优先级

	// 温度差异越大优先级越高
	tempOrder := map[DataTemperature]int{
		TemperatureHot:    4,
		TemperatureWarm:   3,
		TemperatureCold:   2,
		TemperatureFrozen: 1,
	}
	currentOrder := tempOrder[file.Temperature]
	targetOrder := tempOrder[TemperatureCold] // 默认
	switch targetTier {
	case TierNVMe:
		targetOrder = 4
	case TierSSD:
		targetOrder = 3
	case TierHDD:
		targetOrder = 2
	case TierCloud, TierTape:
		targetOrder = 1
	}

	diff := currentOrder - targetOrder
	if diff < 0 {
		diff = -diff
	}
	priority += diff * 10

	// 文件越大优先级越高（减少频繁迁移大文件）
	if file.SizeBytes > 10*1024*1024*1024 { // >10GB
		priority += 20
	}

	// 长时间未访问的冷数据优先迁移
	daysSinceAccess := m.nowFunc().Sub(file.LastAccessed).Hours() / 24
	if daysSinceAccess > 30 {
		priority += 15
	}

	return priority
}

func (m *Manager) estimateCostSavings(recommendations []MigrationTask) float64 {
	savings := 0.0
	for _, rec := range recommendations {
		sourceCost := 0.0
		targetCost := 0.0
		if tc, ok := m.tiers[rec.SourceTier]; ok {
			sourceCost = tc.CostPerTBMonth
		}
		if tc, ok := m.tiers[rec.TargetTier]; ok {
			targetCost = tc.CostPerTBMonth
		}
		sizeTB := float64(rec.SizeBytes) / (1024 * 1024 * 1024 * 1024)
		savings += (sourceCost - targetCost) * sizeTB
	}
	return savings
}
