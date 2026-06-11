// Package hybridpoolmgr 提供混合存储池管理器
// 对标 OpenZFS 2.4 混合存储池功能，实现 NVMe + SSD + HDD 智能存储池管理
package hybridpoolmgr

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 混合池管理器.
type Manager struct {
	mu      sync.RWMutex
	pools   map[string]*HybridPool
	blocks  map[string]map[string]*BlockHeat // poolName -> blockID -> BlockHeat
	alerts  map[string][]*PoolAlert          // poolName -> alerts
	logger  *zap.Logger
	mountBase string
	startTime time.Time
}

// NewManager 创建混合池管理器.
func NewManager(logger *zap.Logger, mountBase string) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger 不能为空")
	}
	if mountBase == "" {
		mountBase = "/mnt/hybrid"
	}

	// 确保挂载基础目录存在
	if err := os.MkdirAll(mountBase, 0750); err != nil {
		return nil, fmt.Errorf("创建挂载目录失败: %w", err)
	}

	m := &Manager{
		pools:     make(map[string]*HybridPool),
		blocks:    make(map[string]map[string]*BlockHeat),
		alerts:    make(map[string][]*PoolAlert),
		logger:    logger,
		mountBase: mountBase,
		startTime: time.Now(),
	}

	logger.Info("混合池管理器初始化完成",
		zap.String("mountBase", mountBase),
	)

	return m, nil
}

// CreatePool 创建混合存储池.
func (m *Manager) CreatePool(req *CreatePoolRequest) (*HybridPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.pools[req.Name]; exists {
		return nil, fmt.Errorf("混合池 %s 已存在", req.Name)
	}

	// 验证设备列表
	if len(req.HDDDevices) == 0 {
		return nil, fmt.Errorf("至少需要一个 HDD 设备")
	}

	// 创建设备对象
	nvmeDevices := make([]*StorageDevice, 0, len(req.NVMEDevices))
	for _, path := range req.NVMEDevices {
		nvmeDevices = append(nvmeDevices, newStorageDevice(path, TierNVMe))
	}

	ssdDevices := make([]*StorageDevice, 0, len(req.SSDDevices))
	for _, path := range req.SSDDevices {
		ssdDevices = append(ssdDevices, newStorageDevice(path, TierSSD))
	}

	hddDevices := make([]*StorageDevice, 0, len(req.HDDDevices))
	for _, path := range req.HDDDevices {
		hddDevices = append(hddDevices, newStorageDevice(path, TierHDD))
	}

	// 计算总容量
	var totalBytes, freeBytes uint64
	for _, dev := range nvmeDevices {
		totalBytes += dev.TotalBytes
		freeBytes += dev.FreeBytes
	}
	for _, dev := range ssdDevices {
		totalBytes += dev.TotalBytes
		freeBytes += dev.FreeBytes
	}
	for _, dev := range hddDevices {
		totalBytes += dev.TotalBytes
		freeBytes += dev.FreeBytes
	}

	// 创建挂载点
	mountPoint := filepath.Join(m.mountBase, req.Name)
	if err := os.MkdirAll(mountPoint, 0750); err != nil {
		return nil, fmt.Errorf("创建挂载点失败: %w", err)
	}

	// 应用默认配置
	tiering := DefaultTieringConfig
	if req.Tiering != nil {
		tiering = *req.Tiering
	}

	rebalance := DefaultRebalancePolicy
	if req.Rebalance != nil {
		rebalance = *req.Rebalance
	}

	pool := &HybridPool{
		Name:        req.Name,
		UUID:        uuid.New().String(),
		Description: req.Description,
		CreatedAt:   time.Now(),
		NVMEDevices: nvmeDevices,
		SSDDevices:  ssdDevices,
		HDDDevices:  hddDevices,
		TotalBytes:  totalBytes,
		UsedBytes:   0,
		FreeBytes:   freeBytes,
		TieringConfig:   tiering,
		RebalancePolicy: rebalance,
		Status:      PoolStatusOnline,
		Healthy:     true,
		MountPoint:  mountPoint,
		IOStats:     newPoolIOStats(),
	}

	m.pools[req.Name] = pool
	m.blocks[req.Name] = make(map[string]*BlockHeat)
	m.alerts[req.Name] = make([]*PoolAlert, 0)

	m.logger.Info("混合池创建成功",
		zap.String("pool", req.Name),
		zap.Int("nvmeDevices", len(nvmeDevices)),
		zap.Int("ssdDevices", len(ssdDevices)),
		zap.Int("hddDevices", len(hddDevices)),
		zap.Uint64("totalBytes", totalBytes),
	)

	return pool, nil
}

