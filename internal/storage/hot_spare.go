// Package storage 提供 Hot Spare (热备盘) 管理功能
package storage

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"nas-os/pkg/btrfs"
)

// ========== Hot Spare 数据结构 ==========

// HotSpare 热备盘配置
type HotSpare struct {
	ID              string    `json:"id"`              // 热备盘标识
	Device          string    `json:"device"`          // 设备路径 (如 /dev/sdb)
	VolumeName      string    `json:"volumeName"`      // 关联的卷名 (空表示全局热备)
	Status          string    `json:"status"`          // available, standby, rebuilding, failed
	Capacity        uint64    `json:"capacity"`        // 容量 (字节)
	AddedAt         time.Time `json:"addedAt"`         // 添加时间
	LastCheckedAt   time.Time `json:"lastCheckedAt"`   // 最后检查时间
	RebuildProgress float64   `json:"rebuildProgress"` // 重建进度 (0-100)
	RebuildStarted  time.Time `json:"rebuildStarted"`  // 重建开始时间
	RebuildEnded    time.Time `json:"rebuildEnded"`    // 重建结束时间
	FailedDevice    string    `json:"failedDevice"`    // 替换的故障设备
	ErrorMessage    string    `json:"errorMessage"`    // 错误信息
}

// DeviceHealthStatus 设备健康状态
type DeviceHealthStatus struct {
	Device          string `json:"device"`          // 设备路径
	Healthy         bool   `json:"healthy"`         // 是否健康
	WriteErrors     uint64 `json:"writeErrors"`     // 写错误数
	ReadErrors      uint64 `json:"readErrors"`      // 读错误数
	IOErrors        uint64 `json:"ioErrors"`        // IO 错误数
	CorruptionErrors uint64 `json:"corruptionErrors"` // 数据损坏错误数
	Message         string `json:"message"`         // 状态消息
}

// ReplaceStatus 设备替换状态
type ReplaceStatus struct {
	Running        bool      `json:"running"`        // 是否正在运行
	Progress       float64   `json:"progress"`       // 进度 0-100
	SourceDevice   string    `json:"sourceDevice"`   // 源设备
	TargetDevice   string    `json:"targetDevice"`   // 目标设备
	StartTime      time.Time `json:"startTime"`      // 开始时间
	Finished       bool      `json:"finished"`       // 是否完成
	ErrorCode      int       `json:"errorCode"`      // 错误码 (0 表示成功)
}

// HotSpareConfig Hot Spare 配置
type HotSpareConfig struct {
	Enabled           bool          `json:"enabled"`           // 是否启用
	CheckInterval     time.Duration `json:"checkInterval"`     // 检查间隔 (默认 30s)
	AutoRebuild       bool          `json:"autoRebuild"`       // 自动重建
	NotifyOnStart     bool          `json:"notifyOnStart"`     // 重建开始通知
	NotifyOnComplete  bool          `json:"notifyOnComplete"`  // 重建完成通知
	NotifyOnFailure   bool          `json:"notifyOnFailure"`   // 重建失败通知
	RebuildTimeout    time.Duration `json:"rebuildTimeout"`    // 重建超时 (默认 24h)
	MinCapacityMargin int           `json:"minCapacityMargin"` // 最小容量余量百分比 (默认 5%)
}

// HotSpareEvent 热备盘事件
type HotSpareEvent struct {
	Type        string    `json:"type"`        // added, removed, activated, rebuild_start, rebuild_complete, rebuild_failed, error
	HotSpareID  string    `json:"hotSpareId"`  // 热备盘 ID
	Device      string    `json:"device"`      // 设备路径
	VolumeName  string    `json:"volumeName"`  // 卷名
	FailedDevice string   `json:"failedDevice"` // 故障设备 (重建事件)
	Message     string    `json:"message"`     // 事件消息
	Timestamp   time.Time `json:"timestamp"`   // 事件时间
}

// HotSpareStatus 热备盘系统状态
type HotSpareStatus struct {
	TotalHotSpares   int           `json:"totalHotSpares"`   // 总热备盘数
	AvailableCount   int           `json:"availableCount"`   // 可用数
	RebuildingCount  int           `json:"rebuildingCount"`  // 重建中数
	FailedCount      int           `json:"failedCount"`      // 失败数
	LastCheckTime    time.Time     `json:"lastCheckTime"`    // 最后检查时间
	Config           HotSpareConfig `json:"config"`          // 当前配置
}

