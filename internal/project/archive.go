// Package project provides project archive functionality
package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ArchiveStatus 归档状态
type ArchiveStatus string

// 归档状态常量
const (
	ArchiveStatusActive   ArchiveStatus = "active"
	ArchiveStatusArchived ArchiveStatus = "archived"
	ArchiveStatusDeleted  ArchiveStatus = "deleted"
)

// Archive 项目归档记录
type Archive struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	ProjectName string                 `json:"project_name"`
	ArchivePath string                 `json:"archive_path"`
	ArchiveSize int64                  `json:"archive_size"`
	Status      ArchiveStatus          `json:"status"`
	ArchivedAt  time.Time              `json:"archived_at"`
	ArchivedBy  string                 `json:"archived_by"`
	RestoredAt  *time.Time             `json:"restored_at,omitempty"`
	RestoredBy  string                 `json:"restored_by,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectArchive 是 Archive 的别名，保持向后兼容
type ProjectArchive = Archive

// ArchiveConfig 归档配置
type ArchiveConfig struct {
	StoragePath        string `json:"storage_path"`        // 归档存储路径
	RetentionDays      int    `json:"retention_days"`      // 归档保留天数
	AutoArchiveDays    int    `json:"auto_archive_days"`   // 自动归档天数（项目完成后）
	CompressArchive    bool   `json:"compress_archive"`    // 是否压缩归档
	IncludeAttachments bool   `json:"include_attachments"` // 是否包含附件
	MaxArchiveSize     int64  `json:"max_archive_size"`    // 最大归档大小（字节）
}

// DefaultArchiveConfig 默认归档配置
func DefaultArchiveConfig() ArchiveConfig {
	return ArchiveConfig{
		StoragePath:        "./archives",
		RetentionDays:      365,
		AutoArchiveDays:    30,
		CompressArchive:    true,
		IncludeAttachments: false,
		MaxArchiveSize:     100 * 1024 * 1024, // 100MB
	}
}

// ArchiveManager 归档管理器
type ArchiveManager struct {
	mu        sync.RWMutex
	archives  map[string]*Archive
	config    ArchiveConfig
	manager   *Manager
	exportMgr *ExportManager
}

// NewArchiveManager 创建归档管理器
func NewArchiveManager(mgr *Manager, config ArchiveConfig) *ArchiveManager {
	am := &ArchiveManager{
		archives:  make(map[string]*Archive),
		config:    config,
		manager:   mgr,
		exportMgr: NewExportManager(mgr),
	}

	// 确保存储目录存在
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		_ = err // 目录创建失败，但不影响管理器初始化
	}

	return am
}

// ArchiveProject 归档项目
func (am *ArchiveManager) ArchiveProject(projectID, archivedBy string, deleteAfterArchive bool) (*Archive, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 检查项目是否存在
	project, err := am.manager.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	// 检查是否已归档
	for _, archive := range am.archives {
		if archive.ProjectID == projectID && archive.Status == ArchiveStatusArchived {
			return nil, errors.New("项目已归档")
		}
	}

	// 导出项目数据
	options := ExportOptions{
		IncludeComments:  true,
		IncludeHistory:   true,
		IncludeCompleted: true,
		IncludeArchived:  true,
	}

	exportData, err := am.exportMgr.ExportToJSON(projectID, archivedBy, options)
	if err != nil {
		return nil, err
	}

	// 保存归档文件
	archiveID := uuid.New().String()
	archiveFileName := archiveID + ".json"
	archivePath := filepath.Join(am.config.StoragePath, archiveFileName)

	if err := os.WriteFile(archivePath, exportData, 0644); err != nil {
		return nil, err
	}

	// 获取文件大小
	fileInfo, _ := os.Stat(archivePath)
	var archiveSize int64
	if fileInfo != nil {
		archiveSize = fileInfo.Size()
	}

	// 计算过期时间
	var expiresAt *time.Time
	if am.config.RetentionDays > 0 {
		exp := time.Now().AddDate(0, 0, am.config.RetentionDays)
		expiresAt = &exp
	}

	// 创建归档记录
	archive := &Archive{
		ID:          archiveID,
		ProjectID:   projectID,
		ProjectName: project.Name,
		ArchivePath: archivePath,
		ArchiveSize: archiveSize,
		Status:      ArchiveStatusArchived,
		ArchivedAt:  time.Now(),
		ArchivedBy:  archivedBy,
		ExpiresAt:   expiresAt,
		Metadata: map[string]interface{}{
			"task_count":      project.TaskCount,
			"done_count":      project.DoneCount,
			"original_status": project.Status,
		},
	}

	am.archives[archiveID] = archive

	// 更新项目状态
	_, _ = am.manager.UpdateProject(projectID, map[string]interface{}{
		"status": "archived",
	})

	// 可选：删除项目数据
	if deleteAfterArchive {
		_ = am.manager.DeleteProject(projectID)
	}

	return archive, nil
}

// RestoreProject 恢复项目
func (am *ArchiveManager) RestoreProject(archiveID, restoredBy string, options ImportOptions) (*ImportResult, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	archive, exists := am.archives[archiveID]
	if !exists {
		return nil, ErrArchiveNotFound
	}

	if archive.Status != ArchiveStatusArchived {
		return nil, errors.New("归档状态无效，无法恢复")
	}

	// 读取归档文件
	data, err := os.ReadFile(archive.ArchivePath)
	if err != nil {
		return nil, err
	}

	// 设置导入选项
	if options.DefaultOwnerID == "" {
		options.DefaultOwnerID = restoredBy
	}
	options.ImportComments = true
	options.ImportHistory = true

	// 导入项目
	result, err := am.exportMgr.ImportProject(data, options)
	if err != nil {
		return nil, err
	}

	// 更新归档记录
	now := time.Now()
	archive.Status = ArchiveStatusActive
	archive.RestoredAt = &now
	archive.RestoredBy = restoredBy

	return result, nil
}

// GetArchive 获取归档信息
func (am *ArchiveManager) GetArchive(archiveID string) (*Archive, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	archive, exists := am.archives[archiveID]
	if !exists {
		return nil, ErrArchiveNotFound
	}
	return archive, nil
}

// ListArchives 列出归档
func (am *ArchiveManager) ListArchives(status ArchiveStatus, limit, offset int) []*Archive {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Archive, 0)
	for _, archive := range am.archives {
		if status == "" || archive.Status == status {
			result = append(result, archive)
		}
	}

	// 按归档时间倒序
	am.sortArchives(result)

	if offset > len(result) {
		offset = len(result)
	}
	end := offset + limit
	if limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// DeleteArchive 删除归档
func (am *ArchiveManager) DeleteArchive(archiveID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	archive, exists := am.archives[archiveID]
	if !exists {
		return ErrArchiveNotFound
	}

	// 删除归档文件
	if archive.ArchivePath != "" {
		_ = os.Remove(archive.ArchivePath)
	}

	// 更新状态
	archive.Status = ArchiveStatusDeleted
	delete(am.archives, archiveID)

	return nil
}

// ExtendArchiveRetention 延长归档保留期
func (am *ArchiveManager) ExtendArchiveRetention(archiveID string, additionalDays int) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	archive, exists := am.archives[archiveID]
	if !exists {
		return ErrArchiveNotFound
	}

	if archive.ExpiresAt != nil {
		newExpiry := archive.ExpiresAt.AddDate(0, 0, additionalDays)
		archive.ExpiresAt = &newExpiry
	}

	return nil
}

// GetArchiveStats 获取归档统计
func (am *ArchiveManager) GetArchiveStats() ArchiveStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := ArchiveStats{
		ByStatus: make(map[ArchiveStatus]int),
	}

	for _, archive := range am.archives {
		stats.Total++
		stats.TotalSize += archive.ArchiveSize
		stats.ByStatus[archive.Status]++

		if archive.Status == ArchiveStatusArchived {
			stats.ActiveArchives++
		}
	}

	return stats
}

// ArchiveStats 归档统计
type ArchiveStats struct {
	Total          int                   `json:"total"`
	TotalSize      int64                 `json:"total_size"`
	ActiveArchives int                   `json:"active_archives"`
	ByStatus       map[ArchiveStatus]int `json:"by_status"`
}

// CleanupExpiredArchives 清理过期归档
func (am *ArchiveManager) CleanupExpiredArchives() ([]string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	deleted := make([]string, 0)

	for id, archive := range am.archives {
		if archive.ExpiresAt != nil && archive.ExpiresAt.Before(now) {
			// 删除归档文件
			if archive.ArchivePath != "" {
				_ = os.Remove(archive.ArchivePath)
			}
			delete(am.archives, id)
			deleted = append(deleted, id)
		}
	}

	return deleted, nil
}

// ExportArchive 导出归档文件
func (am *ArchiveManager) ExportArchive(archiveID string) ([]byte, error) {
	archive, err := am.GetArchive(archiveID)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(archive.ArchivePath)
}

// GetArchivesByProject 获取项目的归档列表
func (am *ArchiveManager) GetArchivesByProject(projectID string) []*Archive {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Archive, 0)
	for _, archive := range am.archives {
		if archive.ProjectID == projectID {
			result = append(result, archive)
		}
	}
	return result
}

// sortArchives 按时间排序归档
func (am *ArchiveManager) sortArchives(archives []*Archive) {
	n := len(archives)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if archives[j].ArchivedAt.Before(archives[j+1].ArchivedAt) {
				archives[j], archives[j+1] = archives[j+1], archives[j]
			}
		}
	}
}

// AutoArchive 自动归档（用于定时任务）
func (am *ArchiveManager) AutoArchive() ([]string, error) {
	if am.config.AutoArchiveDays <= 0 {
		return nil, nil
	}

	archived := make([]string, 0)
	threshold := time.Now().AddDate(0, 0, -am.config.AutoArchiveDays)

	// 获取所有项目
	projects := am.manager.ListProjects("", 1000, 0)
	for _, project := range projects {
		// 检查是否满足自动归档条件
		if project.Status == "completed" || project.Status == "cancelled" {
			if project.UpdatedAt.Before(threshold) {
				_, err := am.ArchiveProject(project.ID, "system", false)
				if err == nil {
					archived = append(archived, project.ID)
				}
			}
		}
	}

	return archived, nil
}

// SaveArchives 保存归档索引
func (am *ArchiveManager) SaveArchives(path string) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	data, err := json.MarshalIndent(am.archives, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadArchives 加载归档索引
func (am *ArchiveManager) LoadArchives(path string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &am.archives)
}

// ErrArchiveNotFound 归档不存在错误
var ErrArchiveNotFound = errors.New("归档不存在")