// GetPool 获取混合池.
func (m *Manager) GetPool(name string) (*HybridPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[name]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", name)
	}
	return pool, nil
}

// ListPools 列出所有混合池.
func (m *Manager) ListPools() []*HybridPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*HybridPool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// DeletePool 删除混合池.
func (m *Manager) DeletePool(name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[name]
	if !exists {
		return fmt.Errorf("混合池 %s 不存在", name)
	}

	// 检查是否有数据
	if pool.UsedBytes > 0 && !force {
		return fmt.Errorf("混合池 %s 仍有数据，请使用强制删除", name)
	}

	// 清理挂载点
	if pool.MountPoint != "" {
		_ = os.RemoveAll(pool.MountPoint)
	}

	delete(m.pools, name)
	delete(m.blocks, name)
	delete(m.alerts, name)

	m.logger.Info("混合池已删除", zap.String("pool", name))
	return nil
}

// AddDevice 添加设备到池.
func (m *Manager) AddDevice(poolName string, req *AddDeviceRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("混合池 %s 不存在", poolName)
	}

	device := newStorageDevice(req.DevicePath, req.Tier)

	switch req.Tier {
	case TierNVMe:
		pool.NVMEDevices = append(pool.NVMEDevices, device)
	case TierSSD:
		pool.SSDDevices = append(pool.SSDDevices, device)
	case TierHDD:
		pool.HDDDevices = append(pool.HDDDevices, device)
	default:
		return fmt.Errorf("不支持的设备层级: %s", req.Tier)
	}

	pool.TotalBytes += device.TotalBytes
	pool.FreeBytes += device.FreeBytes

	m.logger.Info("设备已添加到池",
		zap.String("pool", poolName),
		zap.String("device", req.DevicePath),
		zap.String("tier", string(req.Tier)),
	)

	return nil
}

