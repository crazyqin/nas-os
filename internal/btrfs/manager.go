package btrfs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// BtrfsManager Btrfs存储管理器
type BtrfsManager struct {
	logger *zap.Logger
}

// NewBtrfsManager 创建Btrfs管理器
func NewBtrfsManager(logger *zap.Logger) *BtrfsManager {
	return &BtrfsManager{logger: logger}
}

// SubvolumeInfo 子卷信息
type SubvolumeInfo struct {
	ID        int       `json:"id"`
	Path      string    `json:"path"`
	ParentID  int       `json:"parent_id"`
	TopLevel  int       `json:"top_level"`
	CreatedAt time.Time `json:"created_at"`
}

// BtrfsPoolInfo Btrfs池信息
type BtrfsPoolInfo struct {
	UUID         string `json:"uuid"`
	Label        string `json:"label"`
	TotalSize    uint64 `json:"total_size"`
	UsedSize     uint64 `json:"used_size"`
	FreeSize     uint64 `json:"free_size"`
	Devices      []BtrfsDevice `json:"devices"`
	Profiles     map[string]string `json:"profiles"`
}

// BtrfsDevice Btrfs设备信息
type BtrfsDevice struct {
	Path      string `json:"path"`
	Size      uint64 `json:"size"`
	Used      uint64 `json:"used"`
	UUID      string `json:"uuid"`
}

// RAIDProfile RAID配置文件类型
type RAIDProfile string

const (
	RAIDSingle  RAIDProfile = "single"
	RAIDRAID0   RAIDProfile = "raid0"
	RAIDRAID1   RAIDProfile = "raid1"
	RAIDRAID1C3 RAIDProfile = "raid1c3"
	RAIDRAID1C4 RAIDProfile = "raid1c4"
	RAIDRAID5   RAIDProfile = "raid5"
	RAIDRAID6   RAIDProfile = "raid6"
	RAIDRAID10  RAIDProfile = "raid10"
	RAIDDUP     RAIDProfile = "dup"
)

// CreatePool 创建Btrfs池
func (bm *BtrfsManager) CreatePool(ctx context.Context, label string, devices []string, profile RAIDProfile) error {
	args := []string{"-L", label}
	if profile != "" {
		args = append(args, "-d", string(profile), "-m", string(profile))
	}
	args = append(args, devices...)

	bm.logger.Info("Creating Btrfs pool",
		zap.String("label", label),
		zap.Strings("devices", devices),
		zap.String("profile", string(profile)))

	cmd := exec.CommandContext(ctx, "mkfs.btrfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.btrfs failed: %s: %w", string(output), err)
	}
	return nil
}

// MountPool 挂载Btrfs池
func (bm *BtrfsManager) MountPool(ctx context.Context, device, mountpoint string, options map[string]string) error {
	args := []string{"mount"}
	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}
	args = append(args, device, mountpoint)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mount btrfs failed: %s: %w", string(output), err)
	}
	return nil
}

// UnmountPool 卸载Btrfs池
func (bm *BtrfsManager) UnmountPool(ctx context.Context, mountpoint string) error {
	cmd := exec.CommandContext(ctx, "umount", mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount failed: %s: %w", string(output), err)
	}
	return nil
}

// GetPoolInfo 获取池信息
func (bm *BtrfsManager) GetPoolInfo(ctx context.Context, mountpoint string) (*BtrfsPoolInfo, error) {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "show", mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("btrfs filesystem show failed: %w", err)
	}

	info := &BtrfsPoolInfo{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "uuid:") {
			info.UUID = strings.TrimSpace(strings.TrimPrefix(line, "uuid:"))
		} else if strings.HasPrefix(line, "Label:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				info.Label = strings.TrimSpace(parts[1])
			}
		}
	}

	return info, nil
}

// CreateSubvolume 创建子卷
func (bm *BtrfsManager) CreateSubvolume(ctx context.Context, path string) error {
	bm.logger.Info("Creating Btrfs subvolume", zap.String("path", path))
	cmd := exec.CommandContext(ctx, "btrfs", "subvolume", "create", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume create failed: %s: %w", string(output), err)
	}
	return nil
}

// DeleteSubvolume 删除子卷
func (bm *BtrfsManager) DeleteSubvolume(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "subvolume", "delete", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete failed: %s: %w", string(output), err)
	}
	return nil
}

// ListSubvolumes 列出子卷
func (bm *BtrfsManager) ListSubvolumes(ctx context.Context, mountpoint string) ([]SubvolumeInfo, error) {
	cmd := exec.CommandContext(ctx, "btrfs", "subvolume", "list", "-t", mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("btrfs subvolume list failed: %w", err)
	}

	var subvols []SubvolumeInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ID") && strings.Contains(line, "gen") {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) >= 9 {
			subvol := SubvolumeInfo{}
			fmt.Sscanf(fields[0], "%d", &subvol.ID)
			subvol.Path = fields[len(fields)-1]
			subvols = append(subvols, subvol)
		}
	}
	return subvols, nil
}

// Snapshot 创建快照
func (bm *BtrfsManager) Snapshot(ctx context.Context, source, dest string, readOnly bool) error {
	args := []string{"subvolume", "snapshot"}
	if readOnly {
		args = append(args, "-r")
	}
	args = append(args, source, dest)

	bm.logger.Info("Creating Btrfs snapshot",
		zap.String("source", source),
		zap.String("dest", dest),
		zap.Bool("readonly", readOnly))

	cmd := exec.CommandContext(ctx, "btrfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs snapshot failed: %s: %w", string(output), err)
	}
	return nil
}

