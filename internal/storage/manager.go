// Package storage 提供存储管理功能
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/pkg/btrfs"
)

// Manager 存储管理器
type Manager struct {
	client    *btrfs.Client
	volumes   map[string]*Volume
	mu        sync.RWMutex
	mountBase string // 挂载基础目录
}

// Volume 卷信息
type Volume struct {
	Name       string            `json:"name"`
	UUID       string            `json:"uuid"`
	Devices    []string          `json:"devices"`
	Size       uint64            `json:"size"`  // 总大小（字节）
	Used       uint64            `json:"used"`  // 已使用（字节）
	Free       uint64            `json:"free"`  // 可用空间（字节）
	DataProfile string           `json:"dataProfile"`  // 数据配置（single, raid0, raid1, raid5, raid6, raid10）
	MetaProfile string           `json:"metaProfile"`  // 元数据配置
	MountPoint string            `json:"mountPoint"`   // 挂载点
	Subvolumes []*SubVolume      `json:"subvolumes"`
	Status     VolumeStatus      `json:"status"`
	CreatedAt  time.Time         `json:"createdAt"`
}

// VolumeStatus 卷状态
type VolumeStatus struct {
	BalanceRunning bool    `json:"balanceRunning"`
	BalanceProgress float64 `json:"balanceProgress"`
	ScrubRunning    bool    `json:"scrubRunning"`
	ScrubProgress   float64 `json:"scrubProgress"`
	ScrubErrors     uint64  `json:"scrubErrors"`
	Healthy         bool    `json:"healthy"`
}

// SubVolume 子卷信息
type SubVolume struct {
	ID        uint64      `json:"id"`
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	ParentID  uint64      `json:"parentId"`
	ReadOnly  bool        `json:"readOnly"`
	UUID      string      `json:"uuid"`
	Size      uint64      `json:"size"` // 估算大小
	Snapshots []*Snapshot `json:"snapshots"`
}

// Snapshot 快照信息
type Snapshot struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Source     string    `json:"source"`     // 源子卷名
	SourceUUID string    `json:"sourceUuid"` // 源子卷 UUID
	ReadOnly   bool      `json:"readOnly"`
	CreatedAt  time.Time `json:"createdAt"`
	Size       uint64    `json:"size"` // 快照大小（估算）
}

// SnapshotConfig 快照配置
type SnapshotConfig struct {
	Prefix     string        // 快照名称前缀
	Suffix     string        // 快照名称后缀
	ReadOnly   bool          // 是否创建只读快照
	Timestamp  bool          // 是否添加时间戳
	TimeFormat string        // 时间格式（默认 20060102-150405）
	SnapDir    string        // 快照目录（默认 .snapshots）
}

// DefaultSnapshotConfig 默认快照配置
var DefaultSnapshotConfig = SnapshotConfig{
	Prefix:     "",
	Suffix:     "",
	ReadOnly:   true,
	Timestamp:  true,
	TimeFormat: "20060102-150405",
	SnapDir:    ".snapshots",
}

// RAIDConfig RAID 配置
type RAIDConfig struct {
	DataProfile     string   `json:"dataProfile"`     // single, raid0, raid1, raid5, raid6, raid10
	MetaProfile     string   `json:"metaProfile"`     // 元数据配置
	MinDevices      int      `json:"minDevices"`      // 最少设备数
	RecommendedDev  int      `json:"recommendedDev"`  // 推荐设备数
	FaultTolerance  int      `json:"faultTolerance"`  // 容错磁盘数
	Description     string   `json:"description"`
}