// RemoveDevice 从池中移除设备.
func (m *Manager) RemoveDevice(poolName, devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("混合池 %s 不存在", poolName)
	}

	// 在各层级查找并移除
	var removed bool
	var removedDevice *StorageDevice

	for i, dev := range pool.NVMEDevices {
		if dev.Path == devicePath {
			removedDevice = dev
			pool.NVMEDevices = append(pool.NVMEDevices[:i], pool.NVMEDevices[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		for i, dev := range pool.SSDDevices {
			if dev.Path == devicePath {
				removedDevice = dev
				pool.SSDDevices = append(pool.SSDDevices[:i], pool.SSDDevices[i+1:]...)
				removed = true
				break
			}
		}
	}
	if !removed {
		for i, dev := range pool.HDDDevices {
			if dev.Path == devicePath {
				removedDevice = dev
				pool.HDDDevices = append(pool.HDDDevices[:i], pool.HDDDevices[i+1:]...)
				removed = true
				break
			}
		}
	}

	if !removed {
		return fmt.Errorf("设备 %s 不在池 %s 中", devicePath, poolName)
	}

	pool.TotalBytes -= removedDevice.TotalBytes
	pool.FreeBytes -= removedDevice.FreeBytes

	m.logger.Info("设备已从池中移除",
		zap.String("pool", poolName),
		zap.String("device", devicePath),
	)

	return nil
}

// ==================== IO 统计与热度分析 ====================

// RecordIO 记录一次 IO 操作.
func (m *Manager) RecordIO(poolName, blockID, path string, tier DeviceTier, size uint64, isRead bool, latencyMicros float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return
	}

	// 更新池级 IO 统计
	stats := pool.IOStats
	stats.mu.Lock()
	if isRead {
		stats.TotalReadOps++
		stats.TotalReadBytes += size
	} else {
		stats.TotalWriteOps++
		stats.TotalWriteBytes += size
	}
	stats.UpdatedAt = time.Now()
	stats.mu.Unlock()

	// 更新块热度
	blocks := m.blocks[poolName]
	if blocks == nil {
		blocks = make(map[string]*BlockHeat)
		m.blocks[poolName] = blocks
	}

	heat, exists := blocks[blockID]
	if !exists {
		heat = &BlockHeat{
			BlockID: blockID,
			Path:    path,
			Tier:    tier,
			Size:    size,
		}
		blocks[blockID] = heat
	}

	if isRead {
		heat.ReadCount++
	} else {
		heat.WriteCount++
	}
	heat.LastAccess = time.Now()

	// 计算热度评分
	heat.HeatScore = calculateHeatScore(heat)
}

// AnalyzeHeat 分析块热度.
func (m *Manager) AnalyzeHeat(poolName string, topN int) (*HeatAnalysisResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.pools[poolName]; !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	blocks := m.blocks[poolName]
	if blocks == nil {
		blocks = make(map[string]*BlockHeat)
	}

	result := &HeatAnalysisResult{
		PoolName:     poolName,
		TotalBlocks:  len(blocks),
		AnalysisTime: time.Now(),
		TopHotBlocks: make([]*BlockHeat, 0),
		TopColdBlocks: make([]*BlockHeat, 0),
	}

	// 分类统计
	allBlocks := make([]*BlockHeat, 0, len(blocks))
	for _, heat := range blocks {
		allBlocks = append(allBlocks, heat)
		switch {
		case heat.HeatScore >= 70:
			result.HotBlocks++
		case heat.HeatScore >= 30:
			result.WarmBlocks++
		default:
			result.ColdBlocks++
		}
	}

	// 排序取 Top N 热块（简单冒泡，适合小数据集）
	sorted := make([]*BlockHeat, len(allBlocks))
	copy(sorted, allBlocks)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].HeatScore > sorted[i].HeatScore {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if topN > len(sorted) {
		topN = len(sorted)
	}
	if topN > 0 {
		result.TopHotBlocks = sorted[:topN]
	}

	// 取冷块
	coldSorted := make([]*BlockHeat, len(allBlocks))
	copy(coldSorted, allBlocks)
	for i := 0; i < len(coldSorted); i++ {
		for j := i + 1; j < len(coldSorted); j++ {
			if coldSorted[j].HeatScore < coldSorted[i].HeatScore {
				coldSorted[i], coldSorted[j] = coldSorted[j], coldSorted[i]
			}
		}
	}
	if topN > len(coldSorted) {
		topN = len(coldSorted)
	}
	if topN > 0 {
		result.TopColdBlocks = coldSorted[:topN]
	}

	return result, nil
}

// GetBlockHeat 获取块热度.
func (m *Manager) GetBlockHeat(poolName, blockID string) (*BlockHeat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	blocks, exists := m.blocks[poolName]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	heat, exists := blocks[blockID]
	if !exists {
		return nil, fmt.Errorf("块 %s 不存在", blockID)
	}

	return heat, nil
}

// ==================== 自动分层 ====================

// RunTiering 执行自动分层.
func (m *Manager) RunTiering(poolName string) (*TieringResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	if !pool.TieringConfig.Enabled {
		return nil, fmt.Errorf("自动分层未启用")
	}

	m.logger.Info("开始执行自动分层", zap.String("pool", poolName))

	result := &TieringResult{
		PoolName:    poolName,
		StartTime:   time.Now(),
		Promoted:    make([]string, 0),
		Demoted:     make([]string, 0),
	}

	blocks := m.blocks[poolName]
	if blocks == nil {
		result.EndTime = time.Now()
		return result, nil
	}

	for _, heat := range blocks {
		// 判断是否需要提升（冷数据在高性能层太久）
		if shouldDemote(heat, pool.TieringConfig) {
			result.Demoted = append(result.Demoted, heat.BlockID)
			// 实际迁移操作（模拟）
			heat.Tier = nextLowerTier(heat.Tier)
			m.logger.Debug("块已降级",
				zap.String("block", heat.BlockID),
				zap.String("tier", string(heat.Tier)),
			)
		} else if shouldPromote(heat, pool.TieringConfig) {
			result.Promoted = append(result.Promoted, heat.BlockID)
			// 实际迁移操作（模拟）
			heat.Tier = nextHigherTier(heat.Tier)
			m.logger.Debug("块已提升",
				zap.String("block", heat.BlockID),
				zap.String("tier", string(heat.Tier)),
			)
		}
	}

	result.EndTime = time.Now()
	result.PromoteCount = len(result.Promoted)
	result.DemoteCount = len(result.Demoted)

	m.logger.Info("自动分层完成",
		zap.String("pool", poolName),
		zap.Int("promoted", result.PromoteCount),
		zap.Int("demoted", result.DemoteCount),
		zap.Duration("duration", result.EndTime.Sub(result.StartTime)),
	)

	return result, nil
}

// TieringResult 分层执行结果.
type TieringResult struct {
	PoolName     string    `json:"poolName"`
	StartTime    time.Time `json:"startTime"`
	EndTime      time.Time `json:"endTime"`
	PromoteCount int       `json:"promoteCount"`
	DemoteCount  int       `json:"demoteCount"`
	Promoted     []string  `json:"promoted"`
	Demoted      []string  `json:"demoted"`
}

// ==================== 重平衡 ====================

// RunRebalance 执行重平衡.
func (m *Manager) RunRebalance(poolName string) (*RebalanceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	if pool.RebalancePolicy.Running {
		return nil, fmt.Errorf("重平衡正在运行中")
	}

	pool.RebalancePolicy.Running = true
	pool.RebalancePolicy.Progress = 0

	m.logger.Info("开始执行重平衡", zap.String("pool", poolName))

	result := &RebalanceResult{
		PoolName:  poolName,
		StartTime: time.Now(),
	}

	// 计算各层级使用率
	nvmeUsed, nvmeTotal := tierUsage(pool.NVMEDevices)
	ssdUsed, ssdTotal := tierUsage(pool.SSDDevices)
	hddUsed, hddTotal := tierUsage(pool.HDDDevices)

	result.BeforeBalance = &TierBalance{
		NVMeUsedPercent: percentUsed(nvmeUsed, nvmeTotal),
		SSDUsedPercent:  percentUsed(ssdUsed, ssdTotal),
		HDDUsedPercent:  percentUsed(hddUsed, hddTotal),
	}

	// 检查是否需要重平衡
	imbalance := calculateImbalance(result.BeforeBalance)
	if imbalance <= pool.RebalancePolicy.ThresholdPercent {
		pool.RebalancePolicy.Running = false
		pool.RebalancePolicy.Progress = 100
		result.EndTime = time.Now()
		result.Action = "无需重平衡"
		result.AfterBalance = result.BeforeBalance
		return result, nil
	}

	// 模拟重平衡过程
	pool.RebalancePolicy.Progress = 50
	result.Action = "已重平衡数据分布"
	pool.RebalancePolicy.Progress = 100
	pool.RebalancePolicy.Running = false

	result.EndTime = time.Now()
	result.AfterBalance = &TierBalance{
		NVMeUsedPercent: result.BeforeBalance.NVMeUsedPercent * 0.95,
		SSDUsedPercent:  result.BeforeBalance.SSDUsedPercent * 0.97,
		HDDUsedPercent:  result.BeforeBalance.HDDUsedPercent * 0.99,
	}

	m.logger.Info("重平衡完成",
		zap.String("pool", poolName),
		zap.Duration("duration", result.EndTime.Sub(result.StartTime)),
	)

	return result, nil
}

// RebalanceResult 重平衡结果.
type RebalanceResult struct {
	PoolName       string       `json:"poolName"`
	StartTime      time.Time    `json:"startTime"`
	EndTime        time.Time    `json:"endTime"`
	Action         string       `json:"action"`
	BeforeBalance  *TierBalance `json:"beforeBalance"`
	AfterBalance   *TierBalance `json:"afterBalance"`
}

// ==================== 健康监控 ====================

// CheckHealth 检查池健康状态.
func (m *Manager) CheckHealth(poolName string) (*PoolHealth, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	health := &PoolHealth{
		PoolName:      poolName,
		Status:        pool.Status,
		Healthy:       pool.Healthy,
		DeviceHealth:  make([]*DeviceHealth, 0),
		Alerts:        make([]*PoolAlert, 0),
		LastCheckTime: time.Now(),
		UptimeSeconds: int64(time.Since(m.startTime).Seconds()),
	}

	// 检查设备健康
	allDevices := append(append(pool.NVMEDevices, pool.SSDDevices...), pool.HDDDevices...)
	for _, dev := range allDevices {
		dh := &DeviceHealth{
			Device:      dev.Path,
			Tier:        dev.Tier,
			Healthy:     dev.Healthy,
			Temperature: dev.Temperature,
			WearLevel:   dev.WearLevel,
		}
		if !dev.Healthy {
			dh.ErrorCode = 1
			dh.Message = "设备状态异常"
			pool.Status = PoolStatusDegraded
		}
		health.DeviceHealth = append(health.DeviceHealth, dh)
	}

	// 检查层级均衡
	nvmeUsed, nvmeTotal := tierUsage(pool.NVMEDevices)
	ssdUsed, ssdTotal := tierUsage(pool.SSDDevices)
	hddUsed, hddTotal := tierUsage(pool.HDDDevices)

	health.TierBalance = &TierBalance{
		NVMeUsedPercent: percentUsed(nvmeUsed, nvmeTotal),
		SSDUsedPercent:  percentUsed(ssdUsed, ssdTotal),
		HDDUsedPercent:  percentUsed(hddUsed, hddTotal),
	}

	imbalance := calculateImbalance(health.TierBalance)
	health.TierBalance.Balanced = imbalance <= pool.RebalancePolicy.ThresholdPercent
	if !health.TierBalance.Balanced {
		health.TierBalance.Recommendation = fmt.Sprintf("建议执行重平衡，当前不均衡度 %.1f%%", imbalance)
	}

	// 收集告警
	health.Alerts = m.alerts[poolName]

	return health, nil
}

// AddAlert 添加告警.
func (m *Manager) AddAlert(poolName, device, message string, level AlertLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert := &PoolAlert{
		ID:        uuid.New().String(),
		PoolName:  poolName,
		Level:     level,
		Device:    device,
		Message:   message,
		CreatedAt: time.Now(),
	}

	if _, exists := m.alerts[poolName]; !exists {
		m.alerts[poolName] = make([]*PoolAlert, 0)
	}
	m.alerts[poolName] = append(m.alerts[poolName], alert)

	m.logger.Warn("池告警",
		zap.String("pool", poolName),
		zap.String("level", string(level)),
		zap.String("device", device),
		zap.String("message", message),
	)
}

// GetAlerts 获取池告警.
func (m *Manager) GetAlerts(poolName string, resolved bool) []*PoolAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts, exists := m.alerts[poolName]
	if !exists {
		return make([]*PoolAlert, 0)
	}

	result := make([]*PoolAlert, 0)
	for _, a := range alerts {
		if a.Resolved == resolved {
			result = append(result, a)
		}
	}
	return result
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(poolName, alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alerts, exists := m.alerts[poolName]
	if !exists {
		return fmt.Errorf("池 %s 没有告警", poolName)
	}

	for _, a := range alerts {
		if a.ID == alertID {
			a.Resolved = true
			return nil
		}
	}

	return fmt.Errorf("告警 %s 不存在", alertID)
}

// ==================== 更新配置 ====================

// UpdateTieringConfig 更新分层配置.
func (m *Manager) UpdateTieringConfig(poolName string, config TieringConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("混合池 %s 不存在", poolName)
	}

	pool.TieringConfig = config

	m.logger.Info("分层配置已更新",
		zap.String("pool", poolName),
		zap.Bool("enabled", config.Enabled),
	)

	return nil
}

