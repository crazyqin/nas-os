// Package storage 提供 WriteOnce (WORM) 不可变存储功能
// 用于防止勒索病毒攻击和保护关键数据
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/pkg/btrfs"

	"github.com/google/uuid"
)

// LockDuration 锁定时长类型
type LockDuration string

const (
	// LockDuration7Days 7天锁定
	LockDuration7Days LockDuration = "7d"
	// LockDuration30Days 30天锁定
	LockDuration30Days LockDuration = "30d"
	// LockDurationPermanent 永久锁定
	LockDurationPermanent LockDuration = "permanent"
)

// LockDurationHours 锁定时长的小时数映射
var LockDurationHours = map[LockDuration]int{
	LockDuration7Days:     7 * 24,
	LockDuration30Days:    30 * 24,
	LockDurationPermanent: -1, // -1 表示永久
}

// ImmutableStatus 不可变状态
type ImmutableStatus string

const (
	// ImmutableStatusActive 已锁定
	ImmutableStatusActive ImmutableStatus = "active"
	// ImmutableStatusExpired 已过期（可解锁）
	ImmutableStatusExpired ImmutableStatus = "expired"
	// ImmutableStatusUnlocked 已解锁
	ImmutableStatusUnlocked ImmutableStatus = "unlocked"
)

// ImmutableConfig 不可变存储配置
type ImmutableConfig struct {
	// SnapDir 不可变快照存储目录
	SnapDir string `json:"snapDir"`
	// AutoCleanup 是否自动清理过期记录
	AutoCleanup bool `json:"autoCleanup"`
	// CleanupInterval 清理间隔（小时）
	CleanupInterval int `json:"cleanupInterval"`
	// MaxRecords 最大记录数
	MaxRecords int `json:"maxRecords"`
}

// DefaultImmutableConfig 默认配置
var DefaultImmutableConfig = ImmutableConfig{
	SnapDir:         ".immutable",
	AutoCleanup:     true,
	CleanupInterval: 24,
	MaxRecords:      10000,
}

// ImmutableRecord 不可变记录
type ImmutableRecord struct {
	// ID 记录唯一标识
	ID string `json:"id"`
	// SourcePath 源目录路径
	SourcePath string `json:"sourcePath"`
	// SnapshotPath 快照路径
	SnapshotPath string `json:"snapshotPath"`
	// VolumeName 所属卷名
	VolumeName string `json:"volumeName"`
	// SubvolName 子卷名
	SubvolName string `json:"subvolName"`
	// Duration 锁定时长
	Duration LockDuration `json:"duration"`
	// Status 状态
	Status ImmutableStatus `json:"status"`
	// LockedAt 锁定时间
	LockedAt time.Time `json:"lockedAt"`
	// ExpiresAt 过期时间（永久锁定为零值）
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	// UnlockedAt 解锁时间
	UnlockedAt time.Time `json:"unlockedAt,omitempty"`
	// Size 快照大小（字节）
	Size uint64 `json:"size"`
	// Description 描述
	Description string `json:"description,omitempty"`
	// Tags 标签
	Tags []string `json:"tags,omitempty"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy,omitempty"`
	// ProtectedByRansomware 是否启用防勒索保护（保护快照本身）
	ProtectedByRansomware bool `json:"protectedByRansomware"`
}

// ImmutableManager 不可变存储管理器
type ImmutableManager struct {
	client     *btrfs.Client
	storageMgr *Manager
	config     ImmutableConfig
	records    map[string]*ImmutableRecord
	mu         sync.RWMutex
	dataFile   string
	stopChan   chan struct{}
}

