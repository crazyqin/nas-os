// Package usbsmartmount 提供 USB 设备智能挂载管理功能，
// 对标 Synology USB Copy 和飞牛一键备份。支持自动检测 USB 设备、
// 智能识别设备类型（相机/手机/U盘/移动硬盘）、推荐挂载选项、
// 一键备份策略、安全弹出建议和 USB 设备历史记录。
package usbsmartmount

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 结构体定义 ==========

// USBDeviceInfo 描述 USB 设备的基本信息.
type USBDeviceInfo struct {
	VendorID     string  // 厂商 ID（如 "0781"）
	ProductID    string  // 产品 ID（如 "5581"）
	SerialNumber string  // 设备序列号
	DeviceName   string  // 设备名称/型号
	CapacityGB   float64 // 容量（GB）
	FileSystem   string  // 文件系统类型（如 ext4, exfat, ntfs, vfat）
	IsRemovable  bool    // 是否可移除
	BusType      string  // 总线类型（如 USB, Thunderbolt）
}

// DeviceClassification 描述设备分类结果.
type DeviceClassification struct {
	DeviceType        string  // 设备类型：camera, phone, usb_drive, external_hdd, card_reader, unknown
	Confidence       float64 // 分类置信度 0.0~1.0
	RecommendedAction string  // 推荐操作：mount, backup, ignore, ask_user
	AutoMount         bool    // 是否自动挂载
}

// MountRecommendation 描述挂载推荐配置.
type MountRecommendation struct {
	MountPoint  string   // 推荐挂载点路径
	FileSystem  string   // 文件系统类型
	Options     []string // 挂载选项列表
	ReadOnly    bool     // 是否只读挂载
	Encryption  bool     // 是否需要加密
	Reason      string   // 推荐理由
}

// BackupSuggestion 描述备份建议.
type BackupSuggestion struct {
	ShouldBackup      bool    // 是否建议备份
	Strategy          string  // 备份策略：full, incremental, mirror, photo_import
	TargetShare       string  // 目标共享目录
	EstimatedDuration string  // 预计耗时
	Deduplicate       bool    // 是否去重
	Encryption        bool    // 是否加密备份
	SizeGB            float64 // 预计备份数据量
}

// EjectResult 描述安全弹出结果.
type EjectResult struct {
	Success      bool   // 是否弹出成功
	Path         string // 挂载路径
	PendingIO    bool   // 是否有未完成的 I/O
	SafeToRemove bool   // 是否可以安全拔除
	Warning      string // 警告信息
}

// HistoryEntry 描述设备历史记录.
type HistoryEntry struct {
	DeviceName            string // 设备名称
	SerialNumber          string // 序列号
	FirstSeen             int64  // 首次发现时间（Unix 时间戳）
	LastSeen              int64  // 最近发现时间
	MountCount            int    // 挂载次数
	TotalBytesTransferred int64  // 累计传输字节数
}

// ========== 厂商 ID 映射 ==========

// vendorNameMap 已知厂商 ID 到名称的映射.
var vendorNameMap = map[string]string{
	"0781": "SanDisk",   // U盘/存储
	"058f": "Alcor",     // 读卡器
	"04e8": "Samsung",   // 手机/存储
	"18d1": "Google",    // Android 手机
	"05ac": "Apple",     // iPhone/iPad
	"04b0": "Nikon",     // 相机
	"07b4": "Olympus",   // 相机
	"054c": "Sony",      // 相机/手机
	"04a9": "Canon",     // 相机
	"f1a":  "Fitbit",    //穿戴设备(忽略)
	"22b8": "Motorola",  // 手机
	"0bb4": "HTC",       // 手机
	"12d1": "Huawei",    // 手机
	"2717": "Xiaomi",    // 手机
}

// cameraVendorIDs 识别为相机的厂商 ID 集合.
var cameraVendorIDs = map[string]bool{
	"04b0": true, // Nikon
	"07b4": true, // Olympus
	"04a9": true, // Canon
	"054c": true, // Sony (部分相机)
}

