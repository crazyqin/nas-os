// Package btrfs 提供与 btrfs 文件系统交互的低级命令封装
package btrfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 命令参数验证
var (
	// 安全的设备路径模式：/dev/sdX, /dev/nvmeXnY, /dev/mapper/XXX
	devicePathRegex = regexp.MustCompile(`^/dev/(sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+|mapper/[a-zA-Z0-9_-]+)$`)
)

// validateDevicePath 验证设备路径是否安全
func validateDevicePath(device string) error {
	if !devicePathRegex.MatchString(device) {
		return fmt.Errorf("invalid device path: %s", device)
	}
	return nil
}

// VolumeInfo 表示一个 btrfs 卷的信息
type VolumeInfo struct {
	Name     string   // 卷标签
	UUID     string   // 卷 UUID
	Devices  []string // 设备列表
	Size     uint64   // 总大小（字节）
	Used     uint64   // 已使用（字节）
	Profile  string   // 数据配置（single, raid0, raid1, raid10, raid5, raid6）
	Metadata string   // 元数据配置
}

// SubVolumeInfo 表示子卷信息
type SubVolumeInfo struct {
	ID        uint64    // 子卷 ID
	Name      string    // 子卷名称
	Path      string    // 子卷路径
	ParentID  uint64    // 父卷 ID
	ReadOnly  bool      // 是否只读
	UUID      string    // 子卷 UUID
	CreatedAt time.Time // 创建时间
	Size      uint64    // 大小（估算）
}

// SnapshotInfo 表示快照信息
type SnapshotInfo struct {
	Name      string    // 快照名称
	Path      string    // 快照路径
	Source    string    // 源子卷
	ReadOnly  bool      // 是否只读
	CreatedAt time.Time // 创建时间
}

// DeviceStats 设备统计
type DeviceStats struct {
	Device  string // 设备路径
	Size    uint64 // 设备大小
	Used    uint64 // 已使用
	Profile string // 配置类型
}

// BalanceStatus 平衡状态
type BalanceStatus struct {
	Running   bool      // 是否正在运行
	Progress  float64   // 进度百分比
	StartTime time.Time // 开始时间
}

// ScrubStatus 校验状态
type ScrubStatus struct {
	Running      bool      // 是否正在运行
	Progress     float64   // 进度百分比
	DataScrubbed uint64    // 已校验数据量
	Errors       uint64    // 发现的错误数
	StartTime    time.Time // 开始时间
}

// Executer 接口定义 btrfs 命令执行器
type Executer interface {
	Execute(args ...string) ([]byte, error)
	ExecuteWithInput(input string, args ...string) ([]byte, error)
}

// Commander 实现 Executer 接口
type Commander struct {
	sudo bool // 是否使用 sudo
}

// NewCommander 创建命令执行器
func NewCommander(sudo bool) *Commander {
	return &Commander{sudo: sudo}
}

// Execute 执行 btrfs 命令
func (c *Commander) Execute(args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if c.sudo {
		allArgs := append([]string{"btrfs"}, args...)
		cmd = exec.Command("sudo", allArgs...)
	} else {
		cmd = exec.Command("btrfs", args...)
	}
	return cmd.Output()
}

// ExecuteWithInput 执行命令并传入输入
func (c *Commander) ExecuteWithInput(input string, args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if c.sudo {
		allArgs := append([]string{"btrfs"}, args...)
		cmd = exec.Command("sudo", allArgs...)
	} else {
		cmd = exec.Command("btrfs", args...)
	}
	cmd.Stdin = strings.NewReader(input)
	return cmd.Output()
}

// Client btrfs 客户端
type Client struct {
	exec Executer
}

// NewClient 创建 btrfs 客户端
func NewClient(sudo bool) *Client {
	return &Client{
		exec: NewCommander(sudo),
	}
}

// NewClientWithExecuter 使用自定义执行器创建客户端
func NewClientWithExecuter(exec Executer) *Client {
	return &Client{exec: exec}
}

// ========== 卷管理 ==========

// ListVolumes 列出所有 btrfs 卷
func (c *Client) ListVolumes() ([]VolumeInfo, error) {
	output, err := c.exec.Execute("filesystem", "show")
	if err != nil {
		return nil, fmt.Errorf("获取卷列表失败: %w", err)
	}

	return parseVolumeList(output)
}

