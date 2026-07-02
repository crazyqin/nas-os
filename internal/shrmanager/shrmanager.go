package shrmanager

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// SHR 冗余级别.
const (
	SHR1 = "SHR-1" // 容忍 1 块盘故障，类似 RAID5
	SHR2 = "SHR-2" // 容忍 2 块盘故障，类似 RAID6
)

// 磁盘状态.
const (
	DiskStatusHealthy  = "healthy"  // 健康
	DiskStatusDegraded = "degraded" // 降级（预警）
	DiskStatusFailed   = "failed"   // 故障
	DiskStatusSpare    = "spare"    // 热备
)

// 存储池状态.
const (
	PoolStatusNormal    = "normal"    // 正常
	PoolStatusDegraded  = "degraded"  // 降级（有故障盘）
	PoolStatusMigrating = "migrating" // 迁移中
	PoolStatusExpanding = "expanding" // 扩容中
	PoolStatusCreating  = "creating"  // 创建中
	PoolStatusError     = "error"     // 错误
)

// 迁移状态.
const (
	MigrationStatusPending    = "pending"     // 等待中
	MigrationStatusInProgress = "in_progress" // 进行中
	MigrationStatusCompleted  = "completed"   // 完成
	MigrationStatusFailed     = "failed"      // 失败
)

// validRedundancyLevels 合法的冗余级别.
var validRedundancyLevels = map[string]bool{
	SHR1: true,
	SHR2: true,
}

// SHRDisk 表示一块物理硬盘.
type SHRDisk struct {
	Device   string    `json:"device"`    // 设备路径，如 /dev/sda
	Model    string    `json:"model"`     // 硬盘型号
	Serial   string    `json:"serial"`    // 序列号
	Capacity int64     `json:"capacity"`  // 容量（字节）
	Status   string    `json:"status"`    // 状态
	TempC    int       `json:"temp_c"`    // 温度（摄氏度）
	Health   int       `json:"health"`    // 健康度 0-100
	PoolName string    `json:"pool_name"` // 所属存储池（空表示未分配）
	IsSpare  bool      `json:"is_spare"`  // 是否为热备盘
	AddedAt  time.Time `json:"added_at"`  // 加入时间
}

// SHRArrange 表示 SHR 内部的一个子阵列
// SHR 会将不同容量的硬盘分组，每组形成独立的 RAID 子阵列.
type SHRArrange struct {
	Level     string   `json:"level"`      // 子阵列 RAID 级别 (RAID1/RAID5/RAID6)
	Devices   []string `json:"devices"`    // 参与的设备列表
	ChunkSize int64    `json:"chunk_size"` // 条带大小（字节）
	TotalSize int64    `json:"total_size"` // 子阵列总容量（字节）
}

