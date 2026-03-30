package lxc

import (
	"context"
	"encoding/json"
	"fmt"
)

// StorageManager handles LXC storage pool and volume operations.
type StorageManager struct {
	manager *Manager
}

// NewStorageManager creates a new StorageManager.
func NewStorageManager(manager *Manager) *StorageManager {
	return &StorageManager{manager: manager}
}

// ListPools lists all storage pools.
func (s *StorageManager) ListPools(ctx context.Context) ([]*StoragePool, error) {
	cmd := s.manager.cmd("storage", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list storage pools: %w", err)
	}

	var raw []struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Driver      string            `json:"driver"`
		UsedBy      []string          `json:"used_by"`
		Config      map[string]string `json:"config"`
		Status      string            `json:"status"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse storage pool list: %w", err)
	}

	var pools []*StoragePool
	for _, r := range raw {
		pool := &StoragePool{
			Name:        r.Name,
			Description: r.Description,
			Driver:      r.Driver,
			InUse:       len(r.UsedBy) > 0,
			Config:      r.Config,
		}

		// Parse size info from config
		if size, ok := r.Config["size"]; ok {
			pool.TotalSize = parseSize(size)
		}
		if used, ok := r.Config["used"]; ok {
			pool.UsedSize = parseSize(used)
		}
		if pool.TotalSize > pool.UsedSize {
			pool.Available = pool.TotalSize - pool.UsedSize
		}

		pools = append(pools, pool)
	}

	return pools, nil
}

// GetPool retrieves a specific storage pool.
func (s *StorageManager) GetPool(ctx context.Context, name string) (*StoragePool, error) {
	cmd := s.manager.cmd("storage", "show", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage pool %s: %w", name, err)
	}

	var raw struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Driver      string            `json:"driver"`
		UsedBy      []string          `json:"used_by"`
		Config      map[string]string `json:"config"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse storage pool info: %w", err)
	}

	pool := &StoragePool{
		Name:        raw.Name,
		Description: raw.Description,
		Driver:      raw.Driver,
		InUse:       len(raw.UsedBy) > 0,
		Config:      raw.Config,
	}

	if size, ok := raw.Config["size"]; ok {
		pool.TotalSize = parseSize(size)
	}
	if used, ok := raw.Config["used"]; ok {
		pool.UsedSize = parseSize(used)
	}
	if pool.TotalSize > pool.UsedSize {
		pool.Available = pool.TotalSize - pool.UsedSize
	}

	return pool, nil
}

// CreatePool creates a new storage pool.
func (s *StorageManager) CreatePool(ctx context.Context, config *StoragePoolCreateConfig) (*StoragePool, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	args := []string{"storage", "create", config.Name, config.Driver}

	// Add source
	if config.Source != "" {
		args = append(args, "--config", fmt.Sprintf("source=%s", config.Source))
	}

	// Add size
	if config.Size > 0 {
		args = append(args, "--config", fmt.Sprintf("size=%dGB", config.Size))
	}

	// Add additional config
	for k, v := range config.Config {
		args = append(args, "--config", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create storage pool: %w, output: %s", err, string(output))
	}

	return s.GetPool(ctx, config.Name)
}

// DeletePool deletes a storage pool.
func (s *StorageManager) DeletePool(ctx context.Context, name string, force bool) error {
	args := []string{"storage", "delete", name}
	if force {
		args = append(args, "--force")
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete storage pool: %w, output: %s", err, string(output))
	}
	return nil
}

// ListVolumes lists all volumes in a storage pool.
func (s *StorageManager) ListVolumes(ctx context.Context, poolName string) ([]*StorageVolume, error) {
	cmd := s.manager.cmd("storage", "volume", "list", poolName, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var raw []struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Type        string            `json:"type"`
		UsedBy      []string          `json:"used_by"`
		Config      map[string]string `json:"config"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse volume list: %w", err)
	}

	var volumes []*StorageVolume
	for _, r := range raw {
		vol := &StorageVolume{
			Name:        r.Name,
			Description: r.Description,
			Type:        r.Type,
			Pool:        poolName,
			InUse:       len(r.UsedBy) > 0,
			Config:      r.Config,
		}

		if size, ok := r.Config["size"]; ok {
			vol.Size = parseSize(size)
		}

		volumes = append(volumes, vol)
	}

	return volumes, nil
}

// GetVolume retrieves a specific volume.
func (s *StorageManager) GetVolume(ctx context.Context, poolName, volumeName, volumeType string) (*StorageVolume, error) {
	if volumeType == "" {
		volumeType = "custom"
	}
	_ = volumeType // volumeType reserved for future use with --type flag

	cmd := s.manager.cmd("storage", "volume", "show", poolName, volumeName, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume %s: %w", volumeName, err)
	}

	var raw struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Type        string            `json:"type"`
		Config      map[string]string `json:"config"`
		UsedBy      []string          `json:"used_by"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse volume info: %w", err)
	}

	vol := &StorageVolume{
		Name:        raw.Name,
		Description: raw.Description,
		Type:        raw.Type,
		Pool:        poolName,
		InUse:       len(raw.UsedBy) > 0,
		Config:      raw.Config,
	}

	if size, ok := raw.Config["size"]; ok {
		vol.Size = parseSize(size)
	}

	return vol, nil
}

// CreateVolume creates a new storage volume.
func (s *StorageManager) CreateVolume(ctx context.Context, config *StorageVolumeCreateConfig) (*StorageVolume, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	args := []string{"storage", "volume", "create", config.Pool, config.Name}

	// Add size
	if config.Size > 0 {
		args = append(args, "--config", fmt.Sprintf("size=%dGB", config.Size))
	}

	// Add additional config
	for k, v := range config.Config {
		args = append(args, "--config", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w, output: %s", err, string(output))
	}

	return s.GetVolume(ctx, config.Pool, config.Name, config.Type)
}

// DeleteVolume deletes a storage volume.
func (s *StorageManager) DeleteVolume(ctx context.Context, poolName, volumeName, volumeType string, force bool) error {
	if volumeType == "" {
		volumeType = "custom"
	}
	_ = volumeType // volumeType reserved for future use with --type flag

	args := []string{"storage", "volume", "delete", poolName, volumeName}
	if force {
		args = append(args, "--force")
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w, output: %s", err, string(output))
	}
	return nil
}

// AttachVolume attaches a volume to a container.
func (s *StorageManager) AttachVolume(ctx context.Context, poolName, volumeName, container, mountPath string, readOnly bool) error {
	args := []string{"storage", "volume", "attach", poolName, volumeName, container, mountPath}
	if readOnly {
		args = append(args, "--config", "readonly=true")
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach volume: %w, output: %s", err, string(output))
	}
	return nil
}

// DetachVolume detaches a volume from a container.
func (s *StorageManager) DetachVolume(ctx context.Context, poolName, volumeName, container string) error {
	args := []string{"storage", "volume", "detach", poolName, volumeName, container}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to detach volume: %w, output: %s", err, string(output))
	}
	return nil
}

// CopyVolume copies a volume to another location.
func (s *StorageManager) CopyVolume(ctx context.Context, srcPool, srcVolume, dstPool, dstVolume string) error {
	args := []string{"storage", "volume", "copy",
		fmt.Sprintf("%s/%s", srcPool, srcVolume),
		fmt.Sprintf("%s/%s", dstPool, dstVolume),
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy volume: %w, output: %s", err, string(output))
	}
	return nil
}

// MoveVolume moves a volume to another location.
func (s *StorageManager) MoveVolume(ctx context.Context, srcPool, srcVolume, dstPool, dstVolume string) error {
	args := []string{"storage", "volume", "move",
		fmt.Sprintf("%s/%s", srcPool, srcVolume),
		fmt.Sprintf("%s/%s", dstPool, dstVolume),
	}

	cmd := s.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move volume: %w, output: %s", err, string(output))
	}
	return nil
}

