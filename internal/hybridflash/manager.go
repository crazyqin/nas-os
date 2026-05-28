// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
package hybridflash

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// HybridFlashManager 混合闪存池管理器.
type HybridFlashManager struct {
	mu              sync.RWMutex
	pools           map[string]*PoolStatus
	configs         map[string]*HybridPoolConfig
	blockTracker    *BlockHeatTracker
	engine          *TieringEngine
	ctx             context.Context
	cancel          context.CancelFunc
	rebalanceTasks  map[string]*RebalanceResult
}

// NewHybridFlashManager 创建混合闪存池管理器.
func NewHybridFlashManager(engine *TieringEngine) *HybridFlashManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &HybridFlashManager{
		pools:          make(map[string]*PoolStatus),
		configs:        make(map[string]*HybridPoolConfig),
		blockTracker:   NewBlockHeatTracker(DefaultHeatTrackingConfig()),
		engine:         engine,
		ctx:            ctx,
		cancel:         cancel,
		rebalanceTasks: make(map[string]*RebalanceResult),
	}
}

// CreatePool 创建混合闪存池.
func (m *HybridFlashManager) CreatePool(config *HybridPoolConfig) (*PoolStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 检查是否已存在同名池
	for _, pool := range m.pools {
		if pool.PoolName == config.PoolName {
			return nil, fmt.Errorf("池 %s 已存在", config.PoolName)
		}
	}

	// 生成池 ID
	poolID := fmt.Sprintf("pool-%d", time.Now().UnixNano())

	// 创建 Flash 设备列表
	flashDevices := make([]FlashDevice, 0, len(config.FlashDevices))
	var flashTotal int64
	for i, path := range config.FlashDevices {
		device := FlashDevice{
			ID:         fmt.Sprintf("flash-%s-%d", poolID, i),
			Name:       fmt.Sprintf("nvme%dn1", i),
			Path:       path,
			Type:       config.FlashType,
			CacheRole:  CacheRole(config.FlashRole),
			Capacity:   512 * 1024 * 1024 * 1024, // 默认 512GB
			Enabled:    true,
			Health:     100.0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		flashDevices = append(flashDevices, device)
		flashTotal += device.Capacity
	}

	// 创建 HDD 设备列表
	hddDevices := make([]HDDDevice, 0, len(config.HDDDevices))
	var hddTotal int64
	for i, path := range config.HDDDevices {
		device := HDDDevice{
			ID:         fmt.Sprintf("hdd-%s-%d", poolID, i),
			Name:       fmt.Sprintf("sd%c", 'a'+i),
			Path:       path,
			Capacity:   4 * 1024 * 1024 * 1024 * 1024, // 默认 4TB
			Enabled:    true,
			Health:     100.0,
			RPM:        7200,
		}
		hddDevices = append(hddDevices, device)
		hddTotal += device.Capacity
	}

	// 创建默认策略
	tierPolicy := config.TierPolicy
	if tierPolicy == nil {
		tierPolicy = DefaultTierPolicy()
	}

	poolStatus := &PoolStatus{
		PoolID:       poolID,
		PoolName:     config.PoolName,
		State:        PoolStateOnline,
		FlashTotal:   flashTotal,
		HDDTotal:     hddTotal,
		FlashDevices: flashDevices,
		HDDDevices:   hddDevices,
		FlashRole:    config.FlashRole,
		TierPolicy:   tierPolicy,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.pools[poolID] = poolStatus
	m.configs[poolID] = config

	// 注册到分层引擎
	hybridPool := &HybridPool{
		ID:           poolID,
		Name:         config.PoolName,
		State:        PoolStateOnline,
		FlashDevices: convertFlashDevices(flashDevices),
		HDDDevices:   convertHDDDevices(hddDevices),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.engine.RegisterPool(hybridPool)

	slog.Info("混合闪存池已创建",
		"poolId", poolID,
		"poolName", config.PoolName,
		"flashRole", config.FlashRole,
		"flashDevices", len(config.FlashDevices),
		"hddDevices", len(config.HDDDevices),
	)

	return poolStatus, nil
}

// GetPool 获取混合闪存池状态.
func (m *HybridFlashManager) GetPool(poolID string) (*PoolStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("池 %s 不存在", poolID)
	}

	// 更新实时状态
	pool.UpdatedAt = time.Now()
	pool.HitRatio = m.calculateHitRatio(poolID)

	return pool, nil
}

// ListPools 列出所有混合闪存池.
func (m *HybridFlashManager) ListPools() []*PoolStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*PoolStatus, 0, len(m.pools))
	for _, pool := range m.pools {
		pool.HitRatio = m.calculateHitRatio(pool.PoolID)
		pools = append(pools, pool)
	}

	return pools
}