// NotificationFunc 通知回调函数类型
type NotificationFunc func(event HotSpareEvent)

// ========== HotSpareManager 热备盘管理器 ==========

// HotSpareManager 热备盘管理器
type HotSpareManager struct {
	manager     *Manager
	client      *btrfs.Client
	hotSpares   map[string]*HotSpare // key: device path
	config      HotSpareConfig
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	notifyFunc  NotificationFunc
	eventChan   chan HotSpareEvent
}

// DefaultHotSpareConfig 默认配置
var DefaultHotSpareConfig = HotSpareConfig{
	Enabled:           true,
	CheckInterval:     30 * time.Second,
	AutoRebuild:       true,
	NotifyOnStart:     true,
	NotifyOnComplete:  true,
	NotifyOnFailure:   true,
	RebuildTimeout:    24 * time.Hour,
	MinCapacityMargin: 5,
}

// NewHotSpareManager 创建热备盘管理器
func NewHotSpareManager(manager *Manager) *HotSpareManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &HotSpareManager{
		manager:   manager,
		client:    btrfs.NewClient(true),
		hotSpares: make(map[string]*HotSpare),
		config:    DefaultHotSpareConfig,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan HotSpareEvent, 100),
	}
}

// SetConfig 设置配置
func (h *HotSpareManager) SetConfig(config HotSpareConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config = config
}

// GetConfig 获取配置
func (h *HotSpareManager) GetConfig() HotSpareConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// SetNotificationFunc 设置通知回调
func (h *HotSpareManager) SetNotificationFunc(fn NotificationFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notifyFunc = fn
}

// ========== 热备盘配置管理 ==========

// AddHotSpare 添加热备盘
func (h *HotSpareManager) AddHotSpare(device, volumeName string) (*HotSpare, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查是否已存在
	if _, exists := h.hotSpares[device]; exists {
		return nil, fmt.Errorf("热备盘 %s 已存在", device)
	}

	// 获取设备信息
	capacity, err := h.getDeviceCapacity(device)
	if err != nil {
		return nil, fmt.Errorf("获取设备容量失败: %w", err)
	}

	// 检查设备是否可用
	if err := h.checkDeviceAvailable(device); err != nil {
		return nil, fmt.Errorf("设备不可用: %w", err)
	}

	// 如果指定了卷，验证卷存在且 RAID 级别支持热备
	if volumeName != "" {
		vol := h.manager.GetVolume(volumeName)
		if vol == nil {
			return nil, fmt.Errorf("卷 %s 不存在", volumeName)
		}
		// 验证设备容量是否足够
		if err := h.validateCapacity(device, vol); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	hs := &HotSpare{
		ID:            generateID(device),
		Device:        device,
		VolumeName:    volumeName,
		Status:        "available",
		Capacity:      capacity,
		AddedAt:       now,
		LastCheckedAt: now,
	}

	h.hotSpares[device] = hs

	// 发送事件
	h.emitEvent(HotSpareEvent{
		Type:       "added",
		HotSpareID: hs.ID,
		Device:     device,
		VolumeName: volumeName,
		Message:    fmt.Sprintf("热备盘 %s 已添加", device),
		Timestamp:  now,
	})

	return hs, nil
}

// RemoveHotSpare 移除热备盘
func (h *HotSpareManager) RemoveHotSpare(device string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	hs, exists := h.hotSpares[device]
	if !exists {
		return fmt.Errorf("热备盘 %s 不存在", device)
	}

	// 检查状态
	if hs.Status == "rebuilding" {
		return fmt.Errorf("热备盘 %s 正在重建，无法移除", device)
	}

	delete(h.hotSpares, device)

	// 发送事件
	h.emitEvent(HotSpareEvent{
		Type:       "removed",
		HotSpareID: hs.ID,
		Device:     device,
		VolumeName: hs.VolumeName,
		Message:    fmt.Sprintf("热备盘 %s 已移除", device),
		Timestamp:  time.Now(),
	})

	return nil
}

// ListHotSpares 列出所有热备盘
func (h *HotSpareManager) ListHotSpares(volumeName string) []*HotSpare {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*HotSpare, 0)
	for _, hs := range h.hotSpares {
		if volumeName == "" || hs.VolumeName == volumeName || hs.VolumeName == "" {
			result = append(result, hs)
		}
	}
	return result
}

// GetHotSpare 获取热备盘详情
func (h *HotSpareManager) GetHotSpare(device string) (*HotSpare, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hs, exists := h.hotSpares[device]
	if !exists {
		return nil, fmt.Errorf("热备盘 %s 不存在", device)
	}
	return hs, nil
}

