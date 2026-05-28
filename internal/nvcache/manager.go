// Package nvcache 提供 NVMe 缓存管理核心逻辑
package nvcache

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager NVMe 缓存管理器
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *CacheGlobalConfig
	devices       map[string]*CacheDevice
	pools         map[string]*CachePool
	mappings      map[string]*CacheMapping
	tierRules     map[string]*TierRule
	warmupTasks   map[string]*WarmupTask
	consistChecks map[string]*ConsistencyCheck
	stats         map[string]*CacheStats
	statsHistory  map[string][]*CacheStats
	stopChan      chan struct{}
	running       bool
}

// NewManager 创建 NVMe 缓存管理器
func NewManager(logger *zap.Logger, config *CacheGlobalConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultCacheGlobalConfig()
	}

	return &Manager{
		logger:        logger,
		config:        config,
		devices:       make(map[string]*CacheDevice),
		pools:         make(map[string]*CachePool),
		mappings:      make(map[string]*CacheMapping),
		tierRules:     make(map[string]*TierRule),
		warmupTasks:   make(map[string]*WarmupTask),
		consistChecks: make(map[string]*ConsistencyCheck),
		stats:         make(map[string]*CacheStats),
		statsHistory:  make(map[string][]*CacheStats),
		stopChan:      make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// RegisterDevice 注册缓存设备
func (m *Manager) RegisterDevice(req *RegisterDeviceRequest) (*CacheDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查路径是否已注册
	for _, d := range m.devices {
		if d.Path == req.Path {
			return nil, fmt.Errorf("device path %s already registered", req.Path)
		}
	}

	device := &CacheDevice{
		ID:           generateID(),
		Name:         req.Name,
		Path:         req.Path,
		Role:         req.Role,
		CapacityGB:   req.CapacityGB,
		IsActive:     true,
		HealthPercent: 100,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.devices[device.ID] = device
	m.logger.Info("device registered",
		zap.String("id", device.ID),
		zap.String("name", device.Name),
		zap.String("path", device.Path),
		zap.String("role", string(device.Role)))

	return device, nil
}

// GetDevice 获取设备信息
func (m *Manager) GetDevice(id string) (*CacheDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices(role DeviceRole) []*CacheDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*CacheDevice, 0)
	for _, d := range m.devices {
		if role == "" || d.Role == role {
			devices = append(devices, d)
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})

	return devices
}

// UnregisterDevice 注销设备
func (m *Manager) UnregisterDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("device not found: %s", id)
	}

	// 检查设备是否被缓存池使用
	for _, pool := range m.pools {
		for _, d := range pool.Devices {
			if d.ID == id {
				return fmt.Errorf("device %s is in use by pool %s", id, pool.ID)
			}
		}
	}

	delete(m.devices, id)
	m.logger.Info("device unregistered",
		zap.String("id", id),
		zap.String("name", device.Name))

	return nil
}

