// Package storagepool 提供存储池管理功能
// 存储池是一个逻辑存储单元，可以包含多个磁盘设备，支持 RAID 级别配置
package storagepool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PoolStatus 存储池状态.
type PoolStatus string

const (
	// PoolStatusCreating 表示存储池正在创建中.
	PoolStatusCreating PoolStatus = "creating"
	// PoolStatusHealthy 表示存储池状态健康.
	PoolStatusHealthy PoolStatus = "healthy"
	// PoolStatusDegraded 表示存储池降级（部分磁盘故障）.
	PoolStatusDegraded PoolStatus = "degraded"
	// PoolStatusRebuilding 表示存储池正在重建中.
	PoolStatusRebuilding PoolStatus = "rebuilding"
	// PoolStatusFaulted 表示存储池故障（不可用）.
	PoolStatusFaulted PoolStatus = "faulted"
	// PoolStatusOffline 表示存储池离线.
	PoolStatusOffline PoolStatus = "offline"
)

// RAIDLevel RAID 级别.
type RAIDLevel string

const (
	// RAIDLevelSingle 表示单盘模式.
	RAIDLevelSingle RAIDLevel = "single"
	// RAIDLevelRAID0 表示 RAID0 条带模式.
	RAIDLevelRAID0 RAIDLevel = "raid0"
	// RAIDLevelRAID1 表示 RAID1 镜像模式.
	RAIDLevelRAID1 RAIDLevel = "raid1"
	// RAIDLevelRAID5 表示 RAID5 分布式奇偶校验模式.
	RAIDLevelRAID5 RAIDLevel = "raid5"
	// RAIDLevelRAID6 表示 RAID6 双奇偶校验模式.
	RAIDLevelRAID6 RAIDLevel = "raid6"
	// RAIDLevelRAID10 表示 RAID10 条带镜像模式.
	RAIDLevelRAID10 RAIDLevel = "raid10"
)

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	// DeviceStatusOnline 表示设备在线状态.
	DeviceStatusOnline DeviceStatus = "online"
	// DeviceStatusOffline 表示设备离线状态.
	DeviceStatusOffline DeviceStatus = "offline"
	// DeviceStatusFaulted 表示设备故障状态.
	DeviceStatusFaulted DeviceStatus = "faulted"
	// DeviceStatusSpare 表示设备为热备状态.
	DeviceStatusSpare DeviceStatus = "spare"
	// DeviceStatusRemoved 表示设备已移除状态.
	DeviceStatusRemoved DeviceStatus = "removed"
)

// Device 磁盘设备信息.
type Device struct {
	ID           string       `json:"id"`           // 设备唯一标识（如 /dev/sda）
	Path         string       `json:"path"`         // 设备路径
	Name         string       `json:"name"`         // 设备名称
	Model        string       `json:"model"`        // 型号
	Serial       string       `json:"serial"`       // 序列号
	Size         uint64       `json:"size"`         // 容量（字节）
	Used         uint64       `json:"used"`         // 已使用（字节）
	Health       string       `json:"health"`       // 健康状态 (SMART)
	Temperature  int          `json:"temperature"`  // 温度（摄氏度）
	Status       DeviceStatus `json:"status"`       // 设备状态
	IsSystemDisk bool         `json:"isSystemDisk"` // 是否系统盘
	AddedAt      time.Time    `json:"addedAt"`      // 添加时间
}

// Pool 存储池信息.
type Pool struct {
	ID              string     `json:"id"`              // 存储池唯一标识
	Name            string     `json:"name"`            // 存储池名称
	Description     string     `json:"description"`     // 描述
	RAIDLevel       RAIDLevel  `json:"raidLevel"`       // RAID 级别
	Devices         []*Device  `json:"devices"`         // 设备列表
	SpareDevices    []*Device  `json:"spareDevices"`    // 热备设备
	Size            uint64     `json:"size"`            // 总容量（字节）
	Used            uint64     `json:"used"`            // 已使用（字节）
	Free            uint64     `json:"free"`            // 可用空间（字节）
	Status          PoolStatus `json:"status"`          // 状态
	MountPoint      string     `json:"mountPoint"`      // 挂载点
	FileSystem      string     `json:"fileSystem"`      // 文件系统类型 (btrfs, zfs, ext4)
	HealthScore     int        `json:"healthScore"`     // 健康分数 0-100
	ReadSpeed       uint64     `json:"readSpeed"`       // 读取速度 (bytes/s)
	WriteSpeed      uint64     `json:"writeSpeed"`      // 写入速度 (bytes/s)
	IOps            uint64     `json:"ioPs"`            // IOPS
	RebuildProgress float64    `json:"rebuildProgress"` // 重建进度 0-100
	CreatedAt       time.Time  `json:"createdAt"`       // 创建时间
	UpdatedAt       time.Time  `json:"updatedAt"`       // 更新时间
	Tags            []string   `json:"tags"`            // 标签
}