// 预定义的 RAID 配置
var RAIDConfigs = map[string]RAIDConfig{
	"single": {
		DataProfile:    "single",
		MetaProfile:    "single",
		MinDevices:     1,
		RecommendedDev: 1,
		FaultTolerance: 0,
		Description:    "单盘模式，无冗余",
	},
	"raid0": {
		DataProfile:    "raid0",
		MetaProfile:    "raid0",
		MinDevices:     2,
		RecommendedDev: 2,
		FaultTolerance: 0,
		Description:    "条带模式，性能最佳，无冗余",
	},
	"raid1": {
		DataProfile:    "raid1",
		MetaProfile:    "raid1",
		MinDevices:     2,
		RecommendedDev: 2,
		FaultTolerance: 1,
		Description:    "镜像模式，允许 1 盘故障",
	},
	"raid10": {
		DataProfile:    "raid10",
		MetaProfile:    "raid10",
		MinDevices:     4,
		RecommendedDev: 4,
		FaultTolerance: 1,
		Description:    "条带镜像，性能与冗余平衡",
	},
	"raid5": {
		DataProfile:    "raid5",
		MetaProfile:    "raid5",
		MinDevices:     3,
		RecommendedDev: 3,
		FaultTolerance: 1,
		Description:    "分布式奇偶校验，允许 1 盘故障",
	},
	"raid6": {
		DataProfile:    "raid6",
		MetaProfile:    "raid6",
		MinDevices:     4,
		RecommendedDev: 4,
		FaultTolerance: 2,
		Description:    "双奇偶校验，允许 2 盘故障",
	},
}

// NewManager 创建存储管理器
func NewManager(mountBase string) (*Manager, error) {
	if mountBase == "" {
		mountBase = "/mnt"
	}
	
	// 确保挂载基础目录存在
	if err := os.MkdirAll(mountBase, 0755); err != nil {
		return nil, fmt.Errorf("创建挂载目录失败: %w", err)
	}
	
	m := &Manager{
		client:    btrfs.NewClient(true), // 使用 sudo
		volumes:   make(map[string]*Volume),
		mountBase: mountBase,
	}
	
	// 扫描现有 btrfs 卷
	if err := m.scanVolumes(); err != nil {
		return nil, fmt.Errorf("扫描卷失败: %w", err)
	}
	
	return m, nil
}

// scanVolumes 扫描系统上的 btrfs 卷
func (m *Manager) scanVolumes() error {
	vols, err := m.client.ListVolumes()
	if err != nil {
		return err
	}
	
	for _, v := range vols {
		volume := &Volume{
			Name:        v.Name,
			UUID:        v.UUID,
			Devices:     v.Devices,
			DataProfile: v.Profile,
			MetaProfile: v.Metadata,
			MountPoint:  filepath.Join(m.mountBase, v.Name),
			CreatedAt:   time.Now(),
		}
		
		// 检查是否已挂载
		if _, err := os.Stat(volume.MountPoint); err == nil {
			// 获取使用情况
			total, used, free, err := m.client.GetUsage(volume.MountPoint)
			if err == nil {
				volume.Size = total
				volume.Used = used
				volume.Free = free
			}
			
			// 获取子卷列表
			m.scanSubvolumes(volume)
		}
		
		m.volumes[v.Name] = volume
	}
	
	return nil
}

// scanSubvolumes 扫描卷的子卷
func (m *Manager) scanSubvolumes(vol *Volume) error {
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	subvols, err := m.client.ListSubVolumes(vol.MountPoint)
	if err != nil {
		return err
	}
	
	vol.Subvolumes = make([]*SubVolume, 0, len(subvols))
	for _, sv := range subvols {
		subvol := &SubVolume{
			ID:       sv.ID,
			Name:     sv.Name,
			Path:     sv.Path,
			ParentID: sv.ParentID,
			ReadOnly: sv.ReadOnly,
			UUID:     sv.UUID,
		}
		vol.Subvolumes = append(vol.Subvolumes, subvol)
	}
	
	return nil
}

// ========== 卷管理 ==========

// ListVolumes 获取所有卷
func (m *Manager) ListVolumes() []*Volume {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		result = append(result, v)
	}
	return result
}

// GetVolume 获取指定卷
func (m *Manager) GetVolume(name string) *Volume {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.volumes[name]
}