// GetStatus 获取热备盘系统状态
func (h *HotSpareManager) GetStatus() HotSpareStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var available, rebuilding, failed int
	for _, hs := range h.hotSpares {
		switch hs.Status {
		case "available", "standby":
			available++
		case "rebuilding":
			rebuilding++
		case "failed":
			failed++
		}
	}

	return HotSpareStatus{
		TotalHotSpares:  len(h.hotSpares),
		AvailableCount:  available,
		RebuildingCount: rebuilding,
		FailedCount:     failed,
		LastCheckTime:   time.Now(),
		Config:          h.config,
	}
}

// ========== 自动故障检测 ==========

// Start 启动故障检测
func (h *HotSpareManager) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return fmt.Errorf("热备盘管理器已在运行")
	}

	h.running = true
	go h.monitorLoop()
	go h.eventLoop()

	return nil
}

// Stop 停止故障检测
func (h *HotSpareManager) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.cancel()
	h.running = false
}

// monitorLoop 监控循环
func (h *HotSpareManager) monitorLoop() {
	ticker := time.NewTicker(h.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if h.config.Enabled {
				h.checkAllDevices()
			}
		}
	}
}

// eventLoop 事件处理循环
func (h *HotSpareManager) eventLoop() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case event := <-h.eventChan:
			if h.notifyFunc != nil {
				h.notifyFunc(event)
			}
		}
	}
}

// checkAllDevices 检查所有设备状态
func (h *HotSpareManager) checkAllDevices() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查每个卷的设备状态
	volumes := h.manager.ListVolumes()
	for _, vol := range volumes {
		if vol.MountPoint == "" {
			continue
		}

		// 获取设备统计
		stats, err := h.client.GetDeviceStats(vol.MountPoint)
		if err != nil {
			continue
		}

		// 检查每个设备
		for _, stat := range stats {
			// 获取设备健康状态
			health := h.checkDeviceHealth(stat.Device)
			
			// 检测故障
			if !health.Healthy {
				// 发送故障事件
				h.emitEventUnlocked(HotSpareEvent{
					Type:         "error",
					Device:       stat.Device,
					VolumeName:   vol.Name,
					FailedDevice: stat.Device,
					Message:      fmt.Sprintf("检测到设备故障: %s, %s", stat.Device, health.Message),
					Timestamp:    time.Now(),
				})

				// 自动重建
				if h.config.AutoRebuild {
					go h.handleDeviceFailure(vol.Name, stat.Device)
				}
			}
		}
	}

	// 更新热备盘最后检查时间
	for _, hs := range h.hotSpares {
		hs.LastCheckedAt = time.Now()
	}
}

// checkDeviceHealth 检查设备健康状态
func (h *HotSpareManager) checkDeviceHealth(device string) DeviceHealthStatus {
	health := DeviceHealthStatus{
		Device:  device,
		Healthy: true,
	}

	// 使用 btrfs device stats 命令获取错误统计
	cmd := exec.Command("sudo", "btrfs", "device", "stats", device)
	output, err := cmd.Output()
	if err != nil {
		// 如果无法获取状态，假设设备有问题
		health.Healthy = false
		health.Message = fmt.Sprintf("无法获取设备状态: %v", err)
		return health
	}

	// 解析输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "write_io_errs") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[len(fields)-1], 10, 64); err == nil && val > 0 {
					health.WriteErrors = val
					health.Healthy = false
				}
			}
		} else if strings.Contains(line, "read_io_errs") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[len(fields)-1], 10, 64); err == nil && val > 0 {
					health.ReadErrors = val
					health.Healthy = false
				}
			}
		} else if strings.Contains(line, "corruption_errs") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[len(fields)-1], 10, 64); err == nil && val > 0 {
					health.CorruptionErrors = val
					health.Healthy = false
				}
			}
		} else if strings.Contains(line, "flush_io_errs") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[len(fields)-1], 10, 64); err == nil && val > 0 {
					health.IOErrors = val
					health.Healthy = false
				}
			}
		}
	}

	if !health.Healthy {
		health.Message = fmt.Sprintf("检测到错误: 写错误=%d, 读错误=%d, IO错误=%d, 损坏错误=%d",
			health.WriteErrors, health.ReadErrors, health.IOErrors, health.CorruptionErrors)
	}

	return health
}

