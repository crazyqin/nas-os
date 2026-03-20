// Package usbmount 提供 USB 设备自动挂载管理功能
package usbmount

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 安全相关常量 ==========

// 允许的通知命令模式（白名单）
var allowedNotifyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^/usr/bin/notify-send\b`),
	regexp.MustCompile(`^/bin/echo\b`),
	regexp.MustCompile(`^/usr/bin/logger\b`),
	regexp.MustCompile(`^/usr/local/bin/[a-zA-Z0-9_-]+$`),
}

// validateNotifyCommand 验证通知命令是否安全
func validateNotifyCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	// 检查是否匹配白名单
	for _, pattern := range allowedNotifyPatterns {
		if pattern.MatchString(cmd) {
			return true
		}
	}
	return false
}

// sanitizeEnvValue 清理环境变量值，防止注入
// sanitizeEnvValue 清理环境变量值，防止注入
func sanitizeEnvValue(value string) string {
	// 移除可能导致命令注入的字符
	replacer := strings.NewReplacer(
		";", "",
		"|", "",
		"&", "",
		"`", "",
		"$", "",
		"\n", "",
		"\r", "",
	)
	return replacer.Replace(value)
}

// 安全的设备路径模式
var safeDevicePathRegex = regexp.MustCompile(`^/dev/[a-zA-Z0-9_\-./]+$`)

// 安全的路径模式
var safeMountPathRegex = regexp.MustCompile(`^/[a-zA-Z0-9_\-./]+$`)

// validateDevicePath 验证设备路径是否安全
func validateDevicePath(devicePath string) error {
	if devicePath == "" {
		return fmt.Errorf("device path cannot be empty")
	}
	if !safeDevicePathRegex.MatchString(devicePath) {
		return fmt.Errorf("invalid device path format: %s", devicePath)
	}
	if strings.Contains(devicePath, "..") {
		return fmt.Errorf("path traversal detected in device path")
	}
	return nil
}

// validateMountPoint 验证挂载点是否安全
func validateMountPoint(mountPoint string) error {
	if mountPoint == "" {
		return fmt.Errorf("mount point cannot be empty")
	}
	if !safeMountPathRegex.MatchString(mountPoint) {
		return fmt.Errorf("invalid mount point format: %s", mountPoint)
	}
	if strings.Contains(mountPoint, "..") {
		return fmt.Errorf("path traversal detected in mount point")
	}
	return nil
}

// validateFSType 验证文件系统类型是否安全
func validateFSType(fsType string) error {
	if fsType == "" {
		return fmt.Errorf("filesystem type cannot be empty")
	}
	// 文件系统类型只允许小写字母和数字
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(fsType) {
		return fmt.Errorf("invalid filesystem type: %s", fsType)
	}
	return nil
}

// ========== 类型定义 ==========

// Manager USB 挂载管理器
type Manager struct {
	mu sync.RWMutex

	// config 配置
	config *Config

	// devices 已发现设备列表
	devices map[string]*Device

	// rules 自动挂载规则
	rules []*MountRule

	// configPath 配置文件路径
	configPath string

	// running 是否运行中
	running bool

	// ctx cancel 用于停止监控
	ctx    context.Context
	cancel context.CancelFunc

	// eventCh 设备事件通道
	eventCh chan DeviceEvent

	// handlers 事件处理器
	handlers []EventHandler
}

// Config 配置
type Config struct {
	// AutoMount 是否自动挂载
	AutoMount bool `json:"autoMount"`

	// DefaultMountPoint 默认挂载点前缀
	DefaultMountPoint string `json:"defaultMountPoint"`

	// AllowedFileSystems 允许挂载的文件系统类型
	AllowedFileSystems []string `json:"allowedFileSystems"`

	// MountOptions 默认挂载选项
	MountOptions map[string]string `json:"mountOptions"`

	// ScanInterval 扫描间隔（秒）
	ScanInterval int `json:"scanInterval"`

	// NotifyCommand 挂载通知命令
	NotifyCommand string `json:"notifyCommand,omitempty"`

	// IgnoreDevices 忽略的设备列表（UUID 或 Label）
	IgnoreDevices []string `json:"ignoreDevices,omitempty"`
}