// CreatePool 创建缓存池
func (m *Manager) CreatePool(req *CreatePoolRequest) (*CachePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证缓存策略
	if !IsValidPolicy(req.Policy) {
		return nil, fmt.Errorf("invalid cache policy: %s", req.Policy)
	}
	if !IsValidEviction(req.EvictionPolicy) {
		return nil, fmt.Errorf("invalid eviction policy: %s", req.EvictionPolicy)
	}
	if !IsValidRAIDLevel(req.RAIDLevel) {
		return nil, fmt.Errorf("invalid RAID level: %s", req.RAIDLevel)
	}

	// 获取并验证设备
	devices := make([]*CacheDevice, 0, len(req.DeviceIDs))
	var totalCapacity int64
	for _, deviceID := range req.DeviceIDs {
		device, ok := m.devices[deviceID]
		if !ok {
			return nil, fmt.Errorf("device not found: %s", deviceID)
		}
		if device.Role != RoleCache {
			return nil, fmt.Errorf("device %s is not a cache device", deviceID)
		}
		if !device.IsActive {
			return nil, fmt.Errorf("device %s is not active", deviceID)
		}
		devices = append(devices, device)
		totalCapacity += device.CapacityGB
	}

	// RAID 级别验证
	if req.RAIDLevel != "" {
		minDevices := getMinDevicesForRAID(req.RAIDLevel)
		if len(devices) < minDevices {
			return nil, fmt.Errorf("RAID %s requires at least %d devices, got %d",
				req.RAIDLevel, minDevices, len(devices))
		}

		// 计算 RAID 后的有效容量
		totalCapacity = calculateRAIDCapacity(req.RAIDLevel, devices)
	}

	pool := &CachePool{
		ID:               generateID(),
		Name:             req.Name,
		Devices:          devices,
		RAIDLevel:        req.RAIDLevel,
		TotalCapacityGB:  totalCapacity,
		Policy:           req.Policy,
		EvictionPolicy:   req.EvictionPolicy,
		Status:           StatusActive,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	m.pools[pool.ID] = pool
	m.stats[pool.ID] = &CacheStats{
		CachePoolID: pool.ID,
		CollectedAt: time.Now(),
	}

	m.logger.Info("cache pool created",
		zap.String("id", pool.ID),
		zap.String("name", pool.Name),
		zap.Int64("capacity_gb", totalCapacity),
		zap.String("policy", string(req.Policy)))

	return pool, nil
}

// getMinDevicesForRAID 获取 RAID 级别最少设备数
func getMinDevicesForRAID(level RAIDLevel) int {
	switch level {
	case RAID0:
		return 2
	case RAID1:
		return 2
	case RAID5:
		return 3
	case RAID10:
		return 4
	default:
		return 1
	}
}

// calculateRAIDCapacity 计算 RAID 有效容量
func calculateRAIDCapacity(level RAIDLevel, devices []*CacheDevice) int64 {
	if len(devices) == 0 {
		return 0
	}

	// 找最小设备容量
	minCapacity := devices[0].CapacityGB
	for _, d := range devices {
		if d.CapacityGB < minCapacity {
			minCapacity = d.CapacityGB
		}
	}

	switch level {
	case RAID0:
		return minCapacity * int64(len(devices))
	case RAID1:
		return minCapacity
	case RAID5:
		return minCapacity * int64(len(devices)-1)
	case RAID10:
		return minCapacity * int64(len(devices)/2)
	default:
		return minCapacity * int64(len(devices))
	}
}

// GetPool 获取缓存池
func (m *Manager) GetPool(id string) (*CachePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("cache pool not found: %s", id)
	}
	return pool, nil
}

// ListPools 列出所有缓存池
func (m *Manager) ListPools() []*CachePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*CachePool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}

	sort.Slice(pools, func(i, j int) bool {
		return pools[i].CreatedAt.Before(pools[j].CreatedAt)
	})

	return pools
}

// DeletePool 删除缓存池
func (m *Manager) DeletePool(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[id]
	if !ok {
		return fmt.Errorf("cache pool not found: %s", id)
	}

	// 检查是否有映射使用此缓存池
	for _, mapping := range m.mappings {
		if mapping.CachePoolID == id && mapping.IsActive {
			return fmt.Errorf("pool %s has active mappings, remove them first", id)
		}
	}

	// 检查是否有进行中的预热任务
	for _, task := range m.warmupTasks {
		if task.CachePoolID == id && task.Status == StatusActive {
			return fmt.Errorf("pool %s has active warmup tasks", id)
		}
	}

	delete(m.pools, id)
	delete(m.stats, id)
	delete(m.statsHistory, id)

	m.logger.Info("cache pool deleted",
		zap.String("id", id),
		zap.String("name", pool.Name))

	return nil
}

// UpdatePoolPolicy 更新缓存池策略
func (m *Manager) UpdatePoolPolicy(id string, req *UpdatePolicyRequest) (*CachePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("cache pool not found: %s", id)
	}

	if !IsValidPolicy(req.Policy) {
		return nil, fmt.Errorf("invalid cache policy: %s", req.Policy)
	}

	pool.Policy = req.Policy
	if req.EvictionPolicy != "" {
		if !IsValidEviction(req.EvictionPolicy) {
			return nil, fmt.Errorf("invalid eviction policy: %s", req.EvictionPolicy)
		}
		pool.EvictionPolicy = req.EvictionPolicy
	}
	pool.UpdatedAt = time.Now()

	m.logger.Info("pool policy updated",
		zap.String("pool_id", id),
		zap.String("policy", string(req.Policy)))

	return pool, nil
}