// parseVolumeList 解析 btrfs filesystem show 输出
func parseVolumeList(output []byte) ([]VolumeInfo, error) {
	var volumes []VolumeInfo
	var current *VolumeInfo

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text() // 不 TrimSpace，保留前导空白
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 解析卷头：Label: 'name'  uuid: xxx
		if strings.HasPrefix(trimmed, "Label:") {
			if current != nil {
				volumes = append(volumes, *current)
			}
			current = &VolumeInfo{}

			// 提取 Label
			if idx := strings.Index(trimmed, "'"); idx >= 0 {
				end := strings.Index(trimmed[idx+1:], "'")
				if end >= 0 {
					current.Name = trimmed[idx+1 : idx+1+end]
				}
			}

			// 提取 UUID
			if idx := strings.Index(trimmed, "uuid:"); idx >= 0 {
				current.UUID = strings.TrimSpace(trimmed[idx+5:])
			}
		} else if strings.Contains(trimmed, "devid") && current != nil {
			// 解析设备行：devid    1 size 931.51GiB used 842.42GiB path /dev/sda1
			fields := strings.Fields(trimmed)
			for i, f := range fields {
				if f == "path" && i+1 < len(fields) {
					current.Devices = append(current.Devices, fields[i+1])
				}
			}
		}
	}

	if current != nil {
		volumes = append(volumes, *current)
	}

	return volumes, nil
}

// GetUsage 获取卷使用情况
func (c *Client) GetUsage(mountPoint string) (total, used, free uint64, err error) {
	output, err := c.exec.Execute("filesystem", "usage", "-b", mountPoint)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("获取使用情况失败: %w", err)
	}

	return parseUsage(output)
}

// parseUsage 解析 btrfs filesystem usage 输出
func parseUsage(output []byte) (total, used, free uint64, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Device size: 1000000000000
		if strings.HasPrefix(line, "Device size:") {
			sizeStr := strings.TrimSpace(strings.TrimPrefix(line, "Device size:"))
			if val, parseErr := strconv.ParseUint(sizeStr, 10, 64); parseErr == nil {
				total = val
			}
		}
		// Used: 500000000000
		if strings.HasPrefix(line, "Used:") {
			usedStr := strings.TrimSpace(strings.TrimPrefix(line, "Used:"))
			if val, parseErr := strconv.ParseUint(usedStr, 10, 64); parseErr == nil {
				used = val
			}
		}
		// Free (estimated): 400000000000
		if strings.HasPrefix(line, "Free (estimated):") {
			freeStr := strings.TrimSpace(strings.TrimPrefix(line, "Free (estimated):"))
			if val, parseErr := strconv.ParseUint(freeStr, 10, 64); parseErr == nil {
				free = val
			}
		}
	}

	free = total - used
	return
}

// CreateVolume 创建新的 btrfs 卷
func (c *Client) CreateVolume(label string, devices []string, dataProfile, metadataProfile string) error {
	// 验证设备路径（防止命令注入）
	for _, device := range devices {
		if err := validateDevicePath(device); err != nil {
			return fmt.Errorf("invalid device path: %w", err)
		}
	}
	// 验证标签
	if strings.ContainsAny(label, ";|&$`\\") {
		return fmt.Errorf("invalid label characters")
	}

	args := []string{"-f", "-L", label}
	if dataProfile != "" && dataProfile != "single" {
		args = append(args, "-d", dataProfile)
	}
	if metadataProfile != "" && metadataProfile != "single" {
		args = append(args, "-m", metadataProfile)
	}
	args = append(args, devices...)

	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", append([]string{"mkfs.btrfs"}, args...)...)
	} else {
		cmd = exec.Command("mkfs.btrfs", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建卷失败: %w, output: %s", err, string(output))
	}

	return nil
}

// DeleteVolume 删除卷（危险操作）
func (c *Client) DeleteVolume(device string) error {
	// 验证设备路径（防止命令注入）
	if err := validateDevicePath(device); err != nil {
		return fmt.Errorf("invalid device path: %w", err)
	}

	// 使用 wipefs 清除文件系统签名
	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", "wipefs", "-a", device)
	} else {
		cmd = exec.Command("wipefs", "-a", device)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除卷失败: %w, output: %s", err, string(output))
	}

	return nil
}

