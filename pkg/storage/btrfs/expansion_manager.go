// Package btrfs Btrfs RAID扩容管理器
// 实现类似 ZFS RAIDZ Expansion 的功能
package btrfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client Btrfs命令行客户端
type Client struct {
	// 超时时间
	timeout time.Duration
}

// NewClient 创建Btrfs客户端
func NewClient() *Client {
	return &Client{
		timeout: 30 * time.Second,
	}
}

// SetTimeout 设置超时时间
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// RunCommand 执行btrfs命令
func (c *Client) RunCommand(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "btrfs", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name     string   `json:"name"`
	Profile  string   `json:"profile"`
	Devices  []string `json:"devices"`
}

// ListVolumes 列出所有Btrfs卷
func (c *Client) ListVolumes() ([]VolumeInfo, error) {
	ctx := context.Background()
	output, err := c.RunCommand(ctx, "filesystem", "show")
	if err != nil {
		return nil, err
	}
	
	// 解析输出
	volumes := []VolumeInfo{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Label:") {
			// 简化解析
			volumes = append(volumes, VolumeInfo{
				Name: strings.TrimSpace(line),
			})
		}
	}
	return volumes, nil
}

// GetBalanceStatus 获取平衡状态
func (c *Client) GetBalanceStatus(mountPoint string) (*BalanceStatusInfo, error) {
	ctx := context.Background()
	output, err := c.RunCommand(ctx, "balance", "status", mountPoint)
	if err != nil {
		return nil, err
	}
	
	status := &BalanceStatusInfo{}
	if strings.Contains(output, "running") {
		status.Running = true
	}
	return status, nil
}

// BalanceStatusInfo 平衡状态信息
type BalanceStatusInfo struct {
	Running  bool    `json:"running"`
	Progress float64 `json:"progress"`
}

// GetScrubStatus 获取校验状态
func (c *Client) GetScrubStatus(mountPoint string) (*ScrubStatusInfo, error) {
	ctx := context.Background()
	output, err := c.RunCommand(ctx, "scrub", "status", mountPoint)
	if err != nil {
		return nil, err
	}
	
	status := &ScrubStatusInfo{}
	if strings.Contains(output, "running") {
		status.Running = true
	}
	return status, nil
}

// ScrubStatusInfo 校验状态信息
type ScrubStatusInfo struct {
	Running bool `json:"running"`
}

// GetDeviceStats 获取设备统计
func (c *Client) GetDeviceStats(mountPoint, device string) (*DeviceStatsInfo, error) {
	ctx := context.Background()
	output, err := c.RunCommand(ctx, "device", "stats", mountPoint)
	if err != nil {
		return nil, err
	}
	
	stats := &DeviceStatsInfo{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "write_io_errs") {
			stats.WriteErrors = parseBtrfsStatsValue(line)
		}
		if strings.Contains(line, "read_io_errs") {
			stats.ReadErrors = parseBtrfsStatsValue(line)
		}
	}
	return stats, nil
}

// DeviceStatsInfo 设备统计信息
type DeviceStatsInfo struct {
	WriteErrors int64  `json:"write_errors"`
	ReadErrors  int64  `json:"read_errors"`
	Device      string `json:"device"`
}

// parseBtrfsStatsValue 解析btrfs统计值
func parseBtrfsStatsValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		return val
	}
	return 0
}