// CreateMapping 创建缓存映射
func (m *Manager) CreateMapping(req *CreateMappingRequest) (*CacheMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[req.CachePoolID]
	if !ok {
		return nil, fmt.Errorf("cache pool not found: %s", req.CachePoolID)
	}

	if pool.Status != StatusActive {
		return nil, fmt.Errorf("cache pool %s is not active", req.CachePoolID)
	}

	// 检查后端设备是否已被映射
	for _, mapping := range m.mappings {
		if mapping.BackendDevice == req.BackendDevice && mapping.IsActive {
			return nil, fmt.Errorf("backend device %s already mapped", req.BackendDevice)
		}
	}

	// 验证缓存策略
	policy := req.Policy
	if policy == "" {
		policy = pool.Policy
	}
	if !IsValidPolicy(policy) {
		return nil, fmt.Errorf("invalid cache policy: %s", policy)
	}

	// 默认块大小
	blockSize := req.BlockSizeKB
	if blockSize == 0 {
		blockSize = m.config.DefaultBlockSizeKB
	}

	mapping := &CacheMapping{
		ID:            generateID(),
		CachePoolID:   req.CachePoolID,
		BackendDevice: req.BackendDevice,
		MountPoint:    req.MountPoint,
		Policy:        policy,
		BlockSizeKB:   blockSize,
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	m.mappings[mapping.ID] = mapping
	pool.UsedCapacityGB += 1 // 简化模拟

	m.logger.Info("cache mapping created",
		zap.String("id", mapping.ID),
		zap.String("pool_id", req.CachePoolID),
		zap.String("backend", req.BackendDevice),
		zap.String("mount", req.MountPoint))

	return mapping, nil
}

// GetMapping 获取缓存映射
func (m *Manager) GetMapping(id string) (*CacheMapping, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mapping, ok := m.mappings[id]
	if !ok {
		return nil, fmt.Errorf("mapping not found: %s", id)
	}
	return mapping, nil
}

// ListMappings 列出缓存映射
func (m *Manager) ListMappings(poolID string) []*CacheMapping {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mappings := make([]*CacheMapping, 0)
	for _, mapping := range m.mappings {
		if poolID == "" || mapping.CachePoolID == poolID {
			mappings = append(mappings, mapping)
		}
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].CreatedAt.Before(mappings[j].CreatedAt)
	})

	return mappings
}

// DeleteMapping 删除缓存映射
func (m *Manager) DeleteMapping(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapping, ok := m.mappings[id]
	if !ok {
		return fmt.Errorf("mapping not found: %s", id)
	}

	pool, poolOk := m.pools[mapping.CachePoolID]
	if poolOk && pool.UsedCapacityGB > 0 {
		pool.UsedCapacityGB -= 1
	}

	delete(m.mappings, id)

	m.logger.Info("cache mapping deleted",
		zap.String("id", id),
		zap.String("backend", mapping.BackendDevice))

	return nil
}

// CreateTierRule 创建分层规则
func (m *Manager) CreateTierRule(req *CreateTierRuleRequest) (*TierRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &TierRule{
		ID:                generateID(),
		Name:              req.Name,
		Description:       req.Description,
		HotThreshold:      req.HotThreshold,
		ColdThreshold:     req.ColdThreshold,
		PromoteEnabled:    req.PromoteEnabled,
		DemoteEnabled:     req.DemoteEnabled,
		PromoteScheduleMB: req.PromoteScheduleMB,
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if rule.PromoteScheduleMB == 0 {
		rule.PromoteScheduleMB = 256 // 默认 256MB
	}

	m.tierRules[rule.ID] = rule

	m.logger.Info("tier rule created",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name))

	return rule, nil
}

// GetTierRule 获取分层规则
func (m *Manager) GetTierRule(id string) (*TierRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.tierRules[id]
	if !ok {
		return nil, fmt.Errorf("tier rule not found: %s", id)
	}
	return rule, nil
}