// CreateVolume 创建新卷
func (m *Manager) CreateVolume(name string, devices []string, profile string) (*Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查是否已存在
	if _, exists := m.volumes[name]; exists {
		return nil, fmt.Errorf("卷 %s 已存在", name)
	}
	
	// 验证 RAID 配置
	config, ok := RAIDConfigs[profile]
	if !ok {
		profile = "single"
		config = RAIDConfigs["single"]
	}
	
	if len(devices) < config.MinDevices {
		return nil, fmt.Errorf("RAID %s 需要至少 %d 个设备，当前 %d 个", profile, config.MinDevices, len(devices))
	}
	
	// 创建 btrfs 卷
	if err := m.client.CreateVolume(name, devices, config.DataProfile, config.MetaProfile); err != nil {
		return nil, err
	}
	
	// 创建挂载点
	mountPoint := filepath.Join(m.mountBase, name)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return nil, fmt.Errorf("创建挂载点失败: %w", err)
	}
	
	// 挂载卷
	if err := m.client.Mount(devices[0], mountPoint, nil); err != nil {
		return nil, fmt.Errorf("挂载卷失败: %w", err)
	}
	
	vol := &Volume{
		Name:        name,
		Devices:     devices,
		DataProfile: config.DataProfile,
		MetaProfile: config.MetaProfile,
		MountPoint:  mountPoint,
		Subvolumes:  make([]*SubVolume, 0),
		Status: VolumeStatus{
			Healthy: true,
		},
		CreatedAt: time.Now(),
	}
	
	m.volumes[name] = vol
	return vol, nil
}

// DeleteVolume 删除卷（危险操作）
func (m *Manager) DeleteVolume(name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[name]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", name)
	}
	
	// 检查是否有子卷
	if len(vol.Subvolumes) > 0 && !force {
		return fmt.Errorf("卷包含 %d 个子卷，请先删除子卷或使用强制删除", len(vol.Subvolumes))
	}
	
	// 卸载
	if vol.MountPoint != "" {
		if err := m.client.Unmount(vol.MountPoint); err != nil {
			if !force {
				return fmt.Errorf("卸载失败: %w", err)
			}
		}
	}
	
	// 删除文件系统
	for _, dev := range vol.Devices {
		if err := m.client.DeleteVolume(dev); err != nil {
			if !force {
				return err
			}
		}
	}
	
	// 删除挂载点
	if vol.MountPoint != "" {
		os.RemoveAll(vol.MountPoint)
	}
	
	delete(m.volumes, name)
	return nil
}

// MountVolume 挂载卷
func (m *Manager) MountVolume(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[name]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", name)
	}
	
	if vol.MountPoint == "" {
		vol.MountPoint = filepath.Join(m.mountBase, name)
	}
	
	// 创建挂载点
	if err := os.MkdirAll(vol.MountPoint, 0755); err != nil {
		return fmt.Errorf("创建挂载点失败: %w", err)
	}
	
	// 挂载
	if err := m.client.Mount(vol.Devices[0], vol.MountPoint, nil); err != nil {
		return err
	}
	
	// 更新使用情况
	total, used, free, err := m.client.GetUsage(vol.MountPoint)
	if err == nil {
		vol.Size = total
		vol.Used = used
		vol.Free = free
	}
	
	// 刷新子卷列表
	m.scanSubvolumes(vol)
	
	return nil
}

// UnmountVolume 卸载卷
func (m *Manager) UnmountVolume(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[name]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", name)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	if err := m.client.Unmount(vol.MountPoint); err != nil {
		return err
	}
	
	return nil
}

// GetUsage 获取卷使用情况
func (m *Manager) GetUsage(name string) (total, used, free uint64, err error) {
	m.mu.RLock()
	vol, exists := m.volumes[name]
	m.mu.RUnlock()
	
	if !exists {
		return 0, 0, 0, fmt.Errorf("卷 %s 不存在", name)
	}
	
	if vol.MountPoint == "" {
		return 0, 0, 0, fmt.Errorf("卷未挂载")
	}
	
	return m.client.GetUsage(vol.MountPoint)
}

// ========== 子卷管理 ==========

// ListSubVolumes 列出卷的所有子卷
func (m *Manager) ListSubVolumes(volumeName string) ([]*SubVolume, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	// 刷新子卷列表
	m.mu.Lock()
	m.scanSubvolumes(vol)
	m.mu.Unlock()
	
	return vol.Subvolumes, nil
}