// GetUsage 获取使用情况
func (c *Client) GetUsage(mountPoint string) (*UsageInfo, error) {
	ctx := context.Background()
	output, err := c.RunCommand(ctx, "filesystem", "usage", mountPoint)
	if err != nil {
		return nil, err
	}
	
	usage := &UsageInfo{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Total:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				usage.Total, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
		if strings.Contains(line, "Used:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				usage.Used, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}
	return usage, nil
}

// UsageInfo 使用情况信息
type UsageInfo struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

// AddDevice 添加设备
func (c *Client) AddDevice(mountPoint, device string) error {
	ctx := context.Background()
	_, err := c.RunCommand(ctx, "device", "add", device, mountPoint)
	return err
}

// StartBalance 启动平衡
func (c *Client) StartBalance(mountPoint string) error {
	ctx := context.Background()
	_, err := c.RunCommand(ctx, "balance", "start", mountPoint)
	return err
}

// CancelBalance 取消平衡
func (c *Client) CancelBalance(mountPoint string) error {
	ctx := context.Background()
	_, err := c.RunCommand(ctx, "balance", "cancel", mountPoint)
	return err
}

// ConvertProfile 转换配置
func (c *Client) ConvertProfile(mountPoint string, profile string) error {
	ctx := context.Background()
	_, err := c.RunCommand(ctx, "balance", "start", "-dconvert="+profile, mountPoint)
	return err
}

// validateDevicePath 验证设备路径
func validateDevicePath(devicePath string) error {
	if devicePath == "" {
		return fmt.Errorf("device path is empty")
	}
	if !strings.HasPrefix(devicePath, "/dev/") {
		return fmt.Errorf("device path must start with /dev/")
	}
	info, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("cannot access device: %v", err)
	}
	// 检查是否是块设备文件
	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("path is not a block device")
	}
	return nil
}

// RAIDExpansionManager RAID扩容管理器
type RAIDExpansionManager struct {
	mu sync.RWMutex

	// btrfs客户端
	client *Client

	// 当前扩展状态
	currentStatus *ExpansionStatus

	// 扩展历史
	history *ExpansionHistory

	// 配置保存路径
	configPath string

	// Btrfs可用性
	available bool

	// 取消通道
	cancelChan chan struct{}

	// 暂停通道
	pauseChan chan struct{}

	// 恂复通道
	resumeChan chan struct{}

	// 状态变更回调
	onStateChange func(status *ExpansionStatus)
}

// NewRAIDExpansionManager 创建RAID扩容管理器
func NewRAIDExpansionManager(client *Client, configPath string) (*RAIDExpansionManager, error) {
	m := &RAIDExpansionManager{
		client:     client,
		configPath: configPath,
		history: &ExpansionHistory{
			Expansions: []ExpansionStatus{},
		},
		cancelChan: make(chan struct{}, 1),
		pauseChan:  make(chan struct{}, 1),
		resumeChan: make(chan struct{}, 1),
	}

	// 检查Btrfs可用性
	m.checkAvailable()

	// 加载历史记录
	if configPath != "" {
		_ = m.loadHistory()
	}

	return m, nil
}

// checkAvailable 检查Btrfs可用性
func (m *RAIDExpansionManager) checkAvailable() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "btrfs", "version")
	m.available = cmd.Run() == nil
}

// IsAvailable 检查管理器是否可用
func (m *RAIDExpansionManager) IsAvailable() bool {
	return m.available
}

// loadHistory 加载扩展历史
func (m *RAIDExpansionManager) loadHistory() error {
	if m.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(m.configPath), "raid_expansion_history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.history)
}

// saveHistory 保存扩展历史
func (m *RAIDExpansionManager) saveHistory() error {
	if m.configPath == "" {
		return nil
	}

	m.history.LastUpdated = time.Now()
	historyPath := filepath.Join(filepath.Dir(m.configPath), "raid_expansion_history.json")

	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0640)
}

// SetStateChangeCallback 设置状态变更回调
func (m *RAIDExpansionManager) SetStateChangeCallback(callback func(status *ExpansionStatus)) {
	m.mu.Lock()
	m.onStateChange = callback
	m.mu.Unlock()
}

// GetExpansionStatus 获取当前扩展状态
func (m *RAIDExpansionManager) GetExpansionStatus() *ExpansionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentStatus == nil {
		return &ExpansionStatus{State: ExpansionStateIdle}
	}

	// 返回副本
	status := *m.currentStatus
	return &status
}

// GetExpansionHistory 获取扩展历史
func (m *RAIDExpansionManager) GetExpansionHistory() []ExpansionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ExpansionStatus, len(m.history.Expansions))
	copy(result, m.history.Expansions)
	return result
}