// ListTierRules 列出分层规则
func (m *Manager) ListTierRules() []*TierRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*TierRule, 0, len(m.tierRules))
	for _, r := range m.tierRules {
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})

	return rules
}

// UpdateTierRule 更新分层规则
func (m *Manager) UpdateTierRule(id string, req *CreateTierRuleRequest) (*TierRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.tierRules[id]
	if !ok {
		return nil, fmt.Errorf("tier rule not found: %s", id)
	}

	rule.Name = req.Name
	rule.Description = req.Description
	rule.HotThreshold = req.HotThreshold
	rule.ColdThreshold = req.ColdThreshold
	rule.PromoteEnabled = req.PromoteEnabled
	rule.DemoteEnabled = req.DemoteEnabled
	rule.PromoteScheduleMB = req.PromoteScheduleMB
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// DeleteTierRule 删除分层规则
func (m *Manager) DeleteTierRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tierRules[id]; !ok {
		return fmt.Errorf("tier rule not found: %s", id)
	}

	delete(m.tierRules, id)
	return nil
}

// CreateWarmupTask 创建缓存预热任务
func (m *Manager) CreateWarmupTask(req *CreateWarmupRequest) (*WarmupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证缓存池存在
	pool, ok := m.pools[req.CachePoolID]
	if !ok {
		return nil, fmt.Errorf("cache pool not found: %s", req.CachePoolID)
	}

	if pool.Status != StatusActive {
		return nil, fmt.Errorf("cache pool %s is not active", req.CachePoolID)
	}

	task := &WarmupTask{
		ID:           generateID(),
		Name:         req.Name,
		CachePoolID:  req.CachePoolID,
		SourcePath:   req.SourcePath,
		FilePattern:  req.FilePattern,
		Status:       StatusActive,
		StartedAt:    time.Now(),
		CreatedAt:    time.Now(),
	}

	m.warmupTasks[task.ID] = task

	// 模拟预热过程
	go m.executeWarmupTask(task)

	m.logger.Info("warmup task created",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.String("source", req.SourcePath))

	return task, nil
}

// executeWarmupTask 执行预热任务
func (m *Manager) executeWarmupTask(task *WarmupTask) {
	// 模拟预热过程
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	task.Status = StatusActive
	task.TotalFiles = 100
	task.TotalBytes = 1024 * 1024 * 512 // 512MB
	task.WarmedFiles = 100
	task.WarmedBytes = task.TotalBytes
	task.CompletedAt = time.Now()
	m.mu.Unlock()

	m.logger.Info("warmup task completed",
		zap.String("id", task.ID),
		zap.Int("files", task.WarmedFiles))
}

// GetWarmupTask 获取预热任务
func (m *Manager) GetWarmupTask(id string) (*WarmupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.warmupTasks[id]
	if !ok {
		return nil, fmt.Errorf("warmup task not found: %s", id)
	}
	return task, nil
}

// ListWarmupTasks 列出预热任务
func (m *Manager) ListWarmupTasks(poolID string) []*WarmupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*WarmupTask, 0)
	for _, t := range m.warmupTasks {
		if poolID == "" || t.CachePoolID == poolID {
			tasks = append(tasks, t)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})

	return tasks
}

// CancelWarmupTask 取消预热任务
func (m *Manager) CancelWarmupTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.warmupTasks[id]
	if !ok {
		return fmt.Errorf("warmup task not found: %s", id)
	}

	if task.Status != StatusActive {
		return fmt.Errorf("warmup task %s is not active", id)
	}

	task.Status = StatusInactive
	task.Error = "cancelled by user"

	m.logger.Info("warmup task cancelled", zap.String("id", id))
	return nil
}

// StartConsistencyCheck 启动一致性检查
func (m *Manager) StartConsistencyCheck(poolID string) (*ConsistencyCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("cache pool not found: %s", poolID)
	}

	if pool.Status != StatusActive {
		return nil, fmt.Errorf("cache pool %s is not active", poolID)
	}

	// 检查是否已有进行中的检查
	for _, check := range m.consistChecks {
		if check.CachePoolID == poolID && check.Status == StatusActive {
			return nil, fmt.Errorf("consistency check already running for pool %s", poolID)
		}
	}

	check := &ConsistencyCheck{
		ID:           generateID(),
		CachePoolID:  poolID,
		Status:       StatusActive,
		TotalBlocks:  pool.TotalCapacityGB * 1024 * 4, // 模拟块数
		StartedAt:    time.Now(),
	}

	m.consistChecks[check.ID] = check

	// 模拟一致性检查过程
	go m.executeConsistencyCheck(check)

	m.logger.Info("consistency check started",
		zap.String("id", check.ID),
		zap.String("pool_id", poolID))

	return check, nil
}

