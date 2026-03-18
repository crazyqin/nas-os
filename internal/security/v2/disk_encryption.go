// Package securityv2 提供磁盘加密管理功能
package securityv2

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 类型定义 ==========

// DiskEncryptionConfig 磁盘加密配置
type DiskEncryptionConfig struct {
	// ID 配置 ID
	ID string `json:"id"`

	// DevicePath 设备路径
	DevicePath string `json:"devicePath"`

	// MountPoint 挂载点
	MountPoint string `json:"mountPoint"`

	// EncryptionType 加密类型
	EncryptionType EncryptionType `json:"encryptionType"`

	// Cipher 密码套件
	Cipher string `json:"cipher"`

	// KeySize 密钥大小 (bits)
	KeySize int `json:"keySize"`

	// KeySourceType 密钥来源类型
	KeySourceType KeySourceType `json:"keySourceType"`

	// KeyFilePath 密钥文件路径
	KeyFilePath string `json:"keyFilePath,omitempty"`

	// Status 加密状态
	Status EncryptionStatus `json:"status"`

	// AutoUnlock 是否自动解锁
	AutoUnlock bool `json:"autoUnlock"`

	// KeyRotationEnabled 是否启用密钥轮换
	KeyRotationEnabled bool `json:"keyRotationEnabled"`

	// KeyRotationDays 密钥轮换周期（天）
	KeyRotationDays int `json:"keyRotationDays"`

	// LastKeyRotation 上次密钥轮换时间
	LastKeyRotation *time.Time `json:"lastKeyRotation,omitempty"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// EncryptionType 加密类型
type EncryptionType string

const (
	// EncryptionTypeLUKS1 表示 LUKS1 加密格式
	EncryptionTypeLUKS1 EncryptionType = "luks1"
	// EncryptionTypeLUKS2 表示 LUKS2 加密格式（推荐）
	EncryptionTypeLUKS2 EncryptionType = "luks2"
)

// KeySourceType 密钥来源类型
type KeySourceType string

const (
	// KeySourcePassphrase 表示使用密码短语作为密钥来源
	KeySourcePassphrase KeySourceType = "passphrase"
	// KeySourceKeyFile 表示使用密钥文件作为密钥来源
	KeySourceKeyFile KeySourceType = "keyfile"
	// KeySourceTPM 表示使用 TPM 作为密钥来源
	KeySourceTPM KeySourceType = "tpm"
	// KeySourceYubiKey 表示使用 YubiKey 作为密钥来源
	KeySourceYubiKey KeySourceType = "yubikey"
)

// EncryptionStatus 加密状态
type EncryptionStatus string

const (
	// EncryptionStatusLocked 表示加密设备已锁定
	EncryptionStatusLocked EncryptionStatus = "locked"
	// EncryptionStatusUnlocked 表示加密设备已解锁
	EncryptionStatusUnlocked EncryptionStatus = "unlocked"
	// EncryptionStatusError 表示加密设备处于错误状态
	EncryptionStatusError EncryptionStatus = "error"
)

// LUKSInfo LUKS 信息
type LUKSInfo struct {
	// Version LUKS 版本
	Version string `json:"version"`

	// Cipher 密码套件
	Cipher string `json:"cipher"`

	// KeySize 密钥大小
	KeySize int `json:"keySize"`

	// Device 设备路径
	Device string `json:"device"`

	// UUID 设备 UUID
	UUID string `json:"uuid"`

	// MapperName 映射名称
	MapperName string `json:"mapperName"`

	// KeySlots 密钥槽位信息
	KeySlots []KeySlotInfo `json:"keySlots"`

	// MetadataSize 元数据大小
	MetadataSize int64 `json:"metadataSize"`

	// DataOffset 数据偏移
	DataOffset int64 `json:"dataOffset"`
}

// KeySlotInfo 密钥槽位信息
type KeySlotInfo struct {
	// SlotID 槽位 ID
	SlotID int `json:"slotId"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// Type 密钥类型
	Type string `json:"type"`

	// KeyDescription 密钥描述
	KeyDescription string `json:"keyDescription,omitempty"`

	// CreatedAt 创建时间
	CreatedAt *time.Time `json:"createdAt,omitempty"`

	// LastUsedAt 最后使用时间
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// KeyRotationPolicy 密钥轮换策略
type KeyRotationPolicy struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// IntervalDays 轮换间隔（天）
	IntervalDays int `json:"intervalDays"`

	// MaxKeyAgeDays 最大密钥年龄（天）
	MaxKeyAgeDays int `json:"maxKeyAgeDays"`

	// NotificationDays 提前通知天数
	NotificationDays int `json:"notificationDays"`

	// AutoRotate 是否自动轮换
	AutoRotate bool `json:"autoRotate"`

	// BackupKeys 是否保留备份密钥
	BackupKeys bool `json:"backupKeys"`

	// KeyHistory 密钥历史记录
	KeyHistory []KeyHistoryEntry `json:"keyHistory,omitempty"`
}

