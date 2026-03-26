// Package storage 提供智能 RAID 管理 (SmartRAID) 功能
// 类似群晖 SHR，支持不同容量硬盘混用、智能空间利用和在线扩容
package storage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nas-os/pkg/btrfs"
)

// ========== 核心数据结构 ==========

// SmartPool 智能存储池
// 支持不同容量硬盘混用，智能分配存储空间.
type SmartPool struct {
	// 基本信息
	Name        string    `json:"name"`        // 池名称
	UUID        string    `json:"uuid"`        // 唯一标识符
	Description string    `json:"description"` // 描述
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间

	// 存储层级
	Tiers []StorageTier `json:"tiers"` // 存储层级（按容量分组）

	// 设备列表
	Devices []*SmartDevice `json:"devices"` // 所有设备

	// RAID 策略
	RAIDPolicy RAIDPolicy `json:"raidPolicy"` // RAID 策略

	// 冗余级别
	RedundancyLevel int `json:"redundancyLevel"` // 冗余级别（1=单盘冗余，2=双盘冗余）

	// 挂载信息
	MountPoint string `json:"mountPoint"` // 挂载点

	// 空间统计
	TotalCapacity  uint64 `json:"totalCapacity"`  // 总容量（实际可用）
	RawCapacity    uint64 `json:"rawCapacity"`    // 原始容量（所有磁盘总和）
	UsedCapacity   uint64 `json:"usedCapacity"`   // 已使用容量
	FreeCapacity   uint64 `json:"freeCapacity"`   // 可用容量
	WastedCapacity uint64 `json:"wastedCapacity"` // 浪费容量（因 RAID 对齐）

	// 子卷
	Subvolumes []*SmartSubvolume `json:"subvolumes"`

	// 状态
	Status SmartPoolStatus `json:"status"`

	// 扩容状态
	ExpansionState *ExpansionState `json:"expansionState,omitempty"`

	// 元数据
	mu sync.RWMutex
}

// StorageTier 存储层级
// 按设备容量分组，每个层级独立管理 RAID.
type StorageTier struct {
	// 层级标识
	ID       int    `json:"id"`       // 层级 ID
	Name     string `json:"name"`     // 层级名称（如 "tier-1", "tier-2"）
	Capacity uint64 `json:"capacity"` // 该层级单盘容量（用于对齐）

	// 设备列表
	Devices []string `json:"devices"` // 属于该层级的设备

	// RAID 配置
	RAIDType string `json:"raidType"` // 该层级的 RAID 类型

	// 空间统计
	RawCapacity  uint64 `json:"rawCapacity"`  // 原始容量
	UsedCapacity uint64 `json:"usedCapacity"` // 已使用
	FreeCapacity uint64 `json:"freeCapacity"` // 可用
	WastedSize   uint64 `json:"wastedSize"`   // 浪费的空间（因容量对齐）

	// 性能指标
	ReadOps  uint64 `json:"readOps"`  // 读操作次数
	WriteOps uint64 `json:"writeOps"` // 写操作次数
}

// SmartDevice 智能设备
// 表示池中的一个设备，包含详细的设备信息.
type SmartDevice struct {
	// 基本信息
	ID       string `json:"id"`       // 设备 ID
	Device   string `json:"device"`   // 设备路径（如 /dev/sda）
	Serial   string `json:"serial"`   // 序列号
	Model    string `json:"model"`    // 型号
	Type     string `json:"type"`     // 设备类型：HDD, SSD, NVMe
	TierID   int    `json:"tierId"`   // 所属层级
	Position int    `json:"position"` // 在层级中的位置

	// 容量信息
	Capacity     uint64 `json:"capacity"`     // 总容量
	UsedCapacity uint64 `json:"usedCapacity"` // 已使用
	FreeCapacity uint64 `json:"freeCapacity"` // 可用

	// 健康状态
	Health       string `json:"health"`       // healthy, warning, failing
	SmartStatus  string `json:"smartStatus"`  // SMART 状态
	Temperature  int    `json:"temperature"`  // 温度（摄氏度）
	PowerOnHours uint64 `json:"powerOnHours"` // 通电时间

	// 状态
	Status     string    `json:"status"`     // online, offline, rebuilding, failed
	AddedAt    time.Time `json:"addedAt"`    // 添加时间
	LastCheck  time.Time `json:"lastCheck"`  // 最后检查时间
	IsNew      bool      `json:"isNew"`      // 是否新添加（等待扩容）
	IsReplaced bool      `json:"isReplaced"` // 是否是替换盘
}

// SmartSubvolume 智能子卷.
type SmartSubvolume struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	ParentID uint64 `json:"parentId"`
	ReadOnly bool   `json:"readOnly"`
	UUID     string `json:"uuid"`
	Size     uint64 `json:"size"`

	// 分层信息
	PrimaryTier int `json:"primaryTier"` // 主要存储层级
}