// StorageVolume represents an LXC storage volume.
type StorageVolume struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"` // container, image, custom
	Pool        string            `json:"pool"`
	Size        uint64            `json:"size"` // Size in MB
	InUse       bool              `json:"inUse"`
	Config      map[string]string `json:"config"`
}

// StoragePoolCreateConfig holds parameters for creating a storage pool.
type StoragePoolCreateConfig struct {
	Name        string            `json:"name"`
	Driver      string            `json:"driver"` // zfs, btrfs, dir, lvm, ceph
	Source      string            `json:"source"` // Source device/path
	Size        uint64            `json:"size"`   // Size in GB
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
}

// Validate validates StoragePoolCreateConfig.
func (c *StoragePoolCreateConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("pool name is required")
	}

	validDrivers := map[string]bool{
		"zfs":    true,
		"btrfs":  true,
		"dir":    true,
		"lvm":    true,
		"ceph":   true,
		"cephfs": true,
	}
	if !validDrivers[c.Driver] {
		return fmt.Errorf("invalid storage driver: %s", c.Driver)
	}

	return nil
}

// StorageVolumeCreateConfig holds parameters for creating a storage volume.
type StorageVolumeCreateConfig struct {
	Name        string            `json:"name"`
	Pool        string            `json:"pool"`
	Type        string            `json:"type"` // custom, container, image
	Size        uint64            `json:"size"` // Size in GB
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
}

// Validate validates StorageVolumeCreateConfig.
func (c *StorageVolumeCreateConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("volume name is required")
	}
	if c.Pool == "" {
		return fmt.Errorf("pool name is required")
	}
	// Note: Size is uint64 and cannot be negative
	return nil
}

// DefaultStoragePool returns a default storage pool configuration.
func DefaultStoragePool(name, driver string) *StoragePoolCreateConfig {
	return &StoragePoolCreateConfig{
		Name:   name,
		Driver: driver,
		Size:   100, // 100GB default
	}
}

// ZFSPoolConfig returns a ZFS-specific pool configuration.
func ZFSPoolConfig(name, source string, size uint64) *StoragePoolCreateConfig {
	return &StoragePoolCreateConfig{
		Name:   name,
		Driver: "zfs",
		Source: source,
		Size:   size,
		Config: map[string]string{
			"zfs.pool_name": name,
		},
	}
}

// DirectoryPoolConfig returns a directory-based pool configuration.
func DirectoryPoolConfig(name, path string) *StoragePoolCreateConfig {
	return &StoragePoolCreateConfig{
		Name:   name,
		Driver: "dir",
		Source: path,
	}
}
