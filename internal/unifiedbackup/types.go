// Package unifiedbackup 提供统一备份管理功能
// 对标群晖 Active Backup for Business / TrueNAS Active Backup
package unifiedbackup

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrTaskNotFound 备份任务不存在
	ErrTaskNotFound = errors.New("备份任务不存在")
	// ErrTaskRunning 任务正在运行中
	ErrTaskRunning = errors.New("任务正在运行中")
	// ErrTaskNotRunning 任务未在运行
	ErrTaskNotRunning = errors.New("任务未在运行")
	// ErrTaskNotPaused 任务未暂停
	ErrTaskNotPaused = errors.New("任务未暂停")
	// ErrTaskPaused 任务已暂停
	ErrTaskPaused = errors.New("任务已暂停")
	// ErrRestorePointNotFound 恢复点不存在
	ErrRestorePointNotFound = errors.New("恢复点不存在")
	// ErrInvalidSource 无效的备份源配置
	ErrInvalidSource = errors.New("无效的备份源配置")
	// ErrInvalidSchedule 无效的调度配置
	ErrInvalidSchedule = errors.New("无效的调度配置")
	// ErrStorageFull 存储空间不足
	ErrStorageFull = errors.New("存储空间不足")
	// ErrEncryptionKeyRequired 加密密钥必填
	ErrEncryptionKeyRequired = errors.New("加密密钥必填")
)

// ========== 备份源类型 ==========

// SourceType 备份源类型
type SourceType string

const (
	// SourceFileSystem 文件系统备份
	SourceFileSystem SourceType = "filesystem"
	// SourceDatabase 数据库备份
	SourceDatabase SourceType = "database"
	// SourceVM 虚拟机备份
	SourceVM SourceType = "vm"
	// SourceSMB SMB共享备份
	SourceSMB SourceType = "smb"
)

// ========== 备份模式 ==========

// BackupMode 备份模式
type BackupMode string