// RAIDPolicy RAID 策略.
type RAIDPolicy struct {
	// 冗余级别
	RedundancyLevel int `json:"redundancyLevel"` // 1=单盘冗余，2=双盘冗余

	// 自动 RAID 选择
	AutoSelect bool `json:"autoSelect"` // 是否自动选择最佳 RAID

	// 强制 RAID 类型（仅当 AutoSelect=false）
	ForcedRAIDType string `json:"forcedRaidType"` // 强制使用的 RAID 类型

	// 性能优先
	PerformanceMode bool `json:"performanceMode"` // 性能优先模式（优先使用 SSD/NVMe）

	// 混合模式
	MixedModeEnabled bool `json:"mixedModeEnabled"` // 允许不同类型设备混用
}

// SmartPoolStatus 智能池状态.
type SmartPoolStatus struct {
	Healthy           bool    `json:"healthy"`
	BalanceRunning    bool    `json:"balanceRunning"`
	BalanceProgress   float64 `json:"balanceProgress"`
	ScrubRunning      bool    `json:"scrubRunning"`
	ScrubProgress     float64 `json:"scrubProgress"`
	ScrubErrors       uint64  `json:"scrubErrors"`
	ExpansionRunning  bool    `json:"expansionRunning"`
	ExpansionProgress float64 `json:"expansionProgress"`

	// 警告信息
	Warnings []string `json:"warnings"`
}

// ExpansionState 扩容状态.
type ExpansionState struct {
	// 扩容类型
	Type string `json:"type"` // add_device, replace_device, resize

	// 状态
	Status string `json:"status"` // pending, running, completed, failed

	// 进度
	Progress float64 `json:"progress"` // 0-100

	// 时间
	StartedAt    time.Time  `json:"startedAt"`
	EndedAt      *time.Time `json:"endedAt,omitempty"`
	EstimatedEnd time.Time  `json:"estimatedEnd"`

	// 涉及设备
	Devices []string `json:"devices"`

	// 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ========== SmartRAID Manager ==========

// SmartRAIDManager 智能 RAID 管理器.
type SmartRAIDManager struct {
	client    *btrfs.Client
	pools     map[string]*SmartPool
	mu        sync.RWMutex
	mountBase string
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
}

// NewSmartRAIDManager 创建智能 RAID 管理器.
func NewSmartRAIDManager(mountBase string) (*SmartRAIDManager, error) {
	if mountBase == "" {
		mountBase = "/mnt/smart-raid"
	}

	// 确保挂载基础目录存在
	if err := os.MkdirAll(mountBase, 0750); err != nil {
		return nil, fmt.Errorf("创建挂载目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &SmartRAIDManager{
		client:    btrfs.NewClient(true),
		pools:     make(map[string]*SmartPool),
		mountBase: mountBase,
		ctx:       ctx,
		cancel:    cancel,
	}

	// 扫描现有智能池
	if err := m.scanPools(); err != nil {
		return nil, fmt.Errorf("扫描智能池失败: %w", err)
	}

	return m, nil
}

// DefaultRAIDPolicy 默认 RAID 策略.
var DefaultRAIDPolicy = RAIDPolicy{
	RedundancyLevel:  1,
	AutoSelect:       true,
	MixedModeEnabled: true,
}

// ========== 创建智能池 ==========

// CreateSmartPoolRequest 创建智能池请求.
type CreateSmartPoolRequest struct {
	Name            string      `json:"name" binding:"required"`
	Description     string      `json:"description"`
	Devices         []string    `json:"devices" binding:"required,min=1"`
	RAIDPolicy      *RAIDPolicy `json:"raidPolicy"`
	RedundancyLevel int         `json:"redundancyLevel"` // 1 或 2
}

// CreateSmartPool 创建智能存储池.
func (m *SmartRAIDManager) CreateSmartPool(req *CreateSmartPoolRequest) (*SmartPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.pools[req.Name]; exists {
		return nil, fmt.Errorf("智能池 %s 已存在", req.Name)
	}

	// 验证设备
	if len(req.Devices) < 1 {
		return nil, fmt.Errorf("至少需要一个设备")
	}

	// 获取设备信息
	devices := make([]*SmartDevice, 0, len(req.Devices))
	for _, devPath := range req.Devices {
		devInfo, err := m.getDeviceInfo(m.ctx, devPath)
		if err != nil {
			return nil, fmt.Errorf("获取设备 %s 信息失败: %w", devPath, err)
		}
		devices = append(devices, devInfo)
	}

	// 应用 RAID 策略
	policy := DefaultRAIDPolicy
	if req.RAIDPolicy != nil {
		policy = *req.RAIDPolicy
	}
	if req.RedundancyLevel > 0 {
		policy.RedundancyLevel = req.RedundancyLevel
	}

	// 计算存储层级
	tiers := m.calculateTiers(devices, policy)

	// 计算最佳 RAID 配置
	raidConfig := m.selectRAIDConfig(tiers, policy)

	// 创建挂载点
	mountPoint := filepath.Join(m.mountBase, req.Name)
	if err := os.MkdirAll(mountPoint, 0750); err != nil {
		return nil, fmt.Errorf("创建挂载点失败: %w", err)
	}

	// 创建 Btrfs 卷
	if err := m.createBtrfsVolume(req.Name, devices, tiers, raidConfig, mountPoint); err != nil {
		_ = os.RemoveAll(mountPoint)
		return nil, fmt.Errorf("创建 Btrfs 卷失败: %w", err)
	}

	// 计算容量
	totalCap, rawCap, wastedCap := m.calculateCapacity(tiers, raidConfig)

	// 创建智能池对象
	pool := &SmartPool{
		Name:            req.Name,
		UUID:            generateSmartUUID(),
		Description:     req.Description,
		CreatedAt:       time.Now(),
		Tiers:           tiers,
		Devices:         devices,
		RAIDPolicy:      policy,
		RedundancyLevel: policy.RedundancyLevel,
		MountPoint:      mountPoint,
		TotalCapacity:   totalCap,
		RawCapacity:     rawCap,
		WastedCapacity:  wastedCap,
		FreeCapacity:    totalCap,
		Subvolumes:      make([]*SmartSubvolume, 0),
		Status: SmartPoolStatus{
			Healthy: true,
		},
	}

	m.pools[req.Name] = pool
	return pool, nil
}

// calculateTiers 计算存储层级
// 核心算法：将设备按容量分组，相同容量的设备组成一个层级.
func (m *SmartRAIDManager) calculateTiers(devices []*SmartDevice, policy RAIDPolicy) []StorageTier {
	// 按容量排序
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Capacity > devices[j].Capacity
	})

	// 分组：相同容量的设备为一组（允许 5% 容差）
	tierMap := make(map[uint64][]*SmartDevice)
	const tolerance = 0.05 // 5% 容差

	for _, dev := range devices {
		dev := dev
		assigned := false

		// 查找匹配的层级
		for capKey := range tierMap {
			lower := uint64(float64(capKey) * (1 - tolerance))
			upper := uint64(float64(capKey) * (1 + tolerance))

			if dev.Capacity >= lower && dev.Capacity <= upper {
				tierMap[capKey] = append(tierMap[capKey], dev)
				dev.TierID = len(tierMap[capKey])
				assigned = true
				break
			}
		}

		if !assigned {
			tierMap[dev.Capacity] = []*SmartDevice{dev}
			dev.TierID = 1
		}
	}

	// 转换为层级列表
	tiers := make([]StorageTier, 0, len(tierMap))
	tierID := 1

	// 按容量从大到小排序层级
	capKeys := make([]uint64, 0, len(tierMap))
	for k := range tierMap {
		capKeys = append(capKeys, k)
	}
	sort.Slice(capKeys, func(i, j int) bool {
		return capKeys[i] > capKeys[j]
	})

	for _, capKey := range capKeys {
		devs := tierMap[capKey]
		devPaths := make([]string, 0, len(devs))
		var totalRaw uint64

		for i, d := range devs {
			d.TierID = tierID
			d.Position = i + 1
			devPaths = append(devPaths, d.Device)
			totalRaw += d.Capacity
		}

		tier := StorageTier{
			ID:          tierID,
			Name:        fmt.Sprintf("tier-%d", tierID),
			Capacity:    capKey,
			Devices:     devPaths,
			RawCapacity: totalRaw,
		}

		tiers = append(tiers, tier)
		tierID++
	}

	return tiers
}

