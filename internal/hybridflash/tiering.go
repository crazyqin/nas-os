package hybridflash

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// TieringEngine 混合分层引擎.
type TieringEngine struct {
	mu                sync.RWMutex
	config            TieringConfig
	heatConfig        HeatTrackingConfig
	pools             map[string]*HybridPool
	blockTracker      *BlockHeatTracker
	migrationQueue    chan *MigrateTask
	runningTasks      map[string]*MigrateTask
	completedTasks    []*MigrateTask
	cancel            context.CancelFunc
	ctx               context.Context
	checkInterval     time.Duration
	heatCheckInterval time.Duration
}

// BlockHeatTracker 块热度追踪器.
type BlockHeatTracker struct {
	mu             sync.RWMutex
	blocks         map[string]*BlockAccessRecord
	hotBlocks      []*BlockAccessRecord
	warmBlocks     []*BlockAccessRecord
	coldBlocks     []*BlockAccessRecord
	heatThresholds HeatThresholds
	config         HeatTrackingConfig
}

// HeatThresholds 热度阈值.
type HeatThresholds struct {
	HotThreshold  int64
	WarmThreshold int64
	ColdAgeHours  int
}

// NewTieringEngine 创建新的分层引擎.
func NewTieringEngine(config TieringConfig, heatConfig HeatTrackingConfig) *TieringEngine {
	ctx, cancel := context.WithCancel(context.Background())

	// 解析间隔
	checkInterval, _ := time.ParseDuration(config.CheckInterval)
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute
	}
	heatCheckInterval, _ := time.ParseDuration(heatConfig.HeatCheckInterval)
	if heatCheckInterval == 0 {
		heatCheckInterval = 1 * time.Minute
	}

	return &TieringEngine{
		config:            config,
		heatConfig:        heatConfig,
		pools:             make(map[string]*HybridPool),
		blockTracker:      NewBlockHeatTracker(heatConfig),
		migrationQueue:    make(chan *MigrateTask, 100),
		runningTasks:      make(map[string]*MigrateTask),
		completedTasks:    make([]*MigrateTask, 0),
		ctx:               ctx,
		cancel:            cancel,
		checkInterval:     checkInterval,
		heatCheckInterval: heatCheckInterval,
	}
}

// NewBlockHeatTracker 创建块热度追踪器.
func NewBlockHeatTracker(config HeatTrackingConfig) *BlockHeatTracker {
	return &BlockHeatTracker{
		blocks: make(map[string]*BlockAccessRecord),
		heatThresholds: HeatThresholds{
			HotThreshold:  100,
			WarmThreshold: 10,
			ColdAgeHours:  720,
		},
		config: config,
	}
}

// Start 启动分层引擎.
func (e *TieringEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.config.Enabled {
		return fmt.Errorf("分层引擎未启用")
	}

	go e.monitorLoop()
	go e.migrationWorker()

	log.Println("[HybridFlash] 分层引擎已启动")
	return nil
}

// Stop 停止分层引擎.
func (e *TieringEngine) Stop() {
	e.cancel()
	log.Println("[HybridFlash] 分层引擎已停止")
}

// monitorLoop 监控循环.
func (e *TieringEngine) monitorLoop() {
	ticker := time.NewTicker(e.checkInterval)
	heatTicker := time.NewTicker(e.heatCheckInterval)
	defer ticker.Stop()
	defer heatTicker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAndMigrate()
		case <-heatTicker.C:
			e.updateHeatLevels()
		}
	}
}

// evaluateAndMigrate 评估并触发迁移.
func (e *TieringEngine) evaluateAndMigrate() {
	if !e.config.AutoMigrateEnabled {
		return
	}

	// 检查是否在迁移窗口内
	if !e.isInMigrationWindow() {
		return
	}

	e.mu.RLock()
	pools := make([]*HybridPool, 0, len(e.pools))
	for _, pool := range e.pools {
		pools = append(pools, pool)
	}
	e.mu.RUnlock()

	for _, pool := range pools {
		tasks := e.generateMigrationTasks(pool)
		for _, task := range tasks {
			e.migrationQueue <- task
		}
	}
}