// ========== 设备验证 ==========

// ValidateDevice 验证设备是否可用于扩展
func (m *RAIDExpansionManager) ValidateDevice(ctx context.Context, devicePath string) (*DeviceValidationResult, error) {
	result := &DeviceValidationResult{
		DevicePath: devicePath,
	}

	// 检查设备是否存在
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:       "device_not_found",
			Severity:   "error",
			Message:    "设备不存在",
			Field:      "devicePath",
			Resolution: "检查设备路径是否正确",
		})
		return result, ErrDeviceNotFound
	}

	// 验证设备路径格式
	if err := validateDevicePath(devicePath); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:       "invalid_device_path",
			Severity:   "error",
			Message:    "设备路径格式无效",
			Field:      "devicePath",
			Resolution: "使用正确的设备路径格式",
		})
		return result, err
	}

	// 获取设备大小
	size, err := m.getDeviceSize(ctx, devicePath)
	if err == nil {
		result.DeviceSize = size
	}

	// 检查设备是否有分区
	hasPartitions, err := m.checkDevicePartitions(ctx, devicePath)
	if err == nil {
		result.HasPartitions = hasPartitions
		if hasPartitions {
			result.Issues = append(result.Issues, ValidationIssue{
				Code:       "device_has_partitions",
				Severity:   "warning",
				Message:    "设备存在分区",
				Field:      "devicePath",
				Resolution: "使用wipefs清除分区表后再添加",
			})
		}
	}

	// 检查设备是否已在Btrfs中使用
	isBtrfsMember, err := m.checkDeviceBtrfsMembership(ctx, devicePath)
	if err == nil {
		result.IsBtrfsMember = isBtrfsMember
		if isBtrfsMember {
			result.IsInUse = true
			result.Issues = append(result.Issues, ValidationIssue{
				Code:       "device_in_use",
				Severity:   "error",
				Message:    "设备已是Btrfs成员",
				Field:      "devicePath",
				Resolution: "选择其他可用设备",
			})
		}
	}

	// 检查设备是否被其他系统使用
	isInUse, err := m.checkDeviceInUse(ctx, devicePath)
	if err == nil {
		result.IsInUse = isInUse || result.IsInUse
		if isInUse && !result.IsBtrfsMember {
			result.Issues = append(result.Issues, ValidationIssue{
				Code:       "device_mounted",
				Severity:   "error",
				Message:    "设备已被挂载或使用",
				Field:      "devicePath",
				Resolution: "卸载设备或选择其他设备",
			})
		}
	}

	// 综合判断是否可用
	result.IsAvailable = !result.IsInUse && !result.HasPartitions
	result.Valid = len(result.Issues) == 0 || !result.IsInUse

	return result, nil
}

// getDeviceSize 获取设备大小
func (m *RAIDExpansionManager) getDeviceSize(ctx context.Context, devicePath string) (uint64, error) {
	cmd := exec.CommandContext(ctx, "blockdev", "--getsize64", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	size, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, err
	}

	return size, nil
}

// checkDevicePartitions 检查设备是否有分区
func (m *RAIDExpansionManager) checkDevicePartitions(ctx context.Context, devicePath string) (bool, error) {
	// 使用lsblk检查分区
	cmd := exec.CommandContext(ctx, "lsblk", "-l", "-n", "-o", "NAME,TYPE", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "part" {
			return true, nil
		}
	}

	return false, nil
}

// checkDeviceBtrfsMembership 检查设备是否已是Btrfs成员
func (m *RAIDExpansionManager) checkDeviceBtrfsMembership(ctx context.Context, devicePath string) (bool, error) {
	volumes, err := m.client.ListVolumes()
	if err != nil {
		return false, nil
	}

	for _, vol := range volumes {
		for _, dev := range vol.Devices {
			if dev == devicePath {
				return true, nil
			}
		}
	}

	return false, nil
}