// selectRAIDConfig 选择 RAID 配置.
func (m *SmartRAIDManager) selectRAIDConfig(tiers []StorageTier, policy RAIDPolicy) *TierRAIDConfig {
	config := &TierRAIDConfig{
		Tiers: make([]TierRAIDInfo, 0, len(tiers)),
	}

	for _, tier := range tiers {
		raidType := m.selectTierRAID(len(tier.Devices), policy)

		tierConfig := TierRAIDInfo{
			TierID:      tier.ID,
			RAIDType:    raidType,
			DataProfile: m.getRAIDDataProfile(raidType),
			MetaProfile: m.getRAIDMetaProfile(raidType),
		}

		config.Tiers = append(config.Tiers, tierConfig)
	}

	return config
}

// selectTierRAID 为单个层级选择 RAID 类型.
func (m *SmartRAIDManager) selectTierRAID(deviceCount int, policy RAIDPolicy) string {
	// 如果强制指定了 RAID 类型
	if !policy.AutoSelect && policy.ForcedRAIDType != "" {
		return policy.ForcedRAIDType
	}

	// 根据设备数量和冗余级别自动选择
	switch {
	case deviceCount == 1:
		return "single"
	case deviceCount == 2 && policy.RedundancyLevel >= 1:
		return "raid1"
	case deviceCount == 3 && policy.RedundancyLevel == 1:
		return "raid5"
	case deviceCount >= 4 && policy.RedundancyLevel == 2:
		return "raid6"
	case deviceCount >= 4:
		if policy.PerformanceMode {
			return "raid10"
		}
		return "raid5"
	case deviceCount == 3:
		return "raid5"
	default:
		return "raid1"
	}
}

// TierRAIDConfig 层级 RAID 配置.
type TierRAIDConfig struct {
	Tiers []TierRAIDInfo `json:"tiers"`
}

// TierRAIDInfo 层级 RAID 信息.
type TierRAIDInfo struct {
	TierID      int    `json:"tierId"`
	RAIDType    string `json:"raidType"`
	DataProfile string `json:"dataProfile"`
	MetaProfile string `json:"metaProfile"`
}