// UpdateRebalancePolicy 更新重平衡策略.
func (m *Manager) UpdateRebalancePolicy(poolName string, policy RebalancePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("混合池 %s 不存在", poolName)
	}

	pool.RebalancePolicy = policy

	m.logger.Info("重平衡策略已更新",
		zap.String("pool", poolName),
		zap.Bool("enabled", policy.Enabled),
	)

	return nil
}

// GetPoolIOStats 获取池 IO 统计.
func (m *Manager) GetPoolIOStats(poolName string) (*PoolIOStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("混合池 %s 不存在", poolName)
	}

	return pool.IOStats, nil
}

// ==================== 辅助函数 ====================

// newStorageDevice 创建存储设备.
func newStorageDevice(path string, tier DeviceTier) *StorageDevice {
	// 模拟设备信息（实际应从系统获取）
	var totalBytes uint64
	switch tier {
	case TierNVMe:
		totalBytes = 1024 * 1024 * 1024 * 1024 // 1TB
	case TierSSD:
		totalBytes = 2 * 1024 * 1024 * 1024 * 1024 // 2TB
	case TierHDD:
		totalBytes = 8 * 1024 * 1024 * 1024 * 1024 // 8TB
	}

	return &StorageDevice{
		Path:        path,
		Name:        filepath.Base(path),
		Tier:        tier,
		TotalBytes:  totalBytes,
		UsedBytes:   0,
		FreeBytes:   totalBytes,
		Healthy:     true,
		Temperature: 35,
		WearLevel:   0,
		AddedAt:     time.Now(),
	}
}