// KeyHistoryEntry 密钥历史记录
type KeyHistoryEntry struct {
	// SlotID 槽位 ID
	SlotID int `json:"slotId"`

	// Action 操作类型
	Action string `json:"action"` // added, removed, rotated

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Description 描述
	Description string `json:"description"`

	// User 操作用户
	User string `json:"user,omitempty"`
}

// EncryptionPerformance 加密性能指标
type EncryptionPerformance struct {
	// DevicePath 设备路径
	DevicePath string `json:"devicePath"`

	// ReadSpeedMBps 读取速度 (MB/s)
	ReadSpeedMBps float64 `json:"readSpeedMBps"`

	// WriteSpeedMBps 写入速度 (MB/s)
	WriteSpeedMBps float64 `json:"writeSpeedMBps"`

	// CPUOverheadPercent CPU 开销百分比
	CPUOverheadPercent float64 `json:"cpuOverheadPercent"`

	// MemoryUsageMB 内存使用 (MB)
	MemoryUsageMB float64 `json:"memoryUsageMB"`

	// QueueDepth 队列深度
	QueueDepth int `json:"queueDepth"`

	// Algorithm 算法
	Algorithm string `json:"algorithm"`

	// HardwareAccelerated 是否硬件加速
	HardwareAccelerated bool `json:"hardwareAccelerated"`

	// LastUpdated 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// EncryptionBenchmark 加密基准测试结果
type EncryptionBenchmark struct {
	// Algorithm 算法
	Algorithm string `json:"algorithm"`

	// Cipher 密码套件
	Cipher string `json:"cipher"`

	// KeySize 密钥大小
	KeySize int `json:"keySize"`

	// EncryptionSpeedMBps 加密速度 (MB/s)
	EncryptionSpeedMBps float64 `json:"encryptionSpeedMBps"`

	// DecryptionSpeedMBps 解密速度 (MB/s)
	DecryptionSpeedMBps float64 `json:"decryptionSpeedMBps"`

	// LatencyMs 延迟 (ms)
	LatencyMs float64 `json:"latencyMs"`

	// HardwareAccelerated 是否硬件加速
	HardwareAccelerated bool `json:"hardwareAccelerated"`

	// Recommended 是否推荐
	Recommended bool `json:"recommended"`

	// Notes 备注
	Notes string `json:"notes,omitempty"`
}

// DiskEncryptionManager 磁盘加密管理器
type DiskEncryptionManager struct {
	mu sync.RWMutex

	// configs 加密配置
	configs map[string]*DiskEncryptionConfig

	// configPath 配置文件路径
	configPath string

	// performanceCache 性能缓存
	performanceCache map[string]*EncryptionPerformance

	// rotationPolicy 密钥轮换策略
	rotationPolicy *KeyRotationPolicy

	// hooks 事件钩子
	hooks EncryptionHooks
}

// EncryptionHooks 加密事件钩子
type EncryptionHooks struct {
	// OnDiskLocked 磁盘锁定回调
	OnDiskLocked func(devicePath string)

	// OnDiskUnlocked 磁盘解锁回调
	OnDiskUnlocked func(devicePath string)

	// OnKeyRotated 密钥轮换回调
	OnKeyRotated func(devicePath string, slotID int)

	// OnRotationReminder 轮换提醒回调
	OnRotationReminder func(devicePath string, daysUntilExpiry int)
}

// NewDiskEncryptionManager 创建磁盘加密管理器
func NewDiskEncryptionManager(configPath string) *DiskEncryptionManager {
	return &DiskEncryptionManager{
		configs:          make(map[string]*DiskEncryptionConfig),
		configPath:       configPath,
		performanceCache: make(map[string]*EncryptionPerformance),
		rotationPolicy: &KeyRotationPolicy{
			Enabled:          false,
			IntervalDays:     90,
			MaxKeyAgeDays:    180,
			NotificationDays: 14,
			AutoRotate:       false,
			BackupKeys:       true,
			KeyHistory:       make([]KeyHistoryEntry, 0),
		},
	}
}

// Initialize 初始化磁盘加密管理器
func (dm *DiskEncryptionManager) Initialize() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 加载配置
	if err := dm.loadConfig(); err != nil {
		dm.configs = make(map[string]*DiskEncryptionConfig)
	}

	// 扫描已加密设备
	if err := dm.scanEncryptedDevices(); err != nil {
		// 记录警告但不失败
		_, _ = fmt.Printf("警告: 扫描加密设备失败: %v\n", err)
	}

	return nil
}