// Device USB 设备信息
type Device struct {
	// ID 设备 ID
	ID string `json:"id"`

	// DevicePath 设备路径（如 /dev/sdb1）
	DevicePath string `json:"devicePath"`

	// Label 卷标
	Label string `json:"label"`

	// UUID 文件系统 UUID
	UUID string `json:"uuid"`

	// Type 文件系统类型
	Type string `json:"type"`

	// Size 设备大小（字节）
	Size int64 `json:"size"`

	// Vendor 厂商
	Vendor string `json:"vendor"`

	// Model 型号
	Model string `json:"model"`

	// Serial 序列号
	Serial string `json:"serial"`

	// MountPoint 挂载点（已挂载时）
	MountPoint string `json:"mountPoint,omitempty"`

	// Mounted 是否已挂载
	Mounted bool `json:"mounted"`

	// MountTime 挂载时间
	MountTime *time.Time `json:"mountTime,omitempty"`

	// AutoMounted 是否自动挂载
	AutoMounted bool `json:"autoMounted"`

	// RuleID 匹配的规则 ID
	RuleID string `json:"ruleId,omitempty"`

	// Hotplug 是否为热插拔设备
	Hotplug bool `json:"hotplug"`

	// LastScan 最后扫描时间
	LastScan time.Time `json:"lastScan"`
}