// getRAIDDataProfile 获取 RAID 数据配置.
func (m *SmartRAIDManager) getRAIDDataProfile(raidType string) string {
	profiles := map[string]string{
		"single": "single",
		"raid0":  "raid0",
		"raid1":  "raid1",
		"raid5":  "raid5",
		"raid6":  "raid6",
		"raid10": "raid10",
	}
	if p, ok := profiles[raidType]; ok {
		return p
	}
	return "single"
}

// getRAIDMetaProfile 获取 RAID 元数据配置.
func (m *SmartRAIDManager) getRAIDMetaProfile(raidType string) string {
	// 元数据通常使用更保守的配置
	switch raidType {
	case "raid0":
		return "single" // 元数据不使用 raid0
	case "raid5":
		return "raid1" // 元数据使用 raid1 更安全
	case "raid6":
		return "raid1"
	default:
		return m.getRAIDDataProfile(raidType)
	}
}

// calculateCapacity 计算容量.
func (m *SmartRAIDManager) calculateCapacity(tiers []StorageTier, config *TierRAIDConfig) (total, raw, wasted uint64) {
	for i, tier := range tiers {
		raw += tier.RawCapacity

		if i < len(config.Tiers) {
			raidType := config.Tiers[i].RAIDType
			efficiency := m.getRAIDEfficiency(raidType, len(tier.Devices))

			// 有效容量 = 原始容量 * 效率
			effective := uint64(float64(tier.RawCapacity) * efficiency)
			total += effective

			// 浪费容量 = 原始容量 - 有效容量
			wasted += tier.RawCapacity - effective
		}
	}

	return total, raw, wasted
}

// getRAIDEfficiency 获取 RAID 效率.
func (m *SmartRAIDManager) getRAIDEfficiency(raidType string, deviceCount int) float64 {
	switch raidType {
	case "single":
		return 1.0
	case "raid0":
		return 1.0
	case "raid1":
		return 0.5
	case "raid5":
		return float64(deviceCount-1) / float64(deviceCount)
	case "raid6":
		return float64(deviceCount-2) / float64(deviceCount)
	case "raid10":
		return 0.5
	default:
		return 1.0
	}
}

// createBtrfsVolume 创建 Btrfs 卷.
func (m *SmartRAIDManager) createBtrfsVolume(name string, devices []*SmartDevice, tiers []StorageTier, config *TierRAIDConfig, mountPoint string) error {
	// 收集所有设备路径
	allDevices := make([]string, 0, len(devices))
	for _, d := range devices {
		allDevices = append(allDevices, d.Device)
	}

	// 确定主 RAID 配置（使用第一层级的配置）
	var dataProfile, metaProfile string
	if len(config.Tiers) > 0 {
		dataProfile = config.Tiers[0].DataProfile
		metaProfile = config.Tiers[0].MetaProfile
	} else {
		dataProfile = "single"
		metaProfile = "single"
	}

	// 创建 Btrfs 卷
	if err := m.client.CreateVolume(name, allDevices, dataProfile, metaProfile); err != nil {
		return err
	}

	// 挂载卷
	if err := m.client.Mount(allDevices[0], mountPoint, nil); err != nil {
		_ = m.client.DeleteVolume(allDevices[0])
		return err
	}

	return nil
}

// ========== 设备管理 ==========

// AddDevice 添加设备到智能池
// 支持在线扩容，自动重新计算存储层级.
func (m *SmartRAIDManager) AddDevice(poolName, devicePath string) (*SmartPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("智能池 %s 不存在", poolName)
	}

	if pool.MountPoint == "" {
		return nil, fmt.Errorf("智能池未挂载")
	}

	// 获取新设备信息
	devInfo, err := m.getDeviceInfo(m.ctx, devicePath)
	if err != nil {
		return nil, fmt.Errorf("获取设备信息失败: %w", err)
	}

	// 检查设备是否已在池中
	for _, d := range pool.Devices {
		if d.Device == devicePath {
			return nil, fmt.Errorf("设备 %s 已在池中", devicePath)
		}
	}

	// 标记为新设备，等待扩容
	devInfo.IsNew = true
	devInfo.Status = "online"
	devInfo.AddedAt = time.Now()

	// 启动扩容流程
	go m.expandPool(pool, devInfo)

	return pool, nil
}

