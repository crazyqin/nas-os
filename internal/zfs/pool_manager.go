package zfs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// PoolManager ZFS池管理器 - 完整的ZFS池生命周期管理.
type PoolManager struct {
	logger *zap.Logger
}

// NewPoolManager 创建ZFS池管理器.
func NewPoolManager(logger *zap.Logger) *PoolManager {
	return &PoolManager{logger: logger}
}

// PoolInfo ZFS池信息.
type PoolInfo struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"` // ONLINE, DEGRADED, FAULTED, OFFLINE
	Size      uint64            `json:"size"`
	Used      uint64            `json:"used"`
	Free      uint64            `json:"free"`
	Allocated uint64            `json:"allocated"`
	Frag      int               `json:"frag"`
	Health    string            `json:"health"`
	Dedup     string            `json:"dedup"`
	Compress  string            `json:"compress"`
	VDevs     []VDevInfo        `json:"vdevs"`
	Props     map[string]string `json:"props"`
	ScanInfo  *ScanInfo         `json:"scan,omitempty"`
}

// VDevInfo 虚拟设备信息.
type VDevInfo struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"` // mirror, raidz1, raidz2, raidz3, disk
	Status   string     `json:"status"`
	Children []VDevInfo `json:"children,omitempty"`
	Devices  []string   `json:"devices,omitempty"`
}