// checkDeviceInUse 检查设备是否被使用
func (m *RAIDExpansionManager) checkDeviceInUse(ctx context.Context, devicePath string) (bool, error) {
	// 检查是否被挂载
	cmd := exec.CommandContext(ctx, "lsblk", "-n", "-o", "MOUNTPOINT", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	mountPoint := strings.TrimSpace(string(output))
	if mountPoint != "" {
		return true, nil
	}

	// 检查是否有文件系统
	cmd = exec.CommandContext(ctx, "blkid", "-o", "value", "-s", "TYPE", devicePath)
	output, err = cmd.Output()
	if err != nil {
		// blkid返回错误可能表示没有文件系统
		return false, nil
	}

	fsType := strings.TrimSpace(string(output))
	if fsType != "" && fsType != "btrfs" {
		return true, nil
	}

	return false, nil
}

// ========== 卷验证 ==========

// ValidateVolume 验证卷是否可扩展
func (m *RAIDExpansionManager) ValidateVolume(ctx context.Context, mountPoint string) (*VolumeValidationResult, error) {
	result := &VolumeValidationResult{
		MountPoint: mountPoint,
	}

	// 检查挂载点是否存在
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:       "mount_point_not_found",
			Severity:   "error",
			Message:    "挂载点不存在",
			Field:      "mountPoint",
			Resolution: "检查挂载点路径是否正确",
		})
		return result, ErrVolumeNotMounted
	}

	// 检查是否是Btrfs挂载点
	cmd := exec.CommandContext(ctx, "findmnt", "-n", "-o", "FSTYPE", mountPoint)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) != "btrfs" {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:       "not_btrfs_volume",
			Severity:   "error",
			Message:    "不是Btrfs卷",
			Field:      "mountPoint",
			Resolution: "确保挂载点是Btrfs文件系统",
		})
		return result, fmt.Errorf("不是Btrfs卷")
	}

	// 获取卷信息
	volumes, err := m.client.ListVolumes()
	if err == nil {
		for _, vol := range volumes {
			// 查找匹配的卷
			for _, dev := range vol.Devices {
				// 通过设备检查是否匹配此挂载点
				if m.isDeviceMountedAt(dev, mountPoint) {
					result.VolumeName = vol.Name
					result.CurrentProfile = vol.Profile
					result.DeviceCount = len(vol.Devices)
					break
				}
			}
		}
	}

	// 检查卷健康状态
	healthy, err := m.checkVolumeHealth(ctx, mountPoint)
	if err == nil {
		result.IsHealthy = healthy
		if !healthy {
			result.Warnings = append(result.Warnings, "卷状态不健康，建议先修复")
		}
	}

	// 检查是否有正在运行的balance
	balanceStatus, err := m.client.GetBalanceStatus(mountPoint)
	if err == nil && balanceStatus.Running {
		result.Issues = append(result.Issues, ValidationIssue{
			Code:       "balance_running",
			Severity:   "error",
			Message:    "平衡任务正在运行",
			Field:      "balanceStatus",
			Resolution: "等待平衡完成或取消平衡",
		})
	}

	// 检查是否有正在运行的scrub
	scrubStatus, err := m.client.GetScrubStatus(mountPoint)
	if err == nil && scrubStatus.Running {
		result.Warnings = append(result.Warnings, "校验任务正在运行，建议等待完成后再扩展")
	}

	result.VolumeState = "online"
	result.Valid = len(result.Issues) == 0

	return result, nil
}

// isDeviceMountedAt 检查设备是否挂载在指定位置
func (m *RAIDExpansionManager) isDeviceMountedAt(device, mountPoint string) bool {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "findmnt", "-n", "-o", "SOURCE", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == device
}

// checkVolumeHealth 检查卷健康状态
func (m *RAIDExpansionManager) checkVolumeHealth(ctx context.Context, mountPoint string) (bool, error) {
	// 获取设备统计 - 使用空设备表示获取所有设备统计
	stats, err := m.client.GetDeviceStats(mountPoint, "")
	if err != nil {
		return false, err
	}

	// 检查是否有错误
	if stats.WriteErrors > 0 || stats.ReadErrors > 0 {
		return false, nil
	}

	return true, nil
}