// expandPool 扩容池
// 核心扩容算法.
func (m *SmartRAIDManager) expandPool(pool *SmartPool, newDevice *SmartDevice) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// 设置扩容状态
	pool.ExpansionState = &ExpansionState{
		Type:      "add_device",
		Status:    "running",
		StartedAt: time.Now(),
		Devices:   []string{newDevice.Device},
	}
	pool.Status.ExpansionRunning = true
	pool.Status.ExpansionProgress = 0

	defer func() {
		pool.Status.ExpansionRunning = false
		if pool.ExpansionState.Status == "running" {
			pool.ExpansionState.Status = "completed"
			now := time.Now()
			pool.ExpansionState.EndedAt = &now
		}
	}()

	// 第一步：添加设备到 Btrfs 卷
	if err := m.client.AddDevice(pool.MountPoint, newDevice.Device); err != nil {
		pool.ExpansionState.Status = "failed"
		pool.ExpansionState.ErrorMessage = fmt.Sprintf("添加设备失败: %v", err)
		pool.Status.Warnings = append(pool.Status.Warnings, pool.ExpansionState.ErrorMessage)
		return
	}
	pool.Status.ExpansionProgress = 20

	// 第二步：重新计算层级
	oldTiers := pool.Tiers
	pool.Devices = append(pool.Devices, newDevice)
	newTiers := m.calculateTiers(pool.Devices, pool.RAIDPolicy)
	pool.Tiers = newTiers
	pool.Status.ExpansionProgress = 30

	// 第三步：判断是否需要重新分配 RAID
	needRebalance := m.needRebalanceAfterExpansion(oldTiers, newTiers, newDevice)

	if needRebalance {
		// 第四步：执行 balance 以重新分配数据
		if err := m.client.StartBalance(pool.MountPoint); err != nil {
			pool.ExpansionState.Status = "failed"
			pool.ExpansionState.ErrorMessage = fmt.Sprintf("启动数据重平衡失败: %v", err)
			pool.Status.Warnings = append(pool.Status.Warnings, pool.ExpansionState.ErrorMessage)
			return
		}

		// 监控 balance 进度
		m.monitorBalanceProgress(pool)
	}
	pool.Status.ExpansionProgress = 80

	// 第五步：更新容量计算
	raidConfig := m.selectRAIDConfig(pool.Tiers, pool.RAIDPolicy)
	totalCap, rawCap, wastedCap := m.calculateCapacity(pool.Tiers, raidConfig)
	pool.TotalCapacity = totalCap
	pool.RawCapacity = rawCap
	pool.WastedCapacity = wastedCap
	pool.FreeCapacity = totalCap - pool.UsedCapacity

	// 第七步：完成
	newDevice.IsNew = false
	pool.Status.ExpansionProgress = 100
	pool.Status.Warnings = m.removeWarning(pool.Status.Warnings, "扩容")
}

// needRebalanceAfterExpansion 判断扩容后是否需要重平衡.
func (m *SmartRAIDManager) needRebalanceAfterExpansion(oldTiers, newTiers []StorageTier, newDevice *SmartDevice) bool {
	// 如果层级数量变化，需要重平衡
	if len(oldTiers) != len(newTiers) {
		return true
	}

	// 如果新设备创建了新层级，需要重平衡
	for _, tier := range newTiers {
		for _, dev := range tier.Devices {
			if dev == newDevice.Device && len(tier.Devices) == 1 {
				return true
			}
		}
	}

	// 默认需要重平衡以优化空间利用
	return true
}

// monitorBalanceProgress 监控 balance 进度.
func (m *SmartRAIDManager) monitorBalanceProgress(pool *SmartPool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			status, err := m.client.GetBalanceStatus(pool.MountPoint)
			if err != nil {
				continue
			}

			if !status.Running {
				// balance 完成
				return
			}

			pool.Status.BalanceProgress = status.Progress
			pool.Status.ExpansionProgress = 30 + status.Progress*0.5
		}
	}
}

// ReplaceDevice 替换设备
// 支持用更大的设备替换现有设备，自动扩展存储空间.
func (m *SmartRAIDManager) ReplaceDevice(poolName, oldDevice, newDevice string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("智能池 %s 不存在", poolName)
	}

	if pool.MountPoint == "" {
		return fmt.Errorf("智能池未挂载")
	}

	// 验证旧设备存在
	var oldDev *SmartDevice
	for _, d := range pool.Devices {
		if d.Device == oldDevice {
			oldDev = d
			break
		}
	}
	if oldDev == nil {
		return fmt.Errorf("设备 %s 不在池中", oldDevice)
	}

	// 获取新设备信息
	newDevInfo, err := m.getDeviceInfo(m.ctx, newDevice)
	if err != nil {
		return fmt.Errorf("获取新设备信息失败: %w", err)
	}

	// 检查新设备容量是否足够
	if newDevInfo.Capacity < oldDev.Capacity {
		return fmt.Errorf("新设备容量 %d 小于旧设备容量 %d", newDevInfo.Capacity, oldDev.Capacity)
	}

	// 设置扩容状态
	pool.ExpansionState = &ExpansionState{
		Type:      "replace_device",
		Status:    "running",
		StartedAt: time.Now(),
		Devices:   []string{oldDevice, newDevice},
	}
	pool.Status.ExpansionRunning = true

	// 执行设备替换
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "btrfs", "replace", "start", oldDevice, newDevice, pool.MountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		pool.ExpansionState.Status = "failed"
		pool.ExpansionState.ErrorMessage = fmt.Sprintf("设备替换失败: %v, %s", err, string(output))
		pool.Status.ExpansionRunning = false
		return fmt.Errorf("设备替换失败: %w", err)
	}

	// 更新设备列表
	newDevInfo.ID = oldDev.ID
	newDevInfo.TierID = oldDev.TierID
	newDevInfo.Position = oldDev.Position
	newDevInfo.IsReplaced = true
	newDevInfo.AddedAt = time.Now()

	// 移除旧设备，添加新设备
	for i, d := range pool.Devices {
		if d.Device == oldDevice {
			pool.Devices[i] = newDevInfo
			break
		}
	}

	// 监控替换进度
	go m.monitorReplaceProgress(pool, oldDevice, newDevInfo)

	return nil
}