// UpdateTierPolicy 更新分层策略.
func (m *HybridFlashManager) UpdateTierPolicy(poolID string, policy *TierPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("池 %s 不存在", poolID)
	}

	// 验证策略
	if err := m.validateTierPolicy(policy); err != nil {
		return fmt.Errorf("策略验证失败: %w", err)
	}

	pool.TierPolicy = policy
	pool.UpdatedAt = time.Now()

	// 更新配置
	if config, ok := m.configs[poolID]; ok {
		config.TierPolicy = policy
	}

	slog.Info("分层策略已更新",
		"poolId", poolID,
		"hotDataThreshold", policy.HotDataThreshold,
		"coldDataAge", policy.ColdDataAge,
		"metadataPreference", policy.MetadataPreference,
	)

	return nil
}

// Rebalance 触发数据重平衡.
func (m *HybridFlashManager) Rebalance(poolID string, req *RebalanceRequest) (*RebalanceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("池 %s 不存在", poolID)
	}

	if pool.Rebalancing && !req.Force {
		return nil, fmt.Errorf("池 %s 正在重平衡中", poolID)
	}

	// 创建重平衡任务
	taskID := fmt.Sprintf("rebalance-%s-%d", poolID, time.Now().UnixNano())
	result := &RebalanceResult{
		TaskID:    taskID,
		Status:    "running",
		StartedAt: time.Now(),
	}

	pool.State = PoolStateRebalancing
	pool.Rebalancing = true
	pool.UpdatedAt = time.Now()

	m.rebalanceTasks[taskID] = result

	// 异步执行重平衡
	go m.executeRebalance(poolID, result, req)

	slog.Info("重平衡任务已启动",
		"taskId", taskID,
		"poolId", poolID,
		"force", req.Force,
		"dryRun", req.DryRun,
	)

	return result, nil
}

// executeRebalance 执行重平衡.
func (m *HybridFlashManager) executeRebalance(poolID string, result *RebalanceResult, req *RebalanceRequest) {
	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		pool, exists := m.pools[poolID]
		if exists {
			pool.State = PoolStateOnline
			pool.Rebalancing = false
			pool.UpdatedAt = time.Now()
		}
	}()

	// 模拟重平衡过程
	startTime := time.Now()

	// 分析需要移动的块
	blocksToMove := m.analyzeBlocksForRebalance(poolID, req)

	if req.DryRun {
		result.Status = "dry_run_completed"
		result.BlocksMoved = int64(len(blocksToMove))
		result.BytesMoved = m.calculateTotalBytes(blocksToMove)
		result.CompletedAt = time.Now()
		result.Duration = time.Since(startTime).String()
		return
	}

	// 执行块移动
	movedBlocks := int64(0)
	movedBytes := int64(0)
	for _, block := range blocksToMove {
		if err := m.moveBlock(poolID, block); err != nil {
			slog.Error("块移动失败", "blockId", block.BlockID, "error", err)
			continue
		}
		movedBlocks++
		movedBytes += block.Size
	}

	// 更新结果
	result.Status = "completed"
	result.BlocksMoved = movedBlocks
	result.BytesMoved = movedBytes
	result.CompletedAt = time.Now()
	result.Duration = time.Since(startTime).String()

	// 更新 Flash 使用率
	m.mu.RLock()
	if pool, exists := m.pools[poolID]; exists {
		result.FlashUsageAfter = float64(pool.FlashUsed) / float64(pool.FlashTotal)
	}
	m.mu.RUnlock()

	slog.Info("重平衡完成",
		"taskId", result.TaskID,
		"poolId", poolID,
		"blocksMoved", movedBlocks,
		"bytesMoved", movedBytes,
		"duration", result.Duration,
	)
}

// analyzeBlocksForRebalance 分析需要重平衡的块.
func (m *HybridFlashManager) analyzeBlocksForRebalance(poolID string, req *RebalanceRequest) []*BlockAccessRecord {
	m.blockTracker.mu.RLock()
	defer m.blockTracker.mu.RUnlock()

	var blocksToMove []*BlockAccessRecord

	for _, block := range m.blockTracker.blocks {
		if block.PoolID != poolID {
			continue
		}

		// 根据策略决定是否需要移动
		if m.shouldMoveBlock(block, req) {
			blocksToMove = append(blocksToMove, block)
		}
	}

	return blocksToMove
}