// isInMigrationWindow 检查是否在迁移窗口内.
func (e *TieringEngine) isInMigrationWindow() bool {
	now := time.Now()
	startTime, err := time.Parse("15:04", e.config.MigrationWindowStart)
	if err != nil {
		return true // 默认允许迁移
	}
	endTime, err := time.Parse("15:04", e.config.MigrationWindowEnd)
	if err != nil {
		return true
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())

	if end.Before(start) {
		// 跨午夜
		return now.After(start) || now.Before(end)
	}
	return now.After(start) && now.Before(end)
}

// generateMigrationTasks 生成迁移任务.
func (e *TieringEngine) generateMigrationTasks(pool *HybridPool) []*MigrateTask {
	e.blockTracker.mu.RLock()
	defer e.blockTracker.mu.RUnlock()

	var tasks []*MigrateTask

	// 找出需要从 SSD 迁移到 HDD 的冷数据
	for _, block := range e.blockTracker.blocks {
		if block.CurrentTier == FlashTypeSSD && block.HeatLevel == HeatLevelCold {
			if e.shouldMigrateToHDD(block, pool) {
				task := &MigrateTask{
					ID:          fmt.Sprintf("migrate-%s-%d", block.BlockID, time.Now().UnixNano()),
					Status:      MigrateStatusPending,
					CreatedAt:   time.Now(),
					SourcePath:  block.BlockID,
					TargetPath:  block.BlockID,
					SourceTier:  FlashTypeSSD,
					TargetTier:  FlashTypeHDD,
					BlockSize:   block.Size,
					TotalBlocks: 1,
					TotalBytes:  block.Size,
				}
				tasks = append(tasks, task)
			}
		}
	}

	// 找出需要从 HDD 提升到 SSD 的热数据
	for _, block := range e.blockTracker.blocks {
		if block.CurrentTier == FlashTypeHDD && block.HeatLevel == HeatLevelHot {
			if e.shouldMigrateToSSD(block, pool) {
				task := &MigrateTask{
					ID:          fmt.Sprintf("migrate-%s-%d", block.BlockID, time.Now().UnixNano()),
					Status:      MigrateStatusPending,
					CreatedAt:   time.Now(),
					SourcePath:  block.BlockID,
					TargetPath:  block.BlockID,
					SourceTier:  FlashTypeHDD,
					TargetTier:  FlashTypeSSD,
					BlockSize:   block.Size,
					TotalBlocks: 1,
					TotalBytes:  block.Size,
				}
				tasks = append(tasks, task)
			}
		}
	}

	return tasks
}

// shouldMigrateToHDD 检查是否应该迁移到 HDD.
func (e *TieringEngine) shouldMigrateToHDD(block *BlockAccessRecord, pool *HybridPool) bool {
	if block.AccessCount > e.config.HotThreshold {
		return false
	}
	if pool.SSDUsage > e.config.SSDCapacityThreshold {
		return true
	}
	return block.HeatLevel == HeatLevelCold || block.HeatLevel == HeatLevelFrozen
}

// shouldMigrateToSSD 检查是否应该迁移到 SSD.
func (e *TieringEngine) shouldMigrateToSSD(block *BlockAccessRecord, pool *HybridPool) bool {
	if block.AccessCount < e.config.HotThreshold {
		return false
	}
	if pool.SSDUsage > e.config.SSDCapacityThreshold {
		return false
	}
	return block.HeatLevel == HeatLevelHot
}

// updateHeatLevels 更新热度级别.
func (e *TieringEngine) updateHeatLevels() {
	e.blockTracker.mu.Lock()
	defer e.blockTracker.mu.Unlock()

	now := time.Now()
	hotBlocks := make([]*BlockAccessRecord, 0)
	warmBlocks := make([]*BlockAccessRecord, 0)
	coldBlocks := make([]*BlockAccessRecord, 0)

	for _, block := range e.blockTracker.blocks {
		// 计算热度
		block.HeatLevel = e.calculateHeatLevel(block, now)

		switch block.HeatLevel {
		case HeatLevelHot:
			hotBlocks = append(hotBlocks, block)
		case HeatLevelWarm:
			warmBlocks = append(warmBlocks, block)
		default:
			coldBlocks = append(coldBlocks, block)
		}
	}

	e.blockTracker.hotBlocks = hotBlocks
	e.blockTracker.warmBlocks = warmBlocks
	e.blockTracker.coldBlocks = coldBlocks
}