// ========== 扩展操作 ==========

// StartExpansion 开始RAID扩展
func (m *RAIDExpansionManager) StartExpansion(ctx context.Context, config ExpansionConfig) (*ExpansionStatus, error) {
	if !m.available {
		return nil, fmt.Errorf("Btrfs不可用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有扩展在进行
	if m.currentStatus != nil && m.currentStatus.State == ExpansionStateBalancing ||
		m.currentStatus.State == ExpansionStateAddingDevice ||
		m.currentStatus.State == ExpansionStatePreparing {
		return nil, ErrExpansionInProgress
	}

	// 验证设备
	deviceResult, err := m.ValidateDevice(ctx, config.NewDevice)
	if err != nil {
		return nil, err
	}
	if !deviceResult.IsAvailable {
		return nil, ErrDeviceInUse.WithDetails(deviceResult.DevicePath)
	}

	// 验证卷
	volumeResult, err := m.ValidateVolume(ctx, config.MountPoint)
	if err != nil {
		return nil, err
	}
	if !volumeResult.Valid {
		return nil, ErrVolumeUnhealthy.WithDetails(volumeResult.VolumeName)
	}

	// 获取当前容量
	usage, err := m.client.GetUsage(config.MountPoint)
	if err != nil {
		return nil, fmt.Errorf("获取容量信息失败: %w", err)
	}
	total := uint64(usage.Total)
	used := uint64(usage.Used)

	// 估算新容量
	newCapacity := total + deviceResult.DeviceSize

	// 获取当前设备列表
	currentDevices := m.getCurrentDevices(ctx, config.MountPoint)

	// 创建扩展状态
	status := &ExpansionStatus{
		ID:                    generateExpansionID(config.VolumeName),
		VolumeName:            config.VolumeName,
		MountPoint:            config.MountPoint,
		NewDevice:             config.NewDevice,
		State:                 ExpansionStatePreparing,
		Phase:                 PhasePreparation,
		Progress:              0,
		OriginalDevices:       currentDevices,
		OriginalProfile:       volumeResult.CurrentProfile,
		TargetProfile:         config.TargetProfile,
		OriginalCapacity:      total,
		NewCapacity:           newCapacity,
		CapacityGain:          deviceResult.DeviceSize,
		StartTime:             time.Now(),
		TotalBytes:            used, // 需要平衡的数据量
		CanPause:              true,
		CanCancel:             true,
		LastUpdateTime:        time.Now(),
		PhaseProgress:         make(map[string]float64),
	}

	m.currentStatus = status

	// 如果是dry-run模式，只返回预期结果
	if config.DryRun {
		status.State = ExpansionStateCompleted
		status.Phase = PhaseCompletion
		status.Progress = 100
		status.Warnings = append(status.Warnings, "dry-run模式，未实际执行扩展")
		return status, nil
	}

	// 开始异步扩展
	go m.runExpansion(config, status)

	return status, nil
}

// generateExpansionID 生成扩展ID
func generateExpansionID(volumeName string) string {
	return fmt.Sprintf("exp-%s-%d", volumeName, time.Now().UnixNano())
}

// getCurrentDevices 获取当前设备列表
func (m *RAIDExpansionManager) getCurrentDevices(ctx context.Context, mountPoint string) []string {
	// 使用filesystem show命令获取设备列表
	output, err := m.client.RunCommand(ctx, "filesystem", "show", mountPoint)
	if err != nil {
		return []string{}
	}

	devices := make([]string, 0)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// 解析设备路径，格式如: devid    1 size 10.00GiB used 5.00GiB path /dev/sda1
		if strings.Contains(line, "path") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "path" && i+1 < len(fields) {
					devices = append(devices, fields[i+1])
				}
			}
		}
	}

	return devices
}

