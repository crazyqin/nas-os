package lock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 版本类型定义 ==========

// VersionType 版本类型
type VersionType int

const (
	// VersionTypeManual 手动版本
	VersionTypeManual VersionType = iota
	// VersionTypeAuto 自动版本
	VersionTypeAuto
	// VersionTypeCheckpoint 检查点版本
	VersionTypeCheckpoint
	// VersionTypeLocked 锁定关联版本
	VersionTypeLocked
)

func (vt VersionType) String() string {
	switch vt {
	case VersionTypeManual:
		return "manual"
	case VersionTypeAuto:
		return "auto"
	case VersionTypeCheckpoint:
		return "checkpoint"
	case VersionTypeLocked:
		return "locked"
	default:
		return "unknown"
	}
}

// ParseVersionType 解析版本类型
func ParseVersionType(s string) VersionType {
	switch s {
	case "manual":
		return VersionTypeManual
	case "auto":
		return VersionTypeAuto
	case "checkpoint":
		return VersionTypeCheckpoint
	case "locked":
		return VersionTypeLocked
	default:
		return VersionTypeManual
	}
}

// ========== 版本状态 ==========

// VersionStatus 版本状态
type VersionStatus int

const (
	// VersionStatusActive 活跃版本
	VersionStatusActive VersionStatus = iota
	// VersionStatusArchived 已归档版本
	VersionStatusArchived
	// VersionStatusDeleted 已删除版本
	VersionStatusDeleted
)