// calculateHeatLevel 计算热度级别.
func (e *TieringEngine) calculateHeatLevel(block *BlockAccessRecord, now time.Time) DataHeatLevel {
	age := now.Sub(block.AccessTime)
	hours := age.Hours()

	// 根据访问频率和最近访问时间判断
	if block.AccessCount >= e.config.HotThreshold {
		return HeatLevelHot
	}
	if block.AccessCount >= e.config.WarmThreshold {
		if hours < 24 {
			return HeatLevelHot
		}
		return HeatLevelWarm
	}

	// 访问次数少，根据时间判断
	if hours < 24*7 { // 7 天内
		return HeatLevelWarm
	}
	if hours < float64(e.config.ColdAgeHours) {
		return HeatLevelCold
	}
	return HeatLevelFrozen
}

// migrationWorker 迁移工作线程.
func (e *TieringEngine) migrationWorker() {
	semaphore := make(chan struct{}, e.config.MaxConcurrentMigrates)

	for {
		select {
		case <-e.ctx.Done():
			return
		case task := <-e.migrationQueue:
			semaphore <- struct{}{}
			go func(t *MigrateTask) {
				defer func() { <-semaphore }()
				e.executeMigration(t)
			}(task)
		}
	}
}

// executeMigration 执行迁移.
func (e *TieringEngine) executeMigration(task *MigrateTask) {
	e.mu.Lock()
	task.Status = MigrateStatusRunning
	task.StartedAt = time.Now()
	e.runningTasks[task.ID] = task
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.runningTasks, task.ID)
		e.completedTasks = append(e.completedTasks, task)
		if len(e.completedTasks) > 1000 {
			e.completedTasks = e.completedTasks[1:]
		}
		e.mu.Unlock()
	}()

	// 模拟迁移过程
	log.Printf("[HybridFlash] 开始迁移任务 %s: %s -> %s", task.ID, task.SourceTier, task.TargetTier)

	// 模拟块迁移
	for i := int64(0); i < task.TotalBlocks; i++ {
		select {
		case <-e.ctx.Done():
			task.Status = MigrateStatusCancelled
			task.CompletedAt = time.Now()
			return
		default:
			time.Sleep(10 * time.Millisecond) // 模拟迁移延迟
			task.ProcessedBlocks++
			task.ProcessedBytes += task.BlockSize
		}
	}

	task.Status = MigrateStatusCompleted
	task.CompletedAt = time.Now()
	log.Printf("[HybridFlash] 迁移任务 %s 完成", task.ID)
}

// RecordBlockAccess 记录块访问.
func (e *TieringEngine) RecordBlockAccess(blockID, filePath string, offset, size int64, pattern AccessPattern) {
	e.blockTracker.mu.Lock()
	defer e.blockTracker.mu.Unlock()

	block, exists := e.blockTracker.blocks[blockID]
	if !exists {
		block = &BlockAccessRecord{
			BlockID:       blockID,
			FilePath:      filePath,
			Offset:        offset,
			Size:          size,
			AccessPattern: pattern,
			CurrentTier:   FlashTypeHDD, // 默认在 HDD
			AccessTime:    time.Now(),
			LastModified:  time.Now(),
		}
		e.blockTracker.blocks[blockID] = block
	}

	block.AccessCount++
	block.AccessTime = time.Now()
	if pattern == AccessPatternRandom {
		block.ReadBytes += size
	} else {
		block.WriteBytes += size
	}

	// 更新热度级别
	block.HeatLevel = e.calculateHeatLevel(block, time.Now())

	// 检查是否需要清理
	if len(e.blockTracker.blocks) > e.heatConfig.MaxTrackedBlocks {
		e.evictOldBlocks()
	}
}