// CreatePoolRequest 创建存储池请求.
type CreatePoolRequest struct {
	Name        string    `json:"name" validate:"required,min=1,max=64"`
	Description string    `json:"description"`
	RAIDLevel   RAIDLevel `json:"raidLevel" validate:"required,oneof=single raid0 raid1 raid5 raid6 raid10"`
	DevicePaths []string  `json:"devicePaths" validate:"required,min=1"`
	SparePaths  []string  `json:"sparePaths"`
	MountPoint  string    `json:"mountPoint" validate:"omitempty"`
	FileSystem  string    `json:"fileSystem" validate:"oneof=btrfs zfs ext4"`
	Tags        []string  `json:"tags"`
}

// AddDeviceRequest 添加设备请求.
type AddDeviceRequest struct {
	DevicePaths []string `json:"devicePaths" validate:"required,min=1"`
	IsSpare     bool     `json:"isSpare"` // 是否作为热备盘
}

// RemoveDeviceRequest 移除设备请求.
type RemoveDeviceRequest struct {
	DeviceID string `json:"deviceId" validate:"required"`
	Force    bool   `json:"force"` // 强制移除（可能丢失数据）
}

// ResizePoolRequest 扩容/缩容请求.
type ResizePoolRequest struct {
	NewRAIDLevel  RAIDLevel `json:"newRaidLevel"`  // 新的 RAID 级别（可选）
	AddDevices    []string  `json:"addDevices"`    // 添加的设备
	RemoveDevices []string  `json:"removeDevices"` // 移除的设备
}

// RAIDConfig RAID 配置信息.
type RAIDConfig struct {
	Level          RAIDLevel `json:"level"`
	MinDevices     int       `json:"minDevices"`
	MaxDevices     int       `json:"maxDevices"`     // 0 表示无限制
	RecommendedDev int       `json:"recommendedDev"` // 推荐设备数
	FaultTolerance int       `json:"faultTolerance"` // 容错磁盘数
	UsableCapacity float64   `json:"usableCapacity"` // 可用容量比例 (0-1)
	Description    string    `json:"description"`
}

// RAIDConfigs 预定义的 RAID 配置映射表.
var RAIDConfigs = map[RAIDLevel]RAIDConfig{
	RAIDLevelSingle: {
		Level:          RAIDLevelSingle,
		MinDevices:     1,
		MaxDevices:     1,
		RecommendedDev: 1,
		FaultTolerance: 0,
		UsableCapacity: 1.0,
		Description:    "单盘模式，无冗余",
	},
	RAIDLevelRAID0: {
		Level:          RAIDLevelRAID0,
		MinDevices:     2,
		MaxDevices:     0,
		RecommendedDev: 2,
		FaultTolerance: 0,
		UsableCapacity: 1.0,
		Description:    "条带模式，性能最佳，无冗余",
	},
	RAIDLevelRAID1: {
		Level:          RAIDLevelRAID1,
		MinDevices:     2,
		MaxDevices:     2,
		RecommendedDev: 2,
		FaultTolerance: 1,
		UsableCapacity: 0.5,
		Description:    "镜像模式，允许 1 盘故障",
	},
	RAIDLevelRAID5: {
		Level:          RAIDLevelRAID5,
		MinDevices:     3,
		MaxDevices:     0,
		RecommendedDev: 4,
		FaultTolerance: 1,
		UsableCapacity: 0.67,
		Description:    "分布式奇偶校验，允许 1 盘故障",
	},
	RAIDLevelRAID6: {
		Level:          RAIDLevelRAID6,
		MinDevices:     4,
		MaxDevices:     0,
		RecommendedDev: 6,
		FaultTolerance: 2,
		UsableCapacity: 0.67,
		Description:    "双奇偶校验，允许 2 盘故障",
	},
	RAIDLevelRAID10: {
		Level:          RAIDLevelRAID10,
		MinDevices:     4,
		MaxDevices:     0,
		RecommendedDev: 4,
		FaultTolerance: 1,
		UsableCapacity: 0.5,
		Description:    "条带镜像，性能与冗余平衡",
	},
}