// monitorReplaceProgress 监控设备替换进度.
func (m *SmartRAIDManager) monitorReplaceProgress(pool *SmartPool, oldDevice string, newDevice *SmartDevice) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			status, err := m.getReplaceStatus(pool.MountPoint)
			if err != nil {
				continue
			}

			if status.Finished {
				// 替换完成
				pool.mu.Lock()
				pool.Status.ExpansionRunning = false
				pool.ExpansionState.Status = "completed"
				now := time.Now()
				pool.ExpansionState.EndedAt = &now
				newDevice.IsReplaced = false

				// 重新计算层级和容量
				pool.Tiers = m.calculateTiers(pool.Devices, pool.RAIDPolicy)
				raidConfig := m.selectRAIDConfig(pool.Tiers, pool.RAIDPolicy)
				totalCap, rawCap, wastedCap := m.calculateCapacity(pool.Tiers, raidConfig)
				pool.TotalCapacity = totalCap
				pool.RawCapacity = rawCap
				pool.WastedCapacity = wastedCap
				pool.mu.Unlock()
				return
			}

			pool.Status.ExpansionProgress = status.Progress
		}
	}
}

// ========== 查询操作 ==========

// ListPools 列出所有智能池.
func (m *SmartRAIDManager) ListPools() []*SmartPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SmartPool, 0, len(m.pools))
	for _, p := range m.pools {
		result = append(result, p)
	}
	return result
}

// GetPool 获取智能池.
func (m *SmartRAIDManager) GetPool(name string) *SmartPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[name]
}

// GetPoolStats 获取池统计信息.
func (m *SmartRAIDManager) GetPoolStats(poolName string) (*SmartPoolStats, error) {
	m.mu.RLock()
	pool, exists := m.pools[poolName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("智能池 %s 不存在", poolName)
	}

	if pool.MountPoint == "" {
		return nil, fmt.Errorf("智能池未挂载")
	}

	// 获取 Btrfs 使用情况
	total, used, free, err := m.client.GetUsage(pool.MountPoint)
	if err != nil {
		return nil, err
	}

	stats := &SmartPoolStats{
		PoolName:        poolName,
		TotalCapacity:   total,
		UsedCapacity:    used,
		FreeCapacity:    free,
		RawCapacity:     pool.RawCapacity,
		WastedCapacity:  pool.WastedCapacity,
		TierCount:       len(pool.Tiers),
		DeviceCount:     len(pool.Devices),
		RedundancyLevel: pool.RedundancyLevel,
		Healthy:         pool.Status.Healthy,
	}

	// 层级统计
	stats.TierStats = make([]TierStats, 0, len(pool.Tiers))
	for _, tier := range pool.Tiers {
		stats.TierStats = append(stats.TierStats, TierStats{
			TierID:      tier.ID,
			DeviceCount: len(tier.Devices),
			RawCapacity: tier.RawCapacity,
			RAIDType:    tier.RAIDType,
		})
	}

	// 设备类型统计
	for _, dev := range pool.Devices {
		switch dev.Type {
		case "NVMe":
			stats.NVMeCount++
		case "SSD":
			stats.SSDCount++
		case "HDD":
			stats.HDDCount++
		}
	}

	return stats, nil
}

// SmartPoolStats 智能池统计信息.
type SmartPoolStats struct {
	PoolName        string      `json:"poolName"`
	TotalCapacity   uint64      `json:"totalCapacity"`
	UsedCapacity    uint64      `json:"usedCapacity"`
	FreeCapacity    uint64      `json:"freeCapacity"`
	RawCapacity     uint64      `json:"rawCapacity"`
	WastedCapacity  uint64      `json:"wastedCapacity"`
	TierCount       int         `json:"tierCount"`
	DeviceCount     int         `json:"deviceCount"`
	RedundancyLevel int         `json:"redundancyLevel"`
	Healthy         bool        `json:"healthy"`
	TierStats       []TierStats `json:"tierStats"`
	NVMeCount       int         `json:"nvmeCount"`
	SSDCount        int         `json:"ssdCount"`
	HDDCount        int         `json:"hddCount"`
}

// TierStats 层级统计.
type TierStats struct {
	TierID      int    `json:"tierId"`
	DeviceCount int    `json:"deviceCount"`
	RawCapacity uint64 `json:"rawCapacity"`
	RAIDType    string `json:"raidType"`
}