// CreateSubVolume 创建子卷
func (m *Manager) CreateSubVolume(volumeName, name string) (*SubVolume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	// 检查子卷是否已存在
	for _, sv := range vol.Subvolumes {
		if sv.Name == name {
			return nil, fmt.Errorf("子卷 %s 已存在", name)
		}
	}
	
	// 创建子卷路径
	subvolPath := filepath.Join(vol.MountPoint, name)
	
	if err := m.client.CreateSubVolume(subvolPath); err != nil {
		return nil, err
	}
	
	// 获取子卷信息
	info, err := m.client.GetSubVolumeInfo(subvolPath)
	if err != nil {
		return nil, err
	}
	
	subvol := &SubVolume{
		ID:       info.ID,
		Name:     name,
		Path:     subvolPath,
		ParentID: info.ParentID,
		ReadOnly: info.ReadOnly,
		UUID:     info.UUID,
	}
	
	vol.Subvolumes = append(vol.Subvolumes, subvol)
	return subvol, nil
}

// DeleteSubVolume 删除子卷
func (m *Manager) DeleteSubVolume(volumeName, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	// 查找子卷
	var subvol *SubVolume
	var index int
	for i, sv := range vol.Subvolumes {
		if sv.Name == name {
			subvol = sv
			index = i
			break
		}
	}
	
	if subvol == nil {
		return fmt.Errorf("子卷 %s 不存在", name)
	}
	
	// 删除子卷
	if err := m.client.DeleteSubVolume(subvol.Path); err != nil {
		return err
	}
	
	// 从列表中移除
	vol.Subvolumes = append(vol.Subvolumes[:index], vol.Subvolumes[index+1:]...)
	return nil
}

// GetSubVolume 获取子卷详情
func (m *Manager) GetSubVolume(volumeName, name string) (*SubVolume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	for _, sv := range vol.Subvolumes {
		if sv.Name == name {
			return sv, nil
		}
	}
	
	return nil, fmt.Errorf("子卷 %s 不存在", name)
}

// SetSubVolumeReadOnly 设置子卷只读属性
func (m *Manager) SetSubVolumeReadOnly(volumeName, name string, readOnly bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	var subvol *SubVolume
	for _, sv := range vol.Subvolumes {
		if sv.Name == name {
			subvol = sv
			break
		}
	}
	
	if subvol == nil {
		return fmt.Errorf("子卷 %s 不存在", name)
	}
	
	if err := m.client.SetSubVolumeReadOnly(subvol.Path, readOnly); err != nil {
		return err
	}
	
	subvol.ReadOnly = readOnly
	return nil
}

// MountSubVolume 挂载子卷到指定目录
// mountPath: 挂载目标路径，如 "/mnt/documents"
func (m *Manager) MountSubVolume(volumeName, subvolName, mountPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	// 查找子卷
	var subvol *SubVolume
	for _, sv := range vol.Subvolumes {
		if sv.Name == subvolName {
			subvol = sv
			break
		}
	}
	
	if subvol == nil {
		return fmt.Errorf("子卷 %s 不存在", subvolName)
	}
	
	// 创建挂载点
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return fmt.Errorf("创建挂载点失败: %w", err)
	}
	
	// 挂载子卷
	if err := m.client.MountSubVolume(vol.Devices[0], subvolName, mountPath); err != nil {
		os.Remove(mountPath)
		return err
	}
	
	return nil
}

// MountSubVolumeByID 通过 ID 挂载子卷
func (m *Manager) MountSubVolumeByID(volumeName string, subvolID uint64, mountPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	// 创建挂载点
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return fmt.Errorf("创建挂载点失败: %w", err)
	}
	
	// 通过 ID 挂载
	if err := m.client.MountSubVolumeByID(vol.Devices[0], subvolID, mountPath); err != nil {
		os.Remove(mountPath)
		return err
	}
	
	return nil
}

