// Package systemsnapshot 提供系统配置快照与回滚管理
// 支持网络、存储、服务等配置的快照、一键回滚、差异对比、自动快照
package systemsnapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	// Version 模块版本
	Version = "1.0.0"

	// DefaultSnapshotDir 默认快照存储目录
	DefaultSnapshotDir = "/var/lib/nas-os/snapshots"

	// MaxSnapshots 最大快照数量
	MaxSnapshots = 100

	// MaxSnapshotSize 单个快照最大大小 (100MB)
	MaxSnapshotSize = 100 * 1024 * 1024

	// DefaultRetentionDays 默认保留天数
	DefaultRetentionDays = 30

	// CleanupInterval 清理检查间隔
	CleanupInterval = 24 * time.Hour

	// ConfigVersion 配置格式版本
	ConfigVersion = "1.0"
)

// ========== 快照类型 ==========

// SnapshotType 快照类型
type SnapshotType string

const (
	SnapshotTypeManual    SnapshotType = "manual"     // 手动快照
	SnapshotTypeAuto      SnapshotType = "auto"       // 自动快照
	SnapshotTypePreUpdate SnapshotType = "pre_update"  // 更新前快照
	SnapshotTypePreChange SnapshotType = "pre_change"  // 配置变更前快照
	SnapshotTypeScheduled SnapshotType = "scheduled"   // 定时快照
)

// ========== 快照状态 ==========

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	StatusCreating  SnapshotStatus = "creating"
	StatusReady     SnapshotStatus = "ready"
	StatusRestoring SnapshotStatus = "restoring"
	StatusRestored  SnapshotStatus = "restored"
	StatusFailed    SnapshotStatus = "failed"
	StatusCorrupted SnapshotStatus = "corrupted"
	StatusExpired   SnapshotStatus = "expired"
)

// ========== 配置类别 ==========

// ConfigCategory 配置类别
type ConfigCategory string

const (
	CategoryNetwork  ConfigCategory = "network"  // 网络配置
	CategoryStorage  ConfigCategory = "storage"  // 存储配置
	CategoryService  ConfigCategory = "service"  // 服务配置
	CategorySecurity ConfigCategory = "security" // 安全配置
	CategorySystem   ConfigCategory = "system"   // 系统配置
	CategoryDocker   ConfigCategory = "docker"   // Docker配置
	CategoryShare    ConfigCategory = "share"    // 共享配置
	CategoryUser     ConfigCategory = "user"     // 用户配置
	CategoryPlugin   ConfigCategory = "plugin"   // 插件配置
	CategoryOther    ConfigCategory = "other"    // 其他配置
)

// ========== 变更类型 ==========

// ChangeType 变更类型
type ChangeType string

const (
	ChangeTypeAdd    ChangeType = "add"    // 新增
	ChangeTypeModify ChangeType = "modify" // 修改
	ChangeTypeDelete ChangeType = "delete" // 删除
)

// ========== 数据结构 ==========

// Snapshot 系统快照
type Snapshot struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Type        SnapshotType     `json:"type"`
	Status      SnapshotStatus   `json:"status"`
	Version     string           `json:"version"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	ExpiresAt   *time.Time       `json:"expires_at,omitempty"`
	Size        int64            `json:"size"`
	Checksum    string           `json:"checksum"`
	Tags        []string         `json:"tags,omitempty"`
	Configs     []ConfigItem     `json:"configs"`
	Metadata    SnapshotMetadata `json:"metadata"`
	Error       string           `json:"error,omitempty"`
}

// ConfigItem 配置项
type ConfigItem struct {
	Category   ConfigCategory `json:"category"`
	Path       string         `json:"path"`
	Content    string         `json:"content"`
	Checksum   string         `json:"checksum"`
	Permission os.FileMode    `json:"permission"`
	Owner      string         `json:"owner"`
	ModTime    time.Time      `json:"mod_time"`
	Size       int64          `json:"size"`
}

// SnapshotMetadata 快照元数据
type SnapshotMetadata struct {
	Hostname      string            `json:"hostname"`
	OSVersion     string            `json:"os_version"`
	KernelVersion string            `json:"kernel_version"`
	Platform      string            `json:"platform"`
	Architecture  string            `json:"architecture"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// SnapshotDiff 快照差异