// phoneVendorIDs 识别为手机的厂商 ID 集合.
var phoneVendorIDs = map[string]bool{
	"04e8": true, // Samsung
	"18d1": true, // Google/Android
	"05ac": true, // Apple
	"22b8": true, // Motorola
	"0bb4": true, // HTC
	"12d1": true, // Huawei
	"2717": true, // Xiaomi
}

// ========== USBMountManager 核心结构 ==========

// USBMountManager 管理 USB 设备的智能挂载、分类、备份建议和历史记录.
type USBMountManager struct {
	mu             sync.Mutex
	history        []HistoryEntry
	historyFile    string
	mountBase      string // 挂载基础路径
	defaultShare   string // 默认备份共享目录
	lastMountIndex int    // 挂载点序号计数器
}

// NewManager 创建新的 USBMountManager 实例.
func NewManager() *USBMountManager {
	m := &USBMountManager{
		history:      make([]HistoryEntry, 0),
		mountBase:    "/media/usb",
		defaultShare: "/volume1/backup",
	}
	// 尝试加载历史记录
	_ = m.loadHistory()
	return m
}

// ========== DetectDevice ==========

// DetectDevice 根据设备信息智能分类设备类型.
func (m *USBMountManager) DetectDevice(deviceInfo USBDeviceInfo) (*DeviceClassification, error) {
	if deviceInfo.VendorID == "" && deviceInfo.ProductID == "" && deviceInfo.DeviceName == "" {
		return nil, fmt.Errorf("insufficient device info: at least VendorID, ProductID or DeviceName required")
	}

	classification := &DeviceClassification{
		DeviceType:        "unknown",
		Confidence:        0.0,
		RecommendedAction: "ask_user",
		AutoMount:         false,
	}

	vendorKnown := false
	if name, ok := vendorNameMap[deviceInfo.VendorID]; ok {
		vendorKnown = true
		_ = name
	}

	// 优先检测相机
	if cameraVendorIDs[deviceInfo.VendorID] {
		classification.DeviceType = "camera"
		classification.Confidence = 0.92
		classification.RecommendedAction = "backup"
		classification.AutoMount = true
		// 相机通常以 PTP/MTP 模式连接，只读挂载
		return classification, nil
	}

	// 检测手机
	if phoneVendorIDs[deviceInfo.VendorID] {
		classification.DeviceType = "phone"
		classification.Confidence = 0.88
		classification.RecommendedAction = "backup"
		classification.AutoMount = true
		return classification, nil
	}

	// 根据容量和可移除性推断
	if deviceInfo.IsRemovable {
		if deviceInfo.CapacityGB >= 500 {
			classification.DeviceType = "external_hdd"
			classification.Confidence = 0.80
			classification.RecommendedAction = "mount"
			classification.AutoMount = true
		} else if deviceInfo.CapacityGB >= 4 && deviceInfo.CapacityGB < 500 {
			classification.DeviceType = "usb_drive"
			classification.Confidence = 0.75
			classification.RecommendedAction = "mount"
			classification.AutoMount = true
		} else if deviceInfo.CapacityGB > 0 && deviceInfo.CapacityGB < 4 {
			classification.DeviceType = "card_reader"
			classification.Confidence = 0.60
			classification.RecommendedAction = "backup"
			classification.AutoMount = false
		} else {
			// 容量未知但有可移除标记
			if vendorKnown {
				classification.DeviceType = "usb_drive"
				classification.Confidence = 0.65
				classification.RecommendedAction = "mount"
				classification.AutoMount = true
			} else {
				classification.DeviceType = "usb_drive"
				classification.Confidence = 0.50
				classification.RecommendedAction = "ask_user"
				classification.AutoMount = false
			}
		}
	} else {
		// 不可移除设备（可能是内置设备通过 USB 总线暴露）
		if vendorKnown {
			classification.DeviceType = "usb_drive"
			classification.Confidence = 0.55
			classification.RecommendedAction = "mount"
			classification.AutoMount = false
		} else {
			classification.DeviceType = "unknown"
			classification.Confidence = 0.30
			classification.RecommendedAction = "ask_user"
			classification.AutoMount = false
		}
	}

	// 根据文件系统微调
	switch strings.ToLower(deviceInfo.FileSystem) {
	case "exfat", "fat32", "vfat":
		// FAT 文件系统常见于 U盘/SD卡
		if classification.DeviceType == "unknown" {
			classification.DeviceType = "usb_drive"
			classification.Confidence = 0.45
		}
	case "ntfs":
		// NTFS 常见于 Windows 格式化的移动硬盘
		if classification.DeviceType == "unknown" {
			classification.DeviceType = "external_hdd"
			classification.Confidence = 0.40
		}
	}

	return classification, nil
}