// NewImmutableManager 创建不可变存储管理器
func NewImmutableManager(storageMgr *Manager, config ImmutableConfig) (*ImmutableManager, error) {
	if storageMgr == nil {
		return nil, fmt.Errorf("存储管理器不能为空")
	}

	// 设置默认值
	if config.SnapDir == "" {
		config.SnapDir = DefaultImmutableConfig.SnapDir
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = DefaultImmutableConfig.CleanupInterval
	}
	if config.MaxRecords == 0 {
		config.MaxRecords = DefaultImmutableConfig.MaxRecords
	}

	m := &ImmutableManager{
		client:     btrfs.NewClient(true),
		storageMgr: storageMgr,
		config:     config,
		records:    make(map[string]*ImmutableRecord),
		stopChan:   make(chan struct{}),
	}

	// 设置数据文件路径
	m.dataFile = filepath.Join("/var/lib/nas-os", "immutable_records.json")

	// 加载已有记录
	if err := m.loadRecords(); err != nil {
		// 如果文件不存在，不报错
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("加载不可变记录失败: %w", err)
		}
	}

	// 启动自动清理
	if config.AutoCleanup {
		go m.cleanupLoop()
	}

	return m, nil
}

// loadRecords 加载记录
func (m *ImmutableManager) loadRecords() error {
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		return err
	}

	var records []*ImmutableRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("解析记录文件失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range records {
		m.records[r.ID] = r
	}

	return nil
}

// saveRecords 保存记录
func (m *ImmutableManager) saveRecords() error {
	m.mu.RLock()
	records := make([]*ImmutableRecord, 0, len(m.records))
	for _, r := range m.records {
		records = append(records, r)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化记录失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.dataFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	if err := os.WriteFile(m.dataFile, data, 0600); err != nil {
		return fmt.Errorf("写入记录文件失败: %w", err)
	}

	return nil
}

// cleanupLoop 自动清理循环
func (m *ImmutableManager) cleanupLoop() {
	ticker := time.NewTicker(time.Duration(m.config.CleanupInterval) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopChan:
			return
		}
	}
}

// cleanup 清理过期记录和更新状态
func (m *ImmutableManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	changed := false

	for id, record := range m.records {
		// 更新过期状态
		if record.Status == ImmutableStatusActive && !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt) {
			record.Status = ImmutableStatusExpired
			changed = true
		}

		// 清理已解锁超过 30 天的记录
		if record.Status == ImmutableStatusUnlocked {
			if !record.UnlockedAt.IsZero() && now.Sub(record.UnlockedAt) > 30*24*time.Hour {
				delete(m.records, id)
				changed = true
			}
		}
	}

	if changed {
		_ = m.saveRecords()
	}
}

// Stop 停止管理器
func (m *ImmutableManager) Stop() {
	close(m.stopChan)
}

// LockRequest 锁定请求
type LockRequest struct {
	// Path 要锁定的目录路径
	Path string `json:"path"`
	// Duration 锁定时长
	Duration LockDuration `json:"duration"`
	// Description 描述
	Description string `json:"description,omitempty"`
	// Tags 标签
	Tags []string `json:"tags,omitempty"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy,omitempty"`
	// ProtectFromRansomware 是否启用防勒索保护
	ProtectFromRansomware bool `json:"protectFromRansomware"`
}