// GetExpansionPlan 获取扩容计划
// 分析当前池状态，提供扩容建议.
func (m *SmartRAIDManager) GetExpansionPlan(poolName string) (*ExpansionPlan, error) {
	m.mu.RLock()
	pool, exists := m.pools[poolName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("智能池 %s 不存在", poolName)
	}

	plan := &ExpansionPlan{
		PoolName:        poolName,
		CurrentState:    pool,
		Recommendations: make([]ExpansionRecommendation, 0),
	}

	// 分析当前利用率
	utilizationRate := float64(pool.UsedCapacity) / float64(pool.TotalCapacity)

	// 如果利用率超过 80%，建议扩容
	if utilizationRate > 0.8 {
		plan.Recommendations = append(plan.Recommendations, ExpansionRecommendation{
			Type:        "add_capacity",
			Priority:    "high",
			Reason:      fmt.Sprintf("存储利用率 %.1f%%，建议扩容", utilizationRate*100),
			Suggestion:  "添加新硬盘以增加存储容量",
			MinCapacity: pool.UsedCapacity / 4, // 建议添加至少 25% 当前容量的新盘
		})
	}

	// 分析层级不平衡
	if len(pool.Tiers) > 1 {
		maxTierCap := pool.Tiers[0].RawCapacity
		minTierCap := pool.Tiers[len(pool.Tiers)-1].RawCapacity

		if maxTierCap > minTierCap*2 {
			plan.Recommendations = append(plan.Recommendations, ExpansionRecommendation{
				Type:       "balance_tiers",
				Priority:   "medium",
				Reason:     "存储层级不平衡，小盘空间利用效率低",
				Suggestion: "考虑将小盘替换为大盘，或添加更多相同容量的硬盘",
			})
		}
	}

	// 分析设备健康状态
	for _, dev := range pool.Devices {
		if dev.Health == "warning" {
			plan.Recommendations = append(plan.Recommendations, ExpansionRecommendation{
				Type:       "replace_device",
				Priority:   "high",
				Reason:     fmt.Sprintf("设备 %s 状态警告", dev.Device),
				Suggestion: fmt.Sprintf("建议尽快替换设备 %s", dev.Device),
				DevicePath: dev.Device,
			})
		}
	}

	// 计算潜在扩容空间
	plan.PotentialCapacity = m.calculatePotentialCapacity(pool)

	return plan, nil
}

// ExpansionPlan 扩容计划.
type ExpansionPlan struct {
	PoolName          string                    `json:"poolName"`
	CurrentState      *SmartPool                `json:"currentState"`
	Recommendations   []ExpansionRecommendation `json:"recommendations"`
	PotentialCapacity uint64                    `json:"potentialCapacity"` // 潜在扩容空间
}

// ExpansionRecommendation 扩容建议.
type ExpansionRecommendation struct {
	Type        string `json:"type"`     // add_capacity, replace_device, balance_tiers
	Priority    string `json:"priority"` // high, medium, low
	Reason      string `json:"reason"`
	Suggestion  string `json:"suggestion"`
	DevicePath  string `json:"devicePath,omitempty"`
	MinCapacity uint64 `json:"minCapacity,omitempty"`
}

// calculatePotentialCapacity 计算潜在容量.
func (m *SmartRAIDManager) calculatePotentialCapacity(pool *SmartPool) uint64 {
	// 如果所有设备都替换为最大容量设备的容量
	if len(pool.Devices) == 0 {
		return 0
	}

	maxCap := pool.Devices[0].Capacity
	for _, d := range pool.Devices {
		if d.Capacity > maxCap {
			maxCap = d.Capacity
		}
	}

	// 计算全部替换为最大盘时的容量
	efficiency := m.getRAIDEfficiency(pool.Tiers[0].RAIDType, len(pool.Devices))
	return uint64(float64(maxCap) * float64(len(pool.Devices)) * efficiency)
}

// ========== 子卷管理 ==========

// CreateSubvolume 创建子卷.
func (m *SmartRAIDManager) CreateSubvolume(poolName, subvolName string) (*SmartSubvolume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("智能池 %s 不存在", poolName)
	}

	if pool.MountPoint == "" {
		return nil, fmt.Errorf("智能池未挂载")
	}

	// 检查是否已存在
	for _, sv := range pool.Subvolumes {
		if sv.Name == subvolName {
			return nil, fmt.Errorf("子卷 %s 已存在", subvolName)
		}
	}

	// 创建子卷
	subvolPath := filepath.Join(pool.MountPoint, subvolName)
	if err := m.client.CreateSubVolume(subvolPath); err != nil {
		return nil, err
	}

	// 获取子卷信息
	info, err := m.client.GetSubVolumeInfo(subvolPath)
	if err != nil {
		_ = m.client.DeleteSubVolume(subvolPath)
		return nil, err
	}

	subvol := &SmartSubvolume{
		ID:       info.ID,
		Name:     subvolName,
		Path:     subvolPath,
		ParentID: info.ParentID,
		ReadOnly: info.ReadOnly,
		UUID:     info.UUID,
	}

	pool.Subvolumes = append(pool.Subvolumes, subvol)
	return subvol, nil
}