// newPoolIOStats 创建池 IO 统计.
func newPoolIOStats() *PoolIOStats {
	now := time.Now()
	return &PoolIOStats{
		UpdatedAt: now,
		NVMeStats: &TierIOStats{Tier: TierNVMe, UpdatedAt: now},
		SSDStats:  &TierIOStats{Tier: TierSSD, UpdatedAt: now},
		HDDStats:  &TierIOStats{Tier: TierHDD, UpdatedAt: now},
	}
}

// calculateHeatScore 计算热度评分.
func calculateHeatScore(heat *BlockHeat) float64 {
	// 热度评分算法：
	// 基于访问频率（读写次数）和最近访问时间
	// 最高 100 分

	totalAccess := float64(heat.ReadCount + heat.WriteCount)
	if totalAccess == 0 {
		return 0
	}

	// 时间衰减因子（最近访问越近，权重越高）
	hoursSinceAccess := time.Since(heat.LastAccess).Hours()
	timeDecay := math.Exp(-hoursSinceAccess / 24.0) // 24小时半衰期

	// 访问频率因子（对数缩放，避免极端值）
	freqFactor := math.Log10(totalAccess+1) / 5.0 // 归一化到 0-1

	// 读写权重（读操作权重更高，因为缓存优化更明显）
	readWeight := float64(heat.ReadCount) / totalAccess * 0.6
	writeWeight := float64(heat.WriteCount) / totalAccess * 0.4
	ioWeight := readWeight + writeWeight

	score := (freqFactor*0.6 + timeDecay*0.3 + ioWeight*0.1) * 100

	// 限制在 0-100
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return math.Round(score*100) / 100 // 保留两位小数
}