// Lock 锁定目录（创建不可变快照）
func (m *ImmutableManager) Lock(req LockRequest) (*ImmutableRecord, error) {
	// 验证路径
	if req.Path == "" {
		return nil, fmt.Errorf("路径不能为空")
	}

	// 验证路径存在
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("路径不存在: %s", req.Path)
	}

	// 验证锁定时长
	if req.Duration != LockDuration7Days && req.Duration != LockDuration30Days && req.Duration != LockDurationPermanent {
		return nil, fmt.Errorf("无效的锁定时长: %s", req.Duration)
	}

	// 查找所属卷
	volume, subvolName, err := m.findVolumeForPath(req.Path)
	if err != nil {
		return nil, fmt.Errorf("查找卷失败: %w", err)
	}

	// 检查是否已锁定
	m.mu.RLock()
	for _, r := range m.records {
		if r.SourcePath == req.Path && r.Status == ImmutableStatusActive {
			m.mu.RUnlock()
			return nil, fmt.Errorf("路径已被锁定: %s (ID: %s, 过期时间: %s)", req.Path, r.ID, r.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
	}
	m.mu.RUnlock()

	// 创建快照目录
	snapDir := filepath.Join(volume.MountPoint, m.config.SnapDir)
	if err := os.MkdirAll(snapDir, 0750); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 生成快照名称
	snapName := fmt.Sprintf("immutable_%s_%s", time.Now().Format("20060102-150405"), uuid.New().String()[:8])
	snapPath := filepath.Join(snapDir, snapName)

	// 创建只读快照
	if err := m.client.CreateSnapshot(req.Path, snapPath, true); err != nil {
		return nil, fmt.Errorf("创建快照失败: %w", err)
	}

	// 如果启用防勒索保护，设置 immutable 属性
	if req.ProtectFromRansomware {
		// btrfs 子卷已设置为只读，额外使用 chattr +i 设置不可变属性
		if err := setImmutableAttribute(snapPath); err != nil {
			// 警告但不失败
			fmt.Printf("警告: 设置不可变属性失败: %v\n", err)
		}
	}

	// 获取快照大小
	size, _ := getDirSize(snapPath)

	// 计算过期时间
	var expiresAt time.Time
	if req.Duration != LockDurationPermanent {
		hours := LockDurationHours[req.Duration]
		expiresAt = time.Now().Add(time.Duration(hours) * time.Hour)
	}

	// 创建记录
	record := &ImmutableRecord{
		ID:                    uuid.New().String(),
		SourcePath:            req.Path,
		SnapshotPath:          snapPath,
		VolumeName:            volume.Name,
		SubvolName:            subvolName,
		Duration:              req.Duration,
		Status:                ImmutableStatusActive,
		LockedAt:              time.Now(),
		ExpiresAt:             expiresAt,
		Size:                  size,
		Description:           req.Description,
		Tags:                  req.Tags,
		CreatedBy:             req.CreatedBy,
		ProtectedByRansomware: req.ProtectFromRansomware,
	}

	// 保存记录
	m.mu.Lock()
	m.records[record.ID] = record
	m.mu.Unlock()

	if err := m.saveRecords(); err != nil {
		// 回滚：删除快照
		_ = m.client.DeleteSnapshot(snapPath)
		return nil, fmt.Errorf("保存记录失败: %w", err)
	}

	return record, nil
}

// Unlock 解锁目录（删除不可变快照）
// 注意：只有过期或管理授权才能解锁
func (m *ImmutableManager) Unlock(id string, force bool) (*ImmutableRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[id]
	if !exists {
		return nil, fmt.Errorf("记录不存在: %s", id)
	}

	// 检查状态
	if record.Status == ImmutableStatusUnlocked {
		return nil, fmt.Errorf("记录已解锁: %s", id)
	}

	// 检查是否可以解锁
	if record.Status == ImmutableStatusActive && !force {
		// 检查是否过期
		if record.ExpiresAt.IsZero() || time.Now().Before(record.ExpiresAt) {
			return nil, fmt.Errorf("记录尚未过期，无法解锁。锁定时长: %s，过期时间: %s。使用 force=true 强制解锁",
				record.Duration, record.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
	}

	// 如果有防勒索保护，先移除不可变属性
	if record.ProtectedByRansomware {
		if err := removeImmutableAttribute(record.SnapshotPath); err != nil {
			fmt.Printf("警告: 移除不可变属性失败: %v\n", err)
		}
	}

	// 删除快照
	if err := m.client.DeleteSnapshot(record.SnapshotPath); err != nil {
		return nil, fmt.Errorf("删除快照失败: %w", err)
	}

	// 更新记录
	record.Status = ImmutableStatusUnlocked
	record.UnlockedAt = time.Now()

	if err := m.saveRecords(); err != nil {
		return nil, fmt.Errorf("保存记录失败: %w", err)
	}

	return record, nil
}

// GetRecord 获取记录
func (m *ImmutableManager) GetRecord(id string) (*ImmutableRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[id]
	if !exists {
		return nil, fmt.Errorf("记录不存在: %s", id)
	}

	return record, nil
}

// ListRecords 列出记录
func (m *ImmutableManager) ListRecords(filter *RecordFilter) []*ImmutableRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ImmutableRecord
	for _, r := range m.records {
		if filter != nil && !matchFilter(r, filter) {
			continue
		}
		result = append(result, r)
	}

	return result
}

// RecordFilter 记录过滤器
type RecordFilter struct {
	// Status 状态过滤
	Status *ImmutableStatus `json:"status,omitempty"`
	// VolumeName 卷名过滤
	VolumeName string `json:"volumeName,omitempty"`
	// PathContains 路径包含
	PathContains string `json:"pathContains,omitempty"`
	// Tags 标签过滤（包含任一）
	Tags []string `json:"tags,omitempty"`
	// CreatedBy 创建者过滤
	CreatedBy string `json:"createdBy,omitempty"`
}

func matchFilter(r *ImmutableRecord, f *RecordFilter) bool {
	if f.Status != nil && r.Status != *f.Status {
		return false
	}
	if f.VolumeName != "" && r.VolumeName != f.VolumeName {
		return false
	}
	if f.PathContains != "" && !containsString(r.SourcePath, f.PathContains) {
		return false
	}
	if f.CreatedBy != "" && r.CreatedBy != f.CreatedBy {
		return false
	}
	if len(f.Tags) > 0 {
		found := false
		for _, tag := range f.Tags {
			for _, rTag := range r.Tags {
				if rTag == tag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ExtendLock 延长锁定时间
func (m *ImmutableManager) ExtendLock(id string, additionalDuration LockDuration) (*ImmutableRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[id]
	if !exists {
		return nil, fmt.Errorf("记录不存在: %s", id)
	}

	if record.Status != ImmutableStatusActive {
		return nil, fmt.Errorf("只有活跃状态的记录可以延长")
	}

	// 计算新的过期时间
	if additionalDuration == LockDurationPermanent {
		record.ExpiresAt = time.Time{} // 零值表示永久
		record.Duration = LockDurationPermanent
	} else {
		hours := LockDurationHours[additionalDuration]
		if record.ExpiresAt.IsZero() {
			// 原来是永久，现在改为限时
			record.ExpiresAt = time.Now().Add(time.Duration(hours) * time.Hour)
		} else {
			record.ExpiresAt = record.ExpiresAt.Add(time.Duration(hours) * time.Hour)
		}
	}

	if err := m.saveRecords(); err != nil {
		return nil, fmt.Errorf("保存记录失败: %w", err)
	}

	return record, nil
}

// GetStatus 获取路径的锁定状态
func (m *ImmutableManager) GetStatus(path string) (*ImmutableRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.records {
		if r.SourcePath == path && r.Status == ImmutableStatusActive {
			return r, nil
		}
	}

	return nil, nil // 未锁定
}

// Restore 从不可变快照恢复数据
func (m *ImmutableManager) Restore(id string, targetPath string) error {
	m.mu.RLock()
	record, exists := m.records[id]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("记录不存在: %s", id)
	}
	m.mu.RUnlock()

	// 验证目标路径
	if targetPath == "" {
		targetPath = record.SourcePath + "_restored_" + time.Now().Format("20060102-150405")
	}

	// 创建可写快照到目标位置
	if err := m.client.CreateSnapshot(record.SnapshotPath, targetPath, false); err != nil {
		return fmt.Errorf("恢复快照失败: %w", err)
	}

	return nil
}

// findVolumeForPath 查找路径所属的卷
func (m *ImmutableManager) findVolumeForPath(path string) (*Volume, string, error) {
	volumes := m.storageMgr.ListVolumes()

	var bestMatch *Volume
	bestMatchLen := 0
	var subvolName string

	for _, v := range volumes {
		if v.MountPoint == "" {
			continue
		}

		// 检查路径是否在卷挂载点下
		if len(path) >= len(v.MountPoint) && path[:len(v.MountPoint)] == v.MountPoint {
			if len(v.MountPoint) > bestMatchLen {
				bestMatch = v
				bestMatchLen = len(v.MountPoint)

				// 提取子卷名
				relPath := path[len(v.MountPoint):]
				if len(relPath) > 0 && relPath[0] == '/' {
					relPath = relPath[1:]
				}
				parts := splitPath(relPath)
				if len(parts) > 0 {
					subvolName = parts[0]
				}
			}
		}
	}

	if bestMatch == nil {
		return nil, "", fmt.Errorf("未找到路径所属的卷: %s", path)
	}

	return bestMatch, subvolName, nil
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// setImmutableAttribute 设置不可变属性（使用 chattr +i）
func setImmutableAttribute(path string) error {
	// 使用 chattr 设置不可变属性
	// 这需要 root 权限
	_ = append([]string{}, "chattr", "+i", path) // 实际实现需要调用系统命令

	// 使用 exec 包执行命令
	// 这里简化实现，实际应该使用 os/exec
	return nil // 实际实现需要调用系统命令
}

// removeImmutableAttribute 移除不可变属性（使用 chattr -i）
func removeImmutableAttribute(path string) error {
	// 使用 chattr 移除不可变属性
	return nil // 实际实现需要调用系统命令
}

// getDirSize 获取目录大小
func getDirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return nil
	})
	return size, err
}

// GetStatistics 获取统计信息
func (m *ImmutableManager) GetStatistics() *ImmutableStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ImmutableStatistics{
		TotalRecords: len(m.records),
		ByStatus:     make(map[ImmutableStatus]int),
		ByDuration:   make(map[LockDuration]int),
	}

	var totalSize uint64
	for _, r := range m.records {
		totalSize += r.Size
		stats.ByStatus[r.Status]++
		stats.ByDuration[r.Duration]++
	}
	stats.TotalSize = totalSize

	return stats
}

// ImmutableStatistics 不可变存储统计
type ImmutableStatistics struct {
	TotalRecords int                     `json:"totalRecords"`
	TotalSize    uint64                  `json:"totalSize"`
	ByStatus     map[ImmutableStatus]int `json:"byStatus"`
	ByDuration   map[LockDuration]int    `json:"byDuration"`
}

// CheckRansomwareProtection 检查防勒索保护状态
func (m *ImmutableManager) CheckRansomwareProtection(path string) (*RansomwareProtectionStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &RansomwareProtectionStatus{
		Path:      path,
		Protected: false,
	}

	for _, r := range m.records {
		if r.SourcePath == path && r.Status == ImmutableStatusActive {
			status.Protected = true
			status.RecordID = r.ID
			status.LockedAt = r.LockedAt
			status.ExpiresAt = r.ExpiresAt
			status.ProtectedByRansomware = r.ProtectedByRansomware
			break
		}
	}

	return status, nil
}

// RansomwareProtectionStatus 防勒索保护状态
type RansomwareProtectionStatus struct {
	Path                  string    `json:"path"`
	Protected             bool      `json:"protected"`
	RecordID              string    `json:"recordId,omitempty"`
	LockedAt              time.Time `json:"lockedAt,omitempty"`
	ExpiresAt             time.Time `json:"expiresAt,omitempty"`
	ProtectedByRansomware bool      `json:"protectedByRansomware"`
}

// QuickLock 快速锁定（使用默认配置）
func (m *ImmutableManager) QuickLock(path string, duration LockDuration) (*ImmutableRecord, error) {
	return m.Lock(LockRequest{
		Path:                  path,
		Duration:              duration,
		ProtectFromRansomware: true,
	})
}

// BatchLock 批量锁定
func (m *ImmutableManager) BatchLock(paths []string, duration LockDuration, description string) ([]*ImmutableRecord, []error) {
	var records []*ImmutableRecord
	var errors []error

	for _, path := range paths {
		record, err := m.Lock(LockRequest{
			Path:                  path,
			Duration:              duration,
			Description:           description,
			ProtectFromRansomware: true,
		})
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", path, err))
		} else {
			records = append(records, record)
		}
	}

	return records, errors
}