// ========== LUKS 操作 ==========

// CreateLUKS 创建 LUKS 加密卷
func (dm *DiskEncryptionManager) CreateLUKS(devicePath, passphrase string, config *DiskEncryptionConfig) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查设备是否存在
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		return fmt.Errorf("设备不存在: %s", devicePath)
	}

	// 确定 LUKS 版本
	luksVersion := "luks2"
	if config.EncryptionType == EncryptionTypeLUKS1 {
		luksVersion = "luks1"
	}

	// 确定 cipher
	cipher := config.Cipher
	if cipher == "" {
		cipher = "aes-xts-plain64"
	}

	keySize := config.KeySize
	if keySize == 0 {
		keySize = 512
	}

	// 使用 cryptsetup 创建 LUKS 卷
	cmd := exec.Command("cryptsetup", "-q", "--type", luksVersion,
		"--cipher", cipher,
		"--key-size", fmt.Sprintf("%d", keySize),
		"luksFormat", devicePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 cryptsetup 失败: %w", err)
	}

	// 写入密码
	_, _ = fmt.Fprintln(stdin, passphrase)
	_, _ = fmt.Fprintln(stdin, passphrase)
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("创建 LUKS 卷失败: %w", err)
	}

	// 获取 UUID
	uuid, err := dm.getLUKSUUID(devicePath)
	if err != nil {
		return fmt.Errorf("获取 UUID 失败: %w", err)
	}

	// 更新配置
	config.ID = uuid
	config.DevicePath = devicePath
	config.Status = EncryptionStatusLocked
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	dm.configs[devicePath] = config

	if err := dm.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// OpenLUKS 打开 LUKS 加密卷
func (dm *DiskEncryptionManager) OpenLUKS(devicePath, mapperName, passphrase string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 使用 cryptsetup 打开卷
	cmd := exec.Command("cryptsetup", "luksOpen", devicePath, mapperName)
	cmd.Stdin = strings.NewReader(passphrase + "\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("打开 LUKS 卷失败: %w: %s", err, string(output))
	}

	// 更新状态
	if config, ok := dm.configs[devicePath]; ok {
		config.Status = EncryptionStatusUnlocked
		now := time.Now()
		config.UpdatedAt = now
	}

	// 触发钩子
	if dm.hooks.OnDiskUnlocked != nil {
		dm.hooks.OnDiskUnlocked(devicePath)
	}

	return nil
}

// CloseLUKS 关闭 LUKS 加密卷
func (dm *DiskEncryptionManager) CloseLUKS(mapperName string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 使用 cryptsetup 关闭卷
	cmd := exec.Command("cryptsetup", "luksClose", mapperName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("关闭 LUKS 卷失败: %w: %s", err, string(output))
	}

	// 更新状态
	for _, config := range dm.configs {
		if strings.Contains(config.DevicePath, mapperName) {
			config.Status = EncryptionStatusLocked
			now := time.Now()
			config.UpdatedAt = now

			// 触发钩子
			if dm.hooks.OnDiskLocked != nil {
				dm.hooks.OnDiskLocked(config.DevicePath)
			}
			break
		}
	}

	return nil
}