// executeConsistencyCheck 执行一致性检查
func (m *Manager) executeConsistencyCheck(check *ConsistencyCheck) {
	// 模拟检查过程
	time.Sleep(3 * time.Second)

	m.mu.Lock()
	check.Status = StatusActive
	check.CheckedBlocks = check.TotalBlocks
	check.InconsistentBlocks = int64(float64(check.TotalBlocks) * 0.001) // 0.1% 不一致
	check.RepairedBlocks = check.InconsistentBlocks
	check.CompletedAt = time.Now()
	m.mu.Unlock()

	m.logger.Info("consistency check completed",
		zap.String("id", check.ID),
		zap.Int64("inconsistent", check.InconsistentBlocks))
}

// GetConsistencyCheck 获取一致性检查结果
func (m *Manager) GetConsistencyCheck(id string) (*ConsistencyCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	check, ok := m.consistChecks[id]
	if !ok {
		return nil, fmt.Errorf("consistency check not found: %s", id)
	}
	return check, nil
}

// ListConsistencyChecks 列出一致性检查结果
func (m *Manager) ListConsistencyChecks(poolID string) []*ConsistencyCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	checks := make([]*ConsistencyCheck, 0)
	for _, c := range m.consistChecks {
		if poolID == "" || c.CachePoolID == poolID {
			checks = append(checks, c)
		}
	}

	sort.Slice(checks, func(i, j int) bool {
		return checks[i].StartedAt.After(checks[j].StartedAt)
	})

	return checks
}

// GetStats 获取缓存统计信息
func (m *Manager) GetStats(poolID string) (*CacheStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, ok := m.stats[poolID]
	if !ok {
		return nil, fmt.Errorf("stats not found for pool: %s", poolID)
	}

	// 更新统计信息
	stats.HitRate = calculateHitRate(stats.HitCount, stats.MissCount)
	stats.CollectedAt = time.Now()

	return stats, nil
}

// calculateHitRate 计算命中率
func calculateHitRate(hit, miss int64) float64 {
	total := hit + miss
	if total == 0 {
		return 0
	}
	return math.Round(float64(hit)/float64(total)*10000) / 100
}

// GetStatsHistory 获取统计历史
func (m *Manager) GetStatsHistory(poolID string, limit int) []*CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.statsHistory[poolID]
	if !ok {
		return nil
	}

	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	start := len(history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*CacheStats, limit)
	copy(result, history[start:])
	return result
}

// UpdateStats 更新缓存统计（模拟 IO 操作时调用）
func (m *Manager) UpdateStats(poolID string, hit bool, isRead bool, bytes int64, latencyUs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, ok := m.stats[poolID]
	if !ok {
		return
	}

	if hit {
		stats.HitCount++
		if isRead {
			stats.ReadHitCount++
		} else {
			stats.WriteHitCount++
		}
	} else {
		stats.MissCount++
		if isRead {
			stats.ReadMissCount++
		} else {
			stats.WriteMissCount++
		}
	}

	if isRead {
		stats.TotalReadBytes += bytes
	} else {
		stats.TotalWriteBytes += bytes
	}

	// 更新平均延迟
	totalIOs := stats.HitCount + stats.MissCount
	if totalIOs > 0 {
		stats.AverageLatencyUs = (stats.AverageLatencyUs*float64(totalIOs-1) + latencyUs) / float64(totalIOs)
	}

	// 更新 IOPS 和带宽
	stats.IOPS = totalIOs
	stats.BandwidthMBps = float64(stats.TotalReadBytes+stats.TotalWriteBytes) / 1024 / 1024
}