// DeleteSubvolume 删除子卷.
func (m *SmartRAIDManager) DeleteSubvolume(poolName, subvolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolName]
	if !exists {
		return fmt.Errorf("智能池 %s 不存在", poolName)
	}

	var subvol *SmartSubvolume
	var index int
	for i, sv := range pool.Subvolumes {
		if sv.Name == subvolName {
			subvol = sv
			index = i
			break
		}
	}

	if subvol == nil {
		return fmt.Errorf("子卷 %s 不存在", subvolName)
	}

	if err := m.client.DeleteSubVolume(subvol.Path); err != nil {
		return err
	}

	pool.Subvolumes = append(pool.Subvolumes[:index], pool.Subvolumes[index+1:]...)
	return nil
}

// ========== 辅助方法 ==========

// getDeviceInfo 获取设备信息.
func (m *SmartRAIDManager) getDeviceInfo(ctx context.Context, devicePath string) (*SmartDevice, error) {
	dev := &SmartDevice{
		ID:     generateSmartUUID(),
		Device: devicePath,
		Status: "online",
	}

	// 获取设备容量
	cmd := exec.CommandContext(ctx, "sudo", "blockdev", "--getsize64", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取设备容量失败: %w", err)
	}
	dev.Capacity = parseUint64(strings.TrimSpace(string(output)))

	// 获取设备信息
	cmd = exec.CommandContext(ctx, "sudo", "lsblk", "-d", "-o", "NAME,SERIAL,MODEL,ROTA", "-n", devicePath)
	output, err = cmd.Output()
	if err == nil {
		fields := strings.Fields(string(output))
		if len(fields) >= 4 {
			dev.Serial = fields[1]
			dev.Model = fields[2]
			// ROTA: 1 = HDD, 0 = SSD
			if fields[3] == "0" {
				// 检查是否 NVMe
				if strings.Contains(devicePath, "nvme") {
					dev.Type = "NVMe"
				} else {
					dev.Type = "SSD"
				}
			} else {
				dev.Type = "HDD"
			}
		}
	}

	// 获取 SMART 信息
	cmd = exec.CommandContext(ctx, "sudo", "smartctl", "-H", devicePath)
	output, _ = cmd.Output()
	if strings.Contains(string(output), "PASSED") {
		dev.Health = "healthy"
		dev.SmartStatus = "PASSED"
	} else if strings.Contains(string(output), "WARNING") {
		dev.Health = "warning"
		dev.SmartStatus = "WARNING"
	} else {
		dev.Health = "unknown"
		dev.SmartStatus = "UNKNOWN"
	}

	dev.AddedAt = time.Now()
	dev.LastCheck = time.Now()

	return dev, nil
}

// scanPools 扫描现有智能池.
func (m *SmartRAIDManager) scanPools() error {
	// 从配置文件或数据库加载
	// 这里简化实现
	return nil
}

// getReplaceStatus 获取设备替换状态.
func (m *SmartRAIDManager) getReplaceStatus(mountPoint string) (*ReplaceStatus, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sudo", "btrfs", "replace", "status", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取替换状态失败: %w", err)
	}

	status := &ReplaceStatus{}
	if strings.Contains(string(output), "finished") {
		status.Finished = true
		status.Progress = 100
	} else if strings.Contains(string(output), "started") || strings.Contains(string(output), "running") {
		status.Running = true
		// 解析进度
		if idx := strings.Index(string(output), "%"); idx > 0 {
			percentStr := string(output)[:idx]
			var numStr string
			for _, c := range percentStr {
				if c >= '0' && c <= '9' || c == '.' {
					numStr += string(c)
				}
			}
			if numStr != "" {
				_, _ = fmt.Sscanf(numStr, "%f", &status.Progress)
			}
		}
	}

	return status, nil
}

// removeWarning 移除警告.
func (m *SmartRAIDManager) removeWarning(warnings []string, keyword string) []string {
	result := make([]string, 0)
	for _, w := range warnings {
		if !strings.Contains(w, keyword) {
			result = append(result, w)
		}
	}
	return result
}

// parseUint64 解析 uint64.
func parseUint64(s string) uint64 {
	var result uint64
	_, _ = fmt.Sscanf(s, "%d", &result)
	return result
}

// generateSmartUUID 生成 UUID.
func generateSmartUUID() string {
	return fmt.Sprintf("smart-%d", time.Now().UnixNano())
}

// DeletePool 删除智能池.
func (m *SmartRAIDManager) DeletePool(name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[name]
	if !exists {
		return fmt.Errorf("智能池 %s 不存在", name)
	}

	// 检查子卷
	if len(pool.Subvolumes) > 0 && !force {
		return fmt.Errorf("智能池包含 %d 个子卷，请先删除子卷或使用强制删除", len(pool.Subvolumes))
	}

	// 卸载
	if pool.MountPoint != "" {
		_ = m.client.Unmount(pool.MountPoint)
	}

	// 删除文件系统
	for _, dev := range pool.Devices {
		_ = m.client.DeleteVolume(dev.Device)
	}

	// 删除挂载点
	_ = os.RemoveAll(pool.MountPoint)

	delete(m.pools, name)
	return nil
}

// Start 启动监控.
func (m *SmartRAIDManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("智能 RAID 管理器已在运行")
	}

	m.running = true
	return nil
}

// Stop 停止监控.
func (m *SmartRAIDManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.running = false
}