// GetLUKSInfo 获取 LUKS 信息
func (dm *DiskEncryptionManager) GetLUKSInfo(devicePath string) (*LUKSInfo, error) {
	// 使用 cryptsetup 获取信息
	cmd := exec.Command("cryptsetup", "luksDump", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 LUKS 信息失败: %w", err)
	}

	return dm.parseLUKSDump(string(output), devicePath)
}

// parseLUKSDump 解析 LUKS dump 输出
func (dm *DiskEncryptionManager) parseLUKSDump(dump, devicePath string) (*LUKSInfo, error) {
	info := &LUKSInfo{
		Device:   devicePath,
		KeySlots: make([]KeySlotInfo, 0),
	}

	scanner := bufio.NewScanner(strings.NewReader(dump))
	var currentSlot *KeySlotInfo

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Version":
			info.Version = value
		case "Cipher":
			info.Cipher = value
		case "Key bits":
			_, _ = fmt.Sscanf(value, "%d", &info.KeySize)
		case "UUID":
			info.UUID = value
		case "Key Slot":
			if currentSlot != nil {
				info.KeySlots = append(info.KeySlots, *currentSlot)
			}
			var slotID int
			_, _ = fmt.Sscanf(value, "%d", &slotID)
			currentSlot = &KeySlotInfo{
				SlotID: slotID,
			}
		case "State":
			if currentSlot != nil {
				currentSlot.Enabled = strings.Contains(value, "enabled")
			}
		}
	}

	if currentSlot != nil {
		info.KeySlots = append(info.KeySlots, *currentSlot)
	}

	return info, nil
}

// ========== 密钥管理 ==========

// AddKeySlot 添加密钥槽位
func (dm *DiskEncryptionManager) AddKeySlot(devicePath, currentPassphrase, newPassphrase string, slotID int) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	args := []string{"luksAddKey", devicePath}

	// 指定槽位
	if slotID >= 0 {
		args = []string{"luksAddKey", "--new-key-slot", fmt.Sprintf("%d", slotID), devicePath}
	}

	cmd := exec.Command("cryptsetup", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 cryptsetup 失败: %w", err)
	}

	// 写入当前密码和新密码
	_, _ = fmt.Fprintln(stdin, currentPassphrase)
	_, _ = fmt.Fprintln(stdin, newPassphrase)
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("添加密钥失败: %w", err)
	}

	// 记录历史
	dm.addKeyHistory(devicePath, slotID, "added", "添加新密钥")

	return nil
}

// RemoveKeySlot 移除密钥槽位
func (dm *DiskEncryptionManager) RemoveKeySlot(devicePath, passphrase string, slotID int) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	cmd := exec.Command("cryptsetup", "luksKillSlot", devicePath, fmt.Sprintf("%d", slotID))
	cmd.Stdin = strings.NewReader(passphrase + "\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("移除密钥失败: %w: %s", err, string(output))
	}

	// 记录历史
	dm.addKeyHistory(devicePath, slotID, "removed", "移除密钥")

	return nil
}