// SHRPool 表示一个 SHR 存储池.
type SHRPool struct {
	Name         string       `json:"name"`          // 池名称
	Redundancy   string       `json:"redundancy"`    // 冗余级别 SHR-1 / SHR-2
	Devices      []string     `json:"devices"`       // 参与的设备列表
	SpareDevices []string     `json:"spare_devices"` // 热备设备列表
	Arrangements []SHRArrange `json:"arrangements"`  // 内部子阵列
	TotalSize    int64        `json:"total_size"`    // 总容量（字节）
	UsedSize     int64        `json:"used_size"`     // 已用容量（字节）
	FreeSize     int64        `json:"free_size"`     // 可用容量（字节）
	Status       string       `json:"status"`        // 池状态
	MaxFaultTol  int          `json:"max_fault_tol"` // 最大容忍故障盘数
	CreatedAt    time.Time    `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time    `json:"updated_at"`    // 最后更新时间
}

// MigrationTask 表示一个数据迁移任务.
type MigrationTask struct {
	ID          string    `json:"id"`           // 任务 ID
	PoolName    string    `json:"pool_name"`    // 目标存储池
	TargetLevel string    `json:"target_level"` // 目标冗余级别
	Status      string    `json:"status"`       // 迁移状态
	Progress    int       `json:"progress"`     // 进度百分比 0-100
	StartedAt   time.Time `json:"started_at"`   // 开始时间
	CompletedAt time.Time `json:"completed_at"` // 完成时间
	Error       string    `json:"error"`        // 错误信息
}

// SHRConfig SHR 管理器配置.
type SHRConfig struct {
	AutoOptimize      bool   `json:"auto_optimize"`       // 是否自动优化阵列布局
	DefaultRedundancy string `json:"default_redundancy"`  // 默认冗余级别
	MinDisksForSHR1   int    `json:"min_disks_shr1"`      // SHR-1 最少磁盘数
	MinDisksForSHR2   int    `json:"min_disks_shr2"`      // SHR-2 最少磁盘数
	AutoReplaceFailed bool   `json:"auto_replace_failed"` // 自动替换故障盘
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *SHRConfig {
	return &SHRConfig{
		AutoOptimize:      true,
		DefaultRedundancy: SHR1,
		MinDisksForSHR1:   2,
		MinDisksForSHR2:   4,
		AutoReplaceFailed: true,
	}
}

// SHRManager 管理 SHR 存储池.
type SHRManager struct {
	mu         sync.RWMutex
	config     *SHRConfig
	pools      map[string]*SHRPool
	disks      map[string]*SHRDisk
	migrations map[string]*MigrationTask
	nextMigID  int
}

// NewSHRManager 创建新的 SHR 管理器.
func NewSHRManager() *SHRManager {
	return &SHRManager{
		config:     DefaultConfig(),
		pools:      make(map[string]*SHRPool),
		disks:      make(map[string]*SHRDisk),
		migrations: make(map[string]*MigrationTask),
		nextMigID:  1,
	}
}

// NewSHRManagerWithConfig 使用自定义配置创建 SHR 管理器.
func NewSHRManagerWithConfig(cfg *SHRConfig) *SHRManager {
	m := NewSHRManager()
	if cfg != nil {
		m.config = cfg
	}
	return m
}

// RegisterDisk 注册一块硬盘.
func (m *SHRManager) RegisterDisk(device, model, serial string, capacity int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device == "" {
		return fmt.Errorf("设备路径不能为空")
	}
	if capacity <= 0 {
		return fmt.Errorf("硬盘容量必须大于 0")
	}
	if _, exists := m.disks[device]; exists {
		return fmt.Errorf("硬盘已注册: %s", device)
	}

	m.disks[device] = &SHRDisk{
		Device:   device,
		Model:    model,
		Serial:   serial,
		Capacity: capacity,
		Status:   DiskStatusHealthy,
		Health:   100,
		AddedAt:  time.Now(),
	}
	log.Printf("硬盘已注册: %s (容量: %d GB)", device, capacity/(1024*1024*1024))
	return nil
}

// UnregisterDisk 注销一块硬盘.
func (m *SHRManager) UnregisterDisk(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[device]
	if !exists {
		return fmt.Errorf("硬盘未注册: %s", device)
	}
	if disk.PoolName != "" {
		return fmt.Errorf("硬盘 %s 正在被存储池 %s 使用，无法注销", device, disk.PoolName)
	}

	delete(m.disks, device)
	log.Printf("硬盘已注销: %s", device)
	return nil
}

// GetDisk 获取硬盘信息.
func (m *SHRManager) GetDisk(device string) (*SHRDisk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[device]
	if !exists {
		return nil, fmt.Errorf("硬盘未注册: %s", device)
	}
	return disk, nil
}

// ListDisks 列出所有硬盘.
func (m *SHRManager) ListDisks() []*SHRDisk {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SHRDisk, 0, len(m.disks))
	for _, d := range m.disks {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Device < result[j].Device
	})
	return result
}

// ListAvailableDisks 列出可用（未分配）的硬盘.
func (m *SHRManager) ListAvailableDisks() []*SHRDisk {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SHRDisk, 0)
	for _, d := range m.disks {
		if d.PoolName == "" && d.Status != DiskStatusFailed {
			result = append(result, d)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Device < result[j].Device
	})
	return result
}

// CreatePool 创建 SHR 存储池.
func (m *SHRManager) CreatePool(name, redundancy string, devices []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return fmt.Errorf("存储池名称不能为空")
	}
	if _, exists := m.pools[name]; exists {
		return fmt.Errorf("存储池已存在: %s", name)
	}
	if !validRedundancyLevels[redundancy] {
		return fmt.Errorf("无效的冗余级别: %s (支持 SHR-1, SHR-2)", redundancy)
	}
	if len(devices) == 0 {
		return fmt.Errorf("至少需要一块硬盘")
	}

	minDisks := m.config.MinDisksForSHR1
	if redundancy == SHR2 {
		minDisks = m.config.MinDisksForSHR2
	}
	if len(devices) < minDisks {
		return fmt.Errorf("SHR-%s 至少需要 %d 块硬盘，当前: %d", redundancy[4:], minDisks, len(devices))
	}

	disks := make([]*SHRDisk, 0, len(devices))
	for _, dev := range devices {
		disk, exists := m.disks[dev]
		if !exists {
			return fmt.Errorf("硬盘未注册: %s", dev)
		}
		if disk.PoolName != "" {
			return fmt.Errorf("硬盘 %s 已分配给存储池 %s", dev, disk.PoolName)
		}
		if disk.Status == DiskStatusFailed {
			return fmt.Errorf("硬盘 %s 已故障，无法使用", dev)
		}
		disks = append(disks, disk)
	}

	arrangements, totalSize, err := m.calculateArrangements(disks, redundancy)
	if err != nil {
		return fmt.Errorf("计算阵列布局失败: %v", err)
	}

	now := time.Now()
	pool := &SHRPool{
		Name:         name,
		Redundancy:   redundancy,
		Devices:      devices,
		SpareDevices: []string{},
		Arrangements: arrangements,
		TotalSize:    totalSize,
		UsedSize:     0,
		FreeSize:     totalSize,
		Status:       PoolStatusNormal,
		MaxFaultTol:  m.calculateMaxFaultTolerance(redundancy, len(devices)),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	for _, disk := range disks {
		disk.PoolName = name
	}

	m.pools[name] = pool
	log.Printf("存储池已创建: %s (冗余: %s, 磁盘数: %d, 容量: %d GB)",
		name, redundancy, len(devices), totalSize/(1024*1024*1024))
	return nil
}

// DeletePool 删除存储池.
func (m *SHRManager) DeletePool(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[name]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", name)
	}
	if pool.Status == PoolStatusMigrating || pool.Status == PoolStatusExpanding {
		return fmt.Errorf("存储池 %s 正在操作中，无法删除", name)
	}

	for _, dev := range pool.Devices {
		if disk, ok := m.disks[dev]; ok {
			disk.PoolName = ""
		}
	}
	for _, dev := range pool.SpareDevices {
		if disk, ok := m.disks[dev]; ok {
			disk.PoolName = ""
			disk.IsSpare = false
		}
	}

	delete(m.pools, name)
	log.Printf("存储池已删除: %s", name)
	return nil
}

// GetPool 获取存储池信息.
func (m *SHRManager) GetPool(name string) (*SHRPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[name]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", name)
	}
	return pool, nil
}

// ListPools 列出所有存储池.
func (m *SHRManager) ListPools() []*SHRPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SHRPool, 0, len(m.pools))
	for _, p := range m.pools {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// AddDisk 在线扩容：向存储池添加新硬盘.
func (m *SHRManager) AddDisk(poolName string, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolName)
	}
	if pool.Status == PoolStatusMigrating {
		return fmt.Errorf("存储池正在迁移中，无法添加硬盘")
	}
	if pool.Status == PoolStatusExpanding {
		return fmt.Errorf("存储池正在扩容中，请等待完成")
	}

	disk, exists := m.disks[device]
	if !exists {
		return fmt.Errorf("硬盘未注册: %s", device)
	}
	if disk.PoolName != "" {
		return fmt.Errorf("硬盘 %s 已分配给其他存储池", device)
	}
	if disk.Status == DiskStatusFailed {
		return fmt.Errorf("硬盘 %s 已故障", device)
	}

	for _, dev := range pool.Devices {
		if dev == device {
			return fmt.Errorf("硬盘已在存储池中: %s", device)
		}
	}

	pool.Devices = append(pool.Devices, device)
	disk.PoolName = poolName

	allDisks := make([]*SHRDisk, 0, len(pool.Devices))
	for _, dev := range pool.Devices {
		allDisks = append(allDisks, m.disks[dev])
	}

	oldTotal := pool.TotalSize
	arrangements, newTotal, err := m.calculateArrangements(allDisks, pool.Redundancy)
	if err != nil {
		pool.Devices = pool.Devices[:len(pool.Devices)-1]
		disk.PoolName = ""
		return fmt.Errorf("重新计算阵列布局失败: %v", err)
	}

	pool.Arrangements = arrangements
	pool.TotalSize = newTotal
	pool.FreeSize += newTotal - oldTotal
	pool.Status = PoolStatusExpanding
	pool.MaxFaultTol = m.calculateMaxFaultTolerance(pool.Redundancy, len(pool.Devices))
	pool.UpdatedAt = time.Now()

	log.Printf("存储池 %s 正在扩容，新增硬盘: %s (新增容量: %d GB)",
		poolName, device, (newTotal-oldTotal)/(1024*1024*1024))
	return nil
}

// AddSpareDisk 添加热备盘.
func (m *SHRManager) AddSpareDisk(poolName string, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolName)
	}

	disk, exists := m.disks[device]
	if !exists {
		return fmt.Errorf("硬盘未注册: %s", device)
	}
	if disk.PoolName != "" {
		return fmt.Errorf("硬盘 %s 已分配给其他存储池", device)
	}

	for _, dev := range pool.SpareDevices {
		if dev == device {
			return fmt.Errorf("硬盘已是热备盘: %s", device)
		}
	}

	pool.SpareDevices = append(pool.SpareDevices, device)
	disk.PoolName = poolName
	disk.IsSpare = true
	disk.Status = DiskStatusSpare
	pool.UpdatedAt = time.Now()

	log.Printf("已向存储池 %s 添加热备盘: %s", poolName, device)
	return nil
}

// RemoveSpareDisk 移除热备盘.
func (m *SHRManager) RemoveSpareDisk(poolName string, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolName)
	}

	for i, dev := range pool.SpareDevices {
		if dev == device {
			pool.SpareDevices = append(pool.SpareDevices[:i], pool.SpareDevices[i+1:]...)
			if disk, ok := m.disks[device]; ok {
				disk.PoolName = ""
				disk.IsSpare = false
				disk.Status = DiskStatusHealthy
			}
			pool.UpdatedAt = time.Now()
			log.Printf("已从存储池 %s 移除热备盘: %s", poolName, device)
			return nil
		}
	}
	return fmt.Errorf("热备盘不存在: %s", device)
}

// MigrateRedundancy 在线迁移冗余级别（如 SHR-1 → SHR-2）.
func (m *SHRManager) MigrateRedundancy(poolName string, targetRedundancy string) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolName)
	}
	if !validRedundancyLevels[targetRedundancy] {
		return nil, fmt.Errorf("无效的目标冗余级别: %s", targetRedundancy)
	}
	if pool.Redundancy == targetRedundancy {
		return nil, fmt.Errorf("存储池已经是 %s 级别", targetRedundancy)
	}
	if pool.Status == PoolStatusMigrating {
		return nil, fmt.Errorf("存储池正在迁移中")
	}
	if pool.Status == PoolStatusExpanding {
		return nil, fmt.Errorf("存储池正在扩容中")
	}

	if targetRedundancy == SHR2 && len(pool.Devices) < m.config.MinDisksForSHR2 {
		return nil, fmt.Errorf("SHR-2 至少需要 %d 块硬盘，当前: %d", m.config.MinDisksForSHR2, len(pool.Devices))
	}

	migID := fmt.Sprintf("mig_%d", m.nextMigID)
	m.nextMigID++

	task := &MigrationTask{
		ID:          migID,
		PoolName:    poolName,
		TargetLevel: targetRedundancy,
		Status:      MigrationStatusInProgress,
		Progress:    0,
		StartedAt:   time.Now(),
	}

	pool.Status = PoolStatusMigrating
	pool.UpdatedAt = time.Now()
	m.migrations[migID] = task

	log.Printf("存储池 %s 开始冗余级别迁移: %s → %s (任务: %s)",
		poolName, pool.Redundancy, targetRedundancy, migID)

	go m.doMigration(migID, poolName, targetRedundancy)
	return task, nil
}

// doMigration 异步执行迁移.
func (m *SHRManager) doMigration(migID, poolName, targetRedundancy string) {
	task := m.migrations[migID]
	if task == nil {
		return
	}

	for i := 10; i <= 100; i += 10 {
		time.Sleep(100 * time.Millisecond)
		m.mu.Lock()
		task.Progress = i
		m.mu.Unlock()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		task.Status = MigrationStatusFailed
		task.Error = "存储池已不存在"
		return
	}

	oldRedundancy := pool.Redundancy
	pool.Redundancy = targetRedundancy
	pool.Status = PoolStatusNormal
	pool.MaxFaultTol = m.calculateMaxFaultTolerance(targetRedundancy, len(pool.Devices))

	allDisks := make([]*SHRDisk, 0, len(pool.Devices))
	for _, dev := range pool.Devices {
		if disk, ok := m.disks[dev]; ok {
			allDisks = append(allDisks, disk)
		}
	}
	arrangements, newTotal, err := m.calculateArrangements(allDisks, targetRedundancy)
	if err == nil {
		pool.Arrangements = arrangements
		pool.FreeSize += newTotal - pool.TotalSize
		pool.TotalSize = newTotal
	}
	pool.UpdatedAt = time.Now()

	task.Status = MigrationStatusCompleted
	task.Progress = 100
	task.CompletedAt = time.Now()

	log.Printf("存储池 %s 冗余级别迁移完成: %s → %s", poolName, oldRedundancy, targetRedundancy)
}

// GetMigration 获取迁移任务状态.
func (m *SHRManager) GetMigration(taskID string) (*MigrationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.migrations[taskID]
	if !exists {
		return nil, fmt.Errorf("迁移任务不存在: %s", taskID)
	}
	return task, nil
}

// ListMigrations 列出所有迁移任务.
func (m *SHRManager) ListMigrations() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrationTask, 0, len(m.migrations))
	for _, t := range m.migrations {
		result = append(result, t)
	}
	return result
}

// ReplaceFailedDisk 替换故障盘（自动用热备盘替换或标记为待替换）.
func (m *SHRManager) ReplaceFailedDisk(poolName string, failedDevice string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolName)
	}

	found := false
	for _, dev := range pool.Devices {
		if dev == failedDevice {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("硬盘不在存储池中: %s", failedDevice)
	}

	disk, ok := m.disks[failedDevice]
	if !ok {
		return fmt.Errorf("硬盘未注册: %s", failedDevice)
	}
	disk.Status = DiskStatusFailed
	pool.Status = PoolStatusDegraded

	// 尝试用热备盘自动替换
	if m.config.AutoReplaceFailed && len(pool.SpareDevices) > 0 {
		spareDev := pool.SpareDevices[0]
		pool.SpareDevices = pool.SpareDevices[1:]

		for i, dev := range pool.Devices {
			if dev == failedDevice {
				pool.Devices[i] = spareDev
				break
			}
		}

		spareDisk := m.disks[spareDev]
		spareDisk.IsSpare = false
		spareDisk.Status = DiskStatusHealthy
		disk.PoolName = ""
		pool.Status = PoolStatusNormal

		log.Printf("存储池 %s: 热备盘 %s 已自动替换故障盘 %s", poolName, spareDev, failedDevice)
	} else {
		log.Printf("存储池 %s: 硬盘 %s 标记为故障，等待手动替换", poolName, failedDevice)
	}

	pool.UpdatedAt = time.Now()
	return nil
}

// MarkDiskDegraded 标记硬盘为降级状态.
func (m *SHRManager) MarkDiskDegraded(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[device]
	if !exists {
		return fmt.Errorf("硬盘未注册: %s", device)
	}
	disk.Status = DiskStatusDegraded

	if disk.PoolName != "" {
		if pool, ok := m.pools[disk.PoolName]; ok {
			pool.Status = PoolStatusDegraded
			pool.UpdatedAt = time.Now()
		}
	}

	log.Printf("硬盘 %s 已标记为降级", device)
	return nil
}

// CalculateRedundancy 计算指定配置的冗余度.
func (m *SHRManager) CalculateRedundancy(devices []string, redundancy string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !validRedundancyLevels[redundancy] {
		return 0, fmt.Errorf("无效的冗余级别: %s", redundancy)
	}

	for _, dev := range devices {
		if _, exists := m.disks[dev]; !exists {
			return 0, fmt.Errorf("硬盘未注册: %s", dev)
		}
	}

	return m.calculateMaxFaultTolerance(redundancy, len(devices)), nil
}

// GetPoolStatus 获取存储池详细状态.
func (m *SHRManager) GetPoolStatus(poolName string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolName)
	}

	diskStatuses := make([]map[string]interface{}, 0, len(pool.Devices))
	for _, dev := range pool.Devices {
		if disk, ok := m.disks[dev]; ok {
			diskStatuses = append(diskStatuses, map[string]interface{}{
				"device":   disk.Device,
				"status":   disk.Status,
				"health":   disk.Health,
				"capacity": disk.Capacity,
			})
		}
	}

	return map[string]interface{}{
		"name":          pool.Name,
		"redundancy":    pool.Redundancy,
		"status":        pool.Status,
		"devices":       pool.Devices,
		"spare_devices": pool.SpareDevices,
		"total_size":    pool.TotalSize,
		"used_size":     pool.UsedSize,
		"free_size":     pool.FreeSize,
		"max_fault_tol": pool.MaxFaultTol,
		"arrangements":  pool.Arrangements,
		"disk_statuses": diskStatuses,
		"disk_count":    len(pool.Devices),
		"spare_count":   len(pool.SpareDevices),
		"created_at":    pool.CreatedAt,
		"updated_at":    pool.UpdatedAt,
	}, nil
}

// OptimizeLayout 自动优化存储池布局.
func (m *SHRManager) OptimizeLayout(poolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolName)
	}
	if pool.Status == PoolStatusMigrating || pool.Status == PoolStatusExpanding {
		return fmt.Errorf("存储池正在操作中，无法优化")
	}

	allDisks := make([]*SHRDisk, 0, len(pool.Devices))
	for _, dev := range pool.Devices {
		if disk, ok := m.disks[dev]; ok {
			allDisks = append(allDisks, disk)
		}
	}

	oldTotal := pool.TotalSize
	arrangements, newTotal, err := m.calculateArrangements(allDisks, pool.Redundancy)
	if err != nil {
		return fmt.Errorf("优化布局失败: %v", err)
	}

	pool.Arrangements = arrangements
	pool.TotalSize = newTotal
	pool.FreeSize += newTotal - oldTotal
	pool.UpdatedAt = time.Now()

	log.Printf("存储池 %s 布局已优化 (容量变化: %d GB → %d GB)",
		poolName, oldTotal/(1024*1024*1024), newTotal/(1024*1024*1024))
	return nil
}

// GetConfig 获取当前配置.
func (m *SHRManager) GetConfig() *SHRConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *SHRManager) UpdateConfig(cfg *SHRConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}
	m.config = cfg
	log.Printf("SHR 配置已更新")
	return nil
}

// --- 内部辅助函数 ---

// calculateArrangements 计算最优阵列布局
// SHR 核心算法：将不同容量硬盘分组，每组形成独立 RAID 子阵列.
func (m *SHRManager) calculateArrangements(disks []*SHRDisk, redundancy string) ([]SHRArrange, int64, error) {
	if len(disks) == 0 {
		return nil, 0, fmt.Errorf("没有可用硬盘")
	}

	// 按容量排序（升序）
	sortedDisks := make([]*SHRDisk, len(disks))
	copy(sortedDisks, disks)
	sort.Slice(sortedDisks, func(i, j int) bool {
		return sortedDisks[i].Capacity < sortedDisks[j].Capacity
	})

	if redundancy == SHR1 {
		return m.arrangeSHR1(sortedDisks)
	}
	return m.arrangeSHR2(sortedDisks)
}

// arrangeSHR1 SHR-1 布局算法
// - 2 块盘：RAID1 镜像
// - 3 块及以上：RAID5，剩余空间两两配对 RAID1.
func (m *SHRManager) arrangeSHR1(disks []*SHRDisk) ([]SHRArrange, int64, error) {
	arrangements := make([]SHRArrange, 0)
	var totalSize int64
	n := len(disks)

	if n == 2 {
		minCap := min64(disks[0].Capacity, disks[1].Capacity)
		arrangements = append(arrangements, SHRArrange{
			Level:     "RAID1",
			Devices:   []string{disks[0].Device, disks[1].Device},
			ChunkSize: 64 * 1024,
			TotalSize: minCap,
		})
		return arrangements, minCap, nil
	}

	// 3块及以上：所有盘用最小容量组成 RAID5
	minCap := disks[0].Capacity
	devs := make([]string, n)
	for i, d := range disks {
		devs[i] = d.Device
	}
	raid5Size := minCap * int64(n-1)
	arrangements = append(arrangements, SHRArrange{
		Level:     "RAID5",
		Devices:   devs,
		ChunkSize: 64 * 1024,
		TotalSize: raid5Size,
	})
	totalSize = raid5Size

	// 大于最小容量的盘，剩余空间两两配对做 RAID1
	bigDisks := make([]*SHRDisk, 0)
	for _, d := range disks {
		if d.Capacity > minCap {
			bigDisks = append(bigDisks, d)
		}
	}
	for i := 0; i+1 < len(bigDisks); i += 2 {
		extra := min64(bigDisks[i].Capacity-minCap, bigDisks[i+1].Capacity-minCap)
		if extra > 0 {
			arrangements = append(arrangements, SHRArrange{
				Level:     "RAID1",
				Devices:   []string{bigDisks[i].Device, bigDisks[i+1].Device},
				ChunkSize: 64 * 1024,
				TotalSize: extra,
			})
			totalSize += extra
		}
	}

	return arrangements, totalSize, nil
}

// arrangeSHR2 SHR-2 布局算法
// - 所有盘用最小容量组成 RAID6，剩余空间两两 RAID1.
func (m *SHRManager) arrangeSHR2(disks []*SHRDisk) ([]SHRArrange, int64, error) {
	arrangements := make([]SHRArrange, 0)
	var totalSize int64
	n := len(disks)

	minCap := disks[0].Capacity
	devs := make([]string, n)
	for i, d := range disks {
		devs[i] = d.Device
	}

	// 所有盘用最小容量组成 RAID6
	raid6Size := minCap * int64(n-2)
	arrangements = append(arrangements, SHRArrange{
		Level:     "RAID6",
		Devices:   devs,
		ChunkSize: 64 * 1024,
		TotalSize: raid6Size,
	})
	totalSize = raid6Size

	// 大于最小容量的盘，剩余空间两两配对做 RAID1
	bigDisks := make([]*SHRDisk, 0)
	for _, d := range disks {
		if d.Capacity > minCap {
			bigDisks = append(bigDisks, d)
		}
	}
	for i := 0; i+1 < len(bigDisks); i += 2 {
		extra := min64(bigDisks[i].Capacity-minCap, bigDisks[i+1].Capacity-minCap)
		if extra > 0 {
			arrangements = append(arrangements, SHRArrange{
				Level:     "RAID1",
				Devices:   []string{bigDisks[i].Device, bigDisks[i+1].Device},
				ChunkSize: 64 * 1024,
				TotalSize: extra,
			})
			totalSize += extra
		}
	}

	return arrangements, totalSize, nil
}

// calculateMaxFaultTolerance 计算最大容忍故障盘数.
func (m *SHRManager) calculateMaxFaultTolerance(redundancy string, diskCount int) int {
	switch redundancy {
	case SHR1:
		if diskCount < 2 {
			return 0
		}
		return 1
	case SHR2:
		if diskCount < 4 {
			return 0
		}
		return 2
	default:
		return 0
	}
}

// min64 返回两个 int64 中较小的值.
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