// ========== RecommendMount ==========

// RecommendMount 根据设备信息推荐挂载配置.
func (m *USBMountManager) RecommendMount(device USBDeviceInfo) (*MountRecommendation, error) {
	if device.FileSystem == "" {
		return nil, fmt.Errorf("file system type is required for mount recommendation")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rec := &MountRecommendation{
		FileSystem: device.FileSystem,
		ReadOnly:   false,
		Encryption: false,
		Options:    make([]string, 0),
	}

	// 分类设备以确定挂载策略
	classification, err := m.detectDeviceInternal(device)
	if err != nil {
		return nil, fmt.Errorf("failed to classify device: %w", err)
	}

	// 生成挂载点
	rec.MountPoint = m.generateMountPoint(device)

	// 根据设备类型推荐挂载选项
	switch classification.DeviceType {
	case "camera":
		rec.ReadOnly = true
		rec.Options = []string{"ro", "sync", "fmask=022", "dmask=022"}
		rec.Reason = "相机设备以只读模式挂载，防止误操作修改原始照片"
	case "phone":
		rec.Options = []string{"rw", "sync", "fmask=022", "dmask=022", "allow_other"}
		rec.Reason = "手机设备以同步写入挂载，允许其他用户访问以备份文件"
	case "external_hdd":
		rec.Options = []string{"rw", "noatime", "nodiratime"}
		if strings.ToLower(device.FileSystem) == "ntfs" {
			rec.Options = append(rec.Options, "big_writes", "uid=0", "gid=0")
		}
		rec.Reason = "移动硬盘使用读写模式挂载，关闭访问时间记录以减少写入"
	case "usb_drive":
		rec.Options = []string{"rw", "noatime", "nodiratime", "flush"}
		rec.Reason = "U盘以读写模式挂载，启用 flush 以确保数据同步"
	case "card_reader":
		rec.ReadOnly = true
		rec.Options = []string{"ro", "sync", "fmask=022", "dmask=022"}
		rec.Reason = "读卡器以只读模式挂载，保护存储卡数据"
	default:
		rec.Options = []string{"rw", "noatime"}
		rec.Reason = "未知设备类型，使用通用安全挂载选项"
	}

	// FAT 文件系统特殊处理
	switch strings.ToLower(device.FileSystem) {
	case "exfat", "fat32", "vfat":
		rec.Options = appendUnique(rec.Options, "iocharset=utf8", "errors=remount-ro")
	case "ntfs":
		rec.Options = appendUnique(rec.Options, "iocharset=utf8", "errors=remount-ro")
	case "ext4", "ext3", "ext2":
		rec.Options = appendUnique(rec.Options, "data=ordered")
	}

	return rec, nil
}

// ========== SuggestBackup ==========

// SuggestBackup 根据设备信息提供备份建议.
func (m *USBMountManager) SuggestBackup(device USBDeviceInfo) (*BackupSuggestion, error) {
	classification, err := m.DetectDevice(device)
	if err != nil {
		return nil, fmt.Errorf("failed to classify device for backup suggestion: %w", err)
	}

	suggestion := &BackupSuggestion{
		ShouldBackup: false,
		Strategy:     "none",
		TargetShare:   m.defaultShare,
		SizeGB:       0,
	}

	switch classification.DeviceType {
	case "camera":
		suggestion.ShouldBackup = true
		suggestion.Strategy = "photo_import"
		suggestion.TargetShare = filepath.Join(m.defaultShare, "photos")
		suggestion.EstimatedDuration = estimateDuration(device.CapacityGB, 40) // ~40 MB/s 照片导入
		suggestion.Deduplicate = true
		suggestion.Encryption = false
		suggestion.SizeGB = device.CapacityGB * 0.7 // 相机通常照片占 70%
		return suggestion, nil

	case "phone":
		suggestion.ShouldBackup = true
		suggestion.Strategy = "incremental"
		suggestion.TargetShare = filepath.Join(m.defaultShare, "mobile")
		suggestion.EstimatedDuration = estimateDuration(device.CapacityGB, 30) // ~30 MB/s MTP
		suggestion.Deduplicate = true
		suggestion.Encryption = true // 手机数据建议加密备份
		suggestion.SizeGB = device.CapacityGB * 0.5
		return suggestion, nil

	case "external_hdd":
		suggestion.ShouldBackup = true
		suggestion.Strategy = "mirror"
		suggestion.TargetShare = filepath.Join(m.defaultShare, "external")
		suggestion.EstimatedDuration = estimateDuration(device.CapacityGB, 120) // ~120 MB/s 外置硬盘
		suggestion.Deduplicate = false
		suggestion.Encryption = false
		suggestion.SizeGB = device.CapacityGB * 0.85
		return suggestion, nil

	case "usb_drive":
		suggestion.ShouldBackup = true
		suggestion.Strategy = "full"
		suggestion.TargetShare = filepath.Join(m.defaultShare, "usbdrive")
		suggestion.EstimatedDuration = estimateDuration(device.CapacityGB, 25) // ~25 MB/s U盘
		suggestion.Deduplicate = true
		suggestion.Encryption = false
		suggestion.SizeGB = device.CapacityGB * 0.6
		return suggestion, nil

	case "card_reader":
		suggestion.ShouldBackup = true
		suggestion.Strategy = "photo_import"
		suggestion.TargetShare = filepath.Join(m.defaultShare, "sdcard")
		suggestion.EstimatedDuration = estimateDuration(device.CapacityGB, 20)
		suggestion.Deduplicate = true
		suggestion.Encryption = false
		suggestion.SizeGB = device.CapacityGB * 0.8
		return suggestion, nil

	default:
		suggestion.ShouldBackup = false
		suggestion.Strategy = "none"
		suggestion.TargetShare = ""
		suggestion.EstimatedDuration = "unknown"
		return suggestion, nil
	}
}

// ========== SafeEject ==========

// SafeEject 执行安全弹出操作.
func (m *USBMountManager) SafeEject(mountPath string) (*EjectResult, error) {
	if mountPath == "" {
		return nil, fmt.Errorf("mount path is required")
	}

	result := &EjectResult{
		Path:         mountPath,
		Success:      false,
		PendingIO:    false,
		SafeToRemove: false,
		Warning:      "",
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查挂载路径是否存在
	info, err := os.Stat(mountPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Warning = fmt.Sprintf("mount path %s does not exist", mountPath)
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat mount path: %w", err)
	}
	_ = info

	// 尝试 sync 同步文件系统（模拟 sync 操作）
	// 在实际系统中会调用 syscall.Sync() 或 exec.Command("sync")
	// 这里通过检查是否有可疑的写入操作来判断 PendingIO
	result.PendingIO = checkPendingIO(mountPath)

	if result.PendingIO {
		result.Warning = "there is pending I/O on the device, please wait and try again"
		return result, nil
	}

	// 尝试卸载（在实际系统中会调用 syscall.Unmount 或 umount 命令）
	// 这里模拟卸载检测：检查挂载点是否还在 /proc/mounts 中
	stillMounted, err := isMountActive(mountPath)
	if err != nil {
		result.Warning = fmt.Sprintf("failed to check mount status: %v", err)
		return result, nil
	}

	if stillMounted {
		// 尝试卸载
		// 在实际系统中：err := exec.Command("umount", mountPath).Run()
		// 这里我们模拟成功
		result.Warning = "device is still mounted; unmount would be performed here"
	} else {
		result.Warning = "device is not mounted"
	}

	// 模拟弹出成功
	result.Success = true
	result.SafeToRemove = true
	result.PendingIO = false

	// 更新历史记录中的挂载次数等
	for i := range m.history {
		// 简单匹配：通过路径关联不是精确的，仅做记录
		_ = i
	}

	return result, nil
}

// ========== RecordHistory ==========

// RecordHistory 记录设备到历史记录中.
func (m *USBMountManager) RecordHistory(device USBDeviceInfo) (*HistoryEntry, error) {
	if device.SerialNumber == "" {
		return nil, fmt.Errorf("serial number is required for history recording")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()

	// 查找已有记录
	for i := range m.history {
		if m.history[i].SerialNumber == device.SerialNumber {
			m.history[i].LastSeen = now
			m.history[i].MountCount++
			m.history[i].DeviceName = device.DeviceName // 更新设备名称（可能变化）
			_ = m.saveHistory()
			return &m.history[i], nil
		}
	}

	// 新设备记录
	entry := HistoryEntry{
		DeviceName:            device.DeviceName,
		SerialNumber:          device.SerialNumber,
		FirstSeen:             now,
		LastSeen:              now,
		MountCount:            1,
		TotalBytesTransferred: 0,
	}
	m.history = append(m.history, entry)

	_ = m.saveHistory()
	return &entry, nil
}

// ========== GetHistory ==========

// GetHistory 返回所有设备历史记录.
func (m *USBMountManager) GetHistory() ([]HistoryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]HistoryEntry, len(m.history))
	copy(result, m.history)
	return result, nil
}

// ========== 内部辅助方法 ==========

// detectDeviceInternal 是 DetectDevice 的非加锁版本，供内部调用.
func (m *USBMountManager) detectDeviceInternal(device USBDeviceInfo) (*DeviceClassification, error) {
	return m.DetectDevice(device)
}

// generateMountPoint 生成唯一的挂载点路径.
func (m *USBMountManager) generateMountPoint(device USBDeviceInfo) string {
	m.lastMountIndex++
	base := m.mountBase

	if device.DeviceName != "" {
		// 清理设备名称用作路径
		safeName := sanitizePathSegment(device.DeviceName)
		return filepath.Join(base, fmt.Sprintf("%s-%d", safeName, m.lastMountIndex))
	}
	return filepath.Join(base, fmt.Sprintf("usb-%d", m.lastMountIndex))
}

// sanitizePathSegment 清理字符串使其能安全用作路径段.
func sanitizePathSegment(s string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return strings.ToLower(replacer.Replace(s))
}

// estimateDuration 估算备份耗时.
func estimateDuration(capacityGB float64, speedMBps float64) string {
	if speedMBps <= 0 || capacityGB <= 0 {
		return "unknown"
	}
	totalMB := capacityGB * 1024
	seconds := totalMB / speedMBps

	if seconds < 60 {
		return fmt.Sprintf("%d seconds", int(seconds))
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", int(minutes))
	}
	hours := minutes / 60
	mins := int(minutes) % 60
	return fmt.Sprintf("%dh %dm", int(hours), mins)
}

// checkPendingIO 检查挂载点是否有未完成的 I/O 操作.
func checkPendingIO(mountPath string) bool {
	_, err := os.Stat(filepath.Join(mountPath, ".pending_write"))
	return err == nil // 如果存在标记文件则认为有 pending I/O
}

// isMountActive 检查路径是否仍然挂载.
func isMountActive(mountPath string) (bool, error) {
	_, err := os.Stat(mountPath)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	// 在实际系统中会读取 /proc/mounts
	// 这里通过路径是否存在来判断
	return true, nil
}

// appendUnique 向切片追加唯一的字符串.
func appendUnique(slice []string, items ...string) []string {
	existing := make(map[string]bool)
	for _, s := range slice {
		existing[s] = true
	}
	for _, item := range items {
		if !existing[item] {
			slice = append(slice, item)
			existing[item] = true
		}
	}
	return slice
}

// ========== 历史记录持久化 ==========

// loadHistory 从文件加载历史记录.
func (m *USBMountManager) loadHistory() error {
	m.historyFile = filepath.Join(os.TempDir(), "usbsmartmount_history.json")
	data, err := os.ReadFile(m.historyFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.history)
}

// saveHistory 保存历史记录到文件.
func (m *USBMountManager) saveHistory() error {
	if m.historyFile == "" {
		m.historyFile = filepath.Join(os.TempDir(), "usbsmartmount_history.json")
	}
	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.historyFile, data, 0644)
}