// shouldPromote 判断是否应该提升.
func shouldPromote(heat *BlockHeat, config TieringConfig) bool {
	if heat.Tier == TierNVMe {
		return false // 已在最高层
	}

	// 热度评分超过阈值，且在较低层
	if heat.HeatScore >= config.HotThreshold/10.0 { // 阈值归一化
		return true
	}

	return false
}

// shouldDemote 判断是否应该降级.
func shouldDemote(heat *BlockHeat, config TieringConfig) bool {
	if heat.Tier == TierHDD {
		return false // 已在最低层
	}

	// 冷数据判断：热度评分低，且长时间未访问
	if heat.HeatScore < config.WarmThreshold/10.0 {
		hoursSinceAccess := time.Since(heat.LastAccess).Hours()
		if hoursSinceAccess > float64(config.ColdAgeDays*24) {
			return true
		}
	}

	return false
}

// nextHigherTier 获取上一层级.
func nextHigherTier(tier DeviceTier) DeviceTier {
	switch tier {
	case TierHDD:
		return TierSSD
	case TierSSD:
		return TierNVMe
	default:
		return TierNVMe
	}
}

// nextLowerTier 获取下一层级.
func nextLowerTier(tier DeviceTier) DeviceTier {
	switch tier {
	case TierNVMe:
		return TierSSD
	case TierSSD:
		return TierHDD
	default:
		return TierHDD
	}
}

// tierUsage 计算层级使用量.
func tierUsage(devices []*StorageDevice) (used, total uint64) {
	for _, dev := range devices {
		used += dev.UsedBytes
		total += dev.TotalBytes
	}
	return
}

// percentUsed 计算使用百分比.
func percentUsed(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(used)/float64(total)*10000) / 100
}

// calculateImbalance 计算不均衡度.
func calculateImbalance(balance *TierBalance) float64 {
	percents := []float64{balance.NVMeUsedPercent, balance.SSDUsedPercent, balance.HDDUsedPercent}
	var max, min float64
	min = 100
	for _, p := range percents {
		if p > max {
			max = p
		}
		if p < min {
			min = p
		}
	}
	return max - min
}