const (
	// BackupModeFull 全量备份
	BackupModeFull BackupMode = "full"
	// BackupModeIncremental 增量备份
	BackupModeIncremental BackupMode = "incremental"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态
type TaskStatus string

const (
	// TaskStatusPending 等待中
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 运行中
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusPaused 已暂停
	TaskStatusPaused TaskStatus = "paused"
	// TaskStatusCompleted 已完成
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败
	TaskStatusFailed TaskStatus = "failed"
)

// ========== 保留策略 ==========

// RetentionMode 保留模式
type RetentionMode string

const (
	// RetentionByCount 按数量保留
	RetentionByCount RetentionMode = "count"
	// RetentionByDays 按天数保留
	RetentionByDays RetentionMode = "days"
)

// ========== 加密类型 ==========

// EncryptionType 加密类型
type EncryptionType string

const (
	// EncryptionNone 不加密
	EncryptionNone EncryptionType = "none"
	// EncryptionAES256 AES-256 加密
	EncryptionAES256 EncryptionType = "aes256"
)

// ========== 恢复类型 ==========

// RestoreType 恢复类型
type RestoreType string

const (
	// RestoreTypePointInTime 按时间点恢复
	RestoreTypePointInTime RestoreType = "point_in_time"
	// RestoreTypeFile 按文件恢复
	RestoreTypeFile RestoreType = "file"
)

// ========== 数据类型 ==========

// BackupSource 备份源配置
type BackupSource struct {
	Type     SourceType  `json:"type"`               // 备份源类型
	Name     string      `json:"name"`               // 源名称
	Host     string      `json:"host,omitempty"`     // 主机地址（SMB/数据库/VM时使用）
	Port     int         `json:"port,omitempty"`     // 端口
	Path     string      `json:"path,omitempty"`     // 路径（文件系统/SMB共享路径）
	Username string      `json:"username,omitempty"` // 认证用户名
	Password string      `json:"password,omitempty"` // 认证密码
	Options  map[string]string `json:"options,omitempty"` // 额外选项
}

// RetentionPolicy 备份保留策略
type RetentionPolicy struct {
	Mode      RetentionMode `json:"mode"`                // 保留模式
	MaxCount  int           `json:"max_count,omitempty"` // 最大保留数量（按数量时）
	MaxDays   int           `json:"max_days,omitempty"`  // 最大保留天数（按天数时）
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Type    EncryptionType `json:"type"`              // 加密类型
	KeyID   string         `json:"key_id,omitempty"`  // 密钥ID
	Enabled bool           `json:"enabled"`           // 是否启用
}

// BackupTask 备份任务
type BackupTask struct {
	ID              string           `json:"id"`                          // 任务ID
	Name            string           `json:"name"`                        // 任务名称
	Description     string           `json:"description,omitempty"`       // 任务描述
	Source          BackupSource     `json:"source"`                      // 备份源
	Mode            BackupMode       `json:"mode"`                        // 备份模式
	Status          TaskStatus       `json:"status"`                      // 任务状态
	Schedule        string           `json:"schedule,omitempty"`          // Cron表达式
	Enabled         bool             `json:"enabled"`                     // 是否启用
	Retention       RetentionPolicy  `json:"retention"`                   // 保留策略
	Encryption      EncryptionConfig `json:"encryption"`                  // 加密配置
	StoragePath     string           `json:"storage_path"`                // 存储路径
	TotalSize       int64            `json:"total_size"`                  // 总备份大小（字节）
	LastRunAt       *time.Time       `json:"last_run_at,omitempty"`       // 最后运行时间
	LastStatus      TaskStatus       `json:"last_status,omitempty"`       // 最后运行状态
	NextRunAt       *time.Time       `json:"next_run_at,omitempty"`       // 下次运行时间
	Progress        float64          `json:"progress"`                    // 进度 0-100
	ErrorMsg        string           `json:"error_msg,omitempty"`         // 错误信息
	RestorePointCount int            `json:"restore_point_count"`         // 恢复点数量
	CreatedAt       time.Time        `json:"created_at"`                  // 创建时间
	UpdatedAt       time.Time        `json:"updated_at"`                  // 更新时间
}

// RestorePoint 恢复点
type RestorePoint struct {
	ID             string    `json:"id"`               // 恢复点ID
	TaskID         string    `json:"task_id"`          // 关联任务ID
	TaskName       string    `json:"task_name"`        // 任务名称
	SourceName     string    `json:"source_name"`      // 备份源名称
	Mode           BackupMode `json:"mode"`            // 备份模式
	Size           int64     `json:"size"`             // 数据大小（字节）
	CompressedSize int64     `json:"compressed_size"`  // 压缩后大小（字节）
	FileCount      int       `json:"file_count"`       // 文件数量
	Encrypted      bool      `json:"encrypted"`        // 是否加密
	StoragePath    string    `json:"storage_path"`     // 存储路径
	CreatedAt      time.Time `json:"created_at"`       // 创建时间
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	RestorePointID string      `json:"restore_point_id"`           // 恢复点ID
	Type           RestoreType `json:"type"`                       // 恢复类型
	TargetPath     string      `json:"target_path"`                // 目标路径
	Files          []string    `json:"files,omitempty"`            // 文件列表（按文件恢复时）
	Overwrite      bool        `json:"overwrite"`                  // 是否覆盖已有文件
}

// RestoreJob 恢复任务
type RestoreJob struct {
	ID             string      `json:"id"`              // 恢复任务ID
	RestorePointID string      `json:"restore_point_id"` // 恢复点ID
	TaskID         string      `json:"task_id"`         // 关联任务ID
	Type           RestoreType `json:"type"`            // 恢复类型
	Status         TaskStatus  `json:"status"`          // 任务状态
	Progress       float64     `json:"progress"`        // 进度 0-100
	TargetPath     string      `json:"target_path"`     // 目标路径
	FilesRestored  int         `json:"files_restored"`  // 已恢复文件数
	TotalFiles     int         `json:"total_files"`     // 总文件数
	ErrorMsg       string      `json:"error_msg,omitempty"` // 错误信息
	StartedAt      time.Time   `json:"started_at"`      // 开始时间
	CompletedAt    *time.Time  `json:"completed_at,omitempty"` // 完成时间
}

// StorageStats 存储统计
type StorageStats struct {
	TotalCapacity   int64   `json:"total_capacity"`    // 总容量（字节）
	UsedSpace       int64   `json:"used_space"`        // 已用空间（字节）
	FreeSpace       int64   `json:"free_space"`        // 可用空间（字节）
	UsagePercent    float64 `json:"usage_percent"`     // 使用率
	TotalTasks      int     `json:"total_tasks"`       // 任务总数
	TotalRestorePoints int  `json:"total_restore_points"` // 恢复点总数
	TotalBackupSize int64   `json:"total_backup_size"` // 总备份数据量（字节）
}

// ========== 请求/响应类型 ==========

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name        string           `json:"name"`                  // 任务名称
	Description string           `json:"description,omitempty"` // 任务描述
	Source      BackupSource     `json:"source"`                // 备份源
	Mode        BackupMode       `json:"mode"`                  // 备份模式
	Schedule    string           `json:"schedule,omitempty"`    // Cron表达式
	Enabled     bool             `json:"enabled"`               // 是否启用
	Retention   RetentionPolicy  `json:"retention"`             // 保留策略
	Encryption  EncryptionConfig `json:"encryption"`            // 加密配置
	StoragePath string           `json:"storage_path"`          // 存储路径
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Mode        *BackupMode       `json:"mode,omitempty"`
	Schedule    *string           `json:"schedule,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Retention   *RetentionPolicy  `json:"retention,omitempty"`
	Encryption  *EncryptionConfig `json:"encryption,omitempty"`
}

// APIResponse 通用API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