// Manager 存储池管理器.
type Manager struct {
	pools     map[string]*Pool
	devices   map[string]*Device
	mu        sync.RWMutex
	dataPath  string // 数据存储路径
	mountBase string // 挂载基础目录
}

// NewManager 创建存储池管理器.
func NewManager(dataPath, mountBase string) (*Manager, error) {
	if dataPath == "" {
		dataPath = "/var/lib/nas-os/storage-pools"
	}
	if mountBase == "" {
		mountBase = "/mnt/pools"
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataPath, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	m := &Manager{
		pools:     make(map[string]*Pool),
		devices:   make(map[string]*Device),
		dataPath:  dataPath,
		mountBase: mountBase,
	}

	// 加载已有数据
	if err := m.load(); err != nil {
		return nil, fmt.Errorf("加载数据失败: %w", err)
	}

	// 扫描系统设备
	if err := m.scanDevices(); err != nil {
		return nil, fmt.Errorf("扫描设备失败: %w", err)
	}

	return m, nil
}

// CreatePool 创建存储池.
func (m *Manager) CreatePool(req *CreatePoolRequest) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证名称唯一性
	for _, p := range m.pools {
		if p.Name == req.Name {
			return nil, fmt.Errorf("存储池名称已存在: %s", req.Name)
		}
	}

	// 验证 RAID 配置
	config, ok := RAIDConfigs[req.RAIDLevel]
	if !ok {
		return nil, fmt.Errorf("不支持的 RAID 级别: %s", req.RAIDLevel)
	}

	// 验证设备数量
	if len(req.DevicePaths) < config.MinDevices {
		return nil, fmt.Errorf("%s 至少需要 %d 个设备，当前 %d 个",
			req.RAIDLevel, config.MinDevices, len(req.DevicePaths))
	}
	if config.MaxDevices > 0 && len(req.DevicePaths) > config.MaxDevices {
		return nil, fmt.Errorf("%s 最多支持 %d 个设备，当前 %d 个",
			req.RAIDLevel, config.MaxDevices, len(req.DevicePaths))
	}

	// 验证设备可用性
	devices := make([]*Device, 0, len(req.DevicePaths))
	for _, path := range req.DevicePaths {
		device, err := m.getAvailableDevice(path)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	// 验证热备设备
	spareDevices := make([]*Device, 0)
	for _, path := range req.SparePaths {
		device, err := m.getAvailableDevice(path)
		if err != nil {
			return nil, err
		}
		device.Status = DeviceStatusSpare
		spareDevices = append(spareDevices, device)
	}

	// 计算总容量
	var totalSize uint64
	for _, d := range devices {
		totalSize += d.Size
	}

	// 计算可用容量
	usableSize := uint64(float64(totalSize) * config.UsableCapacity)

	// 设置挂载点
	mountPoint := req.MountPoint
	if mountPoint == "" {
		mountPoint = filepath.Join(m.mountBase, req.Name)
	}

	// 默认文件系统
	fs := req.FileSystem
	if fs == "" {
		fs = "btrfs"
	}

	// 创建存储池对象
	pool := &Pool{
		ID:           generatePoolID(),
		Name:         req.Name,
		Description:  req.Description,
		RAIDLevel:    req.RAIDLevel,
		Devices:      devices,
		SpareDevices: spareDevices,
		Size:         usableSize,
		Used:         0,
		Free:         usableSize,
		Status:       PoolStatusCreating,
		MountPoint:   mountPoint,
		FileSystem:   fs,
		HealthScore:  100,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Tags:         req.Tags,
	}

	// 更新设备状态
	for _, d := range devices {
		d.Status = DeviceStatusOnline
		d.AddedAt = time.Now()
	}

	// 添加到管理器
	m.pools[pool.ID] = pool

	// 保存数据
	if err := m.save(); err != nil {
		delete(m.pools, pool.ID)
		return nil, fmt.Errorf("保存数据失败: %w", err)
	}

	return pool, nil
}

// GetPool 获取存储池.
func (m *Manager) GetPool(id string) (*Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("存储池不存在: %s", id)
	}

	// 返回副本
	return m.clonePool(pool), nil
}

// GetPoolByName 通过名称获取存储池.
func (m *Manager) GetPoolByName(name string) (*Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pool := range m.pools {
		if pool.Name == name {
			return m.clonePool(pool), nil
		}
	}

	return nil, fmt.Errorf("存储池不存在: %s", name)
}

