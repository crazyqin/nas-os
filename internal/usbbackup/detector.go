// Package usbbackup 提供 USB 设备备份管理功能
package usbbackup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 设备检测器 ==========

// Detector USB 设备检测器.
type Detector struct {
	mu sync.RWMutex

	// devices 已发现设备 map[deviceID]*USBDevice
	devices map[string]*USBDevice

	// running 是否运行中
	running bool

	// ctx / cancel 用于停止监控
	ctx    context.Context
	cancel context.CancelFunc

	// eventCh 设备事件通道
	eventCh chan USBEvent

	// handlers 事件回调列表
	handlers []func(USBEvent)

	// scanInterval 扫描间隔
	scanInterval time.Duration
}

// NewDetector 创建设备检测器.
func NewDetector(scanInterval time.Duration) *Detector {
	ctx, cancel := context.WithCancel(context.Background())
	if scanInterval <= 0 {
		scanInterval = 5 * time.Second
	}
	return &Detector{
		devices:      make(map[string]*USBDevice),
		ctx:          ctx,
		cancel:       cancel,
		eventCh:      make(chan USBEvent, 100),
		handlers:     make([]func(USBEvent), 0),
		scanInterval: scanInterval,
	}
}

// Start 启动设备检测.
func (d *Detector) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	// 首次扫描
	d.scanDevices()

	// 启动事件处理
	go d.processEvents()

	// 启动定期扫描
	go d.monitorDevices()

	d.running = true
	return nil
}

// Stop 停止设备检测.
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	d.cancel()
	d.running = false
}

// IsRunning 是否运行中.
func (d *Detector) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// ListDevices 列出所有已检测到的设备.
func (d *Detector) ListDevices() []*USBDevice {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*USBDevice, 0, len(d.devices))
	for _, dev := range d.devices {
		result = append(result, dev)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConnectedAt.After(result[j].ConnectedAt)
	})
	return result
}

// GetDevice 获取指定设备.
func (d *Detector) GetDevice(id string) (*USBDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dev, ok := d.devices[id]
	if !ok {
		return nil, ErrDeviceNotConnected
	}
	return dev, nil
}

// OnEvent 注册设备事件回调.
func (d *Detector) OnEvent(handler func(USBEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handler)
}

// ========== 内部方法 ==========

// monitorDevices 定期扫描设备.
func (d *Detector) monitorDevices() {
	ticker := time.NewTicker(d.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.scanDevices()
		}
	}
}

// scanDevices 扫描 USB 存储设备.
func (d *Detector) scanDevices() {
	devices := d.detectUSBStorage()

	d.mu.Lock()
	defer d.mu.Unlock()

	currentIDs := make(map[string]bool)

	for _, dev := range devices {
		currentIDs[dev.ID] = true

		if _, exists := d.devices[dev.ID]; !exists {
			// 新设备
			d.devices[dev.ID] = dev
			d.emitEvent(USBEvent{
				Type:      USBEventDeviceConnected,
				Device:    dev,
				Timestamp: time.Now(),
			})
		}
	}

	// 检查已移除的设备
	for id, dev := range d.devices {
		if !currentIDs[id] {
			d.emitEvent(USBEvent{
				Type:      USBEventDeviceDisconnected,
				Device:    dev,
				Timestamp: time.Now(),
			})
			delete(d.devices, id)
		}
	}
}

// detectUSBStorage 检测 USB 存储设备.
func (d *Detector) detectUSBStorage() []*USBDevice {
	// 通过 lsblk 获取块设备信息
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lsblk", "-b", "-J",
		"-o", "NAME,PATH,LABEL,UUID,FSTYPE,SIZE,VENDOR,MODEL,SERIAL,MOUNTPOINT,HOTPLUG,TRAN")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return d.parseLSBLKOutput(output)
}

// lsblkJSON lsblk JSON 输出结构.
type lsblkJSON struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

// lsblkDevice lsblk 设备条目.
type lsblkDevice struct {
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	Label      string         `json:"label"`
	UUID       string         `json:"uuid"`
	FSType     string         `json:"fstype"`
	Size       int64          `json:"size"`
	Vendor     string         `json:"vendor"`
	Model      string         `json:"model"`
	Serial     string         `json:"serial"`
	Mountpoint string         `json:"mountpoint"`
	Hotplug    int            `json:"hotplug"`
	Tran       string         `json:"tran"`
	Children   []lsblkDevice `json:"children"`
}

// parseLSBLKOutput 解析 lsblk JSON 输出.
func (d *Detector) parseLSBLKOutput(output []byte) []*USBDevice {
	var parsed lsblkJSON
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil
	}

	var result []*USBDevice
	for _, dev := range parsed.BlockDevices {
		result = append(result, d.collectUSBDevices(dev)...)
	}
	return result
}