// evictOldBlocks 清理旧块记录.
func (e *TieringEngine) evictOldBlocks() {
	// 按访问时间排序，移除最旧的
	blocks := make([]*BlockAccessRecord, 0, len(e.blockTracker.blocks))
	for _, b := range e.blockTracker.blocks {
		blocks = append(blocks, b)
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].AccessTime.Before(blocks[j].AccessTime)
	})

	// 移除前 10%
	removeCount := len(blocks) / 10
	for i := 0; i < removeCount; i++ {
		delete(e.blockTracker.blocks, blocks[i].BlockID)
	}
}

// getBlock 获取块记录.
func (bt *BlockHeatTracker) getBlock(blockID string) (*BlockAccessRecord, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	block, exists := bt.blocks[blockID]
	if !exists {
		return nil, fmt.Errorf("块 %s 不存在", blockID)
	}
	return block, nil
}

// RegisterPool 注册混合池.
func (e *TieringEngine) RegisterPool(pool *HybridPool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pools[pool.ID] = pool
}

// UnregisterPool 注销混合池.
func (e *TieringEngine) UnregisterPool(poolID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pools, poolID)
}

// GetPool 获取混合池.
func (e *TieringEngine) GetPool(poolID string) (*HybridPool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pool, exists := e.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolID)
	}
	return pool, nil
}

// GetStatus 获取分层状态.
func (e *TieringEngine) GetStatus() *TieringStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runningCount := 0
	pendingCount := 0
	var lastMigration time.Time

	for _, task := range e.runningTasks {
		runningCount++
		_ = task // 使用 task
	}
	pendingCount = len(e.migrationQueue)

	if len(e.completedTasks) > 0 {
		lastMigration = e.completedTasks[len(e.completedTasks)-1].CompletedAt
	}

	pools := make([]*HybridPool, 0, len(e.pools))
	for _, pool := range e.pools {
		pools = append(pools, pool)
	}

	return &TieringStatus{
		Enabled:       e.config.Enabled,
		RunningTasks:  runningCount,
		PendingTasks:  pendingCount,
		LastMigration: lastMigration,
		TotalBlocks:   int64(len(e.blockTracker.blocks)),
		SSDBlocks:     e.countBlocksByTier(FlashTypeSSD),
		HDDBlocks:     e.countBlocksByTier(FlashTypeHDD),
		Pools:         pools,
		Config:        &e.config,
	}
}

// countBlocksByTier 按层级统计块数.
func (e *TieringEngine) countBlocksByTier(tier FlashType) int64 {
	count := int64(0)
	for _, block := range e.blockTracker.blocks {
		if block.CurrentTier == tier {
			count++
		}
	}
	return count
}

// GenerateEfficiencyReport 生成效率报告.
func (e *TieringEngine) GenerateEfficiencyReport(period string) *EfficiencyReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &EfficiencyReport{
		GeneratedAt:      time.Now(),
		Period:           period,
		HitRateByTier:    make(map[FlashType]float64),
		TierDistribution: make(map[FlashType]*TierDistStats),
	}

	// 计算命中率
	totalAccesses := int64(0)
	hits := int64(0)
	for _, block := range e.blockTracker.blocks {
		totalAccesses += block.AccessCount
		if block.CurrentTier == FlashTypeSSD {
			hits += block.AccessCount
		}
	}
	if totalAccesses > 0 {
		report.OverallHitRate = float64(hits) / float64(totalAccesses)
	}

	// 按层级统计
	tierStats := make(map[FlashType]*TierDistStats)
	for _, block := range e.blockTracker.blocks {
		stats, exists := tierStats[block.CurrentTier]
		if !exists {
			stats = &TierDistStats{
				FlashType: block.CurrentTier,
			}
			tierStats[block.CurrentTier] = stats
		}
		stats.BlockCount++
		stats.TotalBytes += block.Size

		switch block.HeatLevel {
		case HeatLevelHot:
			stats.HotBlocks++
		case HeatLevelWarm:
			stats.WarmBlocks++
		case HeatLevelCold:
			stats.ColdBlocks++
		case HeatLevelFrozen:
			stats.FrozenBlocks++
		}
	}
	report.TierDistribution = tierStats

	// Top 热块
	hotBlocks := make([]*BlockAccessRecord, 0)
	for _, block := range e.blockTracker.blocks {
		if block.HeatLevel == HeatLevelHot {
			hotBlocks = append(hotBlocks, block)
		}
	}
	sort.Slice(hotBlocks, func(i, j int) bool {
		return hotBlocks[i].AccessCount > hotBlocks[j].AccessCount
	})
	if len(hotBlocks) > 10 {
		hotBlocks = hotBlocks[:10]
	}
	report.TopHotBlocks = hotBlocks

	// Top 冷块
	coldBlocks := make([]*BlockAccessRecord, 0)
	for _, block := range e.blockTracker.blocks {
		if block.HeatLevel == HeatLevelCold || block.HeatLevel == HeatLevelFrozen {
			coldBlocks = append(coldBlocks, block)
		}
	}
	sort.Slice(coldBlocks, func(i, j int) bool {
		return coldBlocks[i].AccessTime.Before(coldBlocks[j].AccessTime)
	})
	if len(coldBlocks) > 10 {
		coldBlocks = coldBlocks[:10]
	}
	report.TopColdBlocks = coldBlocks

	// 生成建议
	report.Recommendations = e.generateRecommendations(report)

	return report
}