// ListPools 列出所有存储池.
func (m *Manager) ListPools() []*Pool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*Pool, 0, len(m.pools))
	for _, pool := range m.pools {
		pools = append(pools, m.clonePool(pool))
	}

	return pools
}

// DeletePool 删除存储池.
func (m *Manager) DeletePool(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[id]
	if !ok {
		return fmt.Errorf("存储池不存在: %s", id)
	}

	// 检查是否有数据
	if pool.Used > 0 && !force {
		return fmt.Errorf("存储池 %s 包含数据，请先清空或使用强制删除", pool.Name)
	}

	// 释放设备
	for _, d := range pool.Devices {
		d.Status = DeviceStatusOnline
	}
	for _, d := range pool.SpareDevices {
		d.Status = DeviceStatusOnline
	}

	// 删除
	delete(m.pools, id)

	// 保存
	return m.save()
}

// AddDevice 添加设备到存储池.
func (m *Manager) AddDevice(poolID string, req *AddDeviceRequest) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	// 验证设备可用性
	devices := make([]*Device, 0, len(req.DevicePaths))
	for _, path := range req.DevicePaths {
		device, err := m.getAvailableDevice(path)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	// 添加设备
	for _, d := range devices {
		d.Status = DeviceStatusOnline
		if req.IsSpare {
			d.Status = DeviceStatusSpare
			pool.SpareDevices = append(pool.SpareDevices, d)
		} else {
			pool.Devices = append(pool.Devices, d)
		}
		d.AddedAt = time.Now()
	}

	// 重新计算容量
	m.recalculatePoolCapacity(pool)
	pool.UpdatedAt = time.Now()

	// 保存
	if err := m.save(); err != nil {
		return nil, err
	}

	return m.clonePool(pool), nil
}

// RemoveDevice 从存储池移除设备.
func (m *Manager) RemoveDevice(poolID string, req *RemoveDeviceRequest) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	// 查找设备
	var device *Device
	var deviceIndex = -1
	var isSpare = false

	for i, d := range pool.Devices {
		if d.ID == req.DeviceID || d.Path == req.DeviceID {
			device = d
			deviceIndex = i
			break
		}
	}

	if device == nil {
		for i, d := range pool.SpareDevices {
			if d.ID == req.DeviceID || d.Path == req.DeviceID {
				device = d
				deviceIndex = i
				isSpare = true
				break
			}
		}
	}

	if device == nil {
		return nil, fmt.Errorf("设备不在存储池中: %s", req.DeviceID)
	}

	// 验证 RAID 最小设备数
	config := RAIDConfigs[pool.RAIDLevel]
	if !isSpare && len(pool.Devices)-1 < config.MinDevices && !req.Force {
		return nil, fmt.Errorf("%s 至少需要 %d 个设备，移除后不足",
			pool.RAIDLevel, config.MinDevices)
	}

	// 移除设备
	if isSpare {
		pool.SpareDevices = append(pool.SpareDevices[:deviceIndex], pool.SpareDevices[deviceIndex+1:]...)
	} else {
		pool.Devices = append(pool.Devices[:deviceIndex], pool.Devices[deviceIndex+1:]...)
	}
	device.Status = DeviceStatusRemoved

	// 重新计算容量
	m.recalculatePoolCapacity(pool)
	pool.UpdatedAt = time.Now()

	// 检查状态
	if len(pool.Devices) < config.MinDevices {
		pool.Status = PoolStatusDegraded
	}

	// 保存
	if err := m.save(); err != nil {
		return nil, err
	}

	return m.clonePool(pool), nil
}

