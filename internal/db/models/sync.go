// Package models 提供 Cloud Drive Sync 数据库模型定义
// v2.384.0 - 户部实现
package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ========== 同步任务状态常量 ==========

// SyncTaskStatus 同步任务状态类型.
type SyncTaskStatus string

const (
	// SyncTaskStatusActive 活跃.
	SyncTaskStatusActive SyncTaskStatus = "active"
	// SyncTaskStatusPaused 已暂停.
	SyncTaskStatusPaused SyncTaskStatus = "paused"
	// SyncTaskStatusDisabled 已禁用.
	SyncTaskStatusDisabled SyncTaskStatus = "disabled"
	// SyncTaskStatusDeleted 已删除（软删除）.
	SyncTaskStatusDeleted SyncTaskStatus = "deleted"
)

// SyncDirection 同步方向.
type SyncDirection string

const (
	// SyncDirUpload 本地→云端.
	SyncDirUpload SyncDirection = "upload"
	// SyncDirDownload 云端→本地.
	SyncDirDownload SyncDirection = "download"
	// SyncDirBidirect 双向同步.
	SyncDirBidirect SyncDirection = "bidirect"
)

// SyncScheduleType 调度类型.
type SyncScheduleType string

const (
	// SyncScheduleManual 手动触发.
	SyncScheduleManual SyncScheduleType = "manual"
	// SyncScheduleInterval 定时间隔.
	SyncScheduleInterval SyncScheduleType = "interval"
	// SyncScheduleCron Cron 表达式.
	SyncScheduleCron SyncScheduleType = "cron"
	// SyncScheduleRealtime 实时监控.
	SyncScheduleRealtime SyncScheduleType = "realtime"
)

// ConflictStrategy 冲突解决策略.
type ConflictStrategy string

const (
	// ConflictSkip 跳过冲突文件.
	ConflictSkip ConflictStrategy = "skip"
	// ConflictLocalWin 本地优先.
	ConflictLocalWin ConflictStrategy = "local"
	// ConflictRemoteWin 远程优先.
	ConflictRemoteWin ConflictStrategy = "remote"
	// ConflictNewerWin 较新优先.
	ConflictNewerWin ConflictStrategy = "newer"
	// ConflictRename 重命名.
	ConflictRename ConflictStrategy = "rename"
)

// HistoryStatus 同步历史状态.
type HistoryStatus string

const (
	// HistoryStatusRunning 运行中.
	HistoryStatusRunning HistoryStatus = "running"
	// HistoryStatusSuccess 成功.
	HistoryStatusSuccess HistoryStatus = "success"
	// HistoryStatusFailed 失败.
	HistoryStatusFailed HistoryStatus = "failed"
	// HistoryStatusCancelled 已取消.
	HistoryStatusCancelled HistoryStatus = "cancelled"
	// HistoryStatusPartial 部分成功.
	HistoryStatusPartial HistoryStatus = "partial"
)

// ========== 数据库模型 ==========