func (vs VersionStatus) String() string {
	switch vs {
	case VersionStatusActive:
		return "active"
	case VersionStatusArchived:
		return "archived"
	case VersionStatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// ========== 文件版本 ==========

// FileVersion 文件版本
type FileVersion struct {
	// ID 版本唯一标识
	ID string `json:"id"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// FileName 文件名
	FileName string `json:"fileName"`
	// VersionNumber 版本号
	VersionNumber int `json:"versionNumber"`
	// VersionType 版本类型
	VersionType VersionType `json:"versionType"`
	// Status 版本状态
	Status VersionStatus `json:"status"`
	// Size 文件大小
	Size int64 `json:"size"`
	// Checksum 文件校验和（SHA256）
	Checksum string `json:"checksum"`
	// DeltaChecksum 增量校验和（与前一个版本的差异）
	DeltaChecksum string `json:"deltaChecksum,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy"`
	// CreatorName 创建者名称
	CreatorName string `json:"creatorName,omitempty"`
	// Description 版本描述
	Description string `json:"description,omitempty"`
	// Tags 版本标签
	Tags []string `json:"tags,omitempty"`
	// LockID 关联的锁ID
	LockID string `json:"lockId,omitempty"`
	// LockType 关联的锁类型
	LockType string `json:"lockType,omitempty"`
	// Metadata 元数据
	Metadata map[string]string `json:"metadata,omitempty"`
	// StoragePath 存储路径（版本数据存储位置）
	StoragePath string `json:"storagePath,omitempty"`
	// IsDelta 是否增量版本
	IsDelta bool `json:"isDelta"`
	// BaseVersionID 基础版本ID（增量版本）
	BaseVersionID string `json:"baseVersionId,omitempty"`
	// DeltaSize 增量大小
	DeltaSize int64 `json:"deltaSize,omitempty"`

	mu sync.RWMutex
}

// NewFileVersion 创建新版本
func NewFileVersion(filePath, fileName string, versionNumber int, createdBy, creatorName string) *FileVersion {
	return &FileVersion{
		ID:            uuid.New().String(),
		FilePath:      filePath,
		FileName:      fileName,
		VersionNumber: versionNumber,
		VersionType:   VersionTypeManual,
		Status:        VersionStatusActive,
		CreatedAt:     time.Now(),
		CreatedBy:     createdBy,
		CreatorName:   creatorName,
		Metadata:      make(map[string]string),
	}
}

// ========== 版本差异 ==========

// VersionDiff 版本差异
type VersionDiff struct {
	// FromVersionID 源版本ID
	FromVersionID string `json:"fromVersionId"`
	// ToVersionID 目标版本ID
	ToVersionID string `json:"toVersionId"`
	// FromVersionNumber 源版本号
	FromVersionNumber int `json:"fromVersionNumber"`
	// ToVersionNumber 目标版本号
	ToVersionNumber int `json:"toVersionNumber"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// FileName 文件名
	FileName string `json:"fileName"`
	// DiffType 差异类型
	DiffType DiffType `json:"diffType"`
	// Changes 变更列表
	Changes []*VersionChange `json:"changes"`
	// Stats 差异统计
	Stats DiffStats `json:"stats"`
	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generatedAt"`
}

// DiffType 差异类型
type DiffType string

const (
	// DiffTypeBinary 二进制差异
	DiffTypeBinary DiffType = "binary"
	// DiffTypeText 文本差异
	DiffTypeText DiffType = "text"
	// DiffTypeNone 无差异
	DiffTypeNone DiffType = "none"
)

// VersionChange 版本变更
type VersionChange struct {
	// Type 变更类型
	Type ChangeType `json:"type"`
	// LineNumber 行号
	LineNumber int `json:"lineNumber,omitempty"`
	// OldContent 旧内容
	OldContent string `json:"oldContent,omitempty"`
	// NewContent 新内容
	NewContent string `json:"newContent,omitempty"`
	// Context 上下文
	Context string `json:"context,omitempty"`
}

// ChangeType 变更类型
type ChangeType string

const (
	// ChangeTypeAdd 新增
	ChangeTypeAdd ChangeType = "add"
	// ChangeTypeDelete 删除
	ChangeTypeDelete ChangeType = "delete"
	// ChangeTypeModify 修改
	ChangeTypeModify ChangeType = "modify"
	// ChangeTypeMove 移动
	ChangeTypeMove ChangeType = "move"
)

// DiffStats 差异统计
type DiffStats struct {
	// LinesAdded 新增行数
	LinesAdded int `json:"linesAdded"`
	// LinesDeleted 删除行数
	LinesDeleted int `json:"linesDeleted"`
	// LinesModified 修改行数
	LinesModified int `json:"linesModified"`
	// FilesChanged 变更文件数
	FilesChanged int `json:"filesChanged"`
	// BytesAdded 新增字节数
	BytesAdded int64 `json:"bytesAdded"`
	// BytesDeleted 删除字节数
	BytesDeleted int64 `json:"bytesDeleted"`
}

// ========== 版本管理器 ==========

// VersionManager 版本管理器
type VersionManager struct {
	config VersionConfig

	// versions 版本存储
	versions sync.Map // map[string]*FileVersion (key: versionID)

	// fileVersions 文件版本索引
	fileVersions sync.Map // map[string][]*FileVersion (key: filePath)

	// latestVersions 最新版本索引
	latestVersions sync.Map // map[string]*FileVersion (key: filePath)

	// storagePath 版本存储路径
	storagePath string
}

// VersionConfig 版本配置
type VersionConfig struct {
	// MaxVersionsPerFile 每个文件最大版本数
	MaxVersionsPerFile int `json:"maxVersionsPerFile"`
	// MaxVersionAge 版本最大保留天数
	MaxVersionAge int `json:"maxVersionAge"`
	// EnableAutoVersion 是否启用自动版本
	EnableAutoVersion bool `json:"enableAutoVersion"`
	// EnableDelta 是否启用增量版本
	EnableDelta bool `json:"enableDelta"`
	// StoragePath 版本存储路径
	StoragePath string `json:"storagePath"`
}

// DefaultVersionConfig 默认配置
func DefaultVersionConfig() VersionConfig {
	return VersionConfig{
		MaxVersionsPerFile: 100,
		MaxVersionAge:      90,
		EnableAutoVersion:  true,
		EnableDelta:        true,
		StoragePath:        "/var/lib/nas-os/versions",
	}
}

// NewVersionManager 创建版本管理器
func NewVersionManager(config VersionConfig) (*VersionManager, error) {
	// 创建存储目录
	if err := os.MkdirAll(config.StoragePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create version storage: %w", err)
	}

	return &VersionManager{
		config:      config,
		storagePath: config.StoragePath,
	}, nil
}

// ========== 版本创建 ==========

// CreateVersion 创建新版本
func (vm *VersionManager) CreateVersion(ctx context.Context, filePath string, createdBy, creatorName string, versionType VersionType, description string) (*FileVersion, error) {
	// 获取当前版本号
	versionNumber := vm.getNextVersionNumber(filePath)

	// 创建版本记录
	version := NewFileVersion(filePath, filepath.Base(filePath), versionNumber, createdBy, creatorName)
	version.VersionType = versionType
	version.Description = description

	// 读取文件内容计算校验和
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	version.Size = info.Size()

	checksum, err := vm.calculateChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	version.Checksum = checksum

	// 存储版本数据
	storagePath, err := vm.storeVersionData(filePath, version)
	if err != nil {
		return nil, fmt.Errorf("failed to store version data: %w", err)
	}
	version.StoragePath = storagePath

	// 检查是否可以创建增量版本
	if vm.config.EnableDelta && versionNumber > 1 {
		latest := vm.GetLatestVersion(filePath)
		if latest != nil {
			version.BaseVersionID = latest.ID
			version.IsDelta = true
			// 计算增量校验和
			deltaChecksum, err := vm.calculateDeltaChecksum(latest.StoragePath, storagePath)
			if err == nil {
				version.DeltaChecksum = deltaChecksum
			}
		}
	}

	// 存储版本记录
	vm.versions.Store(version.ID, version)
	vm.addToFileVersionIndex(filePath, version)
	vm.latestVersions.Store(filePath, version)

	// 清理旧版本
	go vm.cleanupOldVersions(filePath)

	return version, nil
}

// CreateVersionFromLock 从锁创建版本（锁释放时）
func (vm *VersionManager) CreateVersionFromLock(ctx context.Context, filePath, lockID string, createdBy, creatorName string, lockType string) (*FileVersion, error) {
	version, err := vm.CreateVersion(ctx, filePath, createdBy, creatorName, VersionTypeLocked, fmt.Sprintf("Version created when %s lock released", lockType))
	if err != nil {
		return nil, err
	}

	version.LockID = lockID
	version.LockType = lockType

	return version, nil
}

// CreateCheckpoint 创建检查点
func (vm *VersionManager) CreateCheckpoint(ctx context.Context, filePath, createdBy, creatorName, description string) (*FileVersion, error) {
	return vm.CreateVersion(ctx, filePath, createdBy, creatorName, VersionTypeCheckpoint, description)
}

// ========== 版本查询 ==========

// GetVersion 获取版本
func (vm *VersionManager) GetVersion(versionID string) (*FileVersion, error) {
	raw, ok := vm.versions.Load(versionID)
	if !ok {
		return nil, errors.New("version not found")
	}
	version, ok := raw.(*FileVersion)
	if !ok {
		return nil, errors.New("invalid version type")
	}
	return version, nil
}

// GetLatestVersion 获取最新版本
func (vm *VersionManager) GetLatestVersion(filePath string) *FileVersion {
	raw, ok := vm.latestVersions.Load(filePath)
	if !ok {
		return nil
	}
	version, ok := raw.(*FileVersion)
	if !ok {
		return nil
	}
	return version
}

// GetVersionByNumber 按版本号获取版本
func (vm *VersionManager) GetVersionByNumber(filePath string, versionNumber int) (*FileVersion, error) {
	versions := vm.GetFileVersions(filePath)
	for _, v := range versions {
		if v.VersionNumber == versionNumber {
			return v, nil
		}
	}
	return nil, errors.New("version not found")
}

// GetFileVersions 获取文件的所有版本
func (vm *VersionManager) GetFileVersions(filePath string) []*FileVersion {
	raw, ok := vm.fileVersions.Load(filePath)
	if !ok {
		return nil
	}

	versions, ok := raw.(*[]*FileVersion)
	if !ok {
		return nil
	}
	result := make([]*FileVersion, len(*versions))
	copy(result, *versions)

	// 按版本号降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].VersionNumber > result[j].VersionNumber
	})

	return result
}