// handleDeviceFailure 处理设备故障
func (h *HotSpareManager) handleDeviceFailure(volumeName, failedDevice string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 查找可用的热备盘
	hs := h.findAvailableHotSpare(volumeName)
	if hs == nil {
		h.emitEventUnlocked(HotSpareEvent{
			Type:        "error",
			VolumeName:  volumeName,
			FailedDevice: failedDevice,
			Message:     fmt.Sprintf("没有可用的热备盘替换故障设备 %s", failedDevice),
			Timestamp:   time.Now(),
		})
		return
	}

	// 开始重建
	if err := h.startRebuild(hs, volumeName, failedDevice); err != nil {
		hs.Status = "failed"
		hs.ErrorMessage = err.Error()
		h.emitEventUnlocked(HotSpareEvent{
			Type:        "rebuild_failed",
			HotSpareID:  hs.ID,
			Device:      hs.Device,
			VolumeName:  volumeName,
			FailedDevice: failedDevice,
			Message:     fmt.Sprintf("重建失败: %v", err),
			Timestamp:   time.Now(),
		})
		return
	}
}

// findAvailableHotSpare 查找可用的热备盘
func (h *HotSpareManager) findAvailableHotSpare(volumeName string) *HotSpare {
	// 优先查找专用于该卷的热备盘
	for _, hs := range h.hotSpares {
		if hs.Status == "available" && hs.VolumeName == volumeName {
			return hs
		}
	}

	// 其次查找全局热备盘
	for _, hs := range h.hotSpares {
		if hs.Status == "available" && hs.VolumeName == "" {
			return hs
		}
	}

	return nil
}

// ========== 自动重建逻辑 ==========

// startRebuild 开始重建
func (h *HotSpareManager) startRebuild(hs *HotSpare, volumeName, failedDevice string) error {
	vol := h.manager.GetVolume(volumeName)
	if vol == nil {
		return fmt.Errorf("卷 %s 不存在", volumeName)
	}

	if vol.MountPoint == "" {
		return fmt.Errorf("卷未挂载")
	}

	// 更新状态
	hs.Status = "rebuilding"
	hs.FailedDevice = failedDevice
	hs.RebuildStarted = time.Now()
	hs.RebuildProgress = 0

	// 发送重建开始事件
	h.emitEventUnlocked(HotSpareEvent{
		Type:        "rebuild_start",
		HotSpareID:  hs.ID,
		Device:      hs.Device,
		VolumeName:  volumeName,
		FailedDevice: failedDevice,
		Message:     fmt.Sprintf("开始使用热备盘 %s 替换故障设备 %s", hs.Device, failedDevice),
		Timestamp:   time.Now(),
	})

	// 执行替换
	if err := h.replaceDevice(vol.MountPoint, failedDevice, hs.Device); err != nil {
		return fmt.Errorf("替换设备失败: %w", err)
	}

	// 启动监控重建进度
	go h.monitorRebuildProgress(hs, volumeName)

	return nil
}

// monitorRebuildProgress 监控重建进度
func (h *HotSpareManager) monitorRebuildProgress(hs *HotSpare, volumeName string) {
	timeout := time.After(h.config.RebuildTimeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-timeout:
			h.mu.Lock()
			hs.Status = "failed"
			hs.ErrorMessage = "重建超时"
			hs.RebuildEnded = time.Now()
			h.emitEventUnlocked(HotSpareEvent{
				Type:        "rebuild_failed",
				HotSpareID:  hs.ID,
				Device:      hs.Device,
				VolumeName:  volumeName,
				FailedDevice: hs.FailedDevice,
				Message:     "重建超时",
				Timestamp:   time.Now(),
			})
			h.mu.Unlock()
			return
		case <-ticker.C:
			vol := h.manager.GetVolume(volumeName)
			if vol == nil || vol.MountPoint == "" {
				continue
			}

			// 获取重建状态
			status, err := h.getReplaceStatus(vol.MountPoint)
			if err != nil {
				continue
			}

			h.mu.Lock()
			if status.Finished {
				hs.Status = "standby" // 热备盘已成为活动盘，可以再次成为热备
				hs.RebuildProgress = 100
				hs.RebuildEnded = time.Now()
				
				// 更新卷的设备列表
				vol := h.manager.GetVolume(volumeName)
				if vol != nil {
					// 移除故障设备，添加热备盘设备
					for i, dev := range vol.Devices {
						if dev == hs.FailedDevice {
							vol.Devices[i] = hs.Device
							break
						}
					}
				}

				h.emitEventUnlocked(HotSpareEvent{
					Type:        "rebuild_complete",
					HotSpareID:  hs.ID,
					Device:      hs.Device,
					VolumeName:  volumeName,
					FailedDevice: hs.FailedDevice,
					Message:     fmt.Sprintf("重建完成，热备盘 %s 已替换故障设备", hs.Device),
					Timestamp:   time.Now(),
				})
				h.mu.Unlock()
				return
			}

			hs.RebuildProgress = status.Progress
			h.mu.Unlock()
		}
	}
}