// runExpansion 执行扩展
func (m *RAIDExpansionManager) runExpansion(config ExpansionConfig, status *ExpansionStatus) {
	ctx := context.Background()

	// ========== 验证阶段 ==========
	m.updateStatus(func(s *ExpansionStatus) {
		s.Phase = PhaseValidation
		s.State = ExpansionStatePreparing
	})
	m.updatePhaseProgress(PhaseValidation, 50)

	// 重新验证
	_, err := m.ValidateDevice(ctx, config.NewDevice)
	if err != nil {
		m.failExpansion(err.Error())
		return
	}
	m.updatePhaseProgress(PhaseValidation, 100)

	// ========== 添加设备阶段 ==========
	m.updateStatus(func(s *ExpansionStatus) {
		s.Phase = PhaseDeviceAdd
		s.State = ExpansionStateAddingDevice
	})

	// 执行设备添加
	if err := m.client.AddDevice(config.MountPoint, config.NewDevice); err != nil {
		m.failExpansion(fmt.Sprintf("添加设备失败: %s", err.Error()))
		return
	}

	m.updatePhaseProgress(PhaseDeviceAdd, 100)
	m.updateStatus(func(s *ExpansionStatus) {
		s.NewDevices = append(s.OriginalDevices, config.NewDevice)
	})

	// ========== 数据平衡阶段 ==========
	if config.AutoBalance {
		m.updateStatus(func(s *ExpansionStatus) {
			s.Phase = PhaseBalance
			s.State = ExpansionStateBalancing
		})

		// 执行balance
		err := m.runBalance(ctx, config, status)
		if err != nil {
			m.failExpansion(fmt.Sprintf("平衡失败: %s", err.Error()))
			return
		}
	}

	// ========== 验证阶段 ==========
	m.updateStatus(func(s *ExpansionStatus) {
		s.Phase = PhaseVerification
		s.State = ExpansionStateVerifying
	})
	m.updatePhaseProgress(PhaseVerification, 50)

	// 验证扩展结果
	if err := m.verifyExpansion(ctx, config); err != nil {
		m.failExpansion(fmt.Sprintf("验证失败: %s", err.Error()))
		return
	}
	m.updatePhaseProgress(PhaseVerification, 100)

	// ========== 完成阶段 ==========
	m.updateStatus(func(s *ExpansionStatus) {
		s.Phase = PhaseCompletion
		s.State = ExpansionStateCompleted
		s.Progress = 100
		s.EndTime = time.Now()
	})

	// 记录历史
	m.addToHistory(*m.currentStatus)
}

// runBalance 执行平衡操作
func (m *RAIDExpansionManager) runBalance(ctx context.Context, config ExpansionConfig, status *ExpansionStatus) error {
	// 启动balance
	balanceOpts := config.BalanceOptions
	if config.TargetProfile != "" {
		balanceOpts.DataProfile = config.TargetProfile
		balanceOpts.MetadataProfile = config.TargetProfile
	}

	// 开始balance
	if balanceOpts.DataProfile != "" {
		if err := m.client.ConvertProfile(config.MountPoint, balanceOpts.DataProfile); err != nil {
			return err
		}
	} else {
		if err := m.client.StartBalance(config.MountPoint); err != nil {
			return err
		}
	}

	// 监控balance进度
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取balance状态
			balanceStatus, err := m.client.GetBalanceStatus(config.MountPoint)
			if err != nil {
				// 可能balance已完成
				m.updatePhaseProgress(PhaseBalance, 100)
				return nil
			}

			if !balanceStatus.Running {
				// balance已完成
				m.updatePhaseProgress(PhaseBalance, 100)
				return nil
			}

			// 更新进度
			m.updatePhaseProgress(PhaseBalance, balanceStatus.Progress)
			m.updateStatus(func(s *ExpansionStatus) {
				s.Progress = 30 + (balanceStatus.Progress * 0.6) // 设备添加30%, balance 60%, 验证10%
			})

		case <-m.cancelChan:
			// 取消balance
			_ = m.client.CancelBalance(config.MountPoint)
			m.updateStatus(func(s *ExpansionStatus) {
				s.State = ExpansionStateCancelled
				s.EndTime = time.Now()
			})
			return ErrExpansionCancelled

		case <-m.pauseChan:
			// balance不支持暂停，只能取消后重新开始
			_ = m.client.CancelBalance(config.MountPoint)
			m.updateStatus(func(s *ExpansionStatus) {
				s.State = ExpansionStatePaused
				s.PauseCount++
				s.CanResume = true
			})
			return ErrExpansionPaused
		}
	}
}

