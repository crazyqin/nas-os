package lxcorchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// VolumeManager 存储卷管理器
type VolumeManager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	orchestrator *Orchestrator
	volumes      map[string]*VolumeConfig
	mounts       map[string][]*VolumeMount // container_id -> mounts
}

// VolumeStats 存储卷统计信息
type VolumeStats struct {
	VolumeID    string `json:"volume_id"`
	Name        string `json:"name"`
	Driver      string `json:"driver"`
	Size        int64  `json:"size"`
	Used        int64  `json:"used"`
	Available   int64  `json:"available"`
	Containers  int    `json:"containers"`
	MountPoint  string `json:"mount_point"`
}

// NewVolumeManager 创建存储卷管理器
func NewVolumeManager(logger *zap.Logger, orchestrator *Orchestrator) *VolumeManager {
	return &VolumeManager{
		logger:       logger,
		orchestrator: orchestrator,
		volumes:      make(map[string]*VolumeConfig),
		mounts:       make(map[string][]*VolumeMount),
	}
}

// InitDefaultVolume 初始化默认存储卷
func (vm *VolumeManager) InitDefaultVolume(ctx context.Context) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 检查是否已存在默认卷
	if _, exists := vm.volumes["lxc-data"]; exists {
		return nil
	}

	// 创建默认存储卷
	defaultVolume := &VolumeConfig{
		ID:         "lxc-data",
		Name:       "lxc-data",
		Driver:     "local",
		MountPoint: "/var/lib/lxc/volumes/lxc-data",
		Size:       100 * 1024 * 1024 * 1024, // 100GB
		Labels: map[string]string{
			"managed-by": "lxcorchestrator",
			"default":    "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := vm.createVolume(ctx, defaultVolume); err != nil {
		return fmt.Errorf("failed to create default volume: %w", err)
	}

	vm.volumes["lxc-data"] = defaultVolume

	vm.logger.Info("default volume initialized",
		zap.String("id", defaultVolume.ID),
		zap.String("mount_point", defaultVolume.MountPoint),
	)

	return nil
}

// CreateVolume 创建存储卷
func (vm *VolumeManager) CreateVolume(ctx context.Context, config *VolumeConfig) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// 检查名称重复
	for _, v := range vm.volumes {
		if v.Name == config.Name {
			return fmt.Errorf("volume name already exists: %s", config.Name)
		}
	}

	// 设置默认值
	if config.Driver == "" {
		config.Driver = "local"
	}
	if config.MountPoint == "" {
		config.MountPoint = fmt.Sprintf("/var/lib/lxc/volumes/%s", config.ID)
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := vm.createVolume(ctx, config); err != nil {
		return err
	}

	vm.volumes[config.ID] = config

	vm.logger.Info("volume created",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("driver", config.Driver),
	)

	return nil
}

// DeleteVolume 删除存储卷
func (vm *VolumeManager) DeleteVolume(ctx context.Context, volumeID string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	volume, exists := vm.volumes[volumeID]
	if !exists {
		return fmt.Errorf("volume not found: %s", volumeID)
	}

	// 检查是否有容器使用此卷
	if len(volume.Containers) > 0 {
		return fmt.Errorf("volume in use by containers: %v", volume.Containers)
	}

	if err := vm.deleteVolume(ctx, volume); err != nil {
		return err
	}

	delete(vm.volumes, volumeID)

	vm.logger.Info("volume deleted", zap.String("id", volumeID))
	return nil
}

// GetVolume 获取存储卷配置
func (vm *VolumeManager) GetVolume(volumeID string) (*VolumeConfig, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	volume, exists := vm.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	return volume, nil
}

// ListVolumes 列出存储卷
func (vm *VolumeManager) ListVolumes() []*VolumeConfig {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	volumes := make([]*VolumeConfig, 0, len(vm.volumes))
	for _, v := range vm.volumes {
		volumes = append(volumes, v)
	}

	return volumes
}

// MountVolumes 挂载存储卷到容器
func (vm *VolumeManager) MountVolumes(ctx context.Context, container *ContainerInstance) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, mount := range container.Config.Volumes {
		volume, exists := vm.volumes[mount.VolumeName]
		if !exists {
			// 尝试查找同名卷
			found := false
			for _, v := range vm.volumes {
				if v.Name == mount.VolumeName {
					volume = v
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("volume not found: %s", mount.VolumeName)
			}
		}

		// 挂载卷
		if err := vm.mountVolume(ctx, container, volume, mount); err != nil {
			return fmt.Errorf("failed to mount volume %s: %w", mount.VolumeName, err)
		}

		// 记录容器使用此卷
		volume.Containers = append(volume.Containers, container.Config.ID)
		vm.mounts[container.Config.ID] = append(vm.mounts[container.Config.ID], &mount)

		vm.logger.Info("volume mounted",
			zap.String("container_id", container.Config.ID),
			zap.String("volume_id", volume.ID),
			zap.String("mount_path", mount.MountPath),
		)
	}

	return nil
}

// UnmountVolumes 从容器卸载存储卷
func (vm *VolumeManager) UnmountVolumes(ctx context.Context, container *ContainerInstance) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	mounts, exists := vm.mounts[container.Config.ID]
	if !exists {
		return
	}

	for _, mount := range mounts {
		// 查找卷
		for _, volume := range vm.volumes {
			if volume.Name == mount.VolumeName || volume.ID == mount.VolumeName {
				// 卸载卷
				if err := vm.unmountVolume(ctx, container, volume, mount); err != nil {
					vm.logger.Error("failed to unmount volume",
						zap.String("container_id", container.Config.ID),
						zap.String("volume_id", volume.ID),
						zap.Error(err),
					)
				}

				// 从卷的容器列表中移除
				for i, id := range volume.Containers {
					if id == container.Config.ID {
						volume.Containers = append(volume.Containers[:i], volume.Containers[i+1:]...)
						break
					}
				}
				break
			}
		}
	}

	delete(vm.mounts, container.Config.ID)

	vm.logger.Info("volumes unmounted", zap.String("container_id", container.Config.ID))
}

