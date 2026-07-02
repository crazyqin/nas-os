// Package lxc 沙箱存储管理模块
// 提供沙箱专用的存储池、存储卷和快照管理功能
package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SandboxStorageBackend 沙箱存储后端类型.
type SandboxStorageBackend string

const (
	SandboxBackendZFS   SandboxStorageBackend = "zfs"   // ZFS 文件系统
	SandboxBackendBtrfs SandboxStorageBackend = "btrfs" // Btrfs 文件系统
	SandboxBackendDir   SandboxStorageBackend = "dir"   // 普通目录
	SandboxBackendLVM   SandboxStorageBackend = "lvm"   // LVM 逻辑卷
)

// SandboxVolumeStatus 沙箱存储卷状态.
type SandboxVolumeStatus string

const (
	SandboxVolumeReady   SandboxVolumeStatus = "ready"
	SandboxVolumeInUse   SandboxVolumeStatus = "in_use"
	SandboxVolumeError   SandboxVolumeStatus = "error"
	SandboxVolumeDeleted SandboxVolumeStatus = "deleted"
)

// SandboxStoragePool 沙箱存储池.
type SandboxStoragePool struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`       // 存储池名称
	Backend   SandboxStorageBackend `json:"backend"`    // 存储后端
	Path      string                `json:"path"`       // 存储池根路径
	TotalSize int64                 `json:"total_size"` // 总容量（字节）
	UsedSize  int64                 `json:"used_size"`  // 已用空间（字节）
	Volumes   []string              `json:"volumes"`    // 包含的卷 ID 列表
	Labels    map[string]string     `json:"labels"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// SandboxStorageVolume 沙箱存储卷.
