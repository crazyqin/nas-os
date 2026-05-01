// Package activebackup 提供整机备份管理功能
// Version: v2.480.0 - Active Backup 整机备份模块
package activebackup

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrAgentNotFound Agent 不存在.
	ErrAgentNotFound = errors.New("agent 不存在")
	// ErrAgentExists Agent 已存在.
	ErrAgentExists = errors.New("agent 已存在")
	// ErrAgentOffline Agent 离线.
	ErrAgentOffline = errors.New("agent 离线")
	// ErrTaskNotFound 备份任务不存在.
	ErrTaskNotFound = errors.New("备份任务不存在")
	// ErrTaskRunning 任务正在运行.
	ErrTaskRunning = errors.New("任务正在运行")
	// ErrTaskNotRunning 任务未在运行.
	ErrTaskNotRunning = errors.New("任务未在运行")
	// ErrRestorePointNotFound 恢复点不存在.
	ErrRestorePointNotFound = errors.New("恢复点不存在")
	// ErrStoragePoolFull 存储池已满.
	ErrStoragePoolFull = errors.New("存储池已满")
	// ErrInvalidSchedule 无效的调度配置.
	ErrInvalidSchedule = errors.New("无效的调度配置")
	// ErrAgentAuthFailed Agent 认证失败.
	ErrAgentAuthFailed = errors.New("agent 认证失败")
)

// ========== Agent 类型 ==========

// Platform 平台类型.
type Platform string

const (
	// PlatformWindows Windows 平台.
	PlatformWindows Platform = "windows"
	// PlatformLinux Linux 平台.
	PlatformLinux Platform = "linux"
	// PlatformVMware VMware 虚拟机.
	PlatformVMware Platform = "vmware"
	// PlatformHyperV Hyper-V 虚拟机.
	PlatformHyperV Platform = "hyperv"
	// PlatformKVM KVM 虚拟机.
	PlatformKVM Platform = "kvm"
)

// AgentStatus Agent 状态.
type AgentStatus string

const (
	// AgentStatusOnline 在线.
	AgentStatusOnline AgentStatus = "online"
	// AgentStatusOffline 离线.
	AgentStatusOffline AgentStatus = "offline"
	// AgentStatusBackuping 备份中.
	AgentStatusBackuping AgentStatus = "backuping"
	// AgentStatusRestoring 恢复中.
	AgentStatusRestoring AgentStatus = "restoring"
)

// AgentInfo Agent 信息.
type AgentInfo struct {
	ID           string      `json:"id"`            // Agent ID
	Name         string      `json:"name"`          // 主机名称
	Hostname     string      `json:"hostname"`      // 主机名
	IP           string      `json:"ip"`            // IP 地址
	Platform     Platform    `json:"platform"`      // 平台类型
	OSVersion    string      `json:"os_version"`    // 操作系统版本
	AgentVersion string      `json:"agent_version"` // Agent 版本
	MACAddress   string      `json:"mac_address"`   // MAC 地址（设备指纹）
	Token        string      `json:"-"`             // 认证 Token
	Fingerprint  string      `json:"-"`             // 设备指纹
	Status       AgentStatus `json:"status"`        // 当前状态
	LastSeen     time.Time   `json:"last_seen"`     // 最后心跳时间
	RegisteredAt time.Time   `json:"registered_at"` // 注册时间
	CPU          string      `json:"cpu"`           // CPU 信息
	Memory       uint64      `json:"memory"`        // 内存大小（字节）
	Disks        []DiskInfo  `json:"disks"`         // 磁盘信息
	Tags         []string    `json:"tags"`          // 标签
}

// DiskInfo 磁盘信息.
type DiskInfo struct {
	Device     string `json:"device"`      // 设备路径
	Size       uint64 `json:"size"`        // 磁盘大小（字节）
	Used       uint64 `json:"used"`        // 已用空间（字节）
	Free       uint64 `json:"free"`        // 可用空间（字节）
	FileSystem string `json:"file_system"` // 文件系统类型
	MountPoint string `json:"mount_point"` // 挂载点
}