// generateRecommendations 生成优化建议.
func (e *TieringEngine) generateRecommendations(report *EfficiencyReport) []string {
	var recommendations []string

	if report.OverallHitRate < 0.7 {
		recommendations = append(recommendations, "缓存命中率低于 70%，建议增加 SSD 容量或优化缓存策略")
	}

	if ssdStats, ok := report.TierDistribution[FlashTypeSSD]; ok {
		if ssdStats.TotalBytes > 0 {
			usagePercent := float64(ssdStats.TotalBytes) / float64(1024*1024*1024*100) // 假设 100GB SSD
			if usagePercent > 0.85 {
				recommendations = append(recommendations, "SSD 使用率超过 85%，建议清理冷数据或扩容")
			}
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "分层状态良好，无需优化")
	}

	return recommendations
}

// UpdateConfig 更新配置.
func (e *TieringEngine) UpdateConfig(config TieringConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// GetHeatConfig 获取热度追踪配置.
func (e *TieringEngine) GetHeatConfig() HeatTrackingConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.heatConfig
}

// UpdateHeatConfig 更新热度追踪配置.
func (e *TieringEngine) UpdateHeatConfig(config HeatTrackingConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.heatConfig = config
	e.blockTracker.config = config
}

// GetBlockHeatInfo 获取块热度信息.
func (e *TieringEngine) GetBlockHeatInfo(blockID string) (*BlockAccessRecord, error) {
	e.blockTracker.mu.RLock()
	defer e.blockTracker.mu.RUnlock()

	block, exists := e.blockTracker.blocks[blockID]
	if !exists {
		return nil, fmt.Errorf("块 %s 不存在", blockID)
	}
	return block, nil
}

// GetHotBlocks 获取热块列表.
func (e *TieringEngine) GetHotBlocks(limit int) []*BlockAccessRecord {
	e.blockTracker.mu.RLock()
	defer e.blockTracker.mu.RUnlock()

	blocks := make([]*BlockAccessRecord, 0)
	for _, block := range e.blockTracker.blocks {
		if block.HeatLevel == HeatLevelHot {
			blocks = append(blocks, block)
		}
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].AccessCount > blocks[j].AccessCount
	})

	if limit > 0 && len(blocks) > limit {
		blocks = blocks[:limit]
	}
	return blocks
}

// GetColdBlocks 获取冷块列表.
func (e *TieringEngine) GetColdBlocks(limit int) []*BlockAccessRecord {
	e.blockTracker.mu.RLock()
	defer e.blockTracker.mu.RUnlock()

	blocks := make([]*BlockAccessRecord, 0)
	for _, block := range e.blockTracker.blocks {
		if block.HeatLevel == HeatLevelCold || block.HeatLevel == HeatLevelFrozen {
			blocks = append(blocks, block)
		}
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].AccessTime.Before(blocks[j].AccessTime)
	})

	if limit > 0 && len(blocks) > limit {
		blocks = blocks[:limit]
	}
	return blocks
}