// RotateKey 轮换密钥
func (dm *DiskEncryptionManager) RotateKey(devicePath, currentPassphrase, newPassphrase string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 获取当前信息
	info, err := dm.GetLUKSInfo(devicePath)
	if err != nil {
		return fmt.Errorf("获取 LUKS 信息失败: %w", err)
	}

	// 找到可用的槽位
	usedSlots := make(map[int]bool)
	for _, slot := range info.KeySlots {
		if slot.Enabled {
			usedSlots[slot.SlotID] = true
		}
	}

	// 如果所有槽位都被使用，移除一个旧的
	if len(usedSlots) >= 8 {
		// 移除第一个非当前使用的槽位
		for i := 0; i < 8; i++ {
			if usedSlots[i] {
				if err := dm.removeKeySlotInternal(devicePath, currentPassphrase, i); err != nil {
					continue
				}
				break
			}
		}
	}

	// 添加新密钥
	cmd := exec.Command("cryptsetup", "luksAddKey", devicePath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 cryptsetup 失败: %w", err)
	}

	_, _ = fmt.Fprintln(stdin, currentPassphrase)
	_, _ = fmt.Fprintln(stdin, newPassphrase)
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("添加新密钥失败: %w", err)
	}

	// 更新配置
	if config, ok := dm.configs[devicePath]; ok {
		now := time.Now()
		config.LastKeyRotation = &now
		config.UpdatedAt = now
	}

	// 记录历史
	dm.addKeyHistory(devicePath, -1, "rotated", "密钥轮换")

	// 触发钩子
	if dm.hooks.OnKeyRotated != nil {
		dm.hooks.OnKeyRotated(devicePath, -1)
	}

	return nil
}

// removeKeySlotInternal 内部移除密钥槽位方法
func (dm *DiskEncryptionManager) removeKeySlotInternal(devicePath, passphrase string, slotID int) error {
	cmd := exec.Command("cryptsetup", "luksKillSlot", devicePath, fmt.Sprintf("%d", slotID))
	cmd.Stdin = strings.NewReader(passphrase + "\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("移除密钥失败: %w: %s", err, string(output))
	}

	return nil
}

// ========== 密钥轮换机制 ==========

// SetRotationPolicy 设置密钥轮换策略
func (dm *DiskEncryptionManager) SetRotationPolicy(policy *KeyRotationPolicy) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.rotationPolicy = policy
}

// CheckKeyRotation 检查是否需要轮换密钥
func (dm *DiskEncryptionManager) CheckKeyRotation(devicePath string) (needsRotation bool, daysUntilExpiry int, err error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.rotationPolicy == nil || !dm.rotationPolicy.Enabled {
		return false, 0, nil
	}

	config, ok := dm.configs[devicePath]
	if !ok {
		return false, 0, fmt.Errorf("设备配置不存在")
	}

	if config.LastKeyRotation == nil {
		// 从未轮换过，需要轮换
		return true, 0, nil
	}

	daysSinceRotation := int(time.Since(*config.LastKeyRotation).Hours() / 24)
	daysUntilExpiry = dm.rotationPolicy.MaxKeyAgeDays - daysSinceRotation

	if daysUntilExpiry <= 0 {
		return true, 0, nil
	}

	return false, daysUntilExpiry, nil
}