// Mount 挂载卷
func (c *Client) Mount(device, mountPoint string, options []string) error {
	args := []string{device, mountPoint}
	if len(options) > 0 {
		args = append([]string{"-o", strings.Join(options, ",")}, args...)
	}

	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", append([]string{"mount"}, args...)...)
	} else {
		cmd = exec.Command("mount", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("挂载失败: %w, output: %s", err, string(output))
	}

	return nil
}

// Unmount 卸载卷
func (c *Client) Unmount(mountPoint string) error {
	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", "umount", mountPoint)
	} else {
		cmd = exec.Command("umount", mountPoint)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("卸载失败: %w, output: %s", err, string(output))
	}

	return nil
}

// ========== 子卷管理 ==========

// ListSubVolumes 列出卷下的所有子卷
func (c *Client) ListSubVolumes(mountPoint string) ([]SubVolumeInfo, error) {
	output, err := c.exec.Execute("subvolume", "list", "-p", "-u", "-q", mountPoint)
	if err != nil {
		return nil, fmt.Errorf("获取子卷列表失败: %w", err)
	}

	return parseSubVolumeList(output, mountPoint)
}

// parseSubVolumeList 解析 btrfs subvolume list 输出
func parseSubVolumeList(output []byte, mountPoint string) ([]SubVolumeInfo, error) {
	var subvols []SubVolumeInfo

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 格式: ID 256 gen 10 parent 5 top level 5 uuid xxx parent_uuid xxx path subdir
		subvol := SubVolumeInfo{}

		fields := strings.Fields(line)
		for i := 0; i < len(fields); i++ {
			switch fields[i] {
			case "ID":
				if i+1 < len(fields) {
					if val, parseErr := strconv.ParseUint(fields[i+1], 10, 64); parseErr == nil {
						subvol.ID = val
					}
				}
			case "parent":
				if i+1 < len(fields) {
					if val, parseErr := strconv.ParseUint(fields[i+1], 10, 64); parseErr == nil {
						subvol.ParentID = val
					}
				}
			case "uuid":
				if i+1 < len(fields) {
					subvol.UUID = fields[i+1]
				}
			case "path":
				if i+1 < len(fields) {
					subvol.Name = fields[i+1]
					subvol.Path = mountPoint + "/" + fields[i+1]
				}
			}
		}

		if subvol.Name != "" {
			subvols = append(subvols, subvol)
		}
	}

	return subvols, nil
}

// CreateSubVolume 创建子卷
func (c *Client) CreateSubVolume(path string) error {
	output, err := c.exec.Execute("subvolume", "create", path)
	if err != nil {
		return fmt.Errorf("创建子卷失败: %w, output: %s", err, string(output))
	}
	return nil
}

// DeleteSubVolume 删除子卷
func (c *Client) DeleteSubVolume(path string) error {
	output, err := c.exec.Execute("subvolume", "delete", path)
	if err != nil {
		return fmt.Errorf("删除子卷失败: %w, output: %s", err, string(output))
	}
	return nil
}

// GetSubVolumeInfo 获取子卷详细信息
func (c *Client) GetSubVolumeInfo(path string) (*SubVolumeInfo, error) {
	output, err := c.exec.Execute("subvolume", "show", path)
	if err != nil {
		return nil, fmt.Errorf("获取子卷信息失败: %w", err)
	}

	return parseSubVolumeShow(output, path)
}

// parseSubVolumeShow 解析 btrfs subvolume show 输出
func parseSubVolumeShow(output []byte, path string) (*SubVolumeInfo, error) {
	info := &SubVolumeInfo{Path: path}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Name:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		} else if strings.HasPrefix(line, "UUID:") {
			info.UUID = strings.TrimSpace(strings.TrimPrefix(line, "UUID:"))
		} else if strings.HasPrefix(line, "Parent UUID:") {
			// 父 UUID
		} else if strings.HasPrefix(line, "Parent ID:") {
			idStr := strings.TrimSpace(strings.TrimPrefix(line, "Parent ID:"))
			if val, parseErr := strconv.ParseUint(idStr, 10, 64); parseErr == nil {
				info.ParentID = val
			}
		} else if strings.HasPrefix(line, "Flags:") {
			flags := strings.TrimSpace(strings.TrimPrefix(line, "Flags:"))
			info.ReadOnly = strings.Contains(flags, "readonly")
		}
	}

	return info, nil
}