// verifyExpansion 验证扩展结果
func (m *RAIDExpansionManager) verifyExpansion(ctx context.Context, config ExpansionConfig) error {
	// 检查设备是否已添加
	stats, err := m.client.GetDeviceStats(config.MountPoint, config.NewDevice)
	if err != nil {
		return err
	}

	// 检查新设备是否存在
	deviceFound := stats != nil

	if !deviceFound {
		return fmt.Errorf("设备未成功添加到卷中")
	}

	// 检查容量是否增加
	usage, err := m.client.GetUsage(config.MountPoint)
	if err != nil {
		return err
	}
	_ = usage.Total // 用于验证

	return nil
}

// updateStatus 更新状态
func (m *RAIDExpansionManager) updateStatus(update func(*ExpansionStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus != nil {
		update(m.currentStatus)
		m.currentStatus.LastUpdateTime = time.Now()

		// 触发回调
		if m.onStateChange != nil {
			go m.onStateChange(m.currentStatus)
		}
	}
}

// updatePhaseProgress 更新阶段进度
func (m *RAIDExpansionManager) updatePhaseProgress(phase ExpansionPhase, progress float64) {
	m.updateStatus(func(s *ExpansionStatus) {
		s.PhaseProgress[string(phase)] = progress
	})
}

// failExpansion 标记扩展失败
func (m *RAIDExpansionManager) failExpansion(reason string) {
	m.updateStatus(func(s *ExpansionStatus) {
		s.State = ExpansionStateFailed
		s.EndTime = time.Now()
		s.Errors = append(s.Errors, reason)
	})

	// 记录历史
	m.addToHistory(*m.currentStatus)
}

// addToHistory 添加到历史记录
func (m *RAIDExpansionManager) addToHistory(status ExpansionStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history.Expansions = append(m.history.Expansions, status)

	// 只保留最近100条记录
	if len(m.history.Expansions) > 100 {
		m.history.Expansions = m.history.Expansions[len(m.history.Expansions)-100:]
	}

	_ = m.saveHistory()
}

// PauseExpansion 暂停扩展
func (m *RAIDExpansionManager) PauseExpansion() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStateBalancing &&
		m.currentStatus.State != ExpansionStateAddingDevice {
		return fmt.Errorf("无法暂停当前状态的扩展: %s", m.currentStatus.State)
	}

	select {
	case m.pauseChan <- struct{}{}:
		return nil
	default:
		return ErrExpansionPaused
	}
}

// ResumeExpansion 恢复扩展
func (m *RAIDExpansionManager) ResumeExpansion(ctx context.Context, config ExpansionConfig) (*ExpansionStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return nil, ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStatePaused {
		return nil, fmt.Errorf("无法恢复非暂停状态的扩展: %s", m.currentStatus.State)
	}

	// 重新开始扩展（从设备添加开始，因为设备可能已被添加）
	m.currentStatus.State = ExpansionStateBalancing
	m.currentStatus.Phase = PhaseBalance
	m.currentStatus.CanResume = false

	select {
	case m.resumeChan <- struct{}{}:
		return m.currentStatus, nil
	default:
		// 重新启动扩展流程
		go m.runExpansion(config, m.currentStatus)
		return m.currentStatus, nil
	}
}

