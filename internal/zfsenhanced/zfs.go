package zfsenhanced

import (
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"time"
)

// ============================================================================
// 引擎构造与生命周期
// ============================================================================

// NewEngine 创建 ZFS 增强引擎
func NewEngine(config *EngineConfig, backend StorageBackend, logger Logger) *Engine {
	if config == nil {
		config = DefaultEngineConfig()
	}
	if logger == nil {
		logger = &nopLogger{}
	}

	return &Engine{
		config:     config,
		logger:     logger,
		backend:    backend,
		pools:      make(map[string]*Pool),
		snapshots:  make(map[string]*Snapshot),
		dedupTable: make(map[string]*DedupEntry),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})

	// 启动定期 Scrub 调度
	if e.config.Scrub != nil && e.config.Scrub.Enabled {
		e.wg.Add(1)
		go e.scrubScheduler()
	}

	e.logger.Info("zfs enhanced engine started")
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	e.wg.Wait()
	e.logger.Info("zfs enhanced engine stopped")
}

// IsRunning 返回引擎运行状态
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// ============================================================================
// 池管理
// ============================================================================

// CreatePool 创建存储池
func (e *Engine) CreatePool(config *PoolConfig, disks []Disk) (*Pool, error) {
	if config == nil {
		return nil, fmt.Errorf("pool config is required")
	}
	if len(disks) == 0 {
		return nil, ErrNoDataDisks
	}

	// 验证磁盘数量
	if err := e.validateDiskCount(disks, config); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查池名重复
	for _, p := range e.pools {
		if p.Name == config.Name {
			return nil, ErrPoolExists
		}
	}

	// 检查磁盘是否已被使用
	for _, disk := range disks {
		if e.isDiskInUse(disk.ID) {
			return nil, fmt.Errorf("%w: %s", ErrDiskAlreadyInUse, disk.ID)
		}
	}

	poolID := generateID("pool")
	now := time.Now()

	pool := &Pool{
		ID:          poolID,
		Name:        config.Name,
		State:       VDevStateOnline,
		TotalBytes:  calculateTotalBytes(disks),
		FreeBytes:   calculateTotalBytes(disks),
		Compression: CompressionStats{Algorithm: config.Compression, Level: config.CompressionLvl},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	e.pools[poolID] = pool
	e.logger.Info("pool created", "id", poolID, "name", config.Name, "disks", len(disks))

	return pool, nil
}

// GetPool 获取存储池
func (e *Engine) GetPool(poolID string) (*Pool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pool, ok := e.pools[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// ListPools 列出所有存储池
func (e *Engine) ListPools() []*Pool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pools := make([]*Pool, 0, len(e.pools))
	for _, p := range e.pools {
		pools = append(pools, p)
	}
	return pools
}

// DeletePool 删除存储池
func (e *Engine) DeletePool(poolID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.pools[poolID]; !ok {
		return ErrPoolNotFound
	}

	delete(e.pools, poolID)
	e.logger.Info("pool deleted", "id", poolID)
	return nil
}

// ============================================================================
// RAID-Z 在线扩展
// ============================================================================

// ExpandRAIDZ RAID-Z 在线扩展
func (e *Engine) ExpandRAIDZ(req *ExpandRequest) (*ExpandResult, error) {
	if req == nil || req.VDevID == "" {
		return nil, fmt.Errorf("vdev_id is required")
	}
	if len(req.DiskIDs) == 0 {
		return nil, ErrNoDataDisks
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 查找 VDev
	pool, vdev, err := e.findVDev(req.VDevID)
	if err != nil {
		return nil, err
	}

	// 检查 VDev 状态
	if vdev.State == VDevStateExpanding {
		return nil, fmt.Errorf("vdev is already expanding")
	}

	// 验证新增磁盘数量（RAID-Z 扩展需要完整的条带）
	requiredDisks := int(vdev.Type) + 1 // 数据盘 + 校验盘
	if len(req.DiskIDs)%requiredDisks != 0 {
		return nil, fmt.Errorf("disk count must be multiple of %d for %s", requiredDisks, vdev.Type)
	}

	// 检查磁盘是否已被使用
	for _, diskID := range req.DiskIDs {
		if e.isDiskInUse(diskID) {
			return nil, fmt.Errorf("%w: %s", ErrDiskAlreadyInUse, diskID)
		}
	}

	// 模拟扩展操作
	vdev.State = VDevStateExpanding
	vdev.UpdatedAt = time.Now()

	// 计算新增容量
	var addedCapacity int64
	for _, diskID := range req.DiskIDs {
		// 模拟磁盘信息
		disk := Disk{
			ID:        diskID,
			State:     DiskStateOnline,
			SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, // 4TB 默认
		}
		vdev.Disks = append(vdev.Disks, disk)
		addedCapacity += disk.SizeBytes
	}

	// 更新池容量
	pool.TotalBytes += addedCapacity
	pool.FreeBytes += addedCapacity
	pool.UpdatedAt = time.Now()

	// 更新扩展进度（模拟）
	eta := time.Now().Add(30 * time.Minute)

	result := &ExpandResult{
		Success:      true,
		VDevID:       req.VDevID,
		DisksAdded:   len(req.DiskIDs),
		NewCapacity:  pool.TotalBytes,
		ExpandStatus: "expanding",
		StartedAt:    time.Now(),
		ETA:          &eta,
	}

	e.logger.Info("raidz expansion started",
		"vdev", req.VDevID,
		"disks_added", len(req.DiskIDs),
		"new_capacity", pool.TotalBytes)

	return result, nil
}

// GetExpansionStatus 获取扩展状态
func (e *Engine) GetExpansionStatus(vdevID string) (VDevState, float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	_, vdev, err := e.findVDev(vdevID)
	if err != nil {
		return "", 0, err
	}

	return vdev.State, vdev.ExpandPct, nil
}

// ============================================================================
// 快速去重 (Fast-Dedup)
// ============================================================================

// EnableDedup 启用去重
func (e *Engine) EnableDedup(config *DedupConfig) error {
	if config == nil {
		config = DefaultDedupConfig()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Dedup != nil && e.config.Dedup.Enabled {
		e.logger.Warn("dedup already enabled, updating config")
	}

	e.config.Dedup = config
	e.logger.Info("dedup enabled", "policy", config.Policy, "min_block", config.MinBlockSize)
	return nil
}

// DisableDedup 禁用去重
func (e *Engine) DisableDedup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Dedup != nil {
		e.config.Dedup.Enabled = false
	}
	e.dedupTable = make(map[string]*DedupEntry)
	e.logger.Info("dedup disabled")
	return nil
}

// ProcessDedup 处理数据块去重
func (e *Engine) ProcessDedup(data []byte) (string, bool, error) {
	e.mu.RLock()
	config := e.config.Dedup
	e.mu.RUnlock()

	if config == nil || !config.Enabled {
		return "", false, ErrDedupDisabled
	}

	if int64(len(data)) < config.MinBlockSize {
		return "", false, nil
	}

	// 计算哈希
	hash, err := e.computeHash(data, config.Policy)
	if err != nil {
		return "", false, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查是否已存在
	if entry, ok := e.dedupTable[hash]; ok {
		entry.RefCount++
		entry.LastSeen = time.Now()
		return hash, true, nil // 已去重
	}

	// 新块
	entry := &DedupEntry{
		Hash:      hash,
		Algorithm: config.Policy,
		Size:      int64(len(data)),
		RefCount:  1,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
		BlockPath: fmt.Sprintf("dedup/%s", hash[:8]),
	}
	e.dedupTable[hash] = entry

	return hash, false, nil
}

// GetDedupStats 获取去重统计
func (e *Engine) GetDedupStats() *DedupStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &DedupStats{
		Enabled: e.config.Dedup != nil && e.config.Dedup.Enabled,
	}

	if e.config.Dedup != nil {
		stats.Policy = e.config.Dedup.Policy
	}

	var totalBlocks, uniqueBlocks, savedBytes int64
	for _, entry := range e.dedupTable {
		uniqueBlocks++
		totalBlocks += entry.RefCount
		if entry.RefCount > 1 {
			savedBytes += entry.Size * (entry.RefCount - 1)
		}
	}

	stats.TotalBlocks = totalBlocks
	stats.UniqueBlocks = uniqueBlocks
	stats.SavedBytes = savedBytes
	stats.ActiveEntries = int64(len(e.dedupTable))
	stats.DedupTableSize = int64(len(e.dedupTable))

	if totalBlocks > 0 && uniqueBlocks > 0 {
		stats.DedupRatio = float64(totalBlocks) / float64(uniqueBlocks)
	}

	return stats
}

// computeHash 计算数据哈希
func (e *Engine) computeHash(data []byte, policy DedupPolicy) (string, error) {
	switch policy {
	case DedupPolicySHA256:
		hash := sha256.Sum256(data)
		return fmt.Sprintf("%x", hash), nil
	case DedupPolicyXXHash:
		h := fnv.New64a()
		h.Write(data)
		return fmt.Sprintf("%016x", h.Sum64()), nil
	case DedupPolicyMurmur3:
		h := fnv.New32a()
		h.Write(data)
		return fmt.Sprintf("%08x", h.Sum32()), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidHash, policy)
	}
}

// ============================================================================
// 混合存储池优化
// ============================================================================

// CreateHybridPool 创建混合存储池
func (e *Engine) CreateHybridPool(config *PoolConfig, dataDisks []Disk, specialDisks []Disk, cacheDisks []Disk) (*HybridPool, error) {
	if config == nil {
		return nil, fmt.Errorf("pool config is required")
	}
	if len(dataDisks) == 0 {
		return nil, ErrNoDataDisks
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	poolID := generateID("hybrid")
	now := time.Now()

	pool := &HybridPool{
		ID:        poolID,
		Name:      config.Name,
		State:     VDevStateOnline,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 创建数据 VDev
	dataVDev := VDev{
		ID:         generateID("vdev"),
		Name:       "data",
		Type:       RAIDZ2,
		State:      VDevStateOnline,
		TotalBytes: calculateTotalBytes(dataDisks),
		FreeBytes:  calculateTotalBytes(dataDisks),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	for _, disk := range dataDisks {
		dataVDev.Disks = append(dataVDev.Disks, disk)
	}
	pool.DataVDevs = append(pool.DataVDevs, dataVDev)

	pool.TotalBytes = dataVDev.TotalBytes
	pool.FreeBytes = dataVDev.FreeBytes

	// 创建 Special VDev（SSD 元数据加速）
	if len(specialDisks) > 0 {
		specialVDev := VDev{
			ID:         generateID("vdev"),
			Name:       "special",
			State:      VDevStateOnline,
			TotalBytes: calculateTotalBytes(specialDisks),
			FreeBytes:  calculateTotalBytes(specialDisks),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		for _, disk := range specialDisks {
			specialVDev.Disks = append(specialVDev.Disks, disk)
		}
		pool.SpecialVDev = &specialVDev
	}

	// 创建 Cache VDev（L2ARC 读缓存）
	if len(cacheDisks) > 0 {
		cacheVDev := VDev{
			ID:         generateID("vdev"),
			Name:       "cache",
			State:      VDevStateOnline,
			TotalBytes: calculateTotalBytes(cacheDisks),
			FreeBytes:  calculateTotalBytes(cacheDisks),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		for _, disk := range cacheDisks {
			cacheVDev.Disks = append(cacheVDev.Disks, disk)
		}
		pool.CacheVDev = &cacheVDev
	}

	e.logger.Info("hybrid pool created",
		"id", poolID,
		"name", config.Name,
		"data_disks", len(dataDisks),
		"special_disks", len(specialDisks),
		"cache_disks", len(cacheDisks))

	return pool, nil
}

// GetTieringRecommendation 获取分层建议
func (e *Engine) GetTieringRecommendation(fileSize int64, accessCount int64, lastAccess time.Time) DeviceClass {
	e.mu.RLock()
	policy := e.config.Tiering
	e.mu.RUnlock()

	if policy == nil || !policy.Enabled {
		return DeviceClassData
	}

	// 计算热度分数
	heatScore := e.calculateHeatScore(accessCount, lastAccess)

	if heatScore >= policy.HotThreshold && policy.SSDTier {
		return DeviceClassSpecial // SSD 元数据
	}

	if heatScore >= policy.WarmThreshold {
		return DeviceClassCache // 缓存
	}

	return DeviceClassData // HDD 数据
}

// calculateHeatScore 计算热度分数
func (e *Engine) calculateHeatScore(accessCount int64, lastAccess time.Time) float64 {
	hoursSinceAccess := time.Since(lastAccess).Hours()

	// 访问频率得分 (0-50)
	var freqScore float64
	if accessCount > 0 {
		freqScore = float64(accessCount) * 0.5
		if freqScore > 50 {
			freqScore = 50
		}
	}

	// 时间衰减得分 (0-50)
	var timeScore float64
	if hoursSinceAccess < 24 {
		timeScore = 50
	} else if hoursSinceAccess < 168 { // 1 week
		timeScore = 50 - (hoursSinceAccess/168)*50
	}

	return freqScore + timeScore
}

// ============================================================================
// 数据完整性校验
// ============================================================================

// StartScrub 启动 Scrub
func (e *Engine) StartScrub(poolID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pool, ok := e.pools[poolID]
	if !ok {
		return ErrPoolNotFound
	}

	if pool.ScrubStatus.State == ScrubStateRunning {
		return ErrScrubAlreadyRunning
	}

	now := time.Now()
	pool.ScrubStatus = ScrubStatus{
		State:     ScrubStateRunning,
		StartTime: &now,
		Progress:  0,
	}

	// 模拟 Scrub 启动
	go e.simulateScrub(poolID)

	e.logger.Info("scrub started", "pool", poolID)
	return nil
}

// StopScrub 停止 Scrub
func (e *Engine) StopScrub(poolID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pool, ok := e.pools[poolID]
	if !ok {
		return ErrPoolNotFound
	}

	if pool.ScrubStatus.State != ScrubStateRunning {
		return fmt.Errorf("no scrub running")
	}

	pool.ScrubStatus.State = ScrubStatePaused
	e.logger.Info("scrub paused", "pool", poolID)
	return nil
}

// GetScrubStatus 获取 Scrub 状态
func (e *Engine) GetScrubStatus(poolID string) (*ScrubStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pool, ok := e.pools[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}

	status := pool.ScrubStatus
	return &status, nil
}

// simulateScrub 模拟 Scrub 过程
func (e *Engine) simulateScrub(poolID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.Lock()
			pool, ok := e.pools[poolID]
			if !ok || pool.ScrubStatus.State != ScrubStateRunning {
				e.mu.Unlock()
				return
			}

			pool.ScrubStatus.Progress += 5.0
			pool.ScrubStatus.BytesScanned = int64(float64(pool.TotalBytes) * pool.ScrubStatus.Progress / 100)
			pool.ScrubStatus.ScanRate = 100.0 // MB/s

			if pool.ScrubStatus.Progress >= 100 {
				now := time.Now()
				pool.ScrubStatus.State = ScrubStateFinished
				pool.ScrubStatus.EndTime = &now
				pool.ScrubStatus.Progress = 100
				e.logger.Info("scrub finished", "pool", poolID)
				e.mu.Unlock()
				return
			}

			e.mu.Unlock()
		}
	}
}

// VerifyChecksum 验证校验和
func (e *Engine) VerifyChecksum(data []byte, expected Checksum) (bool, error) {
	actual := e.computeChecksum(data)

	if actual.Value != expected.Value {
		return false, fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expected.Value, actual.Value)
	}

	return true, nil
}

// computeChecksum 计算校验和
func (e *Engine) computeChecksum(data []byte) Checksum {
	hash := sha256.Sum256(data)
	return Checksum{
		Algorithm: "sha256",
		Value:     fmt.Sprintf("%x", hash),
		Size:      int64(len(data)),
	}
}

// ============================================================================
// 快照管理
// ============================================================================

// CreateSnapshot 创建快照
func (e *Engine) CreateSnapshot(req *SnapshotCreateRequest) (*Snapshot, error) {
	if req == nil {
		return nil, fmt.Errorf("snapshot request is required")
	}
	if req.Pool == "" || req.Name == "" {
		return nil, fmt.Errorf("pool and name are required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 验证池存在
	pool, ok := e.pools[req.Pool]
	if !ok {
		return nil, ErrPoolNotFound
	}

	// 检查快照名重复
	fullName := fmt.Sprintf("%s/%s@%s", req.Pool, req.Dataset, req.Name)
	for _, s := range e.snapshots {
		if s.FullName == fullName {
			return nil, fmt.Errorf("snapshot already exists: %s", fullName)
		}
	}

	now := time.Now()
	snapshot := &Snapshot{
		ID:        generateID("snap"),
		Name:      req.Name,
		Pool:      req.Pool,
		Dataset:   req.Dataset,
		FullName:  fullName,
		CreatedAt: now,
	}

	e.snapshots[snapshot.ID] = snapshot
	pool.SnapshotCount++
	pool.UpdatedAt = now

	e.logger.Info("snapshot created", "id", snapshot.ID, "name", fullName)
	return snapshot, nil
}

// DeleteSnapshot 删除快照
func (e *Engine) DeleteSnapshot(snapshotID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	snapshot, ok := e.snapshots[snapshotID]
	if !ok {
		return ErrSnapshotNotFound
	}

	// 检查是否有克隆
	if len(snapshot.Clones) > 0 {
		return ErrSnapshotHasClones
	}

	// 更新池统计
	if pool, ok := e.pools[snapshot.Pool]; ok {
		pool.SnapshotCount--
		pool.UpdatedAt = time.Now()
	}

	delete(e.snapshots, snapshotID)
	e.logger.Info("snapshot deleted", "id", snapshotID)
	return nil
}

// RollbackSnapshot 回滚到快照
func (e *Engine) RollbackSnapshot(req *RollbackRequest) error {
	if req == nil {
		return fmt.Errorf("rollback request is required")
	}

	e.mu.RLock()
	snapshot, ok := e.snapshots[req.SnapshotID]
	e.mu.RUnlock()

	if !ok {
		return ErrSnapshotNotFound
	}

	// 检查是否有克隆且未强制
	if len(snapshot.Clones) > 0 && !req.Force {
		return ErrSnapshotHasClones
	}

	e.logger.Info("snapshot rollback", "id", req.SnapshotID, "name", snapshot.FullName)
	return nil
}

// CloneSnapshot 克隆快照
func (e *Engine) CloneSnapshot(req *CloneRequest) (*Snapshot, error) {
	if req == nil {
		return nil, fmt.Errorf("clone request is required")
	}
	if req.SnapshotID == "" || req.TargetName == "" {
		return nil, fmt.Errorf("snapshot_id and target_name are required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	origin, ok := e.snapshots[req.SnapshotID]
	if !ok {
		return nil, ErrSnapshotNotFound
	}

	// 验证目标池存在
	targetPool := req.TargetPool
	if targetPool == "" {
		targetPool = origin.Pool
	}
	if _, ok := e.pools[targetPool]; !ok {
		return nil, ErrPoolNotFound
	}

	now := time.Now()
	clone := &Snapshot{
		ID:        generateID("clone"),
		Name:      req.TargetName,
		Pool:      targetPool,
		Dataset:   req.TargetName,
		FullName:  fmt.Sprintf("%s/%s", targetPool, req.TargetName),
		IsClone:   true,
		Origin:    origin.FullName,
		CreatedAt: now,
	}

	e.snapshots[clone.ID] = clone
	origin.Clones = append(origin.Clones, clone.ID)

	e.logger.Info("snapshot cloned",
		"origin", origin.FullName,
		"clone", clone.FullName)
	return clone, nil
}

// ListSnapshots 列出快照
func (e *Engine) ListSnapshots(poolID string) []*Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Snapshot
	for _, s := range e.snapshots {
		if poolID == "" || s.Pool == poolID {
			result = append(result, s)
		}
	}
	return result
}

// GetSnapshotStats 获取快照统计
func (e *Engine) GetSnapshotStats() *SnapshotStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &SnapshotStats{
		TotalSnapshots: int64(len(e.snapshots)),
	}

	var oldest, newest time.Time
	var cloneCount int64

	for _, s := range e.snapshots {
		if s.IsClone {
			cloneCount++
		}
		if oldest.IsZero() || s.CreatedAt.Before(oldest) {
			oldest = s.CreatedAt
			stats.OldestSnapshot = s.FullName
		}
		if newest.IsZero() || s.CreatedAt.After(newest) {
			newest = s.CreatedAt
			stats.NewestSnapshot = s.FullName
		}
		stats.TotalUsedBytes += s.UsedBytes
		stats.TotalReferBytes += s.ReferBytes
	}

	stats.CloneCount = cloneCount
	return stats
}

// ============================================================================
// 压缩算法选择
// ============================================================================

// SetCompression 设置压缩算法
func (e *Engine) SetCompression(algo CompressionAlgorithm, level CompressionLevel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Compression == nil {
		e.config.Compression = DefaultCompressionConfig()
	}

	e.config.Compression.DefaultAlgo = algo
	e.config.Compression.DefaultLevel = level

	e.logger.Info("compression updated", "algorithm", algo, "level", level)
	return nil
}

// CompressData 压缩数据
func (e *Engine) CompressData(data []byte) ([]byte, CompressionAlgorithm, error) {
	e.mu.RLock()
	config := e.config.Compression
	e.mu.RUnlock()

	if config == nil {
		config = DefaultCompressionConfig()
	}

	algo := config.DefaultAlgo
	if config.AdaptiveMode {
		algo = e.selectBestAlgorithm(data)
	}

	compressed, err := e.compressWithAlgorithm(data, algo)
	if err != nil {
		return nil, algo, err
	}

	// 检查压缩是否有效
	if float64(len(compressed)) >= float64(len(data))*config.MinRatio {
		// 压缩效果不佳，返回原始数据
		return data, CompressionOff, nil
	}

	return compressed, algo, nil
}

// DecompressData 解压数据
func (e *Engine) DecompressData(data []byte, algo CompressionAlgorithm) ([]byte, error) {
	switch algo {
	case CompressionLZ4, CompressionZSTD:
		// 简化实现：实际应使用对应算法解压
		return data, nil
	case CompressionZLE:
		return data, nil
	case CompressionOff:
		return data, nil
	default:
		return nil, ErrDecompressFailed
	}
}

// GetCompressionStats 获取压缩统计
func (e *Engine) GetCompressionStats() *CompressionStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config.Compression == nil {
		return &CompressionStats{}
	}

	return &CompressionStats{
		Algorithm: e.config.Compression.DefaultAlgo,
		Level:     e.config.Compression.DefaultLevel,
	}
}

// selectBestAlgorithm 自适应选择最佳压缩算法
func (e *Engine) selectBestAlgorithm(data []byte) CompressionAlgorithm {
	size := len(data)

	// 小块数据使用 LZ4（速度快）
	if size < 4096 {
		return CompressionLZ4
	}

	// 中等块数据使用 ZSTD（平衡）
	if size < 64*1024 {
		return CompressionZSTD
	}

	// 大块数据使用 ZSTD 高压缩
	return CompressionZSTD
}

// compressWithAlgorithm 使用指定算法压缩
func (e *Engine) compressWithAlgorithm(data []byte, algo CompressionAlgorithm) ([]byte, error) {
	switch algo {
	case CompressionLZ4:
		// 简化实现：实际应使用 lz4 库
		return data, nil
	case CompressionZSTD:
		// 简化实现：实际应使用 zstd 库
		return data, nil
	case CompressionZLE:
		// 简化实现：ZLE 压缩
		return data, nil
	default:
		return nil, ErrCompressionFailed
	}
}

// ============================================================================
// 统计与监控
// ============================================================================

// GetEngineStats 获取引擎统计
func (e *Engine) GetEngineStats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &EngineStats{
		Pools: len(e.pools),
	}

	var totalCapacity, totalUsed, totalFree int64
	var lastScrub *time.Time

	for _, pool := range e.pools {
		totalCapacity += pool.TotalBytes
		totalUsed += pool.UsedBytes
		totalFree += pool.FreeBytes
		stats.VDevs += len(pool.VDevs)

		if pool.ScrubStatus.EndTime != nil {
			if lastScrub == nil || pool.ScrubStatus.EndTime.After(*lastScrub) {
				lastScrub = pool.ScrubStatus.EndTime
			}
		}
	}

	stats.TotalCapacity = totalCapacity
	stats.TotalUsed = totalUsed
	stats.TotalFree = totalFree
	stats.LastScrubTime = lastScrub
	stats.DedupStats = *e.GetDedupStats()
	stats.SnapshotStats = *e.GetSnapshotStats()
	stats.Compression = *e.GetCompressionStats()

	// 计算下次 Scrub 时间
	if e.config.Scrub != nil && e.config.Scrub.Enabled && lastScrub != nil {
		nextScrub := lastScrub.AddDate(0, 0, e.config.Scrub.IntervalDays)
		stats.NextScrubTime = &nextScrub
	}

	return stats
}

// ============================================================================
// 内部辅助函数
// ============================================================================

// findVDev 查找 VDev
func (e *Engine) findVDev(vdevID string) (*Pool, *VDev, error) {
	for _, pool := range e.pools {
		for i := range pool.VDevs {
			if pool.VDevs[i].ID == vdevID {
				return pool, &pool.VDevs[i], nil
			}
		}
	}
	return nil, nil, ErrVDevNotFound
}

// isDiskInUse 检查磁盘是否已被使用
func (e *Engine) isDiskInUse(diskID string) bool {
	for _, pool := range e.pools {
		for _, vdev := range pool.VDevs {
			for _, disk := range vdev.Disks {
				if disk.ID == diskID {
					return true
				}
			}
		}
	}
	return false
}

// validateDiskCount 验证磁盘数量
func (e *Engine) validateDiskCount(disks []Disk, config *PoolConfig) error {
	// 默认 RAID-Z2 需要至少 4 块磁盘
	minDisks := 4
	if len(disks) < minDisks {
		return fmt.Errorf("%w: need at least %d disks", ErrInsufficientDisks, minDisks)
	}
	return nil
}

// calculateTotalBytes 计算总容量
func calculateTotalBytes(disks []Disk) int64 {
	var total int64
	for _, disk := range disks {
		total += disk.SizeBytes
	}
	return total
}

// generateID 生成唯一 ID
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ============================================================================
// Scrub 调度器
// ============================================================================

// scrubScheduler Scrub 调度循环
func (e *Engine) scrubScheduler() {
	defer e.wg.Done()

	e.mu.RLock()
	interval := e.config.Scrub.IntervalDays
	e.mu.RUnlock()

	if interval <= 0 {
		interval = 14
	}

	ticker := time.NewTicker(time.Duration(interval) * 24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.RLock()
			pools := make([]*Pool, 0, len(e.pools))
			for _, p := range e.pools {
				pools = append(pools, p)
			}
			e.mu.RUnlock()

			for _, pool := range pools {
				if pool.ScrubStatus.State != ScrubStateRunning {
					if err := e.StartScrub(pool.ID); err != nil {
						e.logger.Error("auto scrub failed", "pool", pool.ID, "error", err)
					}
				}
			}
		}
	}
}