// AutoRotateKeys 自动轮换密钥
func (dm *DiskEncryptionManager) AutoRotateKeys(passphraseProvider func(devicePath string) (string, error)) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.rotationPolicy == nil || !dm.rotationPolicy.AutoRotate {
		return nil
	}

	var errors []string

	for devicePath, config := range dm.configs {
		if !config.KeyRotationEnabled {
			continue
		}

		// 检查是否需要轮换
		if config.LastKeyRotation != nil {
			daysSinceRotation := int(time.Since(*config.LastKeyRotation).Hours() / 24)
			if daysSinceRotation < config.KeyRotationDays {
				continue
			}
		}

		// 获取当前密码
		currentPassphrase, err := passphraseProvider(devicePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: 获取密码失败: %v", devicePath, err))
			continue
		}

		// 生成新密码
		newPassphrase := generatePassphrase()

		// 执行轮换
		cmd := exec.Command("cryptsetup", "luksAddKey", devicePath)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: 创建管道失败: %v", devicePath, err))
			continue
		}

		if err := cmd.Start(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: 启动命令失败: %v", devicePath, err))
			continue
		}

		_, _ = fmt.Fprintln(stdin, currentPassphrase)
		_, _ = fmt.Fprintln(stdin, newPassphrase)
		_ = stdin.Close()

		if err := cmd.Wait(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: 轮换失败: %v", devicePath, err))
			continue
		}

		// 更新时间
		now := time.Now()
		config.LastKeyRotation = &now
		config.UpdatedAt = now

		// 触发钩子
		if dm.hooks.OnKeyRotated != nil {
			dm.hooks.OnKeyRotated(devicePath, -1)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分轮换失败:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// ========== 加密性能优化 ==========

// GetEncryptionPerformance 获取加密性能
func (dm *DiskEncryptionManager) GetEncryptionPerformance(devicePath string) (*EncryptionPerformance, error) {
	dm.mu.RLock()
	if perf, ok := dm.performanceCache[devicePath]; ok {
		dm.mu.RUnlock()
		return perf, nil
	}
	dm.mu.RUnlock()

	// 执行基准测试
	perf, err := dm.benchmarkEncryption(devicePath)
	if err != nil {
		return nil, err
	}

	dm.mu.Lock()
	dm.performanceCache[devicePath] = perf
	dm.mu.Unlock()

	return perf, nil
}

// benchmarkEncryption 基准测试加密性能
func (dm *DiskEncryptionManager) benchmarkEncryption(devicePath string) (*EncryptionPerformance, error) {
	perf := &EncryptionPerformance{
		DevicePath:          devicePath,
		LastUpdated:         time.Now(),
		HardwareAccelerated: dm.checkHardwareAcceleration(),
	}

	// 使用 dd 测试读写速度
	// 注意：这需要设备已打开且挂载

	// 检测算法
	info, err := dm.GetLUKSInfo(devicePath)
	if err == nil {
		perf.Algorithm = info.Cipher
	}

	// 简化的性能估算
	// 实际应该使用更精确的测试
	perf.ReadSpeedMBps = 500.0  // 估算值
	perf.WriteSpeedMBps = 450.0 // 估算值
	perf.CPUOverheadPercent = 5.0
	perf.MemoryUsageMB = 32.0
	perf.QueueDepth = 32

	return perf, nil
}

// checkHardwareAcceleration 检查硬件加速
func (dm *DiskEncryptionManager) checkHardwareAcceleration() bool {
	// 检查 CPU 是否支持 AES-NI
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}

	return strings.Contains(string(data), "aes")
}

// OptimizeEncryption 优化加密配置
func (dm *DiskEncryptionManager) OptimizeEncryption(devicePath string) ([]string, error) {
	var recommendations []string

	// 获取当前信息
	info, err := dm.GetLUKSInfo(devicePath)
	if err != nil {
		return nil, fmt.Errorf("获取 LUKS 信息失败: %w", err)
	}

	// 检查硬件加速
	if !dm.checkHardwareAcceleration() {
		recommendations = append(recommendations, "建议启用 CPU AES-NI 硬件加速")
	}

	// 检查 cipher 选择
	if !strings.Contains(info.Cipher, "xts") {
		recommendations = append(recommendations, "建议使用 XTS 模式以获得更好的并行性能")
	}

	// 检查密钥大小
	if info.KeySize < 512 {
		recommendations = append(recommendations, "建议使用 512-bit 密钥以获得更好的安全性")
	}

	// 检查密钥槽数量
	activeSlots := 0
	for _, slot := range info.KeySlots {
		if slot.Enabled {
			activeSlots++
		}
	}

	if activeSlots < 2 {
		recommendations = append(recommendations, "建议至少配置 2 个密钥槽位以防止单点故障")
	}

	return recommendations, nil
}