// FlushCache 刷回缓存中的脏数据
func (m *Manager) FlushCache(ctx context.Context, req *FlushRequest) error {
	m.mu.RLock()
	pool, ok := m.pools[req.CachePoolID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("cache pool not found: %s", req.CachePoolID)
	}

	if pool.Policy != PolicyWriteBack {
		m.mu.RUnlock()
		return fmt.Errorf("flush only applicable for write-back policy")
	}
	m.mu.RUnlock()

	m.mu.Lock()
	pool.Status = StatusSyncing
	pool.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.logger.Info("cache flush started",
		zap.String("pool_id", req.CachePoolID),
		zap.Bool("force", req.Force))

	// 模拟刷回过程
	time.Sleep(1 * time.Second)

	m.mu.Lock()
	pool.Status = StatusActive
	pool.UpdatedAt = time.Now()

	stats, ok := m.stats[req.CachePoolID]
	if ok {
		stats.DirtyBlocks = 0
		stats.DirtyBytes = 0
	}
	m.mu.Unlock()

	m.logger.Info("cache flush completed", zap.String("pool_id", req.CachePoolID))
	return nil
}

// InvalidateCache 失效指定缓存
func (m *Manager) InvalidateCache(poolID string, paths []string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.pools[poolID]
	if !ok {
		return fmt.Errorf("cache pool not found: %s", poolID)
	}

	m.logger.Info("cache invalidated",
		zap.String("pool_id", poolID),
		zap.Int("paths", len(paths)))

	return nil
}

// GetConfig 获取全局配置
func (m *Manager) GetConfig() *CacheGlobalConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新全局配置
func (m *Manager) UpdateConfig(cfg *CacheGlobalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetSupportedPolicies 获取支持的缓存策略
func (m *Manager) GetSupportedPolicies() []CachePolicy {
	return SupportedPolicies()
}

// GetSupportedEvictions 获取支持的淘汰策略
func (m *Manager) GetSupportedEvictions() []EvictionPolicy {
	return SupportedEvictions()
}

// GetSupportedRAIDLevels 获取支持的 RAID 级别
func (m *Manager) GetSupportedRAIDLevels() []RAIDLevel {
	return SupportedRAIDLevels()
}

// GetSystemOverview 获取系统概览
func (m *Manager) GetSystemOverview() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalPools := len(m.pools)
	activePools := 0
	totalCapacity := int64(0)
	usedCapacity := int64(0)
	totalDevices := len(m.devices)
	cacheDevices := 0
	totalMappings := len(m.mappings)

	for _, pool := range m.pools {
		if pool.Status == StatusActive {
			activePools++
		}
		totalCapacity += pool.TotalCapacityGB
		usedCapacity += pool.UsedCapacityGB
	}

	for _, device := range m.devices {
		if device.Role == RoleCache {
			cacheDevices++
		}
	}

	// 计算总体命中率
	totalHit := int64(0)
	totalMiss := int64(0)
	for _, stats := range m.stats {
		totalHit += stats.HitCount
		totalMiss += stats.MissCount
	}
	overallHitRate := calculateHitRate(totalHit, totalMiss)

	return map[string]interface{}{
		"total_pools":       totalPools,
		"active_pools":      activePools,
		"total_capacity_gb": totalCapacity,
		"used_capacity_gb":  usedCapacity,
		"usage_percent":     calculateUsagePercent(usedCapacity, totalCapacity),
		"total_devices":     totalDevices,
		"cache_devices":     cacheDevices,
		"total_mappings":    totalMappings,
		"overall_hit_rate":  overallHitRate,
		"total_hit_count":   totalHit,
		"total_miss_count":  totalMiss,
	}
}

// calculateUsagePercent 计算使用率百分比
func calculateUsagePercent(used, total int64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(used)/float64(total)*10000) / 100
}

// SearchDevices 搜索设备
func (m *Manager) SearchDevices(keyword string) []*CacheDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	results := make([]*CacheDevice, 0)

	for _, device := range m.devices {
		if strings.Contains(strings.ToLower(device.Name), keyword) ||
			strings.Contains(strings.ToLower(device.Path), keyword) ||
			strings.Contains(strings.ToLower(device.Model), keyword) {
			results = append(results, device)
		}
	}

	return results
}