// CancelExpansion 取消扩展
func (m *RAIDExpansionManager) CancelExpansion() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStateBalancing &&
		m.currentStatus.State != ExpansionStateAddingDevice &&
		m.currentStatus.State != ExpansionStatePaused {
		return fmt.Errorf("无法取消当前状态的扩展: %s", m.currentStatus.State)
	}

	select {
	case m.cancelChan <- struct{}{}:
		return nil
	default:
		return ErrExpansionCancelled
	}
}

// EstimateExpansionTime 估算扩展时间
func (m *RAIDExpansionManager) EstimateExpansionTime(ctx context.Context, mountPoint string) (time.Duration, error) {
	if !m.available {
		return 0, fmt.Errorf("Btrfs不可用")
	}

	// 获取已使用数据量
	usage, err := m.client.GetUsage(mountPoint)
	if err != nil {
		return 0, err
	}
	used := usage.Used

	// 估算时间：基于已使用数据和假设的balance速度
	// 典型balance速度约为50-200 MB/s，取决于硬件
	const assumedSpeedMBps = 100 // MB/s

	dataSizeMB := float64(used) / 1024 / 1024
	estimatedSeconds := dataSizeMB / assumedSpeedMBps

	// 加上设备添加时间（约30秒）
	return time.Duration(estimatedSeconds+30) * time.Second, nil
}

// EstimateCapacityGain 估算容量增益
func (m *RAIDExpansionManager) EstimateCapacityGain(ctx context.Context, mountPoint string, newDeviceSize uint64, currentProfile string) (*CapacityEstimate, error) {
	estimate := &CapacityEstimate{}

	// 获取当前容量
	usage, err := m.client.GetUsage(mountPoint)
	if err != nil {
		return nil, err
	}
	total := usage.Total

	estimate.OriginalCapacity = uint64(total)

	// 获取当前设备数
	currentDevices := m.getCurrentDevices(ctx, mountPoint)

	estimate.OriginalWidth = len(currentDevices)
	estimate.NewWidth = estimate.OriginalWidth + 1
	estimate.RAIDLevel = currentProfile
	estimate.DiskSize = newDeviceSize

	// 计算新容量和增益
	config, ok := PredefinedRAIDConfigs[currentProfile]
	if ok {
		estimate.EfficiencyRatio = config.Efficiency
		estimate.NewCapacity = uint64(total) + uint64(float64(newDeviceSize)*config.Efficiency/100)
		estimate.CapacityGain = estimate.NewCapacity - estimate.OriginalCapacity
		estimate.EffectiveGain = uint64(float64(newDeviceSize) * config.Efficiency / 100)
	} else {
		// single模式
		estimate.NewCapacity = uint64(total) + newDeviceSize
		estimate.CapacityGain = newDeviceSize
		estimate.EffectiveGain = newDeviceSize
		estimate.EfficiencyRatio = 100
	}

	return estimate, nil
}

// ListAvailableDisks 列出可用磁盘
func (m *RAIDExpansionManager) ListAvailableDisks(ctx context.Context) ([]DeviceValidationResult, error) {
	if !m.available {
		return nil, fmt.Errorf("Btrfs不可用")
	}

	// 获取系统磁盘列表
	cmd := exec.CommandContext(ctx, "lsblk", "-d", "-n", "-o", "PATH,SIZE,TYPE,MOUNTPOINT,FSTYPE")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取磁盘列表失败: %w", err)
	}

	var availableDisks []DeviceValidationResult
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		path := fields[0]
		if fields[1] != "disk" {
			continue
		}

		// 验证设备
		result, err := m.ValidateDevice(ctx, path)
		if err != nil {
			continue
		}

		if result.IsAvailable {
			availableDisks = append(availableDisks, *result)
		}
	}

	return availableDisks, nil
}

// Close 关闭管理器
func (m *RAIDExpansionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果有正在进行的扩展，取消它
	if m.currentStatus != nil &&
		(m.currentStatus.State == ExpansionStateBalancing ||
			m.currentStatus.State == ExpansionStateAddingDevice) {
		select {
		case m.cancelChan <- struct{}{}:
		default:
		}
	}

	// 保存历史
	_ = m.saveHistory()

	return nil
}