// AgentRegistrationRequest Agent 注册请求.
type AgentRegistrationRequest struct {
	Name         string   `json:"name" binding:"required"`          // 主机名称
	Hostname     string   `json:"hostname" binding:"required"`      // 主机名
	IP           string   `json:"ip" binding:"required"`            // IP 地址
	Platform     Platform `json:"platform" binding:"required"`      // 平台类型
	OSVersion    string   `json:"os_version"`                       // 操作系统版本
	AgentVersion string   `json:"agent_version"`                    // Agent 版本
	MACAddress   string   `json:"mac_address" binding:"required"`   // MAC 地址
	Fingerprint  string   `json:"fingerprint" binding:"required"`   // 设备指纹
	CPU          string   `json:"cpu"`                              // CPU 信息
	Memory       uint64   `json:"memory"`                           // 内存大小
	Tags         []string `json:"tags"`                             // 标签
}

// AgentHeartbeatRequest Agent 心跳请求.
type AgentHeartbeatRequest struct {
	AgentID string     `json:"agent_id" binding:"required"` // Agent ID
	Status  AgentStatus `json:"status"`                     // 当前状态
	Disks   []DiskInfo  `json:"disks"`                      // 磁盘信息
}

// AgentConfig Agent 配置下发.
type AgentConfig struct {
	HeartbeatInterval int    `json:"heartbeat_interval"` // 心跳间隔（秒）
	BandwidthLimit    uint64 `json:"bandwidth_limit"`    // 带宽限制（字节/秒）
	CompressionLevel  int    `json:"compression_level"`  // 压缩级别 1-9
	EncryptionEnabled bool   `json:"encryption_enabled"` // 是否启用加密
	RetryCount        int    `json:"retry_count"`        // 重试次数
	RetryInterval     int    `json:"retry_interval"`     // 重试间隔（秒）
}

// ========== 备份任务类型 ==========

// BackupType 备份类型.
type BackupType string

const (
	// BackupTypeFull 全量备份.
	BackupTypeFull BackupType = "full"
	// BackupTypeIncremental 增量备份.
	BackupTypeIncremental BackupType = "incremental"
	// BackupTypeDifferential 差异备份.
	BackupTypeDifferential BackupType = "differential"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusIdle 空闲.
	TaskStatusIdle TaskStatus = "idle"
	// TaskStatusRunning 运行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusSuccess 成功.
	TaskStatusSuccess TaskStatus = "success"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusQueued 排队中.
	TaskStatusQueued TaskStatus = "queued"
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleTypeManual 手动触发.
	ScheduleTypeManual ScheduleType = "manual"
	// ScheduleTypeScheduled 定时调度.
	ScheduleTypeScheduled ScheduleType = "scheduled"
	// ScheduleTypeEvent 事件触发.
	ScheduleTypeEvent ScheduleType = "event"
)

// CompressionType 压缩类型.
type CompressionType string

const (
	// CompressionNone 不压缩.
	CompressionNone CompressionType = "none"
	// CompressionGzip Gzip 压缩.
	CompressionGzip CompressionType = "gzip"
	// CompressionLZ4 LZ4 压缩.
	CompressionLZ4 CompressionType = "lz4"
	// CompressionZstd Zstandard 压缩.
	CompressionZstd CompressionType = "zstd"
)

// EncryptionType 加密类型.
type EncryptionType string

const (
	// EncryptionNone 不加密.
	EncryptionNone EncryptionType = "none"
	// EncryptionAES256 AES-256 加密.
	EncryptionAES256 EncryptionType = "aes256"
	// EncryptionChaCha20 ChaCha20 加密.
	EncryptionChaCha20 EncryptionType = "chacha20"
)