// SyncTask 同步任务模型（对应 sync_tasks 表）.
type SyncTask struct {
	ID          string         `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	UserID      string         `json:"userId" db:"user_id"`
	ProviderID  string         `json:"providerId" db:"provider_id"`
	ProviderType string        `json:"providerType" db:"provider_type"`
	Status      SyncTaskStatus `json:"status" db:"status"`

	// 同步路径
	LocalPath  string `json:"localPath" db:"local_path"`
	RemotePath string `json:"remotePath" db:"remote_path"`

	// 同步配置
	Direction       SyncDirection   `json:"direction" db:"direction"`
	ConflictStrategy ConflictStrategy `json:"conflictStrategy" db:"conflict_strategy"`
	ScheduleType    SyncScheduleType `json:"scheduleType" db:"schedule_type"`
	ScheduleExpr    string          `json:"scheduleExpr,omitempty" db:"schedule_expr"`

	// 过滤规则
	IncludePatterns string `json:"includePatterns,omitempty" db:"include_patterns"` // JSON 数组
	ExcludePatterns string `json:"excludePatterns,omitempty" db:"exclude_patterns"` // JSON 数组
	MaxFileSize     int64  `json:"maxFileSize,omitempty" db:"max_file_size"`         // 字节，0=不限

	// 高级选项
	DeleteRemote    bool `json:"deleteRemote" db:"delete_remote"`
	PreserveModTime bool `json:"preserveModTime" db:"preserve_mod_time"`
	ChecksumVerify  bool `json:"checksumVerify" db:"checksum_verify"`
	EncryptEnabled  bool `json:"encryptEnabled" db:"encrypt_enabled"`

	// 带宽限制（KB/s，0=不限）
	BandwidthLimit int64 `json:"bandwidthLimit,omitempty" db:"bandwidth_limit"`

	// 保留版本数量（0=不保留）
	MaxVersions int `json:"maxVersions,omitempty" db:"max_versions"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" db:"updated_at"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" db:"deleted_at"`
}

// SyncHistory 同步历史记录模型（对应 sync_history 表）.
type SyncHistory struct {
	ID     string        `json:"id" db:"id"`
	TaskID string        `json:"taskId" db:"task_id"`
	Status HistoryStatus `json:"status" db:"status"`

	// 时间范围
	StartedAt  time.Time  `json:"startedAt" db:"started_at"`
	FinishedAt *time.Time `json:"finishedAt,omitempty" db:"finished_at"`
	Duration   int64      `json:"duration,omitempty" db:"duration"` // 毫秒

	// 统计
	TotalFiles     int64 `json:"totalFiles" db:"total_files"`
	UploadedFiles  int64 `json:"uploadedFiles" db:"uploaded_files"`
	DownloadedFiles int64 `json:"downloadedFiles" db:"downloaded_files"`
	SkippedFiles   int64 `json:"skippedFiles" db:"skipped_files"`
	FailedFiles    int64 `json:"failedFiles" db:"failed_files"`
	DeletedFiles   int64 `json:"deletedFiles" db:"deleted_files"`

	// 传输量
	TotalBytes     int64 `json:"totalBytes" db:"total_bytes"`
	TransferredBytes int64 `json:"transferredBytes" db:"transferred_bytes"`

	// 冲突与错误
	ConflictCount int    `json:"conflictCount" db:"conflict_count"`
	ErrorMessage  string `json:"errorMessage,omitempty" db:"error_message"`
	TriggerType   string `json:"triggerType" db:"trigger_type"` // manual / schedule / realtime
}

// SyncFileVersion 文件版本模型（对应 sync_file_versions 表）.
type SyncFileVersion struct {
	ID      string `json:"id" db:"id"`
	TaskID  string `json:"taskId" db:"task_id"`
	HistoryID string `json:"historyId,omitempty" db:"history_id"`

	// 文件路径
	LocalPath  string `json:"localPath" db:"local_path"`
	RemotePath string `json:"remotePath" db:"remote_path"`

	// 文件属性
	FileSize   int64     `json:"fileSize" db:"file_size"`
	ModTime    time.Time `json:"modTime" db:"mod_time"`
	Hash       string    `json:"hash,omitempty" db:"hash"`             // 内容哈希
	HashAlgo   string    `json:"hashAlgo,omitempty" db:"hash_algo"`    // md5 / sha256
	Version    int       `json:"version" db:"version"`
	Operation  string    `json:"operation" db:"operation"` // upload / download / delete / rename

	// 时间戳
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// ========== 请求/响应 DTO ==========

// CreateSyncTaskRequest 创建同步任务请求.
type CreateSyncTaskRequest struct {
	Name             string          `json:"name" binding:"required"`
	ProviderID       string          `json:"providerId" binding:"required"`
	ProviderType     string          `json:"providerType" binding:"required"`
	LocalPath        string          `json:"localPath" binding:"required"`
	RemotePath       string          `json:"remotePath" binding:"required"`
	Direction        SyncDirection   `json:"direction" binding:"required,oneof=upload download bidirect"`
	ConflictStrategy ConflictStrategy `json:"conflictStrategy" binding:"required,oneof=skip local remote newer rename"`
	ScheduleType     SyncScheduleType `json:"scheduleType" binding:"required,oneof=manual interval cron realtime"`
	ScheduleExpr     string          `json:"scheduleExpr"`
	IncludePatterns  []string        `json:"includePatterns"`
	ExcludePatterns  []string        `json:"excludePatterns"`
	MaxFileSize      int64           `json:"maxFileSize"`
	DeleteRemote     bool            `json:"deleteRemote"`
	PreserveModTime  bool            `json:"preserveModTime"`
	ChecksumVerify   bool            `json:"checksumVerify"`
	EncryptEnabled   bool            `json:"encryptEnabled"`
	BandwidthLimit   int64           `json:"bandwidthLimit"`
	MaxVersions      int             `json:"maxVersions"`
}

// UpdateSyncTaskRequest 更新同步任务请求.
type UpdateSyncTaskRequest struct {
	Name             *string          `json:"name"`
	Status           *SyncTaskStatus  `json:"status"`
	LocalPath        *string          `json:"localPath"`
	RemotePath       *string          `json:"remotePath"`
	Direction        *SyncDirection   `json:"direction"`
	ConflictStrategy *ConflictStrategy `json:"conflictStrategy"`
	ScheduleType     *SyncScheduleType `json:"scheduleType"`
	ScheduleExpr     *string          `json:"scheduleExpr"`
	IncludePatterns  []string         `json:"includePatterns"`
	ExcludePatterns  []string         `json:"excludePatterns"`
	MaxFileSize      *int64           `json:"maxFileSize"`
	DeleteRemote     *bool            `json:"deleteRemote"`
	PreserveModTime  *bool            `json:"preserveModTime"`
	ChecksumVerify   *bool            `json:"checksumVerify"`
	EncryptEnabled   *bool            `json:"encryptEnabled"`
	BandwidthLimit   *int64           `json:"bandwidthLimit"`
	MaxVersions      *int             `json:"maxVersions"`
}

// ListSyncTasksQuery 列出同步任务查询参数.
type ListSyncTasksQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"pageSize,default=20" binding:"min=1,max=100"`
	Status   string `form:"status"`
	SortBy   string `form:"sortBy,default=created_at"`
	SortDir  string `form:"sortDir,default=desc" binding:"oneof=asc desc"`
}