// UnmountSubVolume 卸载子卷挂载
func (m *Manager) UnmountSubVolume(mountPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if err := m.client.Unmount(mountPath); err != nil {
		return err
	}
	
	// 可选：删除空挂载点
	os.Remove(mountPath)
	
	return nil
}

// GetDefaultSubVolume 获取卷的默认子卷
func (m *Manager) GetDefaultSubVolume(volumeName string) (uint64, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return 0, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return 0, fmt.Errorf("卷未挂载")
	}
	
	return m.client.GetDefaultSubVolume(vol.MountPoint)
}

// SetDefaultSubVolume 设置卷的默认子卷
func (m *Manager) SetDefaultSubVolume(volumeName string, subvolID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	return m.client.SetDefaultSubVolume(vol.MountPoint, subvolID)
}

// ========== 快照管理 ==========

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(volumeName, subvolName, snapshotName string, readOnly bool) (*Snapshot, error) {
	config := DefaultSnapshotConfig
	config.ReadOnly = readOnly
	return m.CreateSnapshotWithConfig(volumeName, subvolName, snapshotName, &config)
}

// CreateSnapshotWithConfig 使用配置创建快照
func (m *Manager) CreateSnapshotWithConfig(volumeName, subvolName, snapshotName string, config *SnapshotConfig) (*Snapshot, error) {
	if config == nil {
		config = &DefaultSnapshotConfig
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	// 查找源子卷
	var subvol *SubVolume
	for _, sv := range vol.Subvolumes {
		if sv.Name == subvolName {
			subvol = sv
			break
		}
	}
	
	if subvol == nil {
		return nil, fmt.Errorf("子卷 %s 不存在", subvolName)
	}
	
	// 构建快照名称
	if config.Timestamp {
		timeStr := time.Now().Format(config.TimeFormat)
		if snapshotName == "" {
			snapshotName = timeStr
		} else {
			snapshotName = snapshotName + "-" + timeStr
		}
	}
	snapshotName = config.Prefix + snapshotName + config.Suffix
	
	// 确定快照目录
	snapDir := config.SnapDir
	if snapDir == "" {
		snapDir = DefaultSnapshotConfig.SnapDir
	}
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	
	// 创建快照目录
	if err := os.MkdirAll(snapDirPath, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}
	
	snapPath := filepath.Join(snapDirPath, snapshotName)
	
	// 检查快照是否已存在
	if _, err := os.Stat(snapPath); err == nil {
		return nil, fmt.Errorf("快照 %s 已存在", snapshotName)
	}
	
	// 创建快照
	if err := m.client.CreateSnapshot(subvol.Path, snapPath, config.ReadOnly); err != nil {
		return nil, err
	}
	
	// 获取快照信息
	info, err := m.client.GetSubVolumeInfo(snapPath)
	if err != nil {
		// 快照已创建，但无法获取信息
		info = &btrfs.SubVolumeInfo{}
	}
	
	snap := &Snapshot{
		Name:       snapshotName,
		Path:       snapPath,
		Source:     subvolName,
		SourceUUID: subvol.UUID,
		ReadOnly:   config.ReadOnly,
		CreatedAt:  time.Now(),
	}
	if info != nil {
		snap.ReadOnly = info.ReadOnly
	}
	
	// 添加到子卷的快照列表
	if subvol.Snapshots == nil {
		subvol.Snapshots = make([]*Snapshot, 0)
	}
	subvol.Snapshots = append(subvol.Snapshots, snap)
	
	return snap, nil
}

// CreateTimedSnapshot 创建带时间戳的快照
func (m *Manager) CreateTimedSnapshot(volumeName, subvolName, prefix string, readOnly bool) (*Snapshot, error) {
	config := &SnapshotConfig{
		Prefix:     prefix,
		ReadOnly:   readOnly,
		Timestamp:  true,
		TimeFormat: "20060102-150405",
		SnapDir:    ".snapshots",
	}
	return m.CreateSnapshotWithConfig(volumeName, subvolName, "", config)
}

// ListSnapshots 列出卷的所有快照
func (m *Manager) ListSnapshots(volumeName string) ([]*Snapshot, error) {
	return m.ListSnapshotsInDir(volumeName, "")
}

// ListSnapshotsInDir 列出指定目录下的快照
func (m *Manager) ListSnapshotsInDir(volumeName, snapDir string) ([]*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	// 确定快照目录
	if snapDir == "" {
		snapDir = ".snapshots"
	}
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	
	entries, err := os.ReadDir(snapDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Snapshot{}, nil
		}
		return nil, err
	}
	
	snapshots := make([]*Snapshot, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		snapPath := filepath.Join(snapDirPath, entry.Name())
		info, err := m.client.GetSubVolumeInfo(snapPath)
		if err != nil {
			continue
		}
		
		// 获取创建时间
		stat, err := os.Stat(snapPath)
		var createdAt time.Time
		if err == nil {
			createdAt = stat.ModTime()
		}
		
		// 尝试获取父卷 UUID（源子卷 UUID）
		var sourceUUID string
		parentInfo, err := m.client.GetSubVolumeInfo(snapPath)
		if err == nil && parentInfo != nil {
			// 快照的 ParentID 指向源子卷
			_ = parentInfo.ParentID // 用于后续查找源子卷名称
		}
		
		snapshots = append(snapshots, &Snapshot{
			Name:       entry.Name(),
			Path:       snapPath,
			ReadOnly:   info.ReadOnly,
			SourceUUID: sourceUUID,
			CreatedAt:  createdAt,
		})
	}
	
	return snapshots, nil
}