// ListVersions 列出版本
func (vm *VersionManager) ListVersions(filter *VersionFilter) []*FileVersion {
	var result []*FileVersion

	vm.versions.Range(func(key, value interface{}) bool {
		version, ok := value.(*FileVersion)
		if !ok {
			return true
		}

		if filter != nil {
			if filter.FilePath != "" && version.FilePath != filter.FilePath {
				return true
			}
			if filter.VersionType != 0 && version.VersionType != filter.VersionType {
				return true
			}
			if filter.Status != 0 && version.Status != filter.Status {
				return true
			}
			if filter.CreatedBy != "" && version.CreatedBy != filter.CreatedBy {
				return true
			}
		}

		result = append(result, version)
		return true
	})

	// 按创建时间降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// VersionFilter 版本过滤器
type VersionFilter struct {
	FilePath    string
	VersionType VersionType
	Status      VersionStatus
	CreatedBy   string
	StartTime   *time.Time
	EndTime     *time.Time
}

// ========== 版本恢复 ==========

// RestoreVersion 恢复到指定版本
func (vm *VersionManager) RestoreVersion(ctx context.Context, versionID, restoredBy string) error {
	version, err := vm.GetVersion(versionID)
	if err != nil {
		return err
	}

	// 检查文件是否被锁定
	// 这个检查由调用者完成（带锁管理的版本管理器）

	// 读取版本数据
	data, err := vm.readVersionData(version)
	if err != nil {
		return fmt.Errorf("failed to read version data: %w", err)
	}

	// 备份当前文件（创建恢复前版本）
	if _, err := os.Stat(version.FilePath); err == nil {
		_, _ = vm.CreateVersion(ctx, version.FilePath, restoredBy, "", VersionTypeAuto, "Auto version before restore") //nolint:errcheck // 备份失败不影响恢复
	}

	// 写入文件
	if err := os.WriteFile(version.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// RestorePartial 部分恢复（增量恢复）
func (vm *VersionManager) RestorePartial(ctx context.Context, versionID string, changes []*VersionChange, restoredBy string) error {
	version, err := vm.GetVersion(versionID)
	if err != nil {
		return err
	}

	// 读取当前文件
	currentData, err := os.ReadFile(version.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read current file: %w", err)
	}

	// 应用部分变更
	lines := strings.Split(string(currentData), "\n")
	for _, change := range changes {
		if change.LineNumber > 0 && change.LineNumber <= len(lines) {
			switch change.Type {
			case ChangeTypeAdd:
				lines = append(lines[:change.LineNumber], append([]string{change.NewContent}, lines[change.LineNumber:]...)...)
			case ChangeTypeDelete:
				if change.LineNumber < len(lines) {
					lines = append(lines[:change.LineNumber], lines[change.LineNumber+1:]...)
				}
			case ChangeTypeModify:
				if change.LineNumber <= len(lines) {
					lines[change.LineNumber-1] = change.NewContent
				}
			}
		}
	}

	// 写入文件
	result := strings.Join(lines, "\n")
	if err := os.WriteFile(version.FilePath, []byte(result), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ========== 版本对比 ==========

// CompareVersions 比较两个版本
func (vm *VersionManager) CompareVersions(ctx context.Context, fromVersionID, toVersionID string) (*VersionDiff, error) {
	fromVersion, err := vm.GetVersion(fromVersionID)
	if err != nil {
		return nil, fmt.Errorf("from version not found: %w", err)
	}

	toVersion, err := vm.GetVersion(toVersionID)
	if err != nil {
		return nil, fmt.Errorf("to version not found: %w", err)
	}

	// 读取版本数据
	fromData, err := vm.readVersionData(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to read from version: %w", err)
	}

	toData, err := vm.readVersionData(toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to read to version: %w", err)
	}

	// 创建差异
	diff := &VersionDiff{
		FromVersionID:     fromVersionID,
		ToVersionID:       toVersionID,
		FromVersionNumber: fromVersion.VersionNumber,
		ToVersionNumber:   toVersion.VersionNumber,
		FilePath:          fromVersion.FilePath,
		FileName:          fromVersion.FileName,
		GeneratedAt:       time.Now(),
	}

	// 计算差异
	if bytes.Equal(fromData, toData) {
		diff.DiffType = DiffTypeNone
		return diff, nil
	}

	// 尝试文本差异
	fromLines := strings.Split(string(fromData), "\n")
	toLines := strings.Split(string(toData), "\n")

	// 简单行比较
	diff.DiffType = DiffTypeText
	diff.Changes = vm.computeLineDiff(fromLines, toLines)

	// 计算统计
	for _, c := range diff.Changes {
		switch c.Type {
		case ChangeTypeAdd:
			diff.Stats.LinesAdded++
			diff.Stats.BytesAdded += int64(len(c.NewContent))
		case ChangeTypeDelete:
			diff.Stats.LinesDeleted++
			diff.Stats.BytesDeleted += int64(len(c.OldContent))
		case ChangeTypeModify:
			diff.Stats.LinesModified++
		}
	}
	diff.Stats.FilesChanged = 1

	return diff, nil
}

// CompareWithCurrent 与当前文件比较
func (vm *VersionManager) CompareWithCurrent(ctx context.Context, versionID string) (*VersionDiff, error) {
	version, err := vm.GetVersion(versionID)
	if err != nil {
		return nil, err
	}

	// 读取当前文件
	currentData, err := os.ReadFile(version.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read current file: %w", err)
	}

	// 读取版本数据
	versionData, err := vm.readVersionData(version)
	if err != nil {
		return nil, fmt.Errorf("failed to read version data: %w", err)
	}

	// 创建差异
	diff := &VersionDiff{
		FromVersionID:     versionID,
		FromVersionNumber: version.VersionNumber,
		ToVersionNumber:   0, // 表示当前版本
		FilePath:          version.FilePath,
		FileName:          version.FileName,
		GeneratedAt:       time.Now(),
	}

	if bytes.Equal(currentData, versionData) {
		diff.DiffType = DiffTypeNone
		return diff, nil
	}

	// 计算差异
	fromLines := strings.Split(string(versionData), "\n")
	toLines := strings.Split(string(currentData), "\n")

	diff.DiffType = DiffTypeText
	diff.Changes = vm.computeLineDiff(fromLines, toLines)

	for _, c := range diff.Changes {
		switch c.Type {
		case ChangeTypeAdd:
			diff.Stats.LinesAdded++
		case ChangeTypeDelete:
			diff.Stats.LinesDeleted++
		case ChangeTypeModify:
			diff.Stats.LinesModified++
		}
	}

	return diff, nil
}

// ========== 版本删除 ==========

// DeleteVersion 删除版本
func (vm *VersionManager) DeleteVersion(versionID, deletedBy string) error {
	version, err := vm.GetVersion(versionID)
	if err != nil {
		return err
	}

	// 标记为删除
	version.mu.Lock()
	version.Status = VersionStatusDeleted
	version.mu.Unlock()

	// 删除存储数据
	if version.StoragePath != "" {
		_ = os.Remove(version.StoragePath) //nolint:errcheck // 删除失败不影响版本删除
	}

	// 从索引移除
	vm.removeFromFileVersionIndex(version.FilePath, versionID)
	vm.versions.Delete(versionID)

	return nil
}

// ArchiveVersion 归档版本
func (vm *VersionManager) ArchiveVersion(versionID string) error {
	version, err := vm.GetVersion(versionID)
	if err != nil {
		return err
	}

	version.mu.Lock()
	version.Status = VersionStatusArchived
	version.mu.Unlock()

	return nil
}

// ========== 内部方法 ==========

func (vm *VersionManager) getNextVersionNumber(filePath string) int {
	versions := vm.GetFileVersions(filePath)
	if len(versions) == 0 {
		return 1
	}
	return versions[0].VersionNumber + 1
}

func (vm *VersionManager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // 关闭错误不影响主流程

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (vm *VersionManager) calculateDeltaChecksum(oldPath, newPath string) (string, error) {
	// 简单实现：使用两个文件校验和的组合
	oldData, err := os.ReadFile(oldPath)
	if err != nil {
		return "", err
	}

	newData, err := os.ReadFile(newPath)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	hash.Write(oldData)
	hash.Write([]byte("->"))
	hash.Write(newData)

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (vm *VersionManager) storeVersionData(filePath string, version *FileVersion) (string, error) {
	// 创建版本存储目录
	versionDir := filepath.Join(vm.storagePath, version.ID[:2], version.ID[2:4])
	if err := os.MkdirAll(versionDir, 0750); err != nil {
		return "", err
	}

	// 读取源文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 写入版本文件
	storagePath := filepath.Join(versionDir, version.ID)
	if err := os.WriteFile(storagePath, data, 0640); err != nil {
		return "", err
	}

	// 写入元数据
	metadata := map[string]interface{}{
		"id":            version.ID,
		"filePath":      version.FilePath,
		"versionNumber": version.VersionNumber,
		"checksum":      version.Checksum,
		"createdAt":     version.CreatedAt,
		"createdBy":     version.CreatedBy,
	}
	metadataJSON, _ := json.Marshal(metadata)
	_ = os.WriteFile(storagePath+".meta", metadataJSON, 0640) //nolint:errcheck // 元数据写入失败不影响主流程

	return storagePath, nil
}

func (vm *VersionManager) readVersionData(version *FileVersion) ([]byte, error) {
	if version.StoragePath == "" {
		return nil, errors.New("no storage path for version")
	}
	return os.ReadFile(version.StoragePath)
}

func (vm *VersionManager) addToFileVersionIndex(filePath string, version *FileVersion) {
	raw, _ := vm.fileVersions.LoadOrStore(filePath, &[]*FileVersion{})
	versions, ok := raw.(*[]*FileVersion)
	if ok {
		*versions = append(*versions, version)
	}
}

func (vm *VersionManager) removeFromFileVersionIndex(filePath, versionID string) {
	raw, ok := vm.fileVersions.Load(filePath)
	if !ok {
		return
	}
	versions, ok := raw.(*[]*FileVersion)
	if !ok {
		return
	}
	for i, v := range *versions {
		if v.ID == versionID {
			*versions = append((*versions)[:i], (*versions)[i+1:]...)
			return
		}
	}
}

func (vm *VersionManager) cleanupOldVersions(filePath string) {
	if vm.config.MaxVersionsPerFile <= 0 {
		return
	}

	versions := vm.GetFileVersions(filePath)
	if len(versions) <= vm.config.MaxVersionsPerFile {
		return
	}

	// 删除最旧的版本（保留检查点）
	toDelete := len(versions) - vm.config.MaxVersionsPerFile
	for i := len(versions) - 1; i >= 0 && toDelete > 0; i-- {
		if versions[i].VersionType != VersionTypeCheckpoint {
			_ = vm.DeleteVersion(versions[i].ID, "system") //nolint:errcheck // 清理失败不影响主流程
			toDelete--
		}
	}
}

func (vm *VersionManager) computeLineDiff(oldLines, newLines []string) []*VersionChange {
	var changes []*VersionChange

	// 简单的行差异算法
	oldMap := make(map[string][]int)
	for i, line := range oldLines {
		oldMap[line] = append(oldMap[line], i)
	}

	seenInNew := make(map[int]bool)

	// 找到新增和修改的行
	for i, newLine := range newLines {
		if oldIndexes, exists := oldMap[newLine]; exists {
			// 找到匹配的旧行
			found := false
			for _, oldIdx := range oldIndexes {
				if !seenInNew[oldIdx] {
					seenInNew[oldIdx] = true
					found = true
					break
				}
			}
			if !found {
				// 可能是移动或新增
				changes = append(changes, &VersionChange{
					Type:       ChangeTypeAdd,
					LineNumber: i + 1,
					NewContent: newLine,
				})
			}
		} else {
			// 新增行
			changes = append(changes, &VersionChange{
				Type:       ChangeTypeAdd,
				LineNumber: i + 1,
				NewContent: newLine,
			})
		}
	}

	// 找到删除的行
	for i, line := range oldLines {
		if !seenInNew[i] {
			changes = append(changes, &VersionChange{
				Type:       ChangeTypeDelete,
				LineNumber: i + 1,
				OldContent: line,
			})
		}
	}

	return changes
}

// ========== 版本统计 ==========

// VersionStats 版本统计
type VersionStats struct {
	TotalVersions   int64              `json:"totalVersions"`
	ByType          map[string]int64   `json:"byType"`
	TotalSize       int64              `json:"totalSize"`
	AvgVersionSize  int64              `json:"avgVersionSize"`
	OldestVersion   *time.Time         `json:"oldestVersion,omitempty"`
	NewestVersion   *time.Time         `json:"newestVersion,omitempty"`
	TopContributors []ContributorCount `json:"topContributors"`
}

// ContributorCount 贡献者统计
type ContributorCount struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Count    int64  `json:"count"`
}

// GetVersionStats 获取版本统计
func (vm *VersionManager) GetVersionStats() *VersionStats {
	stats := &VersionStats{
		ByType: make(map[string]int64),
	}

	contributors := make(map[string]*ContributorCount)

	vm.versions.Range(func(key, value interface{}) bool {
		version, ok := value.(*FileVersion)
		if !ok {
			return true
		}

		stats.TotalVersions++
		stats.TotalSize += version.Size
		stats.ByType[version.VersionType.String()]++

		// 时间统计
		if stats.OldestVersion == nil || version.CreatedAt.Before(*stats.OldestVersion) {
			stats.OldestVersion = &version.CreatedAt
		}
		if stats.NewestVersion == nil || version.CreatedAt.After(*stats.NewestVersion) {
			stats.NewestVersion = &version.CreatedAt
		}

		// 贡献者统计
		if version.CreatedBy != "" {
			if c, exists := contributors[version.CreatedBy]; exists {
				c.Count++
			} else {
				contributors[version.CreatedBy] = &ContributorCount{
					UserID:   version.CreatedBy,
					UserName: version.CreatorName,
					Count:    1,
				}
			}
		}

		return true
	})

	// 平均大小
	if stats.TotalVersions > 0 {
		stats.AvgVersionSize = stats.TotalSize / stats.TotalVersions
	}

	// Top贡献者
	for _, c := range contributors {
		stats.TopContributors = append(stats.TopContributors, *c)
	}
	sort.Slice(stats.TopContributors, func(i, j int) bool {
		return stats.TopContributors[i].Count > stats.TopContributors[j].Count
	})
	if len(stats.TopContributors) > 10 {
		stats.TopContributors = stats.TopContributors[:10]
	}

	return stats
}

// ========== 版本 API 类型 ==========

// VersionInfo 版本信息（API响应）
type VersionInfo struct {
	ID            string            `json:"id"`
	FilePath      string            `json:"filePath"`
	FileName      string            `json:"fileName"`
	VersionNumber int               `json:"versionNumber"`
	VersionType   string            `json:"versionType"`
	Status        string            `json:"status"`
	Size          int64             `json:"size"`
	Checksum      string            `json:"checksum"`
	CreatedAt     time.Time         `json:"createdAt"`
	CreatedBy     string            `json:"createdBy"`
	CreatorName   string            `json:"creatorName,omitempty"`
	Description   string            `json:"description,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	LockID        string            `json:"lockId,omitempty"`
	LockType      string            `json:"lockType,omitempty"`
	IsDelta       bool              `json:"isDelta"`
	BaseVersionID string            `json:"baseVersionId,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ToInfo 转换为VersionInfo
func (v *FileVersion) ToInfo() *VersionInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return &VersionInfo{
		ID:            v.ID,
		FilePath:      v.FilePath,
		FileName:      v.FileName,
		VersionNumber: v.VersionNumber,
		VersionType:   v.VersionType.String(),
		Status:        v.Status.String(),
		Size:          v.Size,
		Checksum:      v.Checksum,
		CreatedAt:     v.CreatedAt,
		CreatedBy:     v.CreatedBy,
		CreatorName:   v.CreatorName,
		Description:   v.Description,
		Tags:          v.Tags,
		LockID:        v.LockID,
		LockType:      v.LockType,
		IsDelta:       v.IsDelta,
		BaseVersionID: v.BaseVersionID,
		Metadata:      v.Metadata,
	}
}

// VersionRequest 版本请求
type VersionRequest struct {
	FilePath    string      `json:"filePath" binding:"required"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	VersionType VersionType `json:"versionType"`
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	VersionID string           `json:"versionId" binding:"required"`
	Partial   bool             `json:"partial"`
	Changes   []*VersionChange `json:"changes,omitempty"`
}

// CompareRequest 比较请求
type CompareRequest struct {
	FromVersionID string `json:"fromVersionId" binding:"required"`
	ToVersionID   string `json:"toVersionId,omitempty"` // 空表示与当前版本比较
}

// ========== 带锁管理的版本管理器 ==========

// VersionManagerWithLock 带锁管理的版本管理器
type VersionManagerWithLock struct {
	versionManager *VersionManager
	lockManager    *Manager
}

// NewVersionManagerWithLock 创建带锁的版本管理器
func NewVersionManagerWithLock(versionManager *VersionManager, lockManager *Manager) *VersionManagerWithLock {
	return &VersionManagerWithLock{
		versionManager: versionManager,
		lockManager:    lockManager,
	}
}

// CreateVersionSafe 安全地创建版本（带锁检查）
func (vm *VersionManagerWithLock) CreateVersionSafe(ctx context.Context, filePath, createdBy, creatorName string, versionType VersionType, description string) (*FileVersion, error) {
	// 检查文件是否被锁定
	if vm.lockManager.IsLocked(filePath) {
		info, err := vm.lockManager.GetLockByPath(filePath)
		if err == nil && info.Owner != createdBy {
			return nil, errors.New("file is locked by another user")
		}
	}

	return vm.versionManager.CreateVersion(ctx, filePath, createdBy, creatorName, versionType, description)
}

// RestoreVersionSafe 安全地恢复版本（带锁检查和自动创建锁）
func (vm *VersionManagerWithLock) RestoreVersionSafe(ctx context.Context, versionID, restoredBy, restoredByName string) error {
	version, err := vm.versionManager.GetVersion(versionID)
	if err != nil {
		return err
	}

	// 尝试获取独占锁
	req := &LockRequest{
		FilePath:  version.FilePath,
		LockType:  LockTypeExclusive,
		LockMode:  LockModeAuto,
		Owner:     restoredBy,
		OwnerName: restoredByName,
		Protocol:  "Version",
		Reason:    "Version restore",
	}

	lock, conflict, err := vm.lockManager.Lock(req)
	if err != nil && conflict != nil {
		return fmt.Errorf("file is locked: %s", conflict.Message)
	}

	// 恢复版本
	err = vm.versionManager.RestoreVersion(ctx, versionID, restoredBy)

	// 释放锁
	if lock != nil {
		_ = vm.lockManager.Unlock(lock.ID, restoredBy) //nolint:errcheck // 解锁失败不影响恢复结果
	}

	return err
}

// OnLockReleased 锁释放回调（自动创建版本）
func (vm *VersionManagerWithLock) OnLockReleased(lock *FileLock) {
	if lock == nil {
		return
	}

	ctx := context.Background()
	_, _ = vm.versionManager.CreateVersionFromLock(ctx, lock.FilePath, lock.ID, lock.Owner, lock.OwnerName, lock.LockType.String()) //nolint:errcheck // 回调中错误不影响主流程
}