// ListSyncHistoryQuery 列出同步历史查询参数.
type ListSyncHistoryQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"pageSize,default=20" binding:"min=1,max=100"`
	Status   string `form:"status"`
	SortBy   string `form:"sortBy,default=started_at"`
	SortDir  string `form:"sortDir,default=desc" binding:"oneof=asc desc"`
}

// ========== 建表 SQL ==========

// SyncTablesDDL 返回同步相关表的建表语句.
func SyncTablesDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS sync_tasks (
			id              TEXT PRIMARY KEY,
			name            TEXT NOT NULL,
			user_id         TEXT NOT NULL DEFAULT '',
			provider_id     TEXT NOT NULL,
			provider_type   TEXT NOT NULL DEFAULT '',
			status          TEXT NOT NULL DEFAULT 'active',
			local_path      TEXT NOT NULL,
			remote_path     TEXT NOT NULL DEFAULT '/',
			direction       TEXT NOT NULL DEFAULT 'upload',
			conflict_strategy TEXT NOT NULL DEFAULT 'skip',
			schedule_type   TEXT NOT NULL DEFAULT 'manual',
			schedule_expr   TEXT NOT NULL DEFAULT '',
			include_patterns TEXT NOT NULL DEFAULT '[]',
			exclude_patterns TEXT NOT NULL DEFAULT '[]',
			max_file_size   INTEGER NOT NULL DEFAULT 0,
			delete_remote   INTEGER NOT NULL DEFAULT 0,
			preserve_mod_time INTEGER NOT NULL DEFAULT 1,
			checksum_verify INTEGER NOT NULL DEFAULT 0,
			encrypt_enabled INTEGER NOT NULL DEFAULT 0,
			bandwidth_limit INTEGER NOT NULL DEFAULT 0,
			max_versions    INTEGER NOT NULL DEFAULT 0,
			created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
			deleted_at      DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_user_id ON sync_tasks(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_status ON sync_tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_provider_id ON sync_tasks(provider_id)`,

		`CREATE TABLE IF NOT EXISTS sync_history (
			id               TEXT PRIMARY KEY,
			task_id          TEXT NOT NULL,
			status           TEXT NOT NULL DEFAULT 'running',
			started_at       DATETIME NOT NULL DEFAULT (datetime('now')),
			finished_at      DATETIME,
			duration         INTEGER NOT NULL DEFAULT 0,
			total_files      INTEGER NOT NULL DEFAULT 0,
			uploaded_files   INTEGER NOT NULL DEFAULT 0,
			downloaded_files INTEGER NOT NULL DEFAULT 0,
			skipped_files    INTEGER NOT NULL DEFAULT 0,
			failed_files     INTEGER NOT NULL DEFAULT 0,
			deleted_files    INTEGER NOT NULL DEFAULT 0,
			total_bytes      INTEGER NOT NULL DEFAULT 0,
			transferred_bytes INTEGER NOT NULL DEFAULT 0,
			conflict_count   INTEGER NOT NULL DEFAULT 0,
			error_message    TEXT NOT NULL DEFAULT '',
			trigger_type     TEXT NOT NULL DEFAULT 'manual',
			FOREIGN KEY (task_id) REFERENCES sync_tasks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_history_task_id ON sync_history(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_history_status ON sync_history(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_history_started_at ON sync_history(started_at)`,

		`CREATE TABLE IF NOT EXISTS sync_file_versions (
			id          TEXT PRIMARY KEY,
			task_id     TEXT NOT NULL,
			history_id  TEXT NOT NULL DEFAULT '',
			local_path  TEXT NOT NULL,
			remote_path TEXT NOT NULL DEFAULT '',
			file_size   INTEGER NOT NULL DEFAULT 0,
			mod_time    DATETIME NOT NULL,
			hash        TEXT NOT NULL DEFAULT '',
			hash_algo   TEXT NOT NULL DEFAULT 'sha256',
			version     INTEGER NOT NULL DEFAULT 1,
			operation   TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (task_id) REFERENCES sync_tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (history_id) REFERENCES sync_history(id) ON DELETE SET DEFAULT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_file_versions_task_id ON sync_file_versions(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_file_versions_local_path ON sync_file_versions(local_path)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_file_versions_remote_path ON sync_file_versions(remote_path)`,
	}
}

