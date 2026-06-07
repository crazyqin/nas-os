// Package usbbackup 提供 USB 设备备份管理功能
package usbbackup

import (
	"fmt"
	"time"
)

// ========== 备份策略 ==========

// BackupPolicy 备份触发策略.
type BackupPolicy string

const (
	// PolicyOnInsert 插入即备份.
	PolicyOnInsert BackupPolicy = "on_insert"
	// PolicyScheduled 定时备份.
	PolicyScheduled BackupPolicy = "scheduled"
	// PolicyManual 手动触发.
	PolicyManual BackupPolicy = "manual"
)

// ========== 备份方向 ==========

// BackupDirection 备份方向.
type BackupDirection string

const (
	// DirectionNasToUSB NAS → USB.
	DirectionNasToUSB BackupDirection = "nas_to_usb"
	// DirectionUSBToNAS USB → NAS.
	DirectionUSBToNAS BackupDirection = "usb_to_nas"
	// DirectionBidirectional 双向同步.
	DirectionBidirectional BackupDirection = "bidirectional"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusIdle 空闲.
	TaskStatusIdle TaskStatus = "idle"
	// TaskStatusRunning 运行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusPaused 暂停.
	TaskStatusPaused TaskStatus = "paused"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
)

// ========== 文件过滤 ==========

// FileFilter 文件过滤规则.
type FileFilter struct {
	// Extensions 文件扩展名过滤（如 [".jpg", ".mp4"]），空则不过滤.
	Extensions []string `json:"extensions,omitempty"`

	// MaxFileSize 最大文件大小（字节），0 则不限制.
	MaxFileSize int64 `json:"maxFileSize,omitempty"`

	// MinFileSize 最小文件大小（字节），0 则不限制.
	MinFileSize int64 `json:"minFileSize,omitempty"`

	// ModifiedAfter 只同步此时间之后修改的文件.
	ModifiedAfter *time.Time `json:"modifiedAfter,omitempty"`

	// ModifiedBefore 只同步此时间之前修改的文件.
	ModifiedBefore *time.Time `json:"modifiedBefore,omitempty"`

	// ExcludePatterns 排除路径模式（glob 格式）.
	ExcludePatterns []string `json:"excludePatterns,omitempty"`
}

// ========== USB 设备信息 ==========

// USBDevice USB 存储设备信息.
type USBDevice struct {
	// ID 设备唯一标识.
	ID string `json:"id"`

	// DevicePath 设备路径（如 /dev/sdb1）.
	DevicePath string `json:"devicePath"`

	// Label 卷标.
	Label string `json:"label"`

	// UUID 文件系统 UUID.
	UUID string `json:"uuid"`

	// FileSystem 文件系统类型（vfat, ntfs, ext4, exfat 等）.
	FileSystem string `json:"fileSystem"`

	// TotalCapacity 总容量（字节）.
	TotalCapacity int64 `json:"totalCapacity"`

	// UsedCapacity 已用容量（字节）.
	UsedCapacity int64 `json:"usedCapacity"`

	// MountPoint 挂载点.
	MountPoint string `json:"mountPoint"`

	// Vendor 厂商.
	Vendor string `json:"vendor"`

	// Model 型号.
	Model string `json:"model"`

	// Serial 序列号.
	Serial string `json:"serial"`

	// ConnectedAt 设备插入时间.
	ConnectedAt time.Time `json:"connectedAt"`

	// Hotplug 是否热插拔.
	Hotplug bool `json:"hotplug"`
}

// ========== 备份任务 ==========