// GetContainerMounts 获取容器的挂载列表
func (vm *VolumeManager) GetContainerMounts(containerID string) []*VolumeMount {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.mounts[containerID]
}

// GetVolumeContainers 获取使用指定卷的容器列表
func (vm *VolumeManager) GetVolumeContainers(volumeID string) ([]string, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	volume, exists := vm.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	return volume.Containers, nil
}

// GetVolumeStats 获取存储卷统计信息
func (vm *VolumeManager) GetVolumeStats(volumeID string) (*VolumeStats, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	volume, exists := vm.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	// 模拟统计信息
	stats := &VolumeStats{
		VolumeID:   volume.ID,
		Name:       volume.Name,
		Driver:     volume.Driver,
		Size:       volume.Size,
		Used:       volume.Size / 4, // 模拟使用 25%
		Available:  volume.Size * 3 / 4,
		Containers: len(volume.Containers),
		MountPoint: volume.MountPoint,
	}

	return stats, nil
}

// ResizeVolume 调整存储卷大小
func (vm *VolumeManager) ResizeVolume(ctx context.Context, volumeID string, newSize int64) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	volume, exists := vm.volumes[volumeID]
	if !exists {
		return fmt.Errorf("volume not found: %s", volumeID)
	}

	if newSize <= 0 {
		return fmt.Errorf("invalid size: %d", newSize)
	}

	if newSize < volume.Size {
		return fmt.Errorf("cannot shrink volume: current %d, new %d", volume.Size, newSize)
	}

	// 调整大小
	if err := vm.resizeVolume(ctx, volume, newSize); err != nil {
		return err
	}

	oldSize := volume.Size
	volume.Size = newSize
	volume.UpdatedAt = time.Now()

	vm.logger.Info("volume resized",
		zap.String("id", volumeID),
		zap.Int64("old_size_gb", oldSize/(1024*1024*1024)),
		zap.Int64("new_size_gb", newSize/(1024*1024*1024)),
	)

	return nil
}

// CloneVolume 克隆存储卷
func (vm *VolumeManager) CloneVolume(ctx context.Context, sourceID, newName string) (*VolumeConfig, error) {
	vm.mu.RLock()
	source, exists := vm.volumes[sourceID]
	vm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("source volume not found: %s", sourceID)
	}

	// 创建新卷配置
	newVolume := &VolumeConfig{
		ID:         uuid.New().String(),
		Name:       newName,
		Driver:     source.Driver,
		Size:       source.Size,
		Options:    source.Options,
		Labels:     source.Labels,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 复制标签
	if source.Labels != nil {
		newVolume.Labels = make(map[string]string)
		for k, v := range source.Labels {
			newVolume.Labels[k] = v
		}
		newVolume.Labels["cloned-from"] = sourceID
	}

	if err := vm.CreateVolume(ctx, newVolume); err != nil {
		return nil, err
	}

	vm.logger.Info("volume cloned",
		zap.String("source_id", sourceID),
		zap.String("new_id", newVolume.ID),
		zap.String("new_name", newName),
	)

	return newVolume, nil
}

// createVolume 创建存储卷 (系统调用)
func (vm *VolumeManager) createVolume(ctx context.Context, config *VolumeConfig) error {
	// 模拟创建存储卷
	// 实际实现需要根据 driver 调用不同的存储后端
	vm.logger.Debug("creating volume",
		zap.String("id", config.ID),
		zap.String("driver", config.Driver),
		zap.String("mount_point", config.MountPoint),
	)

	return nil
}

// deleteVolume 删除存储卷 (系统调用)
func (vm *VolumeManager) deleteVolume(ctx context.Context, config *VolumeConfig) error {
	// 模拟删除存储卷
	vm.logger.Debug("deleting volume", zap.String("id", config.ID))
	return nil
}

// mountVolume 挂载存储卷 (系统调用)
func (vm *VolumeManager) mountVolume(ctx context.Context, container *ContainerInstance, volume *VolumeConfig, mount VolumeMount) error {
	// 模拟挂载操作
	// 实际实现需要调用 mount --bind 或类似命令
	vm.logger.Debug("mounting volume",
		zap.String("container_id", container.Config.ID),
		zap.String("volume_id", volume.ID),
		zap.String("mount_path", mount.MountPath),
		zap.String("source", volume.MountPoint),
	)

	return nil
}

// unmountVolume 卸载存储卷 (系统调用)
func (vm *VolumeManager) unmountVolume(ctx context.Context, container *ContainerInstance, volume *VolumeConfig, mount *VolumeMount) error {
	// 模拟卸载操作
	vm.logger.Debug("unmounting volume",
		zap.String("container_id", container.Config.ID),
		zap.String("volume_id", volume.ID),
		zap.String("mount_path", mount.MountPath),
	)

	return nil
}

// resizeVolume 调整存储卷大小 (系统调用)
func (vm *VolumeManager) resizeVolume(ctx context.Context, volume *VolumeConfig, newSize int64) error {
	// 模拟调整大小
	// 实际实现需要根据 driver 调用不同的调整命令
	vm.logger.Debug("resizing volume",
		zap.String("id", volume.ID),
		zap.Int64("new_size", newSize),
	)

	return nil
}