// shouldMoveBlock 判断是否需要移动块.
func (m *HybridFlashManager) shouldMoveBlock(block *BlockAccessRecord, req *RebalanceRequest) bool {
	m.mu.RLock()
	pool, exists := m.pools[block.PoolID]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	policy := pool.TierPolicy
	if policy == nil {
		return false
	}

	// 热数据应该在 Flash 上
	if block.HeatLevel == HeatLevelHot && block.CurrentTier == FlashTypeHDD {
		return true
	}

	// 冷数据应该在 HDD 上
	if block.HeatLevel == HeatLevelCold && block.CurrentTier != FlashTypeHDD {
		// 检查 Flash 使用率
		flashUsage := float64(pool.FlashUsed) / float64(pool.FlashTotal)
		if flashUsage > policy.MaxFlashUsage {
			return true
		}
	}

	// 小文件优先使用 Flash
	if block.Size < m.getSmallFileThreshold(block.PoolID) && block.CurrentTier == FlashTypeHDD {
		flashUsage := float64(pool.FlashUsed) / float64(pool.FlashTotal)
		if flashUsage < policy.MaxFlashUsage {
			return true
		}
	}

	return false
}

// moveBlock 移动块.
func (m *HybridFlashManager) moveBlock(poolID string, block *BlockAccessRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("池 %s 不存在", poolID)
	}

	// 模拟移动
	fromTier := block.CurrentTier
	var toTier FlashType

	if block.HeatLevel == HeatLevelHot {
		toTier = FlashTypeNVMe
	} else {
		toTier = FlashTypeHDD
	}

	// 更新块信息
	block.CurrentTier = toTier
	block.LastModified = time.Now()

	// 更新池使用量
	if fromTier == FlashTypeHDD && toTier != FlashTypeHDD {
		pool.FlashUsed += block.Size
		pool.HDDUsed -= block.Size
	} else if fromTier != FlashTypeHDD && toTier == FlashTypeHDD {
		pool.FlashUsed -= block.Size
		pool.HDDUsed += block.Size
	}

	pool.UpdatedAt = time.Now()

	slog.Debug("块已移动",
		"blockId", block.BlockID,
		"from", fromTier,
		"to", toTier,
		"size", block.Size,
	)

	return nil
}

// RecordAccess 记录数据访问.
func (m *HybridFlashManager) RecordAccess(poolID, filePath string, offset, size int64, pattern AccessPattern) {
	m.blockTracker.mu.Lock()
	defer m.blockTracker.mu.Unlock()

	blockID := fmt.Sprintf("%s:%d:%d", filePath, offset, size)

	block, exists := m.blockTracker.blocks[blockID]
	if !exists {
		block = &BlockAccessRecord{
			BlockID:       blockID,
			PoolID:        poolID,
			FilePath:      filePath,
			Offset:        offset,
			Size:          size,
			AccessPattern: pattern,
			CurrentTier:   FlashTypeHDD,
			AccessTime:    time.Now(),
			LastModified:  time.Now(),
		}
		m.blockTracker.blocks[blockID] = block
	}

	block.AccessCount++
	block.AccessTime = time.Now()

	if pattern == AccessPatternRandom {
		block.ReadBytes += size
	} else {
		block.WriteBytes += size
	}

	// 更新热度级别
	block.HeatLevel = m.calculateHeatLevel(block)
}

// calculateHeatLevel 计算热度级别.
func (m *HybridFlashManager) calculateHeatLevel(block *BlockAccessRecord) DataHeatLevel {
	m.mu.RLock()
	pool, exists := m.pools[block.PoolID]
	m.mu.RUnlock()

	if !exists || pool.TierPolicy == nil {
		return HeatLevelCold
	}

	threshold := pool.TierPolicy.HotDataThreshold
	if threshold == 0 {
		threshold = 100
	}

	if block.AccessCount >= threshold {
		return HeatLevelHot
	}
	if block.AccessCount >= threshold/10 {
		return HeatLevelWarm
	}

	// 检查访问时间
	age := time.Since(block.AccessTime)
	coldAge, err := time.ParseDuration(pool.TierPolicy.ColdDataAge)
	if err != nil {
		coldAge = 720 * time.Hour // 默认 30 天
	}

	if age > coldAge {
		return HeatLevelFrozen
	}
	if age > coldAge/2 {
		return HeatLevelCold
	}

	return HeatLevelWarm
}