// SetSubVolumeReadOnly 设置子卷只读属性
func (c *Client) SetSubVolumeReadOnly(path string, readOnly bool) error {
	var args []string
	if readOnly {
		args = []string{"property", "set", path, "ro", "true"}
	} else {
		args = []string{"property", "set", path, "ro", "false"}
	}

	output, err := c.exec.Execute(args...)
	if err != nil {
		return fmt.Errorf("设置子卷只读属性失败: %w, output: %s", err, string(output))
	}
	return nil
}

// MountSubVolume 挂载指定子卷到目标目录
// subvolPath: 子卷路径（相对于卷根目录，如 "documents" 或 ".snapshots/daily"）
// mountPoint: 目标挂载点
// device: 设备路径
func (c *Client) MountSubVolume(device, subvolPath, mountPoint string) error {
	// 使用 subvol= 挂载选项
	options := fmt.Sprintf("subvol=%s", subvolPath)

	args := []string{"-o", options, device, mountPoint}

	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", append([]string{"mount"}, args...)...)
	} else {
		cmd = exec.Command("mount", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("挂载子卷失败: %w, output: %s", err, string(output))
	}
	return nil
}

// MountSubVolumeByID 通过子卷 ID 挂载子卷
func (c *Client) MountSubVolumeByID(device string, subvolID uint64, mountPoint string) error {
	options := fmt.Sprintf("subvolid=%d", subvolID)

	args := []string{"-o", options, device, mountPoint}

	var cmd *exec.Cmd
	if cm, ok := c.exec.(*Commander); ok && cm.sudo {
		cmd = exec.Command("sudo", append([]string{"mount"}, args...)...)
	} else {
		cmd = exec.Command("mount", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("挂载子卷失败: %w, output: %s", err, string(output))
	}
	return nil
}

// GetDefaultSubVolume 获取默认子卷 ID
func (c *Client) GetDefaultSubVolume(mountPoint string) (uint64, error) {
	output, err := c.exec.Execute("subvolume", "get-default", mountPoint)
	if err != nil {
		return 0, fmt.Errorf("获取默认子卷失败: %w", err)
	}

	// 输出格式: ID 256 (or 5 for root)
	text := strings.TrimSpace(string(output))
	// 提取 ID
	fields := strings.Fields(text)
	if len(fields) >= 2 {
		id, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("解析子卷 ID 失败: %w", err)
		}
		return id, nil
	}

	return 0, fmt.Errorf("无法解析默认子卷 ID: %s", text)
}

// SetDefaultSubVolume 设置默认子卷
func (c *Client) SetDefaultSubVolume(mountPoint string, subvolID uint64) error {
	output, err := c.exec.Execute("subvolume", "set-default", fmt.Sprintf("%d", subvolID), mountPoint)
	if err != nil {
		return fmt.Errorf("设置默认子卷失败: %w, output: %s", err, string(output))
	}
	return nil
}

// ========== 快照管理 ==========

// CreateSnapshot 创建快照
func (c *Client) CreateSnapshot(source, dest string, readOnly bool) error {
	args := []string{"subvolume", "snapshot"}
	if readOnly {
		args = append(args, "-r")
	}
	args = append(args, source, dest)

	output, err := c.exec.Execute(args...)
	if err != nil {
		return fmt.Errorf("创建快照失败: %w, output: %s", err, string(output))
	}
	return nil
}

// DeleteSnapshot 删除快照（与删除子卷相同）
func (c *Client) DeleteSnapshot(path string) error {
	return c.DeleteSubVolume(path)
}

// RestoreSnapshot 恢复快照（实际上是创建一个可写快照）
func (c *Client) RestoreSnapshot(snapshotPath, targetPath string) error {
	// 恢复快照就是创建一个可写快照到目标位置
	return c.CreateSnapshot(snapshotPath, targetPath, false)
}