// ResizePool 扩容/缩容存储池.
func (m *Manager) ResizePool(poolID string, req *ResizePoolRequest) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	// 更新 RAID 级别
	if req.NewRAIDLevel != "" && req.NewRAIDLevel != pool.RAIDLevel {
		config, ok := RAIDConfigs[req.NewRAIDLevel]
		if !ok {
			return nil, fmt.Errorf("不支持的 RAID 级别: %s", req.NewRAIDLevel)
		}

		// 验证设备数量
		totalDevices := len(pool.Devices) + len(req.AddDevices) - len(req.RemoveDevices)
		if totalDevices < config.MinDevices {
			return nil, fmt.Errorf("%s 至少需要 %d 个设备", req.NewRAIDLevel, config.MinDevices)
		}
		if config.MaxDevices > 0 && totalDevices > config.MaxDevices {
			return nil, fmt.Errorf("%s 最多支持 %d 个设备", req.NewRAIDLevel, config.MaxDevices)
		}

		pool.RAIDLevel = req.NewRAIDLevel
	}

	// 添加设备
	for _, path := range req.AddDevices {
		device, err := m.getAvailableDevice(path)
		if err != nil {
			return nil, err
		}
		device.Status = DeviceStatusOnline
		device.AddedAt = time.Now()
		pool.Devices = append(pool.Devices, device)
	}

	// 移除设备
	for _, devID := range req.RemoveDevices {
		for i, d := range pool.Devices {
			if d.ID == devID || d.Path == devID {
				d.Status = DeviceStatusRemoved
				pool.Devices = append(pool.Devices[:i], pool.Devices[i+1:]...)
				break
			}
		}
	}

	// 重新计算容量
	m.recalculatePoolCapacity(pool)
	pool.UpdatedAt = time.Now()

	// 保存
	if err := m.save(); err != nil {
		return nil, err
	}

	return m.clonePool(pool), nil
}

// GetAvailableDevices 获取可用设备列表.
func (m *Manager) GetAvailableDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, d := range m.devices {
		// 只返回未被使用的设备
		if d.Status == DeviceStatusOnline && !d.IsSystemDisk {
			devices = append(devices, d)
		}
	}

	return devices
}

// GetPoolStats 获取存储池统计信息.
func (m *Manager) GetPoolStats(poolID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	stats := map[string]interface{}{
		"id":             pool.ID,
		"name":           pool.Name,
		"status":         pool.Status,
		"healthScore":    pool.HealthScore,
		"totalDevices":   len(pool.Devices),
		"spareDevices":   len(pool.SpareDevices),
		"size":           pool.Size,
		"used":           pool.Used,
		"free":           pool.Free,
		"usagePercent":   float64(pool.Used) / float64(pool.Size) * 100,
		"raidLevel":      pool.RAIDLevel,
		"faultTolerance": RAIDConfigs[pool.RAIDLevel].FaultTolerance,
		"readSpeed":      pool.ReadSpeed,
		"writeSpeed":     pool.WriteSpeed,
		"ioPs":           pool.IOps,
	}

	if pool.Status == PoolStatusRebuilding {
		stats["rebuildProgress"] = pool.RebuildProgress
	}

	return stats, nil
}

// UpdatePoolStats 更新存储池状态（由监控系统调用）.
func (m *Manager) UpdatePoolStats(poolID string, stats *PoolStatsUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return fmt.Errorf("存储池不存在: %s", poolID)
	}

	if stats.Used != nil {
		pool.Used = *stats.Used
		pool.Free = pool.Size - pool.Used
	}
	if stats.Status != "" {
		pool.Status = stats.Status
	}
	if stats.HealthScore != nil {
		pool.HealthScore = *stats.HealthScore
	}
	if stats.ReadSpeed != nil {
		pool.ReadSpeed = *stats.ReadSpeed
	}
	if stats.WriteSpeed != nil {
		pool.WriteSpeed = *stats.WriteSpeed
	}
	if stats.IOps != nil {
		pool.IOps = *stats.IOps
	}
	if stats.RebuildProgress != nil {
		pool.RebuildProgress = *stats.RebuildProgress
	}

	pool.UpdatedAt = time.Now()

	return m.save()
}

// PoolStatsUpdate 存储池状态更新.
type PoolStatsUpdate struct {
	Used            *uint64    `json:"used,omitempty"`
	Status          PoolStatus `json:"status,omitempty"`
	HealthScore     *int       `json:"healthScore,omitempty"`
	ReadSpeed       *uint64    `json:"readSpeed,omitempty"`
	WriteSpeed      *uint64    `json:"writeSpeed,omitempty"`
	IOps            *uint64    `json:"ioPs,omitempty"`
	RebuildProgress *float64   `json:"rebuildProgress,omitempty"`
}

// getAvailableDevice 获取可用设备.
func (m *Manager) getAvailableDevice(path string) (*Device, error) {
	device, ok := m.devices[path]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", path)
	}

	if device.IsSystemDisk {
		return nil, fmt.Errorf("不能使用系统盘: %s", path)
	}

	// 检查是否已被使用
	for _, pool := range m.pools {
		for _, d := range pool.Devices {
			if d.Path == path {
				return nil, fmt.Errorf("设备 %s 已被存储池 %s 使用", path, pool.Name)
			}
		}
		for _, d := range pool.SpareDevices {
			if d.Path == path {
				return nil, fmt.Errorf("设备 %s 已是存储池 %s 的热备盘", path, pool.Name)
			}
		}
	}

	return device, nil
}