// calculateHitRatio 计算命中率.
func (m *HybridFlashManager) calculateHitRatio(poolID string) float64 {
	m.blockTracker.mu.RLock()
	defer m.blockTracker.mu.RUnlock()

	totalAccess := int64(0)
	flashAccess := int64(0)

	for _, block := range m.blockTracker.blocks {
		if block.PoolID != poolID {
			continue
		}
		totalAccess += block.AccessCount
		if block.CurrentTier == FlashTypeNVMe || block.CurrentTier == FlashTypeSSD {
			flashAccess += block.AccessCount
		}
	}

	if totalAccess == 0 {
		return 0.0
	}

	return float64(flashAccess) / float64(totalAccess)
}

// getSmallFileThreshold 获取小文件阈值.
func (m *HybridFlashManager) getSmallFileThreshold(poolID string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if config, ok := m.configs[poolID]; ok && config.TierPolicy != nil {
		return config.TierPolicy.SmallFileThreshold
	}

	return 1024 * 1024 // 默认 1MB
}

// validateConfig 验证配置.
func (m *HybridFlashManager) validateConfig(config *HybridPoolConfig) error {
	if config.PoolName == "" {
		return fmt.Errorf("池名称不能为空")
	}

	if len(config.FlashDevices) == 0 {
		return fmt.Errorf("至少需要一个 Flash 设备")
	}

	if len(config.HDDDevices) == 0 {
		return fmt.Errorf("至少需要一个 HDD 设备")
	}

	// 验证 Flash 角色
	validRoles := map[FlashRole]bool{
		FlashRoleZIL:     true,
		FlashRoleSLOG:    true,
		FlashRoleData:    true,
		FlashRoleL2ARC:   true,
		FlashRoleMetadata: true,
	}
	if !validRoles[config.FlashRole] {
		return fmt.Errorf("无效的 Flash 角色: %s", config.FlashRole)
	}

	return nil
}

// validateTierPolicy 验证分层策略.
func (m *HybridFlashManager) validateTierPolicy(policy *TierPolicy) error {
	if policy == nil {
		return fmt.Errorf("策略不能为空")
	}

	if policy.HotDataThreshold < 0 {
		return fmt.Errorf("热数据阈值不能为负数")
	}

	if policy.MaxFlashUsage < 0 || policy.MaxFlashUsage > 1 {
		return fmt.Errorf("Flash 最大使用率必须在 0-1 之间")
	}

	if policy.MinHotDataRatio < 0 || policy.MinHotDataRatio > 1 {
		return fmt.Errorf("最小热数据比例必须在 0-1 之间")
	}

	// 验证冷数据时间格式
	if policy.ColdDataAge != "" {
		if _, err := time.ParseDuration(policy.ColdDataAge); err != nil {
			return fmt.Errorf("无效的冷数据时间格式: %s", policy.ColdDataAge)
		}
	}

	return nil
}

// convertFlashDevices 转换 Flash 设备列表.
func convertFlashDevices(devices []FlashDevice) []*FlashDevice {
	result := make([]*FlashDevice, len(devices))
	for i := range devices {
		result[i] = &devices[i]
	}
	return result
}

// convertHDDDevices 转换 HDD 设备列表.
func convertHDDDevices(devices []HDDDevice) []*HDDDevice {
	result := make([]*HDDDevice, len(devices))
	for i := range devices {
		result[i] = &devices[i]
	}
	return result
}

// GetRebalanceTask 获取重平衡任务状态.
func (m *HybridFlashManager) GetRebalanceTask(taskID string) (*RebalanceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.rebalanceTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	return task, nil
}

// calculateTotalBytes 计算块列表总字节数.
func (m *HybridFlashManager) calculateTotalBytes(blocks []*BlockAccessRecord) int64 {
	var total int64
	for _, b := range blocks {
		if b != nil {
			total += b.Size
		}
	}
	return total
}

// DeletePool 删除混合闪存池.
func (m *HybridFlashManager) DeletePool(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("池 %s 不存在", poolID)
	}

	if pool.Rebalancing {
		return fmt.Errorf("池 %s 正在重平衡中，无法删除", poolID)
	}

	// 从引擎注销
	m.engine.UnregisterPool(poolID)

	delete(m.pools, poolID)
	delete(m.configs, poolID)

	slog.Info("混合闪存池已删除", "poolId", poolID, "poolName", pool.PoolName)

	return nil
}
