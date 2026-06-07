// Package storagetiering 智能存储分层引擎管理器
package storagetiering

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 预定义错误
var (
	ErrStorageTieringDisabled = fmt.Errorf("storage tiering is disabled")
	ErrFileNotFound           = fmt.Errorf("file not found")
	ErrFilePinned             = fmt.Errorf("file is pinned to current tier")
	ErrAlreadyInTier          = fmt.Errorf("file is already in target tier")
	ErrFileTooSmall           = fmt.Errorf("file size below minimum threshold")
	ErrFileTooLarge           = fmt.Errorf("file size exceeds maximum threshold")
	ErrNoStoragePool          = fmt.Errorf("no storage pool available for tier")
	ErrRuleNotFound           = fmt.Errorf("tiering rule not found")
	ErrPoolNotFound           = fmt.Errorf("storage pool not found")
	ErrTaskNotFound           = fmt.Errorf("migration task not found")
	ErrTaskAlreadyCompleted   = fmt.Errorf("task already completed")
	ErrAutoTieringDisabled    = fmt.Errorf("auto tiering is disabled")
)

// Manager 存储分层管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	config    *StorageTieringConfig
	analyzer  *Analyzer
	scheduler *Scheduler
	files     map[string]*FileMetadata
	pools     map[string]*StoragePool
	rules     map[string]*TieringRule
}

// NewManager 创建存储分层管理器
func NewManager(logger *zap.Logger, config *StorageTieringConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultStorageTieringConfig()
	}

	analyzer := NewAnalyzer(config)
	scheduler := NewScheduler(logger, config, analyzer)

	m := &Manager{
		logger:    logger,
		config:    config,
		analyzer:  analyzer,
		scheduler: scheduler,
		files:     make(map[string]*FileMetadata),
		pools:     make(map[string]*StoragePool),
		rules:     make(map[string]*TieringRule),
	}

	// 初始化默认存储池
	m.initDefaultPools()
	// 初始化默认规则
	m.initDefaultRules()

	return m
}

// initDefaultPools 初始化默认存储池
func (m *Manager) initDefaultPools() {
	defaultPools := []*StoragePool{
		{
			ID:             "pool-ssd",
			Name:           "SSD热存储池",
			Type:           StoragePoolSSD,
			Tier:           TierLevelHot,
			CapacityBytes:  2 * 1024 * 1024 * 1024 * 1024, // 2TB
			UsedBytes:      500 * 1024 * 1024 * 1024,      // 500GB
			AvailableBytes: 1500 * 1024 * 1024 * 1024,     // 1.5TB
			IsActive:       true,
			ReadSpeedMBs:   3500,
			WriteSpeedMBs:  3000,
			IOPS:           500000,
			LatencyMs:      0.1,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "pool-hdd",
			Name:           "HDD温存储池",
			Type:           StoragePoolHDD,
			Tier:           TierLevelWarm,
			CapacityBytes:  10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedBytes:      3 * 1024 * 1024 * 1024 * 1024,  // 3TB
			AvailableBytes: 7 * 1024 * 1024 * 1024 * 1024,  // 7TB
			IsActive:       true,
			ReadSpeedMBs:   200,
			WriteSpeedMBs:  180,
			IOPS:           150,
			LatencyMs:      5,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "pool-cloud",
			Name:           "云冷存储池",
			Type:           StoragePoolCloud,
			Tier:           TierLevelCold,
			CapacityBytes:  50 * 1024 * 1024 * 1024 * 1024, // 50TB
			UsedBytes:      5 * 1024 * 1024 * 1024 * 1024,  // 5TB
			AvailableBytes: 45 * 1024 * 1024 * 1024 * 1024, // 45TB
			IsActive:       true,
			ReadSpeedMBs:   100,
			WriteSpeedMBs:  80,
			IOPS:           100,
			LatencyMs:      50,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, p := range defaultPools {
		m.pools[p.ID] = p
	}
}

// initDefaultRules 初始化默认分层规则
func (m *Manager) initDefaultRules() {
	defaultRules := []*TieringRule{
		{
			ID:             "rule-hot-access",
			Name:           "高频访问热数据规则",
			Description:    "最近7天内访问超过10次的文件升级到热存储",
			IsActive:       true,
			Priority:       1,
			MinAgeDays:     0,
			MaxAgeDays:     36500,
			MinAccessCount: 10,
			TargetTier:     TierLevelHot,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "rule-warm-age",
			Name:           "中等年龄温数据规则",
			Description:    "7-30天未访问的文件迁移到温存储",
			IsActive:       true,
			Priority:       2,
			MinAgeDays:     7,
			MaxAgeDays:     30,
			MaxAccessCount: 5,
			TargetTier:     TierLevelWarm,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "rule-cold-age",
			Name:           "长期未访问冷数据规则",
			Description:    "30天以上未访问的文件归档到冷存储",
			IsActive:       true,
			Priority:       3,
			MinAgeDays:     30,
			MaxAgeDays:     36500,
			MaxAccessCount: 2,
			TargetTier:     TierLevelCold,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:           "rule-large-file",
			Name:         "大文件归档规则",
			Description:  "超过1GB的大文件优先考虑冷存储",
			IsActive:     true,
			Priority:     4,
			MinSizeBytes: 1024 * 1024 * 1024, // 1GB
			MaxHeatScore: 50,
			TargetTier:   TierLevelCold,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, r := range defaultRules {
		m.rules[r.ID] = r
	}
}

// Start 启动管理器
func (m *Manager) Start() {
	m.scheduler.Start()
	m.logger.Info("storage tiering manager started")
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.scheduler.Stop()
	m.logger.Info("storage tiering manager stopped")
}

// RegisterFile 注册文件到分层系统
func (m *Manager) RegisterFile(file *FileMetadata) (*FileMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, ErrStorageTieringDisabled
	}

	if file.ID == "" {
		file.ID = generateID()
	}

	// 设置默认值
	if file.CurrentTier == "" {
		file.CurrentTier = TierLevelWarm
	}
	if file.AccessPattern == "" {
		file.AccessPattern = AccessPatternReadWrite
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now()
	}

	// 分析热度
	file = m.analyzer.AnalyzeFile(file, time.Now())

	m.files[file.ID] = file

	m.logger.Info("file registered",
		zap.String("id", file.ID),
		zap.String("path", file.Path),
		zap.String("tier", string(file.CurrentTier)),
		zap.Float64("heat_score", file.HeatScore))

	return file, nil
}

// GetFile 获取文件信息
func (m *Manager) GetFile(id string) (*FileMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, ok := m.files[id]
	if !ok {
		return nil, ErrFileNotFound
	}
	return file, nil
}

// ListFiles 列出所有文件
func (m *Manager) ListFiles() []*FileMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]*FileMetadata, 0, len(m.files))
	for _, f := range m.files {
		files = append(files, f)
	}
	return files
}