// recalculatePoolCapacity 重新计算存储池容量.
func (m *Manager) recalculatePoolCapacity(pool *Pool) {
	var totalSize uint64
	for _, d := range pool.Devices {
		totalSize += d.Size
	}

	config := RAIDConfigs[pool.RAIDLevel]
	pool.Size = uint64(float64(totalSize) * config.UsableCapacity)
	pool.Free = pool.Size - pool.Used
}

// clonePool 克隆存储池（返回副本）.
func (m *Manager) clonePool(pool *Pool) *Pool {
	clone := &Pool{
		ID:              pool.ID,
		Name:            pool.Name,
		Description:     pool.Description,
		RAIDLevel:       pool.RAIDLevel,
		Size:            pool.Size,
		Used:            pool.Used,
		Free:            pool.Free,
		Status:          pool.Status,
		MountPoint:      pool.MountPoint,
		FileSystem:      pool.FileSystem,
		HealthScore:     pool.HealthScore,
		ReadSpeed:       pool.ReadSpeed,
		WriteSpeed:      pool.WriteSpeed,
		IOps:            pool.IOps,
		RebuildProgress: pool.RebuildProgress,
		CreatedAt:       pool.CreatedAt,
		UpdatedAt:       pool.UpdatedAt,
		Tags:            append([]string{}, pool.Tags...),
	}

	// 复制设备
	clone.Devices = make([]*Device, len(pool.Devices))
	for i, d := range pool.Devices {
		clone.Devices[i] = m.cloneDevice(d)
	}

	clone.SpareDevices = make([]*Device, len(pool.SpareDevices))
	for i, d := range pool.SpareDevices {
		clone.SpareDevices[i] = m.cloneDevice(d)
	}

	return clone
}

// cloneDevice 克隆设备.
func (m *Manager) cloneDevice(d *Device) *Device {
	return &Device{
		ID:           d.ID,
		Path:         d.Path,
		Name:         d.Name,
		Model:        d.Model,
		Serial:       d.Serial,
		Size:         d.Size,
		Used:         d.Used,
		Health:       d.Health,
		Temperature:  d.Temperature,
		Status:       d.Status,
		IsSystemDisk: d.IsSystemDisk,
		AddedAt:      d.AddedAt,
	}
}

// load 加载数据.
func (m *Manager) load() error {
	dataFile := filepath.Join(m.dataPath, "pools.json")
	data, err := os.ReadFile(dataFile)
	if os.IsNotExist(err) {
		return nil // 首次运行，没有数据文件
	}
	if err != nil {
		return fmt.Errorf("读取数据文件失败: %w", err)
	}

	var persistData struct {
		Pools   map[string]*Pool   `json:"pools"`
		Devices map[string]*Device `json:"devices"`
	}

	if err := json.Unmarshal(data, &persistData); err != nil {
		return fmt.Errorf("解析数据失败: %w", err)
	}

	m.pools = persistData.Pools
	if m.pools == nil {
		m.pools = make(map[string]*Pool)
	}
	m.devices = persistData.Devices
	if m.devices == nil {
		m.devices = make(map[string]*Device)
	}

	return nil
}

// save 保存数据.
func (m *Manager) save() error {
	dataFile := filepath.Join(m.dataPath, "pools.json")

	persistData := struct {
		Pools   map[string]*Pool   `json:"pools"`
		Devices map[string]*Device `json:"devices"`
	}{
		Pools:   m.pools,
		Devices: m.devices,
	}

	data, err := json.MarshalIndent(persistData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	return os.WriteFile(dataFile, data, 0640)
}

// scanDevices 扫描系统设备.
func (m *Manager) scanDevices() error {
	// 扫描 /dev/disk/by-id/ 和 /sys/block/ 获取设备信息
	// 这里简化实现，实际应该读取 /sys/block/ 和 lsblk
	// 实际项目中会调用系统命令或读取 /sys/block/

	// 模拟设备扫描 - 实际实现会使用 lsblk 或读取 /sys/block
	// 这里只做框架，实际会调用系统工具

	return nil
}

// generatePoolID 生成存储池 ID.
func generatePoolID() string {
	return fmt.Sprintf("pool-%d", time.Now().UnixNano())
}