// BackupTask 备份任务.
type BackupTask struct {
	ID              string           `json:"id"`               // 任务 ID
	Name            string           `json:"name"`             // 任务名称
	Description     string           `json:"description"`      // 任务描述
	AgentID         string           `json:"agent_id"`         // 关联 Agent ID
	BackupType      BackupType       `json:"backup_type"`      // 备份类型
	Status          TaskStatus       `json:"status"`           // 任务状态
	ScheduleType    ScheduleType     `json:"schedule_type"`    // 调度类型
	Schedule        string           `json:"schedule"`         // Cron 表达式（定时调度时使用）
	StoragePoolID   string           `json:"storage_pool_id"`  // 存储池 ID
	Compression     CompressionType  `json:"compression"`      // 压缩类型
	Encryption      EncryptionType   `json:"encryption"`       // 加密类型
	EncryptionKey   string           `json:"-"`                // 加密密钥
	RetentionDays   int              `json:"retention_days"`   // 保留天数
	MaxVersions     int              `json:"max_versions"`     // 最大版本数
	Enabled         bool             `json:"enabled"`          // 是否启用
	IncludeVolumes  []string         `json:"include_volumes"`  // 包含的卷/分区
	ExcludePatterns []string         `json:"exclude_patterns"` // 排除的文件模式
	BandwidthLimit  uint64           `json:"bandwidth_limit"`  // 带宽限制（字节/秒）
	PreScript       string           `json:"pre_script"`       // 备份前执行脚本
	PostScript      string           `json:"post_script"`      // 备份后执行脚本
	NotifyOnSuccess bool             `json:"notify_on_success"` // 成功时通知
	NotifyOnFailure bool             `json:"notify_on_failure"` // 失败时通知

	// 运行状态
	LastRunAt    *time.Time   `json:"last_run_at,omitempty"`    // 最后运行时间
	LastStatus   TaskStatus   `json:"last_status"`              // 最后运行状态
	NextRunAt    *time.Time   `json:"next_run_at,omitempty"`    // 下次运行时间
	Progress     float64      `json:"progress"`                 // 进度百分比 0-100
	SpeedBytes   uint64       `json:"speed_bytes"`              // 备份速度（字节/秒）
	Transferred  uint64       `json:"transferred"`              // 已传输字节数
	TotalBytes   uint64       `json:"total_bytes"`              // 总字节数
	ErrorMsg     string       `json:"error_msg,omitempty"`      // 错误信息
	RestorePoint string       `json:"restore_point,omitempty"`  // 最新恢复点 ID

	// 统计
	TotalRuns   int       `json:"total_runs"`   // 总运行次数
	SuccessRuns int       `json:"success_runs"` // 成功次数
	FailRuns    int       `json:"fail_runs"`    // 失败次数
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	Name            string          `json:"name" binding:"required"`      // 任务名称
	Description     string          `json:"description"`                  // 任务描述
	AgentID         string          `json:"agent_id" binding:"required"`  // 关联 Agent ID
	BackupType      BackupType      `json:"backup_type" binding:"required"` // 备份类型
	ScheduleType    ScheduleType    `json:"schedule_type"`                // 调度类型
	Schedule        string          `json:"schedule"`                     // Cron 表达式
	StoragePoolID   string          `json:"storage_pool_id"`              // 存储池 ID
	Compression     CompressionType `json:"compression"`                  // 压缩类型
	Encryption      EncryptionType  `json:"encryption"`                   // 加密类型
	EncryptionKey   string          `json:"encryption_key"`               // 加密密钥
	RetentionDays   int             `json:"retention_days"`               // 保留天数
	MaxVersions     int             `json:"max_versions"`                 // 最大版本数
	Enabled         bool            `json:"enabled"`                      // 是否启用
	IncludeVolumes  []string        `json:"include_volumes"`              // 包含卷
	ExcludePatterns []string        `json:"exclude_patterns"`             // 排除模式
	BandwidthLimit  uint64          `json:"bandwidth_limit"`              // 带宽限制
	PreScript       string          `json:"pre_script"`                   // 前置脚本
	PostScript      string          `json:"post_script"`                  // 后置脚本
	NotifyOnSuccess bool            `json:"notify_on_success"`            // 成功通知
	NotifyOnFailure bool            `json:"notify_on_failure"`            // 失败通知
}