// ListSubVolumeSnapshots 列出指定子卷的快照
func (m *Manager) ListSubVolumeSnapshots(volumeName, subvolName string) ([]*Snapshot, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	// 查找子卷
	for _, sv := range vol.Subvolumes {
		if sv.Name == subvolName {
			return sv.Snapshots, nil
		}
	}
	
	return nil, fmt.Errorf("子卷 %s 不存在", subvolName)
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(volumeName, snapshotName string) error {
	return m.DeleteSnapshotInDir(volumeName, snapshotName, "")
}

// DeleteSnapshotInDir 删除指定目录下的快照
func (m *Manager) DeleteSnapshotInDir(volumeName, snapshotName, snapDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if snapDir == "" {
		snapDir = ".snapshots"
	}
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	snapPath := filepath.Join(snapDirPath, snapshotName)
	
	// 检查快照是否存在
	if _, err := os.Stat(snapPath); err != nil {
		return fmt.Errorf("快照 %s 不存在", snapshotName)
	}
	
	if err := m.client.DeleteSnapshot(snapPath); err != nil {
		return err
	}
	
	// 从子卷快照列表中移除
	for _, sv := range vol.Subvolumes {
		for i, snap := range sv.Snapshots {
			if snap.Name == snapshotName {
				sv.Snapshots = append(sv.Snapshots[:i], sv.Snapshots[i+1:]...)
				break
			}
		}
	}
	
	return nil
}

// GetSnapshot 获取快照详情
func (m *Manager) GetSnapshot(volumeName, snapshotName string) (*Snapshot, error) {
	return m.GetSnapshotInDir(volumeName, snapshotName, "")
}

// GetSnapshotInDir 获取指定目录下的快照详情
func (m *Manager) GetSnapshotInDir(volumeName, snapshotName, snapDir string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if snapDir == "" {
		snapDir = ".snapshots"
	}
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	snapPath := filepath.Join(snapDirPath, snapshotName)
	
	info, err := m.client.GetSubVolumeInfo(snapPath)
	if err != nil {
		return nil, fmt.Errorf("获取快照信息失败: %w", err)
	}
	
	stat, err := os.Stat(snapPath)
	var createdAt time.Time
	if err == nil {
		createdAt = stat.ModTime()
	}
	
	return &Snapshot{
		Name:      snapshotName,
		Path:      snapPath,
		ReadOnly:  info.ReadOnly,
		CreatedAt: createdAt,
	}, nil
}

// RestoreSnapshot 恢复快照（创建可写副本）
func (m *Manager) RestoreSnapshot(volumeName, snapshotName, targetName string) error {
	return m.RestoreSnapshotInDir(volumeName, snapshotName, targetName, "")
}

// RestoreSnapshotInDir 从指定目录恢复快照
func (m *Manager) RestoreSnapshotInDir(volumeName, snapshotName, targetName, snapDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if snapDir == "" {
		snapDir = ".snapshots"
	}
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	snapPath := filepath.Join(snapDirPath, snapshotName)
	targetPath := filepath.Join(vol.MountPoint, targetName)
	
	// 检查快照是否存在
	if _, err := os.Stat(snapPath); err != nil {
		return fmt.Errorf("快照 %s 不存在", snapshotName)
	}
	
	// 检查目标是否已存在
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("目标 %s 已存在", targetName)
	}
	
	// 恢复快照（创建可写快照）
	if err := m.client.RestoreSnapshot(snapPath, targetPath); err != nil {
		return err
	}
	
	// 刷新子卷列表
	m.scanSubvolumes(vol)
	
	return nil
}