type SnapshotDiff struct {
	ID           string       `json:"id"`
	SnapshotA    string       `json:"snapshot_a"`
	SnapshotB    string       `json:"snapshot_b"`
	CreatedAt    time.Time    `json:"created_at"`
	TotalChanges int          `json:"total_changes"`
	Changes      []ConfigDiff `json:"changes"`
	Summary      DiffSummary  `json:"summary"`
}

// ConfigDiff 配置差异
type ConfigDiff struct {
	Category    ConfigCategory `json:"category"`
	Path        string         `json:"path"`
	ChangeType  ChangeType     `json:"change_type"`
	OldContent  string         `json:"old_content,omitempty"`
	NewContent  string         `json:"new_content,omitempty"`
	OldChecksum string         `json:"old_checksum,omitempty"`
	NewChecksum string         `json:"new_checksum,omitempty"`
	DiffLines   []DiffLine     `json:"diff_lines,omitempty"`
}

// DiffLine 差异行
type DiffLine struct {
	LineNum int    `json:"line_num"`
	Type    string `json:"type"` // "add", "delete", "context"
	Content string `json:"content"`
}

// DiffSummary 差异摘要
type DiffSummary struct {
	Added     int `json:"added"`
	Modified  int `json:"modified"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	SnapshotID   string           `json:"snapshot_id"`
	Categories   []ConfigCategory `json:"categories,omitempty"`
	DryRun       bool             `json:"dry_run"`
	Force        bool             `json:"force"`
	CreateBackup bool             `json:"create_backup"`
}

// RestoreResult 恢复结果
type RestoreResult struct {
	RequestID     string         `json:"request_id"`
	SnapshotID    string         `json:"snapshot_id"`
	BackupID      string         `json:"backup_id,omitempty"`
	Success       bool           `json:"success"`
	TotalItems    int            `json:"total_items"`
	RestoredItems int            `json:"restored_items"`
	FailedItems   int            `json:"failed_items"`
	SkippedItems  int            `json:"skipped_items"`
	Duration      time.Duration  `json:"duration"`
	Errors        []RestoreError `json:"errors,omitempty"`
	DryRun        bool           `json:"dry_run"`
}

// RestoreError 恢复错误
type RestoreError struct {
	Path    string `json:"path"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// CleanupPolicy 清理策略
type CleanupPolicy struct {
	Enabled         bool          `json:"enabled"`
	MaxAge          time.Duration `json:"max_age"`
	MaxCount        int           `json:"max_count"`
	KeepMinimum     int           `json:"keep_minimum"`
	KeepTags        []string      `json:"keep_tags"`
	AutoCleanup     bool          `json:"auto_cleanup"`
	CleanupSchedule string        `json:"cleanup_schedule"`
}

// AutoSnapshotConfig 自动快照配置
type AutoSnapshotConfig struct {
	Enabled          bool     `json:"enabled"`
	OnUpdate         bool     `json:"on_update"`
	OnConfigChange   bool     `json:"on_config_change"`
	Schedule         string   `json:"schedule"`
	MaxAutoSnapshots int      `json:"max_auto_snapshots"`
	Tags             []string `json:"tags"`
}

// PreviewResult 预览结果
type PreviewResult struct {
	SnapshotID   string            `json:"snapshot_id"`
	Valid        bool              `json:"valid"`
	TotalItems   int               `json:"total_items"`
	ValidItems   int               `json:"valid_items"`
	InvalidItems int               `json:"invalid_items"`
	Issues       []ValidationIssue `json:"issues,omitempty"`
	Preview      []ConfigItem      `json:"preview"`
}

// ValidationIssue 验证问题
type ValidationIssue struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ListOptions 列表选项
type ListOptions struct {
	Type     SnapshotType   `json:"type,omitempty"`
	Status   SnapshotStatus `json:"status,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	SortBy   string         `json:"sort_by,omitempty"`
	SortDesc bool           `json:"sort_desc"`
	Offset   int            `json:"offset"`
	Limit    int            `json:"limit"`
}

// ========== 管理器 ==========

// SnapshotManager 快照管理器
type SnapshotManager struct {
	mu            sync.RWMutex
	snapshots     map[string]*Snapshot
	diffs         map[string]*SnapshotDiff
	baseDir       string
	cleanupPolicy CleanupPolicy
	autoConfig    AutoSnapshotConfig
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// SnapshotManagerConfig 快照管理器配置
type SnapshotManagerConfig struct {
	BaseDir       string             `json:"base_dir"`
	CleanupPolicy CleanupPolicy      `json:"cleanup_policy"`
	AutoConfig    AutoSnapshotConfig `json:"auto_config"`
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(config SnapshotManagerConfig) *SnapshotManager {
	if config.BaseDir == "" {
		config.BaseDir = DefaultSnapshotDir
	}

	sm := &SnapshotManager{
		snapshots:     make(map[string]*Snapshot),
		diffs:         make(map[string]*SnapshotDiff),
		baseDir:       config.BaseDir,
		cleanupPolicy: config.CleanupPolicy,
		autoConfig:    config.AutoConfig,
		stopCleanup:   make(chan struct{}),
	}

	os.MkdirAll(sm.baseDir, 0755)
	sm.loadSnapshots()

	if config.CleanupPolicy.Enabled && config.CleanupPolicy.AutoCleanup {
		sm.startAutoCleanup()
	}

	return sm
}

// CreateSnapshot 创建快照（同步）
func (sm *SnapshotManager) CreateSnapshot(ctx context.Context, name, description string, snapType SnapshotType, categories []ConfigCategory, tags []string) (*Snapshot, error) {
	sm.mu.Lock()

	if len(sm.snapshots) >= MaxSnapshots {
		sm.mu.Unlock()
		return nil, fmt.Errorf("maximum snapshot count (%d) reached", MaxSnapshots)
	}

	snapshot := &Snapshot{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Type:        snapType,
		Status:      StatusCreating,
		Version:     ConfigVersion,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Tags:        tags,
	}

	sm.snapshots[snapshot.ID] = snapshot
	sm.mu.Unlock()

	sm.collectConfigs(ctx, snapshot, categories)

	return snapshot, nil
}

// CreateSnapshotAsync 异步创建快照
func (sm *SnapshotManager) CreateSnapshotAsync(ctx context.Context, name, description string, snapType SnapshotType, categories []ConfigCategory, tags []string) (*Snapshot, error) {
	sm.mu.Lock()

	if len(sm.snapshots) >= MaxSnapshots {
		sm.mu.Unlock()
		return nil, fmt.Errorf("maximum snapshot count (%d) reached", MaxSnapshots)
	}

	snapshot := &Snapshot{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Type:        snapType,
		Status:      StatusCreating,
		Version:     ConfigVersion,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Tags:        tags,
	}

	sm.snapshots[snapshot.ID] = snapshot
	sm.mu.Unlock()

	go sm.collectConfigs(ctx, snapshot, categories)

	return snapshot, nil
}

// CreatePreUpdateSnapshot 创建更新前自动快照
func (sm *SnapshotManager) CreatePreUpdateSnapshot(ctx context.Context) (*Snapshot, error) {
	name := fmt.Sprintf("pre-update-%s", time.Now().Format("20060102-150405"))
	tags := []string{"auto", "pre-update"}
	return sm.CreateSnapshot(ctx, name, "系统更新前自动快照", SnapshotTypePreUpdate, nil, tags)
}

// CreatePreChangeSnapshot 创建配置变更前自动快照
func (sm *SnapshotManager) CreatePreChangeSnapshot(ctx context.Context, category ConfigCategory) (*Snapshot, error) {
	name := fmt.Sprintf("pre-change-%s-%s", string(category), time.Now().Format("20060102-150405"))
	tags := []string{"auto", "pre-change", string(category)}
	categories := []ConfigCategory{category}
	return sm.CreateSnapshot(ctx, name, fmt.Sprintf("配置变更前自动快照 (%s)", category), SnapshotTypePreChange, categories, tags)
}

// collectConfigs 收集系统配置
func (sm *SnapshotManager) collectConfigs(ctx context.Context, snapshot *Snapshot, categories []ConfigCategory) {
	var configs []ConfigItem
	var totalSize int64

	if len(categories) == 0 {
		categories = []ConfigCategory{
			CategoryNetwork, CategoryStorage, CategoryService,
			CategorySecurity, CategorySystem, CategoryDocker,
			CategoryShare, CategoryUser, CategoryPlugin,
		}
	}

	for _, category := range categories {
		select {
		case <-ctx.Done():
			sm.updateSnapshotStatus(snapshot.ID, StatusFailed, "cancelled")
			return
		default:
		}

		items, err := sm.collectCategoryConfigs(category)
		if err != nil {
			continue
		}
		configs = append(configs, items...)
		for _, item := range items {
			totalSize += item.Size
		}
	}

	snapshotData, _ := json.Marshal(configs)
	checksum := sha256.Sum256(snapshotData)

	sm.mu.Lock()
	snapshot.Configs = configs
	snapshot.Size = totalSize
	snapshot.Checksum = hex.EncodeToString(checksum[:])
	snapshot.Status = StatusReady
	snapshot.UpdatedAt = time.Now()
	snapshot.Metadata = sm.collectMetadata()
	sm.mu.Unlock()

	sm.saveSnapshot(snapshot)
}

// collectCategoryConfigs 收集指定类别的配置
func (sm *SnapshotManager) collectCategoryConfigs(category ConfigCategory) ([]ConfigItem, error) {
	var items []ConfigItem
	configPaths := sm.getConfigPaths(category)

	for _, path := range configPaths {
		collected := sm.collectConfigFiles(path, category)
		items = append(items, collected...)
	}

	return items, nil
}

// collectConfigFiles 收集配置文件（支持通配符）
func (sm *SnapshotManager) collectConfigFiles(pathPattern string, category ConfigCategory) []ConfigItem {
	var items []ConfigItem

	if strings.Contains(pathPattern, "*") {
		matches, err := filepath.Glob(pathPattern)
		if err != nil || len(matches) == 0 {
			return nil
		}
		for _, match := range matches {
			item, err := sm.collectConfigFile(match, category)
			if err != nil {
				continue
			}
			items = append(items, *item)
		}
		return items
	}

	item, err := sm.collectConfigFile(pathPattern, category)
	if err != nil {
		return nil
	}
	return append(items, *item)
}

// collectConfigFile 收集单个配置文件
func (sm *SnapshotManager) collectConfigFile(path string, category ConfigCategory) (*ConfigItem, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("path is a directory")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	checksum := sha256.Sum256(content)
	owner := getOwner(path)

	return &ConfigItem{
		Category:   category,
		Path:       path,
		Content:    string(content),
		Checksum:   hex.EncodeToString(checksum[:]),
		Permission: info.Mode(),
		Owner:      owner,
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}, nil
}

// getConfigPaths 获取配置文件路径
func (sm *SnapshotManager) getConfigPaths(category ConfigCategory) []string {
	switch category {
	case CategoryNetwork:
		return []string{
			"/etc/network/interfaces",
			"/etc/hosts",
			"/etc/resolv.conf",
			"/etc/hostname",
			"/etc/sysctl.conf",
			"/etc/NetworkManager/NetworkManager.conf",
			"/etc/netplan/*.yaml",
		}
	case CategoryStorage:
		return []string{
			"/etc/fstab",
			"/etc/mdadm/mdadm.conf",
			"/etc/samba/smb.conf",
			"/etc/nfs.conf",
			"/etc/exports",
		}
	case CategoryService:
		return []string{
			"/etc/systemd/system/*.service",
			"/etc/crontab",
			"/etc/supervisor/supervisord.conf",
		}
	case CategorySecurity:
		return []string{
			"/etc/ssh/sshd_config",
			"/etc/sudoers",
			"/etc/ssl/openssl.cnf",
			"/etc/fail2ban/jail.local",
		}
	case CategorySystem:
		return []string{
			"/etc/locale.conf",
			"/etc/locale.gen",
			"/etc/timezone",
			"/etc/default/grub",
			"/etc/modules",
		}
	case CategoryDocker:
		return []string{
			"/etc/docker/daemon.json",
		}
	case CategoryShare:
		return []string{
			"/etc/samba/smb.conf",
			"/etc/exports",
			"/etc/afp.conf",
			"/etc/avahi/services/*.service",
		}
	case CategoryUser:
		return []string{
			"/etc/passwd",
			"/etc/group",
		}
	case CategoryPlugin:
		return []string{
			"/etc/nas-os/plugins/*.conf",
		}
	default:
		return []string{}
	}
}

// collectMetadata 收集系统元数据
func (sm *SnapshotManager) collectMetadata() SnapshotMetadata {
	hostname, _ := os.Hostname()
	return SnapshotMetadata{
		Hostname:      hostname,
		OSVersion:     getOSVersion(),
		KernelVersion: getKernelVersion(),
		Platform:      runtime.GOOS,
		Architecture:  runtime.GOARCH,
	}
}

// RollbackToSnapshot 回滚到指定快照
func (sm *SnapshotManager) RollbackToSnapshot(ctx context.Context, req RestoreRequest) (*RestoreResult, error) {
	sm.mu.RLock()
	snapshot, exists := sm.snapshots[req.SnapshotID]
	sm.mu.RUnlock()

	if !exists {
		return nil, errors.New("snapshot not found")
	}
	if snapshot.Status != StatusReady {
		return nil, fmt.Errorf("snapshot not ready, status: %s", snapshot.Status)
	}

	startTime := time.Now()
	result := &RestoreResult{
		RequestID:  generateID(),
		SnapshotID: req.SnapshotID,
		DryRun:     req.DryRun,
	}

	sm.updateSnapshotStatus(req.SnapshotID, StatusRestoring, "")

	if req.CreateBackup && !req.DryRun {
		backup, err := sm.CreateSnapshot(ctx,
			fmt.Sprintf("pre-restore-%s", time.Now().Format("20060102-150405")),
			"自动备份 - 回滚前",
			SnapshotTypeAuto,
			nil, []string{"backup", "pre-restore"})
		if err == nil {
			result.BackupID = backup.ID
		}
	}

	configs := sm.filterConfigs(snapshot.Configs, req.Categories)
	result.TotalItems = len(configs)

	for _, config := range configs {
		select {
		case <-ctx.Done():
			result.Success = false
			sm.updateSnapshotStatus(req.SnapshotID, StatusFailed, "cancelled")
			return result, ctx.Err()
		default:
		}

		if req.DryRun {
			result.RestoredItems++
			continue
		}

		err := sm.restoreConfigItem(config)
		if err != nil {
			result.FailedItems++
			result.Errors = append(result.Errors, RestoreError{
				Path:  config.Path,
				Error: err.Error(),
			})
		} else {
			result.RestoredItems++
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = result.FailedItems == 0

	if result.Success {
		sm.updateSnapshotStatus(req.SnapshotID, StatusRestored, "")
	} else {
		sm.updateSnapshotStatus(req.SnapshotID, StatusFailed,
			fmt.Sprintf("%d items failed to restore", result.FailedItems))
	}

	return result, nil
}

// filterConfigs 过滤配置
func (sm *SnapshotManager) filterConfigs(configs []ConfigItem, categories []ConfigCategory) []ConfigItem {
	if len(categories) == 0 {
		return configs
	}

	categorySet := make(map[ConfigCategory]bool)
	for _, c := range categories {
		categorySet[c] = true
	}

	var filtered []ConfigItem
	for _, config := range configs {
		if categorySet[config.Category] {
			filtered = append(filtered, config)
		}
	}
	return filtered
}

// restoreConfigItem 恢复单个配置项
func (sm *SnapshotManager) restoreConfigItem(item ConfigItem) error {
	dir := filepath.Dir(item.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(item.Path, []byte(item.Content), item.Permission); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := os.Chmod(item.Path, item.Permission); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	return nil
}

// CompareSnapshots 对比两个快照
func (sm *SnapshotManager) CompareSnapshots(snapshotA, snapshotB string) (*SnapshotDiff, error) {
	sm.mu.RLock()
	snapA, existsA := sm.snapshots[snapshotA]
	snapB, existsB := sm.snapshots[snapshotB]
	sm.mu.RUnlock()

	if !existsA {
		return nil, fmt.Errorf("snapshot %s not found", snapshotA)
	}
	if !existsB {
		return nil, fmt.Errorf("snapshot %s not found", snapshotB)
	}

	diff := &SnapshotDiff{
		ID:        generateID(),
		SnapshotA: snapshotA,
		SnapshotB: snapshotB,
		CreatedAt: time.Now(),
	}

	configMapA := make(map[string]ConfigItem)
	for _, c := range snapA.Configs {
		configMapA[c.Path] = c
	}
	configMapB := make(map[string]ConfigItem)
	for _, c := range snapB.Configs {
		configMapB[c.Path] = c
	}

	for path, configA := range configMapA {
		if _, exists := configMapB[path]; !exists {
			diff.Changes = append(diff.Changes, ConfigDiff{
				Category:    configA.Category,
				Path:        path,
				ChangeType:  ChangeTypeDelete,
				OldContent:  configA.Content,
				OldChecksum: configA.Checksum,
			})
			diff.Summary.Deleted++
		}
	}

	for path, configB := range configMapB {
		if _, exists := configMapA[path]; !exists {
			diff.Changes = append(diff.Changes, ConfigDiff{
				Category:    configB.Category,
				Path:        path,
				ChangeType:  ChangeTypeAdd,
				NewContent:  configB.Content,
				NewChecksum: configB.Checksum,
			})
			diff.Summary.Added++
		}
	}

	for path, configA := range configMapA {
		configB, exists := configMapB[path]
		if !exists {
			continue
		}
		if configA.Checksum != configB.Checksum {
			diffLines := calculateDiff(configA.Content, configB.Content)
			diff.Changes = append(diff.Changes, ConfigDiff{
				Category:    configA.Category,
				Path:        path,
				ChangeType:  ChangeTypeModify,
				OldContent:  configA.Content,
				NewContent:  configB.Content,
				OldChecksum: configA.Checksum,
				NewChecksum: configB.Checksum,
				DiffLines:   diffLines,
			})
			diff.Summary.Modified++
		} else {
			diff.Summary.Unchanged++
		}
	}

	diff.TotalChanges = len(diff.Changes)

	sm.mu.Lock()
	sm.diffs[diff.ID] = diff
	sm.mu.Unlock()

	return diff, nil
}

// calculateDiff 计算文本差异（行级对比）
func calculateDiff(oldContent, newContent string) []DiffLine {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diffLines []DiffLine
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		switch {
		case i >= len(oldLines):
			diffLines = append(diffLines, DiffLine{LineNum: i + 1, Type: "add", Content: newLines[i]})
		case i >= len(newLines):
			diffLines = append(diffLines, DiffLine{LineNum: i + 1, Type: "delete", Content: oldLines[i]})
		case oldLines[i] != newLines[i]:
			diffLines = append(diffLines, DiffLine{LineNum: i + 1, Type: "delete", Content: oldLines[i]})
			diffLines = append(diffLines, DiffLine{LineNum: i + 1, Type: "add", Content: newLines[i]})
		default:
			diffLines = append(diffLines, DiffLine{LineNum: i + 1, Type: "context", Content: oldLines[i]})
		}
	}
	return diffLines
}

// GetDiff 获取差异记录
func (sm *SnapshotManager) GetDiff(diffID string) (*SnapshotDiff, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	diff, exists := sm.diffs[diffID]
	if !exists {
		return nil, errors.New("diff not found")
	}
	return diff, nil
}

// PreviewSnapshot 预览快照
func (sm *SnapshotManager) PreviewSnapshot(snapshotID string) (*PreviewResult, error) {
	sm.mu.RLock()
	snapshot, exists := sm.snapshots[snapshotID]
	sm.mu.RUnlock()

	if !exists {
		return nil, errors.New("snapshot not found")
	}

	result := &PreviewResult{
		SnapshotID: snapshotID,
		Preview:    snapshot.Configs,
		TotalItems: len(snapshot.Configs),
	}

	for _, config := range snapshot.Configs {
		issue := sm.validateConfigItem(config)
		if issue != nil {
			result.Issues = append(result.Issues, *issue)
			result.InvalidItems++
		} else {
			result.ValidItems++
		}
	}

	result.Valid = result.InvalidItems == 0
	return result, nil
}

// validateConfigItem 验证配置项
func (sm *SnapshotManager) validateConfigItem(item ConfigItem) *ValidationIssue {
	if _, err := os.Stat(item.Path); os.IsNotExist(err) {
		return &ValidationIssue{
			Path:     item.Path,
			Category: string(item.Category),
			Severity: "warning",
			Message:  "配置文件不存在，将创建新文件",
		}
	}

	currentContent, err := os.ReadFile(item.Path)
	if err != nil {
		return &ValidationIssue{
			Path:     item.Path,
			Category: string(item.Category),
			Severity: "error",
			Message:  fmt.Sprintf("无法读取当前配置: %v", err),
		}
	}

	currentChecksum := sha256.Sum256(currentContent)
	if hex.EncodeToString(currentChecksum[:]) == item.Checksum {
		return &ValidationIssue{
			Path:     item.Path,
			Category: string(item.Category),
			Severity: "info",
			Message:  "配置未变更，将跳过",
		}
	}

	return nil
}

// GetSnapshot 获取快照
func (sm *SnapshotManager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	snapshot, exists := sm.snapshots[snapshotID]
	if !exists {
		return nil, errors.New("snapshot not found")
	}
	return snapshot, nil
}

// ListSnapshots 列出快照
func (sm *SnapshotManager) ListSnapshots(opts ListOptions) []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var snapshots []*Snapshot
	for _, s := range sm.snapshots {
		if sm.matchesFilter(s, opts) {
			snapshots = append(snapshots, s)
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		switch opts.SortBy {
		case "name":
			if opts.SortDesc {
				return snapshots[i].Name > snapshots[j].Name
			}
			return snapshots[i].Name < snapshots[j].Name
		case "size":
			if opts.SortDesc {
				return snapshots[i].Size > snapshots[j].Size
			}
			return snapshots[i].Size < snapshots[j].Size
		default:
			if opts.SortDesc {
				return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
			}
			return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
		}
	})

	if opts.Offset > 0 && opts.Offset < len(snapshots) {
		snapshots = snapshots[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(snapshots) {
		snapshots = snapshots[:opts.Limit]
	}
	return snapshots
}

// matchesFilter 匹配过滤条件
func (sm *SnapshotManager) matchesFilter(s *Snapshot, opts ListOptions) bool {
	if opts.Type != "" && s.Type != opts.Type {
		return false
	}
	if opts.Status != "" && s.Status != opts.Status {
		return false
	}
	if len(opts.Tags) > 0 {
		for _, tag := range opts.Tags {
			found := false
			for _, t := range s.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// DeleteSnapshot 删除快照
func (sm *SnapshotManager) DeleteSnapshot(snapshotID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot, exists := sm.snapshots[snapshotID]
	if !exists {
		return errors.New("snapshot not found")
	}
	if sm.isProtected(snapshot) {
		return errors.New("snapshot is protected and cannot be deleted")
	}

	snapshotFile := filepath.Join(sm.baseDir, snapshotID+".json")
	os.Remove(snapshotFile)
	delete(sm.snapshots, snapshotID)
	return nil
}

// isProtected 检查快照是否受保护
func (sm *SnapshotManager) isProtected(s *Snapshot) bool {
	for _, tag := range s.Tags {
		for _, keepTag := range sm.cleanupPolicy.KeepTags {
			if tag == keepTag {
				return true
			}
		}
	}
	return false
}

// CleanupSnapshots 清理过期快照
func (sm *SnapshotManager) CleanupSnapshots() (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.cleanupPolicy.Enabled {
		return 0, nil
	}

	var toDelete []string
	now := time.Now()
	sortedSnapshots := sm.getSortedSnapshots()

	for i, s := range sortedSnapshots {
		if sm.isProtected(s) {
			continue
		}
		if len(sortedSnapshots)-len(toDelete) <= sm.cleanupPolicy.KeepMinimum {
			break
		}
		if sm.cleanupPolicy.MaxCount > 0 && i >= sm.cleanupPolicy.MaxCount {
			toDelete = append(toDelete, s.ID)
			continue
		}
		if sm.cleanupPolicy.MaxAge > 0 && now.Sub(s.CreatedAt) > sm.cleanupPolicy.MaxAge {
			toDelete = append(toDelete, s.ID)
			continue
		}
		if s.ExpiresAt != nil && now.After(*s.ExpiresAt) {
			toDelete = append(toDelete, s.ID)
		}
	}

	deleted := 0
	for _, id := range toDelete {
		snapshotFile := filepath.Join(sm.baseDir, id+".json")
		os.Remove(snapshotFile)
		delete(sm.snapshots, id)
		deleted++
	}
	return deleted, nil
}

// getSortedSnapshots 获取排序后的快照列表（按时间降序）
func (sm *SnapshotManager) getSortedSnapshots() []*Snapshot {
	snapshots := make([]*Snapshot, 0, len(sm.snapshots))
	for _, s := range sm.snapshots {
		snapshots = append(snapshots, s)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	return snapshots
}

// UpdateCleanupPolicy 更新清理策略
func (sm *SnapshotManager) UpdateCleanupPolicy(policy CleanupPolicy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cleanupPolicy = policy
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	if policy.Enabled && policy.AutoCleanup {
		sm.startAutoCleanup()
	}
}

// UpdateAutoSnapshotConfig 更新自动快照配置
func (sm *SnapshotManager) UpdateAutoSnapshotConfig(config AutoSnapshotConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.autoConfig = config
}

// GetStats 获取快照统计信息
func (sm *SnapshotManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var totalSize int64
	typeCount := make(map[SnapshotType]int)
	statusCount := make(map[SnapshotStatus]int)

	for _, s := range sm.snapshots {
		totalSize += s.Size
		typeCount[s.Type]++
		statusCount[s.Status]++
	}

	return map[string]interface{}{
		"total_snapshots": len(sm.snapshots),
		"total_size":      totalSize,
		"type_count":      typeCount,
		"status_count":    statusCount,
		"total_diffs":     len(sm.diffs),
		"cleanup_policy":  sm.cleanupPolicy,
		"auto_config":     sm.autoConfig,
	}
}

// Close 关闭管理器
func (sm *SnapshotManager) Close() {
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	close(sm.stopCleanup)
}

// ========== 内部辅助方法 ==========

// startAutoCleanup 启动自动清理
func (sm *SnapshotManager) startAutoCleanup() {
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	sm.cleanupTicker = time.NewTicker(CleanupInterval)

	go func() {
		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.CleanupSnapshots()
			case <-sm.stopCleanup:
				return
			}
		}
	}()
}

// updateSnapshotStatus 更新快照状态
func (sm *SnapshotManager) updateSnapshotStatus(snapshotID string, status SnapshotStatus, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, exists := sm.snapshots[snapshotID]; exists {
		s.Status = status
		s.UpdatedAt = time.Now()
		if errMsg != "" {
			s.Error = errMsg
		}
	}
}

// saveSnapshot 持久化快照到文件
func (sm *SnapshotManager) saveSnapshot(snapshot *Snapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	path := filepath.Join(sm.baseDir, snapshot.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// loadSnapshots 从文件加载快照
func (sm *SnapshotManager) loadSnapshots() {
	entries, err := os.ReadDir(sm.baseDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(sm.baseDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		sm.snapshots[snapshot.ID] = &snapshot
	}
}

// ========== 系统信息辅助函数 ==========

// getOSVersion 获取操作系统版本
func getOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "unknown"
}

// getKernelVersion 获取内核版本
func getKernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		return parts[2]
	}
	return "unknown"
}

// getOwner 获取文件所有者
func getOwner(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	_ = info
	// 简化实现：默认返回root
	// 在生产环境中应使用syscall.Stat_t获取uid
	return "root"
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), string(b[:8]))
}