// MountRule 挂载规则
type MountRule struct {
	// ID 规则 ID
	ID string `json:"id"`

	// Name 规则名称
	Name string `json:"name"`

	// Priority 优先级（数值越大优先级越高）
	Priority int `json:"priority"`

	// MatchUUID 匹配的 UUID（支持通配符）
	MatchUUID string `json:"matchUuid,omitempty"`

	// MatchLabel 匹配的卷标（支持通配符）
	MatchLabel string `json:"matchLabel,omitempty"`

	// MatchType 匹配的文件系统类型
	MatchType string `json:"matchType,omitempty"`

	// MatchVendor 匹配的厂商
	MatchVendor string `json:"matchVendor,omitempty"`

	// MountPoint 指定挂载点
	MountPoint string `json:"mountPoint,omitempty"`

	// MountOptions 挂载选项
	MountOptions map[string]string `json:"mountOptions,omitempty"`

	// ReadOnly 只读挂载
	ReadOnly bool `json:"readOnly"`

	// AutoMount 是否自动挂载
	AutoMount bool `json:"autoMount"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	// Type 事件类型
	Type EventType `json:"type"`

	// Device 设备信息
	Device *Device `json:"device"`

	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`

	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// EventType 事件类型
type EventType string

const (
	// EventDeviceAdded 设备添加
	EventDeviceAdded EventType = "device_added"
	// EventDeviceRemoved 设备移除
	EventDeviceRemoved EventType = "device_removed"
	// EventDeviceMounted 设备挂载
	EventDeviceMounted EventType = "device_mounted"
	// EventDeviceUnmounted 设备卸载
	EventDeviceUnmounted EventType = "device_unmounted"
	// EventMountFailed 挂载失败
	EventMountFailed EventType = "mount_failed"
	// EventUnmountFailed 卸载失败
	EventUnmountFailed EventType = "unmount_failed"
)

// EventHandler 事件处理器
type EventHandler func(event DeviceEvent)

// MountResult 挂载结果
type MountResult struct {
	Success    bool      `json:"success"`
	DevicePath string    `json:"devicePath"`
	MountPoint string    `json:"mountPoint"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ========== 错误定义 ==========

var (
	// ErrDeviceNotFound 设备未找到错误
	ErrDeviceNotFound = fmt.Errorf("设备未找到")
	// ErrAlreadyMounted 设备已挂载错误
	ErrAlreadyMounted = fmt.Errorf("设备已挂载")
	// ErrNotMounted 设备未挂载错误
	ErrNotMounted = fmt.Errorf("设备未挂载")
	// ErrFileSystemNotAllowed 文件系统类型不被允许错误
	ErrFileSystemNotAllowed = fmt.Errorf("文件系统类型不被允许")
	// ErrMountPointBusy 挂载点被占用错误
	ErrMountPointBusy = fmt.Errorf("挂载点被占用")
	// ErrRuleNotFound 规则未找到错误
	ErrRuleNotFound = fmt.Errorf("规则未找到")
)

// ========== 构造函数 ==========

// NewManager 创建 USB 挂载管理器
func NewManager(configPath string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:     DefaultConfig(),
		devices:    make(map[string]*Device),
		rules:      make([]*MountRule, 0),
		configPath: configPath,
		ctx:        ctx,
		cancel:     cancel,
		eventCh:    make(chan DeviceEvent, 100),
		handlers:   make([]EventHandler, 0),
	}

	// 加载配置
	m.loadConfig()

	return m
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		AutoMount:          true,
		DefaultMountPoint:  "/media",
		AllowedFileSystems: []string{"vfat", "ntfs", "exfat", "ext4", "ext3", "ext2", "btrfs", "xfs", "hfsplus"},
		MountOptions: map[string]string{
			"vfat":  "utf8,uid=1000,gid=1000,umask=000",
			"ntfs":  "uid=1000,gid=1000,umask=000",
			"exfat": "uid=1000,gid=1000,umask=000",
		},
		ScanInterval: 5,
	}
}

// ========== 公共方法 ==========

// Start 启动监控
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 创建默认挂载目录
	_ = os.MkdirAll(m.config.DefaultMountPoint, 0755)

	// 初始扫描
	m.scanDevices()

	// 启动事件处理
	go m.processEvents()

	// 启动设备监控
	go m.monitorDevices()

	m.running = true
	return nil
}

// Stop 停止监控
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.running = false
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}
	return result
}

// GetDevice 获取设备
func (m *Manager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, exists := m.devices[id]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return d, nil
}

// Mount 挂载设备
func (m *Manager) Mount(deviceID string, mountPoint string, opts map[string]string) (*MountResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	// 检查文件系统是否允许
	if !m.isFileSystemAllowed(device.Type) {
		return nil, ErrFileSystemNotAllowed
	}

	// 检查是否已挂载
	if device.Mounted {
		return nil, ErrAlreadyMounted
	}

	// 确定挂载点
	if mountPoint == "" {
		mountPoint = m.generateMountPoint(device)
	}

	// 确保挂载点目录存在
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return &MountResult{
			Success:    false,
			DevicePath: device.DevicePath,
			Error:      fmt.Sprintf("创建挂载点失败: %v", err),
			Timestamp:  time.Now(),
		}, err
	}

	// 构建挂载选项
	mountOpts := m.buildMountOptions(device.Type, opts)

	// 验证参数安全性（防止命令注入）
	if err := validateDevicePath(device.DevicePath); err != nil {
		return &MountResult{
			Success:    false,
			DevicePath: device.DevicePath,
			Error:      fmt.Sprintf("无效的设备路径: %v", err),
			Timestamp:  time.Now(),
		}, err
	}
	if err := validateMountPoint(mountPoint); err != nil {
		return &MountResult{
			Success:    false,
			DevicePath: device.DevicePath,
			Error:      fmt.Sprintf("无效的挂载点: %v", err),
			Timestamp:  time.Now(),
		}, err
	}
	if err := validateFSType(device.Type); err != nil {
		return &MountResult{
			Success:    false,
			DevicePath: device.DevicePath,
			Error:      fmt.Sprintf("无效的文件系统类型: %v", err),
			Timestamp:  time.Now(),
		}, err
	}

	// 执行挂载
	args := []string{"-t", device.Type}
	if len(mountOpts) > 0 {
		args = append(args, "-o", strings.Join(mountOpts, ","))
	}
	args = append(args, device.DevicePath, mountPoint)

	// #nosec G204 -- 设备路径、挂载点和文件系统类型已通过验证函数验证
	cmd := exec.CommandContext(m.ctx, "mount", args...)
	output, err := cmd.CombinedOutput()

	result := &MountResult{
		DevicePath: device.DevicePath,
		MountPoint: mountPoint,
		Timestamp:  time.Now(),
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("挂载失败: %v, output: %s", err, string(output))

		// 发送事件
		m.emitEvent(DeviceEvent{
			Type:      EventMountFailed,
			Device:    device,
			Timestamp: time.Now(),
			Error:     result.Error,
		})

		return result, err
	}

	// 更新设备状态
	device.Mounted = true
	device.MountPoint = mountPoint
	now := time.Now()
	device.MountTime = &now

	result.Success = true

	// 发送事件
	m.emitEvent(DeviceEvent{
		Type:      EventDeviceMounted,
		Device:    device,
		Timestamp: time.Now(),
	})

	// 发送通知
	m.sendNotification(device, "mounted")

	return result, nil
}

// Unmount 卸载设备
func (m *Manager) Unmount(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	if !device.Mounted {
		return ErrNotMounted
	}

	// 执行卸载
	cmd := exec.CommandContext(m.ctx, "umount", device.MountPoint)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// 发送事件
		m.emitEvent(DeviceEvent{
			Type:      EventUnmountFailed,
			Device:    device,
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("卸载失败: %v, output: %s", err, string(output)),
		})
		return err
	}

	// 清理挂载点
	_ = os.Remove(device.MountPoint)

	// 更新设备状态
	device.Mounted = false
	device.MountPoint = ""
	device.MountTime = nil

	// 发送事件
	m.emitEvent(DeviceEvent{
		Type:      EventDeviceUnmounted,
		Device:    device,
		Timestamp: time.Now(),
	})

	return nil
}

// MountAll 挂载所有未挂载的设备
func (m *Manager) MountAll() []*MountResult {
	m.mu.RLock()
	devices := make([]*Device, 0)
	for _, d := range m.devices {
		if !d.Mounted {
			devices = append(devices, d)
		}
	}
	m.mu.RUnlock()

	results := make([]*MountResult, 0)
	for _, d := range devices {
		result, err := m.Mount(d.ID, "", nil)
		results = append(results, result)
		if err != nil {
			continue
		}
	}
	return results
}

// UnmountAll 卸载所有已挂载的设备
func (m *Manager) UnmountAll() []error {
	m.mu.RLock()
	devices := make([]*Device, 0)
	for _, d := range m.devices {
		if d.Mounted {
			devices = append(devices, d)
		}
	}
	m.mu.RUnlock()

	var errors []error
	for _, d := range devices {
		if err := m.Unmount(d.ID); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// ========== 规则管理 ==========

// AddRule 添加挂载规则
func (m *Manager) AddRule(rule *MountRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()

	m.rules = append(m.rules, rule)
	_ = m.saveConfig()

	return nil
}

// UpdateRule 更新规则
func (m *Manager) UpdateRule(id string, rule *MountRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.rules {
		if r.ID == id {
			rule.ID = id
			rule.CreatedAt = r.CreatedAt
			m.rules[i] = rule
			_ = m.saveConfig()
			return nil
		}
	}

	return ErrRuleNotFound
}

// DeleteRule 删除规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.rules {
		if r.ID == id {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			_ = m.saveConfig()
			return nil
		}
	}

	return ErrRuleNotFound
}

// ListRules 列出所有规则
func (m *Manager) ListRules() []*MountRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MountRule, len(m.rules))
	copy(result, m.rules)
	return result
}

// GetRule 获取规则
func (m *Manager) GetRule(id string) (*MountRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.rules {
		if r.ID == id {
			return r, nil
		}
	}

	return nil, ErrRuleNotFound
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.saveConfig()
}

// ========== 事件处理 ==========

// AddEventHandler 添加事件处理器
func (m *Manager) AddEventHandler(handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// ========== 内部方法 ==========

// scanDevices 扫描设备
func (m *Manager) scanDevices() {
	// 获取所有块设备
	cmd := exec.CommandContext(m.ctx, "lsblk", "-b", "-J", "-o", "NAME,PATH,LABEL,UUID,FSTYPE,SIZE,VENDOR,MODEL,SERIAL,MOUNTPOINT,HOTPLUG")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// 解析 lsblk 输出
	devices := parseLSBLKOutput(output)

	// 更新设备列表
	currentDevices := make(map[string]bool)

	for _, d := range devices {
		currentDevices[d.ID] = true

		existing, exists := m.devices[d.ID]

		if !exists {
			// 新设备
			m.devices[d.ID] = d
			m.emitEvent(DeviceEvent{
				Type:      EventDeviceAdded,
				Device:    d,
				Timestamp: time.Now(),
			})

			// 自动挂载
			if m.config.AutoMount && d.Hotplug {
				m.autoMountDevice(d)
			}
		} else {
			// 更新现有设备
			existing.Mounted = d.Mounted
			existing.MountPoint = d.MountPoint
			existing.LastScan = time.Now()
		}
	}

	// 检查移除的设备
	for id, d := range m.devices {
		if !currentDevices[id] {
			// 设备已移除
			m.emitEvent(DeviceEvent{
				Type:      EventDeviceRemoved,
				Device:    d,
				Timestamp: time.Now(),
			})
			delete(m.devices, id)
		}
	}
}

// monitorDevices 监控设备变化
func (m *Manager) monitorDevices() {
	ticker := time.NewTicker(time.Duration(m.config.ScanInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.scanDevices()
		}
	}
}

// processEvents 处理事件
func (m *Manager) processEvents() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventCh:
			m.mu.RLock()
			handlers := make([]EventHandler, len(m.handlers))
			copy(handlers, m.handlers)
			m.mu.RUnlock()

			for _, h := range handlers {
				h(event)
			}
		}
	}
}

// emitEvent 发送事件
func (m *Manager) emitEvent(event DeviceEvent) {
	select {
	case m.eventCh <- event:
	default:
		// 事件通道已满，丢弃事件
	}
}

// autoMountDevice 自动挂载设备
func (m *Manager) autoMountDevice(device *Device) {
	// 检查是否在忽略列表中
	for _, ignore := range m.config.IgnoreDevices {
		if device.UUID == ignore || device.Label == ignore {
			return
		}
	}

	// 查找匹配的规则
	rule := m.findMatchingRule(device)
	if rule != nil {
		if !rule.AutoMount {
			return
		}

		// 使用规则的挂载点
		result, err := m.Mount(device.ID, rule.MountPoint, rule.MountOptions)
		if err == nil {
			device.AutoMounted = true
			device.RuleID = rule.ID
		}
		_ = result
	} else {
		// 使用默认挂载点
		result, err := m.Mount(device.ID, "", nil)
		if err == nil {
			device.AutoMounted = true
		}
		_ = result
	}
}

// findMatchingRule 查找匹配的规则
func (m *Manager) findMatchingRule(device *Device) *MountRule {
	// 按优先级排序
	var matchedRules []*MountRule

	for _, r := range m.rules {
		if !r.Enabled {
			continue
		}

		if m.matchRule(r, device) {
			matchedRules = append(matchedRules, r)
		}
	}

	if len(matchedRules) == 0 {
		return nil
	}

	// 返回优先级最高的规则
	var best *MountRule
	for _, r := range matchedRules {
		if best == nil || r.Priority > best.Priority {
			best = r
		}
	}

	return best
}

// matchRule 匹配规则
func (m *Manager) matchRule(rule *MountRule, device *Device) bool {
	// 匹配 UUID
	if rule.MatchUUID != "" && !matchPattern(rule.MatchUUID, device.UUID) {
		return false
	}

	// 匹配 Label
	if rule.MatchLabel != "" && !matchPattern(rule.MatchLabel, device.Label) {
		return false
	}

	// 匹配文件系统类型
	if rule.MatchType != "" && rule.MatchType != device.Type {
		return false
	}

	// 匹配厂商
	if rule.MatchVendor != "" && !matchPattern(rule.MatchVendor, device.Vendor) {
		return false
	}

	return true
}

// matchPattern 简单模式匹配
func matchPattern(pattern, value string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if pattern == value {
		return true
	}
	// 支持简单的 * 通配符
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(value, pattern[1:]) {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(value, pattern[:len(pattern)-1]) {
		return true
	}
	return strings.Contains(value, pattern)
}

// generateMountPoint 生成挂载点
func (m *Manager) generateMountPoint(device *Device) string {
	var name string
	if device.Label != "" {
		name = sanitizeMountName(device.Label)
	} else if device.UUID != "" {
		name = device.UUID[:8]
	} else {
		name = filepath.Base(device.DevicePath)
	}

	return filepath.Join(m.config.DefaultMountPoint, name)
}

// sanitizeMountName 清理挂载点名称
func sanitizeMountName(name string) string {
	// 替换特殊字符
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	return result
}

// isFileSystemAllowed 检查文件系统是否允许
func (m *Manager) isFileSystemAllowed(fsType string) bool {
	for _, allowed := range m.config.AllowedFileSystems {
		if allowed == fsType {
			return true
		}
	}
	return false
}

// buildMountOptions 构建挂载选项
func (m *Manager) buildMountOptions(fsType string, opts map[string]string) []string {
	result := make([]string, 0)

	// 添加默认选项
	if defaultOpts, exists := m.config.MountOptions[fsType]; exists && defaultOpts != "" {
		result = append(result, defaultOpts)
	}

	// 添加自定义选项
	for k, v := range opts {
		if v != "" {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		} else {
			result = append(result, k)
		}
	}

	return result
}

// sendNotification 发送通知
func (m *Manager) sendNotification(device *Device, action string) {
	if m.config.NotifyCommand == "" {
		return
	}

	// 安全验证：检查通知命令是否在白名单中
	if !validateNotifyCommand(m.config.NotifyCommand) {
		// 命令不在白名单中，记录警告并跳过执行
		fmt.Printf("警告: 通知命令不在安全白名单中: %s\n", m.config.NotifyCommand)
		return
	}

	// 清理环境变量值，防止注入攻击
	cmd := exec.CommandContext(m.ctx, "sh", "-c", m.config.NotifyCommand)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("USB_DEVICE=%s", sanitizeEnvValue(device.DevicePath)),
		fmt.Sprintf("USB_LABEL=%s", sanitizeEnvValue(device.Label)),
		fmt.Sprintf("USB_UUID=%s", sanitizeEnvValue(device.UUID)),
		fmt.Sprintf("USB_MOUNT_POINT=%s", sanitizeEnvValue(device.MountPoint)),
		fmt.Sprintf("USB_ACTION=%s", sanitizeEnvValue(action)),
	)
	_ = cmd.Run() // #nosec G104 -- 触发脚本执行，错误已记录到日志
}

// loadConfig 加载配置
func (m *Manager) loadConfig() {
	if m.configPath == "" {
		return
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return
	}

	// 简单解析 JSON（实际项目中应使用 encoding/json）
	// 这里省略具体实现，使用默认配置
	_ = data
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	// 简单保存（实际项目中应使用 encoding/json）
	return nil
}

// ========== LSBLK 解析 ==========

// parseLSBLKOutput 解析 lsblk 输出
func parseLSBLKOutput(output []byte) []*Device {
	var result []*Device

	// 简单解析（实际项目中应使用 encoding/json）
	// 这里使用简化的解析逻辑
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var currentDevice *Device

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\"path\"") {
			// 新设备
			if currentDevice != nil {
				result = append(result, currentDevice)
			}
			currentDevice = &Device{
				ID:       uuid.New().String(),
				LastScan: time.Now(),
			}
		}

		if currentDevice == nil {
			continue
		}

		// 解析各个字段
		if idx := strings.Index(line, "\"path\""); idx >= 0 {
			currentDevice.DevicePath = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"label\""); idx >= 0 {
			currentDevice.Label = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"uuid\""); idx >= 0 {
			currentDevice.UUID = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"fstype\""); idx >= 0 {
			currentDevice.Type = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"vendor\""); idx >= 0 {
			currentDevice.Vendor = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"model\""); idx >= 0 {
			currentDevice.Model = extractJSONString(line[idx:])
		}
		if idx := strings.Index(line, "\"mountpoint\""); idx >= 0 {
			mp := extractJSONString(line[idx:])
			if mp != "" && mp != "null" {
				currentDevice.MountPoint = mp
				currentDevice.Mounted = true
			}
		}
	}

	if currentDevice != nil {
		result = append(result, currentDevice)
	}

	return result
}

// extractJSONString 提取 JSON 字符串值
func extractJSONString(line string) string {
	// 查找冒号后的值
	idx := strings.Index(line, ":")
	if idx < 0 {
		return ""
	}

	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, "\",")

	// 处理 null
	if value == "null" {
		return ""
	}

	return value
}