type SandboxStorageVolume struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`        // 卷名称
	PoolID     string              `json:"pool_id"`     // 所属存储池 ID
	Size       int64               `json:"size"`        // 卷大小（字节）
	UsedSize   int64               `json:"used_size"`   // 已用空间
	Path       string              `json:"path"`        // 卷在宿主机上的路径
	Status     SandboxVolumeStatus `json:"status"`      // 卷状态
	SandboxID  string              `json:"sandbox_id"`  // 关联的沙箱 ID（空表示未挂载）
	MountPoint string              `json:"mount_point"` // 挂载到沙箱的路径
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// SandboxSnapshot 沙箱存储快照.
type SandboxSnapshot struct {
	ID        string            `json:"id"`
	VolumeID  string            `json:"volume_id"` // 所属卷 ID
	Name      string            `json:"name"`      // 快照名称
	Size      int64             `json:"size"`      // 快照大小
	Path      string            `json:"path"`      // 快照路径
	ParentID  string            `json:"parent_id"` // 父快照 ID（用于增量快照）
	Labels    map[string]string `json:"labels"`
	CreatedAt time.Time         `json:"created_at"`
}

// SandboxStorageManagerConfig 沙箱存储管理器配置.
type SandboxStorageManagerConfig struct {
	DefaultBackend SandboxStorageBackend `json:"default_backend"` // 默认存储后端
	DefaultPool    string                `json:"default_pool"`    // 默认存储池名
	DataDir        string                `json:"data_dir"`        // 数据目录
	EnableQuota    bool                  `json:"enable_quota"`    // 是否启用磁盘配额
}

// SandboxStorageManager 沙箱存储管理器.
type SandboxStorageManager struct {
	mu        sync.RWMutex
	pools     map[string]*SandboxStoragePool
	volumes   map[string]*SandboxStorageVolume
	snapshots map[string]*SandboxSnapshot
	config    *SandboxStorageManagerConfig
	logger    *zap.Logger
	dataDir   string
}

// NewSandboxStorageManager 创建沙箱存储管理器.
func NewSandboxStorageManager(dataDir string, logger *zap.Logger) (*SandboxStorageManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if dataDir == "" {
		dataDir = "/var/lib/nas-os/lxc/storage"
	}

	sm := &SandboxStorageManager{
		pools:     make(map[string]*SandboxStoragePool),
		volumes:   make(map[string]*SandboxStorageVolume),
		snapshots: make(map[string]*SandboxSnapshot),
		config: &SandboxStorageManagerConfig{
			DefaultBackend: SandboxBackendDir,
			DefaultPool:    "default",
			DataDir:        dataDir,
			EnableQuota:    true,
		},
		logger:  logger,
		dataDir: dataDir,
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储数据目录失败: %w", err)
	}

	// 初始化默认存储池
	if err := sm.initDefaultPool(); err != nil {
		return nil, fmt.Errorf("初始化默认存储池失败: %w", err)
	}

	return sm, nil
}

// CreateVolume 创建沙箱存储卷.
func (sm *SandboxStorageManager) CreateVolume(ctx context.Context, name, poolID string, size int64) (*SandboxStorageVolume, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	pool, exists := sm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	// 检查空间是否充足
	available := pool.TotalSize - pool.UsedSize
	if available < size {
		return nil, fmt.Errorf("存储池空间不足: 需要 %d 字节，可用 %d 字节", size, available)
	}

	id := uuid.New().String()
	volumePath := filepath.Join(pool.Path, "volumes", id)

	// 创建卷目录（实际实现中根据后端类型执行不同操作）
	if err := os.MkdirAll(volumePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储卷目录失败: %w", err)
	}

	now := time.Now()
	volume := &SandboxStorageVolume{
		ID:        id,
		Name:      name,
		PoolID:    poolID,
		Size:      size,
		Path:      volumePath,
		Status:    SandboxVolumeReady,
		CreatedAt: now,
		UpdatedAt: now,
	}

	sm.volumes[id] = volume
	pool.Volumes = append(pool.Volumes, id)
	pool.UpdatedAt = now

	sm.logger.Info("沙箱存储卷创建成功",
		zap.String("id", id),
		zap.String("name", name),
		zap.Int64("size", size),
		zap.String("pool", poolID))

	return volume, nil
}

// MountVolume 挂载存储卷到沙箱.
func (sm *SandboxStorageManager) MountVolume(ctx context.Context, volumeID, sandboxID, mountPoint string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	volume, exists := sm.volumes[volumeID]
	if !exists {
		return fmt.Errorf("存储卷 %s 不存在", volumeID)
	}

	if volume.Status == SandboxVolumeDeleted {
		return fmt.Errorf("存储卷 %s 已删除", volumeID)
	}

	// 检查卷是否已被其他沙箱挂载
	if volume.SandboxID != "" && volume.SandboxID != sandboxID {
		return fmt.Errorf("存储卷 %s 已挂载到沙箱 %s", volumeID, volume.SandboxID)
	}

	// 实际挂载操作（通过 LXC 配置或 mount namespace）
	// lxc.mount.entry = /path/to/volume /mount/point none bind,create=dir 0 0

	volume.SandboxID = sandboxID
	volume.MountPoint = mountPoint
	volume.Status = SandboxVolumeInUse
	volume.UpdatedAt = time.Now()

	sm.logger.Info("存储卷挂载成功",
		zap.String("volume_id", volumeID),
		zap.String("sandbox_id", sandboxID),
		zap.String("mount_point", mountPoint))

	return nil
}

// UnmountVolume 从沙箱卸载存储卷.
func (sm *SandboxStorageManager) UnmountVolume(ctx context.Context, volumeID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	volume, exists := sm.volumes[volumeID]
	if !exists {
		return fmt.Errorf("存储卷 %s 不存在", volumeID)
	}

	if volume.SandboxID == "" {
		return fmt.Errorf("存储卷 %s 未挂载", volumeID)
	}

	volume.SandboxID = ""
	volume.MountPoint = ""
	volume.Status = SandboxVolumeReady
	volume.UpdatedAt = time.Now()

	sm.logger.Info("存储卷已卸载", zap.String("volume_id", volumeID))
	return nil
}

// DeleteVolume 删除存储卷.
func (sm *SandboxStorageManager) DeleteVolume(ctx context.Context, volumeID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	volume, exists := sm.volumes[volumeID]
	if !exists {
		return fmt.Errorf("存储卷 %s 不存在", volumeID)
	}

	if volume.Status == SandboxVolumeInUse {
		return fmt.Errorf("存储卷 %s 正在使用中，请先卸载", volumeID)
	}

	// 清理相关快照
	for snapID, snap := range sm.snapshots {
		if snap.VolumeID == volumeID {
			delete(sm.snapshots, snapID)
		}
	}

	// 清理文件系统
	if err := os.RemoveAll(volume.Path); err != nil {
		sm.logger.Warn("清理存储卷目录失败", zap.String("path", volume.Path), zap.Error(err))
	}

	volume.Status = SandboxVolumeDeleted
	sm.logger.Info("存储卷已删除", zap.String("volume_id", volumeID), zap.String("name", volume.Name))
	return nil
}

// Snapshot 创建沙箱存储快照.
func (sm *SandboxStorageManager) Snapshot(ctx context.Context, volumeID, name string, labels map[string]string) (*SandboxSnapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	volume, exists := sm.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("存储卷 %s 不存在", volumeID)
	}

	if volume.Status == SandboxVolumeDeleted {
		return nil, fmt.Errorf("存储卷 %s 已删除", volumeID)
	}

	id := uuid.New().String()
	snapPath := filepath.Join(sm.dataDir, "snapshots", id)

	// 创建快照目录
	if err := os.MkdirAll(snapPath, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 实际实现中根据后端执行快照：
	// ZFS:   zfs snapshot pool/volume@snapshot_name
	// Btrfs: btrfs subvolume snapshot -r /volume /snapshot
	// Dir:   rsync -a /volume/ /snapshot/ 或 cp --reflink=always

	// 查找最近的父快照（用于增量链）
	var parentID string
	for _, snap := range sm.snapshots {
		if snap.VolumeID == volumeID {
			parentID = snap.ID
		}
	}

	now := time.Now()
	snapshot := &SandboxSnapshot{
		ID:        id,
		VolumeID:  volumeID,
		Name:      name,
		Path:      snapPath,
		ParentID:  parentID,
		Labels:    labels,
		CreatedAt: now,
	}

	sm.snapshots[id] = snapshot

	sm.logger.Info("沙箱快照创建成功",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("volume_id", volumeID),
		zap.String("parent_id", parentID))

	return snapshot, nil
}

// ListVolumes 列出存储池中的所有卷.
func (sm *SandboxStorageManager) ListVolumes(poolID string) []*SandboxStorageVolume {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SandboxStorageVolume, 0)
	for _, vol := range sm.volumes {
		if poolID == "" || vol.PoolID == poolID {
			result = append(result, vol)
		}
	}
	return result
}

// ListSnapshots 列出卷的所有快照.
func (sm *SandboxStorageManager) ListSnapshots(volumeID string) []*SandboxSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SandboxSnapshot, 0)
	for _, snap := range sm.snapshots {
		if volumeID == "" || snap.VolumeID == volumeID {
			result = append(result, snap)
		}
	}
	return result
}

// GetPool 获取存储池信息.
func (sm *SandboxStorageManager) GetPool(poolID string) (*SandboxStoragePool, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	pool, exists := sm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}
	return pool, nil
}

// CreatePool 创建存储池.
func (sm *SandboxStorageManager) CreatePool(ctx context.Context, name string, backend SandboxStorageBackend, path string, totalSize int64) (*SandboxStoragePool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := uuid.New().String()
	poolPath := filepath.Join(sm.dataDir, "pools", id)

	if path != "" {
		poolPath = path
	}

	if err := os.MkdirAll(poolPath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储池目录失败: %w", err)
	}

	now := time.Now()
	pool := &SandboxStoragePool{
		ID:        id,
		Name:      name,
		Backend:   backend,
		Path:      poolPath,
		TotalSize: totalSize,
		UsedSize:  0,
		Volumes:   []string{},
		Labels:    make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}

	sm.pools[id] = pool

	sm.logger.Info("沙箱存储池创建成功",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("backend", string(backend)))

	return pool, nil
}

// initDefaultPool 初始化默认存储池.
func (sm *SandboxStorageManager) initDefaultPool() error {
	defaultPath := filepath.Join(sm.dataDir, "pools", "default")
	if err := os.MkdirAll(defaultPath, 0755); err != nil {
		return err
	}

	if _, exists := sm.pools["default"]; !exists {
		now := time.Now()
		sm.pools["default"] = &SandboxStoragePool{
			ID:        "default",
			Name:      "default",
			Backend:   SandboxBackendDir,
			Path:      defaultPath,
			TotalSize: 100 * 1024 * 1024 * 1024, // 默认 100GB（实际会检测磁盘）
			Volumes:   []string{},
			Labels:    make(map[string]string),
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	return nil
}

// Close 关闭存储管理器.
func (sm *SandboxStorageManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	statePath := filepath.Join(sm.dataDir, "state.json")
	data, err := json.MarshalIndent(struct {
		Pools     map[string]*SandboxStoragePool   `json:"pools"`
		Volumes   map[string]*SandboxStorageVolume `json:"volumes"`
		Snapshots map[string]*SandboxSnapshot      `json:"snapshots"`
	}{
		Pools:     sm.pools,
		Volumes:   sm.volumes,
		Snapshots: sm.snapshots,
	}, "", "  ")

	if err != nil {
		sm.logger.Error("序列化存储状态失败", zap.Error(err))
		return err
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		sm.logger.Error("保存存储状态失败", zap.Error(err))
		return err
	}

	sm.logger.Info("沙箱存储管理器已关闭")
	return nil
}