// ScanInfo 扫描信息.
type ScanInfo struct {
	Function  string    `json:"function"`
	State     string    `json:"state"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Errors    int       `json:"errors"`
}

// DatasetInfo 数据集信息.
type DatasetInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // filesystem, volume, snapshot
	Mountpoint  string            `json:"mountpoint"`
	Used        uint64            `json:"used"`
	Available   uint64            `json:"available"`
	Referenced  uint64            `json:"referenced"`
	Compression string            `json:"compression"`
	Dedup       string            `json:"dedup"`
	Quota       string            `json:"quota"`
	Reservation string            `json:"reservation"`
	Props       map[string]string `json:"props"`
}

// ZVOLInfo ZVOL信息.
type ZVOLInfo struct {
	Name       string `json:"name"`
	VolSize    uint64 `json:"volsize"`
	VolMode    string `json:"volmode"` // full, geom, dev, none
	BlockSize  string `json:"blocksize"`
	Sync       string `json:"sync"`
	Used       uint64 `json:"used"`
	Referenced uint64 `json:"referenced"`
}

// ListPools 列出所有ZFS池.
func (pm *PoolManager) ListPools(ctx context.Context) ([]PoolInfo, error) {
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o",
		"name,size,alloc,free,frag,cap,dedup,health,altroot")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool list failed: %w", err)
	}

	var pools []PoolInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pool := PoolInfo{
			Name:   fields[0],
			Health: fields[7],
		}
		pool.Props = map[string]string{"altroot": fields[8]}
		pools = append(pools, pool)
	}

	// Enrich with detailed info
	for i := range pools {
		pm.enrichPoolInfo(ctx, &pools[i])
	}

	return pools, nil
}

// GetPool 获取池详细信息.
func (pm *PoolManager) GetPool(ctx context.Context, name string) (*PoolInfo, error) {
	pool := &PoolInfo{Name: name}
	pm.enrichPoolInfo(ctx, pool)
	if pool.Health == "" {
		return nil, fmt.Errorf("pool %s not found", name)
	}
	return pool, nil
}

func (pm *PoolManager) enrichPoolInfo(ctx context.Context, pool *PoolInfo) {
	cmd := exec.CommandContext(ctx, "zpool", "get", "-H", "-p",
		"size,allocated,free,fragmentation,dedup,compressratio,health",
		pool.Name)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	pool.Props = make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 {
			pool.Props[f[1]] = f[2]
			switch f[1] {
			case "health":
				pool.Health = f[2]
			case "dedup":
				pool.Dedup = f[2]
			}
		}
	}
}

// CreatePool 创建ZFS池.
func (pm *PoolManager) CreatePool(ctx context.Context, name, vdevType string, devices []string, opts map[string]string) error {
	args := []string{"create"}

	// 添加池级属性
	if ashift, ok := opts["ashift"]; ok {
		args = append(args, "-o", "ashift="+ashift)
	}
	if altroot, ok := opts["altroot"]; ok {
		args = append(args, "-R", altroot)
	}

	// 自动trim
	if opts["autotrim"] == "on" {
		args = append(args, "-o", "autotrim=on")
	}

	args = append(args, name)

	// 构建vdev
	switch vdevType {
	case "mirror":
		args = append(args, "mirror")
		args = append(args, devices...)
	case "raidz1":
		args = append(args, "raidz1")
		args = append(args, devices...)
	case "raidz2":
		args = append(args, "raidz2")
		args = append(args, devices...)
	case "raidz3":
		args = append(args, "raidz3")
		args = append(args, devices...)
	case "stripe":
		args = append(args, devices...)
	default:
		args = append(args, devices...)
	}

	pm.logger.Info("Creating ZFS pool", zap.String("name", name), zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool create failed: %s: %w", string(output), err)
	}
	return nil
}

// DestroyPool 销毁ZFS池.
func (pm *PoolManager) DestroyPool(ctx context.Context, name string, force bool) error {
	args := []string{"destroy"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool destroy failed: %s: %w", string(output), err)
	}
	return nil
}

// ExportPool 导出ZFS池.
func (pm *PoolManager) ExportPool(ctx context.Context, name string, force bool) error {
	args := []string{"export"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool export failed: %s: %w", string(output), err)
	}
	return nil
}

// ImportPool 导入ZFS池.
func (pm *PoolManager) ImportPool(ctx context.Context, name string, altroot string) error {
	args := []string{"import"}
	if altroot != "" {
		args = append(args, "-R", altroot)
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool import failed: %s: %w", string(output), err)
	}
	return nil
}

// ScanPool 扫描池( scrub/resilver ).
func (pm *PoolManager) ScanPool(ctx context.Context, name, scanType string) error {
	args := []string{"scan"}
	switch scanType {
	case "scrub":
		args = append(args, "-s") // stop current
		args = []string{"scrub", name}
	case "resilver":
		args = []string{"resilver", name}
	default:
		args = append(args, name)
	}

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool scan failed: %s: %w", string(output), err)
	}
	return nil
}

// GetPoolStatus 获取池状态详情( zpool status ).
func (pm *PoolManager) GetPoolStatus(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("zpool status failed: %w", err)
	}
	return string(out), nil
}

// SetPoolProperty 设置池属性.
func (pm *PoolManager) SetPoolProperty(ctx context.Context, name, prop, value string) error {
	cmd := exec.CommandContext(ctx, "zpool", "set", prop+"="+value, name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool set failed: %s: %w", string(output), err)
	}
	return nil
}

// AddVDev 向池添加vdev.
func (pm *PoolManager) AddVDev(ctx context.Context, poolName, vdevType string, devices []string) error {
	args := []string{"add"}
	if vdevType != "" {
		args = append(args, vdevType)
	}
	args = append(args, poolName)
	args = append(args, devices...)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool add failed: %s: %w", string(output), err)
	}
	return nil
}

// AttachDevice 附加设备(镜像).
func (pm *PoolManager) AttachDevice(ctx context.Context, poolName, existingDev, newDev string) error {
	cmd := exec.CommandContext(ctx, "zpool", "attach", poolName, existingDev, newDev)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool attach failed: %s: %w", string(output), err)
	}
	return nil
}

// DetachDevice 分离设备.
func (pm *PoolManager) DetachDevice(ctx context.Context, poolName, device string) error {
	cmd := exec.CommandContext(ctx, "zpool", "detach", poolName, device)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool detach failed: %s: %w", string(output), err)
	}
	return nil
}

// ReplaceDevice 替换设备.
func (pm *PoolManager) ReplaceDevice(ctx context.Context, poolName, oldDev, newDev string) error {
	cmd := exec.CommandContext(ctx, "zpool", "replace", poolName, oldDev, newDev)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool replace failed: %s: %w", string(output), err)
	}
	return nil
}

// OfflineDevice 离线设备.
func (pm *PoolManager) OfflineDevice(ctx context.Context, poolName, device string, force bool) error {
	args := []string{"offline"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, poolName, device)

	cmd := exec.CommandContext(ctx, "zpool", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool offline failed: %s: %w", string(output), err)
	}
	return nil
}

// OnlineDevice 上线设备.
func (pm *PoolManager) OnlineDevice(ctx context.Context, poolName, device string) error {
	cmd := exec.CommandContext(ctx, "zpool", "online", poolName, device)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool online failed: %s: %w", string(output), err)
	}
	return nil
}

// ClearPool 清除池错误状态.
func (pm *PoolManager) ClearPool(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "zpool", "clear", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool clear failed: %s: %w", string(output), err)
	}
	return nil
}

// DatasetManager 数据集管理器.
type DatasetManager struct {
	logger *zap.Logger
}

// NewDatasetManager 创建数据集管理器.
func NewDatasetManager(logger *zap.Logger) *DatasetManager {
	return &DatasetManager{logger: logger}
}

// CreateDataset 创建数据集(文件系统).
func (dm *DatasetManager) CreateDataset(ctx context.Context, name string, props map[string]string) error {
	args := []string{"create"}
	for k, v := range props {
		args = append(args, "-o", k+"="+v)
	}
	args = append(args, name)

	dm.logger.Info("Creating ZFS dataset", zap.String("name", name))
	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs create failed: %s: %w", string(output), err)
	}
	return nil
}

// DestroyDataset 销毁数据集.
func (dm *DatasetManager) DestroyDataset(ctx context.Context, name string, recursive bool) error {
	args := []string{"destroy"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy failed: %s: %w", string(output), err)
	}
	return nil
}

// RenameDataset 重命名数据集.
func (dm *DatasetManager) RenameDataset(ctx context.Context, oldName, newName string) error {
	cmd := exec.CommandContext(ctx, "zfs", "rename", oldName, newName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs rename failed: %s: %w", string(output), err)
	}
	return nil
}

// MountDataset 挂载数据集.
func (dm *DatasetManager) MountDataset(ctx context.Context, name, mountpoint string) error {
	args := []string{"mount"}
	if mountpoint != "" {
		args = append(args, "-o", "mountpoint="+mountpoint)
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs mount failed: %s: %w", string(output), err)
	}
	return nil
}

// UnmountDataset 卸载数据集.
func (dm *DatasetManager) UnmountDataset(ctx context.Context, name string, force bool) error {
	args := []string{"unmount"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs unmount failed: %s: %w", string(output), err)
	}
	return nil
}

// SetDatasetProperty 设置数据集属性.
func (dm *DatasetManager) SetDatasetProperty(ctx context.Context, name, prop, value string) error {
	cmd := exec.CommandContext(ctx, "zfs", "set", prop+"="+value, name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs set failed: %s: %w", string(output), err)
	}
	return nil
}

// GetDatasetProperty 获取数据集属性.
func (dm *DatasetManager) GetDatasetProperty(ctx context.Context, name, prop string) (string, error) {
	cmd := exec.CommandContext(ctx, "zfs", "get", "-H", "-o", "value", prop, name)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("zfs get failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ListDatasets 列出数据集.
func (dm *DatasetManager) ListDatasets(ctx context.Context, parent string, depth int) ([]DatasetInfo, error) {
	args := []string{"list", "-H", "-o",
		"name,type,mountpoint,used,available,referenced,compression,dedup,quota,reservation"}
	if depth > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", depth))
	}
	if parent != "" {
		args = append(args, "-r", parent)
	}

	cmd := exec.CommandContext(ctx, "zfs", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list failed: %w", err)
	}

	var datasets []DatasetInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		ds := DatasetInfo{
			Name:        f[0],
			Type:        f[1],
			Mountpoint:  f[2],
			Compression: f[6],
			Dedup:       f[7],
			Quota:       f[8],
			Reservation: f[9],
		}
		datasets = append(datasets, ds)
	}
	return datasets, nil
}

// SnapshotManager ZFS快照管理器.
type ZFSSnapshotManager struct {
	logger *zap.Logger
}

// NewZFSSnapshotManager 创建ZFS快照管理器.
func NewZFSSnapshotManager(logger *zap.Logger) *ZFSSnapshotManager {
	return &ZFSSnapshotManager{logger: logger}
}

// CreateSnapshot 创建快照.
func (sm *ZFSSnapshotManager) CreateSnapshot(ctx context.Context, dataset, snapName string, recursive bool) error {
	fullName := dataset + "@" + snapName
	args := []string{"snapshot"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, fullName)

	sm.logger.Info("Creating ZFS snapshot", zap.String("snapshot", fullName))
	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot failed: %s: %w", string(output), err)
	}
	return nil
}

// DestroySnapshot 销毁快照.
func (sm *ZFSSnapshotManager) DestroySnapshot(ctx context.Context, dataset, snapName string, recursive bool) error {
	fullName := dataset + "@" + snapName
	args := []string{"destroy"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy snapshot failed: %s: %w", string(output), err)
	}
	return nil
}

// RollbackSnapshot 回滚到快照.
func (sm *ZFSSnapshotManager) RollbackSnapshot(ctx context.Context, dataset, snapName string, force bool) error {
	fullName := dataset + "@" + snapName
	args := []string{"rollback"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs rollback failed: %s: %w", string(output), err)
	}
	return nil
}

// ListSnapshots 列出快照.
func (sm *ZFSSnapshotManager) ListSnapshots(ctx context.Context, dataset string) ([]string, error) {
	args := []string{"list", "-H", "-o", "name", "-t", "snapshot"}
	if dataset != "" {
		args = append(args, "-r", dataset)
	}

	cmd := exec.CommandContext(ctx, "zfs", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list snapshots failed: %w", err)
	}

	var snaps []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			snaps = append(snaps, line)
		}
	}
	return snaps, nil
}

// SendReceiveManager ZFS send/receive管理器 - 块级增量备份.
type SendReceiveManager struct {
	logger *zap.Logger
}

// NewSendReceiveManager 创建send/receive管理器.
func NewSendReceiveManager(logger *zap.Logger) *SendReceiveManager {
	return &SendReceiveManager{logger: logger}
}

// SendSnapshot 发送快照到文件或管道.
func (srm *SendReceiveManager) SendSnapshot(ctx context.Context, snapshot string, incrementalBase string, output string) error {
	args := []string{"send"}
	if incrementalBase != "" {
		args = append(args, "-i", incrementalBase)
	}
	args = append(args, snapshot)

	srm.logger.Info("ZFS send", zap.String("snapshot", snapshot), zap.String("output", output))

	if output != "" {
		// Use shell for redirection
		shellCmd := fmt.Sprintf("zfs %s > %s", strings.Join(args, " "), output)
		cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("zfs send failed: %s: %w", string(out), err)
		}
	} else {
		cmd := exec.CommandContext(ctx, "zfs", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("zfs send failed: %s: %w", string(out), err)
		}
	}
	return nil
}

// ReceiveSnapshot 接收快照.
func (srm *SendReceiveManager) ReceiveSnapshot(ctx context.Context, dataset string, input string) error {
	shellCmd := fmt.Sprintf("cat %s | zfs receive %s", input, dataset)
	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs receive failed: %s: %w", string(out), err)
	}
	return nil
}

// SendToRemote 通过SSH发送到远程.
func (srm *SendReceiveManager) SendToRemote(ctx context.Context, snapshot, incrementalBase, remoteHost, remoteDataset string) error {
	args := []string{"send"}
	if incrementalBase != "" {
		args = append(args, "-i", incrementalBase)
	}
	args = append(args, snapshot)

	// zfs send snapshot | ssh remoteHost zfs receive remoteDataset
	sendCmd := fmt.Sprintf("zfs send %s", strings.Join(args[1:], " "))
	sshCmd := fmt.Sprintf("ssh %s zfs receive %s", remoteHost, remoteDataset)
	shellCmd := fmt.Sprintf("%s | %s", sendCmd, sshCmd)

	srm.logger.Info("ZFS remote send", zap.String("cmd", shellCmd))
	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs remote send failed: %s: %w", string(out), err)
	}
	return nil
}

// ZVolManager ZVOL管理器.
type ZVolManager struct {
	logger *zap.Logger
}

// NewZVolManager 创建ZVOL管理器.
func NewZVolManager(logger *zap.Logger) *ZVolManager {
	return &ZVolManager{logger: logger}
}

// CreateZVOL 创建ZVOL.
func (zm *ZVolManager) CreateZVOL(ctx context.Context, name string, size string, props map[string]string) error {
	args := []string{"create"}
	for k, v := range props {
		args = append(args, "-o", k+"="+v)
	}
	args = append(args, "-V", size, name)

	zm.logger.Info("Creating ZVOL", zap.String("name", name), zap.String("size", size))
	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs create zvol failed: %s: %w", string(output), err)
	}
	return nil
}

// ResizeZVOL 调整ZVOL大小.
func (zm *ZVolManager) ResizeZVOL(ctx context.Context, name string, newSize string) error {
	cmd := exec.CommandContext(ctx, "zfs", "set", "volsize="+newSize, name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs resize zvol failed: %s: %w", string(output), err)
	}
	return nil
}

// DestroyZVOL 销毁ZVOL.
func (zm *ZVolManager) DestroyZVOL(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "zfs", "destroy", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy zvol failed: %s: %w", string(output), err)
	}
	return nil
}

// ListZVOLs 列出ZVOL.
func (zm *ZVolManager) ListZVOLs(ctx context.Context, parent string) ([]ZVOLInfo, error) {
	args := []string{"list", "-H", "-o", "name,volsize,volmode,volblocksize,sync,used,referenced", "-t", "volume"}
	if parent != "" {
		args = append(args, "-r", parent)
	}

	cmd := exec.CommandContext(ctx, "zfs", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list zvols failed: %w", err)
	}

	var zvols []ZVOLInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		zvol := ZVOLInfo{
			Name:      f[0],
			VolMode:   f[2],
			BlockSize: f[3],
			Sync:      f[4],
		}
		zvols = append(zvols, zvol)
	}
	return zvols, nil
}