// BackupTask 备份任务定义.
type BackupTask struct {
	// ID 任务 ID.
	ID string `json:"id"`

	// Name 任务名称.
	Name string `json:"name"`

	// Direction 备份方向.
	Direction BackupDirection `json:"direction"`

	// Policy 备份策略.
	Policy BackupPolicy `json:"policy"`

	// SourcePath 源路径.
	SourcePath string `json:"sourcePath"`

	// DestPath 目标路径.
	DestPath string `json:"destPath"`

	// DeviceID 绑定的设备 ID（空则匹配任意设备）.
	DeviceID string `json:"deviceId,omitempty"`

	// CronExpr 定时表达式（PolicyScheduled 时使用）.
	CronExpr string `json:"cronExpr,omitempty"`

	// Filter 文件过滤规则.
	Filter *FileFilter `json:"filter,omitempty"`

	// Incremental 是否增量备份（基于 mtime）.
	Incremental bool `json:"incremental"`

	// Enabled 是否启用.
	Enabled bool `json:"enabled"`

	// Status 任务状态.
	Status TaskStatus `json:"status"`

	// LastRun 上次运行时间.
	LastRun *time.Time `json:"lastRun,omitempty"`

	// LastResult 上次运行结果.
	LastResult string `json:"lastResult,omitempty"`

	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 备份进度 ==========

// BackupProgress 备份进度信息.
type BackupProgress struct {
	// TaskID 任务 ID.
	TaskID string `json:"taskId"`

	// Status 当前状态.
	Status TaskStatus `json:"status"`

	// TotalFiles 总文件数.
	TotalFiles int `json:"totalFiles"`

	// CopiedFiles 已复制文件数.
	CopiedFiles int `json:"copiedFiles"`

	// SkippedFiles 跳过的文件数.
	SkippedFiles int `json:"skippedFiles"`

	// FailedFiles 失败的文件数.
	FailedFiles int `json:"failedFiles"`

	// TotalBytes 总字节数.
	TotalBytes int64 `json:"totalBytes"`

	// CopiedBytes 已复制字节数.
	CopiedBytes int64 `json:"copiedBytes"`

	// CurrentFile 当前正在处理的文件.
	CurrentFile string `json:"currentFile,omitempty"`

	// StartedAt 开始时间.
	StartedAt time.Time `json:"startedAt"`

	// UpdatedAt 最后更新时间.
	UpdatedAt time.Time `json:"updatedAt"`

	// Error 错误信息.
	Error string `json:"error,omitempty"`
}

// ========== 设备事件 ==========

// USBEventType USB 设备事件类型.
type USBEventType string

const (
	// USBEventDeviceConnected 设备连接.
	USBEventDeviceConnected USBEventType = "device_connected"
	// USBEventDeviceDisconnected 设备断开.
	USBEventDeviceDisconnected USBEventType = "device_disconnected"
)

// USBEvent USB 设备事件.
type USBEvent struct {
	// Type 事件类型.
	Type USBEventType `json:"type"`

	// Device 设备信息.
	Device *USBDevice `json:"device"`

	// Timestamp 事件时间.
	Timestamp time.Time `json:"timestamp"`
}

// ========== 错误定义 ==========

var (
	// ErrTaskNotFound 任务未找到.
	ErrTaskNotFound = fmt.Errorf("备份任务未找到")
	// ErrTaskRunning 任务正在运行.
	ErrTaskRunning = fmt.Errorf("备份任务正在运行")
	// ErrTaskPaused 任务已暂停.
	ErrTaskPaused = fmt.Errorf("备份任务已暂停")
	// ErrDeviceNotConnected 设备未连接.
	ErrDeviceNotConnected = fmt.Errorf("USB 设备未连接")
	// ErrInvalidDirection 无效的备份方向.
	ErrInvalidDirection = fmt.Errorf("无效的备份方向")
	// ErrInvalidPolicy 无效的备份策略.
	ErrInvalidPolicy = fmt.Errorf("无效的备份策略")
	// ErrSourcePathEmpty 源路径为空.
	ErrSourcePathEmpty = fmt.Errorf("源路径不能为空")
	// ErrDestPathEmpty 目标路径为空.
	ErrDestPathEmpty = fmt.Errorf("目标路径不能为空")
)

// ========== 创建任务请求 ==========

// CreateTaskRequest 创建备份任务请求.
type CreateTaskRequest struct {
	Name        string          `json:"name" binding:"required"`
	Direction   BackupDirection `json:"direction" binding:"required"`
	Policy      BackupPolicy    `json:"policy" binding:"required"`
	SourcePath  string          `json:"sourcePath" binding:"required"`
	DestPath    string          `json:"destPath" binding:"required"`
	DeviceID    string          `json:"deviceId,omitempty"`
	CronExpr    string          `json:"cronExpr,omitempty"`
	Filter      *FileFilter     `json:"filter,omitempty"`
	Incremental bool            `json:"incremental"`
}

// ========== 服务配置 ==========

// Config USB 备份服务配置.
type Config struct {
	// ScanInterval 设备扫描间隔（秒）.
	ScanInterval int `json:"scanInterval"`

	// AutoDetect 是否自动检测设备插入.
	AutoDetect bool `json:"autoDetect"`

	// MaxConcurrentTasks 最大并发备份任务数.
	MaxConcurrentTasks int `json:"maxConcurrentTasks"`

	// DefaultIncremental 默认是否增量备份.
	DefaultIncremental bool `json:"defaultIncremental"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		ScanInterval:       5,
		AutoDetect:         true,
		MaxConcurrentTasks: 2,
		DefaultIncremental: true,
	}
}