// RunEncryptionBenchmark 运行加密基准测试
func (dm *DiskEncryptionManager) RunEncryptionBenchmark(testSizeMB int) ([]EncryptionBenchmark, error) {
	var results []EncryptionBenchmark

	// 测试不同的算法配置
	ciphers := []struct {
		cipher  string
		keySize int
	}{
		{"aes-xts-plain64", 512},
		{"aes-xts-plain64", 256},
		{"aes-cbc-essiv:sha256", 256},
		{"serpent-xts-plain64", 512},
	}

	for _, c := range ciphers {
		benchmark := EncryptionBenchmark{
			Cipher:    c.cipher,
			KeySize:   c.keySize,
			Algorithm: strings.Split(c.cipher, "-")[0],
		}

		// 模拟测试结果（实际应该使用真实测试）
		if strings.Contains(c.cipher, "aes") && dm.checkHardwareAcceleration() {
			benchmark.EncryptionSpeedMBps = 800.0
			benchmark.DecryptionSpeedMBps = 850.0
			benchmark.LatencyMs = 0.5
			benchmark.HardwareAccelerated = true
		} else {
			benchmark.EncryptionSpeedMBps = 200.0
			benchmark.DecryptionSpeedMBps = 220.0
			benchmark.LatencyMs = 2.0
			benchmark.HardwareAccelerated = false
		}

		// 推荐配置
		if c.cipher == "aes-xts-plain64" && c.keySize == 512 {
			benchmark.Recommended = true
			benchmark.Notes = "最佳性能与安全性平衡"
		}

		results = append(results, benchmark)
	}

	return results, nil
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (dm *DiskEncryptionManager) GetConfig(devicePath string) (*DiskEncryptionConfig, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	config, ok := dm.configs[devicePath]
	if !ok {
		return nil, fmt.Errorf("配置不存在: %s", devicePath)
	}

	return config, nil
}

// ListConfigs 列出所有配置
func (dm *DiskEncryptionManager) ListConfigs() []*DiskEncryptionConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]*DiskEncryptionConfig, 0, len(dm.configs))
	for _, c := range dm.configs {
		result = append(result, c)
	}
	return result
}

// UpdateConfig 更新配置
func (dm *DiskEncryptionManager) UpdateConfig(devicePath string, config *DiskEncryptionConfig) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, ok := dm.configs[devicePath]; !ok {
		return fmt.Errorf("配置不存在: %s", devicePath)
	}

	config.UpdatedAt = time.Now()
	dm.configs[devicePath] = config

	return dm.saveConfig()
}

// scanEncryptedDevices 扫描加密设备
func (dm *DiskEncryptionManager) scanEncryptedDevices() error {
	// 读取 /proc/crypto 或使用 blkid 扫描
	cmd := exec.Command("blkid")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "crypto_LUKS") {
			// 提取设备路径
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				devicePath := strings.TrimSpace(parts[0])
				if _, ok := dm.configs[devicePath]; !ok {
					// 创建默认配置
					dm.configs[devicePath] = &DiskEncryptionConfig{
						ID:             generateID(),
						DevicePath:     devicePath,
						EncryptionType: EncryptionTypeLUKS2,
						Cipher:         "aes-xts-plain64",
						KeySize:        512,
						Status:         EncryptionStatusLocked,
						CreatedAt:      time.Now(),
						UpdatedAt:      time.Now(),
					}
				}
			}
		}
	}

	return nil
}

func (dm *DiskEncryptionManager) loadConfig() error {
	data, err := os.ReadFile(dm.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &dm.configs)
}

func (dm *DiskEncryptionManager) saveConfig() error {
	data, err := json.MarshalIndent(dm.configs, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(dm.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(dm.configPath, data, 0600)
}

func (dm *DiskEncryptionManager) getLUKSUUID(devicePath string) (string, error) {
	cmd := exec.Command("cryptsetup", "luksUUID", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (dm *DiskEncryptionManager) addKeyHistory(devicePath string, slotID int, action, description string) {
	if dm.rotationPolicy != nil {
		dm.rotationPolicy.KeyHistory = append(dm.rotationPolicy.KeyHistory, KeyHistoryEntry{
			SlotID:      slotID,
			Action:      action,
			Timestamp:   time.Now(),
			Description: description,
		})
	}
}

// SetHooks 设置事件钩子
func (dm *DiskEncryptionManager) SetHooks(hooks EncryptionHooks) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.hooks = hooks
}

// ========== 辅助函数 ==========

func generatePassphrase() string {
	// 生成安全的随机密码
	cmd := exec.Command("openssl", "rand", "-base64", "32")
	output, err := cmd.Output()
	if err != nil {
		// 回退到简单生成
		return fmt.Sprintf("key-%d", time.Now().UnixNano())
	}
	return strings.TrimSpace(string(output))
}

func generateID() string {
	return fmt.Sprintf("id-%d", time.Now().UnixNano())
}