// ListSnapshots 列出所有快照（通过检查子卷的只读属性）
func (c *Client) ListSnapshots(mountPoint string) ([]SnapshotInfo, error) {
	subvols, err := c.ListSubVolumes(mountPoint)
	if err != nil {
		return nil, err
	}

	var snapshots []SnapshotInfo
	for _, sv := range subvols {
		info, err := c.GetSubVolumeInfo(sv.Path)
		if err != nil {
			continue
		}

		// 检查是否为快照（通常快照名称包含特定前缀或在特定目录）
		// 这里简单判断只读子卷为快照
		if info.ReadOnly || strings.Contains(sv.Name, "snap") || strings.Contains(sv.Name, "snapshot") {
			snapshots = append(snapshots, SnapshotInfo{
				Name:     sv.Name,
				Path:     sv.Path,
				ReadOnly: info.ReadOnly,
			})
		}
	}

	return snapshots, nil
}

// ========== RAID 管理 ==========

// GetDeviceStats 获取设备统计信息
func (c *Client) GetDeviceStats(mountPoint string) ([]DeviceStats, error) {
	output, err := c.exec.Execute("device", "usage", mountPoint)
	if err != nil {
		return nil, fmt.Errorf("获取设备统计失败: %w", err)
	}

	return parseDeviceUsage(output)
}

// parseDeviceUsage 解析 btrfs device usage 输出
func parseDeviceUsage(output []byte) ([]DeviceStats, error) {
	var stats []DeviceStats
	var current *DeviceStats

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if current != nil {
				stats = append(stats, *current)
				current = nil
			}
			continue
		}

		// 设备行：/dev/sda1, ID: 1（不以空格开头，包含逗号）
		if strings.Contains(line, ", ID:") || (strings.HasPrefix(line, "/dev/") && strings.Contains(line, ",")) {
			if current != nil {
				stats = append(stats, *current)
			}
			current = &DeviceStats{}
			current.Device = strings.Split(line, ",")[0]
			current.Device = strings.TrimSpace(current.Device)
		} else if current != nil {
			if strings.HasPrefix(line, "Device size:") {
				sizeStr := strings.TrimSpace(strings.TrimPrefix(line, "Device size:"))
				current.Size = parseSizeStr(sizeStr)
			} else if strings.HasPrefix(line, "Device slack:") || strings.Contains(line, "slack") {
				// slack 不计入
			} else if strings.Contains(line, "Data,") || strings.Contains(line, "Metadata,") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.HasSuffix(f, "GiB") || strings.HasSuffix(f, "MiB") || strings.HasSuffix(f, "TiB") {
						current.Used += parseSizeStr(f)
					}
				}
			}
		}
	}

	if current != nil {
		stats = append(stats, *current)
	}

	return stats, nil
}

// parseSizeStr 解析大小字符串
func parseSizeStr(s string) uint64 {
	s = strings.TrimSpace(s)

	var multiplier uint64 = 1
	if strings.HasSuffix(s, "KiB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KiB")
	} else if strings.HasSuffix(s, "MiB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MiB")
	} else if strings.HasSuffix(s, "GiB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GiB")
	} else if strings.HasSuffix(s, "TiB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TiB")
	} else if strings.HasSuffix(s, "KiB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KiB")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		// 解析失败，使用默认值 0
		val = 0
	}
	return uint64(val * float64(multiplier))
}

// AddDevice 添加设备到卷（扩容）
func (c *Client) AddDevice(mountPoint, device string) error {
	output, err := c.exec.Execute("device", "add", device, mountPoint)
	if err != nil {
		return fmt.Errorf("添加设备失败: %w, output: %s", err, string(output))
	}
	return nil
}

// RemoveDevice 从卷中移除设备
func (c *Client) RemoveDevice(mountPoint, device string) error {
	output, err := c.exec.Execute("device", "delete", device, mountPoint)
	if err != nil {
		return fmt.Errorf("移除设备失败: %w, output: %s", err, string(output))
	}
	return nil
}