// ========== 数据库操作 ==========

// SyncStore 同步任务数据库操作.
type SyncStore struct {
	db *sql.DB
}

// NewSyncStore 创建同步数据存储.
func NewSyncStore(db *sql.DB) *SyncStore {
	return &SyncStore{db: db}
}

// InitSchema 初始化数据库表结构.
func (s *SyncStore) InitSchema() error {
	for _, ddl := range SyncTablesDDL() {
		if _, err := s.db.Exec(ddl); err != nil {
			return fmt.Errorf("初始化同步表失败: %w", err)
		}
	}
	return nil
}

// --- SyncTask CRUD ---

// CreateSyncTask 创建同步任务.
func (s *SyncStore) CreateSyncTask(task *SyncTask) error {
	if task.ID == "" {
		task.ID = "sync_" + uuid.New().String()[:12]
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = SyncTaskStatusActive
	}

	_, err := s.db.Exec(`INSERT INTO sync_tasks (
		id, name, user_id, provider_id, provider_type, status,
		local_path, remote_path, direction, conflict_strategy,
		schedule_type, schedule_expr, include_patterns, exclude_patterns,
		max_file_size, delete_remote, preserve_mod_time, checksum_verify,
		encrypt_enabled, bandwidth_limit, max_versions,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.UserID, task.ProviderID, task.ProviderType, task.Status,
		task.LocalPath, task.RemotePath, task.Direction, task.ConflictStrategy,
		task.ScheduleType, task.ScheduleExpr, task.IncludePatterns, task.ExcludePatterns,
		task.MaxFileSize, task.DeleteRemote, task.PreserveModTime, task.ChecksumVerify,
		task.EncryptEnabled, task.BandwidthLimit, task.MaxVersions,
		task.CreatedAt, task.UpdatedAt,
	)
	return err
}

// GetSyncTask 获取同步任务.
func (s *SyncStore) GetSyncTask(id string) (*SyncTask, error) {
	task := &SyncTask{}
	err := s.db.QueryRow(`SELECT
		id, name, user_id, provider_id, provider_type, status,
		local_path, remote_path, direction, conflict_strategy,
		schedule_type, schedule_expr, include_patterns, exclude_patterns,
		max_file_size, delete_remote, preserve_mod_time, checksum_verify,
		encrypt_enabled, bandwidth_limit, max_versions,
		created_at, updated_at, deleted_at
	FROM sync_tasks WHERE id = ? AND status != 'deleted'`, id).Scan(
		&task.ID, &task.Name, &task.UserID, &task.ProviderID, &task.ProviderType, &task.Status,
		&task.LocalPath, &task.RemotePath, &task.Direction, &task.ConflictStrategy,
		&task.ScheduleType, &task.ScheduleExpr, &task.IncludePatterns, &task.ExcludePatterns,
		&task.MaxFileSize, &task.DeleteRemote, &task.PreserveModTime, &task.ChecksumVerify,
		&task.EncryptEnabled, &task.BandwidthLimit, &task.MaxVersions,
		&task.CreatedAt, &task.UpdatedAt, &task.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// ListSyncTasks 列出同步任务.
func (s *SyncStore) ListSyncTasks(query ListSyncTasksQuery) ([]*SyncTask, int64, error) {
	where := "WHERE status != 'deleted'"
	args := []interface{}{}

	if query.Status != "" {
		where += " AND status = ?"
		args = append(args, query.Status)
	}

	// 总数
	var total int64
	countSQL := "SELECT COUNT(*) FROM sync_tasks " + where
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 排序字段白名单
	allowedSort := map[string]bool{
		"created_at": true, "updated_at": true, "name": true, "status": true,
	}
	sortBy := "created_at"
	if allowedSort[query.SortBy] {
		sortBy = query.SortBy
	}
	sortDir := "DESC"
	if query.SortDir == "asc" {
		sortDir = "ASC"
	}

	offset := (query.Page - 1) * query.PageSize
	listSQL := fmt.Sprintf(`SELECT
		id, name, user_id, provider_id, provider_type, status,
		local_path, remote_path, direction, conflict_strategy,
		schedule_type, schedule_expr, include_patterns, exclude_patterns,
		max_file_size, delete_remote, preserve_mod_time, checksum_verify,
		encrypt_enabled, bandwidth_limit, max_versions,
		created_at, updated_at, deleted_at
	FROM sync_tasks %s ORDER BY %s %s LIMIT ? OFFSET ?`, where, sortBy, sortDir)
	args = append(args, query.PageSize, offset)

	rows, err := s.db.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var tasks []*SyncTask
	for rows.Next() {
		task := &SyncTask{}
		if err := rows.Scan(
			&task.ID, &task.Name, &task.UserID, &task.ProviderID, &task.ProviderType, &task.Status,
			&task.LocalPath, &task.RemotePath, &task.Direction, &task.ConflictStrategy,
			&task.ScheduleType, &task.ScheduleExpr, &task.IncludePatterns, &task.ExcludePatterns,
			&task.MaxFileSize, &task.DeleteRemote, &task.PreserveModTime, &task.ChecksumVerify,
			&task.EncryptEnabled, &task.BandwidthLimit, &task.MaxVersions,
			&task.CreatedAt, &task.UpdatedAt, &task.DeletedAt,
		); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	return tasks, total, rows.Err()
}

// UpdateSyncTask 更新同步任务（部分更新）.
func (s *SyncStore) UpdateSyncTask(id string, req *UpdateSyncTaskRequest) error {
	setClauses := []string{}
	args := []interface{}{}

	if req.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *req.Status)
	}
	if req.LocalPath != nil {
		setClauses = append(setClauses, "local_path = ?")
		args = append(args, *req.LocalPath)
	}
	if req.RemotePath != nil {
		setClauses = append(setClauses, "remote_path = ?")
		args = append(args, *req.RemotePath)
	}
	if req.Direction != nil {
		setClauses = append(setClauses, "direction = ?")
		args = append(args, *req.Direction)
	}
	if req.ConflictStrategy != nil {
		setClauses = append(setClauses, "conflict_strategy = ?")
		args = append(args, *req.ConflictStrategy)
	}
	if req.ScheduleType != nil {
		setClauses = append(setClauses, "schedule_type = ?")
		args = append(args, *req.ScheduleType)
	}
	if req.ScheduleExpr != nil {
		setClauses = append(setClauses, "schedule_expr = ?")
		args = append(args, *req.ScheduleExpr)
	}
	if req.IncludePatterns != nil {
		setClauses = append(setClauses, "include_patterns = ?")
		args = append(args, stringsToJSON(req.IncludePatterns))
	}
	if req.ExcludePatterns != nil {
		setClauses = append(setClauses, "exclude_patterns = ?")
		args = append(args, stringsToJSON(req.ExcludePatterns))
	}
	if req.MaxFileSize != nil {
		setClauses = append(setClauses, "max_file_size = ?")
		args = append(args, *req.MaxFileSize)
	}
	if req.DeleteRemote != nil {
		setClauses = append(setClauses, "delete_remote = ?")
		args = append(args, *req.DeleteRemote)
	}
	if req.PreserveModTime != nil {
		setClauses = append(setClauses, "preserve_mod_time = ?")
		args = append(args, *req.PreserveModTime)
	}
	if req.ChecksumVerify != nil {
		setClauses = append(setClauses, "checksum_verify = ?")
		args = append(args, *req.ChecksumVerify)
	}
	if req.EncryptEnabled != nil {
		setClauses = append(setClauses, "encrypt_enabled = ?")
		args = append(args, *req.EncryptEnabled)
	}
	if req.BandwidthLimit != nil {
		setClauses = append(setClauses, "bandwidth_limit = ?")
		args = append(args, *req.BandwidthLimit)
	}
	if req.MaxVersions != nil {
		setClauses = append(setClauses, "max_versions = ?")
		args = append(args, *req.MaxVersions)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("没有需要更新的字段")
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	query := "UPDATE sync_tasks SET " + joinSetClauses(setClauses) + " WHERE id = ? AND status != 'deleted'"
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("同步任务不存在: %s", id)
	}
	return nil
}

// DeleteSyncTask 软删除同步任务.
func (s *SyncStore) DeleteSyncTask(id string) error {
	result, err := s.db.Exec(
		"UPDATE sync_tasks SET status = 'deleted', deleted_at = datetime('now'), updated_at = datetime('now') WHERE id = ? AND status != 'deleted'",
		id,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("同步任务不存在: %s", id)
	}
	return nil
}

// --- SyncHistory ---

// CreateSyncHistory 创建同步历史记录.
func (s *SyncStore) CreateSyncHistory(h *SyncHistory) error {
	if h.ID == "" {
		h.ID = "hist_" + uuid.New().String()[:12]
	}
	if h.StartedAt.IsZero() {
		h.StartedAt = time.Now()
	}
	if h.Status == "" {
		h.Status = HistoryStatusRunning
	}

	_, err := s.db.Exec(`INSERT INTO sync_history (
		id, task_id, status, started_at, finished_at, duration,
		total_files, uploaded_files, downloaded_files, skipped_files,
		failed_files, deleted_files, total_bytes, transferred_bytes,
		conflict_count, error_message, trigger_type
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.TaskID, h.Status, h.StartedAt, h.FinishedAt, h.Duration,
		h.TotalFiles, h.UploadedFiles, h.DownloadedFiles, h.SkippedFiles,
		h.FailedFiles, h.DeletedFiles, h.TotalBytes, h.TransferredBytes,
		h.ConflictCount, h.ErrorMessage, h.TriggerType,
	)
	return err
}

// GetSyncHistory 获取单条同步历史.
func (s *SyncStore) GetSyncHistory(id string) (*SyncHistory, error) {
	h := &SyncHistory{}
	err := s.db.QueryRow(`SELECT
		id, task_id, status, started_at, finished_at, duration,
		total_files, uploaded_files, downloaded_files, skipped_files,
		failed_files, deleted_files, total_bytes, transferred_bytes,
		conflict_count, error_message, trigger_type
	FROM sync_history WHERE id = ?`, id).Scan(
		&h.ID, &h.TaskID, &h.Status, &h.StartedAt, &h.FinishedAt, &h.Duration,
		&h.TotalFiles, &h.UploadedFiles, &h.DownloadedFiles, &h.SkippedFiles,
		&h.FailedFiles, &h.DeletedFiles, &h.TotalBytes, &h.TransferredBytes,
		&h.ConflictCount, &h.ErrorMessage, &h.TriggerType,
	)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// ListSyncHistory 列出同步历史（按任务ID）.
func (s *SyncStore) ListSyncHistory(taskID string, query ListSyncHistoryQuery) ([]*SyncHistory, int64, error) {
	where := "WHERE task_id = ?"
	args := []interface{}{taskID}

	if query.Status != "" {
		where += " AND status = ?"
		args = append(args, query.Status)
	}

	// 总数
	var total int64
	countSQL := "SELECT COUNT(*) FROM sync_history " + where
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	allowedSort := map[string]bool{
		"started_at": true, "finished_at": true, "status": true, "duration": true,
	}
	sortBy := "started_at"
	if allowedSort[query.SortBy] {
		sortBy = query.SortBy
	}
	sortDir := "DESC"
	if query.SortDir == "asc" {
		sortDir = "ASC"
	}

	offset := (query.Page - 1) * query.PageSize
	listSQL := fmt.Sprintf(`SELECT
		id, task_id, status, started_at, finished_at, duration,
		total_files, uploaded_files, downloaded_files, skipped_files,
		failed_files, deleted_files, total_bytes, transferred_bytes,
		conflict_count, error_message, trigger_type
	FROM sync_history %s ORDER BY %s %s LIMIT ? OFFSET ?`, where, sortBy, sortDir)
	args = append(args, query.PageSize, offset)

	rows, err := s.db.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var histories []*SyncHistory
	for rows.Next() {
		h := &SyncHistory{}
		if err := rows.Scan(
			&h.ID, &h.TaskID, &h.Status, &h.StartedAt, &h.FinishedAt, &h.Duration,
			&h.TotalFiles, &h.UploadedFiles, &h.DownloadedFiles, &h.SkippedFiles,
			&h.FailedFiles, &h.DeletedFiles, &h.TotalBytes, &h.TransferredBytes,
			&h.ConflictCount, &h.ErrorMessage, &h.TriggerType,
		); err != nil {
			return nil, 0, err
		}
		histories = append(histories, h)
	}
	return histories, total, rows.Err()
}

// UpdateSyncHistory 更新同步历史.
func (s *SyncStore) UpdateSyncHistory(h *SyncHistory) error {
	_, err := s.db.Exec(`UPDATE sync_history SET
		status = ?, finished_at = ?, duration = ?,
		total_files = ?, uploaded_files = ?, downloaded_files = ?,
		skipped_files = ?, failed_files = ?, deleted_files = ?,
		total_bytes = ?, transferred_bytes = ?,
		conflict_count = ?, error_message = ?
	WHERE id = ?`,
		h.Status, h.FinishedAt, h.Duration,
		h.TotalFiles, h.UploadedFiles, h.DownloadedFiles,
		h.SkippedFiles, h.FailedFiles, h.DeletedFiles,
		h.TotalBytes, h.TransferredBytes,
		h.ConflictCount, h.ErrorMessage,
		h.ID,
	)
	return err
}

// --- SyncFileVersion ---

// CreateFileVersion 创建文件版本记录.
func (s *SyncStore) CreateFileVersion(v *SyncFileVersion) error {
	if v.ID == "" {
		v.ID = "ver_" + uuid.New().String()[:12]
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	_, err := s.db.Exec(`INSERT INTO sync_file_versions (
		id, task_id, history_id, local_path, remote_path,
		file_size, mod_time, hash, hash_algo, version, operation, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.TaskID, v.HistoryID, v.LocalPath, v.RemotePath,
		v.FileSize, v.ModTime, v.Hash, v.HashAlgo, v.Version, v.Operation, v.CreatedAt,
	)
	return err
}

// ListFileVersions 列出文件版本.
func (s *SyncStore) ListFileVersions(taskID, localPath string, limit int) ([]*SyncFileVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT
		id, task_id, history_id, local_path, remote_path,
		file_size, mod_time, hash, hash_algo, version, operation, created_at
	FROM sync_file_versions
	WHERE task_id = ? AND local_path = ?
	ORDER BY version DESC LIMIT ?`, taskID, localPath, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var versions []*SyncFileVersion
	for rows.Next() {
		v := &SyncFileVersion{}
		if err := rows.Scan(
			&v.ID, &v.TaskID, &v.HistoryID, &v.LocalPath, &v.RemotePath,
			&v.FileSize, &v.ModTime, &v.Hash, &v.HashAlgo, &v.Version, &v.Operation, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// CleanOldVersions 清理过期版本（保留最近 N 个版本）.
func (s *SyncStore) CleanOldVersions(taskID, localPath string, keepVersions int) error {
	if keepVersions <= 0 {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sync_file_versions
	WHERE task_id = ? AND local_path = ? AND version < (
		SELECT MIN(v.version) FROM (
			SELECT version FROM sync_file_versions
			WHERE task_id = ? AND local_path = ?
			ORDER BY version DESC LIMIT ?
		) v
	)`, taskID, localPath, taskID, localPath, keepVersions)
	return err
}

// ========== 辅助函数 ==========

// stringsToJSON 将字符串切片转为 JSON 数组字符串.
func stringsToJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	result := "["
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += `"` + s + `"`
	}
	result += "]"
	return result
}

// joinSetClauses 拼接 SET 子句.
func joinSetClauses(clauses []string) string {
	result := ""
	for i, c := range clauses {
		if i > 0 {
			result += ", "
		}
		result += c
	}
	return result
}