// RollbackSnapshot 回滚到快照（替换当前子卷内容）
// 警告：此操作会删除当前子卷内容并用快照替换
func (m *Manager) RollbackSnapshot(volumeName, subvolName, snapshotName string) error {
	return m.RollbackSnapshotInDir(volumeName, subvolName, snapshotName, "")
}

// RollbackSnapshotInDir 从指定目录回滚快照
func (m *Manager) RollbackSnapshotInDir(volumeName, subvolName, snapshotName, snapDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if snapDir == "" {
		snapDir = ".snapshots"
	}
	
	// 查找子卷
	var subvol *SubVolume
	var subvolIndex int
	for i, sv := range vol.Subvolumes {
		if sv.Name == subvolName {
			subvol = sv
			subvolIndex = i
			break
		}
	}
	
	if subvol == nil {
		return fmt.Errorf("子卷 %s 不存在", subvolName)
	}
	
	snapDirPath := filepath.Join(vol.MountPoint, snapDir)
	snapPath := filepath.Join(snapDirPath, snapshotName)
	
	// 检查快照是否存在
	if _, err := os.Stat(snapPath); err != nil {
		return fmt.Errorf("快照 %s 不存在", snapshotName)
	}
	
	// 创建临时名称
	tempName := subvolName + ".rollback-temp"
	tempPath := filepath.Join(vol.MountPoint, tempName)
	
	// 从快照创建可写副本到临时位置
	if err := m.client.CreateSnapshot(snapPath, tempPath, false); err != nil {
		return fmt.Errorf("创建回滚快照失败: %w", err)
	}
	
	// 删除原始子卷
	if err := m.client.DeleteSubVolume(subvol.Path); err != nil {
		// 回滚失败，删除临时快照
		m.client.DeleteSubVolume(tempPath)
		return fmt.Errorf("删除原始子卷失败: %w", err)
	}
	
	// 重命名临时快照为原始子卷名
	if err := os.Rename(tempPath, subvol.Path); err != nil {
		return fmt.Errorf("重命名子卷失败: %w", err)
	}
	
	// 更新子卷信息
	newInfo, err := m.client.GetSubVolumeInfo(subvol.Path)
	if err == nil && newInfo != nil {
		vol.Subvolumes[subvolIndex].UUID = newInfo.UUID
		vol.Subvolumes[subvolIndex].ReadOnly = newInfo.ReadOnly
	}
	
	return nil
}

// ========== RAID 配置管理 ==========

// GetRAIDConfigs 获取所有 RAID 配置
func (m *Manager) GetRAIDConfigs() map[string]RAIDConfig {
	return RAIDConfigs
}

// GetRAIDConfig 获取指定 RAID 配置
func (m *Manager) GetRAIDConfig(profile string) *RAIDConfig {
	if config, ok := RAIDConfigs[profile]; ok {
		return &config
	}
	return nil
}