// ConvertProfile 转换 RAID 配置
func (c *Client) ConvertProfile(mountPoint, dataProfile, metadataProfile string) error {
	var args []string
	if dataProfile != "" {
		args = append(args, "-d", dataProfile)
	}
	if metadataProfile != "" {
		args = append(args, "-m", metadataProfile)
	}
	args = append(args, mountPoint)

	output, err := c.exec.Execute(append([]string{"balance", "start"}, args...)...)
	if err != nil {
		return fmt.Errorf("转换配置失败: %w, output: %s", err, string(output))
	}
	return nil
}

// ========== 平衡与校验 ==========

// StartBalance 启动数据平衡
func (c *Client) StartBalance(mountPoint string) error {
	output, err := c.exec.Execute("balance", "start", mountPoint)
	if err != nil {
		return fmt.Errorf("启动平衡失败: %w, output: %s", err, string(output))
	}
	return nil
}

// GetBalanceStatus 获取平衡状态
func (c *Client) GetBalanceStatus(mountPoint string) (*BalanceStatus, error) {
	output, err := c.exec.Execute("balance", "status", mountPoint)
	if err != nil {
		// 如果没有运行中的平衡，命令会返回错误
		return &BalanceStatus{Running: false}, nil
	}

	return parseBalanceStatus(output)
}

// parseBalanceStatus 解析平衡状态
func parseBalanceStatus(output []byte) (*BalanceStatus, error) {
	status := &BalanceStatus{}
	text := string(output)

	// 检查是否正在运行
	if strings.Contains(text, "is running") || strings.Contains(text, "running") {
		status.Running = true
	}

	// 解析进度，如 "1% done"
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "%") && strings.Contains(line, "done") {
			// 提取百分比，如 "50% done"
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, "%") {
					percentStr := strings.TrimSuffix(part, "%")
					if val, parseErr := strconv.ParseFloat(percentStr, 64); parseErr == nil {
						status.Progress = val
					}
					break
				}
			}
		}
	}

	return status, nil
}

// CancelBalance 取消平衡操作
func (c *Client) CancelBalance(mountPoint string) error {
	output, err := c.exec.Execute("balance", "cancel", mountPoint)
	if err != nil {
		return fmt.Errorf("取消平衡失败: %w, output: %s", err, string(output))
	}
	return nil
}

// StartScrub 启动数据校验
func (c *Client) StartScrub(mountPoint string) error {
	output, err := c.exec.Execute("scrub", "start", mountPoint)
	if err != nil {
		return fmt.Errorf("启动校验失败: %w, output: %s", err, string(output))
	}
	return nil
}

// GetScrubStatus 获取校验状态
func (c *Client) GetScrubStatus(mountPoint string) (*ScrubStatus, error) {
	output, err := c.exec.Execute("scrub", "status", mountPoint)
	if err != nil {
		return nil, fmt.Errorf("获取校验状态失败: %w", err)
	}

	return parseScrubStatus(output)
}

// parseScrubStatus 解析校验状态
func parseScrubStatus(output []byte) (*ScrubStatus, error) {
	status := &ScrubStatus{}
	text := string(output)

	if strings.Contains(text, "running") || strings.Contains(text, "in progress") || strings.Contains(text, "started") {
		status.Running = true
	}

	// 解析进度
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Progress:") || strings.Contains(line, "%") {
			// 提取百分比
			if idx := strings.Index(line, "%"); idx > 0 {
				percentStr := line[:idx]
				// 提取数字
				var numStr string
				for _, c := range percentStr {
					if c >= '0' && c <= '9' || c == '.' {
						numStr += string(c)
					}
				}
				if numStr != "" {
					if val, parseErr := strconv.ParseFloat(numStr, 64); parseErr == nil {
						status.Progress = val
					}
				}
			}
		}

		if strings.Contains(line, "Error summary:") || strings.Contains(line, "errors") {
			// 提取错误数
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "errors:" && i+1 < len(fields) {
					if val, parseErr := strconv.ParseUint(fields[i+1], 10, 64); parseErr == nil {
						status.Errors = val
					}
				}
			}
		}
	}

	return status, nil
}

// CancelScrub 取消校验操作
func (c *Client) CancelScrub(mountPoint string) error {
	output, err := c.exec.Execute("scrub", "cancel", mountPoint)
	if err != nil {
		return fmt.Errorf("取消校验失败: %w, output: %s", err, string(output))
	}
	return nil
}