// UpdateTaskRequest 更新任务请求.
type UpdateTaskRequest struct {
	Name            *string          `json:"name"`             // 任务名称
	Description     *string          `json:"description"`      // 任务描述
	BackupType      *BackupType      `json:"backup_type"`      // 备份类型
	ScheduleType    *ScheduleType    `json:"schedule_type"`    // 调度类型
	Schedule        *string          `json:"schedule"`         // Cron 表达式
	StoragePoolID   *string          `json:"storage_pool_id"`  // 存储池 ID
	Compression     *CompressionType `json:"compression"`      // 压缩类型
	Encryption      *EncryptionType  `json:"encryption"`       // 加密类型
	EncryptionKey   *string          `json:"encryption_key"`   // 加密密钥
	RetentionDays   *int             `json:"retention_days"`   // 保留天数
	MaxVersions     *int             `json:"max_versions"`     // 最大版本数
	Enabled         *bool            `json:"enabled"`          // 是否启用
	IncludeVolumes  []string         `json:"include_volumes"`  // 包含卷
	ExcludePatterns []string         `json:"exclude_patterns"` // 排除模式
	BandwidthLimit  *uint64          `json:"bandwidth_limit"`  // 带宽限制
	PreScript       *string          `json:"pre_script"`       // 前置脚本
	PostScript      *string          `json:"post_script"`      // 后置脚本
	NotifyOnSuccess *bool            `json:"notify_on_success"` // 成功通知
	NotifyOnFailure *bool            `json:"notify_on_failure"` // 失败通知
}

// ========== 恢复点类型 ==========

// RestorePointType 恢复点类型.
type RestorePointType string

const (
	// RestorePointFull 全量恢复点.
	RestorePointFull RestorePointType = "full"
	// RestorePointIncremental 增量恢复点.
	RestorePointIncremental RestorePointType = "incremental"
)

// RestorePoint 恢复点.
type RestorePoint struct {
	ID           string           `json:"id"`            // 恢复点 ID
	TaskID       string           `json:"task_id"`       // 关联任务 ID
	TaskName     string           `json:"task_name"`     // 任务名称
	AgentID      string           `json:"agent_id"`      // 关联 Agent ID
	AgentName    string           `json:"agent_name"`    // Agent 名称
	Type         RestorePointType `json:"type"`          // 恢复点类型
	Size         uint64           `json:"size"`          // 数据大小（字节）
	CompressedSize uint64         `json:"compressed_size"` // 压缩后大小（字节）
	Encrypted    bool             `json:"encrypted"`     // 是否加密
	StoragePath  string           `json:"storage_path"`  // 存储路径
	CreatedAt    time.Time        `json:"created_at"`    // 创建时间
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"` // 过期时间
	BlockCount   int64            `json:"block_count"`   // 数据块数量
	ParentID     string           `json:"parent_id,omitempty"` // 父恢复点 ID（增量链）
	Volumes      []string         `json:"volumes"`       // 包含的卷
	Checksum     string           `json:"checksum"`      // 校验和
}

// ========== 恢复请求类型 ==========

// RestoreType 恢复类型.
type RestoreType string

const (
	// RestoreTypeFull 整机恢复.
	RestoreTypeFull RestoreType = "full"
	// RestoreTypeFiles 文件级恢复.
	RestoryTypeFiles RestoreType = "files"
)

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	RestorePointID string     `json:"restore_point_id" binding:"required"` // 恢复点 ID
	TargetAgentID  string     `json:"target_agent_id"`                     // 目标 Agent ID（异机恢复）
	RestoreType    RestoreType `json:"restore_type"`                        // 恢复类型
	TargetDisk     string     `json:"target_disk"`                          // 目标磁盘（整机恢复）
	TargetPath     string     `json:"target_path"`                          // 目标路径（文件恢复）
	Files          []string   `json:"files"`                                // 恢复文件列表（文件恢复）
	Options        RestoreOptions `json:"options"`                          // 恢复选项
}

// RestoreOptions 恢复选项.
type RestoreOptions struct {
	OverwriteExisting bool `json:"overwrite_existing"` // 覆盖已存在文件
	VerifyAfterRestore bool `json:"verify_after_restore"` // 恢复后校验
	RebootAfterRestore bool `json:"reboot_after_restore"` // 恢复后重启
	SkipBootloader     bool `json:"skip_bootloader"`       // 跳过引导加载器
}