// ========== 手动操作 ==========

// ActivateHotSpare 手动激活热备盘
func (h *HotSpareManager) ActivateHotSpare(device, volumeName, failedDevice string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	hs, exists := h.hotSpares[device]
	if !exists {
		return fmt.Errorf("热备盘 %s 不存在", device)
	}

	if hs.Status != "available" {
		return fmt.Errorf("热备盘状态为 %s，无法激活", hs.Status)
	}

	// 发送激活事件
	h.emitEventUnlocked(HotSpareEvent{
		Type:        "activated",
		HotSpareID:  hs.ID,
		Device:      device,
		VolumeName:  volumeName,
		FailedDevice: failedDevice,
		Message:     fmt.Sprintf("热备盘 %s 已手动激活", device),
		Timestamp:   time.Now(),
	})

	return h.startRebuild(hs, volumeName, failedDevice)
}

// CancelRebuild 取消重建
func (h *HotSpareManager) CancelRebuild(device string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	hs, exists := h.hotSpares[device]
	if !exists {
		return fmt.Errorf("热备盘 %s 不存在", device)
	}

	if hs.Status != "rebuilding" {
		return fmt.Errorf("热备盘不在重建状态")
	}

	vol := h.manager.GetVolume(hs.VolumeName)
	if vol == nil || vol.MountPoint == "" {
		return fmt.Errorf("卷不存在或未挂载")
	}

	// 取消替换
	if err := h.cancelReplace(vol.MountPoint); err != nil {
		return err
	}

	hs.Status = "available"
	hs.FailedDevice = ""
	hs.RebuildProgress = 0

	h.emitEventUnlocked(HotSpareEvent{
		Type:        "rebuild_cancelled",
		HotSpareID:  hs.ID,
		Device:      device,
		VolumeName:  hs.VolumeName,
		Message:     "重建已取消",
		Timestamp:   time.Now(),
	})

	return nil
}

// ========== 辅助方法 ==========

// getDeviceCapacity 获取设备容量
func (h *HotSpareManager) getDeviceCapacity(device string) (uint64, error) {
	// 使用 blockdev 命令获取设备大小
	cmd := exec.Command("sudo", "blockdev", "--getsize64", device)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取设备容量失败: %w", err)
	}
	
	size, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析设备容量失败: %w", err)
	}
	return size, nil
}

// checkDeviceAvailable 检查设备是否可用
func (h *HotSpareManager) checkDeviceAvailable(device string) error {
	// 检查设备是否存在
	cmd := exec.Command("test", "-b", device)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设备 %s 不存在或不是块设备", device)
	}
	
	// 检查设备是否已挂载
	cmd = exec.Command("grep", "-q", device, "/proc/mounts")
	if cmd.Run() == nil {
		return fmt.Errorf("设备 %s 已挂载", device)
	}
	
	return nil
}

// validateCapacity 验证设备容量是否足够
func (h *HotSpareManager) validateCapacity(device string, vol *Volume) error {
	capacity, err := h.getDeviceCapacity(device)
	if err != nil {
		return err
	}

	// 计算卷中最大设备的容量
	var maxDeviceSize uint64
	for _, dev := range vol.Devices {
		size, err := h.getDeviceCapacity(dev)
		if err == nil && size > maxDeviceSize {
			maxDeviceSize = size
		}
	}

	// 热备盘容量需要至少达到最大设备容量的 (100 - margin)%
	minRequired := maxDeviceSize * uint64(100-h.config.MinCapacityMargin) / 100
	if capacity < minRequired {
		return fmt.Errorf("热备盘容量 %d 字节小于最小要求 %d 字节", capacity, minRequired)
	}

	return nil
}

// replaceDevice 执行设备替换
func (h *HotSpareManager) replaceDevice(mountPoint, srcDevice, tgtDevice string) error {
	cmd := exec.Command("sudo", "btrfs", "replace", "start", srcDevice, tgtDevice, mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动设备替换失败: %w, output: %s", err, string(output))
	}
	return nil
}