// collectUSBDevices 递归收集 USB 存储设备.
func (d *Detector) collectUSBDevices(dev lsblkDevice) []*USBDevice {
	var result []*USBDevice

	// 判断是否为 USB 传输协议
	isUSB := dev.Tran == "usb" || dev.Hotplug == 1

	if isUSB && dev.FSType != "" {
		usbDev := &USBDevice{
			ID:            generateDeviceID(dev.UUID, dev.Path),
			DevicePath:    dev.Path,
			Label:         dev.Label,
			UUID:          dev.UUID,
			FileSystem:    dev.FSType,
			TotalCapacity: dev.Size,
			MountPoint:    dev.Mountpoint,
			Vendor:        strings.TrimSpace(dev.Vendor),
			Model:         strings.TrimSpace(dev.Model),
			Serial:        dev.Serial,
			ConnectedAt:   time.Now(),
			Hotplug:       dev.Hotplug == 1,
		}
		result = append(result, usbDev)
	}

	// 递归处理子设备
	for _, child := range dev.Children {
		result = append(result, d.collectUSBDevices(child)...)
	}

	return result
}

// generateDeviceID 生成设备唯一 ID.
func generateDeviceID(fsUUID, devPath string) string {
	if fsUUID != "" {
		return "usb-" + fsUUID
	}
	return "usb-" + uuid.New().String()
}

// emitEvent 发送事件.
func (d *Detector) emitEvent(event USBEvent) {
	select {
	case d.eventCh <- event:
	default:
		// 通道满，丢弃
	}
}

// processEvents 处理事件.
func (d *Detector) processEvents() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case event := <-d.eventCh:
			d.mu.RLock()
			handlers := make([]func(USBEvent), len(d.handlers))
			copy(handlers, d.handlers)
			d.mu.RUnlock()

			for _, h := range handlers {
				h(event)
			}
		}
	}
}

// ========== udev 监听（可选增强） ==========

// UdevMonitor udev 事件监听器.
type UdevMonitor struct {
	ctx    context.Context
	cancel context.CancelFunc
	events chan USBEvent
}

// NewUdevMonitor 创建 udev 监听器.
func NewUdevMonitor() *UdevMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &UdevMonitor{
		ctx:    ctx,
		cancel: cancel,
		events: make(chan USBEvent, 50),
	}
}

// Start 启动 udev 事件监听.
func (u *UdevMonitor) Start() error {
	go u.monitor()
	return nil
}

// Stop 停止监听.
func (u *UdevMonitor) Stop() {
	u.cancel()
}

// Events 返回事件通道.
func (u *UdevMonitor) Events() <-chan USBEvent {
	return u.events
}

// monitor 使用 udevadm monitor 监听 USB 事件.
func (u *UdevMonitor) monitor() {
	cmd := exec.CommandContext(u.ctx, "udevadm", "monitor", "--udev",
		"--subsystem-match=block", "--property-match=DEVTYPE=disk")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	var currentAction, devPath string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if currentAction != "" && devPath != "" {
				u.handleUdevEvent(currentAction, devPath)
			}
			currentAction = ""
			devPath = ""
			continue
		}

		if strings.HasPrefix(line, "UDEV") || strings.HasPrefix(line, "KERNEL") {
			// 新事件行
			if currentAction != "" {
				u.handleUdevEvent(currentAction, devPath)
			}
			re := regexp.MustCompile(`\[(\w+)\].*block/(.*)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 3 {
				currentAction = matches[1]
				devPath = "/dev/" + filepath.Base(matches[2])
			} else {
				currentAction = ""
				devPath = ""
			}
			continue
		}

		if strings.HasPrefix(line, "ACTION=") {
			currentAction = strings.TrimPrefix(line, "ACTION=")
		}
		if strings.HasPrefix(line, "DEVNAME=") {
			devPath = strings.TrimPrefix(line, "DEVNAME=")
		}
	}

	_ = cmd.Wait()
}

// handleUdevEvent 处理单个 udev 事件.
func (u *UdevMonitor) handleUdevEvent(action, devPath string) {
	now := time.Now()

	switch action {
	case "add":
		dev := &USBDevice{
			ID:          "usb-" + filepath.Base(devPath),
			DevicePath:  devPath,
			ConnectedAt: now,
			Hotplug:     true,
		}
		// 尝试获取更多信息
		u.enrichDevice(dev)

		select {
		case u.events <- USBEvent{Type: USBEventDeviceConnected, Device: dev, Timestamp: now}:
		default:
		}

	case "remove":
		dev := &USBDevice{
			ID:         "usb-" + filepath.Base(devPath),
			DevicePath: devPath,
		}
		select {
		case u.events <- USBEvent{Type: USBEventDeviceDisconnected, Device: dev, Timestamp: now}:
		default:
		}
	}
}

// enrichDevice 通过 blkid 补充设备信息.
func (u *UdevMonitor) enrichDevice(dev *USBDevice) {
	ctx, cancel := context.WithTimeout(u.ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "blkid", "-o", "export", dev.DevicePath)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "UUID":
			dev.UUID = val
			if dev.ID == "usb-"+filepath.Base(dev.DevicePath) {
				dev.ID = "usb-" + val
			}
		case "LABEL":
			dev.Label = val
		case "TYPE":
			dev.FileSystem = val
		}
	}
}

// ========== 辅助函数 ==========

// ListBlockDevices 列出所有 USB 块设备（导出供测试用）.
func ListBlockDevices(ctx context.Context) ([]*USBDevice, error) {
	d := NewDetector(0)
	d.ctx = ctx
	devices := d.detectUSBStorage()
	if devices == nil {
		return nil, fmt.Errorf("failed to detect USB devices")
	}
	return devices, nil
}