// RestoreJob 恢复任务.
type RestoreJob struct {
	ID              string      `json:"id"`               // 恢复任务 ID
	RestorePointID  string      `json:"restore_point_id"` // 恢复点 ID
	AgentID         string      `json:"agent_id"`         // 目标 Agent ID
	RestoreType     RestoreType `json:"restore_type"`     // 恢复类型
	Status          TaskStatus  `json:"status"`           // 任务状态
	Progress        float64     `json:"progress"`         // 进度百分比
	Transferred     uint64      `json:"transferred"`      // 已传输字节数
	TotalBytes      uint64      `json:"total_bytes"`      // 总字节数
	SpeedBytes      uint64      `json:"speed_bytes"`      // 速度
	ErrorMsg        string      `json:"error_msg,omitempty"` // 错误信息
	StartedAt       time.Time   `json:"started_at"`       // 开始时间
	CompletedAt     *time.Time  `json:"completed_at,omitempty"` // 完成时间
}

// BrowseItem 浏览项.
type BrowseItem struct {
	Path         string    `json:"path"`          // 文件路径
	Name         string    `json:"name"`          // 文件名
	IsDir        bool      `json:"is_dir"`        // 是否目录
	Size         uint64    `json:"size"`          // 文件大小
	ModTime      time.Time `json:"mod_time"`      // 修改时间
	Mode         string    `json:"mode"`          // 权限模式
}

// ========== 存储池类型 ==========

// StoragePool 存储池.
type StoragePool struct {
	ID              string    `json:"id"`               // 存储池 ID
	Name            string    `json:"name"`             // 存储池名称
	Path            string    `json:"path"`             // 存储路径
	TotalBytes      uint64    `json:"total_bytes"`      // 总容量（字节）
	UsedBytes       uint64    `json:"used_bytes"`       // 已用空间（字节）
	FreeBytes       uint64    `json:"free_bytes"`       // 可用空间（字节）
	BackupCount     int       `json:"backup_count"`     // 备份数量
	RestorePointCount int     `json:"restore_point_count"` // 恢复点数量
	DedupEnabled    bool      `json:"dedup_enabled"`    // 是否启用去重
	DedupRatio      float64   `json:"dedup_ratio"`      // 去重比
	CreatedAt       time.Time `json:"created_at"`       // 创建时间
}

// ========== 统计类型 ==========

// BackupStats 备份统计.
type BackupStats struct {
	TotalAgents        int     `json:"total_agents"`         // Agent 总数
	OnlineAgents       int     `json:"online_agents"`        // 在线 Agent 数
	TotalTasks         int     `json:"total_tasks"`          // 任务总数
	RunningTasks       int     `json:"running_tasks"`        // 运行中任务数
	TotalRestorePoints int     `json:"total_restore_points"` // 恢复点总数
	TotalDataBytes     uint64  `json:"total_data_bytes"`     // 总数据量
	TotalStorageBytes  uint64  `json:"total_storage_bytes"`  // 总存储量
	CompressionRatio   float64 `json:"compression_ratio"`    // 压缩比
	DedupRatio         float64 `json:"dedup_ratio"`          // 去重比
	SuccessRate        float64 `json:"success_rate"`         // 成功率
	LastBackupAt       *time.Time `json:"last_backup_at,omitempty"` // 最后备份时间
}

// StorageUsage 存储使用情况.
type StorageUsage struct {
	Pools         []StoragePool `json:"pools"`          // 存储池列表
	TotalBytes    uint64        `json:"total_bytes"`    // 总容量
	UsedBytes     uint64        `json:"used_bytes"`     // 已用空间
	FreeBytes     uint64        `json:"free_bytes"`     // 可用空间
	UsagePercent  float64       `json:"usage_percent"`  // 使用率
	RetainedDays  int           `json:"retained_days"`  // 保留天数
	OldestBackup  *time.Time    `json:"oldest_backup,omitempty"` // 最早备份时间
	NewestBackup  *time.Time    `json:"newest_backup,omitempty"` // 最新备份时间
}