// getReplaceStatus 获取设备替换状态
func (h *HotSpareManager) getReplaceStatus(mountPoint string) (*ReplaceStatus, error) {
	cmd := exec.Command("sudo", "btrfs", "replace", "status", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取替换状态失败: %w", err)
	}
	
	return h.parseReplaceStatus(string(output))
}

// parseReplaceStatus 解析替换状态输出
func (h *HotSpareManager) parseReplaceStatus(output string) (*ReplaceStatus, error) {
	status := &ReplaceStatus{}
	
	if strings.Contains(output, "never started") {
		return status, nil
	}
	
	if strings.Contains(output, "finished") {
		status.Finished = true
		status.Progress = 100
		return status, nil
	}
	
	if strings.Contains(output, "running") || strings.Contains(output, "started") {
		status.Running = true
		
		// 解析进度
		if idx := strings.Index(output, "%"); idx > 0 {
			percentStr := output[:idx]
			var numStr string
			for _, c := range percentStr {
				if c >= '0' && c <= '9' || c == '.' {
					numStr += string(c)
				}
			}
			if numStr != "" {
				if val, err := strconv.ParseFloat(numStr, 64); err == nil {
					status.Progress = val
				}
			}
		}
	}
	
	return status, nil
}

// cancelReplace 取消设备替换
func (h *HotSpareManager) cancelReplace(mountPoint string) error {
	cmd := exec.Command("sudo", "btrfs", "replace", "cancel", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("取消替换失败: %w, output: %s", err, string(output))
	}
	return nil
}

// emitEvent 发送事件 (需要持有锁)
func (h *HotSpareManager) emitEvent(event HotSpareEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emitEventUnlocked(event)
}

// emitEventUnlocked 发送事件 (无需持有锁)
func (h *HotSpareManager) emitEventUnlocked(event HotSpareEvent) {
	select {
	case h.eventChan <- event:
	default:
		// 事件队列满，丢弃
	}
}

// generateID 生成热备盘 ID
func generateID(device string) string {
	return fmt.Sprintf("hs-%d", time.Now().UnixNano())
}

// ========== 重建状态查询 ==========

// RebuildStatus 重建状态
type RebuildStatus struct {
	Device          string    `json:"device"`          // 热备盘设备
	VolumeName      string    `json:"volumeName"`      // 卷名
	FailedDevice    string    `json:"failedDevice"`    // 故障设备
	Progress        float64   `json:"progress"`        // 进度 0-100
	StartedAt       time.Time `json:"startedAt"`       // 开始时间
	EstimatedEnd    time.Time `json:"estimatedEnd"`    // 预计结束时间
	Status          string    `json:"status"`          // running, completed, failed, cancelled
	BytesRebuilt    uint64    `json:"bytesRebuilt"`    // 已重建字节数
	BytesTotal      uint64    `json:"bytesTotal"`      // 总字节数
}

// GetRebuildStatus 获取重建状态
func (h *HotSpareManager) GetRebuildStatus(device string) (*RebuildStatus, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hs, exists := h.hotSpares[device]
	if !exists {
		return nil, fmt.Errorf("热备盘 %s 不存在", device)
	}

	status := &RebuildStatus{
		Device:       device,
		VolumeName:   hs.VolumeName,
		FailedDevice: hs.FailedDevice,
		Progress:     hs.RebuildProgress,
		StartedAt:    hs.RebuildStarted,
		Status:       hs.Status,
	}

	// 计算预计结束时间
	if hs.Status == "rebuilding" && hs.RebuildProgress > 0 {
		elapsed := time.Since(hs.RebuildStarted)
		totalEstimate := time.Duration(float64(elapsed) / hs.RebuildProgress * 100)
		status.EstimatedEnd = hs.RebuildStarted.Add(totalEstimate)
	}

	return status, nil
}

// ListRebuilding 列出正在重建的热备盘
func (h *HotSpareManager) ListRebuilding() []*RebuildStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*RebuildStatus, 0)
	for _, hs := range h.hotSpares {
		if hs.Status == "rebuilding" {
			result = append(result, &RebuildStatus{
				Device:       hs.Device,
				VolumeName:   hs.VolumeName,
				FailedDevice: hs.FailedDevice,
				Progress:     hs.RebuildProgress,
				StartedAt:    hs.RebuildStarted,
				Status:       hs.Status,
			})
		}
	}
	return result
}