package storage

import (
	"fmt"
	"os/exec"
)

// Manager 存储管理器
type Manager struct {
	volumes []*Volume
}

// Volume 卷信息
type Volume struct {
	Name      string
	Device    string
	Size      uint64
	Used      uint64
	Profile   string // single, raid0, raid1, raid10
	Subvolumes []*SubVolume
}

// SubVolume 子卷信息
type SubVolume struct {
	Name   string
	Path   string
	Size   uint64
	Snapshots []*Snapshot
}

// Snapshot 快照信息
type Snapshot struct {
	Name      string
	CreatedAt string
	ReadOnly  bool
}

// NewManager 创建存储管理器
func NewManager() (*Manager, error) {
	m := &Manager{
		volumes: make([]*Volume, 0),
	}
	// 扫描现有 btrfs 卷
	if err := m.scanVolumes(); err != nil {
		return nil, err
	}
	return m, nil
}

// scanVolumes 扫描系统上的 btrfs 卷
func (m *Manager) scanVolumes() error {
	cmd := exec.Command("btrfs", "filesystem", "show")
	output, err := cmd.Output()
	if err != nil {
		// 如果没有 btrfs 卷，不算错误
		return nil
	}
	// 解析输出...
	_ = output
	return nil
}

// CreateVolume 创建新卷
func (m *Manager) CreateVolume(name string, devices []string, profile string) (*Volume, error) {
	// btrfs filesystem create
	args := []string{"create", "--label", name}
	if profile != "" && profile != "single" {
		args = append(args, "-d", profile, "-m", profile)
	}
	args = append(args, devices...)

	cmd := exec.Command("mkfs.btrfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建卷失败：%v, %s", err, string(output))
	}

	vol := &Volume{
		Name:    name,
		Device:  devices[0],
		Profile: profile,
	}
	m.volumes = append(m.volumes, vol)
	return vol, nil
}

// CreateSubVolume 创建子卷
func (m *Manager) CreateSubVolume(volumeName, name string) (*SubVolume, error) {
	mountPoint := fmt.Sprintf("/mnt/%s", volumeName)
	subvolPath := fmt.Sprintf("%s/%s", mountPoint, name)

	cmd := exec.Command("btrfs", "subvolume", "create", subvolPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建子卷失败：%v, %s", err, string(output))
	}

	subvol := &SubVolume{
		Name: name,
		Path: subvolPath,
	}
	return subvol, nil
}

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(volumeName, subvolName, snapshotName string, readOnly bool) (*Snapshot, error) {
	mountPoint := fmt.Sprintf("/mnt/%s", volumeName)
	srcPath := fmt.Sprintf("%s/%s", mountPoint, subvolName)
	snapPath := fmt.Sprintf("%s/%s", mountPoint, snapshotName)

	args := []string{"subvolume", "snapshot"}
	if readOnly {
		args = append(args, "-r")
	}
	args = append(args, srcPath, snapPath)

	cmd := exec.Command("btrfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建快照失败：%v, %s", err, string(output))
	}

	snap := &Snapshot{
		Name:     snapshotName,
		ReadOnly: readOnly,
	}
	return snap, nil
}

// GetVolumes 获取所有卷
func (m *Manager) GetVolumes() []*Volume {
	return m.volumes
}

// GetVolume 获取指定卷
func (m *Manager) GetVolume(name string) *Volume {
	for _, v := range m.volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// GetUsage 获取卷使用情况
func (m *Manager) GetUsage(volumeName string) (total, used, free uint64, err error) {
	mountPoint := fmt.Sprintf("/mnt/%s", volumeName)
	cmd := exec.Command("btrfs", "filesystem", "usage", "-b", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}
	// 解析输出获取使用情况
	_ = output
	return 0, 0, 0, nil
}

// Balance 平衡卷数据
func (m *Manager) Balance(volumeName string) error {
	mountPoint := fmt.Sprintf("/mnt/%s", volumeName)
	cmd := exec.Command("btrfs", "balance", "start", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("平衡失败：%v, %s", err, string(output))
	}
	return nil
}

// Scrub 数据校验
func (m *Manager) Scrub(volumeName string) error {
	mountPoint := fmt.Sprintf("/mnt/%s", volumeName)
	cmd := exec.Command("btrfs", "scrub", "start", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("校验失败：%v, %s", err, string(output))
	}
	return nil
}