// ConvertRAID 转换卷的 RAID 配置
func (m *Manager) ConvertRAID(volumeName, newDataProfile, newMetaProfile string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	// 验证新配置
	if newDataProfile != "" {
		config, ok := RAIDConfigs[newDataProfile]
		if !ok {
			return fmt.Errorf("不支持的 RAID 配置: %s", newDataProfile)
		}
		if len(vol.Devices) < config.MinDevices {
			return fmt.Errorf("RAID %s 需要至少 %d 个设备，当前 %d 个", 
				newDataProfile, config.MinDevices, len(vol.Devices))
		}
	}
	
	if err := m.client.ConvertProfile(vol.MountPoint, newDataProfile, newMetaProfile); err != nil {
		return err
	}
	
	vol.DataProfile = newDataProfile
	if newMetaProfile != "" {
		vol.MetaProfile = newMetaProfile
	}
	
	return nil
}

// AddDevice 添加设备到卷（扩容）
func (m *Manager) AddDevice(volumeName, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	if err := m.client.AddDevice(vol.MountPoint, device); err != nil {
		return err
	}
	
	vol.Devices = append(vol.Devices, device)
	return nil
}

// RemoveDevice 从卷中移除设备
func (m *Manager) RemoveDevice(volumeName, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	if err := m.client.RemoveDevice(vol.MountPoint, device); err != nil {
		return err
	}
	
	// 从设备列表中移除
	for i, dev := range vol.Devices {
		if dev == device {
			vol.Devices = append(vol.Devices[:i], vol.Devices[i+1:]...)
			break
		}
	}
	
	return nil
}

// GetDeviceStats 获取设备统计
func (m *Manager) GetDeviceStats(volumeName string) ([]btrfs.DeviceStats, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	return m.client.GetDeviceStats(vol.MountPoint)
}

// ========== 维护操作 ==========

// Balance 启动数据平衡
func (m *Manager) Balance(volumeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	if vol.Status.BalanceRunning {
		return fmt.Errorf("平衡任务正在运行")
	}
	
	if err := m.client.StartBalance(vol.MountPoint); err != nil {
		return err
	}
	
	vol.Status.BalanceRunning = true
	vol.Status.BalanceProgress = 0
	return nil
}

// GetBalanceStatus 获取平衡状态
func (m *Manager) GetBalanceStatus(volumeName string) (*btrfs.BalanceStatus, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	status, err := m.client.GetBalanceStatus(vol.MountPoint)
	if err != nil {
		return nil, err
	}
	
	// 更新缓存状态
	m.mu.Lock()
	vol.Status.BalanceRunning = status.Running
	vol.Status.BalanceProgress = status.Progress
	m.mu.Unlock()
	
	return status, nil
}

// Scrub 启动数据校验
func (m *Manager) Scrub(volumeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	vol, exists := m.volumes[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}
	
	if vol.Status.ScrubRunning {
		return fmt.Errorf("校验任务正在运行")
	}
	
	if err := m.client.StartScrub(vol.MountPoint); err != nil {
		return err
	}
	
	vol.Status.ScrubRunning = true
	vol.Status.ScrubProgress = 0
	return nil
}

// GetScrubStatus 获取校验状态
func (m *Manager) GetScrubStatus(volumeName string) (*btrfs.ScrubStatus, error) {
	m.mu.RLock()
	vol, exists := m.volumes[volumeName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}
	
	if vol.MountPoint == "" {
		return nil, fmt.Errorf("卷未挂载")
	}
	
	status, err := m.client.GetScrubStatus(vol.MountPoint)
	if err != nil {
		return nil, err
	}
	
	// 更新缓存状态
	m.mu.Lock()
	vol.Status.ScrubRunning = status.Running
	vol.Status.ScrubProgress = status.Progress
	vol.Status.ScrubErrors = status.Errors
	if !status.Running && status.Errors == 0 {
		vol.Status.Healthy = true
	} else if status.Errors > 0 {
		vol.Status.Healthy = false
	}
	m.mu.Unlock()
	
	return status, nil
}

// Refresh 刷新卷信息
func (m *Manager) Refresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 清空现有数据
	m.volumes = make(map[string]*Volume)
	
	// 重新扫描
	return m.scanVolumes()
}