// ListFilesByTier 按层级列出文件
func (m *Manager) ListFilesByTier(tier TierLevel) []*FileMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []*FileMetadata
	for _, f := range m.files {
		if f.CurrentTier == tier {
			files = append(files, f)
		}
	}
	return files
}

// ListFilesByHeatLevel 按热度等级列出文件
func (m *Manager) ListFilesByHeatLevel(heatLevel HeatLevel) []*FileMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []*FileMetadata
	for _, f := range m.files {
		if f.HeatLevel == heatLevel {
			files = append(files, f)
		}
	}
	return files
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
	file.LastAccessAt = time.Now()
	file.LastModifiedAt = time.Now()

	// 重新计算热度
	file = m.analyzer.AnalyzeFile(file, time.Now())

	m.logger.Debug("file access recorded",
		zap.String("id", fileID),
		zap.Int64("count", file.AccessCount),
		zap.Float64("heat_score", file.HeatScore))

	return nil
}

// UpdateFileSize 更新文件大小
func (m *Manager) UpdateFileSize(fileID string, sizeBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, ok := m.files[fileID]
	if !ok {
		return ErrFileNotFound
	}

	file.SizeBytes = sizeBytes
	file.LastModifiedAt = time.Now()

	// 重新计算热度
	file = m.analyzer.AnalyzeFile(file, time.Now())

	return nil
}

// PinFile 固定文件在当前层级
func (m *Manager) PinFile(fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, ok := m.files[fileID]
	if !ok {
		return ErrFileNotFound
	}

	file.IsPinned = true

	m.logger.Info("file pinned",
		zap.String("id", fileID))

	return nil
}

// UnpinFile 取消固定文件
func (m *Manager) UnpinFile(fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, ok := m.files[fileID]
	if !ok {
		return ErrFileNotFound
	}

	file.IsPinned = false

	m.logger.Info("file unpinned",
		zap.String("id", fileID))

	return nil
}

// AddPool 添加存储池
func (m *Manager) AddPool(pool *StoragePool) (*StoragePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool.ID == "" {
		pool.ID = generateID()
	}

	pool.IsActive = true
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	m.pools[pool.ID] = pool

	m.logger.Info("storage pool added",
		zap.String("id", pool.ID),
		zap.String("name", pool.Name),
		zap.String("type", string(pool.Type)))

	return pool, nil
}

// GetPool 获取存储池
func (m *Manager) GetPool(id string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[id]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// ListPools 列出所有存储池
func (m *Manager) ListPools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*StoragePool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// ListPoolsByTier 按层级列出存储池
func (m *Manager) ListPoolsByTier(tier TierLevel) []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pools []*StoragePool
	for _, p := range m.pools {
		if p.Tier == tier {
			pools = append(pools, p)
		}
	}
	return pools
}

// AddRule 添加分层规则
func (m *Manager) AddRule(rule *TieringRule) (*TieringRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}

	rule.IsActive = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	m.logger.Info("tiering rule added",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name))

	return rule, nil
}

// GetRule 获取分层规则
func (m *Manager) GetRule(id string) (*TieringRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有分层规则
func (m *Manager) ListRules() []*TieringRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*TieringRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// EvaluateRules 评估规则，返回需要迁移的文件
func (m *Manager) EvaluateRules() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.AutoTieringEnabled {
		return nil
	}

	var tasks []*MigrationTask
	now := time.Now()

	// 获取所有激活规则，按优先级排序
	activeRules := m.getActiveRulesSorted()

	for _, file := range m.files {
		if file.IsPinned {
			continue
		}

		// 重新计算热度
		m.analyzer.AnalyzeFile(file, now)

		// 评估每个规则
		for _, rule := range activeRules {
			if m.matchRule(file, rule, now) {
				// 规则匹配，调度迁移
				task, err := m.scheduler.ScheduleMigration(file, rule.TargetTier, rule.ID)
				if err != nil {
					if err != ErrAlreadyInTier {
						m.logger.Warn("failed to schedule migration",
							zap.String("file_id", file.ID),
							zap.String("rule_id", rule.ID),
							zap.Error(err))
					}
					continue
				}
				tasks = append(tasks, task)
				break // 每个文件只匹配一个规则
			}
		}
	}

	return tasks
}

// getActiveRulesSorted 获取激活规则并按优先级排序
func (m *Manager) getActiveRulesSorted() []*TieringRule {
	var activeRules []*TieringRule
	for _, rule := range m.rules {
		if rule.IsActive {
			activeRules = append(activeRules, rule)
		}
	}

	// 简单的优先级排序（优先级数字越小越优先）
	for i := 0; i < len(activeRules); i++ {
		for j := i + 1; j < len(activeRules); j++ {
			if activeRules[j].Priority < activeRules[i].Priority {
				activeRules[i], activeRules[j] = activeRules[j], activeRules[i]
			}
		}
	}

	return activeRules
}

// matchRule 检查文件是否匹配规则
func (m *Manager) matchRule(file *FileMetadata, rule *TieringRule, now time.Time) bool {
	// 检查年龄
	if rule.MinAgeDays > 0 || rule.MaxAgeDays > 0 {
		fileAgeDays := int(now.Sub(file.CreatedAt).Hours() / 24)
		if rule.MinAgeDays > 0 && fileAgeDays < rule.MinAgeDays {
			return false
		}
		if rule.MaxAgeDays > 0 && fileAgeDays > rule.MaxAgeDays {
			return false
		}
	}

	// 检查大小
	if rule.MinSizeBytes > 0 && file.SizeBytes < rule.MinSizeBytes {
		return false
	}
	if rule.MaxSizeBytes > 0 && file.SizeBytes > rule.MaxSizeBytes {
		return false
	}

	// 检查访问次数
	if rule.MinAccessCount > 0 && file.AccessCount < rule.MinAccessCount {
		return false
	}
	if rule.MaxAccessCount > 0 && file.AccessCount > rule.MaxAccessCount {
		return false
	}

	// 检查热度分数
	if rule.MinHeatScore > 0 && file.HeatScore < rule.MinHeatScore {
		return false
	}
	if rule.MaxHeatScore > 0 && file.HeatScore > rule.MaxHeatScore {
		return false
	}

	// 检查访问模式
	if len(rule.AccessPatterns) > 0 {
		matched := false
		for _, pattern := range rule.AccessPatterns {
			if file.AccessPattern == pattern {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查标签
	if len(rule.IncludeTags) > 0 {
		hasTag := false
		for _, tag := range rule.IncludeTags {
			for _, fileTag := range file.Tags {
				if tag == fileTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	for _, tag := range rule.ExcludeTags {
		for _, fileTag := range file.Tags {
			if tag == fileTag {
				return false
			}
		}
	}

	// 检查目标层级是否与当前层级不同
	if rule.TargetTier == file.CurrentTier {
		return false
	}

	return true
}

// GetAnalysisReport 生成分析报告
func (m *Manager) GetAnalysisReport() *AnalysisReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	files := make([]*FileMetadata, 0, len(m.files))
	for _, f := range m.files {
		files = append(files, f)
	}

	// 计算各层级统计
	tierStats := m.calculateTierStats(files)

	// 获取迁移统计
	migrationStats := m.scheduler.GetStats()

	// 热度分布
	heatDistribution := m.analyzer.GetHeatDistribution(files)

	// 计算总文件数和总字节数
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.SizeBytes
	}

	// 生成建议
	recommendations := m.generateRecommendations(files, tierStats)

	return &AnalysisReport{
		ID:               generateID(),
		GeneratedAt:      now,
		TierStats:        tierStats,
		MigrationStats:   migrationStats,
		TotalFiles:       int64(len(files)),
		TotalBytes:       totalBytes,
		HeatDistribution: heatDistribution,
		Recommendations:  recommendations,
	}
}

// calculateTierStats 计算各层级统计
func (m *Manager) calculateTierStats(files []*FileMetadata) []*TieringStats {
	tierFiles := map[TierLevel][]*FileMetadata{
		TierLevelHot:  {},
		TierLevelWarm: {},
		TierLevelCold: {},
	}

	for _, f := range files {
		tierFiles[f.CurrentTier] = append(tierFiles[f.CurrentTier], f)
	}

	var stats []*TieringStats
	for tier, tierFileList := range tierFiles {
		var totalBytes int64
		var totalHeat float64
		for _, f := range tierFileList {
			totalBytes += f.SizeBytes
			totalHeat += f.HeatScore
		}

		avgHeat := 0.0
		if len(tierFileList) > 0 {
			avgHeat = totalHeat / float64(len(tierFileList))
		}

		// 计算使用率
		pool := m.findPoolForTier(tier)
		usedPercent := 0.0
		if pool != nil && pool.CapacityBytes > 0 {
			usedPercent = float64(pool.UsedBytes) / float64(pool.CapacityBytes) * 100
		}

		stats = append(stats, &TieringStats{
			ID:           generateID(),
			Tier:         tier,
			FileCount:    int64(len(tierFileList)),
			TotalBytes:   totalBytes,
			AvgHeatScore: avgHeat,
			UsedPercent:  usedPercent,
			UpdatedAt:    time.Now(),
		})
	}

	return stats
}

// findPoolForTier 查找层级对应的存储池
func (m *Manager) findPoolForTier(tier TierLevel) *StoragePool {
	for _, pool := range m.pools {
		if pool.Tier == tier {
			return pool
		}
	}
	return nil
}

// generateRecommendations 生成优化建议
func (m *Manager) generateRecommendations(files []*FileMetadata, tierStats []*TieringStats) []string {
	var recommendations []string

	// 检查热存储使用率
	for _, stat := range tierStats {
		if stat.Tier == TierLevelHot && stat.UsedPercent > 80 {
			recommendations = append(recommendations, "热存储使用率超过80%，建议将部分低热度文件迁移到温存储")
		}
		if stat.Tier == TierLevelCold && stat.UsedPercent > 90 {
			recommendations = append(recommendations, "冷存储使用率超过90%，建议清理过期归档数据")
		}
	}

	// 检查是否有大量冻结文件
	frozenCount := 0
	for _, f := range files {
		if f.HeatLevel == HeatLevelFrozen {
			frozenCount++
		}
	}
	if frozenCount > len(files)/2 {
		recommendations = append(recommendations, "超过50%的文件处于冻结状态，建议启用自动归档到冷存储")
	}

	// 检查是否有高频访问文件在冷存储
	for _, f := range files {
		if f.CurrentTier == TierLevelCold && f.HeatScore > 70 {
			recommendations = append(recommendations, fmt.Sprintf("文件 %s 在冷存储但热度较高，建议升级到热存储", f.Path))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前存储分层配置良好，无需优化")
	}

	return recommendations
}

// GetScheduler 获取调度器
func (m *Manager) GetScheduler() *Scheduler {
	return m.scheduler
}

// GetAnalyzer 获取分析器
func (m *Manager) GetAnalyzer() *Analyzer {
	return m.analyzer
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *StorageTieringConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *StorageTieringConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
		m.analyzer = NewAnalyzer(cfg)
	}
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