// SendReceive 发送/接收子卷 (增量备份)
func (bm *BtrfsManager) SendReceive(ctx context.Context, parentSnap, currentSnap, dest string) error {
	args := []string{"send", "-p", parentSnap, currentSnap}
	sendCmd := exec.CommandContext(ctx, "btrfs", args...)

	receiveCmd := exec.CommandContext(ctx, "btrfs", "receive", dest)

	// 管道连接 send | receive
	receiveCmd.Stdin, _ = sendCmd.StdoutPipe()

	if err := receiveCmd.Start(); err != nil {
		return fmt.Errorf("btrfs receive start failed: %w", err)
	}
	if err := sendCmd.Run(); err != nil {
		return fmt.Errorf("btrfs send failed: %w", err)
	}
	if err := receiveCmd.Wait(); err != nil {
		return fmt.Errorf("btrfs receive failed: %w", err)
	}
	return nil
}

// Balance 执行balance操作（在线RAID转换）
func (bm *BtrfsManager) Balance(ctx context.Context, mountpoint string, profile RAIDProfile, filters map[string]string) error {
	args := []string{"balance", "start"}

	// 添加过滤器
	if len(filters) > 0 {
		var filterParts []string
		for k, v := range filters {
			filterParts = append(filterParts, k+"="+v)
		}
		args = append(args, "-d", strings.Join(filterParts, ","))
	}

	if profile != "" {
		args = append(args, "-m", "convert="+string(profile), "-d", "convert="+string(profile))
	}

	args = append(args, mountpoint)

	bm.logger.Info("Starting Btrfs balance",
		zap.String("mountpoint", mountpoint),
		zap.String("profile", string(profile)))

	cmd := exec.CommandContext(ctx, "btrfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs balance failed: %s: %w", string(output), err)
	}
	return nil
}

// BalanceCancel 取消balance操作
func (bm *BtrfsManager) BalanceCancel(ctx context.Context, mountpoint string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "balance", "cancel", mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs balance cancel failed: %s: %w", string(output), err)
	}
	return nil
}

// BalanceStatus 获取balance状态
func (bm *BtrfsManager) BalanceStatus(ctx context.Context, mountpoint string) (string, error) {
	cmd := exec.CommandContext(ctx, "btrfs", "balance", "status", mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("btrfs balance status failed: %w", err)
	}
	return string(out), nil
}

// ScrubStart 启动scrub
func (bm *BtrfsManager) ScrubStart(ctx context.Context, mountpoint string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "scrub", "start", mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs scrub start failed: %s: %w", string(output), err)
	}
	return nil
}

// ScrubStatus 获取scrub状态
func (bm *BtrfsManager) ScrubStatus(ctx context.Context, mountpoint string) (string, error) {
	cmd := exec.CommandContext(ctx, "btrfs", "scrub", "status", mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("btrfs scrub status failed: %w", err)
	}
	return string(out), nil
}

// AddDevice 添加设备到池
func (bm *BtrfsManager) AddDevice(ctx context.Context, mountpoint, device string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "device", "add", device, mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs device add failed: %s: %w", string(output), err)
	}
	return nil
}

// RemoveDevice 从池移除设备
func (bm *BtrfsManager) RemoveDevice(ctx context.Context, mountpoint, device string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "device", "remove", device, mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs device remove failed: %s: %w", string(output), err)
	}
	return nil
}

// ResizeFilesystem 调整文件系统大小
func (bm *BtrfsManager) ResizeFilesystem(ctx context.Context, mountpoint, size string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "resize", size, mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs filesystem resize failed: %s: %w", string(output), err)
	}
	return nil
}

// Defragment 碎片整理
func (bm *BtrfsManager) Defragment(ctx context.Context, path string, compress bool) error {
	args := []string{"filesystem", "defragment"}
	if compress {
		args = append(args, "-czstd")
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, "btrfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs defragment failed: %s: %w", string(output), err)
	}
	return nil
}

// QuotaEnable 启用配额
func (bm *BtrfsManager) QuotaEnable(ctx context.Context, mountpoint string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "quota", "enable", mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs quota enable failed: %s: %w", string(output), err)
	}
	return nil
}

// QuotaSet 设置配额
func (bm *BtrfsManager) QuotaSet(ctx context.Context, subvolume, size string) error {
	cmd := exec.CommandContext(ctx, "btrfs", "qgroup", "limit", size, subvolume)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs qgroup limit failed: %s: %w", string(output), err)
	}
	return nil
}

// GetUsage 获取使用情况
func (bm *BtrfsManager) GetUsage(ctx context.Context, mountpoint string) (string, error) {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "usage", mountpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("btrfs filesystem usage failed: %w", err)
	}
	return string(out), nil
}

// ConvertRAID 在线转换RAID级别
func (bm *BtrfsManager) ConvertRAID(ctx context.Context, mountpoint string, profile RAIDProfile) error {
	bm.logger.Info("Converting Btrfs RAID profile",
		zap.String("mountpoint", mountpoint),
		zap.String("profile", string(profile)))

	// 先转换数据
	cmd := exec.CommandContext(ctx, "btrfs", "balance", "start",
		"-d", "convert="+string(profile),
		"-m", "convert="+string(profile),
		mountpoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs RAID conversion failed: %s: %w", string(output), err)
	}
	return nil